package main

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	analyzer "github.com/stock-analyzer/analyzer"
	"github.com/stock-analyzer/downloader"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx     context.Context
	storage *Storage
	stocks  []StockInfo // 内存中的股票代码库
}

// StockInfo 股票基础信息
type StockInfo struct {
	Code   string `json:"code"`
	Name   string `json:"name"`
	Market string `json:"market"`
}

// WatchlistItem 自选股票项
type WatchlistItem struct {
	Code   string `json:"code"`
	Name   string `json:"name"`
	Market string `json:"market"`
}

// ImportResult CSV导入结果
type ImportResult struct {
	Success      bool     `json:"success"`
	Message      string   `json:"message"`
	BalanceSheet []string `json:"balanceSheet"`
	Income       []string `json:"income"`
	CashFlow     []string `json:"cashFlow"`
}

// DownloadResult 网络下载结果
type DownloadResult struct {
	Success    bool                       `json:"success"`
	Message    string                     `json:"message"`
	Years      []string                   `json:"years"`
	Validation []downloader.ValidationResult `json:"validation"`
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// 初始化本地存储
	storage, err := NewStorage()
	if err != nil {
		fmt.Printf("初始化存储失败: %v\n", err)
		return
	}
	a.storage = storage

	// 加载内置股票代码库
	if err := a.loadStockDB(); err != nil {
		fmt.Printf("加载股票代码库失败: %v\n", err)
	}

	// 初始化行业基准（如不存在则用默认值创建）
	if _, err := a.storage.LoadIndustryBaselines(); os.IsNotExist(err) {
		defaultBaselines := analyzer.BuildIndustryBaselines(nil)
		_ = a.storage.SaveIndustryBaselines(defaultBaselines)
	}

	// 初始化政策库（优先加载外部 JSON，否则使用内置默认值）
	if err := analyzer.InitPolicyLibrary(a.storage.DataDir()); err != nil {
		fmt.Printf("初始化政策库失败: %v\n", err)
	}
}

// loadStockDB 从嵌入的资源或数据目录加载股票列表
func (a *App) loadStockDB() error {
	bytes, err := readStockJSON()
	if err != nil {
		return err
	}
	return json.Unmarshal(bytes, &a.stocks)
}

// SearchStocks 根据关键词搜索股票，返回前10条
func (a *App) SearchStocks(keyword string) []StockInfo {
	q := strings.TrimSpace(strings.ToLower(keyword))
	if q == "" {
		return []StockInfo{}
	}
	var results []StockInfo
	for _, s := range a.stocks {
		if strings.Contains(strings.ToLower(s.Code), q) ||
			strings.Contains(strings.ToLower(s.Name), q) {
			results = append(results, s)
			if len(results) >= 10 {
				break
			}
		}
	}
	return results
}

// GetWatchlist 获取自选列表
// WatchlistActivitySummary 自选股票活跃度摘要
type WatchlistActivitySummary struct {
	Code  string  `json:"code"`
	Score float64 `json:"score"`
	Stars int     `json:"stars"`
	Grade string  `json:"grade"`
}

