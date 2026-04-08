#!/usr/bin/env python3
"""
Fetch RIM external data via akshare for A-share stocks.
Input: {"symbol": "300054"} via stdin
Output: JSON with eps_forecast, rf, beta_fallback, etc.
"""
import sys
import json
import math

try:
    import akshare as ak
except ImportError:
    print(json.dumps({"error": "akshare not installed"}), file=sys.stderr)
    sys.exit(1)


def fetch(symbol: str):
    result = {"symbol": symbol}
    # 1. EPS forecast
    try:
        df = ak.stock_profit_forecast_em(symbol="")
        row = df[df["代码"] == symbol]
        if len(row) == 0:
            result["eps_forecast"] = None
        else:
            r = row.iloc[0]
            forecast = {}
            # Map columns like 2024预测每股收益 -> year
            for col in r.index:
                if "预测每股收益" in col:
                    year = col.replace("预测每股收益", "").strip()
                    try:
                        val = float(r[col])
                        if not math.isnan(val):
                            forecast[year] = val
                    except (ValueError, TypeError):
                        pass
            result["eps_forecast"] = forecast
    except Exception as e:
        result["eps_forecast_error"] = str(e)
        result["eps_forecast"] = None

    # 2. Risk-free rate (China 10-year bond yield) - try fast interfaces first
    rf = 0.0183
    rf_date = ""
    # Fast path: bond_zh_yield single-page interface
    try:
        bond_df = ak.bond_zh_yield(symbol="国债收益率10年", period="日", start_date="", end_date="")
        if len(bond_df) > 0:
            rf = float(bond_df.iloc[-1]["收盘价"]) / 100
            rf_date = str(bond_df.index[-1]) if hasattr(bond_df, "index") else str(bond_df.iloc[-1].get("日期", ""))
    except Exception:
        pass
    # Fallback path
    if rf <= 0:
        try:
            bond_df = ak.bond_zh_us_rate()
            rf_row = bond_df.dropna(subset=["中国国债收益率10年"]).iloc[-1]
            rf = float(rf_row["中国国债收益率10年"]) / 100
            rf_date = str(rf_row["日期"])
        except Exception as e:
            result["rf_error"] = str(e)
    result["rf"] = rf
    result["rf_date"] = rf_date

    # 3. Basic info for shares / price / pb
    try:
        info_df = ak.stock_individual_info_em(symbol=symbol)
        info = dict(zip(info_df["item"], info_df["value"]))
        result["price"] = float(info.get("最新", 0))
        result["total_shares"] = float(info.get("总股本", 0))
        result["market_cap"] = float(info.get("总市值", 0))
    except Exception as e:
        result["info_error"] = str(e)
        result["price"] = 0
        result["total_shares"] = 0
        result["market_cap"] = 0

    # 4. PB from daily indicator (try multiple interfaces)
    pb = 0.0
    try:
        # akshare indicator interface varies by version, try common ones
        df_pb = ak.stock_zh_a_spot_em()
        row_pb = df_pb[df_pb["代码"] == symbol]
        if len(row_pb) > 0:
            pb = float(row_pb.iloc[0].get("市净率", 0))
    except Exception:
        pass
    result["pb"] = pb if pb > 0 else 0

    # 5. Fallback beta and market risk premium
    result["beta"] = 0.98
    result["rm_rf"] = 0.0517

    return result


def main():
    req = json.load(sys.stdin)
    symbol = req.get("symbol", "")
    if not symbol:
        print(json.dumps({"error": "symbol is required"}))
        return
    try:
        resp = fetch(symbol)
        print(json.dumps(resp, ensure_ascii=False, default=str))
    except Exception as e:
        print(json.dumps({"error": str(e)}))


if __name__ == "__main__":
    main()
