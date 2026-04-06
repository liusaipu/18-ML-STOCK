package analyzer

import (
	"fmt"
	"math"
)

// step8MScore 计算Beneish M-Score及其8个子指标
func step8MScore(data *FinancialData) StepResult {
	result := StepResult{
		StepNum:    8,
		StepName:   "财务造假风险分析（Beneish M-Score）",
		YearlyData: make(map[string]map[string]any),
		Pass:       make(map[string]bool),
	}

	// 计算每年的 M-Score（需要上一年数据，因此从第二年/倒数第二年开始）
	for i := 0; i < len(data.Years)-1; i++ {
		curYear := data.Years[i]
		prevYear := data.Years[i+1]

		// --- 基础数据提取 ---
		curRev := data.GetValueOrZero(data.IncomeStatement, "营业收入", curYear)
		prevRev := data.GetValueOrZero(data.IncomeStatement, "营业收入", prevYear)
		curAR := data.GetValueOrZero(data.BalanceSheet, "应收票据及应收账款", curYear)
		prevAR := data.GetValueOrZero(data.BalanceSheet, "应收票据及应收账款", prevYear)
		curAsset := data.GetValueOrZero(data.BalanceSheet, "资产合计", curYear)
		prevAsset := data.GetValueOrZero(data.BalanceSheet, "资产合计", prevYear)
		curLiability := data.GetValueOrZero(data.BalanceSheet, "负债合计", curYear)
		prevLiability := data.GetValueOrZero(data.BalanceSheet, "负债合计", prevYear)
		curCurrentAsset := data.GetValueOrZero(data.BalanceSheet, "流动资产合计", curYear)
		prevCurrentAsset := data.GetValueOrZero(data.BalanceSheet, "流动资产合计", prevYear)
		curFixed := data.GetValueOrZero(data.BalanceSheet, "固定资产", curYear)
		prevFixed := data.GetValueOrZero(data.BalanceSheet, "固定资产", prevYear)
		curOpProfit := data.GetValueOrZero(data.IncomeStatement, "营业利润", curYear)
		curOCF := data.GetValueOrZero(data.CashFlow, "经营活动现金流量净额", curYear)
		curSales := data.GetValueOrZero(data.IncomeStatement, "销售费用", curYear)
		prevSales := data.GetValueOrZero(data.IncomeStatement, "销售费用", prevYear)
		curAdmin := data.GetValueOrZero(data.IncomeStatement, "管理费用", curYear)
		prevAdmin := data.GetValueOrZero(data.IncomeStatement, "管理费用", prevYear)
		curDepreciation := data.GetValueOrZero(data.CashFlow, "固定资产折旧、油气资产折耗、生产性生物资产折旧", curYear)
		prevDepreciation := data.GetValueOrZero(data.CashFlow, "固定资产折旧、油气资产折耗、生产性生物资产折旧", prevYear)

		// 1. DSRI
		dsri := 1.0
		prevDSR := safeDiv(prevAR, prevRev)
		curDSR := safeDiv(curAR, curRev)
		if prevDSR > 0 {
			dsri = curDSR / prevDSR
		}

		// 2. GMI
		prevGM := safeGrossMargin(prevRev, data.GetValueOrZero(data.IncomeStatement, "营业成本", prevYear))
		curGM := safeGrossMargin(curRev, data.GetValueOrZero(data.IncomeStatement, "营业成本", curYear))
		gmi := safeDiv(prevGM, curGM)

		// 3. AQI
		prevAQ := 1.0 - safeDiv(prevCurrentAsset+prevFixed, prevAsset)
		curAQ := 1.0 - safeDiv(curCurrentAsset+curFixed, curAsset)
		aqi := safeDiv(prevAQ, curAQ)

		// 4. SGI
		sgi := safeDiv(curRev, prevRev)

		// 5. DEPI
		prevDepRate := safeDepreciationRate(prevDepreciation, prevFixed)
		curDepRate := safeDepreciationRate(curDepreciation, curFixed)
		depi := safeDiv(prevDepRate, curDepRate)

		// 6. SGAI
		prevSGA := safeDiv(prevSales+prevAdmin, prevRev)
		curSGA := safeDiv(curSales+curAdmin, curRev)
		sgai := safeDiv(prevSGA, curSGA)

		// 7. TATA
		tata := safeDiv(curOpProfit-curOCF, curAsset)

		// 8. LVGI
		prevLev := safeDiv(prevLiability, prevAsset)
		curLev := safeDiv(curLiability, curAsset)
		lvgi := safeDiv(curLev, prevLev)

		// M-Score
		mscore := -4.84 +
			0.92*dsri +
			0.528*gmi +
			0.404*aqi +
			0.892*sgi +
			0.115*depi -
			0.172*sgai +
			4.679*tata -
			0.327*lvgi

		result.YearlyData[curYear] = map[string]any{
			"DSRI":    dsri,
			"GMI":     gmi,
			"AQI":     aqi,
			"SGI":     sgi,
			"DEPI":    depi,
			"SGAI":    sgai,
			"TATA":    tata,
			"LVGI":    lvgi,
			"MScore":  mscore,
			"fraudRisk": mscore > -2.22,
		}
		result.Pass[curYear] = mscore <= -2.22
	}

	// 最早一年（最后一年）没有上一年数据，无法计算 M-Score，标记为跳过
	if len(data.Years) > 0 {
		oldest := data.Years[len(data.Years)-1]
		result.YearlyData[oldest] = map[string]any{"note": "缺少上一年数据，无法计算M-Score"}
		result.Pass[oldest] = true
	}

	result.Conclusion = fmt.Sprintf("M-Score > -2.22 时存在财务操纵嫌疑；%s 最近一年M-Score=%.3f。",
		data.Symbol, getMScore(result.YearlyData, data.Years))
	return result
}

func safeDiv(a, b float64) float64 {
	if b == 0 || math.IsNaN(b) || math.IsInf(b, 0) {
		return 0
	}
	v := a / b
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return v
}

func safeGrossMargin(revenue, cost float64) float64 {
	if revenue == 0 {
		return 0
	}
	v := (revenue - cost) / revenue
	if v <= 0 {
		return 0.0001 // 避免除零或负毛利导致GMI异常
	}
	return v
}

func safeDepreciationRate(depreciation, fixedAsset float64) float64 {
	if fixedAsset <= 0 {
		return 0
	}
	// 折旧率 = 折旧费用 / (折旧费用 + 固定资产净值)
	// 这里简化为 折旧费用 / (折旧费用 + 固定资产)
	denom := depreciation + fixedAsset
	if denom <= 0 {
		return 0
	}
	return depreciation / denom
}

func getMScore(yearly map[string]map[string]any, years []string) float64 {
	if len(years) == 0 {
		return 0
	}
	latest := years[0]
	if v, ok := yearly[latest]["MScore"].(float64); ok {
		return v
	}
	return 0
}
