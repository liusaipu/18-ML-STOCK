package analyzer

import (
	"fmt"
	"math"
	"strings"
)

// ActivityData 交易活跃度分析结果
type ActivityData struct {
	Score            float64 `json:"score"`
	Grade            string  `json:"grade"`
	Stars            int     `json:"stars"`            // 1-5星
	TurnoverDensity  float64 `json:"turnoverDensity"`  // 换手密度（近5日平均换手率）
	AvgAmplitude5    float64 `json:"avgAmplitude5"`    // 近5日平均振幅
	AmountScore      float64 `json:"amountScore"`      // 金额分
	SustainedScore   float64 `json:"sustainedScore"`   // 持续性分
	VolatilityScore  float64 `json:"volatilityScore"`  // 波动分
	TimeStructScore  float64 `json:"timeStructScore"`  // 时间结构分
	IndustryAdjusted float64 `json:"industryAdjusted"` // 行业修正系数
	CapAdjusted      float64 `json:"capAdjusted"`      // 市值修正系数
	Comment          string  `json:"comment"`
	PotentialHint    string  `json:"potentialHint"`    // 潜力提示
}

// IndustryBaseline 单个行业的基准数据
type IndustryBaseline struct {
	AvgTurnover   float64 `json:"avgTurnover"`
	MedianTurnover float64 `json:"medianTurnover"`
	SampleCount   int     `json:"sampleCount"`
}

// DefaultIndustryBaselines 内置默认行业基准（当样本不足时使用）
// 数据来源：A股历史平均水平近似值，可根据市场情况调整
var DefaultIndustryBaselines = map[string]*IndustryBaseline{
	"银行":          {AvgTurnover: 0.35, MedianTurnover: 0.28, SampleCount: 0},
	"保险":          {AvgTurnover: 0.45, MedianTurnover: 0.38, SampleCount: 0},
	"证券":          {AvgTurnover: 2.80, MedianTurnover: 2.20, SampleCount: 0},
	"电力":          {AvgTurnover: 1.20, MedianTurnover: 0.95, SampleCount: 0},
	"水务":          {AvgTurnover: 0.90, MedianTurnover: 0.70, SampleCount: 0},
	"燃气":          {AvgTurnover: 1.10, MedianTurnover: 0.85, SampleCount: 0},
	"煤炭":          {AvgTurnover: 1.50, MedianTurnover: 1.20, SampleCount: 0},
	"石油":          {AvgTurnover: 0.80, MedianTurnover: 0.65, SampleCount: 0},
	"钢铁":          {AvgTurnover: 1.30, MedianTurnover: 1.00, SampleCount: 0},
	"有色金属":       {AvgTurnover: 2.00, MedianTurnover: 1.60, SampleCount: 0},
	"化学制品":       {AvgTurnover: 2.20, MedianTurnover: 1.80, SampleCount: 0},
	"半导体":         {AvgTurnover: 3.50, MedianTurnover: 2.80, SampleCount: 0},
	"电子元件":       {AvgTurnover: 3.00, MedianTurnover: 2.40, SampleCount: 0},
	"软件开发":       {AvgTurnover: 4.00, MedianTurnover: 3.20, SampleCount: 0},
	"计算机设备":     {AvgTurnover: 3.20, MedianTurnover: 2.60, SampleCount: 0},
	"通信设备":       {AvgTurnover: 2.80, MedianTurnover: 2.20, SampleCount: 0},
	"互联网服务":     {AvgTurnover: 3.50, MedianTurnover: 2.80, SampleCount: 0},
	"文化传媒":       {AvgTurnover: 2.50, MedianTurnover: 2.00, SampleCount: 0},
	"游戏":          {AvgTurnover: 3.00, MedianTurnover: 2.40, SampleCount: 0},
	"食品饮料":       {AvgTurnover: 1.80, MedianTurnover: 1.40, SampleCount: 0},
	"白酒":          {AvgTurnover: 1.20, MedianTurnover: 0.90, SampleCount: 0},
	"医药商业":       {AvgTurnover: 1.80, MedianTurnover: 1.40, SampleCount: 0},
	"化学制药":       {AvgTurnover: 2.20, MedianTurnover: 1.70, SampleCount: 0},
	"生物制品":       {AvgTurnover: 2.00, MedianTurnover: 1.50, SampleCount: 0},
	"医疗器械":       {AvgTurnover: 2.20, MedianTurnover: 1.70, SampleCount: 0},
	"医疗服务":       {AvgTurnover: 2.00, MedianTurnover: 1.50, SampleCount: 0},
	"中药":          {AvgTurnover: 2.00, MedianTurnover: 1.60, SampleCount: 0},
	"房地产":         {AvgTurnover: 1.50, MedianTurnover: 1.10, SampleCount: 0},
	"房地产开发":     {AvgTurnover: 1.40, MedianTurnover: 1.00, SampleCount: 0},
	"工程机械":       {AvgTurnover: 1.80, MedianTurnover: 1.40, SampleCount: 0},
	"专用设备":       {AvgTurnover: 2.20, MedianTurnover: 1.70, SampleCount: 0},
	"通用设备":       {AvgTurnover: 2.00, MedianTurnover: 1.50, SampleCount: 0},
	"汽车零部件":     {AvgTurnover: 2.50, MedianTurnover: 2.00, SampleCount: 0},
	"汽车整车":       {AvgTurnover: 1.80, MedianTurnover: 1.40, SampleCount: 0},
	"家电":          {AvgTurnover: 1.60, MedianTurnover: 1.20, SampleCount: 0},
	"纺织":          {AvgTurnover: 2.00, MedianTurnover: 1.50, SampleCount: 0},
	"服装":          {AvgTurnover: 1.80, MedianTurnover: 1.30, SampleCount: 0},
	"造纸":          {AvgTurnover: 1.60, MedianTurnover: 1.20, SampleCount: 0},
	"建材":          {AvgTurnover: 1.40, MedianTurnover: 1.00, SampleCount: 0},
	"建筑装饰":       {AvgTurnover: 1.60, MedianTurnover: 1.20, SampleCount: 0},
	"航运港口":       {AvgTurnover: 1.20, MedianTurnover: 0.90, SampleCount: 0},
	"物流":          {AvgTurnover: 1.50, MedianTurnover: 1.10, SampleCount: 0},
	"航空机场":       {AvgTurnover: 1.20, MedianTurnover: 0.90, SampleCount: 0},
	"旅游酒店":       {AvgTurnover: 2.50, MedianTurnover: 2.00, SampleCount: 0},
	"零售":          {AvgTurnover: 2.20, MedianTurnover: 1.70, SampleCount: 0},
	"农牧饲渔":       {AvgTurnover: 2.00, MedianTurnover: 1.50, SampleCount: 0},
	"军工":          {AvgTurnover: 2.80, MedianTurnover: 2.20, SampleCount: 0},
	"航天航空":       {AvgTurnover: 3.00, MedianTurnover: 2.40, SampleCount: 0},
	"光伏":          {AvgTurnover: 2.50, MedianTurnover: 2.00, SampleCount: 0},
	"电池":          {AvgTurnover: 2.80, MedianTurnover: 2.20, SampleCount: 0},
	"新能源":         {AvgTurnover: 2.60, MedianTurnover: 2.00, SampleCount: 0},
}

