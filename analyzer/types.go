package analyzer

// CalcTrace 单个指标的计算溯源记录
type CalcTrace struct {
	Indicator string                 `json:"indicator"`
	Year      string                 `json:"year"`
	Formula   string                 `json:"formula"`
	Inputs    map[string]InputValue  `json:"inputs"`
	Steps     []CalcStep             `json:"steps"`
	Result    float64                `json:"result"`
	Note      string                 `json:"note"`
}

type InputValue struct {
	Source string  `json:"source"`
	Item   string  `json:"item"`
	Year   string  `json:"year"`
	Value  float64 `json:"value"`
	Note   string  `json:"note"`
}

type CalcStep struct {
	Desc  string  `json:"desc"`
	Expr  string  `json:"expr"`
	Value float64 `json:"value"`
}

// StepResult 单步分析结果
type StepResult struct {
	StepNum    int                             `json:"stepNum"`
	StepName   string                          `json:"stepName"`
	YearlyData map[string]map[string]any       `json:"yearlyData"` // 年份 -> 指标名 -> 值
	Conclusion string                          `json:"conclusion"`
	Pass       map[string]bool                 `json:"pass"`       // 年份 -> 是否达标
	Traces     []CalcTrace                     `json:"traces"`     // 计算溯源
}

// AnalysisReport 完整分析报告
type AnalysisReport struct {
	Symbol          string                 `json:"symbol"`
	CompanyName     string                 `json:"companyName"`
	Years           []string               `json:"years"`
	StepResults     []StepResult           `json:"stepResults"`
	PassSummary     map[string][]PassItem  `json:"passSummary"`
	Score           map[string]float64     `json:"score"`
	OverallGrade    string                 `json:"overallGrade"`
	MarkdownContent string                 `json:"markdownContent"`
	RIM             *RIMData               `json:"rim,omitempty"`
}

// PassItem 单一年度的达标项
type PassItem struct {
	Year   string `json:"year"`
	Passed bool   `json:"passed"`
	Value  any    `json:"value"`
}

// QuoteData 实时行情数据（用于报告填充）
type QuoteData struct {
	CurrentPrice         float64 `json:"currentPrice"`
	ChangePercent        float64 `json:"changePercent"`
	ChangeAmount         float64 `json:"changeAmount"`
	Volume               float64 `json:"volume"`
	TurnoverAmount       float64 `json:"turnoverAmount"`
	TurnoverRate         float64 `json:"turnoverRate"`
	Amplitude            float64 `json:"amplitude"`
	High                 float64 `json:"high"`
	Low                  float64 `json:"low"`
	Open                 float64 `json:"open"`
	PreviousClose        float64 `json:"previousClose"`
	CirculatingMarketCap float64 `json:"circulatingMarketCap"`
	VolumeRatio          float64 `json:"volumeRatio"`
	PE                   float64 `json:"pe"`
	PB                   float64 `json:"pb"`
	MarketCap            float64 `json:"marketCap"`
}

// SentimentSummary 单条舆情摘要
type SentimentSummary struct {
	Title     string  `json:"title"`
	Source    string  `json:"source"`
	Date      string  `json:"date"`
	Sentiment float64 `json:"sentiment"` // [-1, 1]
}

// SentimentData 社交媒体/舆情情绪数据
type SentimentData struct {
	Score          float64            `json:"score"`
	HeatIndex      int                `json:"heatIndex"`
	PositiveWords  []string           `json:"positiveWords"`
	NegativeWords  []string           `json:"negativeWords"`
	Summaries      []SentimentSummary `json:"summaries"`
	HasData        bool               `json:"hasData"`
}

// MLSummary 基于双引擎融合的2-4周综合预测摘要
type MLSummary struct {
	Direction string  `json:"direction"`
	RangeLow  float64 `json:"rangeLow"`
	RangeHigh float64 `json:"rangeHigh"`
	Reason    string  `json:"reason"`
	HasData   bool    `json:"hasData"`
}

// MLPredictionData 机器学习预测结果
type MLPredictionData struct {
	Sentiment *MLSentimentPrediction `json:"sentiment,omitempty"`
	Financial *MLFinancialPrediction `json:"financial,omitempty"`
	EngineD   *MLDRiskPrediction     `json:"engine_d,omitempty"`
	Summary   *MLSummary             `json:"summary,omitempty"`
}

// RIMData 剩余收益模型数据
type RIMData struct {
	HasData bool       `json:"hasData"`
	Params  RIMParams  `json:"params"`
	Result  *RIMResult `json:"result,omitempty"`
	Error   string     `json:"error,omitempty"`
	EPSRaw  map[string]float64 `json:"epsRaw,omitempty"`
	Rf      float64    `json:"rf"`
	Beta    float64    `json:"beta"`
	RmRf    float64    `json:"rmRf"`
}