// GetWatchlistActivity 批量获取自选股的活跃度（带缓存）
func (a *App) GetWatchlistActivity() ([]WatchlistActivitySummary, error) {
	if a.storage == nil {
		return nil, fmt.Errorf("存储未初始化")
	}
	watchlist, err := a.storage.LoadWatchlist()
	if err != nil {
		return nil, err
	}
	fmt.Printf("[GetWatchlistActivity] watchlist count: %d\n", len(watchlist))

	baselines, _ := a.storage.LoadIndustryBaselines()
	var result []WatchlistActivitySummary

	for _, item := range watchlist {
		var activity *analyzer.ActivityData
		// 拆分 code 和 market（item.Code 可能是 002584.SZ 格式）
		pureCode := item.Code
		market := item.Market
		if strings.Contains(item.Code, ".") {
			parts := strings.SplitN(item.Code, ".", 2)
			pureCode = parts[0]
			if market == "" {
				market = strings.ToUpper(parts[1])
			}
		}
		if market == "" {
			switch {
			case strings.HasPrefix(pureCode, "6"):
				market = "SH"
			case strings.HasSuffix(item.Code, ".HK") || strings.HasPrefix(pureCode, "00") || strings.HasPrefix(pureCode, "01") || strings.HasPrefix(pureCode, "02"):
				market = "HK"
			default:
				market = "SZ"
			}
		}
		// 先读缓存
		if cached, err := a.storage.LoadActivityCache(item.Code); err == nil && cached != nil {
			activity = cached
		} else {
			klines, err := downloader.FetchStockKlines(market, pureCode, 60)
			if err != nil || len(klines) < 20 {
				fmt.Printf("[GetWatchlistActivity] %s klines err=%v len=%d\n", item.Code, err, len(klines))
				continue
			}
			quote, err := downloader.FetchStockQuote(market, pureCode)
			if err != nil || quote == nil || quote.CirculatingMarketCap <= 0 {
				fmt.Printf("[GetWatchlistActivity] %s quote err=%v cap=%.0f\n", item.Code, err, quote.CirculatingMarketCap)
				continue
			}
			profile, _ := downloader.FetchStockProfile(market, pureCode)
			industry := ""
			if profile != nil {
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
			activity = analyzer.CalculateActivity(aklines, qLite, industry, baselines)
			_ = a.storage.SaveActivityCache(item.Code, activity)
		}

		if activity != nil && activity.Score > 0 {
			result = append(result, WatchlistActivitySummary{
				Code:  item.Code,
				Score: activity.Score,
				Stars: activity.Stars,
				Grade: activity.Grade,
			})
			fmt.Printf("[GetWatchlistActivity] %s score=%.0f stars=%d\n", item.Code, activity.Score, activity.Stars)
		} else {
			fmt.Printf("[GetWatchlistActivity] %s skipped (nil or score=0)\n", item.Code)
		}
	}

	fmt.Printf("[GetWatchlistActivity] total returned: %d\n", len(result))
	return result, nil
}

func (a *App) GetWatchlist() ([]WatchlistItem, error) {
	if a.storage == nil {
		return nil, fmt.Errorf("存储未初始化")
	}
	return a.storage.LoadWatchlist()
}

// AddToWatchlist 添加股票到自选列表
func (a *App) AddToWatchlist(code string) error {
	if a.storage == nil {
		return fmt.Errorf("存储未初始化")
	}
	list, err := a.storage.LoadWatchlist()
	if err != nil {
		return err
	}
	// 检查是否已存在
	for _, item := range list {
		if item.Code == code {
			return nil // 已存在，不重复添加
		}
	}
	// 检查是否超过100只
	if len(list) >= 100 {
		return fmt.Errorf("自选列表最多100只股票")
	}
	// 查找股票信息
	for _, s := range a.stocks {
		if s.Code == code {
			list = append([]WatchlistItem{{
				Code:   s.Code,
				Name:   s.Name,
				Market: s.Market,
			}}, list...)
			return a.storage.SaveWatchlist(list)
		}
	}
	return fmt.Errorf("未找到股票: %s", code)
}

// RemoveFromWatchlist 从自选列表移除股票
func (a *App) RemoveFromWatchlist(code string) error {
	if a.storage == nil {
		return fmt.Errorf("存储未初始化")
	}
	list, err := a.storage.LoadWatchlist()
	if err != nil {
		return err
	}
	filtered := make([]WatchlistItem, 0, len(list))
	for _, item := range list {
		if item.Code != code {
			filtered = append(filtered, item)
		}
	}
	if err := a.storage.SaveWatchlist(filtered); err != nil {
		return err
	}
	// 清理该股票的所有本地数据
	_ = a.storage.CleanStockData(code)
	// 清理可比公司配置中的该股票记录
	config, _ := a.storage.LoadComparablesConfig()
	delete(config, code)
	_ = a.storage.SaveComparablesConfig(config)
	return nil
}

// ReorderWatchlist 重新排序自选列表
func (a *App) ReorderWatchlist(codes []string) error {
	if a.storage == nil {
		return fmt.Errorf("存储未初始化")
	}
	list, err := a.storage.LoadWatchlist()
	if err != nil {
		return err
	}
	// 建立 code -> item 映射
	itemMap := make(map[string]WatchlistItem, len(list))
	for _, item := range list {
		itemMap[item.Code] = item
	}
	// 按传入的 codes 顺序重建列表
	newList := make([]WatchlistItem, 0, len(codes))
	for _, code := range codes {
		if item, ok := itemMap[code]; ok {
			newList = append(newList, item)
		}
	}
	// 把不在 codes 里的项放到末尾（防御性处理）
	for _, item := range list {
		found := false
		for _, code := range codes {
			if code == item.Code {
				found = true
				break
			}
		}
		if !found {
			newList = append(newList, item)
		}
	}
	return a.storage.SaveWatchlist(newList)
}

// ImportFinancialReports 导入某只股票的财报CSV文件
// 参数 symbol 如 "603501.SH"
// 返回导入结果和可用年份
func (a *App) ImportFinancialReports(symbol string) (*ImportResult, error) {
	if a.storage == nil {
		return nil, fmt.Errorf("存储未初始化")
	}

	result := &ImportResult{Success: false}
	fmt.Printf("[Import] called for %s\n", symbol)

	// 弹出文件选择对话框，允许选择多个 CSV 或 Excel
	selection, err := runtime.OpenMultipleFilesDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择本地 CSV 或 Excel 财报文件",
		Filters: []runtime.FileFilter{
			{DisplayName: "CSV/Excel 文件", Pattern: "*.csv;*.xlsx"},
			{DisplayName: "CSV 文件", Pattern: "*.csv"},
			{DisplayName: "Excel 文件", Pattern: "*.xlsx"},
		},
	})
	fmt.Printf("[Import] dialog returned %d files, err=%v\n", len(selection), err)
	if err != nil {
		return nil, fmt.Errorf("打开文件对话框失败: %w", err)
	}
	if len(selection) == 0 {
		return nil, fmt.Errorf("未选择文件")
	}

	// 创建股票数据目录
	dataDir, err := a.storage.EnsureStockDataDir(symbol)
	if err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}

	var balanceYears, incomeYears, cashYears []string
	importedTypes := make(map[string]bool)
	var errors []string

	for _, filePath := range selection {
		fmt.Printf("[Import] processing file: %s\n", filePath)
		base := strings.ToLower(filepath.Base(filePath))
		var reportType string
		if strings.Contains(base, "debt") || strings.Contains(base, "balance") || strings.Contains(base, "负债") || strings.Contains(base, "资产") {
			reportType = "balance"
		} else if strings.Contains(base, "benefit") || strings.Contains(base, "income") || strings.Contains(base, "利润") || strings.Contains(base, "损益") {
			reportType = "income"
		} else if strings.Contains(base, "cash") || strings.Contains(base, "现金") || strings.Contains(base, "flow") {
			reportType = "cash"
		} else {
			// 尝试通过解析内容来判断
			if t, err := detectReportTypeByContent(filePath); err == nil {
				reportType = t
			} else {
				errors = append(errors, fmt.Sprintf("%s: %v", filepath.Base(filePath), err))
				continue
			}
		}

		// 防止同一类型重复导入（后面的覆盖前面的）
		importedTypes[reportType] = true

		var data map[string]map[string]float64
		var years []string
		ext := strings.ToLower(filepath.Ext(filePath))
		if ext == ".xlsx" {
			data, years, err = ParseThsExcel(filePath)
		} else {
			data, years, err = ParseThsCSV(filePath)
		}
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: 解析失败 (%v)", filepath.Base(filePath), err))
			delete(importedTypes, reportType)
			continue
		}
		fmt.Printf("[Import] parsed %s, years=%v\n", reportType, years)

		// 保存到本地目录
		var targetFile string
		switch reportType {
		case "balance":
			targetFile = filepath.Join(dataDir, "balance_sheet.json")
			balanceYears = years
		case "income":
			targetFile = filepath.Join(dataDir, "income_statement.json")
			incomeYears = years
		case "cash":
			targetFile = filepath.Join(dataDir, "cash_flow.json")
			cashYears = years
		}

		jsonBytes, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: JSON序列化失败", filepath.Base(filePath)))
			delete(importedTypes, reportType)
			continue
		}
		if err := os.WriteFile(targetFile, jsonBytes, 0644); err != nil {
			errors = append(errors, fmt.Sprintf("%s: 保存失败 (%v)", filepath.Base(filePath), err))
			delete(importedTypes, reportType)
			continue
		}
	}

	// 检查是否三表齐全
	missing := []string{}
	if !importedTypes["balance"] {
		missing = append(missing, "资产负债表")
	}
	if !importedTypes["income"] {
		missing = append(missing, "利润表")
	}
	if !importedTypes["cash"] {
		missing = append(missing, "现金流量表")
	}

	if len(missing) > 0 {
		msg := fmt.Sprintf("缺少以下报表: %s", strings.Join(missing, ", "))
		if len(errors) > 0 {
			msg += fmt.Sprintf("\n\n处理过程中的问题:\n%s", strings.Join(errors, "\n"))
		}
		return nil, fmt.Errorf("%s", msg)
	}

	result.Success = true
	result.BalanceSheet = balanceYears
	result.Income = incomeYears
	result.CashFlow = cashYears
	result.Message = fmt.Sprintf("成功导入 %d 张报表 (CSV/Excel 混合支持)", len(importedTypes))
	fmt.Printf("[Import] success: bs=%v income=%v cash=%v\n", balanceYears, incomeYears, cashYears)

	// 归档历史版本
	_ = a.storage.ArchiveStockData(symbol, HistoryMeta{
		Timestamp:  time.Now().Format(time.RFC3339),
		Source:     "csv_excel_import",
		SourceName: "同花顺/本地CSV/Excel",
		Years:      mergeYears(balanceYears, incomeYears, cashYears),
	})

	return result, nil
}

