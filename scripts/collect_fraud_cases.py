#!/usr/bin/env python3
"""
收集 A 股历史 fraud / delisting / ST 案例，为 Engine-D LightGBM 准备数据集
用法: python3 collect_fraud_cases.py <output_dir>
"""

import json
import os
import sys
from datetime import datetime

import akshare as ak


def save_json(path: str, data: dict or list):
    with open(path, "w", encoding="utf-8") as f:
        json.dump(data, f, ensure_ascii=False, indent=2)


def main():
    if len(sys.argv) < 2:
        output_dir = os.path.join(
            os.path.dirname(os.path.dirname(os.path.abspath(__file__))),
            "data", "fraud_dataset"
        )
    else:
        output_dir = sys.argv[1]

    os.makedirs(output_dir, exist_ok=True)

    cases = {
        "collect_time": datetime.now().isoformat(),
        "st_stocks": [],
        "delisted_stocks": [],
        "penalty_stocks": [],
    }
    errors = []

    # 1. 历史 ST/*ST 列表
    try:
        df_st = ak.stock_zt_pool_em(date=datetime.now().strftime("%Y%m%d"))
        # 注意：akshare 的 ST 接口不稳定，备选使用 stock_zt_pool 并不能直接拿到历史 ST。
        # 更稳定的接口：stock_staq_net 或 stock_info
        df_info = ak.stock_info_a_code_name()
        # 通过名称过滤 ST
        if df_info is not None and not df_info.empty:
            df_st_names = df_info[df_info["名称"].str.contains("ST", na=False)]
            for _, row in df_st_names.iterrows():
                cases["st_stocks"].append({
                    "code": str(row.get("代码", "")).strip(),
                    "name": str(row.get("名称", "")).strip(),
                    "type": "ST",
                    "source": "stock_info_name_filter",
                })
    except Exception as e:
        errors.append(f"st_collect: {e}")

    # 2. 退市股票列表
    try:
        df_delist = ak.stock_info_delist_name()
        if df_delist is not None and not df_delist.empty:
            for _, row in df_delist.iterrows():
                cases["delisted_stocks"].append({
                    "code": str(row.get("代码", "")).strip(),
                    "name": str(row.get("名称", "")).strip(),
                    "delist_date": str(row.get("退市日期", "")).strip(),
                    "type": "delisted",
                    "source": "stock_info_delist_name",
                })
    except Exception as e:
        errors.append(f"delist_collect: {e}")

    # 3. 行政处罚 / 市场禁入（证监会）
    try:
        # 尝试获取证监会处罚列表（若接口存在）
        df_penalty = ak.stock_cg_lawsuit()
        if df_penalty is not None and not df_penalty.empty:
            for _, row in df_penalty.head(200).iterrows():
                cases["penalty_stocks"].append({
                    "code": str(row.get("代码", "")).strip(),
                    "name": str(row.get("名称", "")).strip(),
                    "reason": str(row.get("缘由", "")).strip(),
                    "publish_date": str(row.get("发布日期", "")).strip(),
                    "type": "penalty",
                    "source": "stock_cg_lawsuit",
                })
    except Exception as e:
        errors.append(f"penalty_collect: {e}")

    # 去重
    for key in ("st_stocks", "delisted_stocks", "penalty_stocks"):
        seen = set()
        unique = []
        for item in cases[key]:
            cid = item.get("code", "")
            if cid and cid not in seen:
                seen.add(cid)
                unique.append(item)
        cases[key] = unique

    cases["errors"] = errors
    cases["summary"] = {
        "st_count": len(cases["st_stocks"]),
        "delisted_count": len(cases["delisted_stocks"]),
        "penalty_count": len(cases["penalty_stocks"]),
    }

    raw_path = os.path.join(output_dir, "raw_cases.json")
    save_json(raw_path, cases)
    print(f"Saved raw cases to {raw_path}")
    print(json.dumps(cases["summary"], ensure_ascii=False))


if __name__ == "__main__":
    main()
