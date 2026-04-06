package downloader

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// SaveAsJSON 将下载的数据保存为项目标准 JSON 文件
func SaveAsJSON(data *FinancialReportData, dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	files := map[string]map[string]map[string]float64{
		"balance_sheet.json":   data.BalanceSheet,
		"income_statement.json": data.IncomeStatement,
		"cash_flow.json":       data.CashFlow,
	}
	for name, content := range files {
		path := filepath.Join(dir, name)
		b, err := json.MarshalIndent(content, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal %s: %w", name, err)
		}
		if err := os.WriteFile(path, b, 0644); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
	}
	return nil
}
