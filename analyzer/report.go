package analyzer

import (
	"fmt"
	"math"
	"strings"
)

// GenerateMarkdown 生成增强版 Markdown 格式的投资分析报告（14模块标准框架）
func GenerateMarkdown(symbol string, years []string, steps []StepResult, scores map[string]*YearScore, comp *ComparableAnalysis, industry *IndustryComparison, quote *QuoteData, sentiment *SentimentData, policy *PolicyMatchData, technical *TechnicalData, activity *ActivityData, ml *MLPredictionData, rim *RIMData) string {
	if len(years) == 0 {
		return "# 无数据\n\n未找到可用的财务数据。"
	}
	latest := years[0]
	prev := ""
	if len(years) > 1 {
		prev = years[1]
	}
	latestScore := scores[latest]

	var b strings.Builder

	// ==================== 封面 ====================
	b.WriteString(fmt.Sprintf("# %s 深度投资分析报告\n\n", symbol))
	b.WriteString(fmt.Sprintf("**股票代码**: %s  \n", symbol))
	b.WriteString("**分析框架**: 财报透视分析框架  \n")
	b.WriteString(fmt.Sprintf("**最新报告期**: %s  \n", latest))
	b.WriteString(fmt.Sprintf("**数据基础**: 基于 %d 年财务报表数据（%s ~ %s）\n\n", len(years), latest, years[len(years)-1]))
	b.WriteString("---\n\n")

	// ==================== 重大风险提示 ====================
	writeMajorRisks(&b, symbol, steps, latest, prev, latestScore)

	// ==================== 模块1: 执行摘要 ====================
	writeModule1(&b, symbol, steps, latest, prev, latestScore, quote, technical, activity, ml)

	// ==================== 模块2: 换手率深度分析 ====================
	writeModule2(&b, quote)

	// ==================== 模块3: 公司基本面分析 ====================
	writeModule3(&b, steps, years, latest, prev)

	// ==================== 模块4: 行业横向对比分析 ====================
	activityScore := -1.0
	if activity != nil {
		activityScore = activity.Score
	}
	writeModule4(&b, steps, latest, comp, industry, activityScore)

	// ==================== 模块5: 十五五政策匹配度评估 ====================
	writeModule5(&b, policy)

	// ==================== 模块6: 实时行情数据 ====================
	writeModule6(&b, quote)

	// ==================== 模块7: 剩余收益模型估值(RIM) ====================
	writeModule7(&b, steps, latest, quote, rim)

	// ==================== 模块8: A-Score 综合风险画像 ====================
	writeAScoreProfile(&b, steps, years, latest, comp)

	// ==================== 模块9: 技术面分析 ====================
	writeModule8(&b, quote, technical, activity)

	// ==================== 模块10: ML机器学习预测 ====================
	writeModule9(&b, steps, latest, prev, ml)

	// ==================== 模块11: 智能选股7大条件 ====================
	writeModule10(&b, steps, latest, prev)

	// ==================== 模块12: 芒格逆向思维检查 ====================
	writeModule11(&b, steps, latest, latestScore)

	// ==================== 模块13: 巴菲特-芒格投资检查清单 ====================
	writeModule12(&b, steps, latest, latestScore)

	// ==================== 模块14: 社交媒体情绪监控 ====================
	writeModule13(&b, sentiment)

	// ==================== 模块15: 综合投资建议 ====================
	writeModule14(&b, symbol, steps, latest, latestScore, quote, rim, technical, ml, sentiment)

	// ==================== 模块16: 结论与附录 ====================
	writeModule15(&b, symbol, steps, years, latest, latestScore, sentiment)

	b.WriteString("---\n\n")
	b.WriteString("*报告生成时间：基于最新导入的财务数据*\n")

	return b.String()
}

// ========== 重大风险提示 ==========
func writeMajorRisks(b *strings.Builder, symbol string, steps []StepResult, latest, prev string, score *YearScore) {
	var risks []string
	ascore := getStepValue(steps, 8, latest, "AScore")
	if ascore >= 60 {
		if ascore >= 70 {
			risks = append(risks, fmt.Sprintf("**A-Score 偏高**: %.1f，综合财务风险较高 🔴**", ascore))
		} else {
			risks = append(risks, fmt.Sprintf("**A-Score 中等**: %.1f，需关注财务健康度 ⚠️**", ascore))
		}
	}
	growth := getStepValue(steps, 9, latest, "growthRate")
	if growth < 0 {
		risks = append(risks, fmt.Sprintf("**营收负增长**: %.2f%%，成长性承压 ❌**", growth))
	}
	pg := getStepValue(steps, 16, latest, "profitGrowth")
	if pg < -20 {
		risks = append(risks, fmt.Sprintf("**净利润大幅下滑**: %.2f%% ❌**", pg))
	} else if pg < 0 {
		risks = append(risks, fmt.Sprintf("**净利润负增长**: %.2f%% ❌**", pg))
	}
	roe := getStepValue(steps, 16, latest, "roe")
	if roe < 10 {
		risks = append(risks, fmt.Sprintf("**ROE 偏低**: %.2f%%，资本回报率不足 ❌**", roe))
	}
	gm := getStepValue(steps, 10, latest, "grossMargin")
	if gm < 20 {
		risks = append(risks, fmt.Sprintf("**毛利率过低**: %.2f%%，产品竞争力弱 ❌**", gm))
	}
	dr := getStepValue(steps, 3, latest, "debtRatio")
	if dr > 60 {
		risks = append(risks, fmt.Sprintf("**负债率过高**: %.2f%%，偿债压力大 🔴**", dr))
	}
	cashRatio := getStepValue(steps, 15, latest, "cashRatio")
	if cashRatio < 50 && cashRatio != 0 {
		risks = append(risks, fmt.Sprintf("**现金流质量偏弱**: 净利润现金含量 %.2f%% ⚠️**", cashRatio))
	}

	if len(risks) > 0 || (score != nil && score.RawScore < 70) {
		b.WriteString("# ⚠️ 重大风险提示\n\n")
		for _, r := range risks {
			b.WriteString(fmt.Sprintf("- %s\n", r))
		}
		if score != nil && score.RawScore < 70 {
			b.WriteString(fmt.Sprintf("- **综合评分偏低**: %.0f分（%s），财务健康度需关注\n", score.RawScore, score.Grade))
		}
		b.WriteString("\n---\n\n")
	}
}

// ========== 目录 ==========
func writeTOC(b *strings.Builder) {
	b.WriteString("# 目录\n\n")
	b.WriteString("- [模块1: 执行摘要](#模块1-执行摘要)\n")
	b.WriteString("- [模块2: 换手率深度分析](#模块2-换手率深度分析)\n")
	b.WriteString("- [模块3: 公司基本面分析](#模块3-公司基本面分析)\n")
	b.WriteString("- [模块4: 行业横向对比分析](#模块4-行业横向对比分析)\n")
	b.WriteString("- [模块5: 十五五政策匹配度评估](#模块5-十五五政策匹配度评估)\n")
	b.WriteString("- [模块6: 实时行情数据](#模块6-实时行情数据)\n")
	b.WriteString("- [模块7: 剩余收益模型估值(RIM)](#模块7-剩余收益模型估值rim)\n")
	b.WriteString("- [模块8: A-Score 综合风险画像](#模块8-a-score-综合风险画像)\n")
	b.WriteString("- [模块9: 技术面分析](#模块9-技术面分析)\n")
	b.WriteString("- [模块10: ML机器学习预测](#模块10-ml机器学习预测)\n")
	b.WriteString("- [模块11: 智能选股7大条件](#模块11-智能选股7大条件)\n")
	b.WriteString("- [模块12: 芒格逆向思维检查](#模块12-芒格逆向思维检查)\n")
	b.WriteString("- [模块13: 巴菲特-芒格投资检查清单](#模块13-巴菲特-芒格投资检查清单)\n")
	b.WriteString("- [模块14: 社交媒体情绪监控](#模块14-社交媒体情绪监控)\n")
	b.WriteString("- [模块15: 综合投资建议](#模块15-综合投资建议)\n")
	b.WriteString("- [模块16: 结论与附录](#模块16-结论与附录)\n")
	b.WriteString("\n---\n\n")
}

// ========== 8项核心指标高亮 ==========
func writeEightIndicatorsHighlight(b *strings.Builder, steps []StepResult, latest string) {
	indicators := []struct {
		name     string
		value    float64
		unit     string
		passed   bool
		operator string
		threshold float64
	}{
		{"ROE", getStepValue(steps, 16, latest, "roe"), "%", getStepValue(steps, 16, latest, "roe") > 20, ">", 20},
		{"净利润现金比率", getStepValue(steps, 15, latest, "cashRatio"), "%", getStepValue(steps, 15, latest, "cashRatio") > 100, ">", 100},
		{"资产负债率", getStepValue(steps, 3, latest, "debtRatio"), "%", getStepValue(steps, 3, latest, "debtRatio") < 60, "<", 60},
		{"毛利率", getStepValue(steps, 10, latest, "grossMargin"), "%", getStepValue(steps, 10, latest, "grossMargin") > 40, ">", 40},
		{"营业利润率", getStepValue(steps, 14, latest, "coreProfitMargin"), "%", getStepValue(steps, 14, latest, "coreProfitMargin") > 20, ">", 20},
		{"营业收入增长率", getStepValue(steps, 9, latest, "growthRate"), "%", getStepValue(steps, 9, latest, "growthRate") > 10, ">", 10},
		{"固定资产比率", getStepValue(steps, 6, latest, "ratio"), "%", getStepValue(steps, 6, latest, "ratio") < 40, "<", 40},
		{"分红率", getStepValue(steps, 18, latest, "ratio"), "%", getStepValue(steps, 18, latest, "ratio") > 30, ">", 30},
	}

	matchCount := 0
	for _, ind := range indicators {
		if ind.passed {
			matchCount++
		}
	}

	b.WriteString("## 核心指标一览\n\n")
	if matchCount >= 5 {
		b.WriteString("> 🏆 **核心指标亮点**：该企业 8 项核心财务指标中满足 **" + fmt.Sprintf("%d", matchCount) + " 项**，表现优异，值得重点关注。\n\n")
		b.WriteString("> **达标指标**：\n")
		for _, ind := range indicators {
			if ind.passed {
				b.WriteString(fmt.Sprintf("> - ✅ **%s**：%.2f%s（%s %.0f%s）\n", ind.name, ind.value, ind.unit, ind.operator, ind.threshold, ind.unit))
			}
		}
		if matchCount < 8 {
			b.WriteString("> \n")
			b.WriteString("> **未达标指标**：\n")
			for _, ind := range indicators {
				if !ind.passed {
					b.WriteString(fmt.Sprintf("> - ⚠️ **%s**：%.2f%s（%s %.0f%s）\n", ind.name, ind.value, ind.unit, ind.operator, ind.threshold, ind.unit))
				}
			}
		}
		b.WriteString("\n")
	} else {
		b.WriteString("| 指标 | 数值 | 阈值 | 是否达标 |\n")
		b.WriteString("|------|------|------|----------|\n")
		for _, ind := range indicators {
			status := "❌ 未达标"
			if ind.passed {
				status = "✅ 达标"
			}
			b.WriteString(fmt.Sprintf("| **%s** | %.2f%s | %s %.0f%s | %s |\n", ind.name, ind.value, ind.unit, ind.operator, ind.threshold, ind.unit, status))
		}
		b.WriteString(fmt.Sprintf("\n**达标比例**：%d / 8 项\n\n", matchCount))
	}

	// 添加 A-Score 风险评分
	ascore := getStepValue(steps, 8, latest, "AScore")
	b.WriteString("### A-Score 风险评分\n\n")
	b.WriteString(fmt.Sprintf("| 指标 | 数值 | 风险等级 | 说明 |\n"))
	b.WriteString("|------|------|----------|------|\n")
	if ascore > 0 {
		badge := ascoreBadge(ascore)
		comment := ascoreBrief(ascore)
		b.WriteString(fmt.Sprintf("| **A-Score** | **%.1f** | %s | %s |\n", ascore, badge, comment))
	} else {
		b.WriteString("| **A-Score** | — | — | 数据不足，无法计算 |\n")
	}
	b.WriteString("\n")
}

// ========== 模块1: 执行摘要 ==========
func writeModule1(b *strings.Builder, symbol string, steps []StepResult, latest, prev string, score *YearScore, quote *QuoteData, technical *TechnicalData, activity *ActivityData, ml *MLPredictionData) {
	b.WriteString("# 模块1: 执行摘要\n\n")

	writeEightIndicatorsHighlight(b, steps, latest)

	b.WriteString("## 1.1 多维度评分汇总\n\n")
	b.WriteString("| 评估维度 | 评级 | 得分 | 关键结论 |\n")
	b.WriteString("|----------|------|------|----------|\n")
	if score != nil {
		b.WriteString(fmt.Sprintf("| **财务健康度综合评分** | %s | **%.0f/100** | %s |\n", scoreToStars(score.RawScore), score.RawScore, gradeComment(score.RawScore)))
	}
	b.WriteString(fmt.Sprintf("| **成长能力** | %s | %.0f/100 | %s |\n", scoreToStars(growthScore(steps, latest)), growthScore(steps, latest), growthComment(steps, latest)))
	b.WriteString(fmt.Sprintf("| **盈利能力** | %s | %.0f/100 | %s |\n", scoreToStars(profitScore(steps, latest)), profitScore(steps, latest), profitComment(steps, latest)))
	b.WriteString(fmt.Sprintf("| **现金流质量** | %s | %.0f/100 | %s |\n", scoreToStars(cashScore(steps, latest)), cashScore(steps, latest), cashComment(steps, latest)))
	b.WriteString(fmt.Sprintf("| **偿债安全** | %s | %.0f/100 | %s |\n", scoreToStars(debtScore(steps, latest)), debtScore(steps, latest), debtComment(steps, latest)))
	vs := valuationScore(quote)
	vc := valuationComment(quote)
	if quote != nil && (quote.PE > 0 || quote.PB > 0) {
		b.WriteString(fmt.Sprintf("| **实时估值** | %s | %.0f/100 | %s |\n", scoreToStars(vs), vs, vc))
	} else {
		b.WriteString(fmt.Sprintf("| **实时估值** | - | -/100 | %s |\n", vc))
	}
	if technical != nil && technical.Score > 0 {
		b.WriteString(fmt.Sprintf("| **技术形态** | %s | %.0f/100 | %s |\n", scoreToStars(technical.Score), technical.Score, technical.Comment))
	} else {
		b.WriteString(fmt.Sprintf("| **技术形态** | - | -/100 | 待接入技术分析数据 |\n"))
	}
	if activity != nil && activity.Score > 0 {
		b.WriteString(fmt.Sprintf("| **交易活跃度** | %s | %.0f/100 | %s |\n", formatActivityStars(activity.Stars), activity.Score, activity.Comment))
	} else {
		b.WriteString(fmt.Sprintf("| **交易活跃度** | - | -/100 | 数据不足 |\n"))
	}
	ascore := getStepValue(steps, 8, latest, "AScore")
	b.WriteString(fmt.Sprintf("| **A-Score风险** %s | %s | %.0f/100 | %s |\n", infoIcon("a-score"), asEmoji(ascore), ascore, ascoreComment(ascore)))
	if ml != nil && ml.Summary != nil && ml.Summary.HasData {
		sum := ml.Summary
		var predText string
		if sum.RangeHigh > 0 && sum.RangeLow >= 0 {
			predText = fmt.Sprintf("%s +%.0f%%~+%.0f%%", mlDirectionCN(sum.Direction), sum.RangeLow, sum.RangeHigh)
		} else if sum.RangeHigh <= 0 && sum.RangeLow < 0 {
			predText = fmt.Sprintf("%s %.0f%%~%.0f%%", mlDirectionCN(sum.Direction), math.Abs(sum.RangeHigh), math.Abs(sum.RangeLow))
		} else {
			predText = fmt.Sprintf("%s %.0f%%~+%.0f%%", mlDirectionCN(sum.Direction), sum.RangeLow, sum.RangeHigh)
		}
		b.WriteString(fmt.Sprintf("| **ML预测** | - | -/30 | %s |\n", predText))
	} else {
		b.WriteString(fmt.Sprintf("| **ML预测** | - | -/30 | 待接入机器学习模型 |\n"))
	}
	b.WriteString(fmt.Sprintf("| **逆向检查** | %s | %.0f/100 | %s |\n", scoreToStars(reverseScore(steps, latest, score)), reverseScore(steps, latest, score), reverseComment(steps, latest, score)))
	b.WriteString(fmt.Sprintf("| **巴芒清单** | %s | %.1f/10 | %s |\n", scoreToStars(buffettScore(steps, latest, score)*10), buffettScore(steps, latest, score), buffettComment(steps, latest, score)))
	if score != nil {
		weighted := score.RawScore*0.30 + profitScore(steps, latest)*0.25 + cashScore(steps, latest)*0.20 + growthScore(steps, latest)*0.15 + debtScore(steps, latest)*0.10
		b.WriteString(fmt.Sprintf("| **综合建议** | **%s** | **%.0f/100** | %s |\n", investmentGrade(weighted), weighted, strategyAdvice(weighted)))
	}
	b.WriteString("\n")
	b.WriteString("\n")

	b.WriteString("## 1.2 综合评级与建议\n\n")
	if score != nil {
		weighted := score.RawScore*0.30 + profitScore(steps, latest)*0.25 + cashScore(steps, latest)*0.20 + growthScore(steps, latest)*0.15 + debtScore(steps, latest)*0.10
		b.WriteString(fmt.Sprintf("**综合评分**: %.0f/100 %s  \n", weighted, scoreToStars(weighted)))
		b.WriteString(fmt.Sprintf("**投资评级**: **%s**  \n", investmentGrade(weighted)))
		b.WriteString(fmt.Sprintf("**建议仓位**: %s  \n", positionAdvice(weighted)))
		b.WriteString(fmt.Sprintf("**操作策略**: %s  \n\n", strategyAdvice(weighted)))
		b.WriteString("> **一句话建议**: ")
		b.WriteString(oneSentenceAdvice(symbol, weighted, steps, latest))
		b.WriteString("\n\n")
	}
	b.WriteString("---\n\n")
}

