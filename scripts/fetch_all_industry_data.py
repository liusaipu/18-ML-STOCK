#!/usr/bin/env python3
"""
后台全市场行业数据采集脚本（fallback 数据源）
使用 akshare 的 stock_yjbb_em 接口获取 A 股业绩快报（年报/三季报）
仅提取：ROE、毛利率、营收增长率
输出: industry_database_fallback.json
进度: industry_task.json
"""

import sys
import os
import json
import time
import math
from datetime import datetime

# 尝试导入 akshare
try:
    import akshare as ak
except ImportError:
    print(json.dumps({"success": False, "error": "缺少 akshare 依赖"}, ensure_ascii=False))
    sys.exit(1)


def write_task(data_dir: str, status: str, progress: int, total: int, message: str, started_at: str = None):
    """写入任务进度文件"""
    task_path = os.path.join(data_dir, "industry_task.json")
    now = datetime.now().strftime("%Y-%m-%dT%H:%M:%S")
    task = {
        "status": status,
        "progress": progress,
        "total": total,
        "message": message,
        "startedAt": started_at or now,
        "updatedAt": now,
    }
    try:
        with open(task_path, "w", encoding="utf-8") as f:
            json.dump(task, f, ensure_ascii=False, indent=2)
    except Exception as e:
        print(f"写入任务进度失败: {e}", file=sys.stderr)


def safe_float(val) -> float:
    if val is None or (isinstance(val, float) and math.isnan(val)):
        return 0.0
    try:
        return float(val)
    except (ValueError, TypeError):
        return 0.0


def get_latest_report_date() -> str:
    """获取最近可用的业绩快报季度末日期"""
    now = datetime.now()
    year = now.year
    month = now.month
    # 业绩快报发布时间：年报(4月底)、一季报(4月底)、半年报(8月底)、三季报(10月底)
    # 优先尝试年报(上一年12/31)，如果当前在5月之前，可能最新的是三季报(9/30)
    if month >= 5:
        return f"{year - 1}1231"
    elif month >= 11:
        return f"{year}0930"
    elif month >= 9:
        return f"{year}0630"
    else:
        return f"{year - 1}1231"


def fetch_data_for_date(date: str, data_dir: str, started_at: str):
    """获取指定日期的业绩快报数据"""
    write_task(data_dir, "running", 0, 0, f"正在获取 {date} 业绩快报...", started_at)
    try:
        df = ak.stock_yjbb_em(date=date)
    except Exception as e:
        print(f"{date} 获取失败: {e}", file=sys.stderr)
        return None
    if df is None or df.empty:
        print(f"{date} 返回为空", file=sys.stderr)
        return None

    total = len(df)
    write_task(data_dir, "running", total, total, f"已获取 {total} 只股票，正在计算行业均值...", started_at)
    print(f"{date} 获取成功，共 {total} 条", file=sys.stderr)
    return df


def calculate_industry_metrics(stocks_data: list) -> dict:
    """计算行业均值"""
    if not stocks_data:
        return None

    def avg(vals):
        clean = [v for v in vals if v != 0]
        return round(sum(clean) / len(clean), 2) if clean else 0.0

    roes = [d["roe"] for d in stocks_data]
    gms = [d["gross_margin"] for d in stocks_data]
    growths = [d["revenue_growth"] for d in stocks_data]

    return {
        "count": len(stocks_data),
        "roe": avg(roes),
        "gross_margin": avg(gms),
        "revenue_growth": avg(growths),
        "updated_at": datetime.now().strftime("%Y-%m-%dT%H:%M:%S"),
    }


def main():
    if len(sys.argv) < 2:
        print(json.dumps({"success": False, "error": "Usage: fetch_all_industry_data.py <data_dir>"}, ensure_ascii=False))
        sys.exit(1)

    data_dir = sys.argv[1]
    os.makedirs(data_dir, exist_ok=True)

    started_at = datetime.now().strftime("%Y-%m-%dT%H:%M:%S")
    write_task(data_dir, "running", 0, 0, "正在初始化...", started_at)

    print("开始获取全市场 A 股业绩快报数据...", file=sys.stderr)

    # 尝试多个日期（最新的可能还没有数据）
    dates_to_try = []
    base_date = get_latest_report_date()
    dates_to_try.append(base_date)
    # fallback 到上一个季度
    year = int(base_date[:4])
    if base_date.endswith("1231"):
        dates_to_try.append(f"{year}0930")
    elif base_date.endswith("0930"):
        dates_to_try.append(f"{year}0630")
    elif base_date.endswith("0630"):
        dates_to_try.append(f"{year}0331")

    df = None
    used_date = None
    for date in dates_to_try:
        df = fetch_data_for_date(date, data_dir, started_at)
        if df is not None and not df.empty:
            used_date = date
            break
        time.sleep(1)

    if df is None or df.empty:
        write_task(data_dir, "error", 0, 0, "未获取到任何数据", started_at)
        print(json.dumps({"success": False, "error": "未获取到任何数据"}, ensure_ascii=False))
        sys.exit(1)

    # 提取指标
    industry_groups = {}
    for _, row in df.iterrows():
        industry = str(row.get("所处行业", "")).strip()
        if not industry or industry.lower() == "nan":
            continue
        metrics = {
            "roe": safe_float(row.get("净资产收益率")),
            "gross_margin": safe_float(row.get("销售毛利率")),
            "revenue_growth": safe_float(row.get("营业总收入-同比增长")),
        }
        if industry not in industry_groups:
            industry_groups[industry] = []
        industry_groups[industry].append(metrics)

    # 计算行业均值
    db = {
        "version": "1.0",
        "updated_at": datetime.now().strftime("%Y-%m-%dT%H:%M:%S"),
        "source": "akshare.stock_yjbb_em",
        "date": used_date,
        "total_stocks": len(df),
        "industries": {},
    }

    for industry, stocks in industry_groups.items():
        metrics = calculate_industry_metrics(stocks)
        if metrics:
            db["industries"][industry] = {
                "industry": industry,
                **metrics,
            }

    fallback_path = os.path.join(data_dir, "industry_database_fallback.json")
    with open(fallback_path, "w", encoding="utf-8") as f:
        json.dump(db, f, ensure_ascii=False, indent=2)

    completed_msg = f"完成于 {datetime.now().strftime('%H:%M')}（{used_date} 共 {len(df)} 只 A 股，覆盖 {len(db['industries'])} 个行业）"
    write_task(data_dir, "completed", len(df), len(df), completed_msg, started_at)

    result = {
        "success": True,
        "path": fallback_path,
        "date": used_date,
        "total_stocks": len(df),
        "total_industries": len(db["industries"]),
    }
    print(json.dumps(result, ensure_ascii=False))


if __name__ == "__main__":
    main()
