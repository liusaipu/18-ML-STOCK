package downloader

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

const tushareBaseURL = "https://api.tushare.pro"

// TushareClient Tushare HTTP API 客户端
type TushareClient struct {
	token   string
	baseURL string
	client  *http.Client
}

// NewTushareClient 创建 Tushare 客户端
func NewTushareClient(token string) *TushareClient {
	return &TushareClient{
		token:   token,
		baseURL: tushareBaseURL,
		client:  &http.Client{Timeout: 20 * time.Second},
	}
}

// query 通用查询方法
func (c *TushareClient) query(apiName string, params map[string]interface{}, fields []string) (*tushareResponse, error) {
	reqBody := map[string]interface{}{
		"api_name": apiName,
		"token":    c.token,
		"params":   params,
	}
	if len(fields) > 0 {
		reqBody["fields"] = fields
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	req, err := http.NewRequest("POST", c.baseURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	var result tushareResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		prefix := 200
		if len(respBody) < prefix {
			prefix = len(respBody)
		}
		return nil, fmt.Errorf("解析响应失败: %w, body=%s", err, string(respBody[:prefix]))
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("数据源 API 错误: code=%d, msg=%s", result.Code, result.Msg)
	}

	return &result, nil
}

// tushareResponse Tushare 标准响应
type tushareResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		Fields []string        `json:"fields"`
		Items  [][]interface{} `json:"items"`
	} `json:"data"`
}

// buildFieldIndex 根据字段名建立索引映射
func buildFieldIndex(fields []string) map[string]int {
	m := make(map[string]int, len(fields))
	for i, f := range fields {
		m[f] = i
	}
	return m
}

// getStr 从 item 数组中获取字符串值
func getStr(item []interface{}, idxMap map[string]int, field string) string {
	if idx, ok := idxMap[field]; ok && idx < len(item) {
		switch v := item[idx].(type) {
		case string:
			return v
		case nil:
			return ""
		default:
			return fmt.Sprintf("%v", v)
		}
	}
	return ""
}

// getFloat 从 item 数组中获取 float64 值
func getFloat(item []interface{}, idxMap map[string]int, field string) float64 {
	if idx, ok := idxMap[field]; ok && idx < len(item) {
		switch v := item[idx].(type) {
		case float64:
			return v
		case float32:
			return float64(v)
		case int:
			return float64(v)
		case int64:
			return float64(v)
		case string:
			if v == "" || v == "None" || v == "null" {
				return 0
			}
			f, _ := strconv.ParseFloat(v, 64)
			return f
		case nil:
			return 0
		}
	}
	return 0
}

// toTsCode 将 market+code 转为 Tushare 的 ts_code 格式
func toTsCode(market, code string) string {
	switch market {
	case "SH":
		return code + ".SH"
	case "SZ":
		return code + ".SZ"
	case "BJ":
		return code + ".BJ"
	default:
		return code + ".SH"
	}
}

// fromTsCode 将 ts_code 解析为 market+code
func fromTsCode(tsCode string) (market, code string) {
	idx := -1
	for i := len(tsCode) - 1; i >= 0; i-- {
		if tsCode[i] == '.' {
			idx = i
			break
		}
	}
	if idx < 0 {
		return "SH", tsCode
	}
	return tsCode[idx+1:], tsCode[:idx]
}

// ========== 股票基础信息 ==========

// TushareStockBasic 股票基础信息
type TushareStockBasic struct {
	TsCode   string `json:"ts_code"`
	Symbol   string `json:"symbol"`
	Name     string `json:"name"`
	Area     string `json:"area"`
	Industry string `json:"industry"`
	Market   string `json:"market"` // 主板/创业板/科创板/CDR/北交所
	ListDate string `json:"list_date"`
	IsHS     string `json:"is_hs"` // N否 H沪股通 S深股通
}

