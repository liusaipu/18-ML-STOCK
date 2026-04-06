package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stock-analyzer/analyzer"
)

func TestAnalyze603501(t *testing.T) {
	// 创建临时数据目录
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data", "603501")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatal(err)
	}

	files := map[string]string{
		"balance_sheet.json":   "603501_debt_year.csv",
		"income_statement.json": "603501_benefit_year.csv",
		"cash_flow.json":       "603501_cash_year.csv",
	}

	for outName, csvName := range files {
		csvPath := filepath.Join(".", csvName)
		data, years, err := ParseThsCSV(csvPath)
		if err != nil {
			t.Fatalf("parse %s failed: %v", csvName, err)
		}
		fmt.Printf("Parsed %s -> years: %v\n", csvName, years)
		jsonBytes, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dataDir, outName), jsonBytes, 0644); err != nil {
			t.Fatal(err)
		}
	}

	report, err := analyzer.RunAnalysis(tmpDir, "603501")
	if err != nil {
		t.Fatalf("analysis failed: %v", err)
	}

	fmt.Print("\n========== Markdown Report ==========\n\n")
	fmt.Println(report.MarkdownContent)
	fmt.Print("\n========== End of Report ==========\n\n")

	for year, score := range report.Score {
		fmt.Printf("Year %s: Score=%.1f, Grade=%s\n", year, score, report.OverallGrade)
	}
}
