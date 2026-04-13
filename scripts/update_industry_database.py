#!/usr/bin/env python3
"""
行业均值数据库自动更新脚本
从本地已下载的股票财务数据计算行业均值

用法: python3 update_industry_database.py <data_dir>
"""

import json
import os
import sys
from datetime import datetime
from typing import Dict, List
import math
import io

# 修复 Windows 控制台编码问题
sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')
sys.stderr = io.TextIOWrapper(sys.stderr.buffer, encoding='utf-8')


def safe_div(a: float, b: float, default: float = 0.0) -> float:
    """安全除法"""
    if b == 0 or math.isnan(b):
        return default
    return a / b


def load_existing_db(data_dir: str) -> dict:
    """加载现有行业数据库"""
    path = os.path.join(data_dir, "industry_database.json")
    if os.path.exists(path):
        try:
            with open(path, "r", encoding="utf-8") as f:
                return json.load(f)
        except Exception as e:
            print(f"加载现有数据库失败: {e}", file=sys.stderr)
    return {"version": "1.0", "updated_at": "", "industries": {}}


def save_db(data_dir: str, db: dict):
    """保存行业数据库"""
    path = os.path.join(data_dir, "industry_database.json")
    with open(path, "w", encoding="utf-8") as f:
        json.dump(db, f, ensure_ascii=False, indent=2)


def scan_stocks_data(data_dir: str) -> Dict[str, List[dict]]:
    """
    扫描本地股票数据，按行业分组
    返回: {行业名: [股票指标列表]}
    """
    data_path = os.path.join(data_dir, "data")
    if not os.path.exists(data_path):
        return {}
    
    industry_groups: Dict[str, List[dict]] = {}
    
    # 遍历所有股票目录
    for stock_dir in os.listdir(data_path):
        stock_path = os.path.join(data_path, stock_dir)
        if not os.path.isdir(stock_path):
            continue
        
        try:
            # 读取基本资料
            profile_path = os.path.join(stock_path, "profile.json")
            if not os.path.exists(profile_path):
                continue
            
            with open(profile_path, "r", encoding="utf-8") as f:
                profile = json.load(f)
            
            industry = profile.get("industry", "").strip()
            if not industry:
                continue
            
            # 读取最新财务数据
            metrics = extract_financial_metrics(stock_path)
            if metrics:
                metrics["code"] = stock_dir
                if industry not in industry_groups:
                    industry_groups[industry] = []
                industry_groups[industry].append(metrics)
                
        except Exception as e:
            print(f"处理 {stock_dir} 失败: {e}", file=sys.stderr)
            continue
    
    return industry_groups


def get_latest_value(data: dict, keys: list) -> float:
    """从 {科目: {日期: 值}} 格式中获取最新日期的值"""
    for key in keys:
        if key in data:
            values = data[key]
            if not values:
                continue
            # 获取最新日期
            latest_date = sorted(values.keys(), reverse=True)[0]
            val = values.get(latest_date, 0)
            return float(val) if val else 0
    return 0


