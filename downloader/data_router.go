package downloader

import (
	"fmt"
	"time"
)

// DataRouter 数据源路由器，根据配置自动选择最优数据源
type DataRouter struct {
	tushareClient *TushareClient
	tushareEnabled bool
	useForFinancial bool
	useForKline     bool
	useForQuote     bool
	useForMoneyflow bool
}

// NewDataRouter 创建数据源路由器
func NewDataRouter(token string, enabled, useFin, useKline, useQuote, useMF bool) *DataRouter {
	r := &DataRouter{
		tushareEnabled:  enabled && token != "",
		useForFinancial: useFin,
		useForKline:     useKline,
		useForQuote:     useQuote,
		useForMoneyflow: useMF,
	}
	if r.tushareEnabled {
		r.tushareClient = NewTushareClient(token)
	}
	return r
}

// ========== K线数据路由 ==========

// FetchKlines 获取历史K线，按优先级路由
func (r *DataRouter) FetchKlines(market, code string, limit int) ([]KlineData, error) {
	// 1. StockFinLens 数据源（如果启用）
	if r.tushareEnabled && r.useForKline && r.tushareClient != nil {
		end := time.Now().Format("20060102")
		start := time.Now().AddDate(-2, 0, 0).Format("20060102")
		if klines, err := r.tushareClient.FetchDaily(market, code, start, end); err == nil && len(klines) > 0 {
			fmt.Printf("[DataRouter] Klines from StockFinLens: %d bars for %s.%s\n", len(klines), market, code)
			if len(klines) > limit {
				return klines[len(klines)-limit:], nil
			}
			return klines, nil
		}
	}

	// 2. 腾讯财经
	fmt.Printf("[DataRouter] Klines fallback to Tencent for %s.%s\n", market, code)
	if klines, err := fetchKlinesFromTencent(market, code, limit); err == nil && len(klines) > 0 {
		return klines, nil
	}

	// 3. 网易财经
	fmt.Printf("[DataRouter] Klines fallback to NetEase for %s.%s\n", market, code)
	if klines, err := fetchKlinesFromNetEase(market, code, limit); err == nil && len(klines) > 0 {
		return klines, nil
	}

	// 4. Yahoo Finance
	fmt.Printf("[DataRouter] Klines fallback to Yahoo for %s.%s\n", market, code)
	if klines, err := fetchKlinesFromYahoo(market, code, limit); err == nil && len(klines) > 0 {
		return klines, nil
	}

	// 5. 东方财富（最后兜底）
	fmt.Printf("[DataRouter] Klines fallback to EastMoney for %s.%s\n", market, code)
	return FetchStockKlines(market, code, limit)
}

// ========== 实时行情路由 ==========

// FetchQuote 获取实时行情，按优先级路由
func (r *DataRouter) FetchQuote(market, code string) (*StockQuote, error) {
	// 实时行情不走 StockFinLens（daily_basic 是盘后数据）
	// 1. 腾讯财经（最稳定）
	fmt.Printf("[DataRouter] Quote trying Tencent for %s.%s\n", market, code)
	if quote, err := fetchQuoteFromTencent(market, code); err == nil && quote != nil && quote.CurrentPrice > 0 {
		fmt.Printf("[DataRouter] Quote from Tencent: %.2f for %s.%s\n", quote.CurrentPrice, market, code)
		return quote, nil
	}

	// 2. 东方财富
	fmt.Printf("[DataRouter] Quote fallback to EastMoney for %s.%s\n", market, code)
	return FetchStockQuote(market, code)
}

// ========== 每日指标路由 ==========

// FetchDailyMetrics 获取每日指标（PE/PB/市值/换手率），按优先级路由
func (r *DataRouter) FetchDailyMetrics(market, code, tradeDate string) (*StockQuote, error) {
	// 1. StockFinLens daily_basic（如果启用）
	if r.tushareEnabled && r.useForQuote && r.tushareClient != nil {
		if quote, err := r.tushareClient.FetchDailyBasic(market, code, tradeDate); err == nil && quote != nil && quote.CurrentPrice > 0 {
			fmt.Printf("[DataRouter] Metrics from StockFinLens for %s.%s\n", market, code)
			return quote, nil
		}
	}

	// 2. 腾讯财经（含PE/PB/市值）
	fmt.Printf("[DataRouter] Metrics fallback to Tencent for %s.%s\n", market, code)
	if quote, err := fetchQuoteFromTencent(market, code); err == nil && quote != nil && quote.CurrentPrice > 0 {
		return quote, nil
	}

	// 3. 东方财富
	fmt.Printf("[DataRouter] Metrics fallback to EastMoney for %s.%s\n", market, code)
	return FetchStockQuote(market, code)
}

// ========== 财报数据路由 ==========

