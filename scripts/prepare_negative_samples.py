#!/usr/bin/env python3
"""
为 Engine-D 准备负样本（健康公司）
策略：为每个正样本匹配同行业、同市值区间的健康公司

用法: python3 prepare_negative_samples.py
"""

import json
import os
import random
import sys
from datetime import datetime
from pathlib import Path

import akshare as ak
import pandas as pd


def load_json(path: str) -> dict or list:
    if not os.path.exists(path):
        return {}
    with open(path, "r", encoding="utf-8") as f:
        return json.load(f)


def save_json(path: str, data):
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w", encoding="utf-8") as f:
        json.dump(data, f, ensure_ascii=False, indent=2)


def get_all_a_stocks():
    """获取所有 A 股基本信息"""
    try:
        df = ak.stock_info_a_code_name()
        # 统一列名（处理中英文列名差异）
        column_mapping = {
            'code': '代码',
            'name': '名称',
            'Code': '代码',
            'Name': '名称',
        }
        for old, new in column_mapping.items():
            if old in df.columns and new not in df.columns:
                df = df.rename(columns={old: new})
        return df
    except Exception as e:
        print(f"获取股票列表失败: {e}")
        return None


def get_stock_industry(code: str) -> str:
    """获取股票行业信息（简化版）"""
    try:
        # 使用 akshare 获取行业信息
        df = ak.stock_industry_category_cninfo()
        if df is not None and not df.empty:
            row = df[df["股票代码"] == code]
            if not row.empty:
                return str(row.iloc[0].get("行业分类", ""))
    except:
        pass
    return ""


def match_negative_samples(positive_samples: list, ratio: float = 3.0) -> list:
    """
    为每个正样本匹配负样本
    ratio: 负样本/正样本比例，默认 3:1
    
    简化策略：
    1. 获取所有 A 股
    2. 排除所有正样本代码
    3. 随机选取 3 倍于正样本数量的健康公司
    4. 后续可以根据行业/市值进行更精细的匹配
    """
    print("获取所有 A 股列表...")
    df_all = get_all_a_stocks()
    if df_all is None or df_all.empty:
        print("无法获取股票列表")
        return []
    
    # 获取正样本代码集合
    positive_codes = {s["code"] for s in positive_samples}
    
    # 获取列名（可能是中文或英文）
    code_col = "代码" if "代码" in df_all.columns else ("code" if "code" in df_all.columns else df_all.columns[0])
    name_col = "名称" if "名称" in df_all.columns else ("name" if "name" in df_all.columns else df_all.columns[1])
    
    # 排除正样本，并排除名称中包含 ST 的股票
    df_healthy = df_all[
        ~df_all[code_col].isin(positive_codes) & 
        ~df_all[name_col].str.contains("ST|退|摘", na=False)
    ]
    
    print(f"总 A 股数量: {len(df_all)}")
    print(f"正样本数量: {len(positive_codes)}")
    print(f"候选健康公司: {len(df_healthy)}")
    
    # 计算需要的负样本数量
    target_count = int(len(positive_samples) * ratio)
    
    # 如果候选不够，取全部；否则随机采样
    if len(df_healthy) <= target_count:
        df_negative = df_healthy
    else:
        df_negative = df_healthy.sample(n=target_count, random_state=42)
    
    negative_samples = []
    for _, row in df_negative.iterrows():
        code_val = row.get("代码") or row.get("code") or ""
        name_val = row.get("名称") or row.get("name") or ""
        negative_samples.append({
            "code": str(code_val).strip(),
            "name": str(name_val).strip(),
            "label": 0,  # 0 = 健康
            "source": "random_match",
            "match_date": datetime.now().strftime("%Y-%m-%d"),
        })
    
    return negative_samples


def main():
    # 数据目录
    data_dir = os.path.join(
        os.path.dirname(os.path.dirname(os.path.abspath(__file__))),
        "data", "fraud_dataset"
    )
    
    # 加载正样本
    positive_path = os.path.join(data_dir, "positive", "samples.json")
    positive_samples = load_json(positive_path)
    
    if not positive_samples:
        print(f"未找到正样本: {positive_path}")
        return 1
    
    print(f"加载了 {len(positive_samples)} 个正样本")
    print("=" * 60)
    
    # 匹配负样本
    print("开始匹配负样本...")
    negative_samples = match_negative_samples(positive_samples, ratio=3.0)
    
    if not negative_samples:
        print("未能生成负样本")
        return 1
    
    # 保存负样本
    negative_path = os.path.join(data_dir, "negative", "samples.json")
    save_json(negative_path, negative_samples)
    
    print(f"\n✓ 负样本已保存: {negative_path}")
    print(f"  数量: {len(negative_samples)}")
    print(f"  正负样本比例: 1:{len(negative_samples)/len(positive_samples):.1f}")
    
    # 保存完整数据集索引
    dataset_index = {
        "create_time": datetime.now().isoformat(),
        "positive_count": len(positive_samples),
        "negative_count": len(negative_samples),
        "total": len(positive_samples) + len(negative_samples),
        "positive_file": "positive/samples.json",
        "negative_file": "negative/samples.json",
    }
    
    index_path = os.path.join(data_dir, "dataset_index.json")
    save_json(index_path, dataset_index)
    print(f"\n✓ 数据集索引已保存: {index_path}")
    
    return 0


if __name__ == "__main__":
    sys.exit(main())
