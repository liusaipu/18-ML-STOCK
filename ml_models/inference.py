#!/usr/bin/env python3
"""
Unified ONNX inference entry for Engine A & Engine B.
Go calls this script via stdin/stdout JSON.
"""
import json
import os
import sys
import pickle
import numpy as np

try:
    import onnxruntime as ort
except ImportError:
    print(json.dumps({"error": "onnxruntime not installed. Run: pip install onnxruntime"}), file=sys.stderr)
    sys.exit(1)


SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))

# Load sessions lazily
_session_a = None
_session_b = None
_scaler_b = None


def get_session_a():
    global _session_a
    if _session_a is None:
        path = os.path.join(SCRIPT_DIR, "engine_a_sentiment", "sentiment_price_fusion.onnx")
        _session_a = ort.InferenceSession(path, providers=['CPUExecutionProvider'])
    return _session_a


def get_session_b():
    global _session_b, _scaler_b
    if _session_b is None:
        path = os.path.join(SCRIPT_DIR, "engine_b_financial", "financial_lstm.onnx")
        _session_b = ort.InferenceSession(path, providers=['CPUExecutionProvider'])
        with open(os.path.join(SCRIPT_DIR, "engine_b_financial", "scaler.pkl"), "rb") as f:
            _scaler_b = pickle.load(f)
    return _session_b, _scaler_b


def softmax(x):
    e = np.exp(x - np.max(x, axis=-1, keepdims=True))
    return e / np.sum(e, axis=-1, keepdims=True)


def infer_engine_a(payload):
    """
    payload: {
        "text_seq": [[...], ...],  # [16, 32]
        "price_seq": [[...], ...]  # [16, 24]
    }
    """
    text_seq = np.array(payload["text_seq"], dtype=np.float32)
    price_seq = np.array(payload["price_seq"], dtype=np.float32)
    if text_seq.ndim == 2:
        text_seq = np.expand_dims(text_seq, 0)
    if price_seq.ndim == 2:
        price_seq = np.expand_dims(price_seq, 0)

    sess = get_session_a()
    out = sess.run(["movement_logits", "abnormal_prob"], {"text_seq": text_seq, "price_seq": price_seq})
    movement_logits, prob = out
    movement_probs = softmax(movement_logits)[0]
    movement_map = ["down", "flat", "up"]
    direction = movement_map[int(np.argmax(movement_probs))]

    return {
        "direction": direction,
        "direction_probs": {
            "down": round(float(movement_probs[0]), 4),
            "flat": round(float(movement_probs[1]), 4),
            "up": round(float(movement_probs[2]), 4),
        },
        "abnormal_prob": round(float(prob[0][0]), 4),
    }


def infer_engine_b(payload):
    """
    payload: {
        "financial_seq": [[...], ...]  # [8, N_features]
    }
    """
    seq = np.array(payload["financial_seq"], dtype=np.float32)
    if seq.ndim == 2:
        seq = np.expand_dims(seq, 0)

    sess, scaler = get_session_b()
    # normalize
    orig_shape = seq.shape
    seq_2d = seq.reshape(-1, seq.shape[-1])
    seq = scaler.transform(seq_2d).reshape(orig_shape).astype(np.float32)

    out = sess.run(["roe_dir", "rev_dir", "mscore_dir", "health_score"], {"financial_seq": seq})
    roe_dir, rev_dir, mscore_dir, health_score = out

    dir_map = ["down", "flat", "up"]
    roe_probs = softmax(roe_dir)[0]
    rev_probs = softmax(rev_dir)[0]
    mscore_probs = softmax(mscore_dir)[0]

    return {
        "roe": {
            "direction": dir_map[int(np.argmax(roe_probs))],
            "confidence": round(float(np.max(roe_probs)), 4),
        },
        "revenue": {
            "direction": dir_map[int(np.argmax(rev_probs))],
            "confidence": round(float(np.max(rev_probs)), 4),
        },
        "mscore": {
            "direction": dir_map[int(np.argmax(mscore_probs))],
            "confidence": round(float(np.max(mscore_probs)), 4),
        },
        "health_score": round(float(health_score[0][0]), 2),
    }


def main():
    req = json.load(sys.stdin)
    engine = req.get("engine")
    payload = req.get("payload", {})
    try:
        if engine == "A":
            result = infer_engine_a(payload)
        elif engine == "B":
            result = infer_engine_b(payload)
        else:
            result = {"error": f"unknown engine {engine}"}
        print(json.dumps(result))
    except Exception as e:
        print(json.dumps({"error": str(e)}))


if __name__ == "__main__":
    main()
