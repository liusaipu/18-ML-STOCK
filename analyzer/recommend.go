package analyzer

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
)

// ComparableRecommendation 可比公司推荐项
type ComparableRecommendation struct {
	Symbol      string   `json:"symbol"`
	Name        string   `json:"name"`
	Score       float64  `json:"score"`       // 0-100 相似度得分
	Reasons     []string `json:"reasons"`     // 推荐理由
	DataQuality string   `json:"dataQuality"` // high/medium/low（本地是否有财报数据）
}

// RecommendComparables 基于多维度相似度自动推荐可比公司
// targetSymbol: 目标股票代码（带点格式，如 "000001.SZ"）
// targetProfile: 目标股票资料（行业、市值等）
// targetData: 目标股票财务数据（用于提取 ROE、毛利率）
// dataDir: 本地数据根目录（用于扫描候选股票）
// allSymbols: 全市场代码列表（扩大候选池）
// maxResults: 最大返回数量
func RecommendComparables(targetSymbol string, targetProfile *StockProfile, targetData *FinancialData, dataDir string, allSymbols []string, maxResults int) []ComparableRecommendation {
	if maxResults <= 0 {
		maxResults = 5
	}

	// 提取目标股票的关键特征
	targetIndustry := ""
	targetMarketCap := 0.0
	if targetProfile != nil {
		targetIndustry = targetProfile.Industry
		targetMarketCap = targetProfile.MarketCap
	}

	targetROE := 0.0
	targetGM := 0.0
	if targetData != nil && len(targetData.Years) > 0 {
		year := targetData.Years[0]
		equity := targetData.GetValueOrZero(targetData.BalanceSheet, "所有者权益合计", year)
		if equity == 0 {
			totalAssets := targetData.GetValueOrZero(targetData.BalanceSheet, "总资产", year)
			totalLiabilities := targetData.GetValueOrZero(targetData.BalanceSheet, "总负债", year)
			equity = totalAssets - totalLiabilities
		}
		netProfit := targetData.GetValueOrZero(targetData.IncomeStatement, "净利润", year)
		revenue := targetData.GetValueOrZero(targetData.IncomeStatement, "营业收入", year)
		cost := targetData.GetValueOrZero(targetData.IncomeStatement, "营业成本", year)
		if equity > 0 {
			targetROE = netProfit / equity
		}
		if revenue > 0 {
			targetGM = (revenue - cost) / revenue
		}
	}

	// 读取目标股票的概念标签
	targetConcepts := loadConcepts(filepath.Join(dataDir, "data"), targetSymbol)

	// 扫描候选股票（本地 + 全市场）
	candidates := scanLocalCandidates(dataDir, targetSymbol, allSymbols)

	// 计算每个候选股票的相似度
	var scored []ComparableRecommendation
	for _, c := range candidates {
		score, reasons, dataQuality := computeSimilarity(
			targetIndustry, targetMarketCap, targetROE, targetGM, targetConcepts,
			c,
		)
		// 设置最低得分门槛：目标有行业信息时至少15分，否则至少8分
		minScore := 8.0
		if targetIndustry != "" {
			minScore = 15.0
		}
		if score >= minScore {
			scored = append(scored, ComparableRecommendation{
				Symbol:      c.Symbol,
				Name:        c.Name,
				Score:       score,
				Reasons:     reasons,
				DataQuality: dataQuality,
			})
		}
	}

	// 按得分降序排序
	for i := 0; i < len(scored)-1; i++ {
		for j := i + 1; j < len(scored); j++ {
			if scored[j].Score > scored[i].Score {
				scored[i], scored[j] = scored[j], scored[i]
			}
		}
	}

	if len(scored) > maxResults {
		scored = scored[:maxResults]
	}
	return scored
}

// candidateInfo 候选股票内部信息
type candidateInfo struct {
	Symbol    string
	Name      string
	Industry  string
	MarketCap float64
	ROE       float64
	GM        float64
	HasData   bool     // 是否有本地财报数据
	Concepts  []string // 概念/风口标签
}

// StockProfile 推荐算法使用的股票资料子集（避免循环导入）
type StockProfile struct {
	Industry  string
	MarketCap float64
}

