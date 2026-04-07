#!/usr/bin/env python3
"""Export trained Engine B to ONNX for Go inference."""
import argparse
import os
import pickle
import torch

from model import build_model


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--model-dir', type=str, default=os.path.dirname(os.path.abspath(__file__)))
    parser.add_argument('--seq-len', type=int, default=8)
    args = parser.parse_args()

    scaler_path = os.path.join(args.model_dir, 'scaler.pkl')
    model_path = os.path.join(args.model_dir, 'model.pt')
    onnx_path = os.path.join(args.model_dir, 'financial_lstm.onnx')

    with open(scaler_path, 'rb') as f:
        scaler = pickle.load(f)

    input_dim = len(scaler.mean_)
    model = build_model(input_dim=input_dim)
    model.load_state_dict(torch.load(model_path, map_location='cpu'))
    model.eval()

    dummy_input = torch.randn(1, args.seq_len, input_dim)

    torch.onnx.export(
        model,
        dummy_input,
        onnx_path,
        input_names=['financial_seq'],
        output_names=['roe_dir', 'rev_dir', 'mscore_dir', 'health_score'],
        dynamic_axes={
            'financial_seq': {0: 'batch'},
            'roe_dir': {0: 'batch'},
            'rev_dir': {0: 'batch'},
            'mscore_dir': {0: 'batch'},
            'health_score': {0: 'batch'},
        },
        opset_version=11,
    )
    print(f'ONNX exported to {onnx_path} (input_dim={input_dim})')


if __name__ == '__main__':
    main()