// ========== 模块2: 换手率深度分析 ==========
func writeModule2(b *strings.Builder, quote *QuoteData) {
	b.WriteString("# 模块2: 换手率深度分析\n\n")

	if quote == nil || quote.TurnoverRate == 0 {
		b.WriteString("> **说明**: 当前暂无实时换手率数据。请在网络畅通时重新选中股票获取行情。\n\n")
		b.WriteString("---\n\n")
		return
	}

	tr := quote.TurnoverRate
	vol := quote.Volume
	toa := quote.TurnoverAmount
	vr := quote.VolumeRatio

	b.WriteString("## 2.1 实时换手与成交指标\n\n")
	b.WriteString("| 指标 | 数值 | 评估 |\n")
	b.WriteString("|------|------|------|\n")
	b.WriteString(fmt.Sprintf("| **换手率** | %.2f%% | %s |\n", tr, turnoverAssessment(tr)))
	b.WriteString(fmt.Sprintf("| **成交量** | %.0f 手 | - |\n", vol))
	b.WriteString(fmt.Sprintf("| **成交额** | %.0f 万元 | - |\n", toa/10000))
	if vr > 0 {
		b.WriteString(fmt.Sprintf("| **量比** | %.2f | %s |\n", vr, volumeRatioAssessment(vr)))
	}
	b.WriteString("\n")

	b.WriteString("## 2.2 流动性评级\n\n")
	if tr < 1 {
		b.WriteString("- **流动性偏低**：换手率低于 1%，交投清淡，大额买卖可能对价格产生较大冲击。\n")
	} else if tr < 3 {
		b.WriteString("- **流动性正常**：换手率在 1%~3% 区间，交易活跃度适中，流动性风险可控。\n")
	} else if tr < 7 {
		b.WriteString("- **流动性活跃**：换手率在 3%~7% 区间，市场关注度较高，买卖盘相对充裕。\n")
	} else {
		b.WriteString("- **流动性非常活跃**：换手率超过 7%，交投极度活跃，需警惕短期波动放大。\n")
	}
	b.WriteString("\n---\n\n")
}

// ========== 模块3: 公司基本面分析 ==========
func writeModule3(b *strings.Builder, steps []StepResult, years []string, latest, prev string) {
	b.WriteString("# 模块3: 公司基本面分析\n\n")

	b.WriteString("## 3.1 全年核心财务数据" + traceTrigger(3,9,10,15,16) + "\n\n")
	b.WriteString(fmt.Sprintf("| 指标 | %s | %s | 同比 | 评估 |\n", latest, prev))
	b.WriteString("|------|--------|------|------|------|\n")
	writeMetricRow(b, "营业收入", getStepValue(steps, 9, latest, "revenue"), getStepValue(steps, 9, prev, "revenue"), "亿元", 1e8)
	writeMetricRow(b, "归母净利润", getStepValue(steps, 16, latest, "profit"), getStepValue(steps, 16, prev, "profit"), "亿元", 1e8)
	writeMetricRow(b, "ROE", getStepValue(steps, 16, latest, "roe"), getStepValue(steps, 16, prev, "roe"), "%", 1)
	writeMetricRow(b, "毛利率", getStepValue(steps, 10, latest, "grossMargin"), getStepValue(steps, 10, prev, "grossMargin"), "%", 1)
	writeMetricRow(b, "资产负债率", getStepValue(steps, 3, latest, "debtRatio"), getStepValue(steps, 3, prev, "debtRatio"), "%", 1)
	writeMetricRow(b, "经营现金流净额", getStepValue(steps, 15, latest, "operatingCF"), getStepValue(steps, 15, prev, "operatingCF"), "亿元", 1e8)
	b.WriteString("\n")

	b.WriteString("## 3.2 核心财务指标趋势（近5年）" + traceTrigger(3,9,10,15,16) + "\n\n")
	b.WriteString("| 年度 | ROE | 毛利率 | 资产负债率 | 营收增长率 | 净利润现金含量 | M-Score |\n")
	b.WriteString("|------|-----|--------|------------|------------|----------------|---------|\n")
	for i := 0; i < len(years) && i < 5; i++ {
		year := years[i]
		b.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s | %s | %s |\n",
			year,
			fmtVal(getStepValue(steps, 16, year, "roe"), "%"),
			fmtVal(getStepValue(steps, 10, year, "grossMargin"), "%"),
			fmtVal(getStepValue(steps, 3, year, "debtRatio"), "%"),
			fmtVal(getStepValue(steps, 9, year, "growthRate"), "%"),
			fmtVal(getStepValue(steps, 15, year, "cashRatio"), "%"),
			fmtVal(getStepValue(steps, 8, year, "MScore"), ""),
		))
	}
	b.WriteString("\n")
	b.WriteString("> **解读**: 持续观察ROE和毛利率的趋势变化，若连续下滑需警惕竞争力衰退；资产负债率稳定或下降为加分项。M-Score已纳入A-Score综合风险体系，A-Score≥60时建议重点核查财报真实性与偿债能力。\n\n")

	b.WriteString(fmt.Sprintf("## 3.3 财务指标逐项解读（%s）\n\n", latest))
	categories := []struct {
		name  string
		steps []int
	}{
		{"会计与资产质量", []int{2, 5, 6, 7, 8}},
		{"偿债与营运安全", []int{3, 4, 11, 17}},
		{"盈利能力", []int{10, 12, 13, 14, 16}},
		{"现金流与分红", []int{15, 18}},
		{"成长能力", []int{9}},
	}
	b.WriteString("| 维度 | 达标数/总数 | 状态 |\n")
	b.WriteString("|------|-------------|------|\n")
	for _, cat := range categories {
		pass, total := countCategoryPass(steps, cat.steps, latest)
		status := "🟢 健康"
		if pass < total {
			if float64(pass)/float64(total) < 0.6 {
				status = "🔴 偏弱"
			} else {
				status = "🟡 一般"
			}
		}
		b.WriteString(fmt.Sprintf("| %s | %d/%d | %s |\n", cat.name, pass, total, status))
	}
	b.WriteString("\n")

	b.WriteString("## 3.4 核心风险点\n\n")
	risks := extractRisks(steps, latest)
	if len(risks) == 0 {
		b.WriteString("未发现重大风险，财务整体可控。\n")
	} else {
		b.WriteString("| 风险类别 | 风险描述 | 严重程度 |\n")
		b.WriteString("|----------|----------|----------|\n")
		for _, r := range risks {
			b.WriteString(fmt.Sprintf("| %s | %s | %s |\n", r.Category, r.Indicator, r.Severity))
		}
	}
	b.WriteString("\n---\n\n")
}

// ========== 模块4: 行业横向对比分析 ==========
func writeModule4(b *strings.Builder, steps []StepResult, latest string, comp *ComparableAnalysis, industry *IndustryComparison, activityScore float64) {
	b.WriteString("# 模块4: 行业横向对比分析\n\n")

	// 行业均值对比（新增）
	if industry != nil && industry.HasData {
		b.WriteString("## 4.0 行业均值对比\n\n")
		b.WriteString("| 指标 | 当前公司 | 行业均值 | 差异 | 说明 |\n")
		b.WriteString("|------|----------|----------|------|------|\n")
		
		roe := getStepValue(steps, 16, latest, "roe")
		gm := getStepValue(steps, 10, latest, "grossMargin")
		growth := getStepValue(steps, 9, latest, "growthRate")
		debt := getStepValue(steps, 3, latest, "debtRatio")
		
		roeEmoji := "➡️"
		if industry.ROEPercentile >= 75 {
			roeEmoji = "🟢"
		} else if industry.ROEPercentile <= 25 {
			roeEmoji = "🔴"
		}
		
		b.WriteString(fmt.Sprintf("| **ROE** | %.2f%% | %s | %+.2f%% | %s 行业百分位: %.0f%% |\n",
			roe, industry.Industry, industry.GMDiff, roeEmoji, industry.ROEPercentile))
		b.WriteString(fmt.Sprintf("| **毛利率** | %.2f%% | 行业均值 | %+.2f%% | %s |\n",
			gm, industry.GMDiff, getDiffEmoji(industry.GMDiff)))
		b.WriteString(fmt.Sprintf("| **营收增长** | %.2f%% | 行业均值 | %+.2f%% | %s |\n",
			growth, industry.GrowthDiff, getDiffEmoji(industry.GrowthDiff)))
		b.WriteString(fmt.Sprintf("| **负债率** | %.2f%% | 行业均值 | %+.2f%% | %s |\n",
			debt, industry.DebtDiff, getDiffEmoji(-industry.DebtDiff)))
		b.WriteString("\n")
		
		b.WriteString(fmt.Sprintf("> **行业对比总结**: %s\n\n", industry.Summary))
	}

	if comp == nil || !comp.HasData || len(comp.Metrics) == 0 {
		b.WriteString("> **说明**: 当前未配置可比公司，或可比公司数据尚未下载。请在股票详情页的\"可比公司\"面板中添加 3~5 家对标公司并下载其财报数据。\n\n")
		b.WriteString("---\n\n")
		return
	}

	target := &ComparableMetrics{
		Symbol:        "当前公司",
		ROE:           getStepValue(steps, 16, latest, "roe"),
		GrossMargin:   getStepValue(steps, 10, latest, "grossMargin"),
		RevenueGrowth: getStepValue(steps, 9, latest, "growthRate"),
		DebtRatio:     getStepValue(steps, 3, latest, "debtRatio"),
		CashRatio:     getStepValue(steps, 15, latest, "cashRatio"),
		AScore:        getStepValue(steps, 8, latest, "AScore"),
		ActivityScore: activityScore,
	}

	b.WriteString(fmt.Sprintf("## 4.1 可比公司关键指标对比（%s）", latest) + traceTrigger(3,9,10,15,16) + "\n\n")
	b.WriteString("| 指标 | 当前公司 | 可比均值 | 最高 | 最低 | 排名百分位 |\n")
	b.WriteString("|------|----------|----------|------|------|------------|\n")
	b.WriteString(fmt.Sprintf("| **ROE** | %.2f%% | %.2f%% | %.2f%% | %.2f%% | %.0f%% |\n",
		target.ROE, comp.Average.ROE, comp.Max.ROE, comp.Min.ROE, RankPercentile(comp.Metrics, target, "roe")))
	b.WriteString(fmt.Sprintf("| **毛利率** | %.2f%% | %.2f%% | %.2f%% | %.2f%% | %.0f%% |\n",
		target.GrossMargin, comp.Average.GrossMargin, comp.Max.GrossMargin, comp.Min.GrossMargin, RankPercentile(comp.Metrics, target, "grossMargin")))
	b.WriteString(fmt.Sprintf("| **营收增长率** | %.2f%% | %.2f%% | %.2f%% | %.2f%% | %.0f%% |\n",
		target.RevenueGrowth, comp.Average.RevenueGrowth, comp.Max.RevenueGrowth, comp.Min.RevenueGrowth, RankPercentile(comp.Metrics, target, "revenueGrowth")))
	b.WriteString(fmt.Sprintf("| **资产负债率** | %.2f%% | %.2f%% | %.2f%% | %.2f%% | %.0f%% |\n",
		target.DebtRatio, comp.Average.DebtRatio, comp.Max.DebtRatio, comp.Min.DebtRatio, RankPercentile(comp.Metrics, target, "debtRatio")))
	b.WriteString(fmt.Sprintf("| **净利润现金含量** | %.2f%% | %.2f%% | %.2f%% | %.2f%% | %.0f%% |\n",
		target.CashRatio, comp.Average.CashRatio, comp.Max.CashRatio, comp.Min.CashRatio, RankPercentile(comp.Metrics, target, "cashRatio")))
	b.WriteString(fmt.Sprintf("| **A-Score** | %.1f | %.1f | %.1f | %.1f | %.0f%% |\n",
		target.AScore, comp.Average.AScore, comp.Max.AScore, comp.Min.AScore, RankPercentile(comp.Metrics, target, "aScore")))
	if target.ActivityScore >= 0 || comp.Average.ActivityScore >= 0 {
		avgAct := comp.Average.ActivityScore
		if avgAct < 0 { avgAct = 0 }
		b.WriteString(fmt.Sprintf("| **活跃度** | %.0f | %.0f | %.0f | %.0f | %.0f%% |\n",
			math.Max(0, target.ActivityScore), avgAct, math.Max(0, comp.Max.ActivityScore), math.Max(0, comp.Min.ActivityScore), RankPercentile(comp.Metrics, target, "activityScore")))
	}
	b.WriteString("\n")

	// 4.2 可比公司明细（含加权评分、排序、蓝色亮条）
	all := make([]*ComparableMetrics, 0, len(comp.Metrics)+1)
	all = append(all, target)
	for _, m := range comp.Metrics {
		all = append(all, m)
	}

	// 计算缺失活跃度的中位数替代值
	medianActivity := medianActivityScore(all)

	// 按综合得分排序
	type scored struct {
		*ComparableMetrics
		score float64
		rank  int
	}
	scoredList := make([]scored, 0, len(all))
	for _, m := range all {
		scoredList = append(scoredList, scored{
			ComparableMetrics: m,
			score:             calcComparableScore(m, all, medianActivity),
		})
	}
	// 降序
	for i := 0; i < len(scoredList); i++ {
		for j := i + 1; j < len(scoredList); j++ {
			if scoredList[i].score < scoredList[j].score {
				scoredList[i], scoredList[j] = scoredList[j], scoredList[i]
			}
		}
	}
	for i := range scoredList {
		scoredList[i].rank = i + 1
	}

	// 找到当前公司的排名
	targetRank := 1
	for _, s := range scoredList {
		if s.Symbol == "当前公司" {
			targetRank = s.rank
			break
		}
	}
	total := len(scoredList)
	percentile := float64(targetRank-1) / float64(total) * 100 // 越小越好

	var advice string
	if targetRank == 1 {
		advice = "当前公司综合评分最高，建议重点关注/持有 🥇"
	} else if percentile < 30 {
		advice = "当前公司质地优良，建议持有"
	} else if percentile < 60 {
		advice = "当前公司表现中等，可继续持有观察"
	} else {
		advice = "当前公司相对可比公司存在明显短板，建议谨慎"
	}


	// 检查是否有缺失活跃度的可比公司
	hasMissingActivity := false
	for _, s := range scoredList {
		if s.ActivityScore < 0 {
			hasMissingActivity = true
			break
		}
	}

	b.WriteString("## 4.2 可比公司明细")
	b.WriteString(`<details style="display:inline-block;position:relative;margin-left:8px;vertical-align:middle;">`)
	b.WriteString(`<summary style="cursor:pointer;list-style:none;color:#1890ff;font-size:14px;">ℹ️</summary>`)
	b.WriteString(`<div style="position:absolute;left:28px;top:-8px;width:360px;background:#fff;border:1px solid #d9d9d9;border-radius:8px;padding:12px;box-shadow:0 4px 12px rgba(0,0,0,0.15);z-index:100;font-size:13px;line-height:1.6;color:#333;">`)
	b.WriteString(`<strong>综合得分计算方式</strong><br/>`)
	b.WriteString(`在“当前公司 + 可比公司”池内，对每个指标做 Min-Max 标准化（0~100 分），再按以下权重加权求和：<br/>`)
	b.WriteString(`• ROE 25%　• 毛利率 20%　• 营收增长 15%　• 现金含量 10%<br/>`)
	b.WriteString(`• 负债率 10%（反向，越低越好）<br/>`)
	b.WriteString(`• A-Score 10%（反向，越低越好）<br/>`)
	b.WriteString(`• 活跃度 10%<br/>`)
	b.WriteString(`<em>缺失活跃度时，使用可比池有效样本的中位数替代，标记为 *</em>`)
	b.WriteString(`</div></details>`)
	b.WriteString("\n\n")
	if hasMissingActivity {
		b.WriteString(`<div class="activity-hint" style="margin:6px 0 10px;font-size:12px;color:#94a3b8;">`)
		b.WriteString(`<span>部分可比公司活跃度使用样本中位数替代，</span>`)
		b.WriteString(`<span class="fetch-activity-trigger" style="cursor:pointer;color:#3b82f6;text-decoration:underline;" title="获取缺失的真实活跃度数据">[获取真实活跃度]</span>`)
		b.WriteString(`</div>`)
		b.WriteString("\n\n")
	}
	b.WriteString("| 排名 | 公司 | ROE | 毛利率 | 营收增长 | 负债率 | 现金含量 | A-Score | 活跃度 | 综合得分 |\n")
	b.WriteString("|------|------|-----|--------|----------|--------|----------|---------|--------|----------|\n")
	for _, s := range scoredList {
		displayName := s.Name
		if displayName == "" {
			displayName = s.Symbol
		}
		if s.Symbol == "当前公司" {
			displayName = "**当前公司**"
		}
		actStr := "-"
		if s.ActivityScore >= 0 {
			actStr = fmt.Sprintf("%.0f", s.ActivityScore)
		} else if medianActivity > 0 {
			actStr = fmt.Sprintf("%.0f*", medianActivity)
		}
		b.WriteString(fmt.Sprintf("| %d | %s | %.2f%% | %.2f%% | %.2f%% | %.2f%% | %.2f%% | %.1f | %s | %.1f |\n",
			s.rank, displayName, s.ROE, s.GrossMargin, s.RevenueGrowth, s.DebtRatio, s.CashRatio, s.AScore, actStr, s.score))
	}
	avgActStr := "-"
	if comp.Average.ActivityScore >= 0 {
		avgActStr = fmt.Sprintf("%.0f", comp.Average.ActivityScore)
	}
	b.WriteString(fmt.Sprintf("| — | **平均值** | %.2f%% | %.2f%% | %.2f%% | %.2f%% | %.2f%% | %.1f | %s | — |\n",
		comp.Average.ROE, comp.Average.GrossMargin, comp.Average.RevenueGrowth, comp.Average.DebtRatio, comp.Average.CashRatio, comp.Average.AScore, avgActStr))
	b.WriteString("\n")

	b.WriteString("> **解读**: 排名百分位表示当前公司在可比公司中的相对位置（越高越好，负债率与 A-Score 为反向指标）。活跃度带 `*` 表示使用样本中位数替代（该可比公司暂无本地缓存数据）。综合得分基于 ROE(25%)、毛利率(20%)、营收增长(15%)、现金含量(10%)、负债率(10%)、A-Score(10%)、活跃度(10%) 加权计算。\n\n")

	b.WriteString(fmt.Sprintf("> **💡 综合评分排名**（满分100）\n"))
	b.WriteString(fmt.Sprintf("> 当前公司在 **%d** 家可比公司中排名第 **%d**，综合得分 **%.1f**。\n", total, targetRank, scoredList[targetRank-1].score))
	b.WriteString(fmt.Sprintf("> %s\n\n", advice))
	// 多年度趋势对比
	if len(comp.YearlyTrends) >= 2 && len(comp.CommonYears) >= 2 {
		b.WriteString("## 4.3 多年度趋势对比（当前公司 vs 可比均值）\n\n")
		b.WriteString("| 年份 | ROE(公司/均值) | 毛利率(公司/均值) | 负债率(公司/均值) | 现金含量(公司/均值) |\n")
		b.WriteString("|------|----------------|-------------------|-------------------|---------------------|\n")
		for _, yt := range comp.YearlyTrends {
			ty := getStepValue(steps, 16, yt.Year, "roe")
			tgm := getStepValue(steps, 10, yt.Year, "grossMargin")
			tdr := getStepValue(steps, 3, yt.Year, "debtRatio")
			tcr := getStepValue(steps, 15, yt.Year, "cashRatio")
			b.WriteString(fmt.Sprintf("| **%s** | %.2f%% / %.2f%% | %.2f%% / %.2f%% | %.2f%% / %.2f%% | %.2f%% / %.2f%% |\n",
				yt.Year, ty, yt.Average.ROE, tgm, yt.Average.GrossMargin, tdr, yt.Average.DebtRatio, tcr, yt.Average.CashRatio))
		}
		b.WriteString("\n")

		b.WriteString("### 趋势简评\n\n")
		writeComparableTrendComment(b, steps, comp)
		b.WriteString("\n")
	}

	b.WriteString("---\n\n")
}