// scanLocalCandidates 扫描本地数据目录获取候选股票，同时补充全市场代码
func scanLocalCandidates(dataDir, excludeSymbol string, allSymbols []string) []candidateInfo {
	dataRoot := filepath.Join(dataDir, "data")

	// 先扫描本地有数据的股票
	localMap := make(map[string]candidateInfo)
	entries, err := os.ReadDir(dataRoot)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			symbol := entry.Name()
			if symbol == excludeSymbol {
				continue
			}

			info := candidateInfo{Symbol: symbol}

			// 尝试读取 profile.json 获取行业和市值
			profilePath := filepath.Join(dataRoot, symbol, "profile.json")
			if data, err := os.ReadFile(profilePath); err == nil {
				info.Industry = extractJSONString(data, "industry")
				info.Name = extractJSONString(data, "name")
				if mc := extractJSONFloat(data, "market_cap"); mc > 0 {
					info.MarketCap = mc
				}
			}

			// 尝试读取财务数据获取 ROE 和毛利率
			bsPath := filepath.Join(dataRoot, symbol, "balance_sheet.json")
			isPath := filepath.Join(dataRoot, symbol, "income_statement.json")
			if _, err1 := os.Stat(bsPath); err1 == nil {
				if _, err2 := os.Stat(isPath); err2 == nil {
					info.HasData = true
					info.ROE, info.GM = extractLatestMetrics(dataRoot, symbol)
				}
			}

			// 尝试读取概念标签
			info.Concepts = loadConcepts(dataRoot, symbol)

			localMap[symbol] = info
		}
	}

	// 如果有全市场代码列表，补充没有本地资料的候选
	if len(allSymbols) > 0 {
		for _, symbol := range allSymbols {
			if symbol == excludeSymbol {
				continue
			}
			if _, ok := localMap[symbol]; ok {
				continue
			}
			// 尝试读取 profile.json（可能由 batchFetchCandidateProfiles 缓存）
			info := candidateInfo{Symbol: symbol}
			profilePath := filepath.Join(dataRoot, symbol, "profile.json")
			if data, err := os.ReadFile(profilePath); err == nil {
				info.Industry = extractJSONString(data, "industry")
				info.Name = extractJSONString(data, "name")
				if mc := extractJSONFloat(data, "market_cap"); mc > 0 {
					info.MarketCap = mc
				}
			}
			// 补充概念标签
			info.Concepts = loadConcepts(dataRoot, symbol)
			localMap[symbol] = info
		}
	}

	var candidates []candidateInfo
	for _, info := range localMap {
		candidates = append(candidates, info)
	}
	return candidates
}

// industrySynonyms 行业同义词映射（同一产业链的不同表述）
var industrySynonyms = map[string][]string{
	"半导体":       {"集成电路", "芯片", "晶圆", "封测", "IC"},
	"集成电路":     {"半导体", "芯片", "晶圆"},
	"芯片":         {"半导体", "集成电路"},
	"新能源":       {"光伏", "风电", "储能", "锂电池", "动力电池"},
	"光伏":         {"新能源", "太阳能"},
	"锂电池":       {"新能源", "动力电池", "储能"},
	"动力电池":     {"新能源", "锂电池"},
	"医药":         {"制药", "生物", "医疗器械", "中药", "化学药"},
	"制药":         {"医药", "生物", "化学药"},
	"生物":         {"医药", "制药", "生物技术"},
	"医疗器械":     {"医药", "医疗"},
	"银行":         {"金融", "商业银行", "城商行"},
	"保险":         {"金融", "寿险", "财险"},
	"证券":         {"金融", "券商", "投行"},
	"房地产":       {"地产", "房地产开发", "物业管理"},
	"地产":         {"房地产", "房地产开发"},
	"汽车":         {"整车", "新能源汽车", "汽车零部件"},
	"整车":         {"汽车", "新能源汽车"},
	"汽车零部件":   {"汽车", "汽配"},
	"电子":         {"消费电子", "元器件", "PCB", "被动元件"},
	"消费电子":     {"电子", "手机", "可穿戴"},
	"通信":         {"电信", "5G", "光模块", "光纤"},
	"电信运营":     {"通信", "通信服务", "电信", "5G", "光模块"},
	"通信服务":     {"通信", "电信运营", "电信", "5G", "光模块"},
	"5G":           {"通信", "电信"},
	"计算机":       {"软件", "IT", "云计算", "人工智能", "AI"},
	"软件":         {"计算机", "IT", "云计算"},
	"人工智能":     {"计算机", "AI", "软件"},
	"化工":         {"化学", "化学制品", "精细化工", "石化"},
	"化学制品":     {"化工", "精细化工"},
	"食品饮料":     {"食品", "饮料", "白酒", "啤酒", "乳制品"},
	"白酒":         {"食品饮料", "酒类"},
	"家电":         {"家用电器", "白色家电", "厨电"},
	"有色金属":     {"有色", "铜", "铝", "稀土", "锂"},
	"钢铁":         {"黑色金属", "特钢"},
	"煤炭":         {"能源", "焦煤", "动力煤"},
	"石油":         {"能源", "石化", "油气"},
	"电力":         {"公用事业", "火电", "水电", "核电"},
	"交通运输":     {"物流", "航空", "航运", "港口", "铁路"},
	"物流":         {"交通运输", "快递", "供应链"},
	"传媒":         {"媒体", "广告", "影视", "游戏", "互联网"},
	"游戏":         {"传媒", "互联网", "电竞"},
	"互联网":       {"传媒", "软件", "IT"},
	"建筑":         {"基建", "建筑工程", "建材", "装饰"},
	"建材":         {"建筑", "水泥", "玻璃"},
	"农林牧渔":     {"农业", "养殖", "种植", "畜牧"},
	"养殖":         {"农林牧渔", "畜牧", "猪"},
}

