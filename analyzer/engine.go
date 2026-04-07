package analyzer

import (
	"fmt"
)

// RunAnalysis 执行完整的18步分析，返回报告
func RunAnalysis(baseDir, symbol string) (*AnalysisReport, error) {
	return RunAnalysisWithComparablesAndQuoteAndSentimentAndPolicy(baseDir, symbol, nil, nil, nil, nil)
}

// RunAnalysisWithComparables 执行完整的18步分析，并集成可比公司横向对比
func RunAnalysisWithComparables(baseDir, symbol string, comp *ComparableAnalysis) (*AnalysisReport, error) {
	return RunAnalysisWithComparablesAndQuoteAndSentimentAndPolicy(baseDir, symbol, comp, nil, nil, nil)
}

// RunAnalysisWithComparablesAndQuote 执行完整的18步分析，集成可比公司横向对比与实时行情
func RunAnalysisWithComparablesAndQuote(baseDir, symbol string, comp *ComparableAnalysis, quote *QuoteData) (*AnalysisReport, error) {
	return RunAnalysisWithComparablesAndQuoteAndSentimentAndPolicy(baseDir, symbol, comp, quote, nil, nil)
}

// RunAnalysisWithComparablesAndQuoteAndSentiment 执行完整的18步分析，集成可比公司横向对比、实时行情与舆情情绪
func RunAnalysisWithComparablesAndQuoteAndSentiment(baseDir, symbol string, comp *ComparableAnalysis, quote *QuoteData, sentiment *SentimentData) (*AnalysisReport, error) {
	return RunAnalysisWithComparablesAndQuoteAndSentimentAndPolicy(baseDir, symbol, comp, quote, sentiment, nil)
}

// RunAnalysisWithComparablesAndQuoteAndSentimentAndPolicy 执行完整的18步分析，集成可比公司横向对比、实时行情、舆情情绪与政策匹配度
func RunAnalysisWithComparablesAndQuoteAndSentimentAndPolicy(baseDir, symbol string, comp *ComparableAnalysis, quote *QuoteData, sentiment *SentimentData, policy *PolicyMatchData) (*AnalysisReport, error) {
	return RunAnalysisWithAll(baseDir, symbol, comp, quote, sentiment, policy, nil, nil)
}

// RunAnalysisWithAll 执行完整的18步分析，集成所有附加模块（可比公司、行情、舆情、政策、技术形态、交易活跃度）
func RunAnalysisWithAll(baseDir, symbol string, comp *ComparableAnalysis, quote *QuoteData, sentiment *SentimentData, policy *PolicyMatchData, technical *TechnicalData, activity *ActivityData) (*AnalysisReport, error) {
	data, err := LoadFinancialData(baseDir, symbol)
	if err != nil {
		return nil, fmt.Errorf("load financial data: %w", err)
	}
	if len(data.Years) == 0 {
		return nil, fmt.Errorf("no financial data available for %s", symbol)
	}

	steps := []StepResult{
		step1Audit(data),
		step2AssetScale(data),
		step3Solvency(data),
		step4CompetitivePosition(data),
		step5Receivables(data),
		step6FixedAssets(data),
		step7InvestmentAssets(data),
		step8MScore(data),
		step9RevenueGrowth(data),
		step10GrossMargin(data),
		step11OperationEfficiency(data),
		step12CostControl(data),
		step13ExpenseRatio(data),
		step14CoreProfit(data),
		step15CashFlowQuality(data),
		step16ROE(data),
		step17CAPEX(data),
		step18Dividend(data),
	}

	scores := Evaluate(data, steps)

	passSummary := make(map[string][]PassItem)
	for _, step := range steps {
		for _, year := range data.Years {
			p, ok := step.Pass[year]
			var val any
			if yd, ok2 := step.YearlyData[year]; ok2 {
				// 尝试找一个数值型 key 作为展示值
				for k, v := range yd {
					if k != "status" && k != "competitiveness" && k != "risk" && k != "companyType" && k != "focus" && k != "control" && k != "innovation" && k != "salesDifficulty" && k != "profitability" && k != "quality" && k != "assessment" && k != "sustainability" && k != "note" && k != "fraudRisk" {
						val = v
						break
					}
				}
			}
			if !ok {
				p = true
			}
			passSummary[year] = append(passSummary[year], PassItem{
				Year:   year,
				Passed: p,
				Value:  val,
			})
		}
	}

	scoreMap := make(map[string]float64)
	overallGrade := ""
	if len(data.Years) > 0 {
		latest := data.Years[0]
		if s, ok := scores[latest]; ok {
			scoreMap[latest] = s.RawScore
			overallGrade = s.Grade
		}
	}

	md := GenerateMarkdown(symbol, data.Years, steps, scores, comp, quote, sentiment, policy, technical, activity)

	report := &AnalysisReport{
		Symbol:          symbol,
		CompanyName:     symbol,
		Years:           data.Years,
		StepResults:     steps,
		PassSummary:     passSummary,
		Score:           scoreMap,
		OverallGrade:    overallGrade,
		MarkdownContent: md,
	}
	return report, nil
}