// WriteModule4Only 仅生成模块4内容（用于增量更新）
func WriteModule4Only(b *strings.Builder, steps []StepResult, latest string, comp *ComparableAnalysis, industry *IndustryComparison, activityScore float64) {
	writeModule4(b, steps, latest, comp, industry, activityScore)
}

// ========== 模块5: 十五五政策匹配度评估 ==========
func writeModule5(b *strings.Builder, policy *PolicyMatchData) {
	b.WriteString("# 模块5: 十五五政策匹配度评估\n\n")

	if policy == nil || policy.Industry == "" {
		b.WriteString("> **说明**: 当前暂无政策匹配数据。请在网络畅通时重新选中股票获取基本资料。\n\n")
		b.WriteString("---\n\n")
		return
	}

	b.WriteString("## 5.1 政策匹配概览\n\n")
	b.WriteString(fmt.Sprintf("| 评估维度 | 结果 |\n"))
	b.WriteString(fmt.Sprintf("|----------|------|\n"))
	b.WriteString(fmt.Sprintf("| **所属行业** | %s |\n", policy.Industry))
	b.WriteString(fmt.Sprintf("| **匹配评级** | %s |\n", policy.MatchLevel))
	b.WriteString(fmt.Sprintf("| **政策评分** | %d / 100 |\n", policy.Score))
	b.WriteString("\n")

	// 按匹配程度从高到低排序
	sortedPolicies := make([]PolicyItem, len(policy.Policies))
	copy(sortedPolicies, policy.Policies)
	for i := 0; i < len(sortedPolicies)-1; i++ {
		for j := i + 1; j < len(sortedPolicies); j++ {
			if sortedPolicies[i].Level < sortedPolicies[j].Level {
				sortedPolicies[i], sortedPolicies[j] = sortedPolicies[j], sortedPolicies[i]
			}
		}
	}

	b.WriteString("## 5.2 重点政策方向\n\n")
	b.WriteString(`<div class="policy-tags">` + "\n")
	for _, p := range sortedPolicies {
		b.WriteString(fmt.Sprintf(`  <span class="policy-tag"><span class="policy-name">%s</span><span class="policy-signal">%s</span></span>`+"\n", p.Name, policySignalSVG(p.Level)))
	}
	b.WriteString(`</div>` + "\n\n")

	b.WriteString("## 5.3 解读摘要\n\n")
	b.WriteString(fmt.Sprintf("> %s\n\n", policy.Summary))

	b.WriteString("---\n\n")
}

func getDiffEmoji(diff float64) string {
	if diff >= 5 {
		return "🟢 优于行业"
	} else if diff <= -5 {
		return "🔴 低于行业"
	}
	return "➡️ 与行业接近"
}

func policySignalSVG(level int) string {
	if level < 1 {
		level = 1
	}
	if level > 5 {
		level = 5
	}
	color := "#22c55e"
	var rects strings.Builder
	bars := []struct{ x, y, h float64 }{
		{0, 8, 2},
		{3.5, 6, 4},
		{7, 4, 6},
		{10.5, 2, 8},
		{14, 0, 10},
	}
	for i, bar := range bars {
		op := "0.25"
		if i < level {
			op = "1"
		}
		rects.WriteString(fmt.Sprintf(`<rect x="%.1f" y="%.1f" width="2" height="%.1f" fill="%s" opacity="%s" rx="0.3" />`, bar.x, bar.y, bar.h, color, op))
	}
	return fmt.Sprintf(`<svg width="18" height="10" viewBox="0 0 18 10">%s</svg>`, rects.String())
}

// ========== 模块6: 实时行情数据 ==========
func writeModule6(b *strings.Builder, quote *QuoteData) {
	b.WriteString("# 模块6: 实时行情数据\n\n")

	if quote == nil || quote.CurrentPrice == 0 {
		b.WriteString("> **说明**: 当前暂无实时行情数据。请在网络畅通时重新选中股票获取行情。\n\n")
		b.WriteString("---\n\n")
		return
	}

	b.WriteString("## 6.1 实时价格与涨跌\n\n")
	b.WriteString("| 指标 | 数值 |\n")
	b.WriteString("|------|------|\n")
	b.WriteString(fmt.Sprintf("| **最新价** | %.2f 元 |\n", quote.CurrentPrice))
	b.WriteString(fmt.Sprintf("| **涨跌额** | %+.2f 元 |\n", quote.ChangeAmount))
	b.WriteString(fmt.Sprintf("| **涨跌幅** | %+.2f%% |\n", quote.ChangePercent))
	b.WriteString(fmt.Sprintf("| **今日最高** | %.2f 元 |\n", quote.High))
	b.WriteString(fmt.Sprintf("| **今日最低** | %.2f 元 |\n", quote.Low))
	b.WriteString(fmt.Sprintf("| **今开** | %.2f 元 |\n", quote.Open))
	b.WriteString(fmt.Sprintf("| **昨收** | %.2f 元 |\n", quote.PreviousClose))
	b.WriteString(fmt.Sprintf("| **振幅** | %.2f%% |\n", quote.Amplitude))
	b.WriteString("\n")

	b.WriteString("## 6.2 实时估值指标\n\n")
	b.WriteString("| 指标 | 数值 |\n")
	b.WriteString("|------|------|\n")
	if quote.MarketCap > 0 {
		b.WriteString(fmt.Sprintf("| **总市值** | %.2f 亿元 |\n", quote.MarketCap/1e8))
	}
	if quote.CirculatingMarketCap > 0 {
		b.WriteString(fmt.Sprintf("| **流通市值** | %.2f 亿元 |\n", quote.CirculatingMarketCap/1e8))
	}
	if quote.PE > 0 {
		b.WriteString(fmt.Sprintf("| **市盈率(动)** | %.2f |\n", quote.PE))
	}
	if quote.PB > 0 {
		b.WriteString(fmt.Sprintf("| **市净率** | %.2f |\n", quote.PB))
	}
	b.WriteString("\n---\n\n")
}

