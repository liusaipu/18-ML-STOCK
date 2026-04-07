package analyzer

import (
	"fmt"
	"testing"
)

func TestCalculateActivity_MockData(t *testing.T) {
	// 模拟茅台：大盘股，换手0.3%，流通市值2万亿
	mockMoutai := buildMockKlines(1500, 0.3, 2e12, 20)
	quoteMoutai := &StockQuoteLite{CirculatingMarketCap: 2e12} // 2万亿
	activityMoutai := CalculateActivity(mockMoutai, quoteMoutai, "酿酒行业", BuildIndustryBaselines(nil))
	fmt.Printf("茅台模拟: score=%.0f turnoverDensity=%.3f amountScore=%.0f raw=%s\n",
		activityMoutai.Score, activityMoutai.TurnoverDensity, activityMoutai.AmountScore, activityMoutai.Comment)

	// 模拟小盘股：换手8%，流通市值40亿
	mockSmall := buildMockKlines(30, 8.0, 4e10, 20)
	quoteSmall := &StockQuoteLite{CirculatingMarketCap: 4e10} // 40亿
	activitySmall := CalculateActivity(mockSmall, quoteSmall, "半导体", BuildIndustryBaselines(nil))
	fmt.Printf("小盘模拟: score=%.0f turnoverDensity=%.3f amountScore=%.0f raw=%s\n",
		activitySmall.Score, activitySmall.TurnoverDensity, activitySmall.AmountScore, activitySmall.Comment)

	// 模拟中等活跃股：换手2%，流通市值200亿
	mockMid := buildMockKlines(50, 2.0, 2e11, 20)
	quoteMid := &StockQuoteLite{CirculatingMarketCap: 2e11} // 200亿
	activityMid := CalculateActivity(mockMid, quoteMid, "电子元件", BuildIndustryBaselines(nil))
	fmt.Printf("中盘模拟: score=%.0f turnoverDensity=%.3f amountScore=%.0f raw=%s\n",
		activityMid.Score, activityMid.TurnoverDensity, activityMid.AmountScore, activityMid.Comment)
}

func buildMockKlines(close float64, turnoverPercent float64, circCap float64, days int) []ActivityKline {
	var klines []ActivityKline
	amount := turnoverPercent / 100 * circCap
	for i := 0; i < days; i++ {
		klines = append(klines, ActivityKline{
			Time:   fmt.Sprintf("202604%02d", i+1),
			Open:   close * 0.99,
			Close:  close,
			High:   close * 1.02,
			Low:    close * 0.98,
			Volume: amount / close / 100,
			Amount: amount,
		})
	}
	return klines
}
