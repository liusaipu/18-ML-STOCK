package downloader

import "math"

// YearBalanceDiff 单年份资产负债表平衡差异
type YearBalanceDiff struct {
	Year      string  `json:"year"`
	Asset     float64 `json:"asset"`
	Liability float64 `json:"liability"`
	Equity    float64 `json:"equity"`
	Diff      float64 `json:"diff"`       // 绝对差异 = |asset - liability - equity|
	DiffRatio float64 `json:"diff_ratio"` // 相对差异比例
}

// BalanceSheetQuality 资产负债表质量评估结果
type BalanceSheetQuality struct {
	MaxDiffRatio    float64           `json:"max_diff_ratio"`   // 最大差异比例
	AvgDiffRatio    float64           `json:"avg_diff_ratio"`   // 平均差异比例
	UnbalancedCount int               `json:"unbalanced_count"` // 不平衡年份数（差异>5%）
	TotalYears      int               `json:"total_years"`      // 总年份数
	YearDiffs       []YearBalanceDiff `json:"year_diffs"`       // 各年份详细差异
	Score           float64           `json:"score"`            // 质量得分 0-100
}

// EvaluateBalanceQuality 评估 FinancialReportData 的资产负债表平衡质量
// 返回 nil 表示数据不足以评估（缺少关键科目）
func EvaluateBalanceQuality(data *FinancialReportData) *BalanceSheetQuality {
	if data == nil || len(data.Years) == 0 {
		return nil
	}

	var yearDiffs []YearBalanceDiff
	var totalDiffRatio float64
	unbalancedCount := 0
	validYears := 0

	for _, year := range data.Years {
		asset := getValueOrZero(data.BalanceSheet, "资产合计", year)
		liability := getValueOrZero(data.BalanceSheet, "负债合计", year)
		totalEquity := getValueOrZero(data.BalanceSheet, "所有者权益合计", year)

		// 如果总权益缺失，尝试用归母权益 + 少数股东权益推导
		if math.Abs(totalEquity) < 1 {
			parentEquity := getValueOrZero(data.BalanceSheet, "归属于母公司所有者权益合计", year)
			minorityEquity := getValueOrZero(data.BalanceSheet, "少数股东权益", year)
			if parentEquity > 0 || minorityEquity > 0 {
				totalEquity = parentEquity + minorityEquity
			}
		}

		// 必须三个关键科目都有正数数据才参与评估
		if asset <= 0 || liability <= 0 || totalEquity <= 0 {
			continue
		}

		diff := math.Abs(asset - liability - totalEquity)
		diffRatio := diff / asset

		yearDiffs = append(yearDiffs, YearBalanceDiff{
			Year:      year,
			Asset:     asset,
			Liability: liability,
			Equity:    totalEquity,
			Diff:      diff,
			DiffRatio: diffRatio,
		})

		totalDiffRatio += diffRatio
		validYears++
		if diffRatio > 0.05 {
			unbalancedCount++
		}
	}

	if validYears == 0 {
		return nil
	}

	maxDiffRatio := 0.0
	for _, yd := range yearDiffs {
		if yd.DiffRatio > maxDiffRatio {
			maxDiffRatio = yd.DiffRatio
		}
	}

	avgDiffRatio := totalDiffRatio / float64(validYears)

	// 得分计算：以最大差异为基准，差异越小得分越高
	// maxDiffRatio = 0 时得 100 分，>= 0.2 时得 0 分
	score := 100.0 * (1.0 - math.Min(maxDiffRatio*5.0, 1.0))
	if score < 0 {
		score = 0
	}

	return &BalanceSheetQuality{
		MaxDiffRatio:    maxDiffRatio,
		AvgDiffRatio:    avgDiffRatio,
		UnbalancedCount: unbalancedCount,
		TotalYears:      validYears,
		YearDiffs:       yearDiffs,
		Score:           score,
	}
}

// getValueOrZero 从 data[account][year] 中读取 float64，缺失时返回 0
func getValueOrZero(data map[string]map[string]float64, account, year string) float64 {
	if data == nil {
		return 0
	}
	row, ok := data[account]
	if !ok {
		return 0
	}
	return row[year]
}

// IsBetterThan 判断当前质量是否优于另一个质量
// 优先比较最大差异比例，其次比较平均差异，最后比较不平衡年份数
func (q *BalanceSheetQuality) IsBetterThan(other *BalanceSheetQuality) bool {
	if other == nil {
		return true
	}
	if q.MaxDiffRatio != other.MaxDiffRatio {
		return q.MaxDiffRatio < other.MaxDiffRatio
	}
	if q.AvgDiffRatio != other.AvgDiffRatio {
		return q.AvgDiffRatio < other.AvgDiffRatio
	}
	return q.UnbalancedCount < other.UnbalancedCount
}

// SuggestAlternativeSource 根据当前数据质量，返回是否建议尝试其他数据源
// threshold 为触发建议的差异阈值（如 0.05 表示 5%）
func (q *BalanceSheetQuality) SuggestAlternativeSource(threshold float64) bool {
	return q.MaxDiffRatio > threshold && q.TotalYears > 0
}