// ========== 模块7: RIM估值（基于多期预测） ==========
func writeModule7(b *strings.Builder, steps []StepResult, latest string, quote *QuoteData, rim *RIMData) {
	b.WriteString(`<h1 id="模块7-剩余收益模型估值rim">模块7: 剩余收益模型估值(RIM) <button class="rim-adjust-btn" style="float:right;margin-top:2px;">调整RIM</button></h1>` + "\n\n")

	roe := getStepValue(steps, 16, latest, "roe")

	b.WriteString(fmt.Sprintf("## 7.1 模型参数（基于 %s 年报）", latest) + traceTrigger(16) + "\n\n")
	b.WriteString("| <span style=\"white-space:nowrap;\">参数</span> | <span style=\"white-space:nowrap;\">符号</span> | 取值 | 说明 |\n")
	b.WriteString("|------|-----------|------|------|\n")
	b.WriteString(fmt.Sprintf("| <span style=\"white-space:nowrap;\">**ROE**</span> | ROE | %.2f%% | 年报数据 |\n", roe))

	hasRIM := rim != nil && rim.HasData && rim.Result != nil
	var bps0, ke, gTerminal, currentPrice float64

	if hasRIM {
		bps0 = rim.Params.BPS0
		ke = rim.Params.KE
		gTerminal = rim.Params.GTerminal
		currentPrice = rim.Params.CurrentPrice
		b.WriteString(fmt.Sprintf("| <span style=\"white-space:nowrap;\">**每股净资产**</span> | BPS&nbsp; | %.2f元 | %s |\n", bps0, rimSourceDesc(rim, quote)))
		b.WriteString(fmt.Sprintf("| <span style=\"white-space:nowrap;\">**资本成本**</span> | kE&nbsp; | %.2f%% | CAPM(Rf=%.2f%%, Beta=%.2f, Rm-Rf=%.2f%%) |\n", ke*100, rim.Rf*100, rim.Beta, rim.RmRf*100))
		b.WriteString(fmt.Sprintf("| <span style=\"white-space:nowrap;\">**永续增长率**</span> | g&nbsp;&nbsp; | %.1f%% | 稳态假设 |\n", gTerminal*100))
		if currentPrice > 0 {
			b.WriteString(fmt.Sprintf("| <span style=\"white-space:nowrap;\">**当前股价**</span> | P&nbsp;&nbsp; | %.2f元 | 实时行情 |\n", currentPrice))
		} else {
			b.WriteString("| <span style=\"white-space:nowrap;\">**当前股价**</span> | P&nbsp;&nbsp; | - | 待接入实时行情 |\n")
		}
		// 显示预测期 EPS
		if len(rim.Result.Details) > 0 {
			var epsLine string
			for i, d := range rim.Result.Details {
				if i > 0 {
					epsLine += ", "
				}
				epsLine += fmt.Sprintf("第%d年 %.2f", d.Year, d.EPS)
			}
			b.WriteString(fmt.Sprintf("| <span style=\"white-space:nowrap;\">**预测期 EPS**</span> | -&nbsp; | %s | 机构一致预期 |\n", epsLine))
		}
	} else {
		b.WriteString("| <span style=\"white-space:nowrap;\">**每股净资产**</span> | BPS&nbsp; | 待计算 | 需接入实时行情与财报 |\n")
		b.WriteString("| <span style=\"white-space:nowrap;\">**资本成本**</span> | kE&nbsp; | 7.0% | 假设值 |\n")
		b.WriteString("| <span style=\"white-space:nowrap;\">**永续增长率**</span> | g&nbsp;&nbsp; | 3.0% | 假设值 |\n")
		b.WriteString("| <span style=\"white-space:nowrap;\">**当前股价**</span> | P&nbsp;&nbsp; | - | 待接入实时行情 |\n")
	}
	b.WriteString("\n")

	b.WriteString("## 7.2 估值情景\n\n")
	if hasRIM {
		b.WriteString("| 情景 | ROE假设 | 内在价值(元) | 相对现价 | 评级 |\n")
		b.WriteString("|------|---------|-------------|----------|------|\n")
		b.WriteString(fmt.Sprintf("| 悲观 | %.2f%% | %.2f | %+.1f%% | %s |\n", rim.Result.Pessimistic.ROE, rim.Result.Pessimistic.Value, rim.Result.Pessimistic.DiffPct, rim.Result.Pessimistic.Grade))
		b.WriteString(fmt.Sprintf("| 基准 | %.2f%% | %.2f | %+.1f%% | %s |\n", rim.Result.Baseline.ROE, rim.Result.Baseline.Value, rim.Result.Baseline.DiffPct, rim.Result.Baseline.Grade))
		b.WriteString(fmt.Sprintf("| 乐观 | %.2f%% | %.2f | %+.1f%% | %s |\n", rim.Result.Optimistic.ROE, rim.Result.Optimistic.Value, rim.Result.Optimistic.DiffPct, rim.Result.Optimistic.Grade))
		b.WriteString("\n")
	} else {
		b.WriteString("| 情景 | ROE假设 | 内在价值/净资产 | 评级 |\n")
		b.WriteString("|------|---------|----------------|------|\n")
		b.WriteString("| 悲观 | ROE-3pp | 约1.2-1.5x PB | 谨慎 |\n")
		b.WriteString("| 基准 | 维持当前 | 约1.5-2.0x PB | 中性 |\n")
		b.WriteString("| 乐观 | ROE+3pp | 约2.0-2.5x PB | 积极 |\n")
		b.WriteString("\n")
	}

	// 多期明细
	if hasRIM && len(rim.Result.Details) > 0 {
		b.WriteString("## 7.3 多期计算明细\n\n")
		b.WriteString("| 年度 | EPS(元) | DPS(元) | BPS(元) | RE(元) | 折现率 | RE现值(元) |\n")
		b.WriteString("|------|---------|---------|---------|--------|--------|------------|\n")
		runningBPS := 0.0
		if rim != nil {
			runningBPS = rim.Params.BPS0
		}
		for _, d := range rim.Result.Details {
			runningBPS = runningBPS + d.EPS - d.DPS
			b.WriteString(fmt.Sprintf("| 第%d年 | %.2f | %.2f | %.2f | %.4f | %.4f | %.4f |\n", d.Year, d.EPS, d.DPS, runningBPS, d.RE, d.Discount, d.PVRE))
		}
		b.WriteString(fmt.Sprintf("| **RE现值之和** | - | - | - | - | - | **%.4f** |\n", rim.Result.SumPVRE))
		b.WriteString(fmt.Sprintf("| **持续价值 CV** | - | - | - | %.4f | - | **%.4f** |\n", rim.Result.CV, rim.Result.PVCV))
		b.WriteString(fmt.Sprintf("| **每股价值** | - | - | - | - | - | **%.2f** |\n", rim.Result.Value))
		b.WriteString("\n")
		if currentPrice > 0 {
			b.WriteString(fmt.Sprintf("> **多期模型估算每股价值**: %.2f 元/股，相对当前股价 %.2f 元 **%+.1f%%**。\n\n", rim.Result.Value, currentPrice, rim.Result.Upside))
		} else {
			b.WriteString(fmt.Sprintf("> **多期模型估算每股价值**: %.2f 元/股（未接入实时股价，无法计算涨幅）。\n\n", rim.Result.Value))
		}
	}

	b.WriteString("> **解读**: RIM 估值的核心在于 ROE 能否持续高于资本成本。")
	if hasRIM && currentPrice > 0 {
		diff := rim.Result.Upside
		if diff >= 20 {
			b.WriteString(fmt.Sprintf("当前多期模型估算每股价值 %.2f 元显著高于市价 %.2f 元，存在约 %.0f%% 的潜在上行空间。", rim.Result.Value, currentPrice, diff))
		} else if diff >= 0 {
			b.WriteString(fmt.Sprintf("当前多期模型估算每股价值 %.2f 元略高于市价 %.2f 元，上行空间约 %.0f%%，安全边际一般。", rim.Result.Value, currentPrice, diff))
		} else {
			b.WriteString(fmt.Sprintf("当前多期模型估算每股价值 %.2f 元低于市价 %.2f 元，当前价格可能已反映乐观预期。", rim.Result.Value, currentPrice))
		}
	} else {
		if roe >= 15 {
			b.WriteString(fmt.Sprintf("当前 ROE %.2f%% 高于一般资本成本，具备创造价值的能力。", roe))
		} else if roe > 7 {
			b.WriteString(fmt.Sprintf("当前 ROE %.2f%% 略高于资本成本，但安全边际不足。", roe))
		} else {
			b.WriteString(fmt.Sprintf("当前 ROE %.2f%% 低于资本成本，长期可能侵蚀股东价值。", roe))
		}
	}
	b.WriteString("\n\n---\n\n")
}

func rimSourceDesc(rim *RIMData, quote *QuoteData) string {
	if rim == nil || rim.Params.BPS0 <= 0 {
		return "待计算"
	}
	if quote != nil && quote.PB > 0 && quote.CurrentPrice > 0 {
		calc := quote.CurrentPrice / quote.PB
		if math.Abs(calc-rim.Params.BPS0) < 0.5 {
			return "股价/PB推算"
		}
	}
	return "财报股东权益/总股本推算"
}

// ========== 模块9: 技术面分析 ==========
func writeModule8(b *strings.Builder, quote *QuoteData, technical *TechnicalData, activity *ActivityData) {
	b.WriteString("# 模块9: 技术面分析\n\n")

	if quote == nil || quote.CurrentPrice == 0 {
		b.WriteString("> **说明**: 当前暂无实时行情数据，无法生成技术面分析。请在网络畅通时重新选中股票获取行情。\n\n")
		b.WriteString("---\n\n")
		return
	}

	cp := quote.CurrentPrice
	high := quote.High
	low := quote.Low
	open := quote.Open
	prev := quote.PreviousClose
	tr := quote.TurnoverRate
	amp := quote.Amplitude

	b.WriteString("## 9.1 日内价格位置\n\n")
	if high > low {
		pos := (cp - low) / (high - low) * 100
		b.WriteString(fmt.Sprintf("- 当前价格处于今日高低点区间的 **%.1f%%** 位置", pos))
		if pos > 70 {
			b.WriteString("，接近日内高点，多头力量较强。\n")
		} else if pos < 30 {
			b.WriteString("，接近日内低点，空头压力较大。\n")
		} else {
			b.WriteString("，位于中间区域，多空博弈均衡。\n")
		}
	}
	if open > 0 && prev > 0 {
		gap := (open - prev) / prev * 100
		if math.Abs(gap) > 1 {
			b.WriteString(fmt.Sprintf("- 今日开盘跳空 %.2f%%，%s\n", gap, gapDirection(gap)))
		}
	}
	b.WriteString("\n")

	b.WriteString("## 9.2 量价关系简评\n\n")
	// 优先使用近5日平均换手/振幅判断，避免单日异常导致误判
	avgTr := tr
	avgAmp := amp
	if activity != nil {
		if activity.TurnoverDensity > 0 {
			avgTr = activity.TurnoverDensity
		}
		if activity.AvgAmplitude5 > 0 {
			avgAmp = activity.AvgAmplitude5
		}
	}
	if avgTr >= 3 && avgAmp >= 3 {
		b.WriteString("- **高换手高振幅**：交投活跃，资金博弈激烈，短期趋势可能延续。\n")
	} else if avgTr >= 3 && avgAmp < 3 {
		b.WriteString("- **高换手低振幅**：筹码交换充分但价格波动有限，可能是蓄势或出货信号。\n")
	} else if avgTr < 1 && avgAmp >= 3 {
		b.WriteString("- **低换手高振幅**：流动性不足导致价格易受大单影响，波动具有偶然性。\n")
	} else {
		b.WriteString("- **低换手低振幅**：交投清淡，趋势惯性较强，突破需放量确认。\n")
	}

	b.WriteString("\n## 9.3 短期技术倾向\n\n")
	score := 0
	if quote.ChangePercent > 0 { score++ }
	if cp > open { score++ }
	if high > prev && low > prev*0.97 { score++ }
	if tr > 1 { score++ }

	switch score {
	case 4:
		b.WriteString("**偏多**:  price、开盘、高低点及换手均呈现积极信号。\n")
	case 3:
		b.WriteString("**略偏多**: 大部分日内指标偏向积极，但有一处偏弱。\n")
	case 2:
		b.WriteString("**中性**: 多空信号交织，短期方向不明。\n")
	case 1:
		b.WriteString("**略偏空**: 大部分日内指标偏弱，仅有一处积极信号。\n")
	default:
		b.WriteString("**偏空**: price、开盘、高低点及换手均呈现弱势信号。\n")
	}

	// 新增：基于历史K线的技术指标分析
	if technical != nil && technical.Score > 0 {
		b.WriteString("\n## 9.4 历史K线技术指标分析\n\n")
		b.WriteString(fmt.Sprintf("| 指标 | 状态 | 说明 |\n"))
		b.WriteString(fmt.Sprintf("|------|------|------|\n"))
		b.WriteString(fmt.Sprintf("| 技术评分 | %s | %.0f/100（%s） |\n", scoreToStars(technical.Score), technical.Score, technical.Grade))
		b.WriteString(fmt.Sprintf("| 趋势方向 | %s | %s |\n", technical.Trend, technical.MAStatus))
		b.WriteString(fmt.Sprintf("| MACD | %s | 动能信号 |\n", technical.MACDStatus))
		b.WriteString(fmt.Sprintf("| RSI(14) | %s | 超买超卖状态 |\n", technical.RSIStatus))
		b.WriteString(fmt.Sprintf("| 布林带 | %s | 价格相对波动区间 |\n", technical.BollingerStatus))
		b.WriteString(fmt.Sprintf("| 量价关系 | %s | 成交量配合程度 |\n", technical.VolumeStatus))
		b.WriteString(fmt.Sprintf("| 技术形态 | %s | %s |\n", technical.Pattern, technical.SupportResistance))
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("> **综合结论**: %s\n\n", technical.Comment))

		b.WriteString("### 技术指标联动分析图（K线+成交量+MACD+RSI+布林带）\n\n")
		b.WriteString("<div class=\"chart-unified\"></div>\n\n")
	}

	if activity != nil && activity.Score > 0 && activity.PotentialHint != "" {
		b.WriteString("\n## 9.5 交易活跃度潜力提示\n\n")
		b.WriteString(fmt.Sprintf("> **活跃度评级**: %s（%.0f分）\n\n", formatActivityStars(activity.Stars), activity.Score))
		b.WriteString(fmt.Sprintf("> 💡 **提示**: %s\n\n", activity.PotentialHint))
	}

	b.WriteString("\n---\n\n")
}

// ========== 模块10: ML机器学习预测（ONNX 双引擎：Engine-A 情绪+价格 / Engine-B 财务 BiLSTM） ==========
func mlDirectionCN(label string) string {
	switch label {
	case "up":
		return "上涨 ↑"
	case "down":
		return "下跌 ↓"
	case "flat":
		return "持平 →"
	default:
		return label
	}
}

func writeModule9(b *strings.Builder, steps []StepResult, latest, prev string, ml *MLPredictionData) {
	b.WriteString("# 模块10: ML机器学习预测\n\n")

	hasML := ml != nil && (ml.Financial != nil || ml.Sentiment != nil || ml.EngineD != nil)
	if !hasML {
		b.WriteString("> **说明**: 当前版本暂无机器学习模型，以下基于财务趋势做简易方向推断。\n\n")
	} else {
		b.WriteString("> **说明**: 以下预测结果由 ONNX 多引擎模型生成。\n\n")
	}

	// 双引擎融合总结（2-4周预测）
	if ml != nil && ml.Summary != nil && ml.Summary.HasData {
		sum := ml.Summary
		if sum.RangeHigh > 0 && sum.RangeLow >= 0 {
			b.WriteString(fmt.Sprintf("> **未来 2-4 周预测**: %s，预计涨幅 **+%.0f%% ~ +%.0f%%**。%s\n\n", sum.Direction, sum.RangeLow, sum.RangeHigh, sum.Reason))
		} else if sum.RangeHigh <= 0 && sum.RangeLow < 0 {
			b.WriteString(fmt.Sprintf("> **未来 2-4 周预测**: %s，预计跌幅 **%.0f%% ~ %.0f%%**。%s\n\n", sum.Direction, math.Abs(sum.RangeHigh), math.Abs(sum.RangeLow), sum.Reason))
		} else {
			b.WriteString(fmt.Sprintf("> **未来 2-4 周预测**: %s，预计波动区间 **%.0f%% ~ +%.0f%%**。%s\n\n", sum.Direction, sum.RangeLow, sum.RangeHigh, sum.Reason))
		}
	}

	// 引擎B：财务LSTM - 始终显示10.1章节
	b.WriteString("## 10.1 Engine-B 财务趋势预测（BiLSTM+Self-Attention）\n\n")
	if ml != nil && ml.Financial != nil {
		fp := ml.Financial
		b.WriteString(fmt.Sprintf("| 指标 | 预测方向 | 概率 | 说明 |\n"))
		b.WriteString("|------|----------|------|------|\n")
		b.WriteString(fmt.Sprintf("| ROE 趋势 | %s | %.1f%% | 基于最近 8 季度财务序列 |\n", mlDirectionCN(fp.ROEDirection), fp.ROEProb*100))
		b.WriteString(fmt.Sprintf("| 营收趋势 | %s | %.1f%% | 营收增长方向预测 |\n", mlDirectionCN(fp.RevenueDirection), fp.RevenueProb*100))
		b.WriteString(fmt.Sprintf("| M-Score 趋势 | %s | %.1f%% | 财报质量变化方向 |\n", mlDirectionCN(fp.MScoreDirection), fp.MScoreProb*100))
		b.WriteString(fmt.Sprintf("| 财务健康分 | %.2f / 10 | — | 综合健康度评分 |\n\n", fp.HealthScore))
	} else {
		b.WriteString("> **数据缺失**: Engine-B 模型暂不可用。可能原因：\n")
		b.WriteString("> - ONNX 模型文件未加载（需要 `engine_b_financial.onnx`）\n")
		b.WriteString("> - 或历史财务数据不足（需要至少 8 个季度数据）\n\n")
	}

	// 引擎A：舆情+价格 - 始终显示10.2章节
	b.WriteString("## 10.2 Engine-A 市场情绪与价格融合预测（Cross-Attention）\n\n")
	if ml != nil && ml.Sentiment != nil {
		sp := ml.Sentiment
		b.WriteString(fmt.Sprintf("| 指标 | 预测结果 | 概率 | 说明 |\n"))
		b.WriteString("|------|----------|------|------|\n")
		b.WriteString(fmt.Sprintf("| 次日走势 | %s | %.1f%% | 上涨/持平/下跌 |\n", mlDirectionCN(sp.MovementLabel), sp.MovementProb*100))
		b.WriteString(fmt.Sprintf("| 异动概率 | %.2f%% | — | 价格异常波动预警 |\n\n", sp.AnomalyProb*100))
	} else {
		b.WriteString("> **数据缺失**: Engine-A 模型暂不可用。可能原因：\n")
		b.WriteString("> - ONNX 模型文件未加载（需要 `engine_a_sentiment.onnx`）\n")
		b.WriteString("> - 或市场舆情数据获取失败\n\n")
	}

	// 引擎D：风险预警 - 始终显示10.3章节
	b.WriteString("## 10.3 Engine-D 风险预警模型（GradientBoosting）\n\n")
	if ml != nil && ml.EngineD != nil {
		dp := ml.EngineD
		
		// 风险等级颜色标识
		riskEmoji := "🟢"
		if dp.RiskLevel == "中风险" {
			riskEmoji = "🟡"
		} else if dp.RiskLevel == "高风险" {
			riskEmoji = "🔴"
		}
		
		modelStatus := "✅ 模型"
		if !dp.ModelLoaded {
			modelStatus = "⚠️ 规则"
		}
		
		b.WriteString(fmt.Sprintf("| 指标 | 结果 | 说明 |\n"))
		b.WriteString("|------|------|------|\n")
		b.WriteString(fmt.Sprintf("| 风险评级 | %s %s | 基于%s评估 |\n", riskEmoji, dp.RiskLevel, modelStatus))
		b.WriteString(fmt.Sprintf("| 风险概率 | %.1f%% | 财务造假/退市风险概率 |\n", dp.RiskProb*100))
		
		if len(dp.TopFactors) > 0 {
			factorsStr := strings.Join(dp.TopFactors, ", ")
			b.WriteString(fmt.Sprintf("| 主要风险因子 | %s | 影响最大的特征 |\n", factorsStr))
		}
		b.WriteString("\n")
		
		// 风险提示
		if dp.RiskLabel == 1 {
			b.WriteString("> ⚠️ **风险提示**: Engine-D 模型识别到潜在风险信号，建议进一步审慎评估。\n\n")
		} else {
			b.WriteString("> ✅ **风险评估**: 当前无显著风险信号，财务状况相对健康。\n\n")
		}
	} else {
		b.WriteString("> **数据缺失**: Engine-D 模型暂不可用。可能原因：\n")
		b.WriteString("> - ONNX 模型文件未加载（需要 `engine_d_risk.onnx`）\n")
		b.WriteString("> - 或基于规则的回退评估也未启用\n\n")
	}

	// 如果没有 ML，保留原来的简易推断
	if !hasML {
		b.WriteString("## 10.4 负向因子（基于财务指标的简易推断）\n\n")
		var neg, pos []string
		if g := getStepValue(steps, 9, latest, "growthRate"); g < 10 {
			neg = append(neg, fmt.Sprintf("- 营收增长率 %.2f%%，低于理想水平", g))
		}
		if pg := getStepValue(steps, 16, latest, "profitGrowth"); pg < 10 {
			neg = append(neg, fmt.Sprintf("- 净利润增长率 %.2f%%，低于理想水平", pg))
		}
		if roe := getStepValue(steps, 16, latest, "roe"); roe < 15 {
			neg = append(neg, fmt.Sprintf("- ROE %.2f%%，资本回报能力偏弱", roe))
		}
		if gm := getStepValue(steps, 10, latest, "grossMargin"); gm < 40 {
			neg = append(neg, fmt.Sprintf("- 毛利率 %.2f%%，产品竞争力未达高毛利标准", gm))
		}
		if ascore := getStepValue(steps, 8, latest, "AScore"); ascore >= 60 {
			neg = append(neg, fmt.Sprintf("- A-Score %.1f，综合财务风险需关注", ascore))
		}
		if dr := getStepValue(steps, 3, latest, "debtRatio"); dr > 60 {
			neg = append(neg, fmt.Sprintf("- 资产负债率 %.2f%%，偿债压力偏大", dr))
		}
		if len(neg) == 0 {
			neg = append(neg, "- 暂无显著负向因子")
		}
		for _, s := range neg {
			b.WriteString(s + "\n")
		}
		b.WriteString("\n")

		b.WriteString("## 10.4 正向因子\n\n")
		if g := getStepValue(steps, 9, latest, "growthRate"); g >= 10 {
			pos = append(pos, fmt.Sprintf("- 营收增长率 %.2f%%，保持稳健增长", g))
		}
		if pg := getStepValue(steps, 16, latest, "profitGrowth"); pg >= 10 {
			pos = append(pos, fmt.Sprintf("- 净利润增长率 %.2f%%，盈利能力持续改善", pg))
		}
		if roe := getStepValue(steps, 16, latest, "roe"); roe >= 15 {
			pos = append(pos, fmt.Sprintf("- ROE %.2f%%，资本回报能力良好", roe))
		}
		if gm := getStepValue(steps, 10, latest, "grossMargin"); gm >= 40 {
			pos = append(pos, fmt.Sprintf("- 毛利率 %.2f%%，具备较强定价权", gm))
		}
		if dr := getStepValue(steps, 3, latest, "debtRatio"); dr <= 40 {
			pos = append(pos, fmt.Sprintf("- 资产负债率 %.2f%%，财务结构稳健", dr))
		}
		if cr := getStepValue(steps, 15, latest, "cashRatio"); cr >= 100 {
			pos = append(pos, fmt.Sprintf("- 净利润现金含量 %.2f%%，盈利质量高", cr))
		}
		if ascore := getStepValue(steps, 8, latest, "AScore"); ascore < 50 {
			pos = append(pos, fmt.Sprintf("- A-Score %.1f，综合财务风险可控", ascore))
		}
		if len(pos) == 0 {
			pos = append(pos, "- 暂无显著正向因子")
		}
		for _, s := range pos {
			b.WriteString(s + "\n")
		}
		b.WriteString("\n")

		b.WriteString("## 10.5 简易预测结论\n\n")
		score := 50.0
		score -= float64(len(neg)) * 8
		score += float64(len(pos)) * 8
		score = math.Max(0, math.Min(100, score))
		b.WriteString(fmt.Sprintf("**财务趋势评分**: %.0f/100（基于正负向因子简易加权）\n\n", score))
	}

	b.WriteString("---\n\n")
}