// BuildIndustryBaselines 从自选股样本数据构建行业基准（不足时补充默认值）
func BuildIndustryBaselines(samples map[string]*IndustryBaseline) map[string]*IndustryBaseline {
	result := make(map[string]*IndustryBaseline)
	for industry, base := range DefaultIndustryBaselines {
		result[industry] = &IndustryBaseline{
			AvgTurnover:    base.AvgTurnover,
			MedianTurnover: base.MedianTurnover,
			SampleCount:    0,
		}
	}
	// 用样本数据覆盖/混合默认值
	for industry, sample := range samples {
		if sample.SampleCount == 0 {
			continue
		}
		if defaultBase, ok := DefaultIndustryBaselines[industry]; ok && sample.SampleCount < 5 {
			// 样本不足5只时，与默认值加权混合
			w := float64(sample.SampleCount) / 5.0
			result[industry] = &IndustryBaseline{
				AvgTurnover:    defaultBase.AvgTurnover*(1-w) + sample.AvgTurnover*w,
				MedianTurnover: defaultBase.MedianTurnover*(1-w) + sample.MedianTurnover*w,
				SampleCount:    sample.SampleCount,
			}
		} else {
			result[industry] = &IndustryBaseline{
				AvgTurnover:    sample.AvgTurnover,
				MedianTurnover: sample.MedianTurnover,
				SampleCount:    sample.SampleCount,
			}
		}
	}
	return result
}

// ActivityKline 活跃度分析使用的单根K线数据
type ActivityKline struct {
	Time   string  `json:"time"`
	Open   float64 `json:"open"`
	Close  float64 `json:"close"`
	Low    float64 `json:"low"`
	High   float64 `json:"high"`
	Volume float64 `json:"volume"`
	Amount float64 `json:"amount"`
}

// StockQuoteLite 活跃度分析使用的行情数据子集
type StockQuoteLite struct {
	CirculatingMarketCap float64 `json:"circulatingMarketCap"`
}

