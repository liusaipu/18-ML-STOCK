package analyzer

import (
	"fmt"
	"math"
	"strings"
)

// TechnicalKline 技术分析使用的单根K线数据
type TechnicalKline struct {
	Time   string  `json:"time"`
	Open   float64 `json:"open"`
	Close  float64 `json:"close"`
	Low    float64 `json:"low"`
	High   float64 `json:"high"`
	Volume float64 `json:"volume"`
}

// TechnicalData 技术分析结果
type TechnicalData struct {
	Score             float64 `json:"score"`
	Grade             string  `json:"grade"`
	Trend             string  `json:"trend"`             // 上升 / 下降 / 震荡
	MAStatus          string  `json:"maStatus"`          // 多头排列 / 空头排列 / 缠绕
	MACDStatus        string  `json:"macdStatus"`        // 金叉 / 死叉 / 零轴上 / 零轴下 / 中性
	RSIStatus         string  `json:"rsiStatus"`         // 超买 / 超卖 / 偏强 / 偏弱 / 中性
	BollingerStatus   string  `json:"bollingerStatus"`   // 上轨 / 中轨 / 下轨 / 突破上轨 / 跌破下轨
	VolumeStatus      string  `json:"volumeStatus"`      // 放量上涨 / 缩量下跌 / 量价背离 / 平和
	SupportResistance string  `json:"supportResistance"` // 支撑/压力描述
	Pattern           string  `json:"pattern"`           // 识别的形态
	Comment           string  `json:"comment"`           // 总结
}

// AnalyzeTechnical 对日K线数据进行技术分析并打分
func AnalyzeTechnical(klines []TechnicalKline) *TechnicalData {
	if len(klines) < 30 {
		return &TechnicalData{
			Score:   0,
			Grade:   "-",
			Comment: "K线数据不足（少于30根），无法生成技术分析",
		}
	}

	closes := make([]float64, len(klines))
	highs := make([]float64, len(klines))
	lows := make([]float64, len(klines))
	volumes := make([]float64, len(klines))
	for i, k := range klines {
		closes[i] = k.Close
		highs[i] = k.High
		lows[i] = k.Low
		volumes[i] = k.Volume
	}

	// 计算指标
	ma5 := sma(closes, 5)
	ma10 := sma(closes, 10)
	ma20 := sma(closes, 20)
	ma60 := sma(closes, 60)
	macdLine, macdSignal, macdHist := macd(closes)
	rsi := rsi14(closes)
	bbMid, bbUpper, bbLower := bollinger(closes, 20, 2.0)
	volMa5 := sma(volumes, 5)
	volMa20 := sma(volumes, 20)

	latest := len(closes) - 1
	price := closes[latest]

	// 1. 趋势判断 (30分)
	trendScore, trend, maStatus := evaluateTrend(price, ma5, ma10, ma20, ma60, latest)

	// 2. 动能判断 (25分)
	momScore, macdStatus, rsiStatus := evaluateMomentum(macdLine, macdSignal, macdHist, rsi, latest)

	// 3. 量价配合 (20分)
	volScore, volStatus := evaluateVolume(price, closes, volumes, volMa5, volMa20, latest, trend)

	// 4. 位置/波动 (15分)
	posScore, bbStatus := evaluatePosition(price, bbMid, bbUpper, bbLower, latest, closes)

	// 5. 支撑压力与形态 (10分)
	patScore, pattern, srDesc := evaluatePattern(closes, highs, lows, latest)

	totalScore := clamp(trendScore+momScore+volScore+posScore+patScore, 0, 100)
	grade := technicalGrade(totalScore)
	comment := buildTechnicalComment(trend, maStatus, macdStatus, rsiStatus, bbStatus, volStatus, pattern)

	return &TechnicalData{
		Score:             totalScore,
		Grade:             grade,
		Trend:             trend,
		MAStatus:          maStatus,
		MACDStatus:        macdStatus,
		RSIStatus:         rsiStatus,
		BollingerStatus:   bbStatus,
		VolumeStatus:      volStatus,
		SupportResistance: srDesc,
		Pattern:           pattern,
		Comment:           comment,
	}
}

// ============ 基础指标计算 ============

func sma(data []float64, period int) []float64 {
	result := make([]float64, len(data))
	for i := range data {
		if i < period-1 {
			result[i] = math.NaN()
			continue
		}
		sum := 0.0
		for j := 0; j < period; j++ {
			sum += data[i-j]
		}
		result[i] = sum / float64(period)
	}
	return result
}

