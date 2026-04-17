package analyzer

import (
	"fmt"
	"math"
)

// RiskRadarItem 单条风险雷达项
type RiskRadarItem struct {
	Name     string `json:"name"`
	Level    string `json:"level"`   // high / medium / low
	Status   string `json:"status"`  // 异常 / 警告 / 正常
	Message  string `json:"message"`
	Icon     string `json:"icon"`    // 🔴 / 🟡 / 🟢
	Value    string `json:"value"`   // 当前值（如 "18.5%"）
	Industry string `json:"industry"` // 行业均值（如 "7.8%"）
	Desc     string `json:"desc"`    // 指标说明
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
		case "inventoryTurnover":
			return ind.InventoryTurnover
		case "receivableRatio":
			return ind.ReceivableRatio
		}
		return math.NaN()
	}

	// Helper: 格式化行业均值后缀
	formatIndustry := func(val float64, unit string) string {
		if math.IsNaN(val) {
			return ""
		}
		return fmt.Sprintf("%.1f%s", val, unit)
	}

	// Helper: 添加雷达项
	addItem := func(name, level, status, icon, value, indVal, msg, desc string) {
		items = append(items, RiskRadarItem{
			Name:     name,
			Level:    level,
			Status:   status,
			Icon:     icon,
			Value:    value,
			Industry: indVal,
			Message:  msg,
			Desc:     desc,
		})
	}

	// 1. 应收账款异常 (step5) — 与行业均值对比
	if s := findStep(5); s != nil {
		receivableRatio := getFloat(s, latest, "receivableRatio")
		indReceivable := getIndVal("receivableRatio")
		valStr := fmt.Sprintf("%.1f%%", receivableRatio)
		indStr := formatIndustry(indReceivable, "%")
		level := "low"
		if receivableRatio > 20 {
			level = "high"
		} else if receivableRatio > 15 {
			level = "medium"
		} else if !math.IsNaN(indReceivable) && indReceivable > 0 && receivableRatio > indReceivable*1.5 {
			level = "medium"
		}
		if level == "high" {
			addItem("应收账款占比", "high", "异常", "🔴", valStr, indStr, fmt.Sprintf("%.1f%%(高于20%%)", receivableRatio), "占比过高意味着回款压力大，话语权较弱")
		} else if level == "medium" {
			addItem("应收账款占比", "medium", "警告", "🟡", valStr, indStr, valStr, "占比过高意味着回款压力大，话语权较弱")
		} else {
			addItem("应收账款占比", "low", "正常", "🟢", valStr, indStr, valStr, "占比过高意味着回款压力大，话语权较弱")
		}
	}

	// 2. 存货周转 (step11) — 与行业均值对比
	if s := findStep(11); s != nil {
		turnover := getFloat(s, latest, "inventoryTurnover")
		indTurnover := getIndVal("inventoryTurnover")
		valStr := fmt.Sprintf("%.2f次", turnover)
		indStr := formatIndustry(indTurnover, "次")
		level := "low"
		if !math.IsNaN(indTurnover) && indTurnover > 0 && turnover < indTurnover*0.8 {
			level = "medium"
		}
		if level == "medium" {
			addItem("存货周转率", level, "警告", "🟡", valStr, indStr, valStr, "周转越慢可能存在库存积压或减值风险")
		} else {
			addItem("存货周转率", level, "正常", "🟢", valStr, indStr, valStr, "周转越慢可能存在库存积压或减值风险")
		}
	}

	// 3. 现金流质量 (step15)
	if s := findStep(15); s != nil {
		cashContent := getFloat(s, latest, "cashContent")
		prevCash := getFloat(s, prev, "cashContent")
		valStr := fmt.Sprintf("%.1f%%", cashContent)
		indStr := formatIndustry(getIndVal("cashRatio"), "%")
		if cashContent < 100 {
			addItem("净利润现金含量", "medium", "警告", "🟡", valStr, indStr, valStr, "利润中真金白银的比例，低于100%需警惕")
		} else if prev != "" && prevCash > 0 && cashContent < prevCash*0.9 {
			addItem("净利润现金含量", "medium", "警告", "🟡", valStr, indStr, fmt.Sprintf("%.1f%%(上期%.1f%%)", cashContent, prevCash), "利润中真金白银的比例，低于100%需警惕")
		} else {
			addItem("净利润现金含量", "low", "正常", "🟢", valStr, indStr, valStr, "利润中真金白银的比例，低于100%需警惕")
		}
	}

	// 4. ROE (step16)
	if s := findStep(16); s != nil {
		roe := getFloat(s, latest, "roe")
		prevRoe := getFloat(s, prev, "roe")
		indRoe := getIndVal("roe")
		valStr := fmt.Sprintf("%.1f%%", roe)
		indStr := formatIndustry(indRoe, "%")
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
			addItem("ROE", level, "警告", "🟡", valStr, indStr, fmt.Sprintf("%.1f%% %s", roe, detail), "净资产收益率，衡量股东回报能力的核心指标")
		} else {
			addItem("ROE", level, "正常", "🟢", valStr, indStr, valStr, "净资产收益率，衡量股东回报能力的核心指标")
		}
	}

	// 5. 负债率 (step3)
	if s := findStep(3); s != nil {
		debtRatio := getFloat(s, latest, "debtRatio")
		cashDebtDiff := getFloat(s, latest, "cashDebtDiff")
		valStr := fmt.Sprintf("%.1f%%", debtRatio)
		indStr := formatIndustry(getIndVal("debtRatio"), "%")
		if debtRatio > 60 || cashDebtDiff < 0 {
			addItem("资产负债率", "high", "异常", "🔴", valStr, indStr, fmt.Sprintf("%.1f%%(现金负债缺口)", debtRatio), "负债占总资产比例，过高意味着偿债压力大")
		} else if debtRatio > 50 {
			addItem("资产负债率", "medium", "警告", "🟡", valStr, indStr, valStr, "负债占总资产比例，过高意味着偿债压力大")
		} else {
			addItem("资产负债率", "low", "正常", "🟢", valStr, indStr, valStr, "负债占总资产比例，过高意味着偿债压力大")
		}
	}

	// 6. A-Score 风险 (step8)
	if s := findStep(8); s != nil {
		ascore := getFloat(s, latest, "AScore")
		valStr := fmt.Sprintf("%.0f分", ascore)
		indStr := formatIndustry(getIndVal("mScore"), "分")
		if ascore >= 60 {
			addItem("A-Score风险", "high", "异常", "🔴", valStr, indStr, fmt.Sprintf("%.0f分(高风险)", ascore), "综合财务操纵与爆雷风险评分，越低越安全")
		} else if ascore >= 40 {
			addItem("A-Score风险", "medium", "警告", "🟡", valStr, indStr, valStr, "综合财务操纵与爆雷风险评分，越低越安全")
		} else {
			addItem("A-Score风险", "low", "正常", "🟢", valStr, indStr, valStr, "综合财务操纵与爆雷风险评分，越低越安全")
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
			addItem("非财务风险", "medium", "警告", "🟡", msg, "", msg, "股权质押、监管问询、大股东减持等特有风险")
		} else {
			addItem("非财务风险", "low", "正常", "🟢", "无异常", "", "无异常", "股权质押、监管问询、大股东减持等特有风险")
		}
	}

	return items
}
