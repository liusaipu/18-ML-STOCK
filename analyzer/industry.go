package analyzer

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// IndustryMetrics 行业均值指标
type IndustryMetrics struct {
	Industry      string  `json:"industry"`
	Count         int     `json:"count"`             // 统计样本数
	ROE           float64 `json:"roe"`               // 平均ROE
	ROEMedian     float64 `json:"roe_median"`        // ROE中位数
	GrossMargin   float64 `json:"gross_margin"`      // 平均毛利率
	RevenueGrowth float64 `json:"revenue_growth"`    // 平均营收增长
	DebtRatio     float64 `json:"debt_ratio"`        // 平均负债率
	CashRatio     float64 `json:"cash_ratio"`        // 平均现金含量
	MScore        float64 `json:"m_score"`           // 平均M-Score
	InventoryTurnover float64 `json:"inventory_turnover"` // 平均存货周转率
	ReceivableRatio   float64 `json:"receivable_ratio"`   // 平均应收账款占比
	UpdatedAt         string  `json:"updated_at"`         // 更新时间
}

// IndustryDatabase 行业均值数据库
type IndustryDatabase struct {
	Version    string                      `json:"version"`
	UpdatedAt  string                      `json:"updated_at"`
	Industries map[string]*IndustryMetrics `json:"industries"` // 行业名 -> 指标
}

var (
	industryDB         IndustryDatabase
	industryFallbackDB IndustryDatabase
	industryDBMu       sync.RWMutex
	industryDBInit     bool
)

// loadFallbackDB 加载 fallback 行业数据库
func loadFallbackDB(dataDir string) {
	path := filepath.Join(dataDir, "industry_database_fallback.json")
	data, err := os.ReadFile(path)
	if err != nil {
		// fallback 不存在是正常的，静默忽略
		industryFallbackDB = IndustryDatabase{Industries: make(map[string]*IndustryMetrics)}
		return
	}
	if err := json.Unmarshal(data, &industryFallbackDB); err != nil {
		industryFallbackDB = IndustryDatabase{Industries: make(map[string]*IndustryMetrics)}
	}
}

// InitIndustryDatabase 初始化行业均值数据库
func InitIndustryDatabase(dataDir string) error {
	path := filepath.Join(dataDir, "industry_database.json")
	
	// 尝试加载已有数据
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// 创建空数据库
			industryDB = IndustryDatabase{
				Version:    "1.0",
				UpdatedAt:  time.Now().Format(time.RFC3339),
				Industries: make(map[string]*IndustryMetrics),
			}
			loadFallbackDB(dataDir)
			industryDBInit = true
			return nil
		}
		return fmt.Errorf("读取行业数据库失败: %w", err)
	}
	
	if err := json.Unmarshal(data, &industryDB); err != nil {
		return fmt.Errorf("解析行业数据库失败: %w", err)
	}
	
	loadFallbackDB(dataDir)
	industryDBInit = true
	return nil
}

// ReloadIndustryDatabase 重新加载行业数据库
func ReloadIndustryDatabase(dataDir string) error {
	industryDBInit = false
	return InitIndustryDatabase(dataDir)
}

// findIndustry 在行业数据库中查找行业（支持模糊匹配）
func findIndustry(db IndustryDatabase, industry string) (*IndustryMetrics, bool) {
	m, ok := db.Industries[industry]
	if !ok {
		for name, metrics := range db.Industries {
			if contains(name, industry) || contains(industry, name) {
				return metrics, true
			}
		}
	}
	return m, ok
}

// GetLocalIndustryMetrics 仅获取本地行业均值指标（不合并 fallback）
func GetLocalIndustryMetrics(industry string) (*IndustryMetrics, bool) {
	industryDBMu.RLock()
	defer industryDBMu.RUnlock()
	
	if !industryDBInit {
		return nil, false
	}
	return findIndustry(industryDB, industry)
}

// GetIndustryMetrics 获取指定行业的均值指标（本地 + fallback 合并）
func GetIndustryMetrics(industry string) (*IndustryMetrics, bool) {
	industryDBMu.RLock()
	defer industryDBMu.RUnlock()
	
	if !industryDBInit {
		return nil, false
	}
	
	local, localOk := findIndustry(industryDB, industry)
	fallback, fallbackOk := findIndustry(industryFallbackDB, industry)
	
	// 如果本地数据样本充足（>=3），直接返回本地
	if localOk && local != nil && local.Count >= 3 {
		return local, true
	}
	
	// 如果本地样本不足或不存在，尝试用 fallback 补充 ROE/毛利率/营收增长
	if localOk && fallbackOk && fallback != nil {
		merged := *local
		if fallback.ROE != 0 {
			merged.ROE = fallback.ROE
		}
		if fallback.GrossMargin != 0 {
			merged.GrossMargin = fallback.GrossMargin
		}
		if fallback.RevenueGrowth != 0 {
			merged.RevenueGrowth = fallback.RevenueGrowth
		}
		// fallback 的 Count 用负数标记来源，或直接累加说明
		// 这里简单把 count 标记为 fallback 的样本数，让用户知道是外部数据
		merged.Count = fallback.Count
		return &merged, true
	}
	
	// 只有 fallback
	if fallbackOk && fallback != nil {
		return fallback, true
	}
	
	// 只有本地（即使样本不足也返回）
	if localOk {
		return local, true
	}
	
	return nil, false
}

