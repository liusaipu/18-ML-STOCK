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
	"strconv"
	"strings"
	"sync"
	"time"

	analyzer "github.com/liusaipu/stockfinlens/analyzer"
	"github.com/liusaipu/stockfinlens/downloader"
	"github.com/liusaipu/stockfinlens/updater"

	toast "git.sr.ht/~jackmordaunt/go-toast/v2"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/xuri/excelize/v2"
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
	ctx             context.Context
	storage         *Storage
	stocks          []StockInfo // 内存中的股票代码库
	analysisMu      sync.Mutex
	analysisLocks   map[string]*sync.Mutex
	dataRouter      *downloader.DataRouter // 数据源路由
	riskSensitivity string                 // 风险警示敏感度
	appConfig       *AppConfig             // 应用配置（自动更新等）
	currentVersion  string                 // 当前版本号
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
	Success          bool                          `json:"success"`
	Message          string                        `json:"message"`
	Years            []string                      `json:"years"`
	Validation       []downloader.ValidationResult `json:"validation"`
	SourceName       string                        `json:"sourceName"`       // 数据来源（StockFinLens / 东方财富 / 腾讯财经）
	QualityScore     float64                       `json:"qualityScore"`     // 资产负债表质量得分 0-100
	SourceSuggestion string                        `json:"sourceSuggestion"` // 数据源切换建议，空字符串表示无建议
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

	// 初始化数据源路由
	a.reloadDataRouter()

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

	// 加载应用配置
	appCfg, err := a.storage.LoadAppConfig()
	if err != nil {
		fmt.Printf("加载应用配置失败: %v\n", err)
		appCfg = &AppConfig{AutoCheckUpdate: true}
	}
	a.appConfig = appCfg

	// 读取当前版本号（从 wails.json）
	a.currentVersion = readWailsVersion()

	// 启动时自动检查更新（非阻塞，后台执行）
	go a.checkUpdateOnStartup()
}

// loadStockDB 从嵌入的资源或数据目录加载股票列表
func (a *App) loadStockDB() error {
	bytes, err := readStockJSON()
	if err != nil {
		return err
	}
	return json.Unmarshal(bytes, &a.stocks)
}