// ========== 模块11: 智能选股7大条件 ==========
func writeModule10(b *strings.Builder, steps []StepResult, latest, prev string) {
	b.WriteString("# 模块11: 智能选股7大条件\n\n")
	b.WriteString("## 11.1 条件检查表" + traceTrigger(3,9,10,15,16) + "\n\n")

	roe := getStepValue(steps, 16, latest, "roe")
	gm := getStepValue(steps, 10, latest, "grossMargin")
	growth := getStepValue(steps, 9, latest, "growthRate")
	dr := getStepValue(steps, 3, latest, "debtRatio")
	cashDiff := getStepValue(steps, 3, latest, "cashDebtDiff")
	cr := getStepValue(steps, 15, latest, "cashRatio")

	conditions := []struct {
		name   string
		std    string
		value  string
		pass   bool
		points int
		max    int
	}{
		{"① ROE ≥ 15%", "≥15%", fmt.Sprintf("%.2f%%", roe), roe >= 15, 20, 20},
		{"② 毛利率 ≥ 40%", "≥40%", fmt.Sprintf("%.2f%%", gm), gm >= 40, 15, 15},
		{"③ 营收增长 ≥ 10%", "≥10%", fmt.Sprintf("%.2f%%", growth), growth >= 10, 15, 15},
		{"④ A-Score < 60", "<60", fmt.Sprintf("%.1f", getStepValue(steps, 8, latest, "AScore")), getStepValue(steps, 8, latest, "AScore") < 60, 15, 15},
		{"⑤ 资产负债率 ≤ 60%", "≤60%", fmt.Sprintf("%.2f%%", dr), dr <= 60, 10, 10},
		{"⑥ 净利润现金含量 ≥ 100%", "≥100%", fmt.Sprintf("%.2f%%", cr), cr >= 100, 15, 15},
		{"⑦ 准货币资金-有息负债 ≥ 0", "≥0", fmt.Sprintf("%.2f亿", cashDiff/1e8), cashDiff >= 0, 10, 10},
	}

	b.WriteString("| 条件 | 标准 | 实际值 | 是否满足 | 得分 |\n")
	b.WriteString("|------|------|--------|----------|------|\n")
	totalScore := 0
	passCount := 0
	for _, c := range conditions {
		passStr := "❌"
		if c.pass {
			passStr = "✅"
			passCount++
			totalScore += c.points
		}
		b.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %d/%d |\n", c.name, c.std, c.value, passStr, totalScore, c.max))
	}
	b.WriteString(fmt.Sprintf("| **总分** | - | - | **%d/%d项** | **%d/100** |\n", passCount, len(conditions), totalScore))
	b.WriteString("\n")

	if passCount >= 5 {
		b.WriteString("**选股评级**: ✅ **符合**核心买入条件\n\n")
	} else if passCount >= 3 {
		b.WriteString("**选股评级**: 🟡 **部分符合**，需观察短板改善\n\n")
	} else {
		b.WriteString("**选股评级**: ❌ **不符合**买入条件\n\n")
	}
	b.WriteString("---\n\n")
}

// ========== 模块12: 芒格逆向思维检查 ==========
func writeModule11(b *strings.Builder, steps []StepResult, latest string, score *YearScore) {
	b.WriteString("# 模块12: 芒格逆向思维检查\n\n")

	b.WriteString("## 12.1 逆向三问\n\n")

	b.WriteString("### 问1: 市场忽略了什么负面因素？\n\n")
	risks := extractRisks(steps, latest)
	if len(risks) == 0 {
		b.WriteString("- 暂未发现重大被忽略的负面因素。\n")
	} else {
		for _, r := range risks {
			b.WriteString(fmt.Sprintf("- **%s**: %s（%s）\n", r.Category, r.Indicator, r.Desc))
		}
	}
	b.WriteString("\n")

	b.WriteString("### 问2: 悲观时忽略了什么积极因素？\n\n")
	var positives []string
	if g := getStepValue(steps, 9, latest, "growthRate"); g >= 10 {
		positives = append(positives, fmt.Sprintf("- 营收保持 %.2f%% 增长，业务仍在扩张", g))
	}
	if roe := getStepValue(steps, 16, latest, "roe"); roe >= 15 {
		positives = append(positives, fmt.Sprintf("- ROE %.2f%%，长期资本回报能力依然优秀", roe))
	}
	if gm := getStepValue(steps, 10, latest, "grossMargin"); gm >= 40 {
		positives = append(positives, fmt.Sprintf("- 毛利率 %.2f%%，护城河较深", gm))
	}
	if dr := getStepValue(steps, 3, latest, "debtRatio"); dr <= 40 {
		positives = append(positives, fmt.Sprintf("- 资产负债率 %.2f%%，财务结构健康", dr))
	}
	if cr := getStepValue(steps, 15, latest, "cashRatio"); cr >= 100 {
		positives = append(positives, fmt.Sprintf("- 经营现金流充沛，净利润含金量 %.2f%%", cr))
	}
	if ascore := getStepValue(steps, 8, latest, "AScore"); ascore < 50 {
		positives = append(positives, fmt.Sprintf("- A-Score %.1f，财报质量可信", ascore))
	}
	if len(positives) == 0 {
		positives = append(positives, "- 当前财务数据中积极因素不明显，需等待基本面改善信号。")
	}
	for _, p := range positives {
		b.WriteString(p + "\n")
	}
	b.WriteString("\n")

	b.WriteString("### 问3: 什么情况下这笔交易会成功？\n\n")
	b.WriteString("**反转触发器**:\n")
	b.WriteString("1. ROE 持续回升并稳定在 15% 以上\n")
	b.WriteString("2. 毛利率止跌回升，定价权修复\n")
	b.WriteString("3. 经营现金流持续覆盖净利润\n")
	b.WriteString("4. A-Score 回落至 60 以下\n")
	b.WriteString("\n")

	b.WriteString("## 12.2 逆向检查评分\n\n")
	revScore := reverseScore(steps, latest, score)
	b.WriteString(fmt.Sprintf("**基础分**: 85分  \n"))
	b.WriteString(fmt.Sprintf("**扣分合计**: %.0f分  \n", math.Max(0, 85-revScore)))
	b.WriteString(fmt.Sprintf("**最终评分**: %.0f/100  \n", revScore))
	if revScore >= 70 {
		b.WriteString("**风险评级**: 🟢 **中等偏低风险**\n\n")
	} else if revScore >= 50 {
		b.WriteString("**风险评级**: 🟠 **中等风险**\n\n")
	} else {
		b.WriteString("**风险评级**: 🔴 **中高风险**\n\n")
	}
	b.WriteString("---\n\n")
}

// ========== 模块13: 巴菲特-芒格投资检查清单 ==========
func writeModule12(b *strings.Builder, steps []StepResult, latest string, score *YearScore) {
	b.WriteString("# 模块13: 巴菲特-芒格投资检查清单\n\n")
	b.WriteString("## 13.1 7项核心检查" + traceTrigger(3,7,10,15,16,18) + "\n\n")

	roe := getStepValue(steps, 16, latest, "roe")
	gm := getStepValue(steps, 10, latest, "grossMargin")
	growth := getStepValue(steps, 9, latest, "growthRate")
	pg := getStepValue(steps, 16, latest, "profitGrowth")
	dr := getStepValue(steps, 3, latest, "debtRatio")
	cr := getStepValue(steps, 15, latest, "cashRatio")
	divRatio := getStepValue(steps, 18, latest, "ratio")

	checks := []struct {
		dim    string
		weight string
		item   string
		score  float64
		desc   string
	}{
		{"护城河", "15%", "竞争优势（毛利率/ROE）", mapScore(gm, 40, 20, 5), moatComment(gm, roe)},
		{"能力圈", "10%", "业务可理解（主业专注度）", mapScore(getStepValue(steps, 7, latest, "ratio"), 10, 30, 5), "投资类资产占比反映主业专注度"},
		{"安全边际", "20%", "估值有折扣（暂缺股价）", 3, "未接入实时股价，无法计算安全边际"},
		{"长期价值", "10%", "持续经营能力（现金流/分红）", mapScore(divRatio, 45, 70, 5), fmt.Sprintf("分红占比 %.1f%%，分红可持续性待观察", divRatio)},
		{"管理层", "10%", "诚信可靠（A-Score审计质量）", mapScore(100-getStepValue(steps, 8, latest, "AScore"), 40, 1.0, 5), fmt.Sprintf("A-Score=%.1f%s", getStepValue(steps, 8, latest, "AScore"), auditCommentAScore(getStepValue(steps, 8, latest, "AScore")))},
		{"财务稳健", "20%", "现金流健康/负债率低", (mapScore(dr, 60, 80, 5) + mapScore(cr, 100, 50, 5)) / 2, fmt.Sprintf("负债率%.1f%%，现金含量%.1f%%", dr, cr)},
		{"供需格局", "15%", "成长空间（营收增长率）", mapScore(growth, 20, 0, 5), growthComment(steps, latest)},
	}

	b.WriteString("| 维度 | 权重 | 检查项 | 评分 | 说明 |\n")
	b.WriteString("|------|------|--------|------|------|\n")
	total := 0.0
	for _, c := range checks {
		b.WriteString(fmt.Sprintf("| %s | %s | %s | %.1f/5 | %s |\n", c.dim, c.weight, c.item, c.score, c.desc))
		total += c.score
	}
	b.WriteString(fmt.Sprintf("| **总分** | 100%% | - | **%.1f/10** | %s |\n", total/3.5, buffettComment(steps, latest, score)))
	b.WriteString("\n")

	b.WriteString("## 13.2 关键否决项\n\n")
	if roe < 15 {
		b.WriteString(fmt.Sprintf("- ❌ ROE>15%%可持续？**存疑**（当前%.2f%%）\n", roe))
	} else {
		b.WriteString(fmt.Sprintf("- ✅ ROE>15%%可持续？**通过**（当前%.2f%%）\n", roe))
	}
	if growth < 0 || pg < 0 {
		b.WriteString(fmt.Sprintf("- ❌ 业绩正增长？**否**（营收%.2f%%，净利润%.2f%%）\n", growth, pg))
	} else {
		b.WriteString(fmt.Sprintf("- ✅ 业绩正增长？**是**（营收%.2f%%，净利润%.2f%%）\n", growth, pg))
	}
		if ascore := getStepValue(steps, 8, latest, "AScore"); ascore >= 60 {
			b.WriteString(fmt.Sprintf("- ❌ 财报可信？**存疑**（A-Score %.1f）\n", ascore))
		} else {
			b.WriteString(fmt.Sprintf("- ✅ 财报可信？**通过**（A-Score %.1f）\n", ascore))
		}
	b.WriteString("\n---\n\n")
}

