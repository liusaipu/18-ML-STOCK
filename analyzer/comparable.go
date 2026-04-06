package analyzer

import (
	"fmt"
	"path/filepath"
)

// ComparableMetrics 可比公司关键指标
type ComparableMetrics struct {
	Symbol        string  `json:"symbol"`
	ROE           float64 `json:"roe"`
	GrossMargin   float64 `json:"grossMargin"`
	RevenueGrowth float64 `json:"revenueGrowth"`
	DebtRatio     float64 `json:"debtRatio"`
	CashRatio     float64 `json:"cashRatio"`
	MScore        float64 `json:"mScore"`
}

// YearlyComparableMetrics 单一年度的可比公司指标集合
type YearlyComparableMetrics struct {
	Year     string                        `json:"year"`
	Metrics  map[string]*ComparableMetrics `json:"metrics"`
	Average  *ComparableMetrics            `json:"average"`
}

// ComparableAnalysis 横向对比分析结果
type ComparableAnalysis struct {
	Metrics        map[string]*ComparableMetrics `json:"metrics"`
	Average        *ComparableMetrics            `json:"average"`
	Max            *ComparableMetrics            `json:"max"`
	Min            *ComparableMetrics            `json:"min"`
	HasData        bool                          `json:"hasData"`
	YearlyTrends   []*YearlyComparableMetrics    `json:"yearlyTrends"`
	CommonYears    []string                      `json:"commonYears"`
}

// BuildComparableAnalysis 构建可比公司分析数据
func BuildComparableAnalysis(baseDir string, comparables []string) (*ComparableAnalysis, error) {
	if len(comparables) == 0 {
		return &ComparableAnalysis{Metrics: make(map[string]*ComparableMetrics)}, nil
	}

	result := &ComparableAnalysis{
		Metrics: make(map[string]*ComparableMetrics),
	}

	// 收集每家可比公司所有年份的指标
	yearlyData := make(map[string]map[string]*ComparableMetrics) // year -> comp -> metrics
	commonYearSet := make(map[string]int)

	for _, comp := range comparables {
		data, err := loadComparableFinancialData(baseDir, comp)
		if err != nil {
			fmt.Printf("[ComparableAnalysis] skip %s: %v\n", comp, err)
			continue
		}
		if len(data.Years) == 0 {
			continue
		}

		steps := []StepResult{
			step3Solvency(data),
			step8MScore(data),
			step9RevenueGrowth(data),
			step10GrossMargin(data),
			step15CashFlowQuality(data),
			step16ROE(data),
		}

		latest := data.Years[0]
		m := &ComparableMetrics{
			Symbol:        comp,
			ROE:           getStepValue(steps, 16, latest, "roe"),
			GrossMargin:   getStepValue(steps, 10, latest, "grossMargin"),
			RevenueGrowth: getStepValue(steps, 9, latest, "growthRate"),
			DebtRatio:     getStepValue(steps, 3, latest, "debtRatio"),
			CashRatio:     getStepValue(steps, 15, latest, "cashRatio"),
			MScore:        getStepValue(steps, 8, latest, "MScore"),
		}
		result.Metrics[comp] = m
		result.HasData = true

		// 收集各年份数据
		for _, year := range data.Years {
			commonYearSet[year]++
			if yearlyData[year] == nil {
				yearlyData[year] = make(map[string]*ComparableMetrics)
			}
			yearlyData[year][comp] = &ComparableMetrics{
				Symbol:        comp,
				ROE:           getStepValue(steps, 16, year, "roe"),
				GrossMargin:   getStepValue(steps, 10, year, "grossMargin"),
				RevenueGrowth: getStepValue(steps, 9, year, "growthRate"),
				DebtRatio:     getStepValue(steps, 3, year, "debtRatio"),
				CashRatio:     getStepValue(steps, 15, year, "cashRatio"),
				MScore:        getStepValue(steps, 8, year, "MScore"),
			}
		}
	}

	if !result.HasData {
		return result, nil
	}

	result.Average = calcAverage(result.Metrics)
	result.Max = calcMax(result.Metrics)
	result.Min = calcMin(result.Metrics)

	// 计算共同年份（至少有一半可比公司有该年份数据）
	threshold := len(comparables) / 2
	if threshold < 2 {
		threshold = 2
	}
	var commonYears []string
	for year, count := range commonYearSet {
		if count >= threshold {
			commonYears = append(commonYears, year)
		}
	}
	// 降序排序
	for i := 0; i < len(commonYears); i++ {
		for j := i + 1; j < len(commonYears); j++ {
			if commonYears[i] < commonYears[j] {
				commonYears[i], commonYears[j] = commonYears[j], commonYears[i]
			}
		}
	}
	result.CommonYears = commonYears

	// 计算各年份平均值
	for _, year := range commonYears {
		if metrics, ok := yearlyData[year]; ok {
			result.YearlyTrends = append(result.YearlyTrends, &YearlyComparableMetrics{
				Year:    year,
				Metrics: metrics,
				Average: calcAverage(metrics),
			})
		}
	}

	return result, nil
}