// FetchStockBasic 获取股票基础信息
func (c *TushareClient) FetchStockBasic(tsCode string) (*TushareStockBasic, error) {
	params := map[string]interface{}{}
	if tsCode != "" {
		params["ts_code"] = tsCode
	}
	fields := []string{"ts_code", "symbol", "name", "area", "industry", "market", "list_date", "is_hs"}

	resp, err := c.query("stock_basic", params, fields)
	if err != nil {
		return nil, err
	}
	if len(resp.Data.Items) == 0 {
		return nil, fmt.Errorf("未找到股票: %s", tsCode)
	}

	idx := buildFieldIndex(resp.Data.Fields)
	item := resp.Data.Items[0]
	return &TushareStockBasic{
		TsCode:   getStr(item, idx, "ts_code"),
		Symbol:   getStr(item, idx, "symbol"),
		Name:     getStr(item, idx, "name"),
		Area:     getStr(item, idx, "area"),
		Industry: getStr(item, idx, "industry"),
		Market:   getStr(item, idx, "market"),
		ListDate: getStr(item, idx, "list_date"),
		IsHS:     getStr(item, idx, "is_hs"),
	}, nil
}

// ========== 日线行情 ==========

// FetchDaily 获取日线行情，返回 KlineData（Tushare daily 接口）
func (c *TushareClient) FetchDaily(market, code, startDate, endDate string) ([]KlineData, error) {
	tsCode := toTsCode(market, code)
	params := map[string]interface{}{
		"ts_code":    tsCode,
		"start_date": startDate,
		"end_date":   endDate,
	}
	fields := []string{"ts_code", "trade_date", "open", "high", "low", "close", "pre_close", "change", "pct_chg", "vol", "amount"}

	resp, err := c.query("daily", params, fields)
	if err != nil {
		return nil, err
	}
	if len(resp.Data.Items) == 0 {
		return nil, fmt.Errorf("daily 无数据: %s", tsCode)
	}

	// 获取复权因子（用于前复权）
	adjFactors, _ := c.fetchAdjFactors(tsCode, startDate, endDate)

	idx := buildFieldIndex(resp.Data.Fields)
	result := make([]KlineData, 0, len(resp.Data.Items))
	for _, item := range resp.Data.Items {
		tradeDate := getStr(item, idx, "trade_date")
		k := KlineData{
			Time:   tradeDate,
			Open:   getFloat(item, idx, "open"),
			Close:  getFloat(item, idx, "close"),
			High:   getFloat(item, idx, "high"),
			Low:    getFloat(item, idx, "low"),
			Volume: getFloat(item, idx, "vol"),
			Amount: getFloat(item, idx, "amount") * 1000, // Tushare daily 接口 amount 单位为千元，转换为元
		}
		// 前复权处理
		if len(adjFactors) > 0 {
			k = applyAdjFactor(k, adjFactors, tradeDate)
		}
		result = append(result, k)
	}
	// Tushare 返回的数据是按时间倒序排列的（最新在前），需要反转成正序（最新在后）
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}
	return result, nil
}

// fetchAdjFactors 获取复权因子
func (c *TushareClient) fetchAdjFactors(tsCode, startDate, endDate string) (map[string]float64, error) {
	params := map[string]interface{}{
		"ts_code":    tsCode,
		"start_date": startDate,
		"end_date":   endDate,
	}
	resp, err := c.query("adj_factor", params, []string{"trade_date", "adj_factor"})
	if err != nil {
		return nil, err
	}
	idx := buildFieldIndex(resp.Data.Fields)
	result := make(map[string]float64, len(resp.Data.Items))
	for _, item := range resp.Data.Items {
		result[getStr(item, idx, "trade_date")] = getFloat(item, idx, "adj_factor")
	}
	return result, nil
}

func getLatestAdjFactor(m map[string]float64) float64 {
	var latest string
	for k := range m {
		if k > latest {
			latest = k
		}
	}
	return m[latest]
}

// applyAdjFactor 应用前复权
func applyAdjFactor(k KlineData, adjFactors map[string]float64, tradeDate string) KlineData {
	latestAdj := getLatestAdjFactor(adjFactors)
	if latestAdj == 0 {
		return k
	}
	dayAdj := adjFactors[tradeDate]
	if dayAdj == 0 {
		return k
	}
	factor := dayAdj / latestAdj
	return KlineData{
		Time:         k.Time,
		Open:         k.Open * factor,
		Close:        k.Close * factor,
		High:         k.High * factor,
		Low:          k.Low * factor,
		Volume:       k.Volume,
		Amount:       k.Amount * factor,
		TurnoverRate: k.TurnoverRate,
	}
}

// ========== 每日指标 ==========

