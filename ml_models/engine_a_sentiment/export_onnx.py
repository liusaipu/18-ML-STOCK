#!/usr/bin/env python3
"""Export trained Engine A to ONNX for Go inference."""
import argparse
import os
import torch

from model import build_model


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--model-dir', type=str, default=os.path.dirname(os.path.abspath(__file__)))
    parser.add_argument('--seq-len', type=int, default=16)
    args = parser.parse_args()

    model_path = os.path.join(args.model_dir, 'model.pt')
    onnx_path = os.path.join(args.model_dir, 'sentiment_price_fusion.onnx')

    # infer dims from dummy check (we don't store meta here, use fixed dims from train.py defaults)
    text_dim = 32
    price_dim = 24
    model = build_model(text_dim=text_dim, price_dim=price_dim)
    model.load_state_dict(torch.load(model_path, map_location='cpu'))
    model.eval()

    dummy_text = torch.randn(1, args.seq_len, text_dim)
    dummy_price = torch.randn(1, args.seq_len, price_dim)

    torch.onnx.export(
        model,
        (dummy_text, dummy_price),
        onnx_path,
        input_names=['text_seq', 'price_seq'],
        output_names=['movement_logits', 'abnormal_prob'],
        dynamic_axes={
            'text_seq': {0: 'batch'},
            'price_seq': {0: 'batch'},
            'movement_logits': {0: 'batch'},
            'abnormal_prob': {0: 'batch'},
        },
        opset_version=11,
    )
    print(f'ONNX exported to {onnx_path} (text_dim={text_dim}, price_dim={price_dim})')


if __name__ == '__main__':
    main()