func loadComparableFinancialData(baseDir, symbol string) (*FinancialData, error) {
	stockDir := filepath.Join(baseDir, "comparables", symbol)
	bs, err := loadFloatJSONFile(filepath.Join(stockDir, "balance_sheet.json"))
	if err != nil {
		return nil, fmt.Errorf("load balance_sheet.json: %w", err)
	}
	is, err := loadFloatJSONFile(filepath.Join(stockDir, "income_statement.json"))
	if err != nil {
		return nil, fmt.Errorf("load income_statement.json: %w", err)
	}
	cf, err := loadFloatJSONFile(filepath.Join(stockDir, "cash_flow.json"))
	if err != nil {
		return nil, fmt.Errorf("load cash_flow.json: %w", err)
	}

	years := extractYearsFloat(bs)
	if len(years) == 0 {
		years = extractYearsFloat(is)
	}
	if len(years) == 0 {
		years = extractYearsFloat(cf)
	}

	return &FinancialData{
		Symbol:          symbol,
		Years:           years,
		BalanceSheet:    bs,
		IncomeStatement: is,
		CashFlow:        cf,
	}, nil
}

func calcAverage(metrics map[string]*ComparableMetrics) *ComparableMetrics {
	if len(metrics) == 0 {
		return &ComparableMetrics{}
	}
	avg := &ComparableMetrics{Symbol: "平均值"}
	var count float64
	for _, m := range metrics {
		avg.ROE += m.ROE
		avg.GrossMargin += m.GrossMargin
		avg.RevenueGrowth += m.RevenueGrowth
		avg.DebtRatio += m.DebtRatio
		avg.CashRatio += m.CashRatio
		avg.MScore += m.MScore
		count++
	}
	if count > 0 {
		avg.ROE /= count
		avg.GrossMargin /= count
		avg.RevenueGrowth /= count
		avg.DebtRatio /= count
		avg.CashRatio /= count
		avg.MScore /= count
	}
	return avg
}

func calcMax(metrics map[string]*ComparableMetrics) *ComparableMetrics {
	if len(metrics) == 0 {
		return &ComparableMetrics{}
	}
	max := &ComparableMetrics{Symbol: "最高值"}
	first := true
	for _, m := range metrics {
		if first {
			*max = *m
			max.Symbol = "最高值"
			first = false
			continue
		}
		if m.ROE > max.ROE { max.ROE = m.ROE }
		if m.GrossMargin > max.GrossMargin { max.GrossMargin = m.GrossMargin }
		if m.RevenueGrowth > max.RevenueGrowth { max.RevenueGrowth = m.RevenueGrowth }
		if m.DebtRatio > max.DebtRatio { max.DebtRatio = m.DebtRatio }
		if m.CashRatio > max.CashRatio { max.CashRatio = m.CashRatio }
		if m.MScore > max.MScore { max.MScore = m.MScore }
	}
	return max
}

func calcMin(metrics map[string]*ComparableMetrics) *ComparableMetrics {
	if len(metrics) == 0 {
		return &ComparableMetrics{}
	}
	min := &ComparableMetrics{Symbol: "最低值"}
	first := true
	for _, m := range metrics {
		if first {
			*min = *m
			min.Symbol = "最低值"
			first = false
			continue
		}
		if m.ROE < min.ROE { min.ROE = m.ROE }
		if m.GrossMargin < min.GrossMargin { min.GrossMargin = m.GrossMargin }
		if m.RevenueGrowth < min.RevenueGrowth { min.RevenueGrowth = m.RevenueGrowth }
		if m.DebtRatio < min.DebtRatio { min.DebtRatio = m.DebtRatio }
		if m.CashRatio < min.CashRatio { min.CashRatio = m.CashRatio }
		if m.MScore < min.MScore { min.MScore = m.MScore }
	}
	return min
}

// RankPercentile 计算当前指标在可比公司中的排名百分位（0~100，越高越好）
func RankPercentile(metrics map[string]*ComparableMetrics, target *ComparableMetrics, key string) float64 {
	type pair struct {
		symbol string
		value  float64
	}
	var list []pair
	for s, m := range metrics {
		var v float64
		switch key {
		case "roe": v = m.ROE
		case "grossMargin": v = m.GrossMargin
		case "revenueGrowth": v = m.RevenueGrowth
		case "debtRatio": v = -m.DebtRatio // 负债率越低越好
		case "cashRatio": v = m.CashRatio
		case "mScore": v = -m.MScore // MScore 越低越好
		}
		list = append(list, pair{s, v})
	}
	if len(list) == 0 {
		return 0
	}

	// 按 value 降序排序
	for i := 0; i < len(list); i++ {
		for j := i + 1; j < len(list); j++ {
			if list[i].value < list[j].value {
				list[i], list[j] = list[j], list[i]
			}
		}
	}

	var targetVal float64
	switch key {
	case "roe": targetVal = target.ROE
	case "grossMargin": targetVal = target.GrossMargin
	case "revenueGrowth": targetVal = target.RevenueGrowth
	case "debtRatio": targetVal = -target.DebtRatio
	case "cashRatio": targetVal = target.CashRatio
	case "mScore": targetVal = -target.MScore
	}

	for i, p := range list {
		if targetVal >= p.value {
			return float64(i) / float64(len(list)) * 100
		}
	}
	return 0
}
