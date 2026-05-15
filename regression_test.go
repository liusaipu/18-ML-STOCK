package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liusaipu/stockfinlens/analyzer"
)

// Test603501ReportModules 验证 603501 完整分析报告的模块完整性（回归测试）
func Test603501ReportModules(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过端到端回归测试（使用 -short 快速模式）")
	}

	// 复用 integration_test.go 的数据准备逻辑
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data", "603501")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatal(err)
	}

	files := map[string]string{
		"balance_sheet.json":    "603501_debt_year.csv",
		"income_statement.json": "603501_benefit_year.csv",
		"cash_flow.json":        "603501_cash_year.csv",
	}

	for outName, csvName := range files {
		csvPath := filepath.Join(".", csvName)
		data, years, err := ParseThsCSV(csvPath)
		if err != nil {
			t.Fatalf("parse %s failed: %v", csvName, err)
		}
		if len(years) == 0 {
			t.Fatalf("%s 无可用年份数据", csvName)
		}
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

	md := report.MarkdownContent
	if md == "" {
		t.Fatal("报告 Markdown 内容为空")
	}

	// 验证核心模块存在（模块标题作为回归锚点，匹配 report.go 实际输出）
	requiredModules := []string{
		"# 模块1: 执行摘要",
		"# 模块2: 换手率深度分析",
		"# 模块3: 公司基本面分析",
		"# 模块4: 行业横向对比分析",
		"# 模块5: 十五五政策匹配度评估",
		"# 模块6: 剩余收益模型估值(RIM)",
		"# 模块7: A-Score 综合风险画像",
		"# 模块8: 技术面分析",
		"# 模块9: ML机器学习预测",
		"# 模块10: 智能选股7大条件",
		"# 模块11: 逆向思维检查",
		"# 模块12: 投资检查清单",
		"# 模块13: 社交媒体情绪监控",
		"# 模块14: 综合投资建议",
		"# 模块15: 结论与附录",
	}

	for _, module := range requiredModules {
		if !strings.Contains(md, module) {
			t.Errorf("报告缺少模块: %s", module)
		}
	}

	// 验证评分存在
	if len(report.Score) == 0 {
		t.Error("报告评分为空")
	}

	// 验证模块4.2 信息按钮结构（hover tooltip，非 details）
	// 注意：报告 Markdown 中不应包含 <details> 标签（已废弃）
	if strings.Contains(md, "<details") {
		t.Error("报告不应包含 <details> 标签（已废弃）")
	}

	// 验证报告包含核心分析内容（不强制 HTML 元素，因为部分依赖实时数据）
	if !strings.Contains(md, "A-Score") {
		t.Error("报告应包含 A-Score 分析")
	}
	if !strings.Contains(md, "ROE") {
		t.Error("报告应包含 ROE 分析")
	}

	t.Logf("报告验证通过，共 %d 个模块，%d 年评分数据", len(requiredModules), len(report.Score))
}

// TestStorageCRUD 验证存储层 CRUD 操作（回归测试）
func TestStorageCRUD(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过存储测试")
	}

	tmpDir := t.TempDir()
	storage := &Storage{dataDir: tmpDir}

	// Watchlist CRUD
	list := []WatchlistItem{
		{Code: "000001", Name: "平安银行", Market: "SZ"},
		{Code: "600519", Name: "贵州茅台", Market: "SH"},
	}
	if err := storage.SaveWatchlist(list); err != nil {
		t.Fatalf("保存自选列表失败: %v", err)
	}

	loaded, err := storage.LoadWatchlist()
	if err != nil {
		t.Fatalf("加载自选列表失败: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("自选列表应有 2 项, 实际 %d", len(loaded))
	}
	if loaded[0].Code != "000001" {
		t.Errorf("第一项代码应为 000001, 实际 %s", loaded[0].Code)
	}

	// AppConfig CRUD
	cfg := &AppConfig{AutoCheckUpdate: false, SkipVersion: "1.2.3"}
	if err := storage.SaveAppConfig(cfg); err != nil {
		t.Fatalf("保存配置失败: %v", err)
	}

	loadedCfg, err := storage.LoadAppConfig()
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}
	if loadedCfg.AutoCheckUpdate {
		t.Error("AutoCheckUpdate 应为 false")
	}
	if loadedCfg.SkipVersion != "1.2.3" {
		t.Errorf("SkipVersion 应为 1.2.3, 实际 %s", loadedCfg.SkipVersion)
	}
}

// TestAnalyzerScoreConsistency 验证评分计算一致性
func TestAnalyzerScoreConsistency(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过评分一致性测试")
	}

	// 使用与 Test603501ReportModules 相同的数据准备
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data", "603501")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatal(err)
	}

	files := map[string]string{
		"balance_sheet.json":    "603501_debt_year.csv",
		"income_statement.json": "603501_benefit_year.csv",
		"cash_flow.json":        "603501_cash_year.csv",
	}

	for outName, csvName := range files {
		csvPath := filepath.Join(".", csvName)
		data, _, err := ParseThsCSV(csvPath)
		if err != nil {
			t.Fatalf("parse %s failed: %v", csvName, err)
		}
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

	// 验证评分在合理范围内
	for year, score := range report.Score {
		if score < 0 || score > 100 {
			t.Errorf("年份 %s 评分 %.1f 超出 [0, 100] 范围", year, score)
		}
	}

	// 验证评级包含预期字母之一（OverallGrade 可能包含中文描述，如 "B (良好)"）
	validGradeLetters := []string{"A", "B", "C", "D", "E", "F"}
	hasValidGrade := false
	for _, g := range validGradeLetters {
		if strings.Contains(report.OverallGrade, g) {
			hasValidGrade = true
			break
		}
	}
	if !hasValidGrade {
		t.Errorf("评级 %s 不包含有效字母", report.OverallGrade)
	}

	if len(report.Score) > 0 {
		t.Logf("评分一致性通过: %d 年数据, 评级 %s", len(report.Score), report.OverallGrade)
	}
}
