#!/usr/bin/env python3
"""
Train Engine A: Sentiment + Price Fusion Transformer.
This script uses synthetic data by default so it can run standalone.
For real deployment, replace generate_synthetic_data() with your own
30-min bucket feature extraction pipeline.
Usage:
  python train.py --epochs 60
"""
import argparse
import os
import numpy as np
import torch
import torch.nn as nn
from torch.utils.data import Dataset, DataLoader
from sklearn.model_selection import train_test_split

from model import build_model


def generate_synthetic_data(n_samples=3000, seq_len=16, text_dim=32, price_dim=24):
    np.random.seed(2024)
    text_seq = []
    price_seq = []
    y_movement = []
    y_prob = []

    for _ in range(n_samples):
        # sentiment features: mean, std, accel, keyword hits, post volume etc.
        t = np.random.randn(seq_len, text_dim).astype(np.float32)
        # inject sentiment acceleration spike for some samples
        spike_idx = np.random.randint(2, seq_len)
        if np.random.rand() > 0.5:
            t[spike_idx, 1] += np.random.uniform(0.8, 2.5)  # accel spike

        p = np.random.randn(seq_len, price_dim).astype(np.float32)
        # correlate price volume with sentiment spike
        if np.random.rand() > 0.5:
            p[spike_idx, 10] += np.random.uniform(1.0, 3.0)  # volume spike

        text_seq.append(t)
        price_seq.append(p)

        # label: 0=down, 1=flat, 2=up
        # higher sentiment + volume -> up
        movement = 1
        prob = 0.3
        if t[spike_idx, 1] > 1.5 and p[spike_idx, 10] > 1.5:
            movement = 2
            prob = 0.8
        elif t[spike_idx, 1] < -1.0:
            movement = 0
            prob = 0.7

        y_movement.append(movement)
        y_prob.append(prob)

    return (
        np.stack(text_seq),
        np.stack(price_seq),
        np.array(y_movement, dtype=np.int64),
        np.array(y_prob, dtype=np.float32),
    )


class FusionDataset(Dataset):
    def __init__(self, text_seq, price_seq, y_movement, y_prob):
        self.text_seq = torch.from_numpy(text_seq)
        self.price_seq = torch.from_numpy(price_seq)
        self.y_movement = torch.from_numpy(y_movement)
        self.y_prob = torch.from_numpy(y_prob).unsqueeze(1)

    def __len__(self):
        return len(self.text_seq)

    def __getitem__(self, idx):
        return {
            'text_seq': self.text_seq[idx],
            'price_seq': self.price_seq[idx],
            'movement': self.y_movement[idx],
            'prob': self.y_prob[idx],
        }


def focal_loss(inputs, targets, alpha=0.25, gamma=2.0):
    ce = nn.CrossEntropyLoss(reduction='none')(inputs, targets)
    pt = torch.exp(-ce)
    return (alpha * (1 - pt) ** gamma * ce).mean()


def train_epoch(model, loader, optimizer, device):
    model.train()
    total_loss = 0.0
    for batch in loader:
        text = batch['text_seq'].to(device)
        price = batch['price_seq'].to(device)
        out = model(text, price)
        loss_cls = focal_loss(out['movement'], batch['movement'].to(device))
        loss_prob = nn.BCELoss()(out['prob'], batch['prob'].to(device))
        loss = loss_cls + 0.5 * loss_prob

        optimizer.zero_grad()
        loss.backward()
        torch.nn.utils.clip_grad_norm_(model.parameters(), 1.0)
        optimizer.step()
        total_loss += loss.item()
    return total_loss / len(loader)


def eval_epoch(model, loader, device):
    model.eval()
    total_loss = 0.0
    correct = 0
    total = 0
    with torch.no_grad():
        for batch in loader:
            text = batch['text_seq'].to(device)
            price = batch['price_seq'].to(device)
            out = model(text, price)
            loss_cls = focal_loss(out['movement'], batch['movement'].to(device))
            loss_prob = nn.BCELoss()(out['prob'], batch['prob'].to(device))
            loss = loss_cls + 0.5 * loss_prob
            total_loss += loss.item()

            pred = out['movement'].argmax(dim=1)
            correct += (pred == batch['movement'].to(device)).sum().item()
            total += len(text)
    return total_loss / len(loader), correct / total


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--epochs', type=int, default=60)
    parser.add_argument('--batch-size', type=int, default=128)
    parser.add_argument('--lr', type=float, default=1e-3)
    parser.add_argument('--output-dir', type=str, default=os.path.dirname(os.path.abspath(__file__)))
    args = parser.parse_args()

    device = torch.device('cuda' if torch.cuda.is_available() else 'cpu')
    print(f'Using device: {device}')

    text_seq, price_seq, y_movement, y_prob = generate_synthetic_data()
    print(f'Dataset size: {len(text_seq)}')

    (
        t_tr, t_va, p_tr, p_va, y_m_tr, y_m_va, y_p_tr, y_p_va
    ) = train_test_split(text_seq, price_seq, y_movement, y_prob, test_size=0.2, random_state=42)

    train_ds = FusionDataset(t_tr, p_tr, y_m_tr, y_p_tr)
    val_ds = FusionDataset(t_va, p_va, y_m_va, y_p_va)
    train_loader = DataLoader(train_ds, batch_size=args.batch_size, shuffle=True)
    val_loader = DataLoader(val_ds, batch_size=args.batch_size)

    text_dim = text_seq.shape[-1]
    price_dim = price_seq.shape[-1]
    model = build_model(text_dim=text_dim, price_dim=price_dim).to(device)
    optimizer = torch.optim.Adam(model.parameters(), lr=args.lr)
    scheduler = torch.optim.lr_scheduler.ReduceLROnPlateau(optimizer, patience=8, factor=0.5)

    best_val_loss = float('inf')
    for epoch in range(1, args.epochs + 1):
        tr_loss = train_epoch(model, train_loader, optimizer, device)
        val_loss, val_acc = eval_epoch(model, val_loader, device)
        scheduler.step(val_loss)
        if val_loss < best_val_loss:
            best_val_loss = val_loss
            torch.save(model.state_dict(), os.path.join(args.output_dir, 'model.pt'))
        if epoch % 10 == 0 or epoch == 1:
            print(f'Epoch {epoch:03d} | train_loss={tr_loss:.4f} | val_loss={val_loss:.4f} | val_acc={val_acc:.3f}')

    print('Training complete. Best model saved to model.pt')


if __name__ == '__main__':
    main()
