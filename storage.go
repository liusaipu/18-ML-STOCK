package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/liusaipu/stockfinlens/analyzer"
	"github.com/liusaipu/stockfinlens/downloader"
)

// Storage 本地文件存储管理器
type Storage struct {
	dataDir string
}

// NewStorage 创建存储管理器，目录位于 ~/.config/stock-analyzer
func NewStorage() (*Storage, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("获取用户目录失败: %w", err)
	}
	dataDir := filepath.Join(home, ".config", "stock-analyzer")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}
	return &Storage{dataDir: dataDir}, nil
}

// DataDir 返回数据根目录
func (s *Storage) DataDir() string {
	return s.dataDir
}

// WatchlistPath 返回自选列表文件路径
func (s *Storage) WatchlistPath() string {
	return filepath.Join(s.dataDir, "watchlist.json")
}

// LoadWatchlist 加载自选列表
func (s *Storage) LoadWatchlist() ([]WatchlistItem, error) {
	path := s.WatchlistPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []WatchlistItem{}, nil
		}
		return nil, err
	}
	var list []WatchlistItem
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, err
	}
	return list, nil
}

// SaveWatchlist 保存自选列表
func (s *Storage) SaveWatchlist(list []WatchlistItem) error {
	path := s.WatchlistPath()
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// EnsureStockDataDir 确保某只股票的数据目录存在
func (s *Storage) EnsureStockDataDir(symbol string) (string, error) {
	dir := filepath.Join(s.dataDir, "data", symbol)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

// EnsureReportDir 确保某只股票的报告目录存在
func (s *Storage) EnsureReportDir(symbol string) (string, error) {
	dir := filepath.Join(s.dataDir, "reports", symbol)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

// CleanStockData 删除某只股票的所有本地数据（财报、报告、缓存、可比公司等）
func (s *Storage) CleanStockData(symbol string) error {
	var errs []string
	// 删除财报数据目录
	if err := os.RemoveAll(filepath.Join(s.dataDir, "data", symbol)); err != nil {
		errs = append(errs, fmt.Sprintf("data: %v", err))
	}
	// 删除报告目录
	if err := os.RemoveAll(filepath.Join(s.dataDir, "reports", symbol)); err != nil {
		errs = append(errs, fmt.Sprintf("reports: %v", err))
	}
	// 删除可比公司缓存目录
	if err := os.RemoveAll(filepath.Join(s.dataDir, "comparables", symbol)); err != nil {
		errs = append(errs, fmt.Sprintf("comparables: %v", err))
	}
	if len(errs) > 0 {
		return fmt.Errorf("清理数据失败: %s", strings.Join(errs, "; "))
	}
	return nil
}

// HistoryMeta 历史数据批次元信息
type HistoryMeta struct {
	Timestamp  string   `json:"timestamp"`
	Source     string   `json:"source"`
	SourceName string   `json:"sourceName"`
	Years      []string `json:"years"`
}

// StockProfile 股票基本信息
type StockProfile struct {
	Industry             string  `json:"industry"`
	ListingDate          string  `json:"listingDate"`
	TotalShares          float64 `json:"totalShares"`
	MarketCap            float64 `json:"marketCap"`
	PE                   float64 `json:"pe"`
	PB                   float64 `json:"pb"`
	EPS                  float64 `json:"eps"`
	Chairman             string  `json:"chairman"`
	Controller           string  `json:"controller"`
	ChairmanGender       string  `json:"chairmanGender"`
	ChairmanAge          string  `json:"chairmanAge"`
	ChairmanNationality  string  `json:"chairmanNationality"`
	ChairmanHoldRatio    string  `json:"chairmanHoldRatio"`
	PoliticalAffiliation string  `json:"politicalAffiliation"`
	UpdatedAt            string  `json:"updatedAt"`
}

// ArchiveStockData 将当前股票数据归档为历史版本，并只保留最近3批
func (s *Storage) ArchiveStockData(symbol string, meta HistoryMeta) error {
	stockDir := filepath.Join(s.dataDir, "data", symbol)
	historyDir := filepath.Join(stockDir, "history", meta.Timestamp)
	if err := os.MkdirAll(historyDir, 0755); err != nil {
		return fmt.Errorf("创建历史目录失败: %w", err)
	}

	files := []string{"balance_sheet.json", "income_statement.json", "cash_flow.json"}
	for _, name := range files {
		src := filepath.Join(stockDir, name)
		dst := filepath.Join(historyDir, name)
		if err := copyFile(src, dst); err != nil {
			// 如果源文件不存在则跳过（可能只导入了两张表）
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("复制 %s 失败: %w", name, err)
		}
	}

	metaPath := filepath.Join(historyDir, "meta.json")
	metaBytes, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(metaPath, metaBytes, 0644); err != nil {
		return err
	}

	return s.cleanupOldHistory(symbol)
}

// ListStockDataHistory 列出某只股票的历史数据批次
func (s *Storage) ListStockDataHistory(symbol string) ([]HistoryMeta, error) {
	historyRoot := filepath.Join(s.dataDir, "data", symbol, "history")
	entries, err := os.ReadDir(historyRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var metas []HistoryMeta
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		metaPath := filepath.Join(historyRoot, entry.Name(), "meta.json")
		data, err := os.ReadFile(metaPath)
		if err != nil {
			continue
		}
		var meta HistoryMeta
		if err := json.Unmarshal(data, &meta); err != nil {
			continue
		}
		metas = append(metas, meta)
	}
	// 按时间戳从新到旧排序
	sort.Slice(metas, func(i, j int) bool {
		return metas[i].Timestamp > metas[j].Timestamp
	})
	return metas, nil
}

// cleanupOldHistory 清理旧历史数据，只保留最近3批
func (s *Storage) cleanupOldHistory(symbol string) error {
	historyRoot := filepath.Join(s.dataDir, "data", symbol, "history")
	entries, err := os.ReadDir(historyRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var dirs []string
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, entry.Name())
		}
	}
	if len(dirs) <= 3 {
		return nil
	}

	// 按目录名（即时间戳字符串）排序，旧的在后面
	sort.Strings(dirs)
	// 删除最旧的目录，直到只剩3个
	for i := 0; i < len(dirs)-3; i++ {
		target := filepath.Join(historyRoot, dirs[i])
		if err := os.RemoveAll(target); err != nil {
			return err
		}
	}
	return nil
}

// SaveReport 将 Markdown 报告保存到 reports/{symbol}/latest.md
func (s *Storage) SaveReport(symbol string, content string, overwriteLatest bool) (string, error) {
	dir := filepath.Join(s.dataDir, "reports", symbol)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("创建报告目录失败: %w", err)
	}
	// 清理所有旧报告文件，只保留 latest.md
	_ = s.cleanupOldReports(symbol)
	filename := "latest.md"
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("保存报告失败: %w", err)
	}
	return filename, nil
}

