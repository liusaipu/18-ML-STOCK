package analyzer

import (
	"fmt"
	"strings"
	"testing"

	"github.com/liusaipu/stockfinlens/downloader"
)

func TestAScoreValidation(t *testing.T) {
	stocks := []struct {
		symbol string
		name   string
	}{
		{"600519.SH", "贵州茅台"},
		{"300750.SZ", "宁德时代"},
		{"000858.SZ", "五粮液"},
		{"002594.SZ", "比亚迪"},
		{"300059.SZ", "东方财富"},
		{"000002.SZ", "万科A"},
		{"600276.SH", "恒瑞医药"},
		{"601318.SH", "中国平安"},
		{"300319.SZ", "麦捷科技"},
		{"002230.SZ", "科大讯飞"},
	}

	fmt.Println("\n========== A-Score 验证结果 ==========")
	fmt.Printf("%-12s %-10s %-8s %-8s %-8s %-10s %-10s %-10s %-12s\n",
		"股票", "A-Score", "MScore", "ZScore", "CashDev", "ARRisk", "GMRisk", "Crawler", "备注")
	fmt.Println(strings.Repeat("-", 110))

	for _, s := range stocks {
		parts := strings.Split(s.symbol, ".")
		if len(parts) != 2 {
			continue
		}
		code, market := parts[0], strings.ToUpper(parts[1])

		fd, err := downloader.DownloadFinancialReports(market, code)
		if err != nil {
			fmt.Printf("%-12s 数据下载失败: %v\n", s.symbol, err)
			continue
		}

		data := &FinancialData{
			Symbol:          s.symbol,
			Years:           fd.Years,
			BalanceSheet:    fd.BalanceSheet,
			IncomeStatement: fd.IncomeStatement,
			CashFlow:        fd.CashFlow,
		}

		// 尝试获取爬虫数据
		rc, _ := downloader.FetchRiskCrawlerData(s.symbol)
		if rc != nil {
			data.Extras = make(map[string]float64)
			if rc.PledgeRatio != nil {
				data.Extras["pledge_ratio"] = *rc.PledgeRatio
			}
			if rc.InquiryCount1Y != nil {
				data.Extras["inquiry_count_1y"] = float64(*rc.InquiryCount1Y)
			}
			if rc.ReductionCount1Y != nil {
				data.Extras["reduction_count_1y"] = float64(*rc.ReductionCount1Y)
			}
		}

		result := step8RiskAnalysis(data)
		latest := ""
		if len(data.Years) > 0 {
			latest = data.Years[0]
		}

		yd := result.YearlyData[latest]
		as := anyToFloat64(yd["AScore"])
		ms := anyToFloat64(yd["MScore"])
		zs := anyToFloat64(yd["ZScore"])
		cd := anyToFloat64(yd["CashDev"])
		ar := anyToFloat64(yd["ARRisk"])
		gm := anyToFloat64(yd["GMRisk"])
		crawler := anyToFloat64(yd["CrawlerRisk"])

		note := ""
		if s.symbol == "600519.SH" && as >= 40 {
			note = "⚠️茅台应<40"
		} else if s.symbol == "300059.SZ" && as >= 60 {
			note = "⚠️东财应<60"
		}

		fmt.Printf("%-12s %-10.1f %-8.1f %-8.1f %-8.1f %-10.1f %-10.1f %-10.1f %-12s\n",
			s.symbol, as, ms, zs, cd, ar, gm, crawler, note)
	}
	fmt.Println(strings.Repeat("-", 110))
	fmt.Println("说明: Crawler = 非财务信号贡献分（含股权质押/问询函/减持）")
	fmt.Println("=======================================")
	fmt.Println()
}