func ema(data []float64, period int) []float64 {
	result := make([]float64, len(data))
	multiplier := 2.0 / (float64(period) + 1.0)
	for i := range data {
		if i == 0 {
			result[i] = data[i]
			continue
		}
		result[i] = (data[i]-result[i-1])*multiplier + result[i-1]
	}
	return result
}

func macd(data []float64) (line, signal, hist []float64) {
	ema12 := ema(data, 12)
	ema26 := ema(data, 26)
	line = make([]float64, len(data))
	for i := range data {
		line[i] = ema12[i] - ema26[i]
	}
	signal = ema(line, 9)
	hist = make([]float64, len(data))
	for i := range data {
		hist[i] = line[i] - signal[i]
	}
	return
}

func rsi14(data []float64) []float64 {
	period := 14
	result := make([]float64, len(data))
	for i := range data {
		if i < period {
			result[i] = math.NaN()
			continue
		}
		gain, loss := 0.0, 0.0
		for j := i - period + 1; j <= i; j++ {
			diff := data[j] - data[j-1]
			if diff > 0 {
				gain += diff
			} else {
				loss -= diff
			}
		}
		avgGain := gain / float64(period)
		avgLoss := loss / float64(period)
		if avgLoss == 0 {
			result[i] = 100
		} else {
			rs := avgGain / avgLoss
			result[i] = 100 - (100 / (1 + rs))
		}
	}
	return result
}

func stdDev(data []float64, period int) []float64 {
	result := make([]float64, len(data))
	for i := range data {
		if i < period-1 {
			result[i] = math.NaN()
			continue
		}
		sum, sumSq := 0.0, 0.0
		for j := 0; j < period; j++ {
			v := data[i-j]
			sum += v
			sumSq += v * v
		}
		mean := sum / float64(period)
		variance := (sumSq / float64(period)) - mean*mean
		if variance < 0 {
			variance = 0
		}
		result[i] = math.Sqrt(variance)
	}
	return result
}

func bollinger(data []float64, period int, multiplier float64) (mid, upper, lower []float64) {
	mid = sma(data, period)
	sd := stdDev(data, period)
	upper = make([]float64, len(data))
	lower = make([]float64, len(data))
	for i := range data {
		upper[i] = mid[i] + multiplier*sd[i]
		lower[i] = mid[i] - multiplier*sd[i]
	}
	return
}

// ============ 评分逻辑 ============

func evaluateTrend(price float64, ma5, ma10, ma20, ma60 []float64, idx int) (score float64, trend, maStatus string) {
	m5 := safeValue(ma5, idx)
	m10 := safeValue(ma10, idx)
	m20 := safeValue(ma20, idx)
	m60 := safeValue(ma60, idx)

	if math.IsNaN(m5) || math.IsNaN(m10) || math.IsNaN(m20) {
		return 15, "震荡", "数据不足"
	}

	// 均线排列判断
	bullishArrangement := m5 > m10 && m10 > m20
	bearishArrangement := m5 < m10 && m10 < m20

	if !math.IsNaN(m60) {
		bullishArrangement = bullishArrangement && m20 > m60
		bearishArrangement = bearishArrangement && m20 < m60
	}

	// 价格相对短期均线位置
	aboveMA5 := price > m5*1.01
	belowMA5 := price < m5*0.99

	if bullishArrangement && aboveMA5 {
		return 30, "上升", "多头排列"
	}
	if bullishArrangement {
		return 25, "上升", "偏多排列"
	}
	if bearishArrangement && belowMA5 {
		return 0, "下降", "空头排列"
	}
	if bearishArrangement {
		return 5, "下降", "偏空排列"
	}

	// 缠绕判断：各均线间距 < 3%
	spread := math.Abs(m5-m20) / m20 * 100
	if spread < 2 {
		return 12, "震荡", "均线缠绕"
	}
	if price > m20 {
		return 18, "震荡偏强", "价格站上MA20"
	}
	return 10, "震荡偏弱", "价格在MA20下方"
}

