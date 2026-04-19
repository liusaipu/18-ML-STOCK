package main

import (
	"archive/zip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	analyzer "github.com/stock-analyzer/analyzer"
	"github.com/stock-analyzer/downloader"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/xuri/excelize/v2"
	toast "git.sr.ht/~jackmordaunt/go-toast/v2"
)

// debugLog 直接写入日志文件，确保日志被记录
func debugLog(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	
	// 获取可执行文件所在目录
	exePath, err := os.Executable()
	if err != nil {
		return
	}
	exeDir := filepath.Dir(exePath)
	
	// 写入日志文件
	logFile := filepath.Join(exeDir, "debug.log")
	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return
	}
	defer f.Close()
	
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	f.WriteString(fmt.Sprintf("[%s] %s\n", timestamp, msg))
	
	// 同时输出到控制台
	fmt.Printf("[%s] %s\n", timestamp, msg)
}

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

	// 初始化行业均值数据库
	if err := analyzer.InitIndustryDatabase(a.storage.DataDir()); err != nil {
		fmt.Printf("初始化行业数据库失败: %v\n", err)
	}

	// 初始化政策库（优先加载外部 JSON，否则使用内置默认值）
	if err := analyzer.InitPolicyLibrary(a.storage.DataDir()); err != nil {
		fmt.Printf("初始化政策库失败: %v\n", err)
	}

	// 启动后台行业数据采集（如果满足条件）
	go a.startBackgroundIndustryUpdate()
}

// loadStockDB 从嵌入的资源或数据目录加载股票列表
func (a *App) loadStockDB() error {
	bytes, err := readStockJSON()
	if err != nil {
		return err
	}
	return json.Unmarshal(bytes, &a.stocks)
}

// fallbackIndustryScriptPath 返回 fetch_all_industry_data.py 绝对路径
func fallbackIndustryScriptPath() string {
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		p := filepath.Join(exeDir, "scripts", "fetch_all_industry_data.py")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	// 开发模式：从项目根目录查找
	base := filepath.Join(".", "scripts", "fetch_all_industry_data.py")
	if _, err := os.Stat(base); err == nil {
		return base
	}
	return ""
}

// shouldStartBackgroundIndustryUpdate 判断是否需要启动后台行业数据采集
func (a *App) shouldStartBackgroundIndustryUpdate() bool {
	dataDir := a.storage.DataDir()
	taskPath := filepath.Join(dataDir, "industry_task.json")
	
	// 如果任务文件不存在，直接启动
	data, err := os.ReadFile(taskPath)
	if err != nil {
		return true
	}
	
	var task struct {
		Status    string `json:"status"`
		UpdatedAt string `json:"updatedAt"`
	}
	if err := json.Unmarshal(data, &task); err != nil {
		return true
	}
	
	// 如果正在运行，不要重复启动
	if task.Status == "running" {
		return false
	}
	
	// 如果已完成，检查是否超过 7 天
	if task.Status == "completed" && task.UpdatedAt != "" {
		t, err := time.Parse("2006-01-02T15:04:05", task.UpdatedAt)
		if err == nil && time.Since(t) < 7*24*time.Hour {
			return false
		}
	}
	
	return true
}

// startBackgroundIndustryUpdate 启动后台行业数据采集
func (a *App) startBackgroundIndustryUpdate() {
	if a.storage == nil {
		return
	}
	if !a.shouldStartBackgroundIndustryUpdate() {
		return
	}
	
	script := fallbackIndustryScriptPath()
	if script == "" {
		fmt.Println("未找到 fetch_all_industry_data.py，跳过后台行业数据采集")
		return
	}
	
	python := "python"
	if _, err := exec.LookPath("python"); err != nil {
		python = "python3"
	}
	
	fmt.Printf("启动后台行业数据采集: %s\n", script)
	cmd := exec.Command(python, script, a.storage.DataDir())
	cmd.Env = append(os.Environ(), "PYTHONIOENCODING=utf-8")
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("后台行业数据采集失败: %v, output: %s\n", err, string(output))
		return
	}
	
	fmt.Printf("后台行业数据采集完成: %s\n", string(output))
	
	// 完成后重新加载行业数据库
	if reloadErr := analyzer.ReloadIndustryDatabase(a.storage.DataDir()); reloadErr != nil {
		fmt.Printf("后台行业数据热重载失败: %v\n", reloadErr)
	}
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

// WatchlistFilterItem 自选股筛选数据项
type WatchlistFilterItem struct {
	Code                  string  `json:"code"`
	Industry              string  `json:"industry"`
	ShareholderReturnRate float64 `json:"shareholderReturnRate"`
	AScore                float64 `json:"aScore"`
	HasFinancialData      bool    `json:"hasFinancialData"`
	HasSnapshot           bool    `json:"hasSnapshot"`
	LastAnalyzedAt        string  `json:"lastAnalyzedAt"`
}

