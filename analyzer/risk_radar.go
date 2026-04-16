package analyzer

import (
	"fmt"
	"math"
)

// RiskRadarItem 单条风险雷达项
type RiskRadarItem struct {
	Name    string `json:"name"`
	Level   string `json:"level"`   // high / medium / low
	Status  string `json:"status"`  // 异常 / 警告 / 正常
	Message string `json:"message"`
	Icon    string `json:"icon"`    // 🔴 / 🟡 / 🟢
}

// BuildRiskRadar 从18步分析结果中提取最近一年的关键风险信号，并与行业均值对比
func BuildRiskRadar(steps []StepResult, extras map[string]float64, years []string, industry string) []RiskRadarItem {
	if len(years) == 0 {
		return nil
	}
	latest := years[0]
	var prev string
	if len(years) > 1 {
		prev = years[1]
	}

	// 获取行业均值（如果可用）
	ind, _ := GetIndustryMetrics(industry)

	var items []RiskRadarItem

	// Helper: 从 steps 中按 stepNum 查找
	findStep := func(num int) *StepResult {
		for i := range steps {
			if steps[i].StepNum == num {
				return &steps[i]
			}
		}
		return nil
	}

	// Helper: 读取某一步某年份的 float 值
	getFloat := func(step *StepResult, year, key string) float64 {
		if step == nil || step.YearlyData == nil {
			return 0
		}
		yd, ok := step.YearlyData[year]
		if !ok {
			return 0
		}
		v, ok := yd[key]
		if !ok {
			return 0
		}
		if vf, ok2 := v.(float64); ok2 {
			return vf
		}
		return 0
	}

	// Helper: 安全读取行业指标字段
	getIndVal := func(key string) float64 {
		if ind == nil {
			return math.NaN()
		}
		switch key {
		case "roe":
			return ind.ROE
		case "cashRatio":
			return ind.CashRatio
		case "debtRatio":
			return ind.DebtRatio
		case "mScore":
			return ind.MScore
		}
		return math.NaN()
	}

	// Helper: 格式化行业均值后缀
	formatIndustry := func(val float64, unit string) string {
		if math.IsNaN(val) {
			return ""
		}
		return fmt.Sprintf("(行业均值 %.1f%s)", val, unit)
	}

	// 1. 应收账款异常 (step5)
	if s := findStep(5); s != nil {
		receivableRatio := getFloat(s, latest, "receivableRatio")
		receivableYoY := getFloat(s, latest, "receivableYoY")
		if receivableRatio > 20 || receivableYoY > 30 {
			items = append(items, RiskRadarItem{
				Name:    "应收账款占比",
				Level:   "high",
				Status:  "异常",
				Message: fmt.Sprintf("%.1f%%(同比+%.1f%%)", receivableRatio, receivableYoY),
				Icon:    "🔴",
			})
		} else if receivableRatio > 15 || receivableYoY > 20 {
			items = append(items, RiskRadarItem{
				Name:    "应收账款占比",
				Level:   "medium",
				Status:  "警告",
				Message: fmt.Sprintf("%.1f%%(同比+%.1f%%)", receivableRatio, receivableYoY),
				Icon:    "🟡",
			})
		} else {
			items = append(items, RiskRadarItem{
				Name:    "应收账款占比",
				Level:   "low",
				Status:  "正常",
				Message: fmt.Sprintf("%.1f%%", receivableRatio),
				Icon:    "🟢",
			})
		}
	}

	// 2. 存货周转 (step11)
	if s := findStep(11); s != nil {
		turnover := getFloat(s, latest, "inventoryTurnover")
		prevTurnover := getFloat(s, prev, "inventoryTurnover")
		if prev != "" && prevTurnover > 0 && turnover < prevTurnover*0.9 {
			items = append(items, RiskRadarItem{
				Name:    "存货周转率",
				Level:   "medium",
				Status:  "警告",
				Message: fmt.Sprintf("%.2f次(同比%.1f%%)", turnover, (1-turnover/prevTurnover)*100),
				Icon:    "🟡",
			})
		} else {
			items = append(items, RiskRadarItem{
				Name:    "存货周转率",
				Level:   "low",
				Status:  "正常",
				Message: fmt.Sprintf("%.2f次", turnover),
				Icon:    "🟢",
			})
		}
	}

	// 3. 现金流质量 (step15)
	if s := findStep(15); s != nil {
		cashContent := getFloat(s, latest, "cashContent")
		prevCash := getFloat(s, prev, "cashContent")
		msg := fmt.Sprintf("%.1f%% %s", cashContent, formatIndustry(getIndVal("cashRatio"), "%"))
		if cashContent < 100 {
			items = append(items, RiskRadarItem{
				Name:    "净利润现金含量",
				Level:   "medium",
				Status:  "警告",
				Message: msg,
				Icon:    "🟡",
			})
		} else if prev != "" && prevCash > 0 && cashContent < prevCash*0.9 {
			items = append(items, RiskRadarItem{
				Name:    "净利润现金含量",
				Level:   "medium",
				Status:  "警告",
				Message: fmt.Sprintf("%.1f%%(上期%.1f%%) %s", cashContent, prevCash, formatIndustry(getIndVal("cashRatio"), "%")),
				Icon:    "🟡",
			})
		} else {
			items = append(items, RiskRadarItem{
				Name:    "净利润现金含量",
				Level:   "low",
				Status:  "正常",
				Message: msg,
				Icon:    "🟢",
			})
		}
	}

	// 4. ROE (step16)
	if s := findStep(16); s != nil {
		roe := getFloat(s, latest, "roe")
		prevRoe := getFloat(s, prev, "roe")
		indRoe := getIndVal("roe")
		msg := fmt.Sprintf("%.1f%% %s", roe, formatIndustry(indRoe, "%"))
		level := "low"
		if roe < 10 {
			level = "medium"
		} else if prev != "" && prevRoe > 0 && roe < prevRoe*0.85 {
			level = "medium"
		} else if !math.IsNaN(indRoe) && roe < indRoe*0.7 {
			level = "medium"
		}
		if level == "medium" {
			var detail string
			if roe < 10 {
				detail = "(低于10%)"
			} else if prev != "" && prevRoe > 0 && roe < prevRoe*0.85 {
				detail = fmt.Sprintf("(上期%.1f%%)", prevRoe)
			} else {
				detail = "(低于行业均值)"
			}
			items = append(items, RiskRadarItem{
				Name:    "ROE",
				Level:   level,
				Status:  "警告",
				Message: fmt.Sprintf("%.1f%% %s %s", roe, detail, formatIndustry(indRoe, "%")),
				Icon:    "🟡",
			})
		} else {
			items = append(items, RiskRadarItem{
				Name:    "ROE",
				Level:   level,
				Status:  "正常",
				Message: msg,
				Icon:    "🟢",
			})
		}
	}

	// 5. 负债率 (step3)
	if s := findStep(3); s != nil {
		debtRatio := getFloat(s, latest, "debtRatio")
		cashDebtDiff := getFloat(s, latest, "cashDebtDiff")
		msg := fmt.Sprintf("%.1f%% %s", debtRatio, formatIndustry(getIndVal("debtRatio"), "%"))
		if debtRatio > 60 || cashDebtDiff < 0 {
			items = append(items, RiskRadarItem{
				Name:    "资产负债率",
				Level:   "high",
				Status:  "异常",
				Message: fmt.Sprintf("%.1f%%(现金负债缺口) %s", debtRatio, formatIndustry(getIndVal("debtRatio"), "%")),
				Icon:    "🔴",
			})
		} else if debtRatio > 50 {
			items = append(items, RiskRadarItem{
				Name:    "资产负债率",
				Level:   "medium",
				Status:  "警告",
				Message: msg,
				Icon:    "🟡",
			})
		} else {
			items = append(items, RiskRadarItem{
				Name:    "资产负债率",
				Level:   "low",
				Status:  "正常",
				Message: msg,
				Icon:    "🟢",
			})
		}
	}

	// 6. A-Score 风险 (step8)
	if s := findStep(8); s != nil {
		ascore := getFloat(s, latest, "AScore")
		msg := fmt.Sprintf("%.0f分 %s", ascore, formatIndustry(getIndVal("mScore"), "分"))
		if ascore >= 60 {
			items = append(items, RiskRadarItem{
				Name:    "A-Score风险",
				Level:   "high",
				Status:  "异常",
				Message: fmt.Sprintf("%.0f分(高风险) %s", ascore, formatIndustry(getIndVal("mScore"), "分")),
				Icon:    "🔴",
			})
		} else if ascore >= 40 {
			items = append(items, RiskRadarItem{
				Name:    "A-Score风险",
				Level:   "medium",
				Status:  "警告",
				Message: msg,
				Icon:    "🟡",
			})
		} else {
			items = append(items, RiskRadarItem{
				Name:    "A-Score风险",
				Level:   "low",
				Status:  "正常",
				Message: msg,
				Icon:    "🟢",
			})
		}
	}

	// 7. 非财务风险 (extras)
	if len(extras) > 0 {
		pledgeRatio := extras["pledgeRatio"]
		enquiryCount := extras["enquiryCount"]
		reductionCount := extras["reductionCount"]
		var parts []string
		if pledgeRatio > 30 {
			parts = append(parts, fmt.Sprintf("质押%.0f%%", pledgeRatio))
		}
		if enquiryCount > 0 {
			parts = append(parts, fmt.Sprintf("问询%.0f次", enquiryCount))
		}
		if reductionCount > 0 {
			parts = append(parts, fmt.Sprintf("减持%.0f次", reductionCount))
		}
		if len(parts) > 0 {
			msg := parts[0]
			for i := 1; i < len(parts); i++ {
				msg += "/" + parts[i]
			}
			items = append(items, RiskRadarItem{
				Name:    "非财务风险",
				Level:   "medium",
				Status:  "警告",
				Message: msg,
				Icon:    "🟡",
			})
		} else {
			items = append(items, RiskRadarItem{
				Name:    "非财务风险",
				Level:   "low",
				Status:  "正常",
				Message: "无异常",
				Icon:    "🟢",
			})
		}
	}

	return items
}