func evaluateMomentum(macdLine, macdSignal, macdHist, rsi []float64, idx int) (score float64, macdStatus, rsiStatus string) {
	ml := safeValue(macdLine, idx)
	ms := safeValue(macdSignal, idx)
	mhPrev := safeValue(macdHist, idx-1)
	mhCurr := safeValue(macdHist, idx)
	r := safeValue(rsi, idx)

	// MACD 状态
	if !math.IsNaN(ml) && !math.IsNaN(ms) {
		if ml > ms && (math.IsNaN(mhPrev) || mhCurr > mhPrev) {
			macdStatus = "金叉"
		} else if ml < ms && (math.IsNaN(mhPrev) || mhCurr < mhPrev) {
			macdStatus = "死叉"
		} else if ml > 0 {
			macdStatus = "零轴上"
		} else if ml < 0 {
			macdStatus = "零轴下"
		} else {
			macdStatus = "中性"
		}
	} else {
		macdStatus = "数据不足"
	}

	// RSI 状态
	if !math.IsNaN(r) {
		if r > 80 {
			rsiStatus = "超买"
		} else if r < 20 {
			rsiStatus = "超卖"
		} else if r > 60 {
			rsiStatus = "偏强"
		} else if r < 40 {
			rsiStatus = "偏弱"
		} else {
			rsiStatus = "中性"
		}
	} else {
		rsiStatus = "数据不足"
	}

	// 动能评分
	if macdStatus == "金叉" {
		score += 15
	} else if macdStatus == "零轴上" {
		score += 12
	} else if macdStatus == "中性" {
		score += 8
	} else if macdStatus == "零轴下" {
		score += 5
	} else {
		score += 2
	}

	if rsiStatus == "偏强" {
		score += 10
	} else if rsiStatus == "中性" {
		score += 8
	} else if rsiStatus == "偏弱" {
		score += 5
	} else if rsiStatus == "超卖" {
		score += 6 // 潜在反弹
	} else if rsiStatus == "超买" {
		score += 3 // 过热
	}

	return clamp(score, 0, 25), macdStatus, rsiStatus
}

func evaluateVolume(price float64, closes, volumes, volMa5, volMa20 []float64, idx int, trend string) (score float64, status string) {
	v := safeValue(volumes, idx)
	v20 := safeValue(volMa20, idx)
	prevClose := safeValue(closes, idx-1)

	priceUp := price > prevClose
	volHigh := false
	volLow := false
	if !math.IsNaN(v20) && v20 > 0 {
		volRatio := v / v20
		volHigh = volRatio > 1.3
		volLow = volRatio < 0.7
	}

	if trend == "上升" || trend == "震荡偏强" {
		if priceUp && volHigh {
			status = "放量上涨"
			return 20, status
		}
		if priceUp && !volLow {
			status = "量价齐升"
			return 17, status
		}
		if priceUp && volLow {
			status = "上涨缩量"
			return 12, status
		}
		if !priceUp && volHigh {
			status = "回调放量"
			return 8, status
		}
		return 14, "量能平和"
	}

	if trend == "下降" || trend == "震荡偏弱" {
		if !priceUp && volHigh {
			status = "下跌放量"
			return 5, status
		}
		if !priceUp && volLow {
			status = "缩量下跌"
			return 12, status
		}
		if priceUp && volHigh {
			status = "反弹放量"
			return 15, status
		}
		return 10, "量能低迷"
	}

	return 12, "量能中性"
}

func evaluatePosition(price float64, bbMid, bbUpper, bbLower []float64, idx int, closes []float64) (score float64, status string) {
	mid := safeValue(bbMid, idx)
	upper := safeValue(bbUpper, idx)
	lower := safeValue(bbLower, idx)

	if math.IsNaN(mid) {
		return 8, "数据不足"
	}

	if price > upper*0.995 {
		status = "突破上轨"
		score = 5 // 可能超买
	} else if price >= mid {
		if price > upper*0.95 {
			status = "接近上轨"
			score = 10
		} else {
			status = "中轨上方"
			score = 13
		}
	} else {
		if price < lower*1.05 {
			status = "接近下轨"
			score = 8 // 超卖待反弹
		} else {
			status = "中轨下方"
			score = 7
		}
	}

	// 近期振幅适中加分
	if len(closes) >= 20 {
		recentHigh, recentLow := closes[idx], closes[idx]
		for i := idx - 19; i <= idx; i++ {
			if i >= 0 {
				if closes[i] > recentHigh {
					recentHigh = closes[i]
				}
				if closes[i] < recentLow {
					recentLow = closes[i]
				}
			}
		}
		amp := (recentHigh - recentLow) / recentLow * 100
		if amp >= 3 && amp <= 15 {
			score += 2 // 波动适中
		}
	}

	return clamp(score, 0, 15), status
}