// GetAllIndustries 获取所有行业列表
func GetAllIndustries() []string {
	industryDBMu.RLock()
	defer industryDBMu.RUnlock()
	
	var names []string
	for name := range industryDB.Industries {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// SaveIndustryDatabase 保存行业数据库到文件
func SaveIndustryDatabase(dataDir string) error {
	industryDBMu.Lock()
	defer industryDBMu.Unlock()
	
	path := filepath.Join(dataDir, "industry_database.json")
	data, err := json.MarshalIndent(industryDB, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// UpdateIndustryData 更新行业数据（由Python脚本调用后更新）
func UpdateIndustryData(industry string, metrics *IndustryMetrics) {
	industryDBMu.Lock()
	defer industryDBMu.Unlock()
	
	if industryDB.Industries == nil {
		industryDB.Industries = make(map[string]*IndustryMetrics)
	}
	
	metrics.UpdatedAt = time.Now().Format(time.RFC3339)
	industryDB.Industries[industry] = metrics
	industryDB.UpdatedAt = time.Now().Format(time.RFC3339)
}

// GetIndustryDBMeta 获取行业数据库元信息
func GetIndustryDBMeta() (version, updatedAt string, count int) {
	industryDBMu.RLock()
	defer industryDBMu.RUnlock()
	
	return industryDB.Version, industryDB.UpdatedAt, len(industryDB.Industries)
}

// IndustryComparison 行业对比结果
type IndustryComparison struct {
	Industry      string  `json:"industry"`
	HasData       bool    `json:"hasData"`
	ROEPercentile float64 `json:"roePercentile"`           // ROE行业百分位
	GMDiff        float64 `json:"gmDiff"`                  // 毛利率与行业均值差
	GrowthDiff    float64 `json:"growthDiff"`              // 营收增长与行业均值差
	DebtDiff      float64 `json:"debtDiff"`                // 负债率与行业均值差
	Summary       string  `json:"summary"`                 // 对比总结
}

// CompareWithIndustry 与行业均值对比
func CompareWithIndustry(industry string, steps []StepResult, year string) *IndustryComparison {
	ind, ok := GetIndustryMetrics(industry)
	if !ok {
		return &IndustryComparison{Industry: industry, HasData: false}
	}
	
	result := &IndustryComparison{
		Industry: industry,
		HasData:  true,
	}
	
	// 获取当前股票指标
	roe := getStepValue(steps, 16, year, "roe")
	gm := getStepValue(steps, 10, year, "grossMargin")
	growth := getStepValue(steps, 9, year, "growthRate")
	debt := getStepValue(steps, 3, year, "debtRatio")
	
	// 计算差异
	result.GMDiff = gm - ind.GrossMargin
	result.GrowthDiff = growth - ind.RevenueGrowth
	result.DebtDiff = debt - ind.DebtRatio
	
	// 计算ROE百分位（假设正态分布，使用均值和标准差估计）
	if ind.ROE > 0 {
		// 简化计算：(当前值-均值)/均值 * 50 + 50
		result.ROEPercentile = (roe - ind.ROE) / ind.ROE * 50 + 50
		if result.ROEPercentile < 0 {
			result.ROEPercentile = 0
		}
		if result.ROEPercentile > 100 {
			result.ROEPercentile = 100
		}
	}
	
	// 生成总结
	result.Summary = buildIndustrySummary(result, roe, ind.ROE)
	
	return result
}

func buildIndustrySummary(comp *IndustryComparison, roe, indROE float64) string {
	var parts []string
	
	if comp.ROEPercentile >= 75 {
		parts = append(parts, fmt.Sprintf("ROE处于行业前25%%(%.0f%%)，盈利能力优秀", 100-comp.ROEPercentile))
	} else if comp.ROEPercentile >= 50 {
		parts = append(parts, fmt.Sprintf("ROE处于行业中上水平(前%.0f%%)", 100-comp.ROEPercentile))
	} else if comp.ROEPercentile >= 25 {
		parts = append(parts, fmt.Sprintf("ROE处于行业中下水平(后%.0f%%)", comp.ROEPercentile))
	} else {
		parts = append(parts, fmt.Sprintf("ROE处于行业后25%%(%.0f%%)，盈利能力待提升", comp.ROEPercentile))
	}
	
	if math.Abs(comp.GMDiff) >= 5 {
		if comp.GMDiff > 0 {
			parts = append(parts, fmt.Sprintf("毛利率高于行业均值%.1f个百分点", comp.GMDiff))
		} else {
			parts = append(parts, fmt.Sprintf("毛利率低于行业均值%.1f个百分点", -comp.GMDiff))
		}
	}
	
	if math.Abs(comp.DebtDiff) >= 10 {
		if comp.DebtDiff < 0 {
			parts = append(parts, fmt.Sprintf("负债率低于行业均值%.1f个百分点，财务更稳健", -comp.DebtDiff))
		} else {
			parts = append(parts, fmt.Sprintf("负债率高于行业均值%.1f个百分点", comp.DebtDiff))
		}
	}
	
	if len(parts) == 0 {
		return "各项指标与行业均值接近"
	}
	
	return parts[0]
}

func contains(a, b string) bool {
	return len(a) >= len(b) && (a == b || len(a) > 0 && len(b) > 0 && 
		(a[:len(b)] == b || a[len(a)-len(b):] == b || containsSubstring(a, b)))
}

func containsSubstring(a, b string) bool {
	for i := 0; i <= len(a)-len(b); i++ {
		if a[i:i+len(b)] == b {
			return true
		}
	}
	return false
}