// DownloadReports 从网络下载指定股票的财报数据
func (a *App) DownloadReports(symbol string) (*DownloadResult, error) {
	if a.storage == nil {
		return nil, fmt.Errorf("存储未初始化")
	}

	// symbol 格式如 603501.SH，拆分为 market 和 code
	parts := strings.Split(symbol, ".")
	if len(parts) != 2 {
		return nil, fmt.Errorf("无效的股票代码格式: %s", symbol)
	}
	code := parts[0]
	market := strings.ToUpper(parts[1])

	// 下载数据
	data, err := downloader.DownloadFinancialReports(market, code)
	if err != nil {
		return nil, fmt.Errorf("下载财报失败: %w", err)
	}

	// 保存到本地
	dataDir, err := a.storage.EnsureStockDataDir(symbol)
	if err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}
	if err := downloader.SaveAsJSON(data, dataDir); err != nil {
		return nil, fmt.Errorf("保存财报数据失败: %w", err)
	}

	// 多源校验
	validation, _ := downloader.ValidateWithDatacenter(market, code, data)

	// 归档历史版本
	_ = a.storage.ArchiveStockData(symbol, HistoryMeta{
		Timestamp:  time.Now().Format(time.RFC3339),
		Source:     "network_download",
		SourceName: "东方财富",
		Years:      data.Years,
	})

	return &DownloadResult{
		Success:    true,
		Message:    fmt.Sprintf("成功下载 %d 年的年报数据", len(data.Years)),
		Years:      data.Years,
		Validation: validation,
	}, nil
}

// GetComparables 获取某只股票的可比公司列表
func (a *App) GetComparables(symbol string) ([]string, error) {
	if a.storage == nil {
		return nil, fmt.Errorf("存储未初始化")
	}
	return a.storage.GetComparables(symbol)
}

// AddComparable 添加可比公司
func (a *App) AddComparable(symbol, comparable string) error {
	if a.storage == nil {
		return fmt.Errorf("存储未初始化")
	}
	return a.storage.AddComparable(symbol, comparable)
}

// RemoveComparable 移除可比公司
func (a *App) RemoveComparable(symbol, comparable string) error {
	if a.storage == nil {
		return fmt.Errorf("存储未初始化")
	}
	return a.storage.RemoveComparable(symbol, comparable)
}

// DownloadComparableReports 下载所有可比公司的财报数据
func (a *App) DownloadComparableReports(symbol string) (*DownloadResult, error) {
	if a.storage == nil {
		return nil, fmt.Errorf("存储未初始化")
	}
	comparables, err := a.storage.GetComparables(symbol)
	if err != nil {
		return nil, err
	}
	if len(comparables) == 0 {
		return &DownloadResult{Success: true, Message: "无可比公司需要下载"}, nil
	}

	var totalYears int
	var failed []string
	for _, comp := range comparables {
		parts := strings.Split(comp, ".")
		if len(parts) != 2 {
			failed = append(failed, comp)
			continue
		}
		code := parts[0]
		market := strings.ToUpper(parts[1])
		data, err := downloader.DownloadFinancialReports(market, code)
		if err != nil {
			fmt.Printf("[Comparable] download failed for %s: %v\n", comp, err)
			failed = append(failed, comp)
			continue
		}
		dir, err := a.storage.EnsureComparableDataDir(comp)
		if err != nil {
			failed = append(failed, comp)
			continue
		}
		if err := downloader.SaveAsJSON(data, dir); err != nil {
			failed = append(failed, comp)
			continue
		}
		totalYears += len(data.Years)
	}

	msg := fmt.Sprintf("成功下载 %d 家可比公司，共 %d 年数据", len(comparables)-len(failed), totalYears)
	if len(failed) > 0 {
		msg += fmt.Sprintf("；失败 %d 家: %s", len(failed), strings.Join(failed, ", "))
	}
	return &DownloadResult{
		Success: len(failed) < len(comparables),
		Message: msg,
	}, nil
}

// GetStockKlines 获取股票历史K线数据（日K，默认120根）
func (a *App) GetStockKlines(symbol string) ([]downloader.KlineData, error) {
	if a.storage == nil {
		return nil, fmt.Errorf("存储未初始化")
	}
	parts := strings.Split(symbol, ".")
	if len(parts) != 2 {
		return nil, fmt.Errorf("无效的股票代码格式: %s", symbol)
	}
	code := parts[0]
	market := strings.ToUpper(parts[1])
	return downloader.FetchStockKlines(market, code, 120)
}

