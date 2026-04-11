#!/usr/bin/env python3
"""
行业均值数据库自动更新脚本
从 A 股所有股票财务数据计算行业均值

用法: python3 update_industry_database.py <data_dir>
"""

import json
import os
import sys
from datetime import datetime
from typing import Dict, List
import math

try:
    import akshare as ak
except ImportError:
    print(json.dumps({"success": False, "error": "akshare not installed"}))
    sys.exit(1)


def safe_div(a: float, b: float, default: float = 0.0) -> float:
    """安全除法"""
    if b == 0 or math.isnan(b):
        return default
    return a / b


def load_existing_db(data_dir: str) -> dict:
    """加载现有行业数据库"""
    path = os.path.join(data_dir, "industry_database.json")
    if os.path.exists(path):
        with open(path, "r", encoding="utf-8") as f:
            return json.load(f)
    return {"version": "1.0", "updated_at": "", "industries": {}}


def save_db(data_dir: str, db: dict):
    """保存行业数据库"""
    path = os.path.join(data_dir, "industry_database.json")
    with open(path, "w", encoding="utf-8") as f:
        json.dump(db, f, ensure_ascii=False, indent=2)


def get_industry_classification() -> Dict[str, str]:
    """
    获取 A 股所有股票的行业分类
    返回: {股票代码: 行业名称}
    """
    try:
        # 使用 akshare 获取行业分类
        df = ak.stock_industry_category_cninfo()
        if df is None or df.empty:
            return {}
        
        result = {}
        for _, row in df.iterrows():
            code = str(row.get("股票代码", "")).strip()
            industry = str(row.get("行业分类", "")).strip()
            if code and industry:
                result[code] = industry
        return result
    except Exception as e:
        print(f"获取行业分类失败: {e}", file=sys.stderr)
        return {}


def get_stock_financial_metrics(code: str) -> dict:
    """
    获取单只股票的最新财务指标
    返回: {roe, gross_margin, revenue_growth, debt_ratio, ...}
    """
    try:
        # 获取主要财务指标
        df = ak.stock_financial_analysis_indicator(symbol=f"{code[:6]}")
        if df is None or df.empty:
            return None
        
        # 取最新一期数据
        row = df.iloc[0]
        
        # 提取关键指标
        metrics = {
            "roe": float(row.get("净资产收益率", 0) or 0),
            "gross_margin": float(row.get("毛利率", 0) or 0),
            "revenue_growth": float(row.get("营业收入同比增长率", 0) or 0),
            "debt_ratio": float(row.get("资产负债率", 0) or 0),
        }
        return metrics
    except Exception as e:
        # 静默处理单个股票的错误
        return None


def calculate_industry_metrics(stocks_data: List[dict]) -> dict:
    """
    计算行业均值指标
    """
    if not stocks_data:
        return None
    
    # 收集各指标
    roes = [d["roe"] for d in stocks_data if d.get("roe")]
    gms = [d["gross_margin"] for d in stocks_data if d.get("gross_margin")]
    growths = [d["revenue_growth"] for d in stocks_data if d.get("revenue_growth")]
    debts = [d["debt_ratio"] for d in stocks_data if d.get("debt_ratio")]
    
    def median(vals: List[float]) -> float:
        if not vals:
            return 0.0
        s = sorted(vals)
        n = len(s)
        if n % 2 == 1:
            return s[n // 2]
        return (s[n // 2 - 1] + s[n // 2]) / 2
    
    def avg(vals: List[float]) -> float:
        if not vals:
            return 0.0
        return sum(vals) / len(vals)
    
    return {
        "count": len(stocks_data),
        "roe": avg(roes) if roes else 0,
        "roe_median": median(roes) if roes else 0,
        "gross_margin": avg(gms) if gms else 0,
        "revenue_growth": avg(growths) if growths else 0,
        "debt_ratio": avg(debts) if debts else 0,
        "cash_ratio": 0.0,  # 暂不计算
        "m_score": 0.0,     # 暂不计算
    }


def main():
    if len(sys.argv) < 2:
        print("Usage: update_industry_database.py <data_dir>", file=sys.stderr)
        sys.exit(1)
    
    data_dir = sys.argv[1]
    os.makedirs(data_dir, exist_ok=True)
    
    print("开始更新行业均值数据库...", file=sys.stderr)
    
    # 1. 加载现有数据库
    db = load_existing_db(data_dir)
    
    # 2. 获取行业分类
    print("获取行业分类...", file=sys.stderr)
    industry_map = get_industry_classification()
    if not industry_map:
        result = {"success": False, "error": "无法获取行业分类数据"}
        print(json.dumps(result, ensure_ascii=False))
        sys.exit(1)
    
    print(f"获取到 {len(industry_map)} 只股票的行业分类", file=sys.stderr)
    
    # 3. 按行业分组
    industry_groups: Dict[str, List[str]] = {}
    for code, industry in industry_map.items():
        if industry not in industry_groups:
            industry_groups[industry] = []
        industry_groups[industry].append(code)
    
    print(f"共 {len(industry_groups)} 个行业", file=sys.stderr)
    
    # 4. 采样计算（每个行业最多取50只股票，避免太慢）
    updated_count = 0
    skipped_count = 0
    errors = []
    
    for industry, codes in industry_groups.items():
        print(f"处理行业: {industry} ({len(codes)} 只股票)", file=sys.stderr)
        
        # 采样（每个行业最多50只）
        sample_codes = codes[:50] if len(codes) > 50 else codes
        
        stocks_data = []
        for code in sample_codes:
            metrics = get_stock_financial_metrics(code)
            if metrics:
                stocks_data.append(metrics)
        
        if len(stocks_data) < 3:
            skipped_count += 1
            continue
        
        # 计算行业均值
        metrics = calculate_industry_metrics(stocks_data)
        if metrics:
            db["industries"][industry] = {
                "industry": industry,
                **metrics,
                "updated_at": datetime.now().strftime("%Y-%m-%dT%H:%M:%S"),
            }
            updated_count += 1
    
    # 5. 更新元信息
    db["version"] = "1.0"
    db["updated_at"] = datetime.now().strftime("%Y-%m-%dT%H:%M:%S")
    
    # 6. 保存数据库
    save_db(data_dir, db)
    
    result = {
        "success": True,
        "path": os.path.join(data_dir, "industry_database.json"),
        "total_industries": len(db["industries"]),
        "updated_count": updated_count,
        "skipped_count": skipped_count,
        "errors": errors,
    }
    
    print(json.dumps(result, ensure_ascii=False))


if __name__ == "__main__":
    main()