// FetchDailyBasic 获取每日指标（PE/PB/市值/换手率/股息率等）
func (c *TushareClient) FetchDailyBasic(market, code, tradeDate string) (*StockQuote, error) {
	tsCode := toTsCode(market, code)
	params := map[string]interface{}{
		"ts_code": tsCode,
	}
	if tradeDate != "" {
		params["trade_date"] = tradeDate
	}
	fields := []string{"ts_code", "trade_date", "close", "turnover_rate", "turnover_rate_f", "volume_ratio",
		"pe", "pe_ttm", "pb", "ps", "ps_ttm", "dv_ratio", "dv_ttm",
		"total_share", "float_share", "free_share", "total_mv", "circ_mv"}

	resp, err := c.query("daily_basic", params, fields)
	if err != nil {
		return nil, err
	}
	if len(resp.Data.Items) == 0 {
		return nil, fmt.Errorf("daily_basic 无数据: %s", tsCode)
	}

	idx := buildFieldIndex(resp.Data.Fields)
	item := resp.Data.Items[0]
	return &StockQuote{
		CurrentPrice:         getFloat(item, idx, "close"),
		TurnoverRate:         getFloat(item, idx, "turnover_rate"),
		VolumeRatio:          getFloat(item, idx, "volume_ratio"),
		PE:                   getFloat(item, idx, "pe"),
		PB:                   getFloat(item, idx, "pb"),
		DividendYield:        getFloat(item, idx, "dv_ratio") / 100, // 接口返回百分比
		CirculatingMarketCap: getFloat(item, idx, "circ_mv") * 1e4,   // circ_mv 单位：万元
		MarketCap:            getFloat(item, idx, "total_mv") * 1e4,  // total_mv 单位：万元
		QuoteTime:            getStr(item, idx, "trade_date"),
	}, nil
}

// ========== 财务数据 ==========

// TushareIncomeItem 利润表条目
type TushareIncomeItem struct {
	TsCode        string  `json:"ts_code"`
	EndDate       string  `json:"end_date"`     // 报告期
	AnnDate       string  `json:"ann_date"`     // 公告日期
	Revenue       float64 `json:"revenue"`      // 营业收入
	TotalProfit   float64 `json:"total_profit"` // 利润总额
	NetIncome      float64 `json:"net_income"`      // 净利润
	ParentNetIncome float64 `json:"n_income_attr_p"` // 归母净利润
	EPS            float64 `json:"eps"`             // 基本每股收益
	OperateProfit float64 `json:"operate_profit"` // 营业利润
	TotalCogs     float64 `json:"total_cogs"`   // 营业总成本
	OperateCost   float64 `json:"operate_cost"` // 营业成本
	SellExp       float64 `json:"sell_exp"`     // 销售费用
	AdminExp      float64 `json:"admin_exp"`    // 管理费用
	FinExp        float64 `json:"fin_exp"`      // 财务费用
	RDExp         float64 `json:"rd_exp"`       // 研发费用
}

// FetchIncome 获取利润表
func (c *TushareClient) FetchIncome(market, code, startDate, endDate string) ([]TushareIncomeItem, error) {
	tsCode := toTsCode(market, code)
	params := map[string]interface{}{
		"ts_code":    tsCode,
		"start_date": startDate,
		"end_date":   endDate,
	}
	fields := []string{"ts_code", "end_date", "ann_date", "revenue", "total_profit", "n_income", "n_income_attr_p", "basic_eps",
		"operate_profit", "total_cogs", "oper_cost", "sell_exp", "admin_exp", "fin_exp", "rd_exp"}

	resp, err := c.query("income", params, fields)
	if err != nil {
		return nil, err
	}

	idx := buildFieldIndex(resp.Data.Fields)
	result := make([]TushareIncomeItem, 0, len(resp.Data.Items))
	for _, item := range resp.Data.Items {
		result = append(result, TushareIncomeItem{
			TsCode:        getStr(item, idx, "ts_code"),
			EndDate:       getStr(item, idx, "end_date"),
			AnnDate:       getStr(item, idx, "ann_date"),
			Revenue:       getFloat(item, idx, "revenue"),
			TotalProfit:   getFloat(item, idx, "total_profit"),
			NetIncome:       getFloat(item, idx, "n_income"),
			ParentNetIncome: getFloat(item, idx, "n_income_attr_p"),
			EPS:             getFloat(item, idx, "basic_eps"),
			OperateProfit: getFloat(item, idx, "operate_profit"),
			TotalCogs:     getFloat(item, idx, "total_cogs"),
			OperateCost:   getFloat(item, idx, "oper_cost"),
			SellExp:       getFloat(item, idx, "sell_exp"),
			AdminExp:      getFloat(item, idx, "admin_exp"),
			FinExp:        getFloat(item, idx, "fin_exp"),
			RDExp:         getFloat(item, idx, "rd_exp"),
		})
	}
	return result, nil
}