// GetStockQuote 获取股票实时行情（带15分钟本地缓存）
func (a *App) GetStockQuote(symbol string) (*downloader.StockQuote, error) {
	if a.storage == nil {
		return nil, fmt.Errorf("存储未初始化")
	}

	// 尝试读取缓存（15分钟），同时校验数据合理性
	cached, err := a.storage.LoadStockQuote(symbol)
	if err == nil && cached != nil {
		path := filepath.Join(a.storage.DataDir(), "data", symbol, "quote.json")
		info, err := os.Stat(path)
		if err == nil && time.Since(info.ModTime()) < 15*time.Minute {
			// 校验缓存数据是否合理（过滤掉错误解析的巨大盘百分比或时间戳）
			if cached.CurrentPrice > 0 && cached.ChangePercent > -50 && cached.ChangePercent < 50 {
				return cached, nil
			}
		}
	}

	// 拆分 symbol
	parts := strings.Split(symbol, ".")
	if len(parts) != 2 {
		return nil, fmt.Errorf("无效的股票代码格式: %s", symbol)
	}
	code := parts[0]
	market := strings.ToUpper(parts[1])

	// 从网络获取
	quote, err := downloader.FetchStockQuote(market, code)
	if err != nil {
		return nil, fmt.Errorf("获取行情失败: %w", err)
	}
	_ = a.storage.SaveStockQuote(symbol, quote)
	return quote, nil
}

// CacheStatus 分析缓存状态
type CacheStatus struct {
	Unchanged          bool   `json:"unchanged"`
	LastAnalysisAt     string `json:"lastAnalysisAt"`
	DataChanged        bool   `json:"dataChanged"`
	ComparablesChanged bool   `json:"comparablesChanged"`
}

// CheckAnalysisCache 检查分析缓存状态
func (a *App) CheckAnalysisCache(symbol string) (*CacheStatus, error) {
	if a.storage == nil {
		return nil, fmt.Errorf("存储未初始化")
	}
	currentDataHash, err := a.storage.ComputeDataHash(symbol)
	if err != nil {
		return nil, err
	}
	currentCompHash, err := a.storage.ComputeComparablesHash(symbol)
	if err != nil {
		return nil, err
	}
	cache, err := a.storage.LoadAnalysisCache(symbol)
	if err != nil {
		return nil, err
	}

	dataChanged := true
	comparablesChanged := true
	lastAnalysisAt := ""
	if cache != nil {
		dataChanged = cache.DataHash != currentDataHash
		comparablesChanged = cache.ComparablesHash != currentCompHash
		lastAnalysisAt = cache.LastAnalysisAt
	}

	return &CacheStatus{
		Unchanged:          !dataChanged && !comparablesChanged,
		LastAnalysisAt:     lastAnalysisAt,
		DataChanged:        dataChanged,
		ComparablesChanged: comparablesChanged,
	}, nil
}

// AnalyzeStock 对指定股票执行18步财务分析
func (a *App) AnalyzeStock(symbol string, overwriteLatest bool) (*analyzer.AnalysisReport, error) {
	return a.analyzeStockInternal(symbol, overwriteLatest, nil)
}

// AnalyzeStockWithRIM 使用用户自定义RIM参数执行分析
func (a *App) AnalyzeStockWithRIM(symbol string, overwriteLatest bool, rimJSON string) (*analyzer.AnalysisReport, error) {
	var customRIM *analyzer.RIMData
	if rimJSON != "" {
		customRIM = &analyzer.RIMData{}
		if err := json.Unmarshal([]byte(rimJSON), customRIM); err != nil {
			return nil, fmt.Errorf("解析RIM参数失败: %w", err)
		}
		if customRIM.HasData && customRIM.Result == nil {
			customRIM.Result = analyzer.CalculateMultiPeriodRIM(customRIM.Params)
		}
	}
	return a.analyzeStockInternal(symbol, overwriteLatest, customRIM)
}

