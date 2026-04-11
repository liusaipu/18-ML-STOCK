#!/usr/bin/env python3
"""
Unified inference entry for Engine A, B & D.
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
    ort = None

# Engine D imports
try:
    from sklearn.ensemble import GradientBoostingClassifier
    ENGINE_D_AVAILABLE = True
except ImportError:
    ENGINE_D_AVAILABLE = False


SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))

# Load sessions lazily
_session_a = None
_session_b = None
_scaler_b = None
_model_d = None


def get_model_d():
    """加载 Engine-D 风险预警模型"""
    global _model_d
    if _model_d is None and ENGINE_D_AVAILABLE:
        model_path = os.path.join(SCRIPT_DIR, "engine_d_risk", "model", "engine_d_model.pkl")
        if os.path.exists(model_path):
            with open(model_path, "rb") as f:
                _model_d = pickle.load(f)
    return _model_d


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


def infer_engine_d(payload):
    """
    Engine D: 风险预警模型推理
    payload: {
        "features": [mscore, zscore, cash_deviation, ar_risk, ...]  # 26维特征
    }
    
    Returns:
    {
        "risk_label": 0/1,  # 0=健康, 1=风险
        "risk_prob": 0.85,  # 风险概率
        "risk_level": "高风险",  # 低风险/中风险/高风险
        "top_factors": ["ascore", "gm_risk", ...]  # 主要风险因子
    }
    """
    model = get_model_d()
    if model is None:
        # 模型未加载，返回基于规则的简单评估
        features = payload.get("features", [])
        if len(features) < 6:
            return {"error": "insufficient features for Engine-D"}
        
        # 简化的规则评估
        ascore = features[5] if len(features) > 5 else 0  # A-Score
        zscore = features[1] if len(features) > 1 else 3  # Z-Score
        mscore = features[0] if len(features) > 0 else -3  # M-Score
        
        risk_score = 0
        if ascore > 60:
            risk_score += 40
        elif ascore > 50:
            risk_score += 20
        if zscore < 1.81:
            risk_score += 30
        elif zscore < 2.99:
            risk_score += 15
        if mscore > -2.22:
            risk_score += 30
        
        risk_prob = min(risk_score / 100.0, 0.99)
        risk_label = 1 if risk_prob > 0.5 else 0
        
        if risk_prob > 0.7:
            risk_level = "高风险"
        elif risk_prob > 0.4:
            risk_level = "中风险"
        else:
            risk_level = "低风险"
        
        return {
            "risk_label": risk_label,
            "risk_prob": round(risk_prob, 4),
            "risk_level": risk_level,
            "top_factors": ["ascore", "zscore", "mscore"] if risk_label == 1 else [],
            "model_loaded": False,
        }
    
    # 使用模型推理
    features = np.array(payload.get("features", []), dtype=np.float32).reshape(1, -1)
    if features.shape[1] != 25:
        return {"error": f"expected 25 features, got {features.shape[1]}"}
    
    risk_proba = model.predict_proba(features)[0]
    risk_label = model.predict(features)[0]
    
    risk_prob = float(risk_proba[1])  # 风险类的概率
    
    if risk_prob > 0.7:
        risk_level = "高风险"
    elif risk_prob > 0.4:
        risk_level = "中风险"
    else:
        risk_level = "低风险"
    
    # 获取特征重要性作为风险因子
    importances = model.feature_importances_
    feature_names = [
        'mscore', 'zscore', 'cash_deviation', 'ar_risk', 'gm_risk', 'ascore',
        'roe', 'gross_margin', 'revenue_growth', 'debt_ratio', 'ncf_to_profit',
        'goodwill_to_equity', 'inventory_turnover', 'receivable_turnover',
        'pe_ttm', 'pb', 'market_cap', 'turnover_20d', 'volatility_60d', 'max_drawdown_1y',
        'pledge_ratio', 'regulatory_inquiry_count_1y', 'major_shareholder_reduction_1y',
        'auditor_switch_count_2y'
    ]
    
    # 找出最重要的3个特征
    top_indices = np.argsort(importances)[-3:][::-1]
    top_factors = [feature_names[i] for i in top_indices]
    
    return {
        "risk_label": int(risk_label),
        "risk_prob": round(risk_prob, 4),
        "risk_level": risk_level,
        "top_factors": top_factors,
        "model_loaded": True,
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
        elif engine == "D":
            result = infer_engine_d(payload)
        else:
            result = {"error": f"unknown engine {engine}"}
        print(json.dumps(result))
    except Exception as e:
        print(json.dumps({"error": str(e)}))


if __name__ == "__main__":
    main()