// TushareBalanceItem 资产负债表条目
type TushareBalanceItem struct {
	TsCode         string  `json:"ts_code"`
	EndDate        string  `json:"end_date"`
	TotalAssets    float64 `json:"total_assets"`
	TotalLiab      float64 `json:"total_liab"`
	TotalHldrEqy   float64 `json:"total_hldr_eqy"` // 股东权益合计
	MoneyCap       float64 `json:"money_cap"`      // 货币资金
	TradAsset      float64 `json:"trad_asset"`     // 交易性金融资产
	NotesReceiv    float64 `json:"notes_receiv"`   // 应收票据
	AccountsReceiv float64 `json:"accounts_receiv"` // 应收账款
	Prepayment     float64 `json:"prepayment"`     // 预付款项
	ContractAsset  float64 `json:"contract_asset"` // 合同资产
	Inventories    float64 `json:"inventories"`    // 存货
	TotalCurAssets float64 `json:"total_cur_assets"` // 流动资产合计
	FixAssets      float64 `json:"fix_assets"`     // 固定资产
	CIP            float64 `json:"cip"`            // 在建工程
	ConstMaterials float64 `json:"const_materials"` // 工程物资
	IntanAssets    float64 `json:"intan_assets"`   // 无形资产
	Goodwill       float64 `json:"goodwill"`       // 商誉
	TotalNca       float64 `json:"total_nca"`      // 非流动资产合计
	LtEqtInvest    float64 `json:"lt_eqt_invest"`  // 长期股权投资
	OthEqtInvest   float64 `json:"oth_eqt_invest"` // 其他权益工具投资
	OthNca         float64 `json:"oth_nca"`        // 其他非流动资产
	ShortLoan      float64 `json:"short_loan"`     // 短期借款
	LongLoan       float64 `json:"long_loan"`      // 长期借款
	BondsPayable   float64 `json:"bonds_payable"`  // 应付债券
	NotesPayable   float64 `json:"notes_payable"`  // 应付票据
	AccountsPay    float64 `json:"acct_payable"`   // 应付账款
	AdvReceipts    float64 `json:"adv_receipts"`   // 预收款项
	ContractLiab   float64 `json:"contract_liab"`  // 合同负债
	SalaryPayable  float64 `json:"payroll_payable"` // 应付职工薪酬
	TaxPayable     float64 `json:"tax_payable"`    // 应交税费
	TotalCurLiab   float64 `json:"total_cur_liab"` // 流动负债合计
	TotalNcl       float64 `json:"total_ncl"`      // 非流动负债合计
	DeferTaxAsset  float64 `json:"defer_tax_assets"` // 递延所得税资产
	DeferTaxLiab   float64 `json:"defer_tax_liab"`   // 递延所得税负债
	ShareCapital   float64 `json:"share_capital"`  // 实收资本（或股本）
	CapRese        float64 `json:"cap_rese"`       // 资本公积
	SurplusRese    float64 `json:"surplus_rese"`   // 盈余公积
	UndistProfit   float64 `json:"undistributed_profit"` // 未分配利润
	MinorityInt    float64 `json:"minority_int"`   // 少数股东权益
}