func (a *App) analyzeStockInternal(symbol string, overwriteLatest bool, customRIM *analyzer.RIMData) (*analyzer.AnalysisReport, error) {
	if a.storage == nil {
		return nil, fmt.Errorf("存储未初始化")
	}
	comparables, _ := a.storage.GetComparables(symbol)
	nameMap := make(map[string]string, len(a.stocks))
	for _, s := range a.stocks {
		nameMap[s.Code] = s.Name
	}
	compAnalysis, _ := analyzer.BuildComparableAnalysis(a.storage.DataDir(), comparables, nameMap)

	// 并发获取网络数据：实时行情、K线、舆情情绪
	var quoteData *analyzer.QuoteData
	var klines []downloader.KlineData
	var sentimentData *analyzer.SentimentData
	var wgNet sync.WaitGroup
	wgNet.Add(3)

	go func() {
		defer wgNet.Done()
		if q, err := a.GetStockQuote(symbol); err == nil && q != nil {
			quoteData = &analyzer.QuoteData{
				CurrentPrice:         q.CurrentPrice,
				ChangePercent:        q.ChangePercent,
				ChangeAmount:         q.ChangeAmount,
				Volume:               q.Volume,
				TurnoverAmount:       q.TurnoverAmount,
				TurnoverRate:         q.TurnoverRate,
				Amplitude:            q.Amplitude,
				High:                 q.High,
				Low:                  q.Low,
				Open:                 q.Open,
				PreviousClose:        q.PreviousClose,
				CirculatingMarketCap: q.CirculatingMarketCap,
				VolumeRatio:          q.VolumeRatio,
				PE:                   q.PE,
				PB:                   q.PB,
				MarketCap:            q.MarketCap,
			}
		}
	}()

	go func() {
		defer wgNet.Done()
		if list, err := a.GetStockKlines(symbol); err == nil && len(list) >= 20 {
			klines = list
		}
	}()

	go func() {
		defer wgNet.Done()
		if cachedSentiment, err := a.storage.LoadStockSentiment(symbol); err == nil && cachedSentiment != nil {
			path := filepath.Join(a.storage.DataDir(), "data", symbol, "sentiment.json")
			if info, err := os.Stat(path); err == nil && time.Since(info.ModTime()) < 60*time.Minute {
				sentimentData = &analyzer.SentimentData{
					Score:         cachedSentiment.Score,
					HeatIndex:     cachedSentiment.HeatIndex,
					PositiveWords: cachedSentiment.PositiveWords,
					NegativeWords: cachedSentiment.NegativeWords,
					Summaries:     make([]analyzer.SentimentSummary, len(cachedSentiment.Summaries)),
					HasData:       cachedSentiment.HasData,
				}
				for i, s := range cachedSentiment.Summaries {
					sentimentData.Summaries[i] = analyzer.SentimentSummary{
						Title:     s.Title,
						Source:    s.Source,
						Date:      s.Date,
						Sentiment: s.Sentiment,
					}
				}
			}
		}
		if sentimentData == nil {
			parts := strings.Split(symbol, ".")
			if len(parts) == 2 {
				code := parts[0]
				market := strings.ToUpper(parts[1])
				if s, err := downloader.FetchStockSentiment(market, code); err == nil && s != nil {
					sentimentData = &analyzer.SentimentData{
						Score:         s.Score,
						HeatIndex:     s.HeatIndex,
						PositiveWords: s.PositiveWords,
						NegativeWords: s.NegativeWords,
						Summaries:     make([]analyzer.SentimentSummary, len(s.Summaries)),
						HasData:       s.HasData,
					}
					for i, summary := range s.Summaries {
						sentimentData.Summaries[i] = analyzer.SentimentSummary{
							Title:     summary.Title,
							Source:    summary.Source,
							Date:      summary.Date,
							Sentiment: summary.Sentiment,
						}
					}
					_ = a.storage.SaveStockSentiment(symbol, s)
				} else {
					fmt.Printf("[Sentiment] fetch failed for %s: %v\n", symbol, err)
				}
			}
		}
	}()

	wgNet.Wait()

	// 构建十五五政策匹配数据
	var policyData *analyzer.PolicyMatchData
	profile, err := a.GetStockProfile(symbol)
	if err == nil && profile != nil && profile.Industry == "" {
		// 缓存中行业为空，强制刷新
		profile, _ = a.RefreshStockProfile(symbol)
	}
	if profile != nil {
		conceptList := []string{}
		if concepts, err := a.GetStockConcepts(symbol); err == nil && concepts != nil {
			conceptList = concepts.Concepts
		}
		policyData = analyzer.BuildPolicyMatch(profile.Industry, conceptList)
	}

	// 技术形态分析
	var technicalData *analyzer.TechnicalData
	if len(klines) >= 30 {
		tklines := make([]analyzer.TechnicalKline, len(klines))
		for i, k := range klines {
			tklines[i] = analyzer.TechnicalKline{
				Time:   k.Time,
				Open:   k.Open,
				Close:  k.Close,
				Low:    k.Low,
				High:   k.High,
				Volume: k.Volume,
			}
		}
		technicalData = analyzer.AnalyzeTechnical(tklines)
	}

	// 交易活跃度分析
	var activityData *analyzer.ActivityData
	if len(klines) >= 20 && quoteData != nil {
		baselines, _ := a.storage.LoadIndustryBaselines()
		industry := ""
		if profile != nil {
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
		qLite := &analyzer.StockQuoteLite{CirculatingMarketCap: quoteData.CirculatingMarketCap}
		activityData = analyzer.CalculateActivity(aklines, qLite, industry, baselines)
	}
	// 加载财务数据（ML 和 RIM fallback 都需要）
	var finData *analyzer.FinancialData
	if fd, err := analyzer.LoadFinancialData(a.storage.DataDir(), symbol); err == nil && fd != nil {
		finData = fd
	}

	// ML 双引擎预测 + RIM 外部数据获取 并发执行
	var mlData *analyzer.MLPredictionData
	var extRIM *downloader.RIMExternalData
	var rimErr error
	var wg sync.WaitGroup

	// 并发 1: ML Engine B + Engine A
	if finData != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mlLocal := &analyzer.MLPredictionData{}
			// Engine B
			if finSeq := analyzer.BuildMLEngineBInput(finData); len(finSeq) > 0 {
				if fp, err := analyzer.RunMLEngineB(finSeq); err == nil {
					mlLocal.Financial = fp
				} else {
					fmt.Printf("[ML] Engine B failed for %s: %v\n", symbol, err)
				}
			}
			// Engine A（价格序列始终可用；sentiment 为 nil 时 text_seq 补 0）
			if len(klines) >= 16 {
				mlKlines := make([]analyzer.MLKlineData, len(klines))
				for i, k := range klines {
					mlKlines[i] = analyzer.MLKlineData{
						Time: k.Time, Open: k.Open, Close: k.Close,
						Low: k.Low, High: k.High, Volume: k.Volume, Amount: k.Amount,
					}
				}
				textSeq, priceSeq := analyzer.BuildMLEngineAInputFromKlines(mlKlines, sentimentData)
				if textSeq != nil && priceSeq != nil {
					if sp, err := analyzer.RunMLEngineA(textSeq, priceSeq); err == nil {
						mlLocal.Sentiment = sp
					} else {
						fmt.Printf("[ML] Engine A failed for %s: %v\n", symbol, err)
					}
				}
			}
			if mlLocal.Financial != nil || mlLocal.Sentiment != nil {
				mlData = mlLocal
			}
		}()
	}

	// 并发 2: RIM 外部数据获取（带缓存）
	if customRIM == nil && quoteData != nil {
		pureCode := symbol
		if idx := strings.Index(symbol, "."); idx > 0 {
			pureCode = symbol[:idx]
		}
		// 先读缓存
		if cached, err := a.storage.LoadRIMCache(symbol); err == nil && cached != nil {
			extRIM = cached
		} else {
			wg.Add(1)
			go func() {
				defer wg.Done()
				extRIM, rimErr = downloader.FetchRIMExternalData(pureCode)
				if rimErr != nil {
					fmt.Printf("[RIM] fetch failed for %s: %v\n", symbol, rimErr)
				}
				if extRIM != nil {
					_ = a.storage.SaveRIMCache(symbol, extRIM)
				}
			}()
		}
	}

	// 并发 3: A-Score 非财务风险爬虫（股权质押、问询函、减持）
	wg.Add(1)
	go func() {
		defer wg.Done()
		if rc, err := downloader.FetchRiskCrawlerData(symbol); err == nil {
			if finData != nil {
				finData.Extras = make(map[string]float64)
				if rc.PledgeRatio != nil {
					finData.Extras["pledge_ratio"] = *rc.PledgeRatio
				}
				if rc.InquiryCount1Y != nil {
					finData.Extras["inquiry_count_1y"] = float64(*rc.InquiryCount1Y)
				}
				if rc.ReductionCount1Y != nil {
					finData.Extras["reduction_count_1y"] = float64(*rc.ReductionCount1Y)
				}
			}
		} else {
			fmt.Printf("[RiskCrawler] failed for %s: %v\n", symbol, err)
		}
	}()

	wg.Wait()

	// RIM 多期估值数据组装
	var rimData *analyzer.RIMData
	if customRIM != nil {
		rimData = customRIM
	} else if quoteData != nil {
		rimData = &analyzer.RIMData{}
		if extRIM != nil {
			rimData.Rf = extRIM.Rf
			rimData.Beta = extRIM.Beta
			rimData.RmRf = extRIM.RmRf
			rimData.EPSRaw = extRIM.EPSForecast
		} else {
			// 使用默认参数（即使外部数据获取失败也尝试计算）
			rimData.Rf = 0.0183
			rimData.Beta = 0.98
			rimData.RmRf = 0.0517
		}

		bps0 := 0.0
		if quoteData.PB > 0 && quoteData.CurrentPrice > 0 {
			bps0 = quoteData.CurrentPrice / quoteData.PB
		}
		if bps0 <= 0 && extRIM != nil && extRIM.PB > 0 && extRIM.Price > 0 {
			bps0 = extRIM.Price / extRIM.PB
		}
		if bps0 <= 0 {
			totalShares := 0.0
			if extRIM != nil && extRIM.TotalShares > 0 {
				totalShares = extRIM.TotalShares
			} else if quoteData.CurrentPrice > 0 && quoteData.MarketCap > 0 {
				totalShares = quoteData.MarketCap / quoteData.CurrentPrice
			}
			if finData != nil && len(finData.Years) > 0 {
				year := finData.Years[0]
				equity := finData.GetValueOrZero(finData.BalanceSheet, "归母所有者权益合计", year)
				if equity == 0 {
					equity = finData.GetValueOrZero(finData.BalanceSheet, "所有者权益合计", year)
				}
				if equity > 0 && totalShares > 0 {
					bps0 = equity / 1e8 / (totalShares / 1e8) // 元/股
				}
			}
		}

		var epsSeq []float64
		if extRIM != nil && len(extRIM.EPSForecast) > 0 {
			years := make([]string, 0, len(extRIM.EPSForecast))
			for y := range extRIM.EPSForecast {
				years = append(years, y)
			}
			for i := 0; i < len(years)-1; i++ {
				for j := i + 1; j < len(years); j++ {
					if years[i] > years[j] {
						years[i], years[j] = years[j], years[i]
					}
				}
			}
			for _, y := range years {
				if v, ok := extRIM.EPSForecast[y]; ok && v > 0 {
					epsSeq = append(epsSeq, v)
				}
			}
		}
		// 如果外部没有预测数据，用 trailing EPS 做起点然后外推
		if len(epsSeq) == 0 {
			trailingEPS := 0.0
			if finData != nil && len(finData.Years) > 0 {
				finYear := finData.Years[0]
				finProfit := finData.GetValueOrZero(finData.IncomeStatement, "归母净利润", finYear)
				if finProfit == 0 {
					finProfit = finData.GetValueOrZero(finData.IncomeStatement, "净利润", finYear)
				}
				totalShares := 0.0
				if extRIM != nil && extRIM.TotalShares > 0 {
					totalShares = extRIM.TotalShares
				} else if quoteData.CurrentPrice > 0 && quoteData.MarketCap > 0 {
					totalShares = quoteData.MarketCap / quoteData.CurrentPrice
				}
				if finProfit > 0 && totalShares > 0 {
					trailingEPS = finProfit / 1e8 / (totalShares / 1e8)
				}
				if trailingEPS <= 0 && bps0 > 0 {
					netProfit := finData.GetValueOrZero(finData.IncomeStatement, "归母净利润", finYear)
					if netProfit == 0 {
						netProfit = finData.GetValueOrZero(finData.IncomeStatement, "净利润", finYear)
					}
					equity := finData.GetValueOrZero(finData.BalanceSheet, "归母所有者权益合计", finYear)
					if equity == 0 {
						equity = finData.GetValueOrZero(finData.BalanceSheet, "所有者权益合计", finYear)
					}
					roe := 0.0
					if equity > 0 {
						roe = netProfit / equity * 100
					}
					if roe > 0 {
						trailingEPS = bps0 * (roe / 100)
					}
				}
			}
			if trailingEPS > 0 {
				epsSeq = append(epsSeq, trailingEPS)
			}
		}
		// 如果预测年份不足6年，用最后一年增长率外推（默认增长率 10% -> 5%）
		growthRates := []float64{0.10, 0.10, 0.08, 0.05, 0.05, 0.05}
		for len(epsSeq) < 6 {
			last := 0.0
			if len(epsSeq) > 0 {
				last = epsSeq[len(epsSeq)-1]
			}
			g := growthRates[len(epsSeq)%len(growthRates)]
			if last > 0 {
				epsSeq = append(epsSeq, last*(1+g))
			} else {
				epsSeq = append(epsSeq, 0)
			}
		}

		ke := rimData.Rf + rimData.Beta*rimData.RmRf
		gTerminal := 0.05
		price := quoteData.CurrentPrice
		if price <= 0 && extRIM != nil {
			price = extRIM.Price
		}

		if bps0 > 0 && ke > gTerminal {
			params := analyzer.RIMParams{
				BPS0:         bps0,
				KE:           ke,
				GTerminal:    gTerminal,
				Forecast:     analyzer.RIMForecast{EPS: epsSeq},
				CurrentPrice: price,
			}
			rimData.Params = params
			rimData.Result = analyzer.CalculateMultiPeriodRIM(params)
			if rimData.Result != nil {
				rimData.HasData = true
			}
		}
		if !rimData.HasData {
			rimData = nil
		}
	}

	report, err := analyzer.RunAnalysisWithAll(a.storage.DataDir(), symbol, compAnalysis, quoteData, sentimentData, policyData, technicalData, activityData, mlData, rimData)
	if err != nil {
		return nil, err
	}
	// 自动保存报告到本地
	_, _ = a.storage.SaveReport(symbol, report.MarkdownContent, overwriteLatest)
	// 保存分析缓存
	if hash, err := a.storage.ComputeDataHash(symbol); err == nil {
		if compHash, err := a.storage.ComputeComparablesHash(symbol); err == nil {
			_ = a.storage.SaveAnalysisCache(symbol, hash, compHash)
		}
	}
	return report, nil
}