// TushareFinancialData 封装数据源财务数据
type TushareFinancialData struct {
	Income       []TushareIncomeItem
	BalanceSheet []TushareBalanceItem
	Cashflow     []TushareCashflowItem
	Indicators   []TushareFinaIndicator
}

// FetchFinancialData 获取财务数据，按优先级路由
func (r *DataRouter) FetchFinancialData(market, code string) (*TushareFinancialData, error) {
	// 1. StockFinLens 数据源（如果启用）
	if r.tushareEnabled && r.useForFinancial && r.tushareClient != nil {
		fmt.Printf("[DataRouter] Financial from StockFinLens for %s.%s\n", market, code)
		start := time.Now().AddDate(-5, 0, 0).Format("20060102")
		end := time.Now().Format("20060102")

		var data TushareFinancialData
		var hasData bool

		if income, err := r.tushareClient.FetchIncome(market, code, start, end); err == nil && len(income) > 0 {
			data.Income = income
			hasData = true
		}
		if bs, err := r.tushareClient.FetchBalanceSheet(market, code, start, end); err == nil && len(bs) > 0 {
			data.BalanceSheet = bs
			hasData = true
		}
		if cf, err := r.tushareClient.FetchCashflow(market, code, start, end); err == nil && len(cf) > 0 {
			data.Cashflow = cf
			hasData = true
		}
		if ind, err := r.tushareClient.FetchFinaIndicator(market, code, start, end); err == nil && len(ind) > 0 {
			data.Indicators = ind
			hasData = true
		}

		if hasData {
			return &data, nil
		}
	}

	// 2. 东方财富
	fmt.Printf("[DataRouter] Financial fallback to EastMoney for %s.%s\n", market, code)
	return nil, fmt.Errorf("数据源未启用或未获取到数据，请使用 EastMoney 下载")
}

// toYearKey 将 Tushare 日期格式 20241231 转换为 2024-12-31
func toYearKey(endDate string) string {
	if len(endDate) == 8 {
		return endDate[:4] + "-" + endDate[4:6] + "-" + endDate[6:]
	}
	return endDate
}

