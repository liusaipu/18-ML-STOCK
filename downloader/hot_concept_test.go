package downloader

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// mockConceptBoardJSON 模拟东财概念板块 API 返回结构
var mockConceptBoardJSON = []byte(`{
  "rc": 0, "rt": 6, "svr": 181735164, "lt": 1, "full": 1,
  "data": {
    "total": 496,
    "diff": [
      {"f12": "BK1168", "f14": "固态电池", "f2": 1234.56, "f3": 5.23, "f4": 61.34, "f5": 1523000, "f6": 2860000000, "f10": 2.74, "f15": 1260.5, "f62": 890000000, "f184": 8.5, "f204": "宁德时代", "f205": "300750"},
      {"f12": "BK0729", "f14": "人工智能", "f2": 2345.67, "f3": 3.15, "f4": 71.56, "f5": 2100000, "f6": 4500000000, "f10": 1.85, "f15": 2380.2, "f62": 1200000000, "f184": 12.3, "f204": "科大讯飞", "f205": "002230"},
      {"f12": "BK0854", "f14": "光伏设备", "f2": 987.12, "f3": -1.20, "f4": -11.95, "f5": 890000, "f6": 1200000000, "f10": 0.92, "f15": 995.8, "f62": -300000000, "f184": -3.2, "f204": "隆基绿能", "f205": "601012"},
      {"f12": "BK0912", "f14": "半导体",   "f2": 3456.78, "f3": 1.80, "f4": 61.20, "f5": 1800000, "f6": 3200000000, "f10": 1.55, "f15": 3500.1, "f62": 450000000, "f184": 5.1, "f204": "中芯国际", "f205": "688981"},
      {"f12": "BK1023", "f14": "创新药",   "f2": 876.54, "f3": 0.50, "f4": 4.35, "f5": 450000, "f6": 600000000, "f10": 0.78, "f15": 880.3, "f62": 80000000, "f184": 1.2, "f204": "恒瑞医药", "f205": "600276"}
    ]
  }
}`)

// mockConceptConstituentsJSON 模拟成分股 API 返回
var mockConceptConstituentsJSON = []byte(`{
  "rc": 0, "rt": 6,
  "data": {
    "total": 45,
    "diff": [
      {"f12": "300750", "f14": "宁德时代", "f2": 185.20, "f3": 4.52, "f20": 800000000000, "f130": 15.8, "f62": 320000000},
      {"f12": "002074", "f14": "国轩高科", "f2": 21.35, "f3": 3.18, "f20": 120000000000, "f130": 8.5, "f62": 85000000},
      {"f12": "300014", "f14": "亿纬锂能", "f2": 42.10, "f3": -0.85, "f20": 250000000000, "f130": -5.2, "f62": -12000000}
    ]
  }
}`)

func TestParseConceptBoard(t *testing.T) {
	var resp struct {
		Data struct {
			Diff []map[string]interface{} `json:"diff"`
		} `json:"data"`
	}
	if err := json.Unmarshal(mockConceptBoardJSON, &resp); err != nil {
		t.Fatalf("解析 mock 数据失败: %v", err)
	}

	concepts := make([]HotConcept, 0, len(resp.Data.Diff))
	for _, item := range resp.Data.Diff {
		c := HotConcept{
			Code:         parseString(item["f12"]),
			Name:         parseString(item["f14"]),
			ChangePct:    parseFloat(item["f3"]),
			ChangeAmt:    parseFloat(item["f4"]),
			Volume:       parseFloat(item["f5"]),
			Turnover:     parseFloat(item["f6"]),
			MainInflow:   parseFloat(item["f62"]),
			MainInRatio:  parseFloat(item["f184"]),
			TopStock:     parseString(item["f204"]),
			TopStockCode: parseString(item["f205"]),
		}
		concepts = append(concepts, c)
	}

	if len(concepts) != 5 {
		t.Fatalf("期望 5 条概念，实际 %d", len(concepts))
	}
	if concepts[0].Name != "固态电池" {
		t.Errorf("期望第一条是固态电池，实际是 %s", concepts[0].Name)
	}
	if concepts[0].ChangePct != 5.23 {
		t.Errorf("期望固态电池涨幅 5.23，实际是 %.2f", concepts[0].ChangePct)
	}
	if concepts[2].MainInflow != -300000000 {
		t.Errorf("期望光伏设备主力净流入 -300000000，实际是 %.0f", concepts[2].MainInflow)
	}
	t.Log("解析概念板块数据成功")
}