// 等效行业映射（不同数据源/平台对同一行业的不同命名）
var equivalentIndustries = map[string][]string{
	"电信运营": {"通信服务"},
	"通信服务": {"电信运营"},
}

// computeSimilarity 计算候选股票与目标股票的相似度
func computeSimilarity(targetIndustry string, targetMarketCap, targetROE, targetGM float64, targetConcepts []string, c candidateInfo) (float64, []string, string) {
	score := 0.0
	var reasons []string

	// 1. 行业匹配 (65%)
	industryScore := 0.0
	if targetIndustry != "" && c.Industry != "" {
		if targetIndustry == c.Industry {
			industryScore = 65
			reasons = append(reasons, "同属"+targetIndustry)
		} else if isEquivalentIndustry(targetIndustry, c.Industry) {
			industryScore = 45
			reasons = append(reasons, "同属"+targetIndustry)
		} else {
			// 简单的行业关键词匹配
			targetKeys := extractIndustryKeywords(targetIndustry)
			candidateKeys := extractIndustryKeywords(c.Industry)
			matchCount := 0
			for _, tk := range targetKeys {
				for _, ck := range candidateKeys {
					if tk == ck && len(tk) >= 2 {
						matchCount++
					}
				}
			}
			if matchCount > 0 {
				partialScore := 65.0 * float64(matchCount) / float64(len(targetKeys))
				if partialScore > 65 {
					partialScore = 65
				}
				industryScore = partialScore
				reasons = append(reasons, "行业相近")
			} else {
				// 检查同义词映射（产业链相关，但非同一细分行业）
				if isSynonymIndustry(targetIndustry, c.Industry) {
					industryScore = 25
					reasons = append(reasons, "产业链相关")
				}
			}
		}
	}
	score += industryScore

	// 行业惩罚：如果目标有行业但候选没有，大幅降低得分
	if targetIndustry != "" && c.Industry == "" {
		// 无行业数据的候选，最高只能得 35 分（市值10+ROE15+毛利率5+数据质量5的封顶）
		// 实际计算后会在最后统一打折
	}

	// 2. 市值相近 (10%)
	if targetMarketCap > 0 && c.MarketCap > 0 {
		ratio := targetMarketCap / c.MarketCap
		if ratio < 1 {
			ratio = c.MarketCap / targetMarketCap
		}
		if ratio <= 2 {
			score += 10
			reasons = append(reasons, "市值相近")
		} else if ratio <= 5 {
			ms := 10 * (1 - (ratio-2)/3)
			if ms < 0 {
				ms = 0
			}
			score += ms
			reasons = append(reasons, "市值相近")
		}
	}

	// 3. ROE 结构相似 (15%)
	if targetROE != 0 && c.ROE != 0 {
		diff := math.Abs(targetROE - c.ROE)
		if diff < 0.03 {
			score += 15
			reasons = append(reasons, "ROE结构相似")
		} else if diff < 0.10 {
			score += 15 * (1 - (diff-0.03)/0.07)
			reasons = append(reasons, "ROE结构相似")
		}
	}

	// 4. 毛利率结构相似 (5%)
	if targetGM != 0 && c.GM != 0 {
		diff := math.Abs(targetGM - c.GM)
		if diff < 0.05 {
			score += 5
			reasons = append(reasons, "毛利率结构相似")
		} else if diff < 0.15 {
			score += 5 * (1 - (diff-0.05)/0.10)
			reasons = append(reasons, "毛利率结构相似")
		}
	}

	// 5. 概念/风口匹配 (10%)
	if len(targetConcepts) > 0 && len(c.Concepts) > 0 {
		overlap := 0
		for _, tc := range targetConcepts {
			for _, cc := range c.Concepts {
				if tc == cc {
					overlap++
				}
			}
		}
		if overlap > 0 {
			conceptScore := 10.0 * float64(overlap) / float64(len(targetConcepts))
			if conceptScore > 10 {
				conceptScore = 10
			}
			score += conceptScore
			reasons = append(reasons, fmt.Sprintf("共享%d个概念", overlap))
		}
	}

	// 6. 有财务数据加分（权重从 20% 降到 5%，避免"有数据就靠前"）
	dataQuality := "low"
	if c.HasData {
		score += 5
		dataQuality = "high"
		reasons = append(reasons, "本地有财报数据")
	} else if c.Industry != "" {
		score += 2
		dataQuality = "medium"
	}

	// 行业惩罚1：目标有行业但候选无行业数据，得分打 3 折
	if targetIndustry != "" && c.Industry == "" {
		score = score * 0.3
	}

	// 行业惩罚2：目标有行业且候选有行业，但行业完全不相关（非同义词且无关键词重叠），得分打 2 折
	if targetIndustry != "" && c.Industry != "" && industryScore == 0 {
		score = score * 0.2
	}

	return score, reasons, dataQuality
}