// ConvertToFinancialReportData 将 Tushare 财务数据转换为标准 FinancialReportData
func (r *DataRouter) ConvertToFinancialReportData(tfd *TushareFinancialData, symbol string) *FinancialReportData {
	result := &FinancialReportData{
		Symbol:          symbol,
		Years:           make([]string, 0),
		BalanceSheet:    make(map[string]map[string]float64),
		IncomeStatement: make(map[string]map[string]float64),
		CashFlow:        make(map[string]map[string]float64),
	}

	yearSet := make(map[string]struct{})

	// 收入表
	for _, item := range tfd.Income {
		year := toYearKey(item.EndDate)
		yearSet[year] = struct{}{}
		setVal(result.IncomeStatement, "营业收入", year, item.Revenue)
		setVal(result.IncomeStatement, "营业总成本", year, item.TotalCogs)
		setVal(result.IncomeStatement, "营业成本", year, item.OperateCost)
		setVal(result.IncomeStatement, "销售费用", year, item.SellExp)
		setVal(result.IncomeStatement, "管理费用", year, item.AdminExp)
		setVal(result.IncomeStatement, "研发费用", year, item.RDExp)
		setVal(result.IncomeStatement, "财务费用", year, item.FinExp)
		setVal(result.IncomeStatement, "营业利润", year, item.OperateProfit)
		setVal(result.IncomeStatement, "利润总额", year, item.TotalProfit)
		setVal(result.IncomeStatement, "净利润", year, item.NetIncome)
		setVal(result.IncomeStatement, "归母净利润", year, item.ParentNetIncome)
		setVal(result.IncomeStatement, "基本每股收益", year, item.EPS)
	}

	// 资产负债表
	for _, item := range tfd.BalanceSheet {
		year := toYearKey(item.EndDate)
		yearSet[year] = struct{}{}
		setVal(result.BalanceSheet, "资产合计", year, item.TotalAssets)
		setVal(result.BalanceSheet, "负债合计", year, item.TotalLiab)
		setVal(result.BalanceSheet, "所有者权益合计", year, item.TotalHldrEqy)
		setVal(result.BalanceSheet, "货币资金", year, item.MoneyCap)
		setVal(result.BalanceSheet, "交易性金融资产", year, item.TradAsset)
		setVal(result.BalanceSheet, "应收票据", year, item.NotesReceiv)
		setVal(result.BalanceSheet, "应收账款", year, item.AccountsReceiv)
		setVal(result.BalanceSheet, "预付款项", year, item.Prepayment)
		setVal(result.BalanceSheet, "合同资产", year, item.ContractAsset)
		setVal(result.BalanceSheet, "存货", year, item.Inventories)
		setVal(result.BalanceSheet, "流动资产合计", year, item.TotalCurAssets)
		setVal(result.BalanceSheet, "固定资产", year, item.FixAssets)
		setVal(result.BalanceSheet, "在建工程", year, item.CIP)
		setVal(result.BalanceSheet, "工程物资", year, item.ConstMaterials)
		setVal(result.BalanceSheet, "无形资产", year, item.IntanAssets)
		setVal(result.BalanceSheet, "商誉", year, item.Goodwill)
		setVal(result.BalanceSheet, "非流动资产合计", year, item.TotalNca)
		setVal(result.BalanceSheet, "长期股权投资", year, item.LtEqtInvest)
		setVal(result.BalanceSheet, "其他权益工具投资", year, item.OthEqtInvest)
		setVal(result.BalanceSheet, "其他非流动资产", year, item.OthNca)
		setVal(result.BalanceSheet, "短期借款", year, item.ShortLoan)
		setVal(result.BalanceSheet, "长期借款", year, item.LongLoan)
		setVal(result.BalanceSheet, "应付债券", year, item.BondsPayable)
		setVal(result.BalanceSheet, "应付票据", year, item.NotesPayable)
		setVal(result.BalanceSheet, "应付账款", year, item.AccountsPay)
		setVal(result.BalanceSheet, "预收款项", year, item.AdvReceipts)
		setVal(result.BalanceSheet, "合同负债", year, item.ContractLiab)
		setVal(result.BalanceSheet, "应付职工薪酬", year, item.SalaryPayable)
		setVal(result.BalanceSheet, "应交税费", year, item.TaxPayable)
		setVal(result.BalanceSheet, "流动负债合计", year, item.TotalCurLiab)
		setVal(result.BalanceSheet, "非流动负债合计", year, item.TotalNcl)
		setVal(result.BalanceSheet, "递延所得税资产", year, item.DeferTaxAsset)
		setVal(result.BalanceSheet, "递延所得税负债", year, item.DeferTaxLiab)
		setVal(result.BalanceSheet, "实收资本（或股本）", year, item.ShareCapital)
		setVal(result.BalanceSheet, "资本公积", year, item.CapRese)
		setVal(result.BalanceSheet, "盈余公积", year, item.SurplusRese)
		setVal(result.BalanceSheet, "未分配利润", year, item.UndistProfit)
		setVal(result.BalanceSheet, "少数股东权益", year, item.MinorityInt)
		// 计算应收票据及应收账款 = 应收票据 + 应收账款
		setVal(result.BalanceSheet, "应收票据及应收账款", year, item.NotesReceiv+item.AccountsReceiv)
		// 计算应付票据及应付账款 = 应付票据 + 应付账款
		setVal(result.BalanceSheet, "应付票据及应付账款", year, item.NotesPayable+item.AccountsPay)
		// 归母所有者权益 = 股东权益合计 - 少数股东权益
		setVal(result.BalanceSheet, "归属于母公司所有者权益合计", year, item.TotalHldrEqy-item.MinorityInt)
	}

	// 现金流量表
	for _, item := range tfd.Cashflow {
		year := toYearKey(item.EndDate)
		yearSet[year] = struct{}{}
		setVal(result.CashFlow, "经营活动产生的现金流量净额", year, item.NCashflowAct)
		setVal(result.CashFlow, "投资活动产生的现金流量净额", year, item.NCashflowInv)
		setVal(result.CashFlow, "筹资活动产生的现金流量净额", year, item.NCashflowFin)
		setVal(result.CashFlow, "企业自由现金流", year, item.FreeCashflow)
		setVal(result.CashFlow, "销售商品、提供劳务收到的现金", year, item.SalesGoods)
		setVal(result.CashFlow, "支付给职工以及为职工支付的现金", year, item.PayStaff)
		setVal(result.CashFlow, "支付的各项税费", year, item.PayTax)
		setVal(result.CashFlow, "支付其他与经营活动有关的现金", year, item.PayOtherOp)
		setVal(result.CashFlow, "购建固定资产、无形资产和其他长期资产支付的现金", year, item.AcqConstFoliot)
		setVal(result.CashFlow, "分配股利、利润或偿付利息支付的现金", year, item.DividendPay)
		setVal(result.CashFlow, "固定资产折旧、油气资产折耗、生产性生物资产折旧", year, item.FADepr)
	}

	// 收集年份并排序（降序）
	for y := range yearSet {
		result.Years = append(result.Years, y)
	}
	for i := 0; i < len(result.Years); i++ {
		for j := i + 1; j < len(result.Years); j++ {
			if result.Years[i] < result.Years[j] {
				result.Years[i], result.Years[j] = result.Years[j], result.Years[i]
			}
		}
	}

	return result
}

func setVal(target map[string]map[string]float64, account, year string, val float64) {
	if _, ok := target[account]; !ok {
		target[account] = make(map[string]float64)
	}
	target[account][year] = val
}