// GetWatchlistFilterData 批量获取自选股的筛选数据
func (a *App) GetWatchlistFilterData() ([]WatchlistFilterItem, error) {
	if a.storage == nil {
		return nil, fmt.Errorf("存储未初始化")
	}
	watchlist, err := a.storage.LoadWatchlist()
	if err != nil {
		return nil, err
	}

	var results []WatchlistFilterItem
	for _, item := range watchlist {
		result := WatchlistFilterItem{Code: item.Code}

		// 1. 基本信息（行业、PB、市值）
		var profile *StockProfile
		if p, err := a.GetStockProfile(item.Code); err == nil && p != nil {
			profile = p
			result.Industry = p.Industry
		}

		// 2. 股东回报率（优先用本地财报计算，避免批量网络请求）
		if data, err := analyzer.LoadFinancialData(a.storage.DataDir(), item.Code); err == nil && len(data.Years) > 0 {
			year := data.Years[0]
			profit := data.GetValueOrZero(data.IncomeStatement, "净利润", year)
			equity := data.GetValueOrZero(data.BalanceSheet, "所有者权益合计", year)
			if equity > 0 {
				roe := profit / equity // 小数形式，如 0.1973
				if profile != nil && profile.PB > 0 {
					result.ShareholderReturnRate = roe / profile.PB
					// 股息率：用现金流量表分红现金 / 总市值
					dividendCash := data.GetValueOrZero(data.CashFlow, "分配股利、利润或偿付利息支付的现金", year)
					if profile.MarketCap > 0 && dividendCash > 0 {
						dy := dividendCash / profile.MarketCap
						if dy <= 0.20 { // 过滤异常高值
							result.ShareholderReturnRate += dy
						}
					}
				}
			}
		}

		// 3. 快照数据（A-Score）
		if snapshot, err := a.LoadAnalysisSnapshot(item.Code); err == nil && snapshot != nil {
			result.HasSnapshot = true
			if len(snapshot.Years) > 0 {
				latest := snapshot.Years[0]
				for _, step := range snapshot.StepResults {
					if step.StepNum == 8 {
						if yd, ok := step.YearlyData[latest]; ok && yd != nil {
							if v, ok2 := yd["AScore"].(float64); ok2 {
								result.AScore = v
							}
						}
						break
					}
				}
			}
		}

		// 4. 财报数据是否存在
		stockDir := filepath.Join(a.storage.DataDir(), "data", item.Code)
		if _, err := os.Stat(stockDir); err == nil {
			// 检查是否有 balance_sheet.json
			if _, err := os.Stat(filepath.Join(stockDir, "balance_sheet.json")); err == nil {
				result.HasFinancialData = true
			}
		}

		// 5. 最后分析时间（从历史报告文件名推断）
		if files, err := a.storage.ListReports(item.Code); err == nil && len(files) > 0 {
			// 报告文件名格式通常为 "report_YYYYMMDD_HHMMSS.md"
			result.LastAnalyzedAt = files[len(files)-1]
		}

		results = append(results, result)
	}
	return results, nil
}

// FinancialTrendItem 单一年度的财务指标
type FinancialTrendItem struct {
	Year          string   `json:"year"`
	ROE           *float64 `json:"roe"`
	GrossMargin   *float64 `json:"grossMargin"`
	RevenueGrowth *float64 `json:"revenueGrowth"`
	CashContent   *float64 `json:"cashContent"`
	DebtRatio     *float64 `json:"debtRatio"`
}

// FinancialTrendsData 财务指标趋势数据
type FinancialTrendsData struct {
	Symbol string               `json:"symbol"`
	Items  []FinancialTrendItem `json:"items"`
}