// isEquivalentIndustry 检查两个行业是否为同一行业的不同命名
func isEquivalentIndustry(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	if eqs, ok := equivalentIndustries[a]; ok {
		for _, e := range eqs {
			if e == b {
				return true
			}
		}
	}
	if eqs, ok := equivalentIndustries[b]; ok {
		for _, e := range eqs {
			if e == a {
				return true
			}
		}
	}
	return false
}

// isSynonymIndustry 检查两个行业是否通过同义词映射相关
func isSynonymIndustry(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	// 直接检查映射表
	if syns, ok := industrySynonyms[a]; ok {
		for _, s := range syns {
			if strings.Contains(b, s) || strings.Contains(s, b) {
				return true
			}
		}
	}
	// 反向检查
	if syns, ok := industrySynonyms[b]; ok {
		for _, s := range syns {
			if strings.Contains(a, s) || strings.Contains(s, a) {
				return true
			}
		}
	}
	return false
}

// extractIndustryKeywords 从行业名称中提取关键词
func extractIndustryKeywords(industry string) []string {
	parts := strings.FieldsFunc(industry, func(r rune) bool {
		return r == '、' || r == '/' || r == '·' || r == ' ' || r == '，' || r == ','
	})
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if len(p) >= 2 {
			result = append(result, p)
		}
	}
	return result
}

// extractJSONString 从 JSON 字节中简单提取字符串值（无引号转义处理，仅适用于简单 JSON）
func extractJSONString(data []byte, key string) string {
	keyPattern := `"` + key + `"`
	idx := strings.Index(string(data), keyPattern)
	if idx < 0 {
		return ""
	}
	rest := string(data)[idx+len(keyPattern):]
	i := 0
	for i < len(rest) && (rest[i] == ':' || rest[i] == ' ' || rest[i] == '\t' || rest[i] == '\n' || rest[i] == '\r') {
		i++
	}
	if i < len(rest) && rest[i] == '"' {
		i++
		start := i
		for i < len(rest) && rest[i] != '"' {
			i++
		}
		return rest[start:i]
	}
	return ""
}