// GetReportHistory 获取某只股票的历史报告文件名列表
func (a *App) GetReportHistory(symbol string) ([]string, error) {
	if a.storage == nil {
		return nil, fmt.Errorf("存储未初始化")
	}
	return a.storage.ListReports(symbol)
}

// GetReport 读取指定历史报告的 Markdown 内容
func (a *App) GetReport(symbol, filename string) (string, error) {
	if a.storage == nil {
		return "", fmt.Errorf("存储未初始化")
	}
	return a.storage.LoadReport(symbol, filename)
}

// DeleteReport 删除指定历史报告
func (a *App) DeleteReport(symbol, filename string) error {
	if a.storage == nil {
		return fmt.Errorf("存储未初始化")
	}
	return a.storage.DeleteReport(symbol, filename)
}

// GetStockDataHistory 获取某只股票的财务数据历史归档
func (a *App) GetStockDataHistory(symbol string) ([]HistoryMeta, error) {
	if a.storage == nil {
		return nil, fmt.Errorf("存储未初始化")
	}
	return a.storage.ListStockDataHistory(symbol)
}

// GetStockProfile 获取股票基本资料（带7天本地缓存）
func (a *App) GetStockProfile(symbol string) (*StockProfile, error) {
	if a.storage == nil {
		return nil, fmt.Errorf("存储未初始化")
	}

	// 尝试读取缓存（7天内且数据基本完整才用）
	cached, err := a.storage.LoadStockProfile(symbol)
	if err == nil && cached != nil && cached.UpdatedAt != "" {
		t, err := time.Parse(time.RFC3339, cached.UpdatedAt)
		if err == nil && time.Since(t) < 7*24*time.Hour {
			// 缓存必须有至少一项核心字段才认为有效
			if cached.Industry != "" || cached.ListingDate != "" || cached.MarketCap > 0 || cached.PE > 0 {
				return cached, nil
			}
		}
	}

	// 拆分 symbol
	parts := strings.Split(symbol, ".")
	if len(parts) != 2 {
		return nil, fmt.Errorf("无效的股票代码格式: %s", symbol)
	}
	code := parts[0]
	market := strings.ToUpper(parts[1])

	// 从网络获取
	dp, err := downloader.FetchStockProfile(market, code)
	if err != nil {
		// 网络失败时回退到过期缓存（如果有）
		if cached != nil && (cached.Industry != "" || cached.ListingDate != "" || cached.MarketCap > 0 || cached.PE > 0) {
			return cached, nil
		}
		return nil, fmt.Errorf("获取股票资料失败: %w", err)
	}

	profile := &StockProfile{
		Industry:             dp.Industry,
		ListingDate:          dp.ListingDate,
		TotalShares:          dp.TotalShares,
		MarketCap:            dp.MarketCap,
		PE:                   dp.PE,
		PB:                   dp.PB,
		EPS:                  dp.EPS,
		Chairman:             dp.Chairman,
		Controller:           dp.Controller,
		ChairmanGender:       dp.ChairmanGender,
		ChairmanAge:          dp.ChairmanAge,
		ChairmanNationality:  dp.ChairmanNationality,
		ChairmanHoldRatio:    dp.ChairmanHoldRatio,
		PoliticalAffiliation: dp.PoliticalAffiliation,
		UpdatedAt:            time.Now().Format(time.RFC3339),
	}
	// 只有获取到有效数据才缓存，避免空数据占坑 7 天
	if profile.Industry != "" || profile.ListingDate != "" || profile.MarketCap > 0 || profile.PE > 0 {
		_ = a.storage.SaveStockProfile(symbol, profile)
	}
	return profile, nil
}

