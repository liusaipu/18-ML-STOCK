#!/usr/bin/env python3
"""
收集 A 股历史 fraud / delisting / ST 案例，为 Engine-D LightGBM 准备数据集
用法: python3 collect_fraud_cases.py <output_dir>

数据源：
1. ST/*ST 股票：通过 akshare stock_zt_pool 结合名称过滤
2. 退市股票：通过 akshare stock_info 接口 + 历史退市名单
3. 行政处罚：通过证监会公开处罚记录（模拟/手动维护列表）
"""

import json
import os
import sys
from datetime import datetime, timedelta
from pathlib import Path

import akshare as ak
import pandas as pd


def save_json(path: str, data: dict or list):
    """保存 JSON 文件"""
    with open(path, "w", encoding="utf-8") as f:
        json.dump(data, f, ensure_ascii=False, indent=2)


def load_json(path: str) -> dict or list:
    """加载 JSON 文件"""
    if not os.path.exists(path):
        return {}
    with open(path, "r", encoding="utf-8") as f:
        return json.load(f)


def collect_st_stocks():
    """
    收集当前 ST/*ST 股票列表
    通过股票名称中包含 "ST" 来识别
    """
    st_list = []
    errors = []
    
    try:
        # 获取所有 A 股基本信息
        df = ak.stock_info_a_code_name()
        if df is None or df.empty:
            errors.append("stock_info_a_code_name 返回空数据")
            return st_list, errors
            
        # 通过名称过滤 ST 股票
        df_st = df[df["名称"].str.contains("ST", na=False)]
        
        for _, row in df_st.iterrows():
            st_list.append({
                "code": str(row.get("代码", "")).strip(),
                "name": str(row.get("名称", "")).strip(),
                "type": "ST",
                "subtype": "*ST" if "*ST" in str(row.get("名称", "")) else "ST",
                "source": "stock_info_name_filter",
                "collect_date": datetime.now().strftime("%Y-%m-%d"),
            })
    except Exception as e:
        errors.append(f"st_collect: {e}")
    
    return st_list, errors


def collect_delisted_stocks():
    """
    收集已退市股票列表
    尝试多个 akshare 接口
    """
    delist_list = []
    errors = []
    
    # 方法1：尝试 stock_info_sh_delist 和 stock_info_sz_delist
    try:
        # 上海退市
        df_sh = ak.stock_info_sh_delist()
        if df_not_empty(df_sh):
            for _, row in df_sh.iterrows():
                code = str(row.get("公司代码", "")).strip()
                name = str(row.get("公司简称", "")).strip()
                if code:
                    delist_list.append({
                        "code": code,
                        "name": name,
                        "type": "delisted",
                        "market": "SH",
                        "delist_date": str(row.get("退市日期", "")).strip(),
                        "source": "stock_info_sh_delist",
                        "collect_date": datetime.now().strftime("%Y-%m-%d"),
                    })
    except Exception as e:
        errors.append(f"delist_sh: {e}")
    
    try:
        # 深圳退市
        df_sz = ak.stock_info_sz_delist()
        if df_not_empty(df_sz):
            for _, row in df_sz.iterrows():
                code = str(row.get("证券代码", "")).strip()
                name = str(row.get("证券简称", "")).strip()
                if code:
                    delist_list.append({
                        "code": code,
                        "name": name,
                        "type": "delisted",
                        "market": "SZ",
                        "delist_date": str(row.get("退市日期", "")).strip(),
                        "source": "stock_info_sz_delist",
                        "collect_date": datetime.now().strftime("%Y-%m-%d"),
                    })
    except Exception as e:
        errors.append(f"delist_sz: {e}")
    
    # 方法2：尝试 stock_zh_a_hist 获取历史数据判断（备选）
    if not delist_list:
        errors.append("未找到退市股票数据，将使用手动维护列表")
    
    return delist_list, errors


def df_not_empty(df) -> bool:
    """检查 DataFrame 是否不为空"""
    return df is not None and not df.empty


def get_manual_fraud_cases():
    """
    手动维护的历史重大违规/财务造假案例
    这些数据来自公开信息：证监会处罚、市场禁入、造假退市
    """
    cases = [
        # 康美药业 - 财务造假
        {"code": "600518", "name": "康美药业", "type": "fraud", "reason": "财务造假", "year": 2018, "source": "manual"},
        # 康得新 - 财务造假
        {"code": "002450", "name": "康得新", "type": "fraud", "reason": "财务造假", "year": 2019, "source": "manual"},
        # 瑞幸咖啡 - 财务造假（美股，作为参考）
        # {"code": "LK", "name": "瑞幸咖啡", "type": "fraud", "reason": "财务造假", "year": 2020, "source": "manual"},
        # 獐子岛 - 财务造假
        {"code": "002069", "name": "獐子岛", "type": "fraud", "reason": "财务造假", "year": 2018, "source": "manual"},
        # 康得退（康得新退市后）
        {"code": "002450", "name": "康得退", "type": "delisted", "reason": "财务造假退市", "year": 2021, "source": "manual"},
        # 乐视网 - 退市
        {"code": "300104", "name": "乐视网", "type": "delisted", "reason": "经营不善退市", "year": 2020, "source": "manual"},
        # 金亚科技 - 财务造假退市
        {"code": "300028", "name": "金亚科技", "type": "delisted", "reason": "财务造假退市", "year": 2020, "source": "manual"},
        # 千山药机 - 退市
        {"code": "300216", "name": "千山药机", "type": "delisted", "reason": "重大违法退市", "year": 2020, "source": "manual"},
        # 龙力生物 - 财务造假
        {"code": "002604", "name": "龙力生物", "type": "fraud", "reason": "财务造假", "year": 2018, "source": "manual"},
        # 辅仁药业 - 财务造假
        {"code": "600781", "name": "辅仁药业", "type": "fraud", "reason": "财务造假", "year": 2019, "source": "manual"},
        # 瑞华会计所涉及的多个案件...
        # 更多可以手动添加
    ]
    
    # 去重
    seen = set()
    unique_cases = []
    for case in cases:
        key = (case["code"], case["type"])
        if key not in seen:
            seen.add(key)
            case["collect_date"] = datetime.now().strftime("%Y-%m-%d")
            unique_cases.append(case)
    
    return unique_cases


