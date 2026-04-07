#!/usr/bin/env python3
"""
Train Engine B: Financial LSTM.
Usage:
  python train.py --data-dir ~/.config/stock-analyzer/data --epochs 100
Or without data-dir to use synthetic data:
  python train.py --epochs 50
"""
import argparse
import os
import pickle
import numpy as np
import torch
import torch.nn as nn
from torch.utils.data import Dataset, DataLoader
from sklearn.model_selection import train_test_split
from sklearn.preprocessing import StandardScaler

from model import build_model
from features import build_dataset, generate_synthetic_data


class FinancialDataset(Dataset):
    def __init__(self, X, y_roe, y_rev, y_mscore, y_health):
        self.X = torch.from_numpy(X)
        self.y_roe = torch.from_numpy(y_roe)
        self.y_rev = torch.from_numpy(y_rev)
        self.y_mscore = torch.from_numpy(y_mscore)
        self.y_health = torch.from_numpy(y_health).unsqueeze(1)

    def __len__(self):
        return len(self.X)

    def __getitem__(self, idx):
        return {
            'x': self.X[idx],
            'roe_dir': self.y_roe[idx],
            'rev_dir': self.y_rev[idx],
            'mscore_dir': self.y_mscore[idx],
            'health_score': self.y_health[idx],
        }


def train_epoch(model, loader, optimizer, device):
    model.train()
    total_loss = 0.0
    for batch in loader:
        x = batch['x'].to(device)
        out = model(x)
        loss_cls = nn.CrossEntropyLoss()(out['roe_dir'], batch['roe_dir'].to(device)) \
                 + nn.CrossEntropyLoss()(out['rev_dir'], batch['rev_dir'].to(device)) \
                 + nn.CrossEntropyLoss()(out['mscore_dir'], batch['mscore_dir'].to(device))
        loss_reg = nn.MSELoss()(out['health_score'], batch['health_score'].to(device))
        loss = loss_cls + 0.01 * loss_reg

        optimizer.zero_grad()
        loss.backward()
        optimizer.step()
        total_loss += loss.item()
    return total_loss / len(loader)


def eval_epoch(model, loader, device):
    model.eval()
    total_loss = 0.0
    correct = {'roe': 0, 'rev': 0, 'mscore': 0}
    total = 0
    with torch.no_grad():
        for batch in loader:
            x = batch['x'].to(device)
            out = model(x)
            loss_cls = nn.CrossEntropyLoss()(out['roe_dir'], batch['roe_dir'].to(device)) \
                     + nn.CrossEntropyLoss()(out['rev_dir'], batch['rev_dir'].to(device)) \
                     + nn.CrossEntropyLoss()(out['mscore_dir'], batch['mscore_dir'].to(device))
            loss_reg = nn.MSELoss()(out['health_score'], batch['health_score'].to(device))
            loss = loss_cls + 0.01 * loss_reg
            total_loss += loss.item()

            for key in ['roe', 'rev', 'mscore']:
                pred = out[f'{key}_dir'].argmax(dim=1)
                correct[key] += (pred == batch[f'{key}_dir'].to(device)).sum().item()
            total += len(x)
    acc = {k: v / total for k, v in correct.items()}
    return total_loss / len(loader), acc


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--data-dir', type=str, default=None,
                        help='Path to stock-analyzer data dir containing symbol subdirs')
    parser.add_argument('--epochs', type=int, default=80)
    parser.add_argument('--batch-size', type=int, default=64)
    parser.add_argument('--lr', type=float, default=1e-3)
    parser.add_argument('--output-dir', type=str, default=os.path.dirname(os.path.abspath(__file__)))
    args = parser.parse_args()

    device = torch.device('cuda' if torch.cuda.is_available() else 'cpu')
    print(f'Using device: {device}')

    if args.data_dir:
        print(f'Loading real data from {args.data_dir} ...')
        data = build_dataset(args.data_dir)
        if data[0] is None:
            print('No sufficient real data found, falling back to synthetic data.')
            data = generate_synthetic_data()
    else:
        print('Using synthetic training data...')
        data = generate_synthetic_data()

    X, y_roe, y_rev, y_mscore, y_health = data
    print(f'Dataset size: {len(X)}')

    # Normalize per feature across all timesteps
    scaler = StandardScaler()
    orig_shape = X.shape
    X_2d = X.reshape(-1, X.shape[-1])
    scaler.fit(X_2d)
    X = scaler.transform(X_2d).reshape(orig_shape).astype(np.float32)

    X_train, X_val, y_roe_tr, y_roe_va, y_rev_tr, y_rev_va, y_mscore_tr, y_mscore_va, y_health_tr, y_health_va = \
        train_test_split(X, y_roe, y_rev, y_mscore, y_health, test_size=0.2, random_state=42)

    train_ds = FinancialDataset(X_train, y_roe_tr, y_rev_tr, y_mscore_tr, y_health_tr)
    val_ds = FinancialDataset(X_val, y_roe_va, y_rev_va, y_mscore_va, y_health_va)
    train_loader = DataLoader(train_ds, batch_size=args.batch_size, shuffle=True)
    val_loader = DataLoader(val_ds, batch_size=args.batch_size)

    input_dim = X.shape[-1]
    model = build_model(input_dim=input_dim).to(device)
    optimizer = torch.optim.Adam(model.parameters(), lr=args.lr)
    scheduler = torch.optim.lr_scheduler.ReduceLROnPlateau(optimizer, patience=10, factor=0.5)

    best_val_loss = float('inf')
    for epoch in range(1, args.epochs + 1):
        tr_loss = train_epoch(model, train_loader, optimizer, device)
        val_loss, val_acc = eval_epoch(model, val_loader, device)
        scheduler.step(val_loss)
        if val_loss < best_val_loss:
            best_val_loss = val_loss
            torch.save(model.state_dict(), os.path.join(args.output_dir, 'model.pt'))
            with open(os.path.join(args.output_dir, 'scaler.pkl'), 'wb') as f:
                pickle.dump(scaler, f)
        if epoch % 10 == 0 or epoch == 1:
            print(f'Epoch {epoch:03d} | train_loss={tr_loss:.4f} | val_loss={val_loss:.4f} | '
                  f'val_acc roe={val_acc["roe"]:.3f} rev={val_acc["rev"]:.3f} mscore={val_acc["mscore"]:.3f}')

    print('Training complete. Best model saved to model.pt')


if __name__ == '__main__':
    main()