// TestCalcHotScore 验证综合打分排序逻辑
func TestCalcHotScore(t *testing.T) {
	concepts := []HotConcept{
		{Code: "BK001", Name: "概念A", ChangePct: 5.0, MainInflow: 1000000, Turnover: 5000000},
		{Code: "BK002", Name: "概念B", ChangePct: 3.0, MainInflow: 2000000, Turnover: 3000000},
		{Code: "BK003", Name: "概念C", ChangePct: -1.0, MainInflow: -500000, Turnover: 1000000},
	}
	calcHotScore(concepts)

	// 概念B 净流入最高，概念A 涨幅最高，综合后概念A 应该排第一
	if concepts[0].Name != "概念A" {
		t.Errorf("期望第一名是概念A，实际是 %s (score=%.2f)", concepts[0].Name, concepts[0].Score)
	}
	if concepts[1].Name != "概念B" {
		t.Errorf("期望第二名是概念B，实际是 %s (score=%.2f)", concepts[1].Name, concepts[1].Score)
	}
	// 概念C 为负，应排最后
	if concepts[2].Name != "概念C" {
		t.Errorf("期望第三名是概念C，实际是 %s (score=%.2f)", concepts[2].Name, concepts[2].Score)
	}

	for _, c := range concepts {
		t.Logf("%s: score=%.2f", c.Name, c.Score)
	}
}

// TestCalcHotScoreWithMockData 用 mock 数据验证打分后的排序
func TestCalcHotScoreWithMockData(t *testing.T) {
	var resp struct {
		Data struct {
			Diff []map[string]interface{} `json:"diff"`
		} `json:"data"`
	}
	json.Unmarshal(mockConceptBoardJSON, &resp)

	concepts := make([]HotConcept, 0, len(resp.Data.Diff))
	for _, item := range resp.Data.Diff {
		c := HotConcept{
			Code:       parseString(item["f12"]),
			Name:       parseString(item["f14"]),
			ChangePct:  parseFloat(item["f3"]),
			Turnover:   parseFloat(item["f6"]),
			MainInflow: parseFloat(item["f62"]),
		}
		concepts = append(concepts, c)
	}

	calcHotScore(concepts)

	// 人工智能 净流入最高且涨幅较高，应该排第一或第二
	// 固态电池 涨幅最高，也应该排前二
	t.Logf("排序后: 1=%s(score=%.2f) 2=%s(score=%.2f) 3=%s(score=%.2f)",
		concepts[0].Name, concepts[0].Score,
		concepts[1].Name, concepts[1].Score,
		concepts[2].Name, concepts[2].Score)

	// 创新药涨幅和净流入都最小，应该排最后
	if concepts[len(concepts)-1].Name != "创新药" {
		t.Errorf("期望最后一名是创新药，实际是 %s", concepts[len(concepts)-1].Name)
	}
}

// TestFetchHotConceptBoardWithCache 验证完整流程（含缓存，使用 mock）
func TestFetchHotConceptBoardWithCache(t *testing.T) {
	tmpDir := t.TempDir()

	// 先写入一个旧缓存（超过4小时）
	oldBoard := &HotConceptBoard{
		Date:         time.Now().AddDate(0, 0, -1).Format("2006-01-02"),
		UpdatedAt:    time.Now().Add(-5 * time.Hour).Format("2006-01-02 15:04:05"),
		Concepts:     []HotConcept{{Name: "旧数据", Score: 10}},
		CacheVersion: hotConceptCacheVer,
	}
	_ = saveHotConceptCache(tmpDir, oldBoard)

	// 注意：由于无法访问真实 API，这里只验证缓存过期后的加载逻辑
	// 实际生产环境由 FetchHotConceptBoard 走网络分支
	loaded, ok := loadHotConceptCache(tmpDir)
	if ok {
		t.Fatal("旧缓存应该已过期")
	}
	if loaded != nil {
		t.Logf("过期缓存被正确忽略")
	}

	// 写入一个有效缓存
	validBoard := &HotConceptBoard{
		Date:         time.Now().Format("2006-01-02"),
		UpdatedAt:    time.Now().Format("2006-01-02 15:04:05"),
		Concepts:     []HotConcept{{Name: "缓存数据", Score: 99}},
		CacheVersion: hotConceptCacheVer,
	}
	_ = saveHotConceptCache(tmpDir, validBoard)

	loaded2, ok2 := loadHotConceptCache(tmpDir)
	if !ok2 {
		t.Fatal("有效缓存应该命中")
	}
	if loaded2.Concepts[0].Name != "缓存数据" {
		t.Fatalf("缓存数据不匹配: %s", loaded2.Concepts[0].Name)
	}
	t.Log("缓存读写验证通过")
}