// extractJSONFloat 从 JSON 字节中简单提取 float64 值
func extractJSONFloat(data []byte, key string) float64 {
	keyPattern := `"` + key + `"`
	idx := strings.Index(string(data), keyPattern)
	if idx < 0 {
		return 0
	}
	rest := string(data)[idx+len(keyPattern):]
	i := 0
	for i < len(rest) && (rest[i] == ':' || rest[i] == ' ' || rest[i] == '\t' || rest[i] == '\n' || rest[i] == '\r') {
		i++
	}
	start := i
	for i < len(rest) && (rest[i] == '-' || rest[i] == '.' || (rest[i] >= '0' && rest[i] <= '9')) {
		i++
	}
	var val float64
	if _, err := fmt.Sscanf(rest[start:i], "%f", &val); err == nil {
		return val
	}
	return 0
}

// extractLatestMetrics 从本地数据中提取最新年份的 ROE 和毛利率
func extractLatestMetrics(dataRoot, symbol string) (roe, gm float64) {
	fd, err := LoadFinancialData(dataRoot, symbol)
	if err != nil || len(fd.Years) == 0 {
		return 0, 0
	}
	year := fd.Years[0]
	equity := fd.GetValueOrZero(fd.BalanceSheet, "所有者权益合计", year)
	if equity == 0 {
		totalAssets := fd.GetValueOrZero(fd.BalanceSheet, "总资产", year)
		totalLiabilities := fd.GetValueOrZero(fd.BalanceSheet, "总负债", year)
		equity = totalAssets - totalLiabilities
	}
	netProfit := fd.GetValueOrZero(fd.IncomeStatement, "净利润", year)
	revenue := fd.GetValueOrZero(fd.IncomeStatement, "营业收入", year)
	cost := fd.GetValueOrZero(fd.IncomeStatement, "营业成本", year)
	if equity > 0 {
		roe = netProfit / equity
	}
	if revenue > 0 {
		gm = (revenue - cost) / revenue
	}
	return
}

// loadConcepts 从本地 concepts.json 读取股票的概念/风口标签
func loadConcepts(dataRoot, symbol string) []string {
	path := filepath.Join(dataRoot, symbol, "concepts.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	// 简单解析 JSON 中的 concepts 数组
	concepts := extractJSONStringArray(data, "concepts")
	// 对长字符串概念按 '、' 拆分（如 "电信、广播电视和卫星传输服务"）
	var result []string
	seen := make(map[string]struct{})
	for _, c := range concepts {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		// 如果概念包含 '、'，拆分为多个子概念
		if strings.Contains(c, "、") {
			for _, part := range strings.Split(c, "、") {
				part = strings.TrimSpace(part)
				if part != "" {
					if _, ok := seen[part]; !ok {
						seen[part] = struct{}{}
						result = append(result, part)
					}
				}
			}
		} else {
			if _, ok := seen[c]; !ok {
				seen[c] = struct{}{}
				result = append(result, c)
			}
		}
	}
	return result
}

// extractJSONStringArray 从 JSON 字节中简单提取字符串数组
func extractJSONStringArray(data []byte, key string) []string {
	keyPattern := `"` + key + `"`
	idx := strings.Index(string(data), keyPattern)
	if idx < 0 {
		return nil
	}
	rest := string(data)[idx+len(keyPattern):]
	i := 0
	for i < len(rest) && (rest[i] == ':' || rest[i] == ' ' || rest[i] == '\t' || rest[i] == '\n' || rest[i] == '\r') {
		i++
	}
	if i >= len(rest) || rest[i] != '[' {
		return nil
	}
	i++ // skip [
	var result []string
	for i < len(rest) {
		// skip whitespace
		for i < len(rest) && (rest[i] == ' ' || rest[i] == '\t' || rest[i] == '\n' || rest[i] == '\r' || rest[i] == ',') {
			i++
		}
		if i < len(rest) && rest[i] == ']' {
			break
		}
		if i < len(rest) && rest[i] == '"' {
			i++
			start := i
			for i < len(rest) && rest[i] != '"' {
				i++
			}
			result = append(result, rest[start:i])
			if i < len(rest) && rest[i] == '"' {
				i++
			}
		} else {
			i++
		}
	}
	return result
}