// CalculateActivity 计算单只股票的活跃度得分
func CalculateActivity(klines []ActivityKline, quote *StockQuoteLite, industry string, baselines map[string]*IndustryBaseline) *ActivityData {
	if len(klines) < 20 || quote == nil || quote.CirculatingMarketCap <= 0 {
		return &ActivityData{
			Score:   0,
			Grade:   "-",
			Comment: "行情数据不足，无法计算活跃度",
		}
	}

	closes := make([]float64, len(klines))
	highs := make([]float64, len(klines))
	lows := make([]float64, len(klines))
	volumes := make([]float64, len(klines))
	amounts := make([]float64, len(klines))
	for i, k := range klines {
		closes[i] = k.Close
		highs[i] = k.High
		lows[i] = k.Low
		volumes[i] = k.Volume
		amounts[i] = k.Amount
	}

	// 计算均值
	avgTurnover5 := avg(turnoverRates(klines, quote.CirculatingMarketCap, len(klines)-5, len(klines)))
	avgTurnover20 := avg(turnoverRates(klines, quote.CirculatingMarketCap, len(klines)-20, len(klines)))
	avgAmount5 := avgAmount(klines, len(klines)-5, len(klines))
	avgAmplitude5 := avg(amplitudes(highs, lows, closes, len(klines)-5, len(klines)))

	// 1. 换手率评分 (30%)
	turnoverScore := 0.0
	if avgTurnover5 > 0 {
		turnoverScore = math.Log(avgTurnover5+0.2)*40.0 + 10.0
		turnoverScore = clampFloat64(turnoverScore, 0, 100)
	}

	// 2. 成交金额评分 (30%)
	amountScore := 0.0
	if avgAmount5 > 0 {
		amountScore = math.Log(avgAmount5/10e6) * 15.0
		amountScore = clampFloat64(amountScore, 0, 100)
	}

	// 3. 持续性分 (20%)
	sustainedScore := 30.0
	if avgTurnover20 > 0 {
		ratio := avgTurnover5 / avgTurnover20
		sustainedScore = sigmoid((ratio-1.0)*4.0) * 100.0
	}

	// 4. 波动分 (15%)
	volatilityScore := math.Min(100, avgAmplitude5*5.0)

	// 5. 时间结构分 (5%)
	timeStructScore := timeStructureScore(turnoverRates(klines, quote.CirculatingMarketCap, len(klines)-20, len(klines)))

	// 行业修正系数
	industryAdj := 1.0
	if baselines != nil {
		baseline := lookupBaseline(industry, baselines)
		if baseline != nil && baseline.AvgTurnover > 0 && avgTurnover5 > 0 {
			percentile := 0.5
			if avgTurnover5 > baseline.AvgTurnover {
				percentile = 0.5 - math.Min(0.4, (avgTurnover5/baseline.AvgTurnover-1.0)*0.5)
			} else {
				percentile = 0.5 + math.Min(0.4, (1.0-avgTurnover5/baseline.AvgTurnover)*0.5)
			}
			// 把percentile映射到系数：前10%→1.16，后10%→0.84
			industryAdj = 1.0 + (0.5-percentile)*0.8
			industryAdj = clampFloat64(industryAdj, 0.84, 1.16)
		}
	}

	rawScore := turnoverScore*0.30 + amountScore*0.30 + sustainedScore*0.20 + volatilityScore*0.15 + timeStructScore*0.05
	adjustedScore := rawScore * industryAdj
	finalScore := activityFinalScore(adjustedScore)
	finalScore = clampFloat64(finalScore, 0, 100)

	comment := fmt.Sprintf("换手%.2f%%(%.0f分) 金额%.0f万(%.0f分) 持续%.0f分 波动%.0f分 行业修正%.2f",
		avgTurnover5, turnoverScore, avgAmount5/1e4, amountScore, sustainedScore, volatilityScore, industryAdj)

	stars := activityStars(finalScore)
	hint := buildPotentialHint(finalScore, sustainedScore, volatilityScore, avgTurnover5, industryAdj)

	return &ActivityData{
		Score:            finalScore,
		Grade:            activityGrade(finalScore),
		Stars:            stars,
		TurnoverDensity:  avgTurnover5,
		AvgAmplitude5:    avgAmplitude5,
		AmountScore:      amountScore,
		SustainedScore:   sustainedScore,
		VolatilityScore:  volatilityScore,
		TimeStructScore:  timeStructScore,
		IndustryAdjusted: industryAdj,
		CapAdjusted:      1.0,
		Comment:          comment,
		PotentialHint:    hint,
	}
}

// ========== 工具函数 ==========

