#!/usr/bin/env python3
"""
鼎龙股份 (300054) 多期 RIM 估值测试脚本
对比 Excel 计算逻辑 vs 当前程序单阶段公式
"""

import akshare as ak
import math

SYMBOL = "300054"
STOCK_NAME = "鼎龙股份"

# ============================================================
# 1. 数据获取
# ============================================================
print(f"=== {STOCK_NAME} ({SYMBOL}) RIM 估值测试 ===\n")

# 1.1 机构盈利预测 (一致预期 EPS)
forecast_df = ak.stock_profit_forecast_em(symbol="")
dl_forecast = forecast_df[forecast_df["代码"] == SYMBOL].iloc[0]

eps_2024 = float(dl_forecast["2024预测每股收益"])
eps_2025 = float(dl_forecast["2025预测每股收益"])
eps_2026 = float(dl_forecast["2026预测每股收益"])
eps_2027 = float(dl_forecast["2027预测每股收益"])

print("【1. 机构一致预期 EPS】")
print(f"  2024E: {eps_2024:.3f}")
print(f"  2025E: {eps_2025:.3f}")
print(f"  2026E: {eps_2026:.3f}")
print(f"  2027E: {eps_2027:.3f}")

# 1.2 国债收益率 (Rf)
bond_df = ak.bond_zh_us_rate()
rf_row = bond_df.dropna(subset=["中国国债收益率10年"]).iloc[-1]
rf = float(rf_row["中国国债收益率10年"]) / 100
print(f"\n【2. 无风险利率 Rf】")
print(f"  日期: {rf_row['日期']}")
print(f"  10年期国债收益率: {rf*100:.4f}%")

# 1.3 个股基本信息 (股价、股本、市值)
info_df = ak.stock_individual_info_em(symbol=SYMBOL)
info = dict(zip(info_df["item"], info_df["value"]))
price = float(info["最新"])
total_shares = float(info["总股本"])  # 股
market_cap = float(info["总市值"])    # 元

print(f"\n【3. 实时行情】")
print(f"  当前股价: {price:.2f} 元")
print(f"  总股本: {total_shares/1e8:.2f} 亿股")
print(f"  总市值: {market_cap/1e8:.2f} 亿元")

# ============================================================
# 2. 参数设定
# ============================================================
# Excel 中鼎龙股份的参数 (作为测试基准)
BETA = 0.98          # 可从 Yahoo Finance 获取，暂用 Excel 值
RM_RF = 0.0517       # 市场风险溢价，暂用 Excel 值
KE = rf + BETA * RM_RF

# 基期 BPS：优先用财报数据，这里用 Excel 中的 5.48 作为对比基准
# 实际应用中可用 最新股东权益 / 总股本
BPS0 = 5.48

# 预测期设定 (对应 Excel: 2026E ~ 2031E 共 6 年)
# 为对齐 Excel，我们将 2025E 作为第一年 (对应 Excel 的 2026E)
eps_forecast = [eps_2025, eps_2026, eps_2027, eps_2027 * 1.16, eps_2027 * 1.16 * 1.115, eps_2027 * 1.16 * 1.115 * 1.05]
dps_forecast = [0.0, 0.0, 0.0, 0.0, 0.0, 0.0]  # 假设不分红 (与 Excel 一致)

# 永续增长率 (预测期后的 RE 增长率)
G_TERMINAL = 0.05

print(f"\n【4. 模型参数】")
print(f"  Beta: {BETA}")
print(f"  市场风险溢价 (Rm-Rf): {RM_RF*100:.2f}%")
print(f"  资本成本 kE (CAPM): {KE*100:.4f}%")
print(f"  基期 BPS: {BPS0:.2f} 元")
print(f"  永续增长率 g: {G_TERMINAL*100:.1f}%")

# ============================================================
# 3. 多期 RIM 计算 (严格遵循 8 步法)
# ============================================================
print(f"\n【5. 多期 RIM 计算过程】")
print("-" * 80)
print(f"{'年度':>6} | {'EPS':>8} | {'DPS':>8} | {'BPS':>8} | {'RE':>10} | {'折现率':>10} | {'RE现值':>10}")
print("-" * 80)