// ========== 个股资金流向路由 ==========

// FetchMoneyflow 获取个股资金流向，按优先级路由
func (r *DataRouter) FetchMoneyflow(market, code, startDate, endDate string) ([]TushareMoneyflowItem, error) {
	// 1. StockFinLens 数据源（如果启用）
	if r.tushareEnabled && r.useForMoneyflow && r.tushareClient != nil {
		if mf, err := r.tushareClient.FetchMoneyflow(market, code, startDate, endDate); err == nil && len(mf) > 0 {
			fmt.Printf("[DataRouter] Moneyflow from StockFinLens: %d records for %s.%s\n", len(mf), market, code)
			return mf, nil
		}
	}

	// 2. 当前无其他资金流向数据源
	return nil, fmt.Errorf("资金流向数据暂不可用")
}

// ========== 股票基础信息路由 ==========

// FetchStockBasic 获取股票基础信息
func (r *DataRouter) FetchStockBasic(market, code string) (*TushareStockBasic, error) {
	// 1. StockFinLens 数据源（如果启用）
	if r.tushareEnabled && r.tushareClient != nil {
		tsCode := toTsCode(market, code)
		if basic, err := r.tushareClient.FetchStockBasic(tsCode); err == nil && basic != nil {
			fmt.Printf("[DataRouter] StockBasic from StockFinLens for %s.%s\n", market, code)
			return basic, nil
		}
	}

	// 2. 内置股票库（从 app.stocks 查找，需外部传入）
	return nil, fmt.Errorf("股票基础信息未找到")
}

// ========== 概念板块路由 ==========

// FetchConceptList 获取概念板块列表
func (r *DataRouter) FetchConceptList() ([]TushareConcept, error) {
	if r.tushareEnabled && r.tushareClient != nil {
		return r.tushareClient.FetchConceptList()
	}
	return nil, fmt.Errorf("数据源未启用")
}

// FetchConceptDetail 获取概念成分股
func (r *DataRouter) FetchConceptDetail(conceptID string) ([]TushareConceptStock, error) {
	if r.tushareEnabled && r.tushareClient != nil {
		return r.tushareClient.FetchConceptDetail(conceptID)
	}
	return nil, fmt.Errorf("数据源未启用")
}

// FetchProfile 获取股票基本资料，按优先级路由
func (r *DataRouter) FetchProfile(market, code string) (*StockProfile, error) {
	// 1. 东方财富（数据最完整，优先）
	fmt.Printf("[DataRouter] Profile trying EastMoney for %s.%s\n", market, code)
	if profile, err := FetchStockProfile(market, code); err == nil && profile != nil {
		return profile, nil
	}

	// 2. StockFinLens stock_basic（补充基础信息）
	if r.tushareEnabled && r.tushareClient != nil {
		fmt.Printf("[DataRouter] Profile fallback to StockFinLens for %s.%s\n", market, code)
		tsCode := toTsCode(market, code)
		if basic, err := r.tushareClient.FetchStockBasic(tsCode); err == nil && basic != nil {
			profile := &StockProfile{
				Industry:    basic.Industry,
				ListingDate: basic.ListDate,
			}
			return profile, nil
		}
	}

	return nil, fmt.Errorf("无法获取股票资料")
}

// FetchConcepts 获取股票概念板块，按优先级路由
func (r *DataRouter) FetchConcepts(market, code string, changePercent float64) (*StockConcepts, error) {
	// 1. 东方财富（数据最完整，含风口判断，优先）
	fmt.Printf("[DataRouter] Concepts trying EastMoney for %s.%s\n", market, code)
	if concepts, err := FetchStockConcepts(market, code, changePercent); err == nil && concepts != nil {
		return concepts, nil
	}

	// 2. StockFinLens concept_detail（补充基础概念列表）
	if r.tushareEnabled && r.tushareClient != nil {
		fmt.Printf("[DataRouter] Concepts fallback to StockFinLens for %s.%s\n", market, code)
		// 数据源概念数据需通过 concept 列表反向查找，暂不实现
		// 东财失败后直接返回错误，由调用方处理
	}

	return nil, fmt.Errorf("无法获取概念数据")
}

// IsUseForQuote 返回是否启用数据源每日指标
func (r *DataRouter) IsUseForQuote() bool {
	return r.tushareEnabled && r.useForQuote && r.tushareClient != nil
}

// IsUseForMoneyflow 返回是否启用数据源个股资金流向
func (r *DataRouter) IsUseForMoneyflow() bool {
	return r.tushareEnabled && r.useForMoneyflow && r.tushareClient != nil
}

// VerifyTushare 验证数据源授权码
func (r *DataRouter) VerifyTushare() error {
	if r.tushareClient == nil {
		return fmt.Errorf("数据源客户端未初始化")
	}
	return r.tushareClient.VerifyToken()
}