// FetchBalanceSheet 获取资产负债表
func (c *TushareClient) FetchBalanceSheet(market, code, startDate, endDate string) ([]TushareBalanceItem, error) {
	tsCode := toTsCode(market, code)
	params := map[string]interface{}{
		"ts_code":    tsCode,
		"start_date": startDate,
		"end_date":   endDate,
	}
	fields := []string{"ts_code", "end_date", "total_assets", "total_liab", "total_hldr_eqy",
		"money_cap", "trad_asset", "notes_receiv", "accounts_receiv", "prepayment", "contract_asset",
		"inventories", "total_cur_assets", "fix_assets", "cip", "const_materials", "intan_assets",
		"goodwill", "total_nca", "lt_eqt_invest", "oth_eqt_invest", "oth_nca",
		"short_loan", "long_loan", "bonds_payable", "notes_payable", "acct_payable",
		"adv_receipts", "contract_liab", "payroll_payable", "tax_payable",
		"total_cur_liab", "total_ncl", "defer_tax_assets", "defer_tax_liab",
		"share_capital", "cap_rese", "surplus_rese", "undistributed_profit", "minority_int"}

	resp, err := c.query("balancesheet", params, fields)
	if err != nil {
		return nil, err
	}

	idx := buildFieldIndex(resp.Data.Fields)
	result := make([]TushareBalanceItem, 0, len(resp.Data.Items))
	for _, item := range resp.Data.Items {
		result = append(result, TushareBalanceItem{
			TsCode:         getStr(item, idx, "ts_code"),
			EndDate:        getStr(item, idx, "end_date"),
			TotalAssets:    getFloat(item, idx, "total_assets"),
			TotalLiab:      getFloat(item, idx, "total_liab"),
			TotalHldrEqy:   getFloat(item, idx, "total_hldr_eqy"),
			MoneyCap:       getFloat(item, idx, "money_cap"),
			TradAsset:      getFloat(item, idx, "trad_asset"),
			NotesReceiv:    getFloat(item, idx, "notes_receiv"),
			AccountsReceiv: getFloat(item, idx, "accounts_receiv"),
			Prepayment:     getFloat(item, idx, "prepayment"),
			ContractAsset:  getFloat(item, idx, "contract_asset"),
			Inventories:    getFloat(item, idx, "inventories"),
			TotalCurAssets: getFloat(item, idx, "total_cur_assets"),
			FixAssets:      getFloat(item, idx, "fix_assets"),
			CIP:            getFloat(item, idx, "cip"),
			ConstMaterials: getFloat(item, idx, "const_materials"),
			IntanAssets:    getFloat(item, idx, "intan_assets"),
			Goodwill:       getFloat(item, idx, "goodwill"),
			TotalNca:       getFloat(item, idx, "total_nca"),
			LtEqtInvest:    getFloat(item, idx, "lt_eqt_invest"),
			OthEqtInvest:   getFloat(item, idx, "oth_eqt_invest"),
			OthNca:         getFloat(item, idx, "oth_nca"),
			ShortLoan:      getFloat(item, idx, "short_loan"),
			LongLoan:       getFloat(item, idx, "long_loan"),
			BondsPayable:   getFloat(item, idx, "bonds_payable"),
			NotesPayable:   getFloat(item, idx, "notes_payable"),
			AccountsPay:    getFloat(item, idx, "acct_payable"),
			AdvReceipts:    getFloat(item, idx, "adv_receipts"),
			ContractLiab:   getFloat(item, idx, "contract_liab"),
			SalaryPayable:  getFloat(item, idx, "payroll_payable"),
			TaxPayable:     getFloat(item, idx, "tax_payable"),
			TotalCurLiab:   getFloat(item, idx, "total_cur_liab"),
			TotalNcl:       getFloat(item, idx, "total_ncl"),
			DeferTaxAsset:  getFloat(item, idx, "defer_tax_assets"),
			DeferTaxLiab:   getFloat(item, idx, "defer_tax_liab"),
			ShareCapital:   getFloat(item, idx, "share_capital"),
			CapRese:        getFloat(item, idx, "cap_rese"),
			SurplusRese:    getFloat(item, idx, "surplus_rese"),
			UndistProfit:   getFloat(item, idx, "undistributed_profit"),
			MinorityInt:    getFloat(item, idx, "minority_int"),
		})
	}
	return result, nil
}

// TushareCashflowItem 现金流量表条目
type TushareCashflowItem struct {
	TsCode        string  `json:"ts_code"`
	EndDate       string  `json:"end_date"`
	NCashflowAct  float64 `json:"n_cashflow_act"`  // 经营活动现金流净额
	NCashflowInv  float64 `json:"n_cashflow_inv"`  // 投资活动现金流净额
	NCashflowFin  float64 `json:"n_cashflow_fin"`  // 筹资活动现金流净额
	FreeCashflow  float64 `json:"free_cashflow"`   // 企业自由现金流
	SalesGoods    float64 `json:"c_sales_goods"` // 销售商品提供劳务收到的现金
	PayStaff      float64 `json:"c_paid_to_for_empl"` // 支付给职工以及为职工支付的现金
	PayTax        float64 `json:"c_paid_for_taxes"`   // 支付的各项税费
	PayOtherOp    float64 `json:"c_pay_for_others"`   // 支付其他与经营活动有关的现金
	AcqConstFoliot float64 `json:"c_pay_acq_const_foliot"` // 购建固定资产无形资产和其他长期资产支付的现金
	DividendPay   float64 `json:"c_div_profits_or_int_oop"` // 分配股利利润或偿付利息支付的现金
	FADepr        float64 `json:"fa_ir_depreciation"` // 固定资产折旧、油气资产折耗、生产性生物资产折旧
}

