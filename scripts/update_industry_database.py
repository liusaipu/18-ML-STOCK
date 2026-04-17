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


def get_year_value(data: dict, keys: list, year: str) -> float:
    """从 {科目: {日期: 值}} 格式中获取指定年份的值"""
    for key in keys:
        if key in data:
            val = data[key].get(year, 0)
            return float(val) if val else 0
    return 0


def calc_mscore(balance: dict, income: dict, cashflow: dict) -> float:
    """计算 Beneish M-Score（需要连续两年数据）"""
    # 收集所有年份
    years = set()
    for d in (balance, income, cashflow):
        for values in d.values():
            years.update(values.keys())
    years = sorted(years, reverse=True)
    if len(years) < 2:
        return 0.0
    cur_year, prev_year = years[0], years[1]

    cur_rev = get_year_value(income, ["营业收入", "营业总收入"], cur_year)
    prev_rev = get_year_value(income, ["营业收入", "营业总收入"], prev_year)
    cur_ar = get_year_value(balance, ["应收票据及应收账款"], cur_year)
    prev_ar = get_year_value(balance, ["应收票据及应收账款"], prev_year)
    cur_asset = get_year_value(balance, ["资产合计", "资产总计"], cur_year)
    prev_asset = get_year_value(balance, ["资产合计", "资产总计"], prev_year)
    cur_liability = get_year_value(balance, ["负债合计", "负债总计"], cur_year)
    prev_liability = get_year_value(balance, ["负债合计", "负债总计"], prev_year)
    cur_current_asset = get_year_value(balance, ["流动资产合计"], cur_year)
    prev_current_asset = get_year_value(balance, ["流动资产合计"], prev_year)
    cur_fixed = get_year_value(balance, ["固定资产"], cur_year)
    prev_fixed = get_year_value(balance, ["固定资产"], prev_year)
    cur_op_profit = get_year_value(income, ["营业利润"], cur_year)
    cur_ocf = get_year_value(cashflow, ["经营活动产生的现金流量净额", "经营活动现金流量净额"], cur_year)
    cur_sales = get_year_value(income, ["销售费用"], cur_year)
    prev_sales = get_year_value(income, ["销售费用"], prev_year)
    cur_admin = get_year_value(income, ["管理费用"], cur_year)
    prev_admin = get_year_value(income, ["管理费用"], prev_year)
    cur_dep = get_year_value(cashflow, ["固定资产折旧、油气资产折耗、生产性生物资产折旧"], cur_year)
    prev_dep = get_year_value(cashflow, ["固定资产折旧、油气资产折耗、生产性生物资产折旧"], prev_year)
    cur_current_liability = get_year_value(balance, ["流动负债合计"], cur_year)
    cur_equity = get_year_value(balance, ["所有者权益合计", "股东权益合计", "归属于母公司股东权益合计"], cur_year)
    cur_retained = (get_year_value(balance, ["未分配利润"], cur_year) +
                    get_year_value(balance, ["盈余公积"], cur_year))
    if cur_retained == 0:
        cur_retained = cur_equity * 0.6

    prev_dsr = safe_div(prev_ar, prev_rev)
    cur_dsr = safe_div(cur_ar, cur_rev)
    dsri = safe_div(cur_dsr, prev_dsr) if prev_dsr > 0 else 1.0

    def gross_margin(rev, cost):
        return safe_div((rev - cost) * 100, rev) if rev else 0

    prev_gm = gross_margin(prev_rev, get_year_value(income, ["营业成本"], prev_year))
    cur_gm = gross_margin(cur_rev, get_year_value(income, ["营业成本"], cur_year))
    gmi = safe_div(prev_gm, cur_gm) if cur_gm != 0 else 1.0

    prev_aq = 1.0 - safe_div(prev_current_asset + prev_fixed, prev_asset)
    cur_aq = 1.0 - safe_div(cur_current_asset + cur_fixed, cur_asset)
    aqi = safe_div(prev_aq, cur_aq) if cur_aq != 0 else 1.0

    sgi = safe_div(cur_rev, prev_rev) if prev_rev != 0 else 1.0

    prev_dep_rate = safe_div(prev_dep, prev_fixed) if prev_fixed != 0 else 0
    cur_dep_rate = safe_div(cur_dep, cur_fixed) if cur_fixed != 0 else 0
    depi = safe_div(prev_dep_rate, cur_dep_rate) if cur_dep_rate != 0 else 1.0

    prev_sga = safe_div(prev_sales + prev_admin, prev_rev)
    cur_sga = safe_div(cur_sales + cur_admin, cur_rev)
    sgai = safe_div(prev_sga, cur_sga) if cur_sga != 0 else 1.0

    tata = safe_div(cur_op_profit - cur_ocf, cur_asset)

    prev_lev = safe_div(prev_liability, prev_asset)
    cur_lev = safe_div(cur_liability, cur_asset)
    lvgi = safe_div(cur_lev, prev_lev) if prev_lev != 0 else 1.0

    mscore = (-4.84 +
              0.92 * dsri +
              0.528 * gmi +
              0.404 * aqi +
              0.892 * sgi +
              0.115 * depi -
              0.172 * sgai +
              4.679 * tata -
              0.327 * lvgi)
    return mscore


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
        
        # 应收账款占比
        receivable = get_latest_value(balance, ["应收票据及应收账款"])
        contract_asset = get_latest_value(balance, ["合同资产"])
        receivable_ratio = safe_div((receivable + contract_asset) * 100, total_assets) if total_assets else 0

        # 尝试计算存货周转率（需要营业成本和两年存货）
        inventory_turnover = 0.0
        operating_cost = get_latest_value(income, ["营业成本"])
        if "存货" in balance:
            years_inv = sorted(balance["存货"].keys(), reverse=True)
            if len(years_inv) >= 2:
                latest_inv = float(balance["存货"].get(years_inv[0], 0) or 0)
                prev_inv = float(balance["存货"].get(years_inv[1], 0) or 0)
                avg_inv = (latest_inv + prev_inv) / 2
                if avg_inv > 0 and operating_cost > 0:
                    inventory_turnover = operating_cost / avg_inv
        
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
        
        # 计算 M-Score
        m_score = calc_mscore(balance, income, cashflow)
        
        return {
            "roe": round(roe, 2),
            "gross_margin": round(gross_margin, 2),
            "revenue_growth": round(revenue_growth, 2),
            "debt_ratio": round(debt_ratio, 2),
            "cash_ratio": round(cash_ratio, 2),
            "inventory_turnover": round(inventory_turnover, 2),
            "receivable_ratio": round(receivable_ratio, 2),
            "m_score": round(m_score, 2),
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
    inv_turnovers = [d["inventory_turnover"] for d in stocks_data if d.get("inventory_turnover")]
    receivable_ratios = [d["receivable_ratio"] for d in stocks_data if d.get("receivable_ratio")]
    m_scores = [d["m_score"] for d in stocks_data if d.get("m_score")]
    
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
        "inventory_turnover": round(avg(inv_turnovers), 2) if inv_turnovers else 0,
        "receivable_ratio": round(avg(receivable_ratios), 2) if receivable_ratios else 0,
        "m_score": round(avg(m_scores), 2) if m_scores else 0,
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
        
        if len(stocks) < 1:
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