// TestFetchHotConceptHistory 验证历史数据读取
func TestFetchHotConceptHistory(t *testing.T) {
	tmpDir := t.TempDir()

	// 构造 3 天的历史数据
	for i := 0; i < 3; i++ {
		date := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		board := &HotConceptBoard{
			Date: date,
			Concepts: []HotConcept{
				{Name: fmt.Sprintf("概念%dA", i), Score: float64(100 - i*10)},
				{Name: fmt.Sprintf("概念%dB", i), Score: float64(90 - i*10)},
			},
		}
		path := filepath.Join(tmpDir, "hot_concepts", "history", date+".json")
		os.MkdirAll(filepath.Dir(path), 0755)
		data, _ := json.MarshalIndent(board, "", "  ")
		os.WriteFile(path, data, 0644)
	}

	history, err := FetchHotConceptHistory(tmpDir, 7)
	if err != nil {
		t.Fatalf("读取历史失败: %v", err)
	}
	if len(history) != 3 {
		t.Fatalf("期望 3 条历史，实际 %d", len(history))
	}
	for _, h := range history {
		t.Logf("%s: %v", h.Date, h.TopNames)
	}
}

// TestParseConceptConstituents 验证成分股解析
func TestParseConceptConstituents(t *testing.T) {
	var resp struct {
		Data struct {
			Diff []map[string]interface{} `json:"diff"`
		} `json:"data"`
	}
	if err := json.Unmarshal(mockConceptConstituentsJSON, &resp); err != nil {
		t.Fatalf("解析 mock 成分股失败: %v", err)
	}

	result := make([]ConceptConstituent, 0, len(resp.Data.Diff))
	for _, item := range resp.Data.Diff {
		c := ConceptConstituent{
			Code:              parseString(item["f12"]),
			Name:              parseString(item["f14"]),
			Price:             parseFloat(item["f2"]),
			ChangePct:         parseFloat(item["f3"]),
			MarketCap:         parseFloat(item["f20"]),
			HalfYearChangePct: parseFloat(item["f130"]),
			MainInflow:        parseFloat(item["f62"]),
		}
		result = append(result, c)
	}

	if len(result) != 3 {
		t.Fatalf("期望 3 条成分股，实际 %d", len(result))
	}
	if result[0].Name != "宁德时代" {
		t.Errorf("期望第一条是宁德时代，实际是 %s", result[0].Name)
	}
	if result[0].ChangePct != 4.52 {
		t.Errorf("期望宁德时代涨幅 4.52，实际是 %.2f", result[0].ChangePct)
	}
	t.Log("成分股解析成功")
}

func TestParseFloatEdgeCases(t *testing.T) {
	tests := []struct {
		input    interface{}
		expected float64
	}{
		{nil, 0},
		{"-", 0},
		{"", 0},
		{"3.14", 3.14},
		{5.5, 5.5},
		{0, 0},
	}
	for _, tc := range tests {
		got := parseFloat(tc.input)
		if got != tc.expected {
			t.Errorf("parseFloat(%v) = %.2f, want %.2f", tc.input, got, tc.expected)
		}
	}
}

func TestAbs(t *testing.T) {
	if abs(-5.5) != 5.5 {
		t.Error("abs(-5.5) != 5.5")
	}
	if abs(3.0) != 3.0 {
		t.Error("abs(3.0) != 3.0")
	}
	if abs(0) != 0 {
		t.Error("abs(0) != 0")
	}
}