// FetchCashflow 获取现金流量表
func (c *TushareClient) FetchCashflow(market, code, startDate, endDate string) ([]TushareCashflowItem, error) {
	tsCode := toTsCode(market, code)
	params := map[string]interface{}{
		"ts_code":    tsCode,
		"start_date": startDate,
		"end_date":   endDate,
	}
	fields := []string{"ts_code", "end_date", "n_cashflow_act", "n_cashflow_inv", "n_cashflow_fin",
		"free_cashflow", "c_sales_goods", "c_paid_to_for_empl", "c_paid_for_taxes", "c_pay_for_others",
		"c_pay_acq_const_foliot", "c_div_profits_or_int_oop", "fa_ir_depreciation"}

	resp, err := c.query("cashflow", params, fields)
	if err != nil {
		return nil, err
	}

	idx := buildFieldIndex(resp.Data.Fields)
	result := make([]TushareCashflowItem, 0, len(resp.Data.Items))
	for _, item := range resp.Data.Items {
		result = append(result, TushareCashflowItem{
			TsCode:         getStr(item, idx, "ts_code"),
			EndDate:        getStr(item, idx, "end_date"),
			NCashflowAct:   getFloat(item, idx, "n_cashflow_act"),
			NCashflowInv:   getFloat(item, idx, "n_cashflow_inv"),
			NCashflowFin:   getFloat(item, idx, "n_cashflow_fin"),
			FreeCashflow:   getFloat(item, idx, "free_cashflow"),
			SalesGoods:     getFloat(item, idx, "c_sales_goods"),
			PayStaff:       getFloat(item, idx, "c_paid_to_for_empl"),
			PayTax:         getFloat(item, idx, "c_paid_for_taxes"),
			PayOtherOp:     getFloat(item, idx, "c_pay_for_others"),
			AcqConstFoliot: getFloat(item, idx, "c_pay_acq_const_foliot"),
			DividendPay:    getFloat(item, idx, "c_div_profits_or_int_oop"),
			FADepr:         getFloat(item, idx, "fa_ir_depreciation"),
		})
	}
	return result, nil
}

// TushareFinaIndicator 财务指标条目
type TushareFinaIndicator struct {
	TsCode            string  `json:"ts_code"`
	EndDate           string  `json:"end_date"`
	ROE               float64 `json:"roe"`
	ROEDiluted        float64 `json:"roe_diluted"`
	ROEAvg            float64 `json:"roe_avg"`           // 净资产收益率(平均)
	GrossprofitMargin float64 `json:"grossprofit_margin"` // 毛利率
	NetprofitMargin   float64 `json:"netprofit_margin"`   // 净利率
	OpOfGr            float64 `json:"op_of_gr"`           // 营业利润/总收入
	DebtToAssets      float64 `json:"debt_to_assets"`     // 资产负债率
	CurrentRatio      float64 `json:"current_ratio"`      // 流动比率
	QuickRatio        float64 `json:"quick_ratio"`        // 速动比率
	OCFToSales        float64 `json:"ocf_to_sales"`       // 经营活动现金流/营业收入
	OCFToOpIncome     float64 `json:"ocf_to_opincome"`    // 经营活动现金流/营业利润
	ROIC              float64 `json:"roic"`               // 投入资本回报率
	EBITDA            float64 `json:"ebitda"`             // 税息折旧及摊销前利润
}