// ========== 模块14: 社交媒体情绪监控 ==========
func writeModule13(b *strings.Builder, sentiment *SentimentData) {
	b.WriteString("# 模块14: 社交媒体情绪监控\n\n")

	if sentiment == nil || !sentiment.HasData {
		b.WriteString("> **说明**: 当前暂无可用舆情数据（网络受限或该股票近期无相关研报/资讯）。\n\n")
		b.WriteString("---\n\n")
		return
	}

	// 14.1 情绪指标
	b.WriteString("## 14.1 情绪指标\n\n")
	b.WriteString("| 指标 | 数值 | 说明 |\n")
	b.WriteString("|------|------|------|\n")

	scoreEmoji := "🟡"
	scoreDesc := "中性"
	if sentiment.Score > 0.3 {
		scoreEmoji = "🟢"
		scoreDesc = "偏多"
	} else if sentiment.Score < -0.3 {
		scoreEmoji = "🔴"
		scoreDesc = "偏空"
	}
	b.WriteString(fmt.Sprintf("| %s 情绪得分 | %.2f | %s |\n", scoreEmoji, sentiment.Score, scoreDesc))
	b.WriteString(fmt.Sprintf("| 📊 热度指数 | %d 条 | 近一年相关研报/资讯数量 |\n", sentiment.HeatIndex))
	b.WriteString(fmt.Sprintf("| ✅ 利好信号 | %d 个 | 命中正面关键词次数 |\n", len(sentiment.PositiveWords)))
	b.WriteString(fmt.Sprintf("| ⚠️ 风险信号 | %d 个 | 命中负面关键词次数 |\n", len(sentiment.NegativeWords)))
	b.WriteString("\n")

	// 14.2 关键词云
	if len(sentiment.PositiveWords) > 0 || len(sentiment.NegativeWords) > 0 {
		b.WriteString("## 14.2 关键词云\n\n")
		if len(sentiment.PositiveWords) > 0 {
			b.WriteString("**正面关键词**：" + strings.Join(sentiment.PositiveWords, "、") + "\n\n")
		}
		if len(sentiment.NegativeWords) > 0 {
			b.WriteString("**负面关键词**：" + strings.Join(sentiment.NegativeWords, "、") + "\n\n")
		}
	}

	// 14.3 最新舆情摘要
	if len(sentiment.Summaries) > 0 {
		b.WriteString("## 14.3 最新舆情摘要\n\n")
		for _, s := range sentiment.Summaries {
			emoji := "🟡"
			if s.Sentiment > 0.3 {
				emoji = "🟢"
			} else if s.Sentiment < -0.3 {
				emoji = "🔴"
			}
			b.WriteString(fmt.Sprintf("- %s **%s**（%s，%s）\n", emoji, s.Title, s.Source, s.Date))
		}
		b.WriteString("\n")
	}

	b.WriteString("---\n\n")
}

// ========== 模块15: 综合投资建议 ==========
func writeModule14(b *strings.Builder, symbol string, steps []StepResult, latest string, score *YearScore, quote *QuoteData, rim *RIMData, technical *TechnicalData, ml *MLPredictionData, sentiment *SentimentData) {
	b.WriteString("# 模块15: 综合投资建议\n\n")

	weighted := 0.0
	if score != nil {
		weighted = score.RawScore*0.30 + profitScore(steps, latest)*0.25 + cashScore(steps, latest)*0.20 + growthScore(steps, latest)*0.15 + debtScore(steps, latest)*0.10
	}

	b.WriteString("## 15.1 综合评分汇总\n\n")
	b.WriteString("| 模块 | 权重 | 得分 | 加权分 |\n")
	b.WriteString("|------|------|------|--------|\n")
	if score != nil {
		b.WriteString(fmt.Sprintf("| 财务健康度综合评分 | 30%% | %.0f/100 | %.1f |\n", score.RawScore, score.RawScore*0.30))
		b.WriteString(fmt.Sprintf("| 盈利能力 | 15%% | %.0f/100 | %.1f |\n", profitScore(steps, latest), profitScore(steps, latest)*0.15))
		b.WriteString(fmt.Sprintf("| 现金流质量 | 15%% | %.0f/100 | %.1f |\n", cashScore(steps, latest), cashScore(steps, latest)*0.15))
		b.WriteString(fmt.Sprintf("| 成长能力 | 15%% | %.0f/100 | %.1f |\n", growthScore(steps, latest), growthScore(steps, latest)*0.15))
		b.WriteString(fmt.Sprintf("| 偿债安全 | 10%% | %.0f/100 | %.1f |\n", debtScore(steps, latest), debtScore(steps, latest)*0.10))
		b.WriteString(fmt.Sprintf("| 逆向检查 | 10%% | %.0f/100 | %.1f |\n", reverseScore(steps, latest, score), reverseScore(steps, latest, score)*0.10))
		b.WriteString(fmt.Sprintf("| 巴芒清单 | 5%% | %.0f/100 | %.1f |\n", buffettScore(steps, latest, score)*10, buffettScore(steps, latest, score)))
		b.WriteString(fmt.Sprintf("| **总分** | 100%% | - | **%.0f/100** |\n", weighted))
	}
	b.WriteString("\n")

	ascore := getStepValue(steps, 8, latest, "AScore")
	entryRange, stopLoss, target := formatTradeLevels(quote, rim, technical, ml, ascore)

	b.WriteString("## 15.2 投资建议\n\n")
	b.WriteString("| 项目 | 建议 |\n")
	b.WriteString("|------|------|\n")
	b.WriteString(fmt.Sprintf("| **综合评级** | %s |\n", investmentGrade(weighted)))
	b.WriteString(fmt.Sprintf("| **综合评分** | %.0f/100 %s |\n", weighted, scoreToStars(weighted)))
	b.WriteString(fmt.Sprintf("| **操作建议** | %s |\n", strategyAdvice(weighted)))
	b.WriteString(fmt.Sprintf("| **建议仓位** | %s |\n", positionAdvice(weighted)))
	b.WriteString(fmt.Sprintf("| **入场区间** | %s |\n", entryRange))
	b.WriteString(fmt.Sprintf("| **止损位** | %s |\n", stopLoss))
	b.WriteString(fmt.Sprintf("| **目标位** | %s |\n", target))
	if sentiment != nil && sentiment.HasData {
		b.WriteString(fmt.Sprintf("| **舆情情绪** | %s |\n", sentimentSummary(sentiment)))
	}
	b.WriteString("\n")

	b.WriteString("## 15.3 操作策略\n\n")
	if weighted >= 80 {
		b.WriteString("**策略A：积极配置（推荐）**\n")
		b.WriteString("- 基本面健康，可逢低分批建仓\n")
		b.WriteString("- 建议仓位 5-8%，长期持有\n")
		b.WriteString("- 若回撤 10% 可加仓\n\n")
		b.WriteString("**策略B：持有者**\n")
		b.WriteString("- 继续持有，关注 ROE 和毛利率稳定性\n")
	} else if weighted >= 70 {
		b.WriteString("**策略A：逢低试探（推荐）**\n")
		b.WriteString("- 财务整体尚可，存在部分短板\n")
		b.WriteString("- 建议仓位 3-5%，分批试探\n")
		b.WriteString("- 等待 A-Score 或毛利率改善后再加仓\n\n")
		b.WriteString("**策略B：持有者**\n")
		b.WriteString("- 维持现有仓位，观察关键指标修复情况\n")
	} else if weighted >= 60 {
		b.WriteString("**策略A：观望等待（推荐）**\n")
		b.WriteString("- 财务存在明显短板，暂不建仓\n")
		b.WriteString("- 等待年报数据持续改善后再介入\n\n")
		b.WriteString("**策略B：左侧试探（激进）**\n")
		b.WriteString("- 若对行业长期前景有信心，可轻仓 1-3% 试探\n")
		b.WriteString("- 严格止损\n")
	} else {
		b.WriteString("**策略A：回避（推荐）**\n")
		b.WriteString("- 财务风险较高，建议回避\n")
		b.WriteString("- 等待风险释放、基本面反转后再考虑\n\n")
		b.WriteString("**策略B：持有者**\n")
		b.WriteString("- 建议减仓或设置严格止损\n")
	}
	if sentiment != nil && sentiment.HasData {
		b.WriteString("\n> **舆情提示**：")
		if sentiment.Score > 0.3 {
			b.WriteString(fmt.Sprintf("近期舆情整体偏正面（热度 %d 条），可作为辅助参考，但不宜单独作为决策依据。\n", sentiment.HeatIndex))
		} else if sentiment.Score < -0.3 {
			b.WriteString(fmt.Sprintf("近期舆情整体偏负面（热度 %d 条），建议保持谨慎，关注后续公告与风险释放。\n", sentiment.HeatIndex))
		} else {
			b.WriteString(fmt.Sprintf("近期舆情整体中性（热度 %d 条），建议以基本面判断为主，持续跟踪舆情变化。\n", sentiment.HeatIndex))
		}
	}
	b.WriteString("\n---\n\n")
}

// formatTradeLevels 根据实时行情、RIM 估值、技术面与 ML 预测动态计算交易水位
func formatTradeLevels(quote *QuoteData, rim *RIMData, technical *TechnicalData, ml *MLPredictionData, ascore float64) (entry, stop, target string) {
	if quote == nil || quote.CurrentPrice <= 0 {
		return "待接入实时行情后计算", "待接入实时行情后计算", "待接入RIM估值与行情后计算"
	}
	price := quote.CurrentPrice
	hasRIM := rim != nil && rim.HasData && rim.Result != nil && rim.Result.Baseline.Value > 0

	// 动态系数调整
	stopTighten := 1.0   // 止损收紧系数（越小越紧）
	targetFactor := 1.0  // 目标位折扣
	entryDiscount := 1.0 // 入场区间下限折扣（越小越保守）

	if ascore >= 70 {
		stopTighten = 0.92
		entryDiscount = 0.95
	} else if ascore >= 60 {
		stopTighten = 0.95
		entryDiscount = 0.97
	}
	if technical != nil && technical.Score < 40 {
		targetFactor = math.Max(0, targetFactor-0.05)
		entryDiscount = math.Max(0, entryDiscount-0.03)
	}
	if ml != nil && ml.Summary != nil && ml.Summary.HasData {
		if ml.Summary.RangeLow < 0 {
			// ML 预测包含下跌
			targetFactor = math.Max(0, targetFactor-0.05)
			entryDiscount = math.Max(0, entryDiscount-0.03)
		}
	}

	if hasRIM {
		baseline := rim.Result.Baseline.Value
		pessimistic := rim.Result.Pessimistic.Value
		optimistic := rim.Result.Optimistic.Value

		// 目标位：乐观情景价值 × 动态系数
		targetVal := optimistic * targetFactor
		target = fmt.Sprintf("%.2f元 (乐观情景", targetVal)
		if targetFactor < 1.0 {
			target += fmt.Sprintf("×%.0f%%", targetFactor*100)
		}
		target += ")"

		// 入场区间（先计算）
		var low, high float64
		switch {
		case price <= baseline:
			low = price * 0.97 * entryDiscount
			high = price * 1.02
			entry = fmt.Sprintf("%.2f ~ %.2f元 (现价附近可建仓)", low, high)
		case price <= optimistic:
			low = baseline * 0.95 * entryDiscount
			high = baseline
			entry = fmt.Sprintf("%.2f ~ %.2f元 (等待回调至基准价值)", low, high)
		default:
			low = baseline * 0.90 * entryDiscount
			high = baseline * 0.95
			entry = fmt.Sprintf("%.2f ~ %.2f元 (高估/观望)", low, high)
		}

		// 止损位：悲观情景的85%、当前价的88%×收紧系数、入场区间低位的95% 三者取较高者
		// 但必须低于入场区间低位，给买入后留足波动空间
		stopPrice := math.Max(pessimistic*0.85, price*0.88*stopTighten)
		// 确保止损位低于入场区间低位（预留至少5%的缓冲）
		maxStopPrice := low * 0.95
		if stopPrice >= low {
			stopPrice = maxStopPrice
		}
		stop = fmt.Sprintf("%.2f元 (-%.1f%%)", stopPrice, (price-stopPrice)/price*100)
	} else {
		// 无 RIM 时的简易估算
		target = "待接入RIM估值后计算"
		low := price * 0.98 * entryDiscount
		high := price * 1.02
		entry = fmt.Sprintf("%.2f ~ %.2f元 (现价±2%%试探)", low, high)
		
		stopPrice := price * 0.90 * stopTighten
		// 确保止损位低于入场区间低位（预留至少5%的缓冲）
		maxStopPrice := low * 0.95
		if stopPrice >= low {
			stopPrice = maxStopPrice
		}
		stop = fmt.Sprintf("%.2f元 (-%.1f%%)", stopPrice, (price-stopPrice)/price*100)
	}
	return
}

// ========== 模块16: 结论与附录 ==========
func writeModule15(b *strings.Builder, symbol string, steps []StepResult, years []string, latest string, score *YearScore, sentiment *SentimentData) {
	b.WriteString("# 模块16: 结论与附录\n\n")

	weighted := 0.0
	if score != nil {
		weighted = score.RawScore*0.30 + profitScore(steps, latest)*0.25 + cashScore(steps, latest)*0.20 + growthScore(steps, latest)*0.15 + debtScore(steps, latest)*0.10
	}

	b.WriteString("## 16.1 核心结论\n\n")
	b.WriteString("> **")
	b.WriteString(fmt.Sprintf("%s %s年报", symbol, latest))
	b.WriteString(fmt.Sprintf(" 综合评分 %.0f 分，评级 %s。", weighted, investmentGrade(weighted)))
	b.WriteString(oneSentenceAdvice(symbol, weighted, steps, latest))
	if sentiment != nil && sentiment.HasData {
		b.WriteString(fmt.Sprintf(" 舆情方面：%s。", sentimentSummary(sentiment)))
	}
	b.WriteString("**\n\n")

	b.WriteString("## 16.2 关键数据速查\n\n")
	b.WriteString(fmt.Sprintf("| 指标 | %s | 同比 | 评估 |\n", latest))
	b.WriteString("|------|--------|------|------|\n")
	rev := getStepValue(steps, 9, latest, "revenue")
	prevRev := getStepValue(steps, 9, years[minInt(1, len(years)-1)], "revenue")
	b.WriteString(fmt.Sprintf("| 营业总收入 | %.2f亿 | %s | %s |\n", rev/1e8, yoyFmt(rev, prevRev), yoyEmoji(rev, prevRev)))
	prof := getStepValue(steps, 16, latest, "profit")
	prevProf := getStepValue(steps, 16, years[minInt(1, len(years)-1)], "profit")
	b.WriteString(fmt.Sprintf("| 归母净利润 | %.2f亿 | %s | %s |\n", prof/1e8, yoyFmt(prof, prevProf), yoyEmoji(prof, prevProf)))
	b.WriteString(fmt.Sprintf("| 毛利率 | %.2f%% | - | %s |\n", getStepValue(steps, 10, latest, "grossMargin"), gmEmoji(getStepValue(steps, 10, latest, "grossMargin"))))
	b.WriteString(fmt.Sprintf("| 资产负债率 | %.2f%% | - | %s |\n", getStepValue(steps, 3, latest, "debtRatio"), drEmoji(getStepValue(steps, 3, latest, "debtRatio"))))
	b.WriteString(fmt.Sprintf("| A-Score | %.1f | - | %s |\n", getStepValue(steps, 8, latest, "AScore"), asEmoji(getStepValue(steps, 8, latest, "AScore"))))
	if score != nil {
		b.WriteString(fmt.Sprintf("| 财报健康分 | %.0f分（%s） | - | - |\n", score.RawScore, score.Grade))
	}
	b.WriteString("\n")

	b.WriteString("## 16.3 投资逻辑总结\n\n")
	b.WriteString("**负面因素**:\n")
	risks := extractRisks(steps, latest)
	if len(risks) == 0 {
		b.WriteString("1. 未发现显著财务风险点\n")
	} else {
		for i, r := range risks {
			b.WriteString(fmt.Sprintf("%d. %s：%s\n", i+1, r.Category, r.Desc))
		}
	}
	if sentiment != nil && sentiment.HasData && sentiment.Score < -0.3 {
		b.WriteString(fmt.Sprintf("%d. 舆情情绪：近期舆情整体偏负面（热度 %d 条），需关注市场风险。\n", len(risks)+1, sentiment.HeatIndex))
	}
	b.WriteString("\n**正面因素**:\n")
	positives := positiveFactors(steps, latest)
	if len(positives) == 0 {
		b.WriteString("1. 当前数据中积极因素不明显\n")
	} else {
		for i, p := range positives {
			b.WriteString(fmt.Sprintf("%d. %s\n", i+1, p))
		}
	}
	if sentiment != nil && sentiment.HasData && sentiment.Score > 0.3 {
		start := len(positives) + 1
		if len(positives) == 0 {
			start = 1
			b.WriteString(fmt.Sprintf("%d. 舆情情绪：近期舆情整体偏正面（热度 %d 条），市场关注度较好。\n", start, sentiment.HeatIndex))
		} else {
			b.WriteString(fmt.Sprintf("%d. 舆情情绪：近期舆情整体偏正面（热度 %d 条），市场关注度较好。\n", start, sentiment.HeatIndex))
		}
	}
	b.WriteString("\n")

	b.WriteString("## 16.4 免责声明\n\n")
	b.WriteString("本报告基于公开财务报表数据及财报透视分析模型生成，仅供参考，不构成任何投资建议。投资有风险，入市需谨慎。\n\n")
}