func evaluatePattern(closes, highs, lows []float64, idx int) (score float64, pattern, sr string) {
	if idx < 20 {
		return 5, "数据不足", "-"
	}

	// 近期20日支撑/压力
	recentHigh, recentLow := highs[idx], lows[idx]
	for i := idx - 19; i <= idx; i++ {
		if i >= 0 {
			if highs[i] > recentHigh {
				recentHigh = highs[i]
			}
			if lows[i] < recentLow {
				recentLow = lows[i]
			}
		}
	}
	sr = fmt.Sprintf("近期支撑 %.2f / 压力 %.2f", recentLow, recentHigh)

	// 双底 / W底 简化识别（最近40日找两个低点）
	if idx >= 40 {
		lows40 := make([]float64, 40)
		for i := 0; i < 40; i++ {
			lows40[i] = lows[idx-39+i]
		}
		min1Idx, min1Val := 0, lows40[0]
		for i := 1; i < 20; i++ {
			if lows40[i] < min1Val {
				min1Val = lows40[i]
				min1Idx = i
			}
		}
		min2Idx, min2Val := 20, lows40[20]
		for i := 21; i < 40; i++ {
			if lows40[i] < min2Val {
				min2Val = lows40[i]
				min2Idx = i
			}
		}
		// 两个低点接近，中间有反弹
		if math.Abs(min1Val-min2Val)/min1Val < 0.03 {
			midHigh := lows40[0]
			for i := min1Idx; i <= min2Idx; i++ {
				if highs[idx-39+i] > midHigh {
					midHigh = highs[idx-39+i]
				}
			}
			if midHigh > min1Val*1.05 && closes[idx] > midHigh*0.98 {
				return 10, "W底形态（潜在突破）", sr
			}
			return 8, "双底形态", sr
		}

		// 双顶 / M头 简化识别
		max1Val := highs[idx-39]
		for i := 1; i < 20; i++ {
			v := highs[idx-39+i]
			if v > max1Val {
				max1Val = v
			}
		}
		max2Val := highs[idx-20]
		for i := 21; i < 40; i++ {
			v := highs[idx-39+i]
			if v > max2Val {
				max2Val = v
			}
		}
		if math.Abs(max1Val-max2Val)/max1Val < 0.03 && closes[idx] < max1Val*0.95 {
			return 2, "M头形态（谨慎）", sr
		}
	}

	// 突破/跌破近期高点/低点
	if idx >= 5 {
		prevHigh := highs[idx-5]
		for i := idx - 4; i < idx; i++ {
			if highs[i] > prevHigh {
				prevHigh = highs[i]
			}
		}
		if closes[idx] > prevHigh*1.01 {
			return 8, "突破近期高点", sr
		}
	}
	if idx >= 5 {
		prevLow := lows[idx-5]
		for i := idx - 4; i < idx; i++ {
			if lows[i] < prevLow {
				prevLow = lows[i]
			}
		}
		if closes[idx] < prevLow*0.99 {
			return 3, "跌破近期低点", sr
		}
	}

	return 5, "无明显形态", sr
}

func highs40(startOffset, idx int, highs []float64) float64 {
	return highs[idx-39+startOffset]
}

func buildTechnicalComment(trend, maStatus, macdStatus, rsiStatus, bbStatus, volStatus, pattern string) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("趋势%s（%s）", trend, maStatus))
	parts = append(parts, fmt.Sprintf("MACD%s，RSI%s", macdStatus, rsiStatus))
	parts = append(parts, fmt.Sprintf("布林带%s，%s", bbStatus, volStatus))
	if pattern != "无明显形态" && pattern != "数据不足" {
		parts = append(parts, pattern)
	}
	return strings.Join(parts, "；")
}

func technicalGrade(score float64) string {
	if score >= 80 {
		return "优秀"
	}
	if score >= 60 {
		return "良好"
	}
	if score >= 40 {
		return "一般"
	}
	return "偏弱"
}

func safeValue(data []float64, idx int) float64 {
	if idx < 0 || idx >= len(data) {
		return math.NaN()
	}
	return data[idx]
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