// FetchFinaIndicator 获取财务指标
func (c *TushareClient) FetchFinaIndicator(market, code, startDate, endDate string) ([]TushareFinaIndicator, error) {
	tsCode := toTsCode(market, code)
	params := map[string]interface{}{
		"ts_code":    tsCode,
		"start_date": startDate,
		"end_date":   endDate,
	}
	fields := []string{"ts_code", "end_date", "roe", "roe_diluted", "roe_avg",
		"grossprofit_margin", "netprofit_margin", "op_of_gr", "debt_to_assets",
		"current_ratio", "quick_ratio", "ocf_to_sales", "ocf_to_opincome", "roic", "ebitda"}

	resp, err := c.query("fina_indicator", params, fields)
	if err != nil {
		return nil, err
	}

	idx := buildFieldIndex(resp.Data.Fields)
	result := make([]TushareFinaIndicator, 0, len(resp.Data.Items))
	for _, item := range resp.Data.Items {
		result = append(result, TushareFinaIndicator{
			TsCode:            getStr(item, idx, "ts_code"),
			EndDate:           getStr(item, idx, "end_date"),
			ROE:               getFloat(item, idx, "roe"),
			ROEDiluted:        getFloat(item, idx, "roe_diluted"),
			ROEAvg:            getFloat(item, idx, "roe_avg"),
			GrossprofitMargin: getFloat(item, idx, "grossprofit_margin"),
			NetprofitMargin:   getFloat(item, idx, "netprofit_margin"),
			OpOfGr:            getFloat(item, idx, "op_of_gr"),
			DebtToAssets:      getFloat(item, idx, "debt_to_assets"),
			CurrentRatio:      getFloat(item, idx, "current_ratio"),
			QuickRatio:        getFloat(item, idx, "quick_ratio"),
			OCFToSales:        getFloat(item, idx, "ocf_to_sales"),
			OCFToOpIncome:     getFloat(item, idx, "ocf_to_opincome"),
			ROIC:              getFloat(item, idx, "roic"),
			EBITDA:            getFloat(item, idx, "ebitda"),
		})
	}
	return result, nil
}

// ========== 资金流向 ==========

// TushareMoneyflowItem 个股资金流向条目
type TushareMoneyflowItem struct {
	TsCode        string  `json:"ts_code"`
	TradeDate     string  `json:"trade_date"`
	BuySmAmount   float64 `json:"buy_sm_amount"`   // 小单买入金额
	SellSmAmount  float64 `json:"sell_sm_amount"`  // 小单卖出金额
	BuyMdAmount   float64 `json:"buy_md_amount"`   // 中单买入金额
	SellMdAmount  float64 `json:"sell_md_amount"`  // 中单卖出金额
	BuyLgAmount   float64 `json:"buy_lg_amount"`   // 大单买入金额
	SellLgAmount  float64 `json:"sell_lg_amount"`  // 大单卖出金额
	BuyElgAmount  float64 `json:"buy_elg_amount"`  // 特大单买入金额
	SellElgAmount float64 `json:"sell_elg_amount"` // 特大单卖出金额
	NetMfAmount   float64 `json:"net_mf_amount"`   // 净流入额（元）
}

// FetchMoneyflow 获取个股资金流向
func (c *TushareClient) FetchMoneyflow(market, code, startDate, endDate string) ([]TushareMoneyflowItem, error) {
	tsCode := toTsCode(market, code)
	params := map[string]interface{}{
		"ts_code":    tsCode,
		"start_date": startDate,
		"end_date":   endDate,
	}
	fields := []string{"ts_code", "trade_date", "buy_sm_amount", "sell_sm_amount",
		"buy_md_amount", "sell_md_amount", "buy_lg_amount", "sell_lg_amount",
		"buy_elg_amount", "sell_elg_amount", "net_mf_amount"}

	resp, err := c.query("moneyflow", params, fields)
	if err != nil {
		return nil, err
	}

	idx := buildFieldIndex(resp.Data.Fields)
	result := make([]TushareMoneyflowItem, 0, len(resp.Data.Items))
	for _, item := range resp.Data.Items {
		result = append(result, TushareMoneyflowItem{
			TsCode:        getStr(item, idx, "ts_code"),
			TradeDate:     getStr(item, idx, "trade_date"),
			BuySmAmount:   getFloat(item, idx, "buy_sm_amount"),
			SellSmAmount:  getFloat(item, idx, "sell_sm_amount"),
			BuyMdAmount:   getFloat(item, idx, "buy_md_amount"),
			SellMdAmount:  getFloat(item, idx, "sell_md_amount"),
			BuyLgAmount:   getFloat(item, idx, "buy_lg_amount"),
			SellLgAmount:  getFloat(item, idx, "sell_lg_amount"),
			BuyElgAmount:  getFloat(item, idx, "buy_elg_amount"),
			SellElgAmount: getFloat(item, idx, "sell_elg_amount"),
			NetMfAmount:   getFloat(item, idx, "net_mf_amount"),
		})
	}
	return result, nil
}