// ==================== 辅助函数 ====================

type RiskItem struct {
	Category  string
	Indicator string
	Severity  string
	Desc      string
}

func sentimentSummary(sentiment *SentimentData) string {
	if sentiment == nil || !sentiment.HasData {
		return "暂无数据"
	}
	desc := "中性"
	if sentiment.Score > 0.3 {
		desc = "偏多"
	} else if sentiment.Score < -0.3 {
		desc = "偏空"
	}
	return fmt.Sprintf("%s（热度 %d 条）", desc, sentiment.HeatIndex)
}

func scoreToStars(score float64) string {
	switch {
	case score >= 90:
		return "⭐⭐⭐⭐⭐"
	case score >= 80:
		return "⭐⭐⭐⭐"
	case score >= 70:
		return "⭐⭐⭐"
	case score >= 60:
		return "⭐⭐"
	default:
		return "⭐"
	}
}

func formatActivityStars(stars int) string {
	switch stars {
	case 5:
		return "⭐⭐⭐⭐⭐"
	case 4:
		return "⭐⭐⭐⭐"
	case 3:
		return "⭐⭐⭐"
	case 2:
		return "⭐⭐"
	default:
		return "⭐"
	}
}

func gradeComment(score float64) string {
	switch {
	case score >= 90:
		return "财务结构非常健康，各项指标优秀"
	case score >= 80:
		return "财务状况良好，少数维度有改善空间"
	case score >= 70:
		return "财务整体中等，需关注部分风险点"
	case score >= 60:
		return "财务存在明显短板，建议深入核查"
	default:
		return "财务风险较高，建议谨慎对待"
	}
}

func investmentGrade(score float64) string {
	switch {
	case score >= 90:
		return "强烈推荐"
	case score >= 80:
		return "推荐"
	case score >= 70:
		return "谨慎推荐"
	case score >= 60:
		return "观望"
	default:
		return "回避"
	}
}

func positionAdvice(score float64) string {
	switch {
	case score >= 90:
		return "8-12%"
	case score >= 80:
		return "5-8%"
	case score >= 70:
		return "3-5%"
	case score >= 60:
		return "1-3%或观望"
	default:
		return "0-1%或回避"
	}
}

func strategyAdvice(score float64) string {
	switch {
	case score >= 90:
		return "积极配置，长期持有"
	case score >= 80:
		return "逢低分批建仓"
	case score >= 70:
		return "观望/逢低轻仓试探"
	case score >= 60:
		return "观望为主，等待基本面改善"
	default:
		return "回避，等待风险释放"
	}
}

func oneSentenceAdvice(symbol string, score float64, steps []StepResult, year string) string {
	var parts []string
	if score >= 80 {
		parts = append(parts, "财务基本面整体健康")
	} else if score >= 70 {
		parts = append(parts, "财务基本面尚可，但存在部分短板")
	} else {
		parts = append(parts, "财务基本面偏弱，风险点较多")
	}

	roe := getStepValue(steps, 16, year, "roe")
	if roe >= 15 {
		parts = append(parts, fmt.Sprintf("ROE %.1f%%显示公司具备较好的资本回报能力", roe))
	} else if roe > 0 {
		parts = append(parts, fmt.Sprintf("ROE %.1f%%低于理想水平，资本回报能力有待提升", roe))
	}

	debtRatio := getStepValue(steps, 3, year, "debtRatio")
	if debtRatio <= 40 {
		parts = append(parts, "负债率低，财务结构稳健")
	} else if debtRatio > 60 {
		parts = append(parts, "负债率偏高，需关注偿债压力")
	}

	ascore := getStepValue(steps, 8, year, "AScore")
	if ascore >= 60 {
		parts = append(parts, fmt.Sprintf("A-Score %.1f 提示需警惕财报操纵或偿债风险", ascore))
	}

	parts = append(parts, fmt.Sprintf("建议%s", strategyAdvice(score)))
	return strings.Join(parts, "。") + "。"
}

func growthScore(steps []StepResult, year string) float64 {
	g := getStepValue(steps, 9, year, "growthRate")
	if g >= 20 {
		return 90
	} else if g >= 10 {
		return 80
	} else if g >= 0 {
		return 60
	} else if g >= -10 {
		return 40
	}
	return 20
}

func growthComment(steps []StepResult, year string) string {
	g := getStepValue(steps, 9, year, "growthRate")
	if g >= 20 {
		return "高速增长"
	} else if g >= 10 {
		return "稳健增长"
	} else if g >= 0 {
		return "增长放缓"
	} else if g >= -10 {
		return "轻微下滑"
	}
	return "显著下滑"
}

func profitScore(steps []StepResult, year string) float64 {
	roe := getStepValue(steps, 16, year, "roe")
	gm := getStepValue(steps, 10, year, "grossMargin")
	cp := getStepValue(steps, 14, year, "coreProfitMargin")
	score := 50.0
	if roe >= 20 {
		score += 25
	} else if roe >= 15 {
		score += 20
	} else if roe >= 10 {
		score += 10
	} else if roe > 0 {
		score += 5
	}
	if gm >= 40 {
		score += 15
	} else if gm >= 30 {
		score += 10
	} else if gm >= 20 {
		score += 5
	}
	if cp >= 15 {
		score += 10
	} else if cp >= 10 {
		score += 5
	}
	return math.Min(100, score)
}

func profitComment(steps []StepResult, year string) string {
	roe := getStepValue(steps, 16, year, "roe")
	gm := getStepValue(steps, 10, year, "grossMargin")
	if roe >= 15 && gm >= 40 {
		return "盈利能力优秀，高毛利+高ROE"
	} else if roe >= 15 {
		return "盈利能力良好，ROE达标但毛利率偏低"
	} else if roe >= 10 {
		return "盈利能力一般，ROE有提升空间"
	} else if roe > 0 {
		return "盈利能力偏弱，ROE低于理想水平"
	}
	return "盈利能力较差，ROE为负或极低"
}

func cashScore(steps []StepResult, year string) float64 {
	cr := getStepValue(steps, 15, year, "cashRatio")
	ocf := getStepValue(steps, 15, year, "operatingCF")
	score := 50.0
	if cr >= 100 {
		score += 30
	} else if cr >= 50 {
		score += 15
	} else if cr > 0 {
		score += 5
	}
	if ocf > 0 {
		score += 20
	}
	return math.Min(100, score)
}

func cashComment(steps []StepResult, year string) string {
	cr := getStepValue(steps, 15, year, "cashRatio")
	ocf := getStepValue(steps, 15, year, "operatingCF")
	if cr >= 100 && ocf > 0 {
		return "现金流质量优秀，经营现金流充沛"
	} else if ocf > 0 {
		return "经营现金流为正，但净利润含金量有提升空间"
	} else if ocf < 0 {
		return "经营现金流为负，现金流压力需关注"
	}
	return "现金流数据不足"
}

func debtScore(steps []StepResult, year string) float64 {
	dr := getStepValue(steps, 3, year, "debtRatio")
	diff := getStepValue(steps, 3, year, "cashDebtDiff")
	score := 50.0
	if dr <= 40 {
		score += 25
	} else if dr <= 60 {
		score += 15
	} else if dr <= 70 {
		score += 5
	}
	if diff >= 0 {
		score += 25
	} else if diff > -1e9 {
		score += 10
	}
	return math.Min(100, score)
}

func debtComment(steps []StepResult, year string) string {
	dr := getStepValue(steps, 3, year, "debtRatio")
	diff := getStepValue(steps, 3, year, "cashDebtDiff")
	if dr <= 40 && diff >= 0 {
		return "偿债能力优秀，负债率低且现金充裕"
	} else if dr <= 60 && diff >= 0 {
		return "偿债能力良好，结构相对安全"
	} else if dr <= 60 {
		return "偿债能力一般，准货币资金未能完全覆盖有息负债"
	} else if dr <= 70 {
		return "偿债压力较大，负债率偏高"
	}
	return "偿债风险高，需密切关注"
}

func valuationScore(quote *QuoteData) float64 {
	if quote == nil {
		return 0
	}
	score := 50.0
	if quote.PE > 0 {
		if quote.PE < 15 {
			score += 25
		} else if quote.PE < 25 {
			score += 15
		} else if quote.PE < 40 {
			score += 5
		} else {
			score -= 15
		}
	} else {
		score -= 15
	}
	if quote.PB > 0 {
		if quote.PB < 2 {
			score += 25
		} else if quote.PB < 3 {
			score += 15
		} else if quote.PB < 5 {
			score += 5
		} else {
			score -= 15
		}
	} else {
		score -= 15
	}
	return math.Max(0, math.Min(100, score))
}

func valuationComment(quote *QuoteData) string {
	if quote == nil || (quote.PE <= 0 && quote.PB <= 0) {
		return "暂无估值数据"
	}
	s := valuationScore(quote)
	if s >= 80 {
		return "估值较低，具备安全边际"
	} else if s >= 60 {
		return "估值处于合理区间"
	} else if s >= 40 {
		return "估值偏高，需关注性价比"
	}
	return "估值过高，注意风险"
}

func reverseScore(steps []StepResult, year string, score *YearScore) float64 {
	s := 85.0
	if score != nil {
		s -= (100 - score.RawScore) * 0.4
	}
	risks := extractRisks(steps, year)
	s -= float64(len(risks)) * 5
	return math.Max(0, math.Min(100, s))
}

func reverseComment(steps []StepResult, year string, score *YearScore) string {
	s := reverseScore(steps, year, score)
	if s >= 75 {
		return "风险可控，负面因素较少"
	} else if s >= 60 {
		return "存在一定风险，需关注短板"
	} else if s >= 40 {
		return "风险点较多，谨慎对待"
	}
	return "风险较高，建议回避"
}

func buffettScore(steps []StepResult, year string, score *YearScore) float64 {
	roe := getStepValue(steps, 16, year, "roe")
	gm := getStepValue(steps, 10, year, "grossMargin")
	cr := getStepValue(steps, 15, year, "cashRatio")
	dr := getStepValue(steps, 3, year, "debtRatio")
	ia := getStepValue(steps, 7, year, "ratio")

	s := 5.0
	if roe >= 15 {
		s += 1.5
	}
	if gm >= 40 {
		s += 1
	}
	if getStepValue(steps, 8, year, "AScore") < 50 {
		s += 1
	}
	if cr >= 100 {
		s += 0.5
	}
	if dr <= 60 {
		s += 0.5
	}
	if ia <= 10 {
		s += 0.5
	}
	return math.Min(10, s)
}

func buffettComment(steps []StepResult, year string, score *YearScore) string {
	s := buffettScore(steps, year, score)
	if s >= 8 {
		return "基本满足巴芒投资标准"
	} else if s >= 6 {
		return "勉强及格，部分维度待提升"
	} else if s >= 4 {
		return "不满足标准，需等待改善"
	}
	return "明显偏离标准，建议回避"
}

func positiveFactors(steps []StepResult, year string) []string {
	var ps []string
	if g := getStepValue(steps, 9, year, "growthRate"); g >= 10 {
		ps = append(ps, fmt.Sprintf("营收保持 %.2f%% 增长，业务仍在扩张", g))
	}
	if pg := getStepValue(steps, 16, year, "profitGrowth"); pg >= 10 {
		ps = append(ps, fmt.Sprintf("净利润增长 %.2f%%，盈利能力改善", pg))
	}
	if roe := getStepValue(steps, 16, year, "roe"); roe >= 15 {
		ps = append(ps, fmt.Sprintf("ROE %.2f%%，资本回报能力优秀", roe))
	}
	if gm := getStepValue(steps, 10, year, "grossMargin"); gm >= 40 {
		ps = append(ps, fmt.Sprintf("毛利率 %.2f%%，护城河较深", gm))
	}
	if dr := getStepValue(steps, 3, year, "debtRatio"); dr <= 40 {
		ps = append(ps, fmt.Sprintf("资产负债率 %.2f%%，财务结构稳健", dr))
	}
	if cr := getStepValue(steps, 15, year, "cashRatio"); cr >= 100 {
		ps = append(ps, fmt.Sprintf("净利润现金含量 %.2f%%，盈利质量高", cr))
	}
	if ascore := getStepValue(steps, 8, year, "AScore"); ascore < 50 {
		ps = append(ps, fmt.Sprintf("A-Score %.1f，综合财务风险可控", ascore))
	}
	return ps
}

func getStepValue(steps []StepResult, stepNum int, year, key string) float64 {
	for _, s := range steps {
		if s.StepNum != stepNum {
			continue
		}
		yd, ok := s.YearlyData[year]
		if !ok {
			return 0
		}
		return anyToFloat64(yd[key])
	}
	return 0
}

func countCategoryPass(steps []StepResult, stepNums []int, year string) (int, int) {
	pass := 0
	total := 0
	stepMap := make(map[int]bool)
	for _, n := range stepNums {
		stepMap[n] = true
	}
	for _, s := range steps {
		if !stepMap[s.StepNum] {
			continue
		}
		p, ok := s.Pass[year]
		if !ok {
			continue
		}
		total++
		if p {
			pass++
		}
	}
	return pass, total
}

func extractRisks(steps []StepResult, year string) []RiskItem {
	var risks []RiskItem
	for _, s := range steps {
		p, ok := s.Pass[year]
		if !ok || p {
			continue
		}
		switch s.StepNum {
		case 3:
			dr := getStepValue(steps, 3, year, "debtRatio")
			diff := getStepValue(steps, 3, year, "cashDebtDiff")
			if dr > 60 {
				risks = append(risks, RiskItem{"偿债风险", fmt.Sprintf("资产负债率%.1f%%", dr), "🔴 高", "负债率超过60%警戒线"})
			} else if diff < 0 {
				risks = append(risks, RiskItem{"偿债风险", fmt.Sprintf("准货币资金缺口%.1f亿", -diff/1e8), "🟠 中高", "现金未能覆盖有息负债"})
			}
		case 4:
			diff := getStepValue(steps, 4, year, "diff")
			risks = append(risks, RiskItem{"产业链地位", fmt.Sprintf("两头吃差额%.1f亿", diff/1e8), "🟠 中高", "对上下游议价能力偏弱"})
		case 5:
			ratio := getStepValue(steps, 5, year, "ratio")
			if ratio > 20 {
				risks = append(risks, RiskItem{"回款风险", fmt.Sprintf("应收账款占比%.1f%%", ratio), "🔴 高", "回款压力大，销售回款慢"})
			} else {
				risks = append(risks, RiskItem{"回款风险", fmt.Sprintf("应收账款占比%.1f%%", ratio), "🟠 中高", "应收占比偏高"})
			}
		case 6:
			ratio := getStepValue(steps, 6, year, "ratio")
			if ratio > 40 {
				risks = append(risks, RiskItem{"资产结构", fmt.Sprintf("固定资产占比%.1f%%", ratio), "🟠 中高", "重资产模式，维持成本高"})
			}
		case 7:
			ratio := getStepValue(steps, 7, year, "ratio")
			risks = append(risks, RiskItem{"主业专注", fmt.Sprintf("投资类资产占比%.1f%%", ratio), "🟠 中高", "主业专注度不足"})
		case 8:
			ascore := getStepValue(steps, 8, year, "AScore")
			if ascore >= 60 {
				severity := "🟠 中高"
				if ascore >= 70 {
					severity = "🔴 高"
				}
				risks = append(risks, RiskItem{"财务造假/偿债风险", fmt.Sprintf("A-Score %.1f", ascore), severity, "存在财报操纵或偿债压力嫌疑，建议深入核查"})
			}
		case 9:
			g := getStepValue(steps, 9, year, "growthRate")
			if g < 0 {
				risks = append(risks, RiskItem{"成长风险", fmt.Sprintf("营收增长%.1f%%", g), "🔴 高", "营业收入出现负增长"})
			} else if g < 10 {
				risks = append(risks, RiskItem{"成长风险", fmt.Sprintf("营收增长%.1f%%", g), "🟠 中高", "营收增长未达10%理想水平"})
			}
		case 10:
			gm := getStepValue(steps, 10, year, "grossMargin")
			if gm < 20 {
				risks = append(risks, RiskItem{"盈利质量", fmt.Sprintf("毛利率%.1f%%", gm), "🔴 高", "毛利率偏低，产品竞争力弱"})
			} else if gm < 40 {
				risks = append(risks, RiskItem{"盈利质量", fmt.Sprintf("毛利率%.1f%%", gm), "🟠 中高", "毛利率未达高毛利标准"})
			}
		case 16:
			roe := getStepValue(steps, 16, year, "roe")
			if roe < 15 {
				risks = append(risks, RiskItem{"资本回报", fmt.Sprintf("ROE %.1f%%", roe), "🟠 中高", "ROE低于15%理想水平"})
			}
		}
	}
	return risks
}