// GetFinancialTrends 获取股票近5年核心财务指标趋势
func (a *App) GetFinancialTrends(symbol string) (*FinancialTrendsData, error) {
	if a.storage == nil {
		return nil, fmt.Errorf("存储未初始化")
	}
	data, err := analyzer.LoadFinancialData(a.storage.DataDir(), symbol)
	if err != nil {
		return nil, fmt.Errorf("加载财务数据失败: %w", err)
	}
	if len(data.Years) == 0 {
		return &FinancialTrendsData{Symbol: symbol, Items: []FinancialTrendItem{}}, nil
	}

	maxYears := 5
	if len(data.Years) < maxYears {
		maxYears = len(data.Years)
	}

	items := make([]FinancialTrendItem, 0, maxYears)
	for i := 0; i < maxYears; i++ {
		year := data.Years[i]
		item := FinancialTrendItem{Year: year}

		// 1. ROE = 净利润 / 加权平均所有者权益 * 100
		profit := data.GetValueOrZero(data.IncomeStatement, "净利润", year)
		equity := data.GetValueOrZero(data.BalanceSheet, "所有者权益合计", year)
		var prevEquity float64
		if i+1 < len(data.Years) {
			prevEquity = data.GetValueOrZero(data.BalanceSheet, "所有者权益合计", data.Years[i+1])
		}
		weightedEquity := equity
		if prevEquity > 0 {
			weightedEquity = (prevEquity + equity) / 2
		}
		if weightedEquity > 0 {
			roe := profit / weightedEquity * 100
			item.ROE = &roe
		}

		// 2. 毛利率 = (营业收入 - 营业成本) / 营业收入 * 100
		revenue := data.GetValueOrZero(data.IncomeStatement, "营业收入", year)
		cost := data.GetValueOrZero(data.IncomeStatement, "营业成本", year)
		if revenue != 0 {
			gm := (revenue - cost) / revenue * 100
			item.GrossMargin = &gm
		}

		// 3. 营收增长率 = (本年营收 - 上年营收) / 上年营收 * 100
		if i+1 < len(data.Years) {
			prevRevenue := data.GetValueOrZero(data.IncomeStatement, "营业收入", data.Years[i+1])
			if prevRevenue != 0 {
				growth := (revenue - prevRevenue) / prevRevenue * 100
				item.RevenueGrowth = &growth
			}
		}

		// 4. 现金含量 = 经营现金流净额 / 净利润 * 100
		opCash := data.GetValueOrZero(data.CashFlow, "经营活动产生的现金流量净额", year)
		if profit != 0 {
			cc := opCash / profit * 100
			item.CashContent = &cc
		}

		// 5. 资产负债率 = 负债合计 / 资产合计 * 100
		asset := data.GetValueOrZero(data.BalanceSheet, "资产合计", year)
		liability := data.GetValueOrZero(data.BalanceSheet, "负债合计", year)
		if asset != 0 {
			dr := liability / asset * 100
			item.DebtRatio = &dr
		}

		items = append(items, item)
	}

	return &FinancialTrendsData{Symbol: symbol, Items: items}, nil
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

// FetchMissingActivityResult 获取缺失活跃度结果
type FetchMissingActivityResult struct {
	SuccessCount int      `json:"successCount"`
	FailCount    int      `json:"failCount"`
	FailedCodes  []string `json:"failedCodes"`
	Message      string   `json:"message"`
}

// FetchMissingActivity 批量获取可比公司缺失的活跃度（并发）
func (a *App) FetchMissingActivity(codes []string) (*FetchMissingActivityResult, error) {
	if a.storage == nil {
		return nil, fmt.Errorf("存储未初始化")
	}
	baselines, _ := a.storage.LoadIndustryBaselines()

	var wg sync.WaitGroup
	var mu sync.Mutex
	successCount := 0
	var failedCodes []string

	for _, code := range codes {
		// 先检查缓存是否存在
		if cached, err := a.storage.LoadActivityCache(code); err == nil && cached != nil {
			mu.Lock()
			successCount++
			mu.Unlock()
			continue
		}

		wg.Add(1)
		go func(c string) {
			defer wg.Done()

			pureCode := c
			market := ""
			if strings.Contains(c, ".") {
				parts := strings.SplitN(c, ".", 2)
				pureCode = parts[0]
				market = strings.ToUpper(parts[1])
			}
			if market == "" {
				switch {
				case strings.HasPrefix(pureCode, "6"):
					market = "SH"
				case strings.HasSuffix(c, ".HK") || strings.HasPrefix(pureCode, "00") || strings.HasPrefix(pureCode, "01") || strings.HasPrefix(pureCode, "02"):
					market = "HK"
				default:
					market = "SZ"
				}
			}

			klines, err := downloader.FetchStockKlines(market, pureCode, 60)
			if err != nil || len(klines) < 20 {
				fmt.Printf("[FetchMissingActivity] %s klines err=%v len=%d\n", c, err, len(klines))
				mu.Lock()
				failedCodes = append(failedCodes, c)
				mu.Unlock()
				return
			}
			quote, err := downloader.FetchStockQuote(market, pureCode)
			if err != nil || quote == nil || quote.CirculatingMarketCap <= 0 {
				fmt.Printf("[FetchMissingActivity] %s quote err=%v cap=%.0f\n", c, err, quote.CirculatingMarketCap)
				mu.Lock()
				failedCodes = append(failedCodes, c)
				mu.Unlock()
				return
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
			activity := analyzer.CalculateActivity(aklines, qLite, industry, baselines)
			if activity != nil && activity.Score > 0 {
				_ = a.storage.SaveActivityCache(c, activity)
				mu.Lock()
				successCount++
				mu.Unlock()
				fmt.Printf("[FetchMissingActivity] %s score=%.0f stars=%d\n", c, activity.Score, activity.Stars)
			} else {
				fmt.Printf("[FetchMissingActivity] %s skipped (nil or score=0)\n", c)
				mu.Lock()
				failedCodes = append(failedCodes, c)
				mu.Unlock()
			}
		}(code)
	}

	wg.Wait()

	failCount := len(failedCodes)
	msg := fmt.Sprintf("成功获取 %d 家", successCount)
	if failCount > 0 {
		msg += fmt.Sprintf("，失败 %d 家", failCount)
	}

	return &FetchMissingActivityResult{
		SuccessCount: successCount,
		FailCount:    failCount,
		FailedCodes:  failedCodes,
		Message:      msg,
	}, nil
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

	// 归档历史版本（使用Windows安全的时间格式）
	_ = a.storage.ArchiveStockData(symbol, HistoryMeta{
		Timestamp:  time.Now().Format("20060102_150405"),
		Source:     "csv_excel_import",
		SourceName: "同花顺/本地CSV/Excel",
		Years:      mergeYears(balanceYears, incomeYears, cashYears),
	})

	return result, nil
}

// DownloadReports 从网络下载指定股票的财报数据
func (a *App) DownloadReports(symbol string, maxYears int) (*DownloadResult, error) {
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
	data, err := downloader.DownloadFinancialReports(market, code, maxYears)
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

	// 归档历史版本（使用Windows安全的时间格式）
	_ = a.storage.ArchiveStockData(symbol, HistoryMeta{
		Timestamp:  time.Now().Format("20060102_150405"),
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

// UpdateModule4Only 仅更新报告中的模块4（行业横向对比分析），不重新执行完整分析
// 只重新计算财务指标和可比公司数据，不重复下载网络数据（行情、舆情等）
func (a *App) UpdateModule4Only(symbol string) (*analyzer.AnalysisReport, error) {
	if a.storage == nil {
		return nil, fmt.Errorf("存储未初始化")
	}

	// 1. 加载现有报告
	reports, err := a.storage.ListReports(symbol)
	if err != nil || len(reports) == 0 {
		return nil, fmt.Errorf("未找到现有报告，请先执行完整分析")
	}
	// 获取最新报告（通常是 reports[0] 或 latest.md）
	latestReportFile := reports[0]
	for _, r := range reports {
		if r == "latest.md" {
			latestReportFile = r
			break
		}
	}
	reportContent, err := a.storage.LoadReport(symbol, latestReportFile)
	if err != nil {
		return nil, fmt.Errorf("加载现有报告失败: %w", err)
	}

	// 2. 加载财务数据
	finData, err := analyzer.LoadFinancialData(a.storage.DataDir(), symbol)
	if err != nil || finData == nil {
		return nil, fmt.Errorf("加载财务数据失败: %w", err)
	}

	// 3. 获取可比公司分析数据（刚下载的数据）
	comparables, _ := a.storage.GetComparables(symbol)
	nameMap := make(map[string]string, len(a.stocks))
	for _, s := range a.stocks {
		nameMap[s.Code] = s.Name
	}
	compAnalysis, _ := analyzer.BuildComparableAnalysis(a.storage.DataDir(), comparables, nameMap)

	// 4. 获取行业对比数据
	var industry *analyzer.IndustryComparison
	if profile, err := a.storage.LoadStockProfile(symbol); err == nil && profile != nil && profile.Industry != "" {
		// IndustryComparison 会在 RegenerateModule4Only 内部重新计算
		industry = analyzer.CompareWithIndustry(profile.Industry, nil, "")
	}

	// 5. 生成新的模块4内容（只计算模块4，不涉及网络请求）
	newModule4, err := analyzer.RegenerateModule4Only(a.storage.DataDir(), symbol, compAnalysis, industry)
	if err != nil {
		return nil, fmt.Errorf("生成模块4失败: %w", err)
	}

	// 7. 替换报告中的模块4部分
	updatedContent := replaceModule4InReport(reportContent, newModule4)

	// 8. 保存更新后的报告
	_, _ = a.storage.SaveReport(symbol, updatedContent, true)

	// 8.5 保存分析快照（用于前端亮点与风险恢复）
	_ = a.storage.SaveSnapshot(symbol, &analyzer.AnalysisReport{
		Symbol:          symbol,
		MarkdownContent: updatedContent,
		Years:           finData.Years,
	})

	// 9. 更新缓存哈希
	compHash, _ := a.storage.ComputeComparablesHash(symbol)
	_ = a.storage.SaveAnalysisCache(symbol, "", compHash)

	// 10. 返回简化报告对象
	return &analyzer.AnalysisReport{
		Symbol:          symbol,
		MarkdownContent: updatedContent,
		Years:           finData.Years,
	}, nil
}

func replaceModule4InReport(reportContent, newModule4 string) string {
	// 查找模块4的开始位置
	module4Start := strings.Index(reportContent, "# 模块4: 行业横向对比分析")
	if module4Start == -1 {
		// 如果没找到模块4标题，尝试其他模式
		module4Start = strings.Index(reportContent, "# 模块4")
	}
	if module4Start == -1 {
		// 还是找不到，直接追加到末尾
		return reportContent + "\n\n" + newModule4
	}

	// 查找下一个模块的开始位置（模块5或后续内容）
	module5Start := strings.Index(reportContent[module4Start:], "\n# 模块5:")
	if module5Start == -1 {
		// 尝试找任何以 "# 模块" 开头的内容
		module5Start = strings.Index(reportContent[module4Start+1:], "\n# 模块")
	}
	if module5Start == -1 {
		// 如果没找到下一个模块，替换从模块4开始到末尾
		return reportContent[:module4Start] + newModule4
	}

	// 替换模块4内容
	module5Start += module4Start // 调整到绝对位置
	return reportContent[:module4Start] + newModule4 + reportContent[module5Start:]
}

// GetModule4Status 检查模块4是否可以更新（可比公司数据是否已下载）
func (a *App) GetModule4Status(symbol string) (bool, error) {
	if a.storage == nil {
		return false, fmt.Errorf("存储未初始化")
	}

	comparables, err := a.storage.GetComparables(symbol)
	if err != nil || len(comparables) == 0 {
		return false, nil
	}

	// 检查是否已下载可比公司财报
	nameMap := make(map[string]string, len(a.stocks))
	for _, s := range a.stocks {
		nameMap[s.Code] = s.Name
	}
	compAnalysis, err := analyzer.BuildComparableAnalysis(a.storage.DataDir(), comparables, nameMap)
	if err != nil {
		return false, err
	}

	return compAnalysis != nil && compAnalysis.HasData && len(compAnalysis.Metrics) > 0, nil
}

// GetStockKlines 获取股票历史K线数据（日K），优先读取本地缓存
func (a *App) GetStockKlines(symbol string) ([]downloader.KlineData, error) {
	if a.storage == nil {
		return nil, fmt.Errorf("存储未初始化")
	}
	// 优先读取本地缓存（分析报告生成时保存的K线数据）
	if cached, err := a.storage.LoadStockKlines(symbol); err == nil && len(cached) > 0 {
		// 检查缓存的K线是否有换手率，如果没有，尝试用 quote 补算
		hasTurnover := false
		for _, k := range cached {
			if k.TurnoverRate > 0 {
				hasTurnover = true
				break
			}
		}
		if !hasTurnover {
			if quote, err := a.GetStockQuote(symbol); err == nil && quote != nil && quote.CirculatingMarketCap > 0 && quote.CurrentPrice > 0 {
				circulatingShares := quote.CirculatingMarketCap / quote.CurrentPrice
				for i := range cached {
					cached[i].TurnoverRate = (cached[i].Volume * 100 / circulatingShares) * 100
				}
				debugLog("[GetStockKlines] %s cache hit but no turnover, computed for %d klines", symbol, len(cached))
			} else {
				debugLog("[GetStockKlines] %s cache hit but no turnover and no quote available", symbol)
			}
		}
		debugLog("[GetStockKlines] %s cache hit, len=%d, first turnoverRate=%.2f", symbol, len(cached), cached[0].TurnoverRate)
		return cached, nil
	} else if err != nil {
		debugLog("[GetStockKlines] %s cache read error: %v", symbol, err)
	} else {
		debugLog("[GetStockKlines] %s cache miss or empty", symbol)
	}
	parts := strings.Split(symbol, ".")
	if len(parts) != 2 {
		return nil, fmt.Errorf("无效的股票代码格式: %s", symbol)
	}
	code := parts[0]
	market := strings.ToUpper(parts[1])
	return downloader.FetchStockKlines(market, code, 250)
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
				a.fillShareholderReturnRate(symbol, cached)
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
	a.fillShareholderReturnRate(symbol, quote)
	return quote, nil
}

// fillShareholderReturnRate 根据最新财务数据填充股东回报率
func (a *App) fillShareholderReturnRate(symbol string, quote *downloader.StockQuote) {
	if quote == nil || quote.PB <= 0 {
		return
	}
	// 尝试从本地财务数据计算最新一年 ROE（百分比数值，如 20 表示 20%）
	var roe float64
	if data, err := analyzer.LoadFinancialData(a.storage.DataDir(), symbol); err == nil && len(data.Years) > 0 {
		year := data.Years[0] // 最新年份
		profit := data.GetValueOrZero(data.IncomeStatement, "净利润", year)
		equity := data.GetValueOrZero(data.BalanceSheet, "所有者权益合计", year)
		if equity > 0 {
			roe = profit / equity * 100
		}
		// 优先用现金流量表中的分红现金估算股息率（虽包含利息支出，会轻微高估，但不会出现行情接口的异常高值）
		dividendCash := data.GetValueOrZero(data.CashFlow, "分配股利、利润或偿付利息支付的现金", year)
		if dividendCash > 0 && quote.MarketCap > 0 {
			quote.DividendYield = dividendCash / quote.MarketCap
		}
	}
	// 若财务数据未覆盖，且行情接口的股息率异常（>20% 视为不可靠），则清零
	if quote.DividendYield > 0.20 {
		quote.DividendYield = 0
	}
	if roe > 0 {
		// roe 是百分比数值，需先除以 100 转为小数，再与股息率（已是小数）相加
		quote.ShareholderReturnRate = (roe / 100) / quote.PB
		if quote.DividendYield > 0 {
			quote.ShareholderReturnRate += quote.DividendYield
		}
	}
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
	debugLog("[CheckAnalysisCache] Checking cache for %s", symbol)
	if a.storage == nil {
		debugLog("[CheckAnalysisCache] Error: storage not initialized")
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

	result := &CacheStatus{
		Unchanged:          !dataChanged && !comparablesChanged,
		LastAnalysisAt:     lastAnalysisAt,
		DataChanged:        dataChanged,
		ComparablesChanged: comparablesChanged,
	}
	debugLog("[CheckAnalysisCache] Result for %s: unchanged=%v", symbol, result.Unchanged)
	return result, nil
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
	debugLog("[AnalyzeStock] Starting analysis for %s, overwriteLatest=%v", symbol, overwriteLatest)
	if a.storage == nil {
		debugLog("[AnalyzeStock] Error: storage not initialized")
		return nil, fmt.Errorf("存储未初始化")
	}
	debugLog("[AnalyzeStock] Storage initialized, dataDir=%s", a.storage.DataDir())
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
		defer func() {
			if r := recover(); r != nil {
				debugLog("[PANIC] quote goroutine: %v", r)
			}
			wgNet.Done()
		}()
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
		defer func() {
			if r := recover(); r != nil {
				debugLog("[PANIC] klines goroutine: %v", r)
			}
			wgNet.Done()
		}()
		parts := strings.Split(symbol, ".")
		if len(parts) == 2 {
			if list, err := downloader.FetchStockKlines(strings.ToUpper(parts[1]), parts[0], 250); err == nil && len(list) >= 20 {
				klines = list
			}
		}
	}()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				debugLog("[PANIC] sentiment goroutine: %v", r)
			}
			wgNet.Done()
		}()
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
				debugLog("[Sentiment] fetching for %s %s", market, code)
				if s, err := downloader.FetchStockSentiment(market, code); err == nil && s != nil {
					debugLog("[Sentiment] fetched ok for %s, summaries=%d", symbol, len(s.Summaries))
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
					debugLog("[Sentiment] fetch failed for %s: %v", symbol, err)
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
	debugLog("[AnalyzeStock] Starting ML engines, finData=%v, klines=%d", finData != nil, len(klines))
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
					debugLog("[ML] Engine B failed for %s: %v", symbol, err)
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
						debugLog("[ML] Engine A failed for %s: %v", symbol, err)
					}
				}
				// Engine D: 风险预警
				if dFeatures := analyzer.BuildMLEngineDInput(finData); len(dFeatures) > 0 {
					if dp, err := analyzer.RunMLEngineD(dFeatures); err == nil {
						mlLocal.EngineD = dp
					} else {
						debugLog("[ML] Engine D failed for %s: %v", symbol, err)
					}
				}
			}
			if mlLocal.Financial != nil || mlLocal.Sentiment != nil || mlLocal.EngineD != nil {
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
	debugLog("[AnalyzeStock] ML and RIM data fetching completed, mlData=%v, extRIM=%v", mlData != nil, extRIM != nil)

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
	// 保存分析快照（用于前端亮点与风险恢复）
	_ = a.storage.SaveSnapshot(symbol, report)
	// 保存K线数据缓存（供报告图表展示使用）
	if len(klines) > 0 {
		// 如果K线数据里没有换手率，用 quote 数据补算
		hasTurnover := false
		for _, k := range klines {
			if k.TurnoverRate > 0 {
				hasTurnover = true
				break
			}
		}
		if !hasTurnover && quoteData != nil && quoteData.CirculatingMarketCap > 0 && quoteData.CurrentPrice > 0 {
			circulatingShares := quoteData.CirculatingMarketCap / quoteData.CurrentPrice
			for i := range klines {
				klines[i].TurnoverRate = (klines[i].Volume * 100 / circulatingShares) * 100
			}
			debugLog("[AnalyzeStock] %s computed turnoverRate for %d klines, first=%.2f%%", symbol, len(klines), klines[0].TurnoverRate)
		}
		debugLog("[AnalyzeStock] %s saving klines cache, len=%d, first turnoverRate=%.2f", symbol, len(klines), klines[0].TurnoverRate)
		if err := a.storage.SaveStockKlines(symbol, klines); err != nil {
			debugLog("[AnalyzeStock] %s save klines cache error: %v", symbol, err)
		} else {
			debugLog("[AnalyzeStock] %s klines cache saved", symbol)
		}
	} else {
		debugLog("[AnalyzeStock] %s no klines to cache", symbol)
	}
	// 保存分析缓存
	if hash, err := a.storage.ComputeDataHash(symbol); err == nil {
		if compHash, err := a.storage.ComputeComparablesHash(symbol); err == nil {
			_ = a.storage.SaveAnalysisCache(symbol, hash, compHash)
		}
	}
	debugLog("[AnalyzeStock] Analysis completed successfully for %s", symbol)
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

// LoadAnalysisSnapshot 加载指定股票的最新分析快照（用于恢复亮点与风险面板）
func (a *App) LoadAnalysisSnapshot(symbol string) (*analyzer.AnalysisReport, error) {
	if a.storage == nil {
		return nil, fmt.Errorf("存储未初始化")
	}
	return a.storage.LoadSnapshot(symbol)
}

// ReloadIndustryDatabase 重新加载行业均值数据库（供用户手动刷新）
func (a *App) ReloadIndustryDatabase() error {
	if a.storage == nil {
		return fmt.Errorf("存储未初始化")
	}
	return analyzer.ReloadIndustryDatabase(a.storage.DataDir())
}

// GetRiskRadar 获取股票行业对比雷达
func (a *App) GetRiskRadar(symbol string, industry string) ([]analyzer.RiskRadarItem, error) {
	if a.storage == nil {
		return nil, fmt.Errorf("存储未初始化")
	}
	// 优先读取快照，避免重复计算
	snapshot, err := a.storage.LoadSnapshot(symbol)
	if err == nil && snapshot != nil && len(snapshot.StepResults) > 0 {
		return analyzer.BuildRiskRadar(snapshot.StepResults, nil, snapshot.Years, industry), nil
	}
	// 无快照则重新执行分析
	report, err := analyzer.RunAnalysis(a.storage.DataDir(), symbol)
	if err != nil {
		return nil, err
	}
	return analyzer.BuildRiskRadar(report.StepResults, nil, report.Years, industry), nil
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
		// 用户取消保存，不报错
		return nil
	}
	return os.WriteFile(path, []byte(markdownContent), 0644)
}

// ExportReportPDF 将 PDF base64 数据保存为用户选择的文件
func (a *App) ExportReportPDF(symbol string, base64Data string) error {
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "保存 PDF 报告",
		DefaultFilename: fmt.Sprintf("%s_投资分析报告.pdf", symbol),
		Filters: []runtime.FileFilter{
			{DisplayName: "PDF", Pattern: "*.pdf"},
		},
	})
	if err != nil {
		return err
	}
	if path == "" {
		return nil
	}
	data, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return fmt.Errorf("解码 PDF 数据失败: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// ExportReportImage 将图片 base64 DataURL 保存为用户选择的文件
func (a *App) ExportReportImage(symbol string, dataURL string) error {
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "保存图片",
		DefaultFilename: fmt.Sprintf("%s_投资分析报告.png", symbol),
		Filters: []runtime.FileFilter{
			{DisplayName: "PNG", Pattern: "*.png"},
		},
	})
	if err != nil {
		return err
	}
	if path == "" {
		return nil
	}
	parts := strings.SplitN(dataURL, ",", 2)
	if len(parts) != 2 {
		return fmt.Errorf("无效的图片数据")
	}
	data, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return fmt.Errorf("解码图片数据失败: %w", err)
	}
	return os.WriteFile(path, data, 0644)
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