bps = BPS0
sum_pv_re = 0.0
for i, (eps, dps) in enumerate(zip(eps_forecast, dps_forecast)):
    year = 2025 + i
    re = eps - bps * KE
    discount = math.pow(1 + KE, i + 1)
    pv_re = re / discount
    sum_pv_re += pv_re
    print(f"{year:>6} | {eps:>8.3f} | {dps:>8.3f} | {bps:>8.3f} | {re:>10.4f} | {discount:>10.4f} | {pv_re:>10.4f}")
    bps = bps + eps - dps

print("-" * 80)
print(f"{'RE 现值之和':<20} | {sum_pv_re:>10.4f}")

# 持续价值 CV (基于最后一年 RE)
re_terminal = eps_forecast[-1] - (bps - eps_forecast[-1] + dps_forecast[-1]) * KE
cv = re_terminal * (1 + G_TERMINAL) / (KE - G_TERMINAL)
discount_terminal = math.pow(1 + KE, len(eps_forecast))
pv_cv = cv / discount_terminal

print(f"{'持续价值 CV':<20} | {cv:>10.4f}")
print(f"{'CV 现值':<20} | {pv_cv:>10.4f}")

intrinsic_value = BPS0 + sum_pv_re + pv_cv
upside = (intrinsic_value - price) / price * 100

print("=" * 80)
print(f"多期 RIM 内在价值: {intrinsic_value:.2f} 元/股")
print(f"相对当前股价 {price:.2f} 元: {upside:+.1f}%")
print("=" * 80)

# ============================================================
# 4. 当前程序的单阶段公式计算 (作为对比)
# ============================================================
roe_avg = sum(eps_forecast) / len(eps_forecast) / BPS0 * 100
r_simple = 0.07
g_simple = 0.03
single_stage = BPS0 + BPS0 * (roe_avg/100 - r_simple) / (r_simple - g_simple)
upside_single = (single_stage - price) / price * 100

print(f"\n【6. 当前程序单阶段公式对比】")
print(f"  假设 ROE (平均): {roe_avg:.2f}%")
print(f"  假设 r: {r_simple*100:.1f}%")
print(f"  假设 g: {g_simple*100:.1f}%")
print(f"  单阶段内在价值: {single_stage:.2f} 元/股")
print(f"  相对当前股价 {price:.2f} 元: {upside_single:+.1f}%")

# ============================================================
# 5. 差异分析
# ============================================================
print(f"\n【7. 差异分析】")
print(f"  多期 RIM 结果: {intrinsic_value:.2f} 元 ({upside:+.1f}%)")
print(f"  单阶段公式结果: {single_stage:.2f} 元 ({upside_single:+.1f}%)")
print(f"  两者差距: {abs(intrinsic_value - single_stage):.2f} 元 ({abs(upside - upside_single):.1f} 个百分点)")
print(f"\n  原因: 单阶段公式假设 BPS 永远停留在 {BPS0:.2f} 元，")
print(f"       而多期模型中 BPS 通过留存收益滚存到 {bps:.2f} 元，")
print(f"       且享受了 EPS 从 {eps_forecast[0]:.2f} 增长到 {eps_forecast[-1]:.2f} 的成长溢价。")

# ============================================================
# 6. 与 Excel 的对比
# ============================================================
excel_value = 73.56056682
print(f"\n【8. 与 Excel 结果对比】")
print(f"  Excel 计算值: {excel_value:.2f} 元")
print(f"  本脚本计算值: {intrinsic_value:.2f} 元")
print(f"  误差: {abs(intrinsic_value - excel_value):.2f} 元 ({abs(intrinsic_value - excel_value)/excel_value*100:.1f}%)")
if abs(intrinsic_value - excel_value) < 1.0:
    print("  ✅ 结果与 Excel 高度一致")
else:
    print("  ⚠️  存在偏差，可能因 EPS 预测数据版本不同导致")