def extract_financial_metrics(stock_path: str) -> dict:
    """
    从股票财务数据中提取关键指标
    数据格式: {科目: {日期: 值}}
    """
    try:
        # 读取资产负债表
        with open(os.path.join(stock_path, "balance_sheet.json"), "r", encoding="utf-8") as f:
            balance = json.load(f)
        
        # 读取利润表
        with open(os.path.join(stock_path, "income_statement.json"), "r", encoding="utf-8") as f:
            income = json.load(f)
        
        # 读取现金流量表
        with open(os.path.join(stock_path, "cash_flow.json"), "r", encoding="utf-8") as f:
            cashflow = json.load(f)
        
        # 提取关键指标（取最新年份）
        total_equity = get_latest_value(balance, ["归属于母公司股东权益合计", "所有者权益合计", "股东权益合计"])
        net_profit = get_latest_value(income, ["归属于母公司股东的净利润", "净利润"])
        revenue = get_latest_value(income, ["营业收入", "营业总收入"])
        total_assets = get_latest_value(balance, ["资产总计", "资产合计", "总资产"])
        total_liabilities = get_latest_value(balance, ["负债合计", "负债总计", "总负债"])
        operating_cashflow = get_latest_value(cashflow, ["经营活动产生的现金流量净额", "经营活动现金流量净额"])
        
        # 计算指标
        roe = safe_div(net_profit * 100, total_equity) if total_equity else 0
        debt_ratio = safe_div(total_liabilities * 100, total_assets) if total_assets else 0
        cash_ratio = safe_div(operating_cashflow * 100, net_profit) if net_profit else 0
        
        # 尝试计算毛利率（需要营业成本）
        operating_cost = get_latest_value(income, ["营业成本"])
        gross_margin = safe_div((revenue - operating_cost) * 100, revenue) if revenue else 0
        
        # 计算营收增长（需要前一年数据）
        revenue_growth = 0
        if "营业收入" in income or "营业总收入" in income:
            revenue_key = "营业收入" if "营业收入" in income else "营业总收入"
            years = sorted(income[revenue_key].keys(), reverse=True)
            if len(years) >= 2:
                latest_revenue = float(income[revenue_key].get(years[0], 0) or 0)
                prev_revenue = float(income[revenue_key].get(years[1], 0) or 0)
                if prev_revenue > 0:
                    revenue_growth = (latest_revenue - prev_revenue) * 100 / prev_revenue
        
        return {
            "roe": round(roe, 2),
            "gross_margin": round(gross_margin, 2),
            "revenue_growth": round(revenue_growth, 2),
            "debt_ratio": round(debt_ratio, 2),
            "cash_ratio": round(cash_ratio, 2),
        }
    except Exception as e:
        print(f"提取财务指标失败: {e}", file=sys.stderr)
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
    cashs = [d["cash_ratio"] for d in stocks_data if d.get("cash_ratio")]
    
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
        "roe": round(avg(roes), 2) if roes else 0,
        "roe_median": round(median(roes), 2) if roes else 0,
        "gross_margin": round(avg(gms), 2) if gms else 0,
        "revenue_growth": round(avg(growths), 2) if growths else 0,
        "debt_ratio": round(avg(debts), 2) if debts else 0,
        "cash_ratio": round(avg(cashs), 2) if cashs else 0,
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
    
    # 2. 扫描本地股票数据
    print("扫描本地股票数据...", file=sys.stderr)
    industry_groups = scan_stocks_data(data_dir)
    
    if not industry_groups:
        result = {"success": False, "error": "未找到任何股票数据，请先生成或下载股票财务数据"}
        print(json.dumps(result, ensure_ascii=False))
        sys.exit(1)
    
    print(f"找到 {len(industry_groups)} 个行业", file=sys.stderr)
    
    # 3. 计算各行业均值
    updated_count = 0
    skipped_count = 0
    
    for industry, stocks in industry_groups.items():
        print(f"处理行业: {industry} ({len(stocks)} 只股票)", file=sys.stderr)
        
        if len(stocks) < 2:
            skipped_count += 1
            continue
        
        metrics = calculate_industry_metrics(stocks)
        if metrics:
            db["industries"][industry] = {
                "industry": industry,
                **metrics,
                "updated_at": datetime.now().strftime("%Y-%m-%dT%H:%M:%S"),
            }
            updated_count += 1
    
    # 4. 更新元信息
    db["version"] = "1.0"
    db["updated_at"] = datetime.now().strftime("%Y-%m-%dT%H:%M:%S")
    
    # 5. 保存数据库
    save_db(data_dir, db)
    
    result = {
        "success": True,
        "path": os.path.join(data_dir, "industry_database.json"),
        "total_industries": len(db["industries"]),
        "updated_count": updated_count,
        "skipped_count": skipped_count,
    }
    
    print(json.dumps(result, ensure_ascii=False))


if __name__ == "__main__":
    main()