// ListReports 列出某只股票的历史报告文件名（始终只返回 latest.md）
func (s *Storage) ListReports(symbol string) ([]string, error) {
	dir := filepath.Join(s.dataDir, "reports", symbol)
	path := filepath.Join(dir, "latest.md")
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return []string{"latest.md"}, nil
}

// LoadReport 读取指定历史报告的 Markdown 内容
func (s *Storage) LoadReport(symbol, filename string) (string, error) {
	path := filepath.Join(s.dataDir, "reports", symbol, filename)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// DeleteReport 删除指定历史报告
func (s *Storage) DeleteReport(symbol, filename string) error {
	path := filepath.Join(s.dataDir, "reports", symbol, filename)
	if err := os.Remove(path); err != nil {
		return err
	}
	return nil
}

// AnalysisCache 分析缓存元数据
type AnalysisCache struct {
	DataHash       string `json:"dataHash"`
	ComparablesHash string `json:"comparablesHash"`
	LastAnalysisAt string `json:"lastAnalysisAt"`
}

// SaveAnalysisCache 保存分析缓存
func (s *Storage) SaveAnalysisCache(symbol, dataHash, comparablesHash string) error {
	path := filepath.Join(s.dataDir, "data", symbol, "analysis_cache.json")
	cache := AnalysisCache{
		DataHash:        dataHash,
		ComparablesHash: comparablesHash,
		LastAnalysisAt:  time.Now().Format("2006-01-02 15:04:05"),
	}
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// LoadAnalysisCache 读取分析缓存
func (s *Storage) LoadAnalysisCache(symbol string) (*AnalysisCache, error) {
	path := filepath.Join(s.dataDir, "data", symbol, "analysis_cache.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var cache AnalysisCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, err
	}
	return &cache, nil
}

// ComputeDataHash 计算股票原始数据的哈希（用于判断是否需要重新分析）
func (s *Storage) ComputeDataHash(symbol string) (string, error) {
	stockDir := filepath.Join(s.dataDir, "data", symbol)
	files := []string{"balance_sheet.json", "income_statement.json", "cash_flow.json", "profile.json", "quote.json", "sentiment.json"}
	h := sha256.New()
	for _, name := range files {
		path := filepath.Join(stockDir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return "", err
		}
		h.Write([]byte(name))
		h.Write(data)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// ComputeComparablesHash 计算可比公司列表的哈希
func (s *Storage) ComputeComparablesHash(symbol string) (string, error) {
	comps, err := s.GetComparables(symbol)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	h.Write([]byte(symbol))
	for _, c := range comps {
		h.Write([]byte(c))
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// cleanupOldReports 清理旧报告文件，保留 latest.md
func (s *Storage) cleanupOldReports(symbol string) error {
	dir := filepath.Join(s.dataDir, "reports", symbol)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".md") && name != "latest.md" {
			if err := os.Remove(filepath.Join(dir, name)); err != nil {
				return err
			}
		}
	}
	return nil
}

// SaveStockProfile 保存股票基本资料缓存
func (s *Storage) SaveStockProfile(symbol string, profile *StockProfile) error {
	dir := filepath.Join(s.dataDir, "data", symbol)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	path := filepath.Join(dir, "profile.json")
	data, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// LoadStockProfile 读取股票基本资料缓存
func (s *Storage) LoadStockProfile(symbol string) (*StockProfile, error) {
	path := filepath.Join(s.dataDir, "data", symbol, "profile.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var profile StockProfile
	if err := json.Unmarshal(data, &profile); err != nil {
		return nil, err
	}
	return &profile, nil
}

// SaveStockConcepts 保存股票概念与风口缓存
func (s *Storage) SaveStockConcepts(symbol string, concepts *downloader.StockConcepts) error {
	dir := filepath.Join(s.dataDir, "data", symbol)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	path := filepath.Join(dir, "concepts.json")
	data, err := json.MarshalIndent(concepts, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// LoadStockConcepts 读取股票概念与风口缓存
func (s *Storage) LoadStockConcepts(symbol string) (*downloader.StockConcepts, error) {
	path := filepath.Join(s.dataDir, "data", symbol, "concepts.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var concepts downloader.StockConcepts
	if err := json.Unmarshal(data, &concepts); err != nil {
		return nil, err
	}
	return &concepts, nil
}

// SaveStockQuote 保存股票实时行情缓存
func (s *Storage) SaveStockQuote(symbol string, quote *downloader.StockQuote) error {
	dir := filepath.Join(s.dataDir, "data", symbol)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	path := filepath.Join(dir, "quote.json")
	data, err := json.MarshalIndent(quote, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// LoadStockQuote 读取股票实时行情缓存
func (s *Storage) LoadStockQuote(symbol string) (*downloader.StockQuote, error) {
	path := filepath.Join(s.dataDir, "data", symbol, "quote.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var quote downloader.StockQuote
	if err := json.Unmarshal(data, &quote); err != nil {
		return nil, err
	}
	return &quote, nil
}

// SaveStockKlines 保存股票K线数据缓存
func (s *Storage) SaveStockKlines(symbol string, klines []downloader.KlineData) error {
	dir := filepath.Join(s.dataDir, "data", symbol)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	path := filepath.Join(dir, "klines.json")
	data, err := json.MarshalIndent(klines, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// LoadStockKlines 读取股票K线数据缓存
func (s *Storage) LoadStockKlines(symbol string) ([]downloader.KlineData, error) {
	path := filepath.Join(s.dataDir, "data", symbol, "klines.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var klines []downloader.KlineData
	if err := json.Unmarshal(data, &klines); err != nil {
		return nil, err
	}
	return klines, nil
}

// SaveStockSentiment 保存舆情情绪缓存
func (s *Storage) SaveStockSentiment(symbol string, sentiment *downloader.SentimentData) error {
	dir := filepath.Join(s.dataDir, "data", symbol)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	path := filepath.Join(dir, "sentiment.json")
	data, err := json.MarshalIndent(sentiment, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// LoadStockSentiment 读取舆情情绪缓存
func (s *Storage) LoadStockSentiment(symbol string) (*downloader.SentimentData, error) {
	path := filepath.Join(s.dataDir, "data", symbol, "sentiment.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var sentiment downloader.SentimentData
	if err := json.Unmarshal(data, &sentiment); err != nil {
		return nil, err
	}
	return &sentiment, nil
}

// EnsureComparableDataDir 确保可比公司数据目录存在
func (s *Storage) EnsureComparableDataDir(symbol string) (string, error) {
	dir := filepath.Join(s.dataDir, "comparables", symbol)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

// ComparablesConfigPath 返回可比公司配置文件路径
func (s *Storage) ComparablesConfigPath() string {
	return filepath.Join(s.dataDir, "comparables.json")
}

// LoadComparablesConfig 加载可比公司配置
func (s *Storage) LoadComparablesConfig() (map[string][]string, error) {
	path := s.ComparablesConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string][]string), nil
		}
		return nil, err
	}
	var config map[string][]string
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	return config, nil
}

// SaveComparablesConfig 保存可比公司配置
func (s *Storage) SaveComparablesConfig(config map[string][]string) error {
	path := s.ComparablesConfigPath()
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// GetComparables 获取某只股票的可比公司列表
func (s *Storage) GetComparables(symbol string) ([]string, error) {
	config, err := s.LoadComparablesConfig()
	if err != nil {
		return nil, err
	}
	return config[symbol], nil
}

// AddComparable 添加可比公司
func (s *Storage) AddComparable(symbol, comparable string) error {
	config, err := s.LoadComparablesConfig()
	if err != nil {
		return err
	}
	list := config[symbol]
	for _, c := range list {
		if c == comparable {
			return nil // 已存在
		}
	}
	if len(list) >= 5 {
		return fmt.Errorf("可比公司最多5家")
	}
	config[symbol] = append(list, comparable)
	return s.SaveComparablesConfig(config)
}

// RemoveComparable 移除可比公司
func (s *Storage) RemoveComparable(symbol, comparable string) error {
	config, err := s.LoadComparablesConfig()
	if err != nil {
		return err
	}
	list := config[symbol]
	filtered := make([]string, 0, len(list))
	for _, c := range list {
		if c != comparable {
			filtered = append(filtered, c)
		}
	}
	config[symbol] = filtered
	return s.SaveComparablesConfig(config)
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

func mergeYears(yearsList ...[]string) []string {
	seen := make(map[string]struct{})
	for _, list := range yearsList {
		for _, y := range list {
			seen[y] = struct{}{}
		}
	}
	var result []string
	for y := range seen {
		result = append(result, y)
	}
	// 尝试按字符串排序，通常时间格式字符串排序有效
	sort.Strings(result)
	// 反转成从新到旧
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}
	return result
}

// IndustryBaselinePath 返回行业基准文件路径
func (s *Storage) IndustryBaselinePath() string {
	return filepath.Join(s.dataDir, "industry_baseline.json")
}

// LoadIndustryBaselines 加载行业基准数据
func (s *Storage) LoadIndustryBaselines() (map[string]*analyzer.IndustryBaseline, error) {
	path := s.IndustryBaselinePath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var baselines map[string]*analyzer.IndustryBaseline
	if err := json.Unmarshal(data, &baselines); err != nil {
		return nil, err
	}
	return baselines, nil
}

// SaveIndustryBaselines 保存行业基准数据
func (s *Storage) SaveIndustryBaselines(baselines map[string]*analyzer.IndustryBaseline) error {
	path := s.IndustryBaselinePath()
	data, err := json.MarshalIndent(baselines, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// ActivityCachePath 返回某只股票的活跃度缓存文件路径
func (s *Storage) ActivityCachePath(symbol string) string {
	return filepath.Join(s.DataDir(), "data", symbol, "activity.json")
}

// SaveActivityCache 保存活跃度缓存
func (s *Storage) SaveActivityCache(symbol string, data *analyzer.ActivityData) error {
	path := s.ActivityCachePath(symbol)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0644)
}

// LoadActivityCache 加载活跃度缓存（同时校验时效，默认当天有效）
func (s *Storage) LoadActivityCache(symbol string) (*analyzer.ActivityData, error) {
	path := s.ActivityCachePath(symbol)
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if time.Since(info.ModTime()) > 24*time.Hour {
		return nil, fmt.Errorf("缓存过期")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var data analyzer.ActivityData
	if err := json.Unmarshal(b, &data); err != nil {
		return nil, err
	}
	return &data, nil
}

// RIMCachePath 返回某只股票的RIM数据缓存文件路径
func (s *Storage) RIMCachePath(symbol string) string {
	return filepath.Join(s.DataDir(), "data", symbol, "rim_cache.json")
}

// rimCacheWrapper 带时间戳的RIM缓存包装器
type rimCacheWrapper struct {
	Timestamp time.Time              `json:"timestamp"`
	Data      *downloader.RIMExternalData `json:"data"`
}

// SaveRIMCache 保存RIM外部数据缓存
func (s *Storage) SaveRIMCache(symbol string, data *downloader.RIMExternalData) error {
	path := s.RIMCachePath(symbol)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	wrapper := rimCacheWrapper{
		Timestamp: time.Now(),
		Data:      data,
	}
	b, err := json.MarshalIndent(wrapper, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0644)
}

// LoadRIMCache 加载RIM外部数据缓存（默认12小时有效）
func (s *Storage) LoadRIMCache(symbol string) (*downloader.RIMExternalData, error) {
	path := s.RIMCachePath(symbol)
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if time.Since(info.ModTime()) > 12*time.Hour {
		return nil, fmt.Errorf("缓存过期")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var wrapper rimCacheWrapper
	if err := json.Unmarshal(b, &wrapper); err != nil {
		return nil, err
	}
	return wrapper.Data, nil
}

// SaveSnapshot 保存分析报告快照（用于前端亮点与风险恢复）
func (s *Storage) SaveSnapshot(symbol string, report *analyzer.AnalysisReport) error {
	dir := filepath.Join(s.dataDir, "snapshots")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	path := filepath.Join(dir, symbol+".json")
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// LoadSnapshot 加载分析报告快照
func (s *Storage) LoadSnapshot(symbol string) (*analyzer.AnalysisReport, error) {
	path := filepath.Join(s.dataDir, "snapshots", symbol+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var report analyzer.AnalysisReport
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, err
	}
	return &report, nil
}

// DeleteSnapshot 删除分析报告快照
func (s *Storage) DeleteSnapshot(symbol string) error {
	path := filepath.Join(s.dataDir, "snapshots", symbol+".json")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
