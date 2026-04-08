package analyzer

import (
	"fmt"
	"math"
)

// RIMForecast 预测期数据
type RIMForecast struct {
	EPS []float64 // 各期 EPS
	DPS []float64 // 各期 DPS，长度不足时默认 0
}

// RIMParams 多期 RIM 输入参数
type RIMParams struct {
	BPS0         float64
	KE           float64
	GTerminal    float64
	Forecast     RIMForecast
	CurrentPrice float64
}

// RIMYearDetail 单年度计算明细
type RIMYearDetail struct {
	Year     int
	EPS      float64
	DPS      float64
	BPS      float64
	RE       float64
	Discount float64
	PVRE     float64
}

// RIMScenario 单情景结果
type RIMScenario struct {
	ROE     float64
	Value   float64
	DiffPct float64
	Grade   string
}

// RIMResult 多期 RIM 计算结果
type RIMResult struct {
	Details     []RIMYearDetail
	SumPVRE     float64
	CV          float64
	PVCV        float64
	Value       float64
	Upside      float64
	Pessimistic RIMScenario
	Baseline    RIMScenario
	Optimistic  RIMScenario
}



// CalculateMultiPeriodRIM 执行多期剩余收益模型计算
func CalculateMultiPeriodRIM(p RIMParams) *RIMResult {
	n := len(p.Forecast.EPS)
	if n == 0 || p.BPS0 <= 0 || p.KE <= p.GTerminal {
		return nil
	}

	res := &RIMResult{}
	bps := p.BPS0
	sumPVRE := 0.0
	details := make([]RIMYearDetail, 0, n)

	for i := 0; i < n; i++ {
		eps := p.Forecast.EPS[i]
		dps := 0.0
		if i < len(p.Forecast.DPS) {
			dps = p.Forecast.DPS[i]
		}
		re := eps - bps*p.KE
		discount := math.Pow(1+p.KE, float64(i+1))
		pvre := re / discount
		sumPVRE += pvre

		details = append(details, RIMYearDetail{
			Year:     i + 1,
			EPS:      eps,
			DPS:      dps,
			BPS:      bps,
			RE:       re,
			Discount: discount,
			PVRE:     pvre,
		})

		bps = bps + eps - dps
	}

	// 持续价值 CV (基于最后一年 RE)
	lastBPS := details[n-1].BPS
	lastEPS := details[n-1].EPS
	lastDPS := details[n-1].DPS
	reTerminal := lastEPS - (lastBPS-lastEPS+lastDPS)*p.KE
	cv := reTerminal * (1 + p.GTerminal) / (p.KE - p.GTerminal)
	discountTerminal := math.Pow(1+p.KE, float64(n))
	pvcv := cv / discountTerminal

	value := p.BPS0 + sumPVRE + pvcv
	upside := 0.0
	if p.CurrentPrice > 0 {
		upside = (value - p.CurrentPrice) / p.CurrentPrice * 100
	}

	res.Details = details
	res.SumPVRE = sumPVRE
	res.CV = cv
	res.PVCV = pvcv
	res.Value = value
	res.Upside = upside

	// 情景分析: 基于 ROE 假设差异 (悲观 -3pp, 基准, 乐观 +3pp)
	avgROE := 0.0
	if p.BPS0 > 0 {
		for _, d := range details {
			avgROE += d.EPS / d.BPS
		}
		avgROE = avgROE / float64(len(details)) * 100
	}

	grade := "中性"
	if upside >= 30 {
		grade = "积极"
	} else if upside >= 10 {
		grade = "谨慎推荐"
	} else if upside <= -10 {
		grade = "高估"
	}
	res.Baseline = RIMScenario{ROE: avgROE, Value: value, DiffPct: upside, Grade: grade}

	scenarios := []struct {
		name string
		roe  float64
	}{
		{"pessimistic", avgROE - 3},
		{"optimistic", avgROE + 3},
	}
	for _, s := range scenarios {
		// 简化情景: 保持 EPS 序列比例不变，整体按 ROE 差异缩放
		scale := 1.0
		if avgROE != 0 {
			scale = s.roe / avgROE
		}
		scaledEPS := make([]float64, n)
		for i, d := range details {
			scaledEPS[i] = d.EPS * scale
		}
		forecast := RIMForecast{EPS: scaledEPS}
		sim := simulateRIM(p.BPS0, p.KE, p.GTerminal, forecast, p.CurrentPrice)
		sg := "中性"
		if sim.DiffPct >= 30 {
			sg = "积极"
		} else if sim.DiffPct >= 10 {
			sg = "谨慎推荐"
		} else if sim.DiffPct <= -10 {
			sg = "高估"
		}
		sim.Grade = sg
		sim.ROE = s.roe

		switch s.name {
		case "pessimistic":
			res.Pessimistic = sim
		case "optimistic":
			res.Optimistic = sim
		}
	}

	return res
}

func simulateRIM(bps0, ke, gTerminal float64, forecast RIMForecast, currentPrice float64) RIMScenario {
	n := len(forecast.EPS)
	if n == 0 || ke <= gTerminal {
		return RIMScenario{}
	}
	bps := bps0
	sumPVRE := 0.0
	var lastBPS, lastEPS, lastDPS float64
	for i := 0; i < n; i++ {
		eps := forecast.EPS[i]
		dps := 0.0
		if i < len(forecast.DPS) {
			dps = forecast.DPS[i]
		}
		re := eps - bps*ke
		discount := math.Pow(1+ke, float64(i+1))
		sumPVRE += re / discount
		lastBPS = bps
		lastEPS = eps
		lastDPS = dps
		bps = bps + eps - dps
	}
	reTerminal := lastEPS - (lastBPS-lastEPS+lastDPS)*ke
	cv := reTerminal * (1 + gTerminal) / (ke - gTerminal)
	discountTerminal := math.Pow(1+ke, float64(n))
	pvcv := cv / discountTerminal
	value := bps0 + sumPVRE + pvcv
	diffPct := 0.0
	if currentPrice > 0 {
		diffPct = (value - currentPrice) / currentPrice * 100
	}
	return RIMScenario{Value: value, DiffPct: diffPct}
}

// rimGradeComment 生成评级描述
func rimGradeComment(diffPct float64) string {
	switch {
	case diffPct >= 30:
		return "显著低估"
	case diffPct >= 10:
		return "轻度低估"
	case diffPct >= -10:
		return "估值合理"
	case diffPct >= -30:
		return "轻度高估"
	default:
		return "显著高估"
	}
}

// FormatRIMCurrency 格式化金额
func FormatRIMCurrency(v float64) string {
	if v < 0 {
		return fmt.Sprintf("-%.2f", -v)
	}
	return fmt.Sprintf("%.2f", v)
}
