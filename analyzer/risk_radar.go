package analyzer

import "fmt"

// RiskRadarItem 单条风险雷达项
type RiskRadarItem struct {
	Name    string `json:"name"`
	Level   string `json:"level"`   // high / medium / low
	Status  string `json:"status"`  // 异常 / 警告 / 正常
	Message string `json:"message"`
	Icon    string `json:"icon"`    // 🔴 / 🟡 / 🟢
}

// BuildRiskRadar 从18步分析结果中提取最近一年的关键风险信号
func BuildRiskRadar(steps []StepResult, extras map[string]float64, years []string) []RiskRadarItem {
	if len(years) == 0 {
		return nil
	}
	latest := years[0]
	var prev string
	if len(years) > 1 {
		prev = years[1]
	}

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
		if cashContent < 100 {
			items = append(items, RiskRadarItem{
				Name:    "净利润现金含量",
				Level:   "medium",
				Status:  "警告",
				Message: fmt.Sprintf("%.1f%%(低于100%%)", cashContent),
				Icon:    "🟡",
			})
		} else if prev != "" && prevCash > 0 && cashContent < prevCash*0.9 {
			items = append(items, RiskRadarItem{
				Name:    "净利润现金含量",
				Level:   "medium",
				Status:  "警告",
				Message: fmt.Sprintf("%.1f%%(上期%.1f%%)", cashContent, prevCash),
				Icon:    "🟡",
			})
		} else {
			items = append(items, RiskRadarItem{
				Name:    "净利润现金含量",
				Level:   "low",
				Status:  "正常",
				Message: fmt.Sprintf("%.1f%%", cashContent),
				Icon:    "🟢",
			})
		}
	}

	// 4. ROE (step16)
	if s := findStep(16); s != nil {
		roe := getFloat(s, latest, "roe")
		prevRoe := getFloat(s, prev, "roe")
		if roe < 10 {
			items = append(items, RiskRadarItem{
				Name:    "ROE",
				Level:   "medium",
				Status:  "警告",
				Message: fmt.Sprintf("%.1f%%(低于10%%)", roe),
				Icon:    "🟡",
			})
		} else if prev != "" && prevRoe > 0 && roe < prevRoe*0.85 {
			items = append(items, RiskRadarItem{
				Name:    "ROE",
				Level:   "medium",
				Status:  "警告",
				Message: fmt.Sprintf("%.1f%%(上期%.1f%%)", roe, prevRoe),
				Icon:    "🟡",
			})
		} else {
			items = append(items, RiskRadarItem{
				Name:    "ROE",
				Level:   "low",
				Status:  "正常",
				Message: fmt.Sprintf("%.1f%%", roe),
				Icon:    "🟢",
			})
		}
	}

	// 5. 负债率 (step3)
	if s := findStep(3); s != nil {
		debtRatio := getFloat(s, latest, "debtRatio")
		cashDebtDiff := getFloat(s, latest, "cashDebtDiff")
		if debtRatio > 60 || cashDebtDiff < 0 {
			items = append(items, RiskRadarItem{
				Name:    "资产负债率",
				Level:   "high",
				Status:  "异常",
				Message: fmt.Sprintf("%.1f%%(现金负债缺口)", debtRatio),
				Icon:    "🔴",
			})
		} else if debtRatio > 50 {
			items = append(items, RiskRadarItem{
				Name:    "资产负债率",
				Level:   "medium",
				Status:  "警告",
				Message: fmt.Sprintf("%.1f%%", debtRatio),
				Icon:    "🟡",
			})
		} else {
			items = append(items, RiskRadarItem{
				Name:    "资产负债率",
				Level:   "low",
				Status:  "正常",
				Message: fmt.Sprintf("%.1f%%", debtRatio),
				Icon:    "🟢",
			})
		}
	}

	// 6. A-Score 风险 (step8)
	if s := findStep(8); s != nil {
		ascore := getFloat(s, latest, "AScore")
		if ascore >= 60 {
			items = append(items, RiskRadarItem{
				Name:    "A-Score风险",
				Level:   "high",
				Status:  "异常",
				Message: fmt.Sprintf("%.0f分(高风险)", ascore),
				Icon:    "🔴",
			})
		} else if ascore >= 40 {
			items = append(items, RiskRadarItem{
				Name:    "A-Score风险",
				Level:   "medium",
				Status:  "警告",
				Message: fmt.Sprintf("%.0f分", ascore),
				Icon:    "🟡",
			})
		} else {
			items = append(items, RiskRadarItem{
				Name:    "A-Score风险",
				Level:   "low",
				Status:  "正常",
				Message: fmt.Sprintf("%.0f分", ascore),
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
