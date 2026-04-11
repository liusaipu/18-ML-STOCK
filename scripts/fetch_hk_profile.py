#!/usr/bin/env python3
"""
获取港股基本资料
用法: python3 fetch_hk_profile.py <code>
输出 JSON
"""
import json
import sys
import pandas as pd

try:
    import akshare as ak
except ImportError:
    ak = None


def main():
    if len(sys.argv) < 2:
        print(json.dumps({"error": "missing code"}), file=sys.stderr)
        sys.exit(1)

    code = sys.argv[1]
    result = {
        "code": code,
        "industry": "",
        "chairman": "",
        "listing_date": "",
        "exchange": "",
        "sector": "",
    }

    if ak is None:
        print(json.dumps({"error": "akshare not installed"}), file=sys.stderr)
        sys.exit(1)

    errors = []

    try:
        df_sec = ak.stock_hk_security_profile_em(symbol=code)
        if df_sec is not None and not df_sec.empty:
            row = df_sec.iloc[0]
            result["listing_date"] = str(row.get("上市日期", "")).split()[0].replace("-", "")
            result["exchange"] = str(row.get("交易所", ""))
            result["sector"] = str(row.get("板块", ""))
    except Exception as e:
        errors.append(f"security_profile: {e}")

    try:
        df_comp = ak.stock_hk_company_profile_em(symbol=code)
        if df_comp is not None and not df_comp.empty:
            row = df_comp.iloc[0]
            result["industry"] = str(row.get("所属行业", ""))
            result["chairman"] = str(row.get("董事长", ""))
            if not result["listing_date"]:
                # fallback to company establishment date
                founded = str(row.get("公司成立日期", ""))
                if founded and founded != "nan":
                    result["listing_date"] = founded.replace("-", "")
    except Exception as e:
        errors.append(f"company_profile: {e}")

    result["errors"] = errors
    print(json.dumps(result, ensure_ascii=False))


if __name__ == "__main__":
    main()
