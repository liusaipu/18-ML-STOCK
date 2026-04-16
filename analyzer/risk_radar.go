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
				Name:    "应收账款",
				Level:   "high",
				Status:  "异常",
				Message: fmt.Sprintf("应收占比 %.1f%%，同比增幅 %.1f%%", receivableRatio, receivableYoY),
				Icon:    "🔴",
			})
		} else if receivableRatio > 15 || receivableYoY > 20 {
			items = append(items, RiskRadarItem{
				Name:    "应收账款",
				Level:   "medium",
				Status:  "警告",
				Message: fmt.Sprintf("应收占比 %.1f%%，同比增幅 %.1f%%", receivableRatio, receivableYoY),
				Icon:    "🟡",
			})
		} else {
			items = append(items, RiskRadarItem{
				Name:    "应收账款",
				Level:   "low",
				Status:  "正常",
				Message: fmt.Sprintf("应收占比 %.1f%%，处于健康水平", receivableRatio),
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
				Name:    "存货周转",
				Level:   "medium",
				Status:  "警告",
				Message: fmt.Sprintf("周转率 %.2f 次，同比下降 %.1f%%", turnover, (1-turnover/prevTurnover)*100),
				Icon:    "🟡",
			})
		} else {
			items = append(items, RiskRadarItem{
				Name:    "存货周转",
				Level:   "low",
				Status:  "正常",
				Message: fmt.Sprintf("周转率 %.2f 次", turnover),
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
				Name:    "现金流质量",
				Level:   "medium",
				Status:  "警告",
				Message: fmt.Sprintf("净利润现金含量 %.1f%%，低于 100%%", cashContent),
				Icon:    "🟡",
			})
		} else if prev != "" && prevCash > 0 && cashContent < prevCash*0.9 {
			items = append(items, RiskRadarItem{
				Name:    "现金流质量",
				Level:   "medium",
				Status:  "警告",
				Message: fmt.Sprintf("净利润现金含量 %.1f%%，较上期 %.1f%% 下降", cashContent, prevCash),
				Icon:    "🟡",
			})
		} else {
			items = append(items, RiskRadarItem{
				Name:    "现金流质量",
				Level:   "low",
				Status:  "正常",
				Message: fmt.Sprintf("净利润现金含量 %.1f%%", cashContent),
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
				Message: fmt.Sprintf("ROE %.1f%%，低于 10%%", roe),
				Icon:    "🟡",
			})
		} else if prev != "" && prevRoe > 0 && roe < prevRoe*0.85 {
			items = append(items, RiskRadarItem{
				Name:    "ROE",
				Level:   "medium",
				Status:  "警告",
				Message: fmt.Sprintf("ROE %.1f%%，较上期 %.1f%% 明显下滑", roe, prevRoe),
				Icon:    "🟡",
			})
		} else {
			items = append(items, RiskRadarItem{
				Name:    "ROE",
				Level:   "low",
				Status:  "正常",
				Message: fmt.Sprintf("ROE %.1f%%", roe),
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
				Name:    "负债水平",
				Level:   "high",
				Status:  "异常",
				Message: fmt.Sprintf("资产负债率 %.1f%%，准货币资金-有息负债 %.2f 亿", debtRatio, cashDebtDiff/1e8),
				Icon:    "🔴",
			})
		} else if debtRatio > 50 {
			items = append(items, RiskRadarItem{
				Name:    "负债水平",
				Level:   "medium",
				Status:  "警告",
				Message: fmt.Sprintf("资产负债率 %.1f%%，建议关注", debtRatio),
				Icon:    "🟡",
			})
		} else {
			items = append(items, RiskRadarItem{
				Name:    "负债水平",
				Level:   "low",
				Status:  "正常",
				Message: fmt.Sprintf("资产负债率 %.1f%%", debtRatio),
				Icon:    "🟢",
			})
		}
	}

	// 6. A-Score 风险 (step8)
	if s := findStep(8); s != nil {
		ascore := getFloat(s, latest, "AScore")
		if ascore >= 60 {
			items = append(items, RiskRadarItem{
				Name:    "A-Score 风险",
				Level:   "high",
				Status:  "异常",
				Message: fmt.Sprintf("A-Score %.0f，存在较高财务或操纵风险", ascore),
				Icon:    "🔴",
			})
		} else if ascore >= 40 {
			items = append(items, RiskRadarItem{
				Name:    "A-Score 风险",
				Level:   "medium",
				Status:  "警告",
				Message: fmt.Sprintf("A-Score %.0f，需持续关注", ascore),
				Icon:    "🟡",
			})
		} else {
			items = append(items, RiskRadarItem{
				Name:    "A-Score 风险",
				Level:   "low",
				Status:  "正常",
				Message: fmt.Sprintf("A-Score %.0f，财务质量良好", ascore),
				Icon:    "🟢",
			})
		}
	}

	// 7. 非财务风险 (extras)
	if len(extras) > 0 {
		pledgeRatio := extras["pledgeRatio"]
		enquiryCount := extras["enquiryCount"]
		reductionCount := extras["reductionCount"]
		hasRisk := false
		msg := ""
		if pledgeRatio > 30 {
			hasRisk = true
			msg += fmt.Sprintf("股权质押率 %.1f%%；", pledgeRatio)
		}
		if enquiryCount > 0 {
			hasRisk = true
			msg += fmt.Sprintf("监管问询函 %.0f 次；", enquiryCount)
		}
		if reductionCount > 0 {
			hasRisk = true
			msg += fmt.Sprintf("大股东减持 %.0f 次；", reductionCount)
		}
		if hasRisk {
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
				Message: "暂无重大非财务风险信号",
				Icon:    "🟢",
			})
		}
	}

	return items
}
