package analyzer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// FinancialData 包含标准化的三张财务报表数据
type FinancialData struct {
	Symbol          string
	Years           []string
	BalanceSheet    map[string]map[string]float64
	IncomeStatement map[string]map[string]float64
	CashFlow        map[string]map[string]float64
	Extras          map[string]float64 // 非财务风险爬虫数据（股权质押、问询函、减持等）
}

// LoadFinancialData 从 stock-analyzer 存储目录加载某股票的财务报表 JSON
func LoadFinancialData(baseDir, symbol string) (*FinancialData, error) {
	stockDir := filepath.Join(baseDir, "data", symbol)

	bs, err := loadFloatJSONFile(filepath.Join(stockDir, "balance_sheet.json"))
	if err != nil {
		return nil, fmt.Errorf("load balance_sheet.json: %w", err)
	}
	is, err := loadFloatJSONFile(filepath.Join(stockDir, "income_statement.json"))
	if err != nil {
		return nil, fmt.Errorf("load income_statement.json: %w", err)
	}
	cf, err := loadFloatJSONFile(filepath.Join(stockDir, "cash_flow.json"))
	if err != nil {
		return nil, fmt.Errorf("load cash_flow.json: %w", err)
	}

	years := extractYearsFloat(bs)
	if len(years) == 0 {
		years = extractYearsFloat(is)
	}
	if len(years) == 0 {
		years = extractYearsFloat(cf)
	}

	return &FinancialData{
		Symbol:          symbol,
		Years:           years,
		BalanceSheet:    bs,
		IncomeStatement: is,
		CashFlow:        cf,
	}, nil
}

func loadFloatJSONFile(path string) (map[string]map[string]float64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var result map[string]map[string]float64
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func extractYearsFloat(data map[string]map[string]float64) []string {
	yearSet := make(map[string]struct{})
	for _, row := range data {
		for year := range row {
			yearSet[year] = struct{}{}
		}
	}
	years := make([]string, 0, len(yearSet))
	for y := range yearSet {
		years = append(years, y)
	}
	for i := 0; i < len(years); i++ {
		for j := i + 1; j < len(years); j++ {
			if years[i] < years[j] {
				years[i], years[j] = years[j], years[i]
			}
		}
	}
	return years
}

// GetValueOrZero 从 data[account][year] 中读取 float64，缺失时返回 0
func (fd *FinancialData) GetValueOrZero(data map[string]map[string]float64, account, year string) float64 {
	if data == nil {
		return 0
	}
	row, ok := data[account]
	if !ok {
		account = normalizeAccountName(account)
		row, ok = data[account]
		if !ok {
			return 0
		}
	}
	return row[year]
}

// normalizeAccountName 处理常见报表科目的同义别名
// 将 analyzer 侧使用的标准名映射到 csvparser 实际存储的键名
func normalizeAccountName(name string) string {
	switch name {
	case "资产合计":
		return "资产总计"
	case "负债合计":
		return "负债总计"
	case "归母所有者权益合计":
		return "归属于母公司所有者权益合计"
	case "归母净利润":
		return "归属于母公司所有者的净利润"
	// 利润表：同花顺 CSV 中营业收入/营业成本带有"其中："前缀
	case "营业收入":
		return "其中：营业收入"
	case "营业成本":
		return "其中：营业成本"
	// 资产负债表：固定资产/在建工程在 CSV 中为"固定资产合计"/"在建工程合计"
	case "固定资产":
		return "固定资产合计"
	case "在建工程":
		return "在建工程合计"
	// 现金流量表：csvparser 去掉了顿号，analyzer 仍使用原始带顿号名称；部分数据源使用不同表述
	case "经营活动现金流量净额":
		return "经营活动产生的现金流量净额"
	case "购建固定资产、无形资产和其他长期资产支付的现金":
		return "购建固定资产无形资产和其他长期资产支付的现金"
	case "分配股利、利润或偿付利息支付的现金":
		return "分配股利利润或偿付利息支付的现金"
	case "固定资产折旧、油气资产折耗、生产性生物资产折旧":
		return "固定资产折旧油气资产折耗生产性生物资产折旧"
	}
	return name
}