// RefreshStockProfile 强制刷新股票基本资料
func (a *App) RefreshStockProfile(symbol string) (*StockProfile, error) {
	if a.storage == nil {
		return nil, fmt.Errorf("存储未初始化")
	}
	path := filepath.Join(a.storage.DataDir(), "data", symbol, "profile.json")
	_ = os.Remove(path)
	return a.GetStockProfile(symbol)
}

// RefreshIndustryBaselines 基于当前自选股数据刷新行业换手率基准
func (a *App) RefreshIndustryBaselines() (map[string]*analyzer.IndustryBaseline, error) {
	if a.storage == nil {
		return nil, fmt.Errorf("存储未初始化")
	}

	watchlist, err := a.storage.LoadWatchlist()
	if err != nil {
		return nil, fmt.Errorf("加载自选股失败: %w", err)
	}

	// 按行业收集换手率
	industryTurnovers := make(map[string][]float64)
	for _, item := range watchlist {
		profile, err := a.GetStockProfile(item.Code)
		if err != nil || profile == nil || profile.Industry == "" {
			continue
		}
		quote, err := a.GetStockQuote(item.Code)
		if err != nil || quote == nil || quote.TurnoverRate <= 0 {
			continue
		}
		industryTurnovers[profile.Industry] = append(industryTurnovers[profile.Industry], quote.TurnoverRate)
	}

	samples := make(map[string]*analyzer.IndustryBaseline)
	for industry, list := range industryTurnovers {
		if len(list) == 0 {
			continue
		}
		sort.Float64s(list)
		avgVal := 0.0
		for _, v := range list {
			avgVal += v
		}
		avgVal /= float64(len(list))
		medianVal := list[len(list)/2]
		if len(list)%2 == 0 {
			medianVal = (list[len(list)/2-1] + list[len(list)/2]) / 2
		}
		samples[industry] = &analyzer.IndustryBaseline{
			AvgTurnover:    avgVal,
			MedianTurnover: medianVal,
			SampleCount:    len(list),
		}
	}

	baselines := analyzer.BuildIndustryBaselines(samples)
	if err := a.storage.SaveIndustryBaselines(baselines); err != nil {
		return nil, fmt.Errorf("保存行业基准失败: %w", err)
	}
	return baselines, nil
}