def collect_from_newly_delisted():
    """
    尝试从新近退市股票接口获取
    """
    cases = []
    errors = []
    
    try:
        # 尝试获取近期新股和退市数据
        df_new = ak.stock_new_ipo_cninfo()
        if df_not_empty(df_new):
            print(f"获取到新股数据: {len(df_new)} 条")
    except Exception as e:
        errors.append(f"new_ipo: {e}")
    
    return cases, errors


def main():
    # 确定输出目录
    if len(sys.argv) < 2:
        output_dir = os.path.join(
            os.path.dirname(os.path.dirname(os.path.abspath(__file__))),
            "data", "fraud_dataset"
        )
    else:
        output_dir = sys.argv[1]
    
    os.makedirs(output_dir, exist_ok=True)
    os.makedirs(os.path.join(output_dir, "positive"), exist_ok=True)
    os.makedirs(os.path.join(output_dir, "negative"), exist_ok=True)
    
    print(f"数据将保存到: {output_dir}")
    print("=" * 60)
    
    cases = {
        "collect_time": datetime.now().isoformat(),
        "st_stocks": [],
        "delisted_stocks": [],
        "fraud_stocks": [],
        "manual_cases": [],
    }
    all_errors = []
    
    # 1. 收集 ST 股票
    print("[1/4] 收集 ST/*ST 股票...")
    st_list, st_errors = collect_st_stocks()
    cases["st_stocks"] = st_list
    all_errors.extend(st_errors)
    print(f"  ✓ 找到 {len(st_list)} 只 ST/*ST 股票")
    if st_errors:
        print(f"  ⚠ 错误: {st_errors}")
    
    # 2. 收集退市股票
    print("\n[2/4] 收集退市股票...")
    delist_list, delist_errors = collect_delisted_stocks()
    cases["delisted_stocks"] = delist_list
    all_errors.extend(delist_errors)
    print(f"  ✓ 找到 {len(delist_list)} 只退市股票")
    if delist_errors:
        print(f"  ⚠ 错误: {delist_errors}")
    
    # 3. 获取手动维护的重大违规案例
    print("\n[3/4] 加载手动维护的重大违规案例...")
    manual_cases = get_manual_fraud_cases()
    cases["fraud_stocks"] = manual_cases
    print(f"  ✓ 加载 {len(manual_cases)} 个手动案例")
    
    # 4. 尝试其他数据源
    print("\n[4/4] 尝试其他数据源...")
    _, other_errors = collect_from_newly_delisted()
    all_errors.extend(other_errors)
    
    # 合并所有风险股票代码
    all_risk_codes = set()
    for stock in st_list:
        all_risk_codes.add(stock["code"])
    for stock in delist_list:
        all_risk_codes.add(stock["code"])
    for case in manual_cases:
        all_risk_codes.add(case["code"])
    
    cases["all_risk_codes"] = sorted(list(all_risk_codes))
    cases["errors"] = all_errors
    cases["summary"] = {
        "st_count": len(st_list),
        "delisted_count": len(delist_list),
        "fraud_count": len(manual_cases),
        "total_unique_risk": len(all_risk_codes),
        "errors_count": len(all_errors),
    }
    
    # 保存原始案例
    raw_path = os.path.join(output_dir, "raw_cases.json")
    save_json(raw_path, cases)
    print(f"\n{'=' * 60}")
    print(f"✓ 原始案例已保存: {raw_path}")
    print(f"\n汇总:")
    print(json.dumps(cases["summary"], ensure_ascii=False, indent=2))
    
    if all_errors:
        print(f"\n⚠ 共 {len(all_errors)} 个错误:")
        for e in all_errors[:5]:
            print(f"  - {e}")
    
    # 创建样本列表文件
    positive_samples = []
    for code in all_risk_codes:
        sample = {"code": code, "label": 1}  # 1 = 风险
        # 查找额外信息
        for s in st_list:
            if s["code"] == code:
                sample["name"] = s["name"]
                sample["subtype"] = s.get("subtype", "ST")
                break
        positive_samples.append(sample)
    
    positive_path = os.path.join(output_dir, "positive", "samples.json")
    save_json(positive_path, positive_samples)
    print(f"\n✓ 正样本列表已保存: {positive_path} ({len(positive_samples)} 个)")
    
    # 负样本：从所有 A 股中随机选取同行业同市值的健康公司（简化版）
    # 实际应该在训练时动态匹配
    print("\n[下一步] 需要手动准备负样本（同行业同市值健康公司）")
    print("  或使用 prepare_negative_samples.py 脚本自动匹配")
    
    return 0


if __name__ == "__main__":
    sys.exit(main())
