package main

import (
	"fmt"
	"sort"
	"time"

	"github.com/liusaipu/stockfinlens/analyzer"
	"github.com/liusaipu/stockfinlens/downloader"
)

// 50只覆盖各行业和市值特征的样本股
var sampleStocks = []struct {
	Code string
	Name string
}{
	{"600519", "贵州茅台"},
	{"000858", "五粮液"},
	{"601398", "工商银行"},
	{"600036", "招商银行"},
	{"601318", "中国平安"},
	{"600030", "中信证券"},
	{"000001", "平安银行"},
	{"000333", "美的集团"},
	{"000651", "格力电器"},
	{"002415", "海康威视"},
	{"300750", "宁德时代"},
	{"002594", "比亚迪"},
	{"601012", "隆基绿能"},
	{"600900", "长江电力"},
	{"600028", "中国石化"},
	{"601857", "中国石油"},
	{"601088", "中国神华"},
	{"600019", "宝钢股份"},
	{"601899", "紫金矿业"},
	{"600309", "万华化学"},
	{"688981", "中芯国际"},
	{"603501", "韦尔股份"},
	{"000725", "京东方A"},
	{"002371", "北方华创"},
	{"300760", "迈瑞医疗"},
	{"600276", "恒瑞医药"},
	{"000538", "云南白药"},
	{"603259", "药明康德"},
	{"300122", "智飞生物"},
	{"600436", "片仔癀"},
	{"000002", "万科A"},
	{"600048", "保利发展"},
	{"601668", "中国建筑"},
	{"601390", "中国中铁"},
	{"600031", "三一重工"},
	{"000425", "徐工机械"},
	{"601127", "赛力斯"},
	{"000768", "中航西飞"},
	{"600893", "航发动力"},
	{"002230", "科大讯飞"},
	{"000063", "中兴通讯"},
	{"601728", "中国电信"},
	{"600050", "中国联通"},
	{"300413", "芒果超媒"},
	{"002027", "分众传媒"},
	{"002714", "牧原股份"},
	{"600887", "伊利股份"},
	{"603288", "海天味业"},
	{"300999", "金龙鱼"},
	{"600809", "山西汾酒"},
}

func main() {
	fmt.Println("开始验证活跃度分布...")

	var scores []float64
	var details []struct {
		Code  string
		Name  string
		Score float64
		Grade string
	}

	for _, s := range sampleStocks {
		market := "SZ"
		if s.Code[0] == '6' {
			market = "SH"
		}

		// 获取K线
		klines, err := downloader.FetchStockKlines(market, s.Code, 60)
		if err != nil || len(klines) < 20 {
			fmt.Printf("[%s] K线不足: %v\n", s.Code, err)
			continue
		}

		// 获取行情
		quote, err := downloader.FetchStockQuote(market, s.Code)
		if err != nil || quote == nil || quote.CirculatingMarketCap <= 0 {
			fmt.Printf("[%s] 行情缺失\n", s.Code)
			continue
		}

		// 获取行业
		profile, err := downloader.FetchStockProfile(market, s.Code)
		industry := ""
		if err == nil && profile != nil {
			industry = profile.Industry
		}

		aklines := make([]analyzer.ActivityKline, len(klines))
		for i, k := range klines {
			aklines[i] = analyzer.ActivityKline{
				Time:   k.Time,
				Open:   k.Open,
				Close:  k.Close,
				Low:    k.Low,
				High:   k.High,
				Volume: k.Volume,
				Amount: k.Amount,
			}
		}
		qLite := &analyzer.StockQuoteLite{CirculatingMarketCap: quote.CirculatingMarketCap}
		activity := analyzer.CalculateActivity(aklines, qLite, industry, analyzer.BuildIndustryBaselines(nil))

		scores = append(scores, activity.Score)
		details = append(details, struct {
			Code  string
			Name  string
			Score float64
			Grade string
		}{s.Code, s.Name, activity.Score, activity.Grade})

		fmt.Printf("[%s %s] 活跃度=%.0f(%s) 行业=%s\n", s.Code, s.Name, activity.Score, activity.Grade, industry)
		time.Sleep(200 * time.Millisecond)
	}

	if len(scores) == 0 {
		fmt.Println("没有获取到有效数据")
		return
	}

	sort.Float64s(scores)
	fmt.Println("\n========== 分布统计 ==========")
	bins := []struct {
		min, max float64
		label    string
	}{
		{0, 10, "0-10  极低迷"},
		{10, 20, "10-20 低迷"},
		{20, 30, "20-30 一般（目标区间1）"},
		{30, 40, "30-40 一般（目标区间2）"},
		{40, 50, "40-50 较活跃"},
		{50, 70, "50-70 活跃"},
		{70, 100, "70-100 极活跃"},
	}
	for _, bin := range bins {
		cnt := 0
		for _, s := range scores {
			if bin.max == 100 {
				if s >= bin.min {
					cnt++
				}
			} else if s >= bin.min && s < bin.max {
				cnt++
			}
		}
		fmt.Printf("%s: %d只 (%.0f%%)\n", bin.label, cnt, float64(cnt)/float64(len(scores))*100)
	}
	fmt.Printf("中位数: %.1f  均值: %.1f  样本数: %d\n", median(scores), avg(scores), len(scores))

	fmt.Println("\n========== TOP5 活跃 ==========")
	sort.Slice(details, func(i, j int) bool { return details[i].Score > details[j].Score })
	for i := 0; i < minInt(5, len(details)); i++ {
		fmt.Printf("%.0f %s %s\n", details[i].Score, details[i].Code, details[i].Name)
	}
	fmt.Println("\n========== BOTTOM5 低迷 ==========")
	for i := len(details) - 5; i < len(details); i++ {
		if i >= 0 {
			fmt.Printf("%.0f %s %s\n", details[i].Score, details[i].Code, details[i].Name)
		}
	}
}

func median(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}
	m := len(data) / 2
	if len(data)%2 == 0 {
		return (data[m-1] + data[m]) / 2
	}
	return data[m]
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

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
