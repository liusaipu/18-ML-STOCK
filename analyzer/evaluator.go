package analyzer

import (
	"fmt"
	"math"
)

const baseScorePerStep = 100.0 / 18.0

// Evaluate 根据18步结果，为每一年计算总分和评级
type YearScore struct {
	Year        string
	RawScore    float64
	MaxScore    float64
	Grade       string
	PassCount   int
	FailCount   int
	Deductions  []Deduction
}

type Deduction struct {
	StepNum int
	StepName string
	Reason  string
	Points  float64
}

func Evaluate(data *FinancialData, steps []StepResult) map[string]*YearScore {
	scores := make(map[string]*YearScore)
	for _, year := range data.Years {
		scores[year] = &YearScore{Year: year, MaxScore: 100}
	}

	for _, step := range steps {
		for _, year := range data.Years {
			passed, hasPass := step.Pass[year]
			yd, hasData := step.YearlyData[year]
			if !hasData {
				continue
			}
			score := scores[year]
			if !hasPass {
				// 无pass信息，默认给满分（如审计意见）
				score.RawScore += baseScorePerStep
				score.PassCount++
				continue
			}
			if passed {
				score.RawScore += baseScorePerStep
				score.PassCount++
			} else {
				deduction := computeDeduction(step, year, yd)
				score.RawScore += math.Max(0, baseScorePerStep-deduction.Points)
				score.FailCount++
				score.Deductions = append(score.Deductions, deduction)
			}
		}
	}

	for _, s := range scores {
		s.Grade = gradeFromScore(s.RawScore)
	}
	return scores
}