// readWailsVersion 读取 wails.json 中的 productVersion
func readWailsVersion() string {
	data, err := os.ReadFile("wails.json")
	if err != nil {
		// 打包后尝试从可执行文件同级目录读取
		if exe, err := os.Executable(); err == nil {
			data, _ = os.ReadFile(filepath.Join(filepath.Dir(exe), "wails.json"))
		}
	}
	if len(data) == 0 {
		return "unknown"
	}
	var cfg struct {
		Info struct {
			ProductVersion string `json:"productVersion"`
		} `json:"info"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return "unknown"
	}
	return cfg.Info.ProductVersion
}

// checkUpdateOnStartup 启动时自动检查更新（非阻塞）
func (a *App) checkUpdateOnStartup() {
	if a.appConfig == nil || !a.appConfig.AutoCheckUpdate {
		return
	}
	// 避免每次启动都请求：一天最多检查一次
	today := time.Now().Format("2006-01-02")
	if a.appConfig.LastCheckDate == today {
		return
	}
	// 延迟 5 秒再检查，避免影响启动速度
	time.Sleep(5 * time.Second)

	info, err := updater.CheckUpdate(a.currentVersion)
	if err != nil {
		fmt.Printf("[AutoUpdate] 检查更新失败: %v\n", err)
		return
	}
	// 记录检查日期（无论是否发现更新）
	a.appConfig.LastCheckDate = today
	_ = a.storage.SaveAppConfig(a.appConfig)

	if info.HasUpdate && info.LatestVer != a.appConfig.SkipVersion {
		runtime.EventsEmit(a.ctx, "update:available", info)
	}
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

// CheckPythonDependencies 检测 Python 环境及依赖包状态
func (a *App) CheckPythonDependencies() *PythonEnvResult {
	return CheckPythonEnv()
}

// InstallPythonDependencies 安装缺失的 Python 依赖包
func (a *App) InstallPythonDependencies(packages []string) error {
	python := findPythonExecutable()
	if python == "" {
		return fmt.Errorf("未找到 Python 可执行文件，请先安装 Python 3.10+")
	}

	return InstallPythonPackages(python, packages, func(line string) {
		// 通过 Wails Events 向前端发送实时安装日志
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "python:install:progress", strings.TrimSpace(line))
		}
	})
}

// MarkPythonDepsChecked 标记 Python 依赖已检查过
func (a *App) MarkPythonDepsChecked() {
	_ = markDepsChecked()
}

// HasPythonDepsChecked 检查是否已做过依赖检测
func (a *App) HasPythonDepsChecked() bool {
	return hasCheckedDeps()
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
	RiskLevel             string  `json:"riskLevel"`
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

		// 3. 快照数据（A-Score + 风险等级）
		if snapshot, err := a.LoadAnalysisSnapshot(item.Code); err == nil && snapshot != nil {
			result.HasSnapshot = true
			if snapshot.RiskAlert != nil {
				result.RiskLevel = snapshot.RiskAlert.Level
			}
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
			var klines []downloader.KlineData
			var quote *downloader.StockQuote
			var kErr, qErr error
			if a.dataRouter != nil {
				klines, kErr = a.dataRouter.FetchKlines(market, pureCode, 60)
				quote, qErr = a.dataRouter.FetchQuote(market, pureCode)
			} else {
				klines, kErr = downloader.FetchStockKlines(market, pureCode, 60)
				quote, qErr = downloader.FetchStockQuote(market, pureCode)
			}
			if kErr != nil || len(klines) < 20 {
				fmt.Printf("[GetWatchlistActivity] %s klines err=%v len=%d\n", item.Code, kErr, len(klines))
				continue
			}
			if qErr != nil || quote == nil || quote.CirculatingMarketCap <= 0 {
				fmt.Printf("[GetWatchlistActivity] %s quote err=%v cap=%.0f\n", item.Code, qErr, quote.CirculatingMarketCap)
				continue
			}
			var profile *downloader.StockProfile
			if a.dataRouter != nil {
				profile, _ = a.dataRouter.FetchProfile(market, pureCode)
			} else {
				profile, _ = downloader.FetchStockProfile(market, pureCode)
			}
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

			var klines []downloader.KlineData
			var quote *downloader.StockQuote
			var kErr, qErr error
			if a.dataRouter != nil {
				klines, kErr = a.dataRouter.FetchKlines(market, pureCode, 60)
				quote, qErr = a.dataRouter.FetchQuote(market, pureCode)
			} else {
				klines, kErr = downloader.FetchStockKlines(market, pureCode, 60)
				quote, qErr = downloader.FetchStockQuote(market, pureCode)
			}
			if kErr != nil || len(klines) < 20 {
				fmt.Printf("[FetchMissingActivity] %s klines err=%v len=%d\n", c, kErr, len(klines))
				mu.Lock()
				failedCodes = append(failedCodes, c)
				mu.Unlock()
				return
			}
			if qErr != nil || quote == nil || quote.CirculatingMarketCap <= 0 {
				fmt.Printf("[FetchMissingActivity] %s quote err=%v cap=%.0f\n", c, qErr, quote.CirculatingMarketCap)
				mu.Lock()
				failedCodes = append(failedCodes, c)
				mu.Unlock()
				return
			}
			var profile *downloader.StockProfile
			if a.dataRouter != nil {
				profile, _ = a.dataRouter.FetchProfile(market, pureCode)
			} else {
				profile, _ = downloader.FetchStockProfile(market, pureCode)
			}
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

	var data *downloader.FinancialReportData
	var err error
	var sourceName string

	// 1. 优先尝试 DataRouter（StockFinLens 数据源）
	if a.dataRouter != nil && market != "HK" {
		tfd, tErr := a.dataRouter.FetchFinancialData(market, code)
		if tErr == nil && tfd != nil {
			data = a.dataRouter.ConvertToFinancialReportData(tfd, symbol)
			if data != nil && len(data.Years) > 0 {
				fmt.Printf("[DownloadReports] %s financial data from StockFinLens, years=%d\n", symbol, len(data.Years))
				sourceName = "StockFinLens"
			} else {
				data = nil
			}
		}
	}

	// 2. Fallback 到原有下载链（东方财富）
	if data == nil {
		data, err = downloader.DownloadFinancialReports(market, code, maxYears)
		if err != nil {
			return nil, fmt.Errorf("下载财报失败: %w", err)
		}
		sourceName = "东方财富"
	}

	// 3. 评估主数据源质量，若质量差则尝试备用源
	quality := downloader.EvaluateBalanceQuality(data)
	var sourceSuggestion string
	if quality != nil && quality.SuggestAlternativeSource(0.05) {
		fmt.Printf("[DownloadReports] %s 主数据源(%s)质量欠佳: maxDiff=%.2f%%, score=%.1f，尝试备用源\n",
			symbol, sourceName, quality.MaxDiffRatio*100, quality.Score)

		var altData *downloader.FinancialReportData
		var altSourceName string

		if sourceName == "东方财富" && a.dataRouter != nil && market != "HK" {
			// 主源是东财，尝试 StockFinLens
			tfd, tErr := a.dataRouter.FetchFinancialData(market, code)
			if tErr == nil && tfd != nil {
				altData = a.dataRouter.ConvertToFinancialReportData(tfd, symbol)
				if altData != nil && len(altData.Years) > 0 {
					altSourceName = "StockFinLens"
				}
			}
		} else if sourceName == "StockFinLens" {
			// 主源是 StockFinLens，尝试东财
			altData, _ = downloader.DownloadFinancialReports(market, code, maxYears)
			if altData != nil && len(altData.Years) > 0 {
				altSourceName = "东方财富"
			}
		}

		if altData != nil && altSourceName != "" {
			altQuality := downloader.EvaluateBalanceQuality(altData)
			if altQuality != nil {
				fmt.Printf("[DownloadReports] %s 备用源(%s)质量: maxDiff=%.2f%%, score=%.1f\n",
					symbol, altSourceName, altQuality.MaxDiffRatio*100, altQuality.Score)
				if altQuality.IsBetterThan(quality) {
					originalSourceName := sourceName
					originalMaxDiff := quality.MaxDiffRatio
					fmt.Printf("[DownloadReports] %s 选择备用源(%s)数据，质量更优\n", symbol, altSourceName)
					data = altData
					sourceName = altSourceName
					quality = altQuality
					sourceSuggestion = fmt.Sprintf("该股票财报数据已从%s切换至%s，资产负债表平衡性更优（差异从%.1f%%降至%.1f%%）",
						originalSourceName, altSourceName, originalMaxDiff*100, altQuality.MaxDiffRatio*100)
				}
			}
		}
	}

	// 补充缺失的分红数据：若现金流量表中分红字段全为0，尝试从东财API获取
	if data != nil && data.CashFlow != nil {
		allZero := true
		hasDividendField := false
		for _, year := range data.Years {
			if strings.HasSuffix(year, "-12-31") || len(year) == 4 {
				if row, ok := data.CashFlow["分配股利、利润或偿付利息支付的现金"]; ok {
					hasDividendField = true
					if row[year] != 0 {
						allZero = false
						break
					}
				}
			}
		}
		if allZero && hasDividendField {
			if dividendMap, err := downloader.FetchCashFlowDividendFromEastMoney(market, code, len(data.Years)); err == nil && len(dividendMap) > 0 {
				if _, ok := data.CashFlow["分配股利、利润或偿付利息支付的现金"]; !ok {
					data.CashFlow["分配股利、利润或偿付利息支付的现金"] = make(map[string]float64)
				}
				for year, val := range dividendMap {
					data.CashFlow["分配股利、利润或偿付利息支付的现金"][year] = val
					fmt.Printf("[DownloadReports] %s 补充分红数据 %s=%.0f\n", symbol, year, val)
				}
			}
		}
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
		SourceName: sourceName,
		Years:      data.Years,
	})

	result := &DownloadResult{
		Success:    true,
		Message:    fmt.Sprintf("成功下载 %d 年的年报数据", len(data.Years)),
		Years:      data.Years,
		Validation: validation,
		SourceName: sourceName,
	}
	if quality != nil {
		result.QualityScore = quality.Score
	}
	if sourceSuggestion != "" {
		result.SourceSuggestion = sourceSuggestion
	} else if quality != nil && quality.SuggestAlternativeSource(0.05) {
		// 质量仍不合格但无更优备用源时，提示用户手动处理
		result.SourceSuggestion = fmt.Sprintf("当前数据源(%s)资产负债表平衡性欠佳（最大差异%.1f%%），建议通过「设置-数据」切换数据源或手动导入CSV财报",
			sourceName, quality.MaxDiffRatio*100)
	}
	return result, nil
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
		var data *downloader.FinancialReportData
		// 优先尝试 DataRouter
		if a.dataRouter != nil && market != "HK" {
			tfd, tErr := a.dataRouter.FetchFinancialData(market, code)
			if tErr == nil && tfd != nil {
				data = a.dataRouter.ConvertToFinancialReportData(tfd, comp)
				if data == nil || len(data.Years) == 0 {
					data = nil
				}
			}
		}
		// Fallback
		if data == nil {
			var dErr error
			data, dErr = downloader.DownloadFinancialReports(market, code)
			if dErr != nil {
				fmt.Printf("[Comparable] download failed for %s: %v\n", comp, dErr)
				failed = append(failed, comp)
				continue
			}
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
	// 先加载原有快照，保留 RiskAlert/StepResults 等字段，只更新 MarkdownContent
	existingSnapshot, _ := a.storage.LoadSnapshot(symbol)
	if existingSnapshot != nil {
		existingSnapshot.MarkdownContent = updatedContent
		_ = a.storage.SaveSnapshot(symbol, existingSnapshot)
	} else {
		_ = a.storage.SaveSnapshot(symbol, &analyzer.AnalysisReport{
			Symbol:          symbol,
			MarkdownContent: updatedContent,
			Years:           finData.Years,
		})
	}

	// 9. 更新缓存哈希
	compHash, _ := a.storage.ComputeComparablesHash(symbol)
	_ = a.storage.SaveAnalysisCache(symbol, "", compHash)

	// 10. 返回完整报告对象（保留 RiskAlert 等字段供前端使用）
	if existingSnapshot != nil {
		existingSnapshot.MarkdownContent = updatedContent
		return existingSnapshot, nil
	}
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
	// 旧缓存为250条，现需375条（1.5年）支撑全屏250条左侧指标准确性
	if cached, err := a.storage.LoadStockKlines(symbol); err == nil && len(cached) >= 300 {
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
	var klist []downloader.KlineData
	var err error
	if a.dataRouter != nil {
		klist, err = a.dataRouter.FetchKlines(market, code, 375)
	} else {
		klist, err = downloader.FetchStockKlines(market, code, 375)
	}
	if err != nil {
		debugLog("[GetStockKlines] %s FetchStockKlines error: %v", symbol, err)
		return nil, err
	}
	if len(klist) > 0 {
		last := klist[len(klist)-1]
		debugLog("[GetStockKlines] %s fetched %d klines, last={Time:%s Open:%.2f Close:%.2f High:%.2f Low:%.2f}", symbol, len(klist), last.Time, last.Open, last.Close, last.High, last.Low)
	}
	return klist, nil
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
			// 校验缓存数据是否合理（过滤掉错误解析的巨大盘百分比或时间戳，同时校验流通市值）
			if cached.CurrentPrice > 0 && cached.ChangePercent > -50 && cached.ChangePercent < 50 && cached.CirculatingMarketCap > 0 {
				a.fillShareholderReturnRate(symbol, cached)
				return cached, nil
			}
			debugLog("[GetStockQuote] %s cache invalid (price=%.2f change=%.2f cap=%.0f), refetching", symbol, cached.CurrentPrice, cached.ChangePercent, cached.CirculatingMarketCap)
		}
	}

	// 拆分 symbol
	parts := strings.Split(symbol, ".")
	if len(parts) != 2 {
		return nil, fmt.Errorf("无效的股票代码格式: %s", symbol)
	}
	code := parts[0]
	market := strings.ToUpper(parts[1])

	// 从网络获取（通过数据源路由）
	var quote *downloader.StockQuote
	if a.dataRouter != nil {
		quote, err = a.dataRouter.FetchQuote(market, code)
	} else {
		quote, err = downloader.FetchStockQuote(market, code)
	}
	if err != nil {
		return nil, fmt.Errorf("获取行情失败: %w", err)
	}

	// 如果启用了数据源每日指标，用高质量数据补充/覆盖 PE/PB/市值/换手率等
	if a.dataRouter != nil && a.dataRouter.IsUseForQuote() {
		if metrics, mErr := a.dataRouter.FetchDailyMetrics(market, code, ""); mErr == nil && metrics != nil {
			if metrics.PE > 0 {
				quote.PE = metrics.PE
			}
			if metrics.PB > 0 {
				quote.PB = metrics.PB
			}
			if metrics.TurnoverRate > 0 {
				quote.TurnoverRate = metrics.TurnoverRate
			}
			if metrics.VolumeRatio > 0 {
				quote.VolumeRatio = metrics.VolumeRatio
			}
			if metrics.CirculatingMarketCap > 0 {
				quote.CirculatingMarketCap = metrics.CirculatingMarketCap
			}
			if metrics.MarketCap > 0 {
				quote.MarketCap = metrics.MarketCap
			}
			if metrics.DividendYield > 0 {
				quote.DividendYield = metrics.DividendYield
			}
			debugLog("[GetStockQuote] %s merged StockFinLens daily metrics (PE=%.2f PB=%.2f)", symbol, quote.PE, quote.PB)
		}
	}

	debugLog("[GetStockQuote] %s quote={CurrentPrice:%.2f CirculatingMarketCap:%.0f MarketCap:%.0f}", symbol, quote.CurrentPrice, quote.CirculatingMarketCap, quote.MarketCap)
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
	DataMissing        bool   `json:"dataMissing"`
}

// CheckAnalysisCache 检查分析缓存状态
func (a *App) CheckAnalysisCache(symbol string) (*CacheStatus, error) {
	debugLog("[CheckAnalysisCache] Checking cache for %s", symbol)
	if a.storage == nil {
		debugLog("[CheckAnalysisCache] Error: storage not initialized")
		return nil, fmt.Errorf("存储未初始化")
	}

	// 检查核心财报数据是否存在
	stockDir := filepath.Join(a.storage.DataDir(), "data", symbol)
	requiredFiles := []string{"balance_sheet.json", "income_statement.json", "cash_flow.json"}
	dataMissing := false
	for _, name := range requiredFiles {
		if _, err := os.Stat(filepath.Join(stockDir, name)); os.IsNotExist(err) {
			dataMissing = true
			break
		}
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
		Unchanged:          !dataChanged && !comparablesChanged && !dataMissing,
		LastAnalysisAt:     lastAnalysisAt,
		DataChanged:        dataChanged,
		ComparablesChanged: comparablesChanged,
		DataMissing:        dataMissing,
	}
	debugLog("[CheckAnalysisCache] Result for %s: unchanged=%v, dataMissing=%v", symbol, result.Unchanged, result.DataMissing)
	return result, nil
}

// AnalyzeStock 对指定股票执行财报透视分析
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
	// 按股票加锁，防止同一股票被并发分析
	a.analysisMu.Lock()
	if a.analysisLocks == nil {
		a.analysisLocks = make(map[string]*sync.Mutex)
	}
	mu, ok := a.analysisLocks[symbol]
	if !ok {
		mu = &sync.Mutex{}
		a.analysisLocks[symbol] = mu
	}
	a.analysisMu.Unlock()
	mu.Lock()
	defer mu.Unlock()

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

	// 并发获取网络数据：实时行情、K线、舆情情绪、资金流向
	var quoteData *analyzer.QuoteData
	var klines []downloader.KlineData
	var sentimentData *analyzer.SentimentData
	var moneyflowData *analyzer.MoneyflowData
	var wgNet sync.WaitGroup
	wgNet.Add(4)

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
			market := strings.ToUpper(parts[1])
			code := parts[0]
			var list []downloader.KlineData
			var err error
			if a.dataRouter != nil {
				list, err = a.dataRouter.FetchKlines(market, code, 375)
			} else {
				list, err = downloader.FetchStockKlines(market, code, 375)
			}
			if err == nil && len(list) >= 20 {
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

		// 4. 获取资金流向
		go func() {
			defer func() {
				if r := recover(); r != nil {
					debugLog("[PANIC] moneyflow goroutine: %v", r)
				}
				wgNet.Done()
			}()
			if a.dataRouter != nil && a.dataRouter.IsUseForMoneyflow() {
				parts := strings.Split(symbol, ".")
				if len(parts) == 2 {
					market := strings.ToUpper(parts[1])
					code := parts[0]
					end := time.Now().Format("20060102")
					start := time.Now().AddDate(0, 0, -5).Format("20060102")
					items, err := a.dataRouter.FetchMoneyflow(market, code, start, end)
					if err == nil && len(items) > 0 {
						mfItems := make([]analyzer.MoneyflowItem, 0, len(items))
						for _, item := range items {
							mfItems = append(mfItems, analyzer.MoneyflowItem{
								Date:         item.TradeDate,
								MainInflow:   item.BuyLgAmount + item.BuyElgAmount - item.SellLgAmount - item.SellElgAmount,
								SmNetAmount:  item.BuySmAmount - item.SellSmAmount,
								MdNetAmount:  item.BuyMdAmount - item.SellMdAmount,
								LgNetAmount:  item.BuyLgAmount - item.SellLgAmount,
								ElgNetAmount: item.BuyElgAmount - item.SellElgAmount,
							})
						}
						moneyflowData = &analyzer.MoneyflowData{
							HasData: true,
							Items:   mfItems,
						}
						var totalMain float64
						var inflowDays int
						for _, item := range mfItems {
							totalMain += item.MainInflow
							if item.MainInflow > 0 {
								inflowDays++
							}
						}
						dayCount := len(mfItems)
						if inflowDays == dayCount {
							moneyflowData.Summary = fmt.Sprintf("近%d日主力持续流入，累计 %.2f 亿元", dayCount, totalMain/10000)
						} else if inflowDays == 0 {
							moneyflowData.Summary = fmt.Sprintf("近%d日主力持续流出，累计 %.2f 亿元", dayCount, totalMain/10000)
						} else if totalMain > 0 {
							moneyflowData.Summary = fmt.Sprintf("近%d日主力%d日流入，累计净流入 %.2f 亿元", dayCount, inflowDays, totalMain/10000)
						} else {
							moneyflowData.Summary = fmt.Sprintf("近%d日主力%d日流入，累计净流出 %.2f 亿元", dayCount, inflowDays, -totalMain/10000)
						}
					}
				}
			}
		}()
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

	// 补充缺失的分红数据：若现金流量表中分红字段全为0，尝试从东财API获取
	if finData != nil {
		allZero := true
		hasDividendField := false
		for _, year := range finData.Years {
			if strings.HasSuffix(year, "-12-31") || len(year) == 4 {
				v := finData.GetValueOrZero(finData.CashFlow, "分配股利、利润或偿付利息支付的现金", year)
				if v != 0 {
					allZero = false
					break
				}
				hasDividendField = true
			}
		}
		if allZero && hasDividendField {
			parts := strings.Split(symbol, ".")
			if len(parts) == 2 {
				market := strings.ToUpper(parts[1])
				code := parts[0]
				if dividendMap, err := downloader.FetchCashFlowDividendFromEastMoney(market, code, len(finData.Years)); err == nil && len(dividendMap) > 0 {
					for year, val := range dividendMap {
						if _, ok := finData.CashFlow["分配股利、利润或偿付利息支付的现金"]; !ok {
							finData.CashFlow["分配股利、利润或偿付利息支付的现金"] = make(map[string]float64)
						}
						finData.CashFlow["分配股利、利润或偿付利息支付的现金"][year] = val
						debugLog("[AnalyzeStock] %s 补充分红数据 %s=%.0f", symbol, year, val)
					}
				} else {
					debugLog("[AnalyzeStock] %s 尝试从东财补充分红数据失败: %v", symbol, err)
				}
			}
		}
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
			defer func() {
				if r := recover(); r != nil {
					debugLog("[PANIC] ML engines goroutine: %v", r)
				}
				wg.Done()
			}()
			mlLocal := &analyzer.MLPredictionData{}
			// Engine B
			if finSeq := analyzer.BuildMLEngineBInput(finData); len(finSeq) > 0 {
				if fp, err := analyzer.RunMLEngineB(finSeq); err == nil {
					mlLocal.Financial = fp
				} else {
					debugLog("[ML] Engine B failed for %s: %v", symbol, err)
					if mlLocal.MLError == "" {
						mlLocal.MLError = err.Error()
					}
				}
			}
			// Engine D: 风险预警（仅依赖财务数据，独立于 K 线）
			if dFeatures := analyzer.BuildMLEngineDInput(finData); len(dFeatures) > 0 {
				if dp, err := analyzer.RunMLEngineD(dFeatures); err == nil {
					mlLocal.EngineD = dp
				} else {
					debugLog("[ML] Engine D failed for %s: %v", symbol, err)
					if mlLocal.MLError == "" {
						mlLocal.MLError = err.Error()
					}
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
						if mlLocal.MLError == "" {
							mlLocal.MLError = err.Error()
						}
					}
				}
			}
			if mlLocal.Financial != nil || mlLocal.Sentiment != nil || mlLocal.EngineD != nil || mlLocal.MLError != "" {
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
				defer func() {
					if r := recover(); r != nil {
						debugLog("[PANIC] RIM fetch goroutine: %v", r)
					}
					wg.Done()
				}()
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
		defer func() {
			if r := recover(); r != nil {
				debugLog("[PANIC] risk crawler goroutine: %v", r)
			}
			wg.Done()
		}()
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

	// 并发 4: 外部风险数据查询（审计机构、高管变动、诉讼）
	externalRiskData := &analyzer.ExternalRiskData{}
	wg.Add(1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				debugLog("[PANIC] external risk goroutine: %v", r)
			}
			wg.Done()
		}()
		// 审计机构变更
		if ah, err := downloader.FetchAuditorHistory(symbol); err == nil && ah != nil {
			externalRiskData.AuditorChanged = ah.AuditorChanged
			externalRiskData.AuditorName = ah.AuditorName
			externalRiskData.AuditorChangeDetails = make([]analyzer.AuditorChangeDetail, 0, len(ah.ChangeDetails))
			for _, cd := range ah.ChangeDetails {
				externalRiskData.AuditorChangeDetails = append(externalRiskData.AuditorChangeDetails, analyzer.AuditorChangeDetail{
					Date:                 cd.Date,
					OldAuditor:           cd.OldAuditor,
					NewAuditor:           cd.NewAuditor,
					Reason:               cd.Reason,
					IsBeforeAnnualReport: cd.IsBeforeAnnualReport,
					AnnualReportDeadline: cd.AnnualReportDeadline,
					RawTitle:             cd.RawTitle,
				})
			}
		}
		// 高管变动
		if ec, err := downloader.FetchExecChanges(symbol); err == nil && ec != nil {
			// 二次过滤：只保留明确标记为 CFO/审计相关的条目（兼容旧版脚本）
			filteredHistory := make([]string, 0, len(ec.History))
			for _, h := range ec.History {
				if strings.Contains(h, "[CFO]") || strings.Contains(h, "[审计]") {
					filteredHistory = append(filteredHistory, h)
				}
			}
			execCount := len(filteredHistory)
			externalRiskData.ExecChangeCount = execCount
			externalRiskData.ExecHistory = filteredHistory
			externalRiskData.ExecChanged = execCount >= 3
		}
		// 诉讼/担保
		if li, err := downloader.FetchLitigationInfo(symbol); err == nil && li != nil {
			// 二次过滤：排除正常担保额度类公告（兼容旧版脚本）
			filteredHistory := make([]string, 0, len(li.History))
			for _, h := range li.History {
				if strings.Contains(h, "担保额度") || strings.Contains(h, "预计担保") || strings.Contains(h, "为控股子公司提供担保") || strings.Contains(h, "为全资子公司提供担保") || strings.Contains(h, "为子公司提供担保") {
					continue
				}
				filteredHistory = append(filteredHistory, h)
			}
			// 从过滤后的 history 重新判断风险等级
			hasHighRisk := false
			hasFundOccupation := false
			highRiskCount := 0
			hasNormalGuarantee := false
			for _, h := range filteredHistory {
				if strings.Contains(h, "[高风险担保]") || strings.Contains(h, "[诉讼仲裁]") || strings.Contains(h, "[违规处罚]") {
					hasHighRisk = true
					highRiskCount++
				}
				if strings.Contains(h, "资金占用") || strings.Contains(h, "占用资金") {
					hasFundOccupation = true
					highRiskCount++
				}
				if strings.Contains(h, "[担保]") && !strings.Contains(h, "[高风险担保]") {
					hasNormalGuarantee = true
				}
			}
			// 兼容旧版脚本：如果过滤后没有普通担保，但 li.HasGuarantee 为 true，
			// 检查原始 history 中是否真的有非"担保额度"的普通担保
			if !hasNormalGuarantee && li.HasGuarantee {
				for _, h := range li.History {
					if strings.Contains(h, "[担保]") && !strings.Contains(h, "[高风险担保]") &&
						!strings.Contains(h, "担保额度") && !strings.Contains(h, "预计担保") &&
						!strings.Contains(h, "为控股子公司提供担保") && !strings.Contains(h, "为全资子公司提供担保") {
						hasNormalGuarantee = true
						break
					}
				}
			}
			externalRiskData.LitigationCount = highRiskCount
			externalRiskData.LitigationHistory = filteredHistory
			externalRiskData.HasHighRiskGuarantee = hasHighRisk
			externalRiskData.HasFundOccupation = hasFundOccupation
			externalRiskData.HasLitigation = hasHighRisk || hasFundOccupation
			externalRiskData.HasGuarantee = hasNormalGuarantee && !hasHighRisk && !hasFundOccupation
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
		var yearLabels []string
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
			currentYear := time.Now().Year()
			for _, y := range years {
				yearInt, err := strconv.Atoi(y)
				if err != nil || yearInt < currentYear {
					continue // 跳过已披露的历史年份，RIM只预测未来
				}
				if v, ok := extRIM.EPSForecast[y]; ok && v > 0 {
					epsSeq = append(epsSeq, v)
					yearLabels = append(yearLabels, y)
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
				yearLabels = append(yearLabels, fmt.Sprintf("%d", time.Now().Year()))
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
			// 外推年份
			if len(yearLabels) > 0 {
				if lastY, err := strconv.Atoi(yearLabels[len(yearLabels)-1]); err == nil {
					yearLabels = append(yearLabels, fmt.Sprintf("%d", lastY+1))
				} else {
					yearLabels = append(yearLabels, fmt.Sprintf("%d", time.Now().Year()+len(yearLabels)))
				}
			} else {
				yearLabels = append(yearLabels, fmt.Sprintf("%d", time.Now().Year()+len(yearLabels)))
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
				Forecast:     analyzer.RIMForecast{EPS: epsSeq, Years: yearLabels},
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

	// 获取用户设置的风险敏感度
	sensitivity := analyzer.SensitivityStandard
	if s := a.getRiskSensitivity(); s != "" {
		sensitivity = analyzer.SensitivityLevel(s)
	}

	report, err := analyzer.RunAnalysisWithAll(a.storage.DataDir(), symbol, compAnalysis, quoteData, sentimentData, policyData, technicalData, activityData, moneyflowData, mlData, rimData, finData.Extras, externalRiskData, sensitivity)
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
	var dp *downloader.StockProfile
	if a.dataRouter != nil {
		dp, err = a.dataRouter.FetchProfile(market, code)
	} else {
		dp, err = downloader.FetchStockProfile(market, code)
	}
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

	var concepts *downloader.StockConcepts
	if a.dataRouter != nil {
		concepts, err = a.dataRouter.FetchConcepts(market, code, changePercent)
	} else {
		concepts, err = downloader.FetchStockConcepts(market, code, changePercent)
	}
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

// ExportFinancialDataToExcel 将当前股票财务数据及财报分析结果导出为 Excel
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

	// 尝试读取快照获取财报分析结果
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

	// Sheet4: 财报分析汇总
	analysisSheet := "财报分析汇总"
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
		f.SetCellValue(analysisSheet, "A2", "暂无分析数据（请先执行财报分析）")
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
		AppID: "StockFinLens",
		Title: title,
		Body:  content,
	}
	return notification.Push()
}

// GetIndustryDBMeta 获取行业数据库元信息
func (a *App) GetIndustryDBMeta() map[string]interface{} {
	version, updatedAt, count := analyzer.GetIndustryDBMeta()
	return map[string]interface{}{
		"version":   version,
		"updatedAt": updatedAt,
		"count":     count,
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

// ========== 热门概念/风口选票 ==========

// FetchHotConcepts 获取当日热门概念排行（综合排序）
// topN 为返回前 N 个概念，若 <=0 则返回全部
func (a *App) FetchHotConcepts(topN int) (*downloader.HotConceptBoard, error) {
	board, err := downloader.FetchHotConceptBoard(a.storage.DataDir(), topN)
	if err != nil {
		return nil, fmt.Errorf("获取热门概念失败: %w", err)
	}
	return board, nil
}

// FetchHotConceptHistory 获取最近 days 天的历史热点摘要
// 返回每天的 Top 概念名称列表，用于前端"连续上榜"标记
func (a *App) FetchHotConceptHistory(days int) ([]downloader.HotConceptHistoryItem, error) {
	history, err := downloader.FetchHotConceptHistory(a.storage.DataDir(), days)
	if err != nil {
		return nil, fmt.Errorf("获取历史热点失败: %w", err)
	}
	return history, nil
}

// FetchHotConceptConstituents 获取指定概念板块的成分股列表
func (a *App) FetchHotConceptConstituents(conceptCode string) ([]downloader.ConceptConstituent, error) {
	cons, err := downloader.FetchConceptConstituents(conceptCode)
	if err != nil {
		return nil, fmt.Errorf("获取成分股失败: %w", err)
	}
	return cons, nil
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

// ========== 快速分析（热点成分股扫描用）==========

// QuickAnalysis 快速分析结果（精简版，用于热点成分股快速扫描）
type QuickAnalysis struct {
	// 基础信息
	Code   string `json:"code"`
	Name   string `json:"name"`
	Symbol string `json:"symbol"` // 带点格式，如 000001.SZ
	Market string `json:"market"`

	// 流动性（来自 Quote）
	CurrentPrice  float64 `json:"current_price"`
	ChangePercent float64 `json:"change_percent"`
	TurnoverRate  float64 `json:"turnover_rate"`
	VolumeRatio   float64 `json:"volume_ratio"`

	// 资金流向（优先 StockFinLens moneyflow，否则近似估算）
	HasMoneyflowData bool    `json:"has_moneyflow_data"` // 是否有真实资金流向数据
	MainInflow       float64 `json:"main_inflow"`        // 主力净流入（大单+特大单），元
	SmNetAmount      float64 `json:"sm_net_amount"`      // 小单净流入，元
	MdNetAmount      float64 `json:"md_net_amount"`      // 中单净流入，元
	LgNetAmount      float64 `json:"lg_net_amount"`      // 大单净流入，元
	ElgNetAmount     float64 `json:"elg_net_amount"`     // 特大单净流入，元

	// 基本面（来自 Profile）
	Industry  string  `json:"industry"`
	MarketCap float64 `json:"market_cap"`
	PE        float64 `json:"pe"`
	PB        float64 `json:"pb"`
	EPS       float64 `json:"eps"`

	// 舆情（来自 Sentiment）
	SentimentScore    float64  `json:"sentiment_score"`
	SentimentHeat     int      `json:"sentiment_heat"`
	SentimentKeywords []string `json:"sentiment_keywords"`
	HasSentimentData  bool     `json:"has_sentiment_data"`

	// 风口关联（来自 Concepts）
	Concepts     []string `json:"concepts"`
	ConceptMatch []string `json:"concept_match"` // 与当前 Top 20 热点的交集

	// 风险警示
	RiskAlert *analyzer.RiskAlertSummary `json:"riskAlert,omitempty"`

	// 错误信息
	Errors []string `json:"errors"`
}

// QuickAnalyzeStock 对单只股票执行快速分析（并行获取 Quote + Profile + Concepts + Sentiment）
func (a *App) QuickAnalyzeStock(code, name, market, conceptCode string) (*QuickAnalysis, error) {
	if a.storage == nil {
		return nil, fmt.Errorf("存储未初始化")
	}

	// 构造 symbol
	marketUpper := strings.ToUpper(market)
	if marketUpper == "" {
		marketUpper = inferMarketFromCodeQuick(code)
	}
	symbol := code + "." + marketUpper

	result := &QuickAnalysis{
		Code:   code,
		Name:   name,
		Symbol: symbol,
		Market: marketUpper,
		Errors: []string{},
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	// 1. 获取 Quote
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() {
			if r := recover(); r != nil {
				mu.Lock()
				result.Errors = append(result.Errors, fmt.Sprintf("Quote panic: %v", r))
				mu.Unlock()
			}
		}()
		quote, err := a.GetStockQuote(symbol)
		if err != nil || quote == nil {
			mu.Lock()
			result.Errors = append(result.Errors, fmt.Sprintf("获取行情失败: %v", err))
			mu.Unlock()
			return
		}
		mu.Lock()
		result.CurrentPrice = quote.CurrentPrice
		result.ChangePercent = quote.ChangePercent
		result.TurnoverRate = quote.TurnoverRate
		result.VolumeRatio = quote.VolumeRatio
		result.MainInflow = quote.TurnoverAmount * quote.ChangePercent / 100 // 近似主力净流入（若接口无直接字段）
		mu.Unlock()
	}()

	// 2. 获取 Profile
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() {
			if r := recover(); r != nil {
				mu.Lock()
				result.Errors = append(result.Errors, fmt.Sprintf("Profile panic: %v", r))
				mu.Unlock()
			}
		}()
		profile, err := a.GetStockProfile(symbol)
		if err != nil || profile == nil {
			mu.Lock()
			result.Errors = append(result.Errors, fmt.Sprintf("获取资料失败: %v", err))
			mu.Unlock()
			return
		}
		mu.Lock()
		result.Industry = profile.Industry
		result.MarketCap = profile.MarketCap
		result.PE = profile.PE
		result.PB = profile.PB
		result.EPS = profile.EPS
		mu.Unlock()
	}()

	// 3. 获取 Concepts
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() {
			if r := recover(); r != nil {
				mu.Lock()
				result.Errors = append(result.Errors, fmt.Sprintf("Concepts panic: %v", r))
				mu.Unlock()
			}
		}()
		concepts, err := a.GetStockConcepts(symbol)
		if err != nil || concepts == nil {
			mu.Lock()
			result.Errors = append(result.Errors, fmt.Sprintf("获取概念失败: %v", err))
			mu.Unlock()
			return
		}
		mu.Lock()
		result.Concepts = concepts.Concepts
		mu.Unlock()
	}()

	// 4. 获取 Sentiment（直接调用 downloader，非 Wails 绑定）
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() {
			if r := recover(); r != nil {
				mu.Lock()
				result.Errors = append(result.Errors, fmt.Sprintf("Sentiment panic: %v", r))
				mu.Unlock()
			}
		}()
		sentiment, err := downloader.FetchStockSentiment(marketUpper, code)
		if err != nil || sentiment == nil {
			mu.Lock()
			result.HasSentimentData = false
			mu.Unlock()
			return
		}
		mu.Lock()
		result.SentimentScore = sentiment.Score
		result.SentimentHeat = sentiment.HeatIndex
		// 合并正负向关键词
		keywords := make([]string, 0, len(sentiment.PositiveWords)+len(sentiment.NegativeWords))
		keywords = append(keywords, sentiment.PositiveWords...)
		keywords = append(keywords, sentiment.NegativeWords...)
		if len(keywords) > 6 {
			keywords = keywords[:6]
		}
		result.SentimentKeywords = keywords
		result.HasSentimentData = sentiment.HasData
		mu.Unlock()
	}()

	// 5. 获取个股资金流向（优先 StockFinLens moneyflow，替换 Quote 中的近似值）
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() {
			if r := recover(); r != nil {
				mu.Lock()
				result.Errors = append(result.Errors, fmt.Sprintf("Moneyflow panic: %v", r))
				mu.Unlock()
			}
		}()
		if a.dataRouter != nil && a.dataRouter.IsUseForMoneyflow() {
			today := time.Now().Format("20060102")
			items, err := a.dataRouter.FetchMoneyflow(marketUpper, code, today, today)
			if err == nil && len(items) > 0 {
				item := items[0]
				smNet := item.BuySmAmount - item.SellSmAmount
				mdNet := item.BuyMdAmount - item.SellMdAmount
				lgNet := item.BuyLgAmount - item.SellLgAmount
				elgNet := item.BuyElgAmount - item.SellElgAmount
				mainInflow := lgNet + elgNet
				mu.Lock()
				result.HasMoneyflowData = true
				result.MainInflow = mainInflow
				result.SmNetAmount = smNet
				result.MdNetAmount = mdNet
				result.LgNetAmount = lgNet
				result.ElgNetAmount = elgNet
				mu.Unlock()
				return
			}
		}
		// 未启用 StockFinLens 或获取失败：保留 Quote goroutine 中的近似值
	}()

	// 6. 获取非财务风险数据（质押、问询、减持）
	var quickExtras map[string]float64
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() {
			if r := recover(); r != nil {
				mu.Lock()
				result.Errors = append(result.Errors, fmt.Sprintf("RiskCrawler panic: %v", r))
				mu.Unlock()
			}
		}()
		if rc, err := downloader.FetchRiskCrawlerData(symbol); err == nil {
			mu.Lock()
			quickExtras = make(map[string]float64)
			if rc.PledgeRatio != nil {
				quickExtras["pledgeRatio"] = *rc.PledgeRatio
			}
			if rc.InquiryCount1Y != nil {
				quickExtras["inquiryCount"] = float64(*rc.InquiryCount1Y)
			}
			if rc.ReductionCount1Y != nil {
				quickExtras["reductionCount"] = float64(*rc.ReductionCount1Y)
			}
			mu.Unlock()
		}
	}()

	wg.Wait()

	// 构建快速分析风险摘要
	// 优先使用本地缓存的历史分析 RiskAlert
	if cached, err := a.storage.LoadSnapshot(symbol); err == nil && cached != nil && cached.RiskAlert != nil {
		result.RiskAlert = cached.RiskAlert
	} else {
		// 无历史分析时，基于爬虫数据构建简化版风险摘要
		result.RiskAlert = buildQuickRiskAlert(quickExtras)
	}

	// 计算 concept_match：当前股票概念与传入的热点概念名称的交集
	if conceptCode != "" {
		// 查找热点概念名称
		var hotConceptName string
		if board, err := a.FetchHotConcepts(20); err == nil && board != nil {
			for _, c := range board.Concepts {
				if c.Code == conceptCode {
					hotConceptName = c.Name
					break
				}
			}
		}
		if hotConceptName != "" {
			for _, concept := range result.Concepts {
				if strings.Contains(concept, hotConceptName) || strings.Contains(hotConceptName, concept) {
					result.ConceptMatch = append(result.ConceptMatch, concept)
				}
			}
		}
	}

	return result, nil
}

// buildQuickRiskAlert 基于爬虫数据构建简化版风险摘要
func buildQuickRiskAlert(extras map[string]float64) *analyzer.RiskAlertSummary {
	if len(extras) == 0 {
		return nil
	}

	flags := []analyzer.RiskAlertFlag{}
	pledgeRatio := extras["pledgeRatio"]
	inquiryCount := extras["inquiryCount"]
	reductionCount := extras["reductionCount"]

	if pledgeRatio > 70 {
		flags = append(flags, analyzer.RiskAlertFlag{
			Code: "pledge_extreme", Name: "大股东高比例质押", Value: pledgeRatio, Level: "high", Source: "crawler",
			Format: fmt.Sprintf("股权质押 %.0f%%", pledgeRatio),
		})
	} else if pledgeRatio > 30 {
		flags = append(flags, analyzer.RiskAlertFlag{
			Code: "pledge_high", Name: "股权质押比例偏高", Value: pledgeRatio, Level: "medium", Source: "crawler",
			Format: fmt.Sprintf("股权质押 %.0f%%", pledgeRatio),
		})
	}

	if inquiryCount >= 3 {
		flags = append(flags, analyzer.RiskAlertFlag{
			Code: "inquiry_extreme", Name: "一年内多次监管问询", Value: inquiryCount, Level: "high", Source: "crawler",
			Format: fmt.Sprintf("近1年被监管问询 %.0f 次", inquiryCount),
		})
	}

	if reductionCount >= 1 {
		flags = append(flags, analyzer.RiskAlertFlag{
			Code: "reduction", Name: "大股东减持", Value: reductionCount, Level: "medium", Source: "crawler",
			Format: fmt.Sprintf("近1年减持公告 %.0f 次", reductionCount),
		})
	}

	if len(flags) == 0 {
		return &analyzer.RiskAlertSummary{Level: "low", PrimaryMsg: "🟢 未发现重大风险信号"}
	}

	highCount := 0
	mediumCount := 0
	for _, f := range flags {
		if f.Level == "high" {
			highCount++
		} else {
			mediumCount++
		}
	}

	level := "medium"
	msg := fmt.Sprintf("🟡 该股票存在 %d 项中风险信号，需保持关注", mediumCount)
	if highCount > 0 {
		level = "high"
		msg = fmt.Sprintf("🔴 该股票存在 %d 项高风险信号，建议审慎评估", highCount+mediumCount)
	}

	return &analyzer.RiskAlertSummary{
		Level:      level,
		Flags:      flags,
		PrimaryMsg: msg,
	}
}

// inferMarketFromCodeQuick 通过 A 股代码前缀快速推断市场
func inferMarketFromCodeQuick(code string) string {
	if len(code) == 0 {
		return "SH"
	}
	prefix := code[0]
	if prefix == '6' || prefix == '5' {
		return "SH"
	}
	if prefix == '0' || prefix == '3' {
		return "SZ"
	}
	if prefix == '8' || prefix == '4' {
		return "BJ"
	}
	return "SH"
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

// getRiskSensitivity 获取用户设置的风险警示敏感度
func (a *App) getRiskSensitivity() string {
	if a.riskSensitivity == "" {
		return string(analyzer.SensitivityStandard)
	}
	return a.riskSensitivity
}

// GetRiskSensitivity 获取风险警示敏感度（Wails 绑定）
func (a *App) GetRiskSensitivity() string {
	return a.getRiskSensitivity()
}

// SetRiskSensitivity 设置风险警示敏感度（Wails 绑定）
func (a *App) SetRiskSensitivity(sensitivity string) error {
	a.riskSensitivity = sensitivity
	debugLog("[Settings] risk sensitivity set to %s", sensitivity)
	return nil
}

// decodeToken 对授权码进行 base64 解码（失败则返回原值，兼容老用户）
func decodeToken(token string) string {
	if token == "" {
		return ""
	}
	decoded, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return token
	}
	return string(decoded)
}

// ========== StockFinLens SFL 配置 Wails 绑定 ==========

// GetSFLConfig 获取 SFL 配置
func (a *App) GetSFLConfig() (*SFLConfig, error) {
	if a.storage == nil {
		return nil, fmt.Errorf("存储未初始化")
	}
	return a.storage.LoadSFLConfig()
}

// SaveSFLConfig 保存 SFL 配置
func (a *App) SaveSFLConfig(cfg SFLConfig) error {
	if a.storage == nil {
		return fmt.Errorf("存储未初始化")
	}
	if err := a.storage.SaveSFLConfig(&cfg); err != nil {
		return err
	}
	// 配置变更后重新加载数据源路由
	a.reloadDataRouter()
	return nil
}

// reloadDataRouter 根据当前配置重新加载数据源路由
func (a *App) reloadDataRouter() {
	if a.storage == nil {
		return
	}
	cfg, err := a.storage.LoadSFLConfig()
	if err != nil {
		fmt.Printf("[DataRouter] 加载SFL 配置失败: %v\n", err)
		a.dataRouter = downloader.NewDataRouter("", false, false, false, false, false)
		return
	}
	realToken := decodeToken(cfg.Token)
	fmt.Printf("[DataRouter] StockFinLens enabled=%v token=%v financial=%v kline=%v quote=%v moneyflow=%v\n",
		cfg.Enabled, realToken != "", cfg.UseForFinancial, cfg.UseForKline, cfg.UseForQuote, cfg.UseForMoneyflow)
	a.dataRouter = downloader.NewDataRouter(realToken, cfg.Enabled,
		cfg.UseForFinancial, cfg.UseForKline, cfg.UseForQuote, cfg.UseForMoneyflow)

	// 设置热点概念降级用的数据源客户端
	if cfg.Enabled && realToken != "" {
		downloader.SetSFLHotConceptClient(downloader.NewSFLClient(realToken))
	} else {
		downloader.SetSFLHotConceptClient(nil)
	}
}

// SFLVerifyResult 授权码验证结果
type SFLVerifyResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// VerifySFLToken 验证授权码是否有效
func (a *App) VerifySFLToken(token string) (*SFLVerifyResult, error) {
	if token == "" {
		return &SFLVerifyResult{Success: false, Message: "授权码不能为空"}, nil
	}
	realToken := decodeToken(token)
	client := downloader.NewSFLClient(realToken)
	if err := client.VerifyToken(); err != nil {
		return &SFLVerifyResult{Success: false, Message: fmt.Sprintf("验证失败: %v", err)}, nil
	}
	return &SFLVerifyResult{Success: true, Message: "验证通过，授权码有效"}, nil
}

// ========== 个股资金流向 ==========

// StockMoneyflowItem 单日资金流向数据
type StockMoneyflowItem struct {
	Date         string  `json:"date"`
	MainInflow   float64 `json:"main_inflow"`    // 主力净流入（大单+特大单）
	SmNetAmount  float64 `json:"sm_net_amount"`  // 小单净流入
	MdNetAmount  float64 `json:"md_net_amount"`  // 中单净流入
	LgNetAmount  float64 `json:"lg_net_amount"`  // 大单净流入
	ElgNetAmount float64 `json:"elg_net_amount"` // 特大单净流入
}

// StockMoneyflowResult 个股资金流向查询结果
type StockMoneyflowResult struct {
	Symbol    string               `json:"symbol"`
	Items     []StockMoneyflowItem `json:"items"`
	HasData   bool                 `json:"has_data"`
	Summary   string               `json:"summary"`    // 简要分析
	Days      int                  `json:"days"`       // 查询的交易日数
	TodayItem *StockMoneyflowItem  `json:"today_item"` // 当日实时数据（可能为nil）
}

// retailSummary 生成散户（小单）流向文案
// 当主力流出、散户流入时，用"散户接盘"表述，更直观
func retailSummary(totalRetail float64) string {
	if totalRetail > 0 {
		return fmt.Sprintf("散户接盘 %.2f 亿元", totalRetail/10000)
	}
	if totalRetail < 0 {
		return fmt.Sprintf("散户净流出 %.2f 亿元", -totalRetail/10000)
	}
	return "散户无进出"
}

// GetStockMoneyflow 获取个股近 N 日资金流向（优先数据源）
// 逻辑：查询范围扩大到 (days+1)*3 天，从中识别当日数据并分离，历史数据固定保留 days 条
func (a *App) GetStockMoneyflow(symbol string, days int) (*StockMoneyflowResult, error) {
	if a.dataRouter == nil || !a.dataRouter.IsUseForMoneyflow() {
		return &StockMoneyflowResult{Symbol: symbol, HasData: false, Summary: "StockFinLens 资金流向未启用"}, nil
	}

	parts := strings.Split(symbol, ".")
	if len(parts) != 2 {
		return nil, fmt.Errorf("无效的股票代码格式: %s", symbol)
	}
	code := parts[0]
	market := strings.ToUpper(parts[1])

	// 扩大查询范围：确保即使当日有数据，也能补足 days 条历史数据
	end := time.Now().Format("20060102")
	start := time.Now().AddDate(0, 0, -(days+1)*3).Format("20060102")

	items, err := a.dataRouter.FetchMoneyflow(market, code, start, end)
	if err != nil {
		fmt.Printf("[GetStockMoneyflow] %s %s-%s error: %v\n", symbol, start, end, err)
		return &StockMoneyflowResult{Symbol: symbol, HasData: false, Summary: fmt.Sprintf("资金流向获取失败: %v", err)}, nil
	}
	if len(items) == 0 {
		fmt.Printf("[GetStockMoneyflow] %s %s-%s empty result\n", symbol, start, end)
		return &StockMoneyflowResult{Symbol: symbol, HasData: false, Summary: "暂无资金流向数据（API返回空）"}, nil
	}

	today := time.Now().Format("20060102")
	result := &StockMoneyflowResult{
		Symbol:  symbol,
		HasData: true,
		Items:   make([]StockMoneyflowItem, 0, days),
	}

	var totalMain float64
	var totalRetail float64
	var inflowDays int
	var todayItem *StockMoneyflowItem

	for _, item := range items {
		smNet := item.BuySmAmount - item.SellSmAmount
		mdNet := item.BuyMdAmount - item.SellMdAmount
		lgNet := item.BuyLgAmount - item.SellLgAmount
		elgNet := item.BuyElgAmount - item.SellElgAmount
		mainInflow := lgNet + elgNet

		mfItem := StockMoneyflowItem{
			Date:         item.TradeDate,
			MainInflow:   mainInflow,
			SmNetAmount:  smNet,
			MdNetAmount:  mdNet,
			LgNetAmount:  lgNet,
			ElgNetAmount: elgNet,
		}

		// 当日数据（按日期匹配）分离出来，不加入历史 Items
		if item.TradeDate == today && todayItem == nil {
			todayItem = &mfItem
			continue
		}

		// 只收集 days 条历史数据
		if len(result.Items) < days {
			result.Items = append(result.Items, mfItem)
			totalMain += mainInflow
			totalRetail += smNet
			if mainInflow > 0 {
				inflowDays++
			}
		}
	}

	result.TodayItem = todayItem

	// 生成简要分析（仅基于历史数据）
	dayCount := len(result.Items)
	retailText := retailSummary(totalRetail)
	if dayCount == 0 {
		result.Summary = ""
	} else if inflowDays == dayCount {
		result.Summary = fmt.Sprintf("主力持续流入 %.2f 亿元；%s", totalMain/10000, retailText)
	} else if inflowDays == 0 {
		result.Summary = fmt.Sprintf("主力持续流出 %.2f 亿元；%s", -totalMain/10000, retailText)
	} else if totalMain > 0 {
		result.Summary = fmt.Sprintf("主力%d日流入，累计净流入 %.2f 亿元；%s", inflowDays, totalMain/10000, retailText)
	} else {
		result.Summary = fmt.Sprintf("主力%d日流入，累计净流出 %.2f 亿元；%s", inflowDays, -totalMain/10000, retailText)
	}
	result.Days = days

	return result, nil
}


// ========== 自动更新 Wails 绑定 ==========

// CheckForUpdate 手动检查更新（Wails 绑定）
func (a *App) CheckForUpdate() (*updater.UpdateInfo, error) {
	if a.currentVersion == "" || a.currentVersion == "unknown" {
		a.currentVersion = readWailsVersion()
	}
	info, err := updater.CheckUpdate(a.currentVersion)
	if err != nil {
		return nil, err
	}
	return info, nil
}

// DownloadUpdate 下载更新包（Wails 绑定）
// progressFn 通过 Wails Event 推送进度
func (a *App) DownloadUpdate(assetURL, tag string) (string, error) {
	updateDir := filepath.Join(a.storage.DataDir(), "update")
	return updater.DownloadUpdate(assetURL, tag, updateDir, func(percent int) {
		runtime.EventsEmit(a.ctx, "update:progress", percent)
	})
}

// ApplyUpdate 应用已下载的更新包（Wails 绑定）
func (a *App) ApplyUpdate(localPath string) error {
	return updater.ApplyUpdate(localPath)
}

// SetAutoCheckUpdate 设置是否启动时自动检查更新（Wails 绑定）
func (a *App) SetAutoCheckUpdate(enabled bool) error {
	if a.appConfig == nil {
		a.appConfig = &AppConfig{}
	}
	a.appConfig.AutoCheckUpdate = enabled
	return a.storage.SaveAppConfig(a.appConfig)
}

// GetAutoCheckUpdate 获取是否启动时自动检查更新（Wails 绑定）
func (a *App) GetAutoCheckUpdate() bool {
	if a.appConfig == nil {
		return true
	}
	return a.appConfig.AutoCheckUpdate
}

// SkipVersion 跳过指定版本（不再提示）（Wails 绑定）
func (a *App) SkipVersion(version string) error {
	if a.appConfig == nil {
		a.appConfig = &AppConfig{}
	}
	a.appConfig.SkipVersion = version
	return a.storage.SaveAppConfig(a.appConfig)
}

// GetCurrentVersion 获取当前版本号（Wails 绑定）
func (a *App) GetCurrentVersion() string {
	if a.currentVersion == "" || a.currentVersion == "unknown" {
		a.currentVersion = readWailsVersion()
	}
	return a.currentVersion
}