// ExportFinancialDataToExcel 将当前股票财务数据及18步分析结果导出为 Excel
func (a *App) ExportFinancialDataToExcel(symbol string) error {
	if a.storage == nil {
		return fmt.Errorf("存储未初始化")
	}
	data, err := analyzer.LoadFinancialData(a.storage.DataDir(), symbol)
	if err != nil {
		return fmt.Errorf("加载财务数据失败: %w", err)
	}
	if len(data.Years) == 0 {
		return fmt.Errorf("没有可用的财务数据")
	}

	// 尝试读取快照获取18步分析结果
	var steps []analyzer.StepResult
	snapshot, _ := a.storage.LoadSnapshot(symbol)
	if snapshot != nil && len(snapshot.StepResults) > 0 {
		steps = snapshot.StepResults
	}

	f := excelize.NewFile()
	defer f.Close()

	// 辅助函数：写入财务报表 sheet
	writeSheet := func(sheetName string, statement map[string]map[string]float64) {
		f.NewSheet(sheetName)
		// 表头：科目 | 年份1 | 年份2 | ...
		f.SetCellValue(sheetName, "A1", "科目")
		for i, year := range data.Years {
			col, _ := excelize.ColumnNumberToName(i + 2)
			f.SetCellValue(sheetName, col+"1", year)
		}
		// 收集所有科目并按字母排序
		var accounts []string
		for acc := range statement {
			accounts = append(accounts, acc)
		}
		sort.Strings(accounts)
		for r, acc := range accounts {
			row := r + 2
			f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), acc)
			for i, year := range data.Years {
				col, _ := excelize.ColumnNumberToName(i + 2)
				val := statement[acc][year]
				f.SetCellValue(sheetName, fmt.Sprintf("%s%d", col, row), val)
			}
		}
		// 设置列宽
		f.SetColWidth(sheetName, "A", "A", 40)
		for i := 0; i < len(data.Years); i++ {
			col, _ := excelize.ColumnNumberToName(i + 2)
			f.SetColWidth(sheetName, col, col, 16)
		}
	}

	writeSheet("资产负债表", data.BalanceSheet)
	writeSheet("利润表", data.IncomeStatement)
	writeSheet("现金流量表", data.CashFlow)

	// Sheet4: 18步分析汇总
	analysisSheet := "18步分析汇总"
	f.NewSheet(analysisSheet)
	f.SetCellValue(analysisSheet, "A1", "步骤")
	f.SetCellValue(analysisSheet, "B1", "分析维度")
	for i, year := range data.Years {
		col, _ := excelize.ColumnNumberToName(i + 3)
		f.SetCellValue(analysisSheet, col+"1", year)
	}
	if len(steps) > 0 {
		for r, step := range steps {
			row := r + 2
			f.SetCellValue(analysisSheet, fmt.Sprintf("A%d", row), step.StepNum)
			f.SetCellValue(analysisSheet, fmt.Sprintf("B%d", row), step.StepName)
			for i, year := range data.Years {
				col, _ := excelize.ColumnNumberToName(i + 3)
				passed := step.Pass[year]
				status := "未达标"
				if passed {
					status = "达标"
				}
				// 如果有数值型展示值，附加在状态后
				if yd, ok := step.YearlyData[year]; ok && len(yd) > 0 {
					for k, v := range yd {
						if k != "status" && k != "competitiveness" && k != "risk" && k != "companyType" && k != "focus" && k != "control" && k != "innovation" && k != "salesDifficulty" && k != "profitability" && k != "quality" && k != "assessment" && k != "sustainability" && k != "note" && k != "fraudRisk" {
							status += fmt.Sprintf(" (%v: %v)", k, v)
							break
						}
					}
				}
				f.SetCellValue(analysisSheet, fmt.Sprintf("%s%d", col, row), status)
			}
		}
	} else {
		f.SetCellValue(analysisSheet, "A2", "暂无分析数据（请先执行18步分析）")
	}
	f.SetColWidth(analysisSheet, "A", "A", 8)
	f.SetColWidth(analysisSheet, "B", "B", 45)
	for i := 0; i < len(data.Years); i++ {
		col, _ := excelize.ColumnNumberToName(i + 3)
		f.SetColWidth(analysisSheet, col, col, 25)
	}

	// 删除默认 Sheet1
	f.DeleteSheet("Sheet1")

	tmpDir := os.TempDir()
	fileName := fmt.Sprintf("%s_财务数据_%s.xlsx", symbol, time.Now().Format("20060102_150405"))
	tmpPath := filepath.Join(tmpDir, fileName)
	if err := f.SaveAs(tmpPath); err != nil {
		return fmt.Errorf("保存Excel失败: %w", err)
	}
	defer os.Remove(tmpPath)

	savePath, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "导出财务数据 Excel",
		DefaultFilename: fileName,
		Filters: []runtime.FileFilter{
			{DisplayName: "Excel 文件", Pattern: "*.xlsx"},
		},
	})
	if err != nil {
		return err
	}
	if savePath == "" {
		return fmt.Errorf("用户取消保存")
	}
	return copyFile(tmpPath, savePath)
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