func computeDeduction(step StepResult, year string, yd map[string]any) Deduction {
	d := Deduction{StepNum: step.StepNum, StepName: step.StepName, Points: 1}
	switch step.StepNum {
	case 2:
		// 资产规模：负增长，越负越严重
		v := anyToFloat64(yd["growthRate"])
		if v < -10 {
			d.Points = 3
			d.Reason = fmt.Sprintf("%s年总资产大幅萎缩 %.2f%%", year, v)
		} else if v < -5 {
			d.Points = 2
			d.Reason = fmt.Sprintf("%s年总资产萎缩 %.2f%%", year, v)
		} else {
			d.Reason = fmt.Sprintf("%s年总资产增长 %.2f%%，未达10%%", year, v)
		}
	case 3:
		debt := anyToFloat64(yd["debtRatio"])
		diff := anyToFloat64(yd["cashDebtDiff"])
		if debt > 70 && diff < 0 {
			d.Points = 3
			d.Reason = fmt.Sprintf("%s年资产负债率%.1f%%且准货币资金-有息负债<0", year, debt)
		} else if debt > 60 {
			d.Points = 2
			d.Reason = fmt.Sprintf("%s年资产负债率%.1f%%超标", year, debt)
		} else {
			d.Points = 1
			d.Reason = fmt.Sprintf("%s年准货币资金-有息负债=%.0f，偿债能力偏弱", year, diff)
		}
	case 4:
		diff := anyToFloat64(yd["diff"])
		if diff < -1e8 {
			d.Points = 3
		} else if diff < -1e7 {
			d.Points = 2
		} else {
			d.Points = 1
		}
		d.Reason = fmt.Sprintf("%s年两头吃差额=%.0f，产业链地位弱", year, diff)
	case 5:
		ratio := anyToFloat64(yd["ratio"])
		if ratio > 20 {
			d.Points = 3
		} else if ratio > 10 {
			d.Points = 2
		} else {
			d.Points = 1
		}
		d.Reason = fmt.Sprintf("%s年应收账款及合同资产占比=%.2f%%", year, ratio)
	case 6:
		ratio := anyToFloat64(yd["ratio"])
		if ratio > 60 {
			d.Points = 3
		} else if ratio > 40 {
			d.Points = 2
		} else {
			d.Points = 1
		}
		d.Reason = fmt.Sprintf("%s年固定资产工程占比=%.2f%%", year, ratio)
	case 7:
		ratio := anyToFloat64(yd["ratio"])
		if ratio > 20 {
			d.Points = 3
		} else if ratio > 10 {
			d.Points = 2
		} else {
			d.Points = 1
		}
		d.Reason = fmt.Sprintf("%s年投资类资产占比=%.2f%%", year, ratio)
	case 8:
		ascore := anyToFloat64(yd["AScore"])
		if ascore > 70 {
			d.Points = 3
		} else if ascore > 50 {
			d.Points = 2
		} else {
			d.Points = 1
		}
		d.Reason = fmt.Sprintf("%s年A-Score=%.1f，综合财务风险%s", year, ascore, riskComment(ascore))
	case 9:
		v := anyToFloat64(yd["growthRate"])
		if v < 0 {
			d.Points = 3
			d.Reason = fmt.Sprintf("%s年营业收入负增长 %.2f%%", year, v)
		} else if v < 5 {
			d.Points = 2
			d.Reason = fmt.Sprintf("%s年营业收入仅增长 %.2f%%", year, v)
		} else {
			d.Points = 1
			d.Reason = fmt.Sprintf("%s年营业收入增长 %.2f%%，未达10%%", year, v)
		}
	case 10:
		margin := anyToFloat64(yd["grossMargin"])
		if margin < 20 {
			d.Points = 3
		} else if margin < 40 {
			d.Points = 2
		} else {
			d.Points = 1
		}
		d.Reason = fmt.Sprintf("%s年毛利率=%.2f%%", year, margin)
	case 11:
		inv := anyToFloat64(yd["inventoryTurnover"])
		asset := anyToFloat64(yd["assetTurnover"])
		if inv < 0.5 && asset < 0.5 {
			d.Points = 3
		} else {
			d.Points = 2
		}
		d.Reason = fmt.Sprintf("%s年存货周转率=%.2f，总资产周转率=%.2f", year, inv, asset)
	case 12:
		ratio := anyToFloat64(yd["expenseToMargin"])
		if ratio > 80 {
			d.Points = 3
		} else if ratio > 40 {
			d.Points = 2
		} else {
			d.Points = 1
		}
		d.Reason = fmt.Sprintf("%s年期间费用率占毛利率比=%.2f%%", year, ratio)
	case 13:
		rd := anyToFloat64(yd["rdRatio"])
		sales := anyToFloat64(yd["salesRatio"])
		points := 0.0
		reasons := ""
		if rd < 5 {
			points += 1
			reasons += fmt.Sprintf("研发费用率%.2f%%；", rd)
		}
		if sales > 30 {
			points += 1
			reasons += fmt.Sprintf("销售费用率%.2f%%", sales)
		}
		if points == 0 {
			points = 1
		}
		d.Points = points
		d.Reason = fmt.Sprintf("%s年%s", year, reasons)
	case 14:
		margin := anyToFloat64(yd["coreProfitMargin"])
		ratio := anyToFloat64(yd["coreProfitRatio"])
		points := 0.0
		reasons := ""
		if margin < 15 {
			points += 1
			reasons += fmt.Sprintf("主营利润率%.2f%%；", margin)
		}
		if ratio < 80 {
			points += 1.5
			reasons += fmt.Sprintf("主营利润占比%.2f%%", ratio)
		}
		if points == 0 {
			points = 1
		}
		if points > 3 {
			points = 3
		}
		d.Points = points
		d.Reason = fmt.Sprintf("%s年%s", year, reasons)
	case 15:
		ratio := anyToFloat64(yd["cashRatio"])
		if ratio < 50 {
			d.Points = 3
		} else if ratio < 80 {
			d.Points = 2
		} else {
			d.Points = 1
		}
		d.Reason = fmt.Sprintf("%s年净利润现金比率=%.2f%%（5年均值未达100%%）", year, ratio)
	case 16:
		roe := anyToFloat64(yd["roe"])
		growth := anyToFloat64(yd["profitGrowth"])
		points := 0.0
		reasons := ""
		if roe < 15 {
			points += 1.5
			reasons += fmt.Sprintf("ROE %.2f%%；", roe)
		}
		if growth < 10 {
			points += 1.5
			reasons += fmt.Sprintf("归母净利润增长率 %.2f%%", growth)
		}
		if points == 0 {
			points = 1
		}
		if points > 3 {
			points = 3
		}
		d.Points = points
		d.Reason = fmt.Sprintf("%s年%s", year, reasons)
	case 17:
		ratio := anyToFloat64(yd["ratio"])
		if ratio > 100 || ratio < 0 {
			d.Points = 3
		} else if ratio > 60 || ratio < 3 {
			d.Points = 2
		} else {
			d.Points = 1
		}
		d.Reason = fmt.Sprintf("%s年购建长期资产现金支出占比=%.2f%%", year, ratio)
	case 18:
		ratio := anyToFloat64(yd["ratio"])
		if ratio > 100 || ratio < 0 {
			d.Points = 3
		} else if ratio > 70 || ratio < 20 {
			d.Points = 2
		} else {
			d.Points = 1
		}
		d.Reason = fmt.Sprintf("%s年分红现金支出占比=%.2f%%", year, ratio)
	default:
		d.Reason = fmt.Sprintf("%s年未达标", year)
	}
	return d
}

func gradeFromScore(score float64) string {
	switch {
	case score >= 90:
		return "A (优秀)"
	case score >= 80:
		return "B (良好)"
	case score >= 70:
		return "C (中等)"
	case score >= 60:
		return "D (及格)"
	default:
		return "F (不及格)"
	}
}

func anyToFloat64(v any) float64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case string:
		var f float64
		fmt.Sscanf(val, "%f", &f)
		return f
	default:
		return 0
	}
}

func riskComment(ascore float64) string {
	if ascore >= 70 {
		return "较高，建议深入核查"
	}
	if ascore >= 50 {
		return "中等，需保持关注"
	}
	return "可控"
}