func turnoverRates(klines []ActivityKline, circCap float64, start, end int) []float64 {
	var res []float64
	for i := start; i < end && i < len(klines); i++ {
		if i < 0 || klines[i].Close <= 0 || circCap <= 0 {
			continue
		}
		// 优先使用成交金额计算换手率，否则用收盘价×成交量(手)×100 估算
		turnover := 0.0
		if klines[i].Amount > 0 {
			turnover = klines[i].Amount / circCap * 100
		} else {
			turnover = klines[i].Volume * 100 * klines[i].Close / circCap * 100
		}
		res = append(res, turnover)
	}
	return res
}

func amplitudes(highs, lows, closes []float64, start, end int) []float64 {
	var res []float64
	for i := start; i < end && i < len(highs); i++ {
		if i < 0 || closes[i] <= 0 {
			continue
		}
		res = append(res, (highs[i]-lows[i])/closes[i]*100)
	}
	return res
}

func avg(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range data {
		sum += v
	}
	return sum / float64(len(data))
}

func avgAmount(klines []ActivityKline, start, end int) float64 {
	if start < 0 {
		start = 0
	}
	if end > len(klines) {
		end = len(klines)
	}
	if start >= end {
		return 0
	}
	sum := 0.0
	for i := start; i < end; i++ {
		sum += klines[i].Amount
	}
	return sum / float64(end-start)
}

func timeStructureScore(turnovers []float64) float64 {
	if len(turnovers) < 3 {
		return 0
	}
	avg20 := avg(turnovers)
	if avg20 <= 0 {
		return 0
	}
	// 近3日
	count := 0
	for i := len(turnovers) - 3; i < len(turnovers); i++ {
		if i >= 0 && turnovers[i] > avg20*1.2 {
			count++
		}
	}
	return float64(count) * 33.3
}

func sigmoid(x float64) float64 {
	return 1.0 / (1.0 + math.Exp(-x))
}

func activityFinalScore(raw float64) float64 {
	if raw <= 20 {
		return raw * 0.7
	} else if raw <= 40 {
		return 14 + (raw-20)*0.85
	}
	return 31 + (raw-40)*0.75
}

func activityGrade(score float64) string {
	switch activityStars(score) {
	case 5:
		return "极度活跃"
	case 4:
		return "活跃"
	case 3:
		return "较活跃"
	case 2:
		return "温和"
	default:
		return "低迷"
	}
}

func activityStars(score float64) int {
	if score >= 65 {
		return 5
	}
	if score >= 50 {
		return 4
	}
	if score >= 35 {
		return 3
	}
	if score >= 20 {
		return 2
	}
	return 1
}

func buildPotentialHint(score, sustainedScore, volatilityScore, avgTurnover5, industryAdj float64) string {
	stars := activityStars(score)
	var hints []string

	// 持续活跃的二三星股票：蓄势待发提示
	if stars >= 2 && stars <= 3 {
		if sustainedScore >= 55 {
			if avgTurnover5 > 0 && industryAdj >= 1.0 {
				hints = append(hints, "近期交易活跃度持续高于行业平均，当前处于蓄势阶段，若后续放量突破，存在较大概率出现波段上涨行情，建议重点关注。")
			} else {
				hints = append(hints, "近期活跃度持续保持在较好水平，当前处于温和蓄势期，可留意后续量能配合及股价突破信号。")
			}
		} else if sustainedScore >= 40 && volatilityScore >= 20 {
			hints = append(hints, "换手温和但波动放大，资金关注度在提升，具备潜在活跃基础。")
		}
	}

	// 四五星：强势但提示追高风险
	if stars >= 4 {
		if sustainedScore >= 60 {
			hints = append(hints, "当前交投活跃且动能持续，短期强势特征明显，但需注意追高风险，建议回调后再择机关注。")
		} else {
			hints = append(hints, "活跃度较高但持续性一般，可能是消息脉冲或短期热点刺激，需谨慎判断持续性。")
		}
	}

	// 一星：无人问津提示
	if stars == 1 && sustainedScore < 20 {
		hints = append(hints, "该股近期交投极度低迷，缺乏资金关注，短期难有大的上涨动能。")
	}

	if len(hints) == 0 {
		return ""
	}
	return strings.Join(hints, " ")
}

func lookupBaseline(industry string, baselines map[string]*IndustryBaseline) *IndustryBaseline {
	if b, ok := baselines[industry]; ok {
		return b
	}
	// 模糊匹配：取最长前缀
	var best string
	for k := range baselines {
		if len(k) > len(best) && len(k) <= len(industry)+2 {
			if containsSubstr(industry, k) || containsSubstr(k, industry) {
				best = k
			}
		}
	}
	if best != "" {
		return baselines[best]
	}
	// 通用默认
	return &IndustryBaseline{AvgTurnover: 1.5, MedianTurnover: 1.2, SampleCount: 0}
}

func containsSubstr(a, b string) bool {
	return len(a) >= len(b) && (a == b || (len(b) > 1 && strings.Contains(a, b)))
}

func clampFloat64(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
