package analyzer

import (
	"fmt"
	"math"
)

// step8RiskAnalysis 计算 A 股适配综合风险评分 A-Score（0-100，越高越危险）
// 基于 Beneish M-Score + Altman Z-Score + 现金流偏离度 + 应收账款异常 + 毛利率波动
func step8RiskAnalysis(data *FinancialData) StepResult {
	result := StepResult{
		StepNum:    8,
		StepName:   "A股综合风险分析（A-Score）",
		YearlyData: make(map[string]map[string]any),
		Pass:       make(map[string]bool),
	}

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
		curCurrentLiability := data.GetValueOrZero(data.BalanceSheet, "流动负债合计", curYear)
		curEquity := data.GetValueOrZero(data.BalanceSheet, "所有者权益合计", curYear)
		curNetProfit := data.GetValueOrZero(data.IncomeStatement, "净利润", curYear)
		curFinanceCost := data.GetValueOrZero(data.IncomeStatement, "财务费用", curYear)
		curSalesCash := data.GetValueOrZero(data.CashFlow, "销售商品、提供劳务收到的现金", curYear)
		curRetained := data.GetValueOrZero(data.BalanceSheet, "未分配利润", curYear) +
			data.GetValueOrZero(data.BalanceSheet, "盈余公积", curYear)
		if curRetained == 0 {
			curRetained = curEquity * 0.6 // fallback 估算
		}

		// 1. M-Score 子指标
		dsri := 1.0
		prevDSR := safeDiv(prevAR, prevRev)
		curDSR := safeDiv(curAR, curRev)
		if prevDSR > 0 {
			dsri = curDSR / prevDSR
		}

		prevGM := safeGrossMargin(prevRev, data.GetValueOrZero(data.IncomeStatement, "营业成本", prevYear))
		curGM := safeGrossMargin(curRev, data.GetValueOrZero(data.IncomeStatement, "营业成本", curYear))
		gmi := safeDiv(prevGM, curGM)

		prevAQ := 1.0 - safeDiv(prevCurrentAsset+prevFixed, prevAsset)
		curAQ := 1.0 - safeDiv(curCurrentAsset+curFixed, curAsset)
		aqi := safeDiv(prevAQ, curAQ)

		sgi := safeDiv(curRev, prevRev)

		prevDepRate := safeDepreciationRate(prevDepreciation, prevFixed)
		curDepRate := safeDepreciationRate(curDepreciation, curFixed)
		depi := safeDiv(prevDepRate, curDepRate)

		prevSGA := safeDiv(prevSales+prevAdmin, prevRev)
		curSGA := safeDiv(curSales+curAdmin, curRev)
		sgai := safeDiv(prevSGA, curSGA)

		tata := safeDiv(curOpProfit-curOCF, curAsset)

		prevLev := safeDiv(prevLiability, prevAsset)
		curLev := safeDiv(curLiability, curAsset)
		lvgi := safeDiv(curLev, prevLev)

		mscore := -4.84 +
			0.92*dsri +
			0.528*gmi +
			0.404*aqi +
			0.892*sgi +
			0.115*depi -
			0.172*sgai +
			4.679*tata -
			0.327*lvgi

		// 2. Z-Score（A股适配简化版，无市值时用账面权益替代）
		x1 := safeDiv(curCurrentAsset-curCurrentLiability, curAsset)
		x2 := safeDiv(curRetained, curAsset)
		x3 := safeDiv(curOpProfit+curFinanceCost, curAsset)
		x4 := safeDiv(curEquity, curLiability)
		x5 := safeDiv(curRev, curAsset)
		zscore := 1.2*x1 + 1.4*x2 + 3.3*x3 + 0.6*x4 + 1.0*x5

		// 3. 现金流偏离度（0-100）
		cashDev := 0.0
		if curNetProfit != 0 {
			cashDev = math.Max(0, 1.0-curOCF/curNetProfit) * 50.0
		}
		if curRev != 0 && curSalesCash/curRev < 0.8 {
			cashDev += 15.0
		}
		if cashDev > 100.0 {
			cashDev = 100.0
		}

		// 4. 应收账款异常度（0-100）
		arGrowth := safeDiv(curAR-prevAR, prevAR)
		revGrowth := safeDiv(curRev-prevRev, prevRev)
		arRisk := 0.0
		if dsri > 1.2 && arGrowth > revGrowth {
			arRisk = 100.0
		} else if dsri > 1.0 {
			arRisk = 50.0
		}

		// 5. 毛利率异常波动（0-100）
		gmRisk := 0.0
		if gmi > 1.0 && curGM < prevGM {
			gmRisk = 100.0
		} else if gmi > 1.0 {
			gmRisk = 50.0
		}

		// 6. 各子项风险分映射到 0-100
		mRisk := mapMScoreToRisk(mscore)
		zRisk := mapZScoreToRisk(zscore)

		// 7. 非财务爬虫风险分（0-100），缺失时以中性 50 分填充
		crawlerRisk := 50.0
		crawlerParts := 0
		if data.Extras != nil {
			if pr, ok := data.Extras["pledge_ratio"]; ok {
				crawlerParts++
				if pr >= 30 {
					crawlerRisk += 50.0 // 质押比例极高
				} else if pr >= 15 {
					crawlerRisk += 20.0
				} else if pr > 0 {
					crawlerRisk += 5.0
				}
			}
			if iq, ok := data.Extras["inquiry_count_1y"]; ok {
				crawlerParts++
				if iq >= 2 {
					crawlerRisk += 30.0
				} else if iq >= 1 {
					crawlerRisk += 15.0
				}
			}
			if rc, ok := data.Extras["reduction_count_1y"]; ok {
				crawlerParts++
				if rc >= 3 {
					crawlerRisk += 20.0
				} else if rc >= 1 {
					crawlerRisk += 8.0
				}
			}
			if crawlerParts > 0 {
				// 将累加值标准化到 0-100（基础 50 + 各维度累加，最多约 100）
				if crawlerRisk > 100.0 {
					crawlerRisk = 100.0
				}
			}
		}

		// 8. A-Score 综合
		ascore := mRisk*0.15 + zRisk*0.20 + cashDev*0.20 + arRisk*0.15 + gmRisk*0.10 + crawlerRisk*0.20
		if ascore > 100.0 {
			ascore = 100.0
		}

		result.YearlyData[curYear] = map[string]any{
			"DSRI":     dsri,
			"GMI":      gmi,
			"AQI":      aqi,
			"SGI":      sgi,
			"DEPI":     depi,
			"SGAI":     sgai,
			"TATA":     tata,
			"LVGI":     lvgi,
			"MScore":   mscore,
			"ZScore":   zscore,
			"CashDev":  cashDev,
			"ARRisk":   arRisk,
			"GMRisk":   gmRisk,
			"AScore":   ascore,
			"fraudRisk": ascore >= 60, // 黄灯阈值
		}
		result.Pass[curYear] = ascore < 60
	}

	if len(data.Years) > 0 {
		oldest := data.Years[len(data.Years)-1]
		result.YearlyData[oldest] = map[string]any{"note": "缺少上一年数据，无法计算A-Score"}
		result.Pass[oldest] = true
	}

	result.Conclusion = fmt.Sprintf("A-Score ≥ 60 时存在财务操纵或偿债风险嫌疑；%s 最近一年A-Score=%.1f。",
		data.Symbol, getAScore(result.YearlyData, data.Years))
	return result
}

// mapMScoreToRisk 把原始 M-Score 映射到 0-100 风险分
func mapMScoreToRisk(mscore float64) float64 {
	if mscore > -1.78 {
		return 100.0
	}
	if mscore > -2.22 {
		return 50.0
	}
	return 0.0
}

// mapZScoreToRisk 把 Z-Score 映射到 0-100 风险分（A股适配）
func mapZScoreToRisk(z float64) float64 {
	if z < 1.81 {
		return 100.0
	}
	if z < 2.99 {
		return 40.0
	}
	return 0.0
}

// getAScore 获取最新年度的 A-Score
func getAScore(yearly map[string]map[string]any, years []string) float64 {
	if len(years) == 0 {
		return 0
	}
	latest := years[0]
	if v, ok := yearly[latest]["AScore"].(float64); ok {
		return v
	}
	return 0
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
		return 0.0001
	}
	return v
}

func safeDepreciationRate(depreciation, fixedAsset float64) float64 {
	if fixedAsset <= 0 {
		return 0
	}
	denom := depreciation + fixedAsset
	if denom <= 0 {
		return 0
	}
	return depreciation / denom
}