// InitIndustryDatabase 初始化行业均值数据库
func (a *App) InitIndustryDatabase() error {
	if a.storage == nil {
		return fmt.Errorf("存储未初始化")
	}
	return analyzer.InitIndustryDatabase(a.storage.DataDir())
}

// UpdateIndustryDatabase 调用 Python 脚本更新行业均值数据库
func (a *App) UpdateIndustryDatabase() (*downloader.IndustryUpdateResult, error) {
	if a.storage == nil {
		return nil, fmt.Errorf("存储未初始化")
	}
	result, err := downloader.UpdateIndustryDatabase(a.storage.DataDir())
	if err != nil {
		return result, err
	}
	// 重新加载数据库
	if reloadErr := analyzer.ReloadIndustryDatabase(a.storage.DataDir()); reloadErr != nil {
		return result, fmt.Errorf("更新成功但热重载失败: %w", reloadErr)
	}
	return result, nil
}

// SendNotification 发送系统通知（Windows Toast）
func (a *App) SendNotification(title, content string) error {
	notification := toast.Notification{
		AppID: "Stock Analyzer",
		Title: title,
		Body:  content,
	}
	return notification.Push()
}

// GetIndustryDBMeta 获取行业数据库元信息
func (a *App) GetIndustryDBMeta() map[string]interface{} {
	version, updatedAt, count := analyzer.GetIndustryDBMeta()
	return map[string]interface{}{
		"version":    version,
		"updatedAt":  updatedAt,
		"count":      count,
	}
}

