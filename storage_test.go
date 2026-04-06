package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestArchiveAndCleanup(t *testing.T) {
	tmpDir := t.TempDir()
	s := &Storage{dataDir: tmpDir}

	symbol := "603501.SH"
	stockDir, err := s.EnsureStockDataDir(symbol)
	if err != nil {
		t.Fatal(err)
	}

	// 创建模拟的当前数据文件
	files := []string{"balance_sheet.json", "income_statement.json", "cash_flow.json"}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(stockDir, f), []byte("{}"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// 归档 5 次
	for i := 0; i < 5; i++ {
		ts := time.Now().Add(time.Duration(i) * time.Second).Format(time.RFC3339Nano)
		err := s.ArchiveStockData(symbol, HistoryMeta{
			Timestamp:  ts,
			Source:     "test",
			SourceName: "test-source",
			Years:      []string{"2025"},
		})
		if err != nil {
			t.Fatalf("archive %d failed: %v", i, err)
		}
	}

	// 检查历史目录数量
	historyDir := filepath.Join(stockDir, "history")
	entries, err := os.ReadDir(historyDir)
	if err != nil {
		t.Fatal(err)
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e.Name())
		}
	}
	if len(dirs) != 3 {
		t.Fatalf("expected 3 history dirs, got %d", len(dirs))
	}

	// 验证 meta.json 存在
	for _, d := range dirs {
		metaPath := filepath.Join(historyDir, d, "meta.json")
		if _, err := os.Stat(metaPath); os.IsNotExist(err) {
			t.Fatalf("meta.json missing in %s", d)
		}
	}
}

func TestListStockDataHistory(t *testing.T) {
	tmpDir := t.TempDir()
	s := &Storage{dataDir: tmpDir}
	symbol := "000001.SZ"
	stockDir, _ := s.EnsureStockDataDir(symbol)

	for _, f := range []string{"balance_sheet.json", "income_statement.json", "cash_flow.json"} {
		os.WriteFile(filepath.Join(stockDir, f), []byte("{}"), 0644)
	}

	s.ArchiveStockData(symbol, HistoryMeta{
		Timestamp:  "2026-01-02T10:00:00Z",
		Source:     "csv_import",
		SourceName: "同花顺",
		Years:      []string{"2025", "2024"},
	})
	s.ArchiveStockData(symbol, HistoryMeta{
		Timestamp:  "2026-01-01T10:00:00Z",
		Source:     "network_download",
		SourceName: "东方财富",
		Years:      []string{"2025"},
	})

	metas, err := s.ListStockDataHistory(symbol)
	if err != nil {
		t.Fatal(err)
	}
	if len(metas) != 2 {
		t.Fatalf("expected 2 metas, got %d", len(metas))
	}
	// 应该按时间从新到旧排序
	if metas[0].Timestamp != "2026-01-02T10:00:00Z" {
		t.Fatalf("expected newest first, got %s", metas[0].Timestamp)
	}
	if metas[0].SourceName != "同花顺" {
		t.Fatalf("unexpected source name: %s", metas[0].SourceName)
	}
}
