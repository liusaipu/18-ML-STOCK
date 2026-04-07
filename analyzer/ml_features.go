package analyzer

import (
	"math"
)

// MLKlineData 机器学习模块使用的K线数据（避免循环导入）
type MLKlineData struct {
	Time   string
	Open   float64
	Close  float64
	Low    float64
	High   float64
	Volume float64
	Amount float64
}

// BuildMLEngineBInput 从 FinancialData 提取最近 8 个季度的财务特征序列
func BuildMLEngineBInput(data *FinancialData) [][]float64 {
	if data == nil || len(data.Years) == 0 {
		return nil
	}
	records := []map[string]float64{}
	for _, year := range data.Years {
		rec := extractFinancialFeatures(data, year)
		if rec != nil {
			records = append(records, rec)
		}
	}
	if len(records) == 0 {
		return nil
	}

	seq := make([][]float64, 0, 8)
	for i := 0; i < len(records) && i < 8; i++ {
		r := records[i]
		seq = append(seq, []float64{
			r["roe"],
			r["gross_margin"],
			r["debt_ratio"],
			r["cash_ratio"],
			r["turnover"],
			r["mscore_proxy"],
			r["revenue"] / 1e8,
			r["net_profit"] / 1e8,
		})
	}
	return seq
}

func extractFinancialFeatures(data *FinancialData, year string) map[string]float64 {
	bs := data.BalanceSheet[year]
	inc := data.IncomeStatement[year]
	cf := data.CashFlow[year]
	if bs == nil || inc == nil || cf == nil {
		return nil
	}

	revenue := getFloat(inc, "营业收入")
	cost := getFloat(inc, "营业成本")
	netProfit := getFloat(inc, "净利润")
	totalAssets := getFloat(bs, "总资产")
	totalLiabilities := getFloat(bs, "总负债")
	equity := getFloat(bs, "所有者权益合计")
	if equity == 0 {
		equity = totalAssets - totalLiabilities
	}
	if equity == 0 {
		equity = 1e-6
	}

	grossMargin := 0.0
	if revenue != 0 {
		grossMargin = (revenue - cost) / revenue
	}
	roe := netProfit / equity
	debtRatio := 0.0
	if totalAssets != 0 {
		debtRatio = totalLiabilities / totalAssets
	}
	opCash := getFloat(cf, "经营活动产生的现金流量净额")
	cashRatio := 0.0
	if netProfit != 0 {
		cashRatio = opCash / netProfit
	}

	inventory := getFloat(bs, "存货")
	receivables := getFloat(bs, "应收账款")
	turnover := 0.0
	if inventory+receivables > 0 {
		turnover = revenue / (inventory + receivables)
	}

	accruals := netProfit - opCash
	mscoreProxy := 0.0
	if totalAssets != 0 {
		mscoreProxy = -accruals / totalAssets
	}

	return map[string]float64{
		"roe":          roe,
		"gross_margin": grossMargin,
		"debt_ratio":   debtRatio,
		"cash_ratio":   cashRatio,
		"turnover":     turnover,
		"mscore_proxy": mscoreProxy,
		"revenue":      revenue,
		"net_profit":   netProfit,
	}
}

func getFloat(m map[string]float64, key string) float64 {
	if v, ok := m[key]; ok {
		return v
	}
	return 0.0
}

// BuildMLEngineAInputFromKlines 用日K线+舆情数据构造引擎A的简化输入
// text_seq: [16, 32], price_seq: [16, 24]
func BuildMLEngineAInputFromKlines(klines []MLKlineData, sentiment *SentimentData) ([][]float64, [][]float64) {
	if len(klines) < 16 {
		return nil, nil
	}
	// 取最近 16 根日K作为 16 个时间桶（简化版）
	recent := klines[len(klines)-16:]

	priceSeq := make([][]float64, 16)
	for i, k := range recent {
		returns := 0.0
		if i > 0 && recent[i-1].Close != 0 {
			returns = (k.Close - recent[i-1].Close) / recent[i-1].Close
		}
		volMa := 0.0
		if i >= 5 {
			sum := 0.0
			for j := i - 4; j <= i; j++ {
				sum += recent[j].Volume
			}
			volMa = sum / 5.0
		}
		volRatio := 1.0
		if volMa > 0 {
			volRatio = k.Volume / volMa
		}
		amplitude := 0.0
		if k.Open != 0 {
			amplitude = (k.High - k.Low) / k.Open
		}
		// 构建 24 维价格特征（后面用0填充）
		feat := make([]float64, 24)
		feat[0] = returns
		feat[1] = volRatio
		feat[2] = amplitude
		feat[3] = k.Close
		feat[4] = k.Volume / 1e6
		priceSeq[i] = feat
	}

	textSeq := make([][]float64, 16)
	for i := range textSeq {
		feat := make([]float64, 32)
		if sentiment != nil && sentiment.HasData {
			feat[0] = sentiment.Score / 100.0           // 情感分数归一化
			feat[1] = float64(sentiment.HeatIndex) / 100.0 // 热度
			feat[2] = float64(len(sentiment.PositiveWords)) / 10.0
			feat[3] = float64(len(sentiment.NegativeWords)) / 10.0
		}
		// 其余维度保持 0（模型训练时允许部分缺失）
		textSeq[i] = feat
	}
	return textSeq, priceSeq
}

// normalize with zscore simple version
func zscore(values []float64) []float64 {
	if len(values) == 0 {
		return values
	}
	mean := 0.0
	for _, v := range values {
		mean += v
	}
	mean /= float64(len(values))
	std := 0.0
	for _, v := range values {
		d := v - mean
		std += d * d
	}
	std = math.Sqrt(std / float64(len(values)))
	if std == 0 {
		std = 1e-6
	}
	out := make([]float64, len(values))
	for i, v := range values {
		out[i] = (v - mean) / std
	}
	return out
}
