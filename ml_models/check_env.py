#!/usr/bin/env python3
"""检查 ML 环境配置"""
import sys
import os

SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))

print("=== ML 环境检查 ===")
print(f"Python 版本: {sys.version}")
print(f"脚本目录: {SCRIPT_DIR}")
print()

# 检查 onnxruntime
print("--- Engine A & B (ONNX) ---")
try:
    import onnxruntime as ort
    print(f"onnxruntime 版本: {ort.__version__}")
    print(f"可用提供者: {ort.get_available_providers()}")
    
    # 检查模型文件
    model_a = os.path.join(SCRIPT_DIR, "engine_a_sentiment", "sentiment_price_fusion.onnx")
    model_b = os.path.join(SCRIPT_DIR, "engine_b_financial", "financial_lstm.onnx")
    scaler_b = os.path.join(SCRIPT_DIR, "engine_b_financial", "scaler.pkl")
    
    print(f"模型 A 存在: {os.path.exists(model_a)} ({model_a})")
    print(f"模型 B 存在: {os.path.exists(model_b)} ({model_b})")
    print(f"Scaler B 存在: {os.path.exists(scaler_b)} ({scaler_b})")
except ImportError as e:
    print(f"onnxruntime 未安装: {e}")
except Exception as e:
    print(f"检查 Engine A/B 时出错: {e}")

print()
print("--- Engine D (sklearn) ---")
try:
    import sklearn
    print(f"sklearn 版本: {sklearn.__version__}")
    
    model_d = os.path.join(SCRIPT_DIR, "engine_d_risk", "model", "engine_d_model.pkl")
    print(f"模型 D 存在: {os.path.exists(model_d)} ({model_d})")
    
    if os.path.exists(model_d):
        import pickle
        with open(model_d, "rb") as f:
            model = pickle.load(f)
        print(f"模型类型: {type(model).__name__}")
except ImportError as e:
    print(f"sklearn 未安装: {e}")
except Exception as e:
    print(f"检查 Engine D 时出错: {e}")

print()
print("=== 检查完成 ===")