// ========== 概念板块 ==========

// TushareConcept 概念板块
type TushareConcept struct {
	Code string `json:"code"`
	Name string `json:"name"`
	Src  string `json:"src"`
}

// FetchConceptList 获取概念板块列表
func (c *TushareClient) FetchConceptList() ([]TushareConcept, error) {
	resp, err := c.query("concept", map[string]interface{}{}, nil)
	if err != nil {
		return nil, err
	}

	idx := buildFieldIndex(resp.Data.Fields)
	result := make([]TushareConcept, 0, len(resp.Data.Items))
	for _, item := range resp.Data.Items {
		result = append(result, TushareConcept{
			Code: getStr(item, idx, "code"),
			Name: getStr(item, idx, "name"),
			Src:  getStr(item, idx, "src"),
		})
	}
	return result, nil
}

// TushareConceptStock 概念成分股
type TushareConceptStock struct {
	ID          string `json:"id"`
	ConceptName string `json:"concept_name"`
	TsCode      string `json:"ts_code"`
	Name        string `json:"name"`
	InDate      string `json:"in_date"`
	OutDate     string `json:"out_date"`
}

// FetchConceptDetail 获取概念成分股
func (c *TushareClient) FetchConceptDetail(conceptID string) ([]TushareConceptStock, error) {
	params := map[string]interface{}{"id": conceptID}
	resp, err := c.query("concept_detail", params, nil)
	if err != nil {
		return nil, err
	}

	idx := buildFieldIndex(resp.Data.Fields)
	result := make([]TushareConceptStock, 0, len(resp.Data.Items))
	for _, item := range resp.Data.Items {
		result = append(result, TushareConceptStock{
			ID:          getStr(item, idx, "id"),
			ConceptName: getStr(item, idx, "concept_name"),
			TsCode:      getStr(item, idx, "ts_code"),
			Name:        getStr(item, idx, "name"),
			InDate:      getStr(item, idx, "in_date"),
			OutDate:     getStr(item, idx, "out_date"),
		})
	}
	return result, nil
}

// TushareThsHotItem 同花顺热股/热概念数据
type TushareThsHotItem struct {
	TradeDate string  `json:"trade_date"`
	DataType  string  `json:"data_type"`   // 概念板块/个股/行业/期货/美股/港股
	TsCode    string  `json:"ts_code"`
	Name      string  `json:"ts_name"`
	Rank      int     `json:"rank"`
	PctChange float64 `json:"pct_change"`
	Price     float64 `json:"current_price"`
	Hot       float64 `json:"hot"`         // 热度值
	Concept   string  `json:"concept"`
	RankTime  string  `json:"rank_time"`
	Reason    string  `json:"rank_reason"`
}

// FetchThsHot 获取同花顺热搜数据（概念板块、个股、行业等）
func (c *TushareClient) FetchThsHot(tradeDate string) ([]TushareThsHotItem, error) {
	params := map[string]interface{}{}
	if tradeDate != "" {
		params["trade_date"] = tradeDate
	}
	resp, err := c.query("ths_hot", params, nil)
	if err != nil {
		return nil, err
	}

	idx := buildFieldIndex(resp.Data.Fields)
	result := make([]TushareThsHotItem, 0, len(resp.Data.Items))
	for _, item := range resp.Data.Items {
		result = append(result, TushareThsHotItem{
			TradeDate: getStr(item, idx, "trade_date"),
			DataType:  getStr(item, idx, "data_type"),
			TsCode:    getStr(item, idx, "ts_code"),
			Name:      getStr(item, idx, "ts_name"),
			Rank:      int(getFloat(item, idx, "rank")),
			PctChange: getFloat(item, idx, "pct_change"),
			Price:     getFloat(item, idx, "current_price"),
			Hot:       getFloat(item, idx, "hot"),
			Concept:   getStr(item, idx, "concept"),
			RankTime:  getStr(item, idx, "rank_time"),
			Reason:    getStr(item, idx, "rank_reason"),
		})
	}
	return result, nil
}

// VerifyToken 验证 Token 是否有效
func (c *TushareClient) VerifyToken() error {
	_, err := c.query("stock_basic", map[string]interface{}{"limit": 1}, []string{"ts_code"})
	return err
}