// GetStockConcepts 获取股票概念与风口
func (a *App) GetStockConcepts(symbol string) (*downloader.StockConcepts, error) {
	if a.storage == nil {
		return nil, fmt.Errorf("存储未初始化")
	}

	// 尝试读取缓存（1小时）
	cached, err := a.storage.LoadStockConcepts(symbol)
	if err == nil && cached != nil {
		path := filepath.Join(a.storage.DataDir(), "data", symbol, "concepts.json")
		if info, err := os.Stat(path); err == nil && time.Since(info.ModTime()) < time.Hour {
			return cached, nil
		}
	}

	parts := strings.Split(symbol, ".")
	if len(parts) != 2 {
		return nil, fmt.Errorf("无效的股票代码格式: %s", symbol)
	}
	code := parts[0]
	market := strings.ToUpper(parts[1])

	// 获取实时行情用于风口判断
	var changePercent float64
	if q, err := a.GetStockQuote(symbol); err == nil && q != nil {
		changePercent = q.ChangePercent
	}

	concepts, err := downloader.FetchStockConcepts(market, code, changePercent)
	if err != nil {
		return nil, fmt.Errorf("获取概念数据失败: %w", err)
	}
	_ = a.storage.SaveStockConcepts(symbol, concepts)
	return concepts, nil
}

// DownloadReport 将分析报告保存为 Markdown 文件
func (a *App) DownloadReport(symbol string, markdownContent string) error {
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "保存分析报告",
		DefaultFilename: fmt.Sprintf("%s_投资分析报告.md", symbol),
		Filters: []runtime.FileFilter{
			{DisplayName: "Markdown", Pattern: "*.md"},
		},
	})
	if err != nil {
		return err
	}
	if path == "" {
		return fmt.Errorf("用户取消保存")
	}
	return os.WriteFile(path, []byte(markdownContent), 0644)
}

// ExportCurrentFinancialData 将当前股票财务数据导出为 zip
func (a *App) ExportCurrentFinancialData(symbol string) error {
	if a.storage == nil {
		return fmt.Errorf("存储未初始化")
	}
	stockDir := filepath.Join(a.storage.DataDir(), "data", symbol)
	files := []string{"balance_sheet.json", "income_statement.json", "cash_flow.json", "profile.json", "quote.json"}

	tmpDir := os.TempDir()
	zipName := fmt.Sprintf("%s_财务数据_%s.zip", symbol, time.Now().Format("20060102_150405"))
	tmpZip := filepath.Join(tmpDir, zipName)

	if err := createZipFromFiles(tmpZip, stockDir, files); err != nil {
		return err
	}
	defer os.Remove(tmpZip)

	savePath, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "导出当前财务数据",
		DefaultFilename: zipName,
		Filters: []runtime.FileFilter{
			{DisplayName: "ZIP 压缩包", Pattern: "*.zip"},
		},
	})
	if err != nil {
		return err
	}
	if savePath == "" {
		return fmt.Errorf("用户取消保存")
	}
	return copyFile(tmpZip, savePath)
}

// ExportHistoricalFinancialData 将指定历史批次财务数据导出为 zip
func (a *App) ExportHistoricalFinancialData(symbol string, timestamp string) error {
	if a.storage == nil {
		return fmt.Errorf("存储未初始化")
	}
	historyDir := filepath.Join(a.storage.DataDir(), "data", symbol, "history", timestamp)
	if _, err := os.Stat(historyDir); err != nil {
		return fmt.Errorf("历史数据不存在")
	}

	zipName := fmt.Sprintf("%s_财务数据_历史_%s.zip", symbol, timestamp)
	tmpDir := os.TempDir()
	tmpZip := filepath.Join(tmpDir, zipName)

	if err := createZipFromDir(tmpZip, historyDir); err != nil {
		return err
	}
	defer os.Remove(tmpZip)

	savePath, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "导出历史财务数据",
		DefaultFilename: zipName,
		Filters: []runtime.FileFilter{
			{DisplayName: "ZIP 压缩包", Pattern: "*.zip"},
		},
	})
	if err != nil {
		return err
	}
	if savePath == "" {
		return fmt.Errorf("用户取消保存")
	}
	return copyFile(tmpZip, savePath)
}

func createZipFromFiles(dst string, srcDir string, files []string) error {
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	defer zw.Close()
	for _, name := range files {
		src := filepath.Join(srcDir, name)
		if _, err := os.Stat(src); err != nil {
			continue
		}
		srcFile, err := os.Open(src)
		if err != nil {
			return err
		}
		w, err := zw.Create(name)
		if err != nil {
			srcFile.Close()
			return err
		}
		if _, err := io.Copy(w, srcFile); err != nil {
			srcFile.Close()
			return err
		}
		srcFile.Close()
	}
	return nil
}

// ReloadPolicyLibrary 热更新十五五政策库（重新加载外部 policy_library.json）
func (a *App) ReloadPolicyLibrary() error {
	if a.storage == nil {
		return fmt.Errorf("存储未初始化")
	}
	return analyzer.ReloadPolicyLibrary(a.storage.DataDir())
}

// GetPolicyLibraryMeta 获取当前政策库来源与更新时间
func (a *App) GetPolicyLibraryMeta() map[string]string {
	source, updatedAt := analyzer.GetPolicyLibraryMeta()
	return map[string]string{
		"source":     source,
		"updated_at": updatedAt.Format(time.RFC3339),
	}
}

// SaveDefaultPolicyLibrary 将内置默认政策库导出到本地 JSON，供用户编辑后热更新
func (a *App) SaveDefaultPolicyLibrary() error {
	if a.storage == nil {
		return fmt.Errorf("存储未初始化")
	}
	return analyzer.SaveDefaultPolicyLibrary(a.storage.DataDir())
}

// UpdatePolicyLibrary 调用 Python 脚本动态更新政策库 JSON，成功后自动热重载
func (a *App) UpdatePolicyLibrary() (*downloader.PolicyUpdateResult, error) {
	if a.storage == nil {
		return nil, fmt.Errorf("存储未初始化")
	}
	result, err := downloader.UpdatePolicyLibrary(a.storage.DataDir())
	if err != nil {
		return result, err
	}
	if reloadErr := analyzer.ReloadPolicyLibrary(a.storage.DataDir()); reloadErr != nil {
		return result, fmt.Errorf("更新成功但热重载失败: %w", reloadErr)
	}
	return result, nil
}

func createZipFromDir(dst string, srcDir string) error {
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	defer zw.Close()

	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		srcFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer srcFile.Close()
		w, err := zw.Create(rel)
		if err != nil {
			return err
		}
		_, err = io.Copy(w, srcFile)
		return err
	})
}