// GetIndustryTaskStatus 获取后台行业数据采集任务状态
func (a *App) GetIndustryTaskStatus() map[string]interface{} {
	if a.storage == nil {
		return map[string]interface{}{"status": "idle"}
	}
	taskPath := filepath.Join(a.storage.DataDir(), "industry_task.json")
	data, err := os.ReadFile(taskPath)
	if err != nil {
		return map[string]interface{}{"status": "idle"}
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return map[string]interface{}{"status": "idle"}
	}
	return result
}

// GetIndustryMetrics 获取指定行业的均值指标
func (a *App) GetIndustryMetrics(industry string) (*analyzer.IndustryMetrics, bool) {
	return analyzer.GetIndustryMetrics(industry)
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

// ConfirmDialog 显示确认对话框，返回用户是否点击了"确定"
func (a *App) ConfirmDialog(title, message string) bool {
	selection, err := runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
		Type:          runtime.QuestionDialog,
		Title:         title,
		Message:       message,
		Buttons:       []string{"确定", "取消"},
		DefaultButton: "确定",
		CancelButton:  "取消",
	})
	if err != nil {
		debugLog("[ConfirmDialog] error: %v", err)
		return false
	}
	debugLog("[ConfirmDialog] selection=%q", selection)
	// Windows 系统对话框可能返回英文按钮文本，做兼容处理
	return selection == "确定" || selection == "Yes" || selection == "OK"
}