func writeMetricRow(b *strings.Builder, name string, latestVal, prevVal float64, unit string, div float64) {
	lv, pv := latestVal, prevVal
	if div != 1 {
		lv /= div
		pv /= div
	}
	change := "-"
	if pv != 0 {
		pct := (latestVal - prevVal) / math.Abs(prevVal) * 100
		emoji := "➡️"
		if pct > 0 {
			emoji = "📈"
		} else if pct < 0 {
			emoji = "📉"
		}
		change = fmt.Sprintf("%s %.2f%%", emoji, pct)
	}
	assess := "🟡 持平"
	if latestVal > prevVal {
		assess = "🟢 上升"
	} else if latestVal < prevVal {
		assess = "🔴 下降"
	}
	if name == "资产负债率" || name == "期间费用率/毛利率" {
		if latestVal < prevVal {
			assess = "🟢 优化"
		} else if latestVal > prevVal {
			assess = "🔴 恶化"
		}
	}
	if prevVal == 0 {
		assess = "-"
		change = "-"
	}
	b.WriteString(fmt.Sprintf("| **%s** | %.2f%s | %.2f%s | %s | %s |\n", name, lv, unit, pv, unit, change, assess))
}

func fmtVal(v float64, unit string) string {
	if v == 0 {
		return "-"
	}
	if unit == "%" {
		return fmt.Sprintf("%.2f%%", v)
	}
	return fmt.Sprintf("%.3f", v)
}

func infoIcon(key string) string {
	return fmt.Sprintf(`<span class="info-icon" data-key="%s">ℹ️</span>`, key)
}

func traceTrigger(stepNums ...int) string {
	if len(stepNums) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(` <span class="trace-trigger" data-steps="`)
	for i, n := range stepNums {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(fmt.Sprintf("%d", n))
	}
	sb.WriteString(`">ℹ️</span>`)
	return sb.String()
}

func yoyFmt(cur, prev float64) string {
	if prev == 0 {
		return "-"
	}
	pct := (cur - prev) / math.Abs(prev) * 100
	return fmt.Sprintf("%.2f%%", pct)
}

func yoyEmoji(cur, prev float64) string {
	if prev == 0 {
		return "-"
	}
	pct := (cur - prev) / math.Abs(prev) * 100
	if pct > 0 {
		return "🟢 增长"
	} else if pct < 0 {
		return "🔴 下降"
	}
	return "➡️ 持平"
}

func roeEmoji(v float64) string {
	if v >= 15 {
		return "🟢 优秀"
	} else if v >= 10 {
		return "🟡 一般"
	} else if v > 0 {
		return "🔴 偏弱"
	}
	return "🔴 极差"
}

func gmEmoji(v float64) string {
	if v >= 40 {
		return "🟢 高毛利"
	} else if v >= 30 {
		return "🟡 中等"
	} else if v >= 20 {
		return "🔴 偏低"
	}
	return "🔴 过低"
}

func drEmoji(v float64) string {
	if v <= 40 {
		return "🟢 安全"
	} else if v <= 60 {
		return "🟡 适中"
	} else if v <= 70 {
		return "🔴 偏高"
	}
	return "🔴 危险"
}

func asEmoji(v float64) string {
	if v < 50 {
		return "🟢 安全"
	} else if v < 60 {
		return "🟡 关注"
	}
	return "🔴 风险"
}

func moatComment(gm, roe float64) string {
	if gm >= 40 && roe >= 15 {
		return "高毛利+高ROE，护城河较深"
	} else if gm >= 30 {
		return "毛利率尚可，竞争壁垒中等"
	}
	return "毛利率偏低，护城河待验证"
}

func auditCommentAScore(ascore float64) string {
	if ascore < 50 {
		return "，审计质量可信"
	} else if ascore < 60 {
		return "，需关注"
	}
	return "，风险较高"
}

func mapScore(val, good, bad, max float64) float64 {
	if val >= good {
		return max
	}
	if val <= bad {
		return 1
	}
	return 1 + (max-1)*(val-bad)/(good-bad)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func turnoverAssessment(tr float64) string {
	if tr >= 7 {
		return "非常活跃"
	}
	if tr >= 3 {
		return "活跃"
	}
	if tr >= 1 {
		return "正常"
	}
	return "低迷"
}

func volumeRatioAssessment(vr float64) string {
	if vr >= 2 {
		return "放量"
	}
	if vr >= 0.8 {
		return "正常"
	}
	return "缩量"
}

func gapDirection(gap float64) string {
	if gap > 0 {
		return "高开运行"
	}
	return "低开运行"
}

func writeComparableTrendComment(b *strings.Builder, steps []StepResult, comp *ComparableAnalysis) {
	if len(comp.CommonYears) < 2 {
		return
	}
	latest := comp.CommonYears[0]
	oldest := comp.CommonYears[len(comp.CommonYears)-1]

	latestAvg := comp.YearlyTrends[0].Average
	oldestAvg := comp.YearlyTrends[len(comp.YearlyTrends)-1].Average

	latestROE := getStepValue(steps, 16, latest, "roe")
	oldestROE := getStepValue(steps, 16, oldest, "roe")
	roeGap := latestROE - latestAvg.ROE

	if latestROE > oldestROE && latestAvg.ROE < oldestAvg.ROE {
		b.WriteString("- **ROE 走势优异**：公司 ROE 呈上升趋势，而可比均值在下降，竞争优势在扩大。\n")
	} else if latestROE < oldestROE && latestAvg.ROE > oldestAvg.ROE {
		b.WriteString("- **ROE 走势承压**：公司 ROE 下滑，而可比均值在提升，相对竞争力在减弱。\n")
	} else if latestROE > oldestROE {
		b.WriteString("- **ROE 持续改善**：公司与可比均值同步或独立改善。\n")
	} else if latestROE < oldestROE {
		b.WriteString("- **ROE 有所回落**：公司 ROE 较历史高点下降，需关注盈利能力变化。\n")
	}

	if roeGap >= 5 {
		b.WriteString(fmt.Sprintf("- **ROE 领先可比均值 %.2f 个百分点**，资本回报能力在可比公司中处于优势地位。\n", roeGap))
	} else if roeGap <= -5 {
		b.WriteString(fmt.Sprintf("- **ROE 落后可比均值 %.2f 个百分点**，资本回报能力相对偏弱。\n", -roeGap))
	}

	latestGM := getStepValue(steps, 10, latest, "grossMargin")
	oldestGM := getStepValue(steps, 10, oldest, "grossMargin")
	if latestGM > oldestGM+3 {
		b.WriteString("- **毛利率持续提升**：定价权或成本控制能力在改善。\n")
	} else if latestGM < oldestGM-3 {
		b.WriteString("- **毛利率有所下滑**：产品竞争力或行业竞争格局可能趋紧。\n")
	}

	latestDR := getStepValue(steps, 3, latest, "debtRatio")
	oldestDR := getStepValue(steps, 3, oldest, "debtRatio")
	if latestDR < oldestDR-5 {
		b.WriteString("- **负债率明显下降**：财务结构在优化，偿债安全性提升。\n")
	} else if latestDR > oldestDR+5 {
		b.WriteString("- **负债率明显上升**：杠杆扩张较快，需关注偿债压力。\n")
	}

	latestCR := getStepValue(steps, 15, latest, "cashRatio")
	oldestCR := getStepValue(steps, 15, oldest, "cashRatio")
	if latestCR > oldestCR+10 {
		b.WriteString("- **现金流质量改善**：盈利含金量在提升。\n")
	} else if latestCR < oldestCR-10 {
		b.WriteString("- **现金流质量下滑**：净利润中的现金比例在下降。\n")
	}
}

func ascoreComment(v float64) string {
	if v >= 70 {
		return "综合财务风险较高，建议深入核查"
	}
	if v >= 60 {
		return "综合财务风险中等，需保持关注"
	}
	if v >= 40 {
		return "综合财务风险可控"
	}
	return "综合财务风险低，基本面相对稳健"
}

func ascoreBadge(v float64) string {
	if v >= 70 {
		return "🔴 高风险（A-Score ≥ 70）"
	}
	if v >= 60 {
		return "🟡 中风险（A-Score 60-70）"
	}
	if v >= 40 {
		return "🟢 低风险（A-Score 40-60）"
	}
	return "🟢 安全（A-Score < 40）"
}

func normalizeScore(value, min, max float64, reverse bool) float64 {
	if max == min {
		return 50
	}
	if reverse {
		return (max - value) / (max - min) * 100
	}
	return (value - min) / (max - min) * 100
}

func medianActivityScore(list []*ComparableMetrics) float64 {
	var vals []float64
	for _, m := range list {
		if m.ActivityScore >= 0 {
			vals = append(vals, m.ActivityScore)
		}
	}
	if len(vals) == 0 {
		return 50
	}
	for i := 0; i < len(vals); i++ {
		for j := i + 1; j < len(vals); j++ {
			if vals[i] > vals[j] {
				vals[i], vals[j] = vals[j], vals[i]
			}
		}
	}
	mid := len(vals) / 2
	if len(vals)%2 == 1 {
		return vals[mid]
	}
	return (vals[mid-1] + vals[mid]) / 2
}

func calcComparableScore(m *ComparableMetrics, all []*ComparableMetrics, medianActivity float64) float64 {
	var minROE, maxROE, minGM, maxGM, minGrowth, maxGrowth, minDebt, maxDebt, minCash, maxCash, minAScore, maxAScore, minAct, maxAct float64
	first := true
	for _, x := range all {
		if first {
			minROE, maxROE = x.ROE, x.ROE
			minGM, maxGM = x.GrossMargin, x.GrossMargin
			minGrowth, maxGrowth = x.RevenueGrowth, x.RevenueGrowth
			minDebt, maxDebt = x.DebtRatio, x.DebtRatio
			minCash, maxCash = x.CashRatio, x.CashRatio
			minAScore, maxAScore = x.AScore, x.AScore
			first = false
			continue
		}
		if x.ROE < minROE { minROE = x.ROE }
		if x.ROE > maxROE { maxROE = x.ROE }
		if x.GrossMargin < minGM { minGM = x.GrossMargin }
		if x.GrossMargin > maxGM { maxGM = x.GrossMargin }
		if x.RevenueGrowth < minGrowth { minGrowth = x.RevenueGrowth }
		if x.RevenueGrowth > maxGrowth { maxGrowth = x.RevenueGrowth }
		if x.DebtRatio < minDebt { minDebt = x.DebtRatio }
		if x.DebtRatio > maxDebt { maxDebt = x.DebtRatio }
		if x.CashRatio < minCash { minCash = x.CashRatio }
		if x.CashRatio > maxCash { maxCash = x.CashRatio }
		if x.AScore < minAScore { minAScore = x.AScore }
		if x.AScore > maxAScore { maxAScore = x.AScore }
	}
	firstAct := true
	for _, x := range all {
		act := x.ActivityScore
		if act < 0 {
			act = medianActivity
		}
		if firstAct {
			minAct, maxAct = act, act
			firstAct = false
			continue
		}
		if act < minAct { minAct = act }
		if act > maxAct { maxAct = act }
	}
	if firstAct {
		minAct, maxAct = 0, 100
	}

	act := m.ActivityScore
	if act < 0 {
		act = medianActivity
	}

	s := normalizeScore(m.ROE, minROE, maxROE, false)*0.25 +
		normalizeScore(m.GrossMargin, minGM, maxGM, false)*0.20 +
		normalizeScore(m.RevenueGrowth, minGrowth, maxGrowth, false)*0.15 +
		normalizeScore(m.DebtRatio, minDebt, maxDebt, true)*0.10 +
		normalizeScore(m.CashRatio, minCash, maxCash, false)*0.10 +
		normalizeScore(m.AScore, minAScore, maxAScore, true)*0.10 +
		normalizeScore(act, minAct, maxAct, false)*0.10
	return s
}

// HighlightRisk 亮点与风险摘要
type HighlightRisk struct {
	Highlights []string `json:"highlights"`
	Risks      []string `json:"risks"`
}

// ExtractHighlightsAndRisks 从分析步骤中提取亮点与风险摘要
func ExtractHighlightsAndRisks(steps []StepResult, years []string) HighlightRisk {
	if len(years) == 0 {
		return HighlightRisk{Highlights: []string{}, Risks: []string{}}
	}
	latest := years[0]

	roe := getStepValue(steps, 16, latest, "roe")
	gm := getStepValue(steps, 10, latest, "grossMargin")
	growth := getStepValue(steps, 9, latest, "growthRate")
	pg := getStepValue(steps, 16, latest, "profitGrowth")
	ascore := getStepValue(steps, 8, latest, "AScore")
	dr := getStepValue(steps, 3, latest, "debtRatio")
	cr := getStepValue(steps, 15, latest, "cashRatio")

	var highlights, risks []string

	if roe >= 15 {
		highlights = append(highlights, "ROE 优秀，资本回报能力强")
	} else {
		risks = append(risks, "ROE 低于 15%，资本回报能力有待提升")
	}

	if gm >= 40 {
		highlights = append(highlights, "高毛利率，定价权稳固")
	} else {
		risks = append(risks, "毛利率未达 40%，产品竞争力一般")
	}

	if dr <= 40 {
		highlights = append(highlights, "低负债率，财务结构稳健")
	} else if dr > 60 {
		risks = append(risks, "负债率超过 60%，偿债压力偏大")
	}

	if ascore < 40 {
		highlights = append(highlights, "A-Score 安全，财务质量良好")
	} else if ascore < 60 {
		highlights = append(highlights, "A-Score 低风险，财务质量可控")
	} else if ascore < 70 {
		risks = append(risks, "A-Score 中风险，需关注财务健康度")
	} else {
		risks = append(risks, "A-Score 高风险，建议谨慎")
	}

	if growth >= 10 {
		highlights = append(highlights, "营收稳健增长")
	} else if growth < 0 {
		risks = append(risks, "营收负增长，成长性承压")
	}

	if pg >= 10 {
		highlights = append(highlights, "净利润持续增长")
	} else if pg < 0 {
		risks = append(risks, "净利润下滑，盈利能力减弱")
	}

	if cr >= 100 {
		highlights = append(highlights, "经营现金流充沛，盈利质量高")
	} else if cr > 0 {
		risks = append(risks, "现金流含金量不足")
	}

	return HighlightRisk{Highlights: highlights, Risks: risks}
}

// aScoreTooltip 返回 A-Score 指标说明的 Markdown 折叠块
func aScoreTooltip() string {
	return `<details>
<summary>ℹ️ A-Score 是什么？</summary>

**A-Score（0-100分）综合评估企业财务风险，分数越高，潜在隐患越大。**

基于公开财务报表与监管信息，从六个维度打分：**财务造假风险、偿债能力、现金流质量、应收账款健康度、盈利稳定性**，以及**股权质押/减持/监管问询**等非财务信号。其中财务维度适用于 A 股与港股，非财务信号目前主要覆盖 A 股。

**评判标准**：< 40分安全，40-60分低风险，60-70分中风险（需深入核查），≥ 70分高危（建议回避）。

**核心价值**：在财报暴雷前发现财务隐患，快速筛掉有明显问题的公司。

</details>

`
}
