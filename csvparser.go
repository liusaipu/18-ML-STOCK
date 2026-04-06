package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

// FinancialReport 解析后的财务报告数据
type FinancialReport struct {
	Symbol      string                       `json:"symbol"`
	BalanceSheet map[string]map[string]float64 `json:"balanceSheet"` // 科目 -> 年份 -> 值
	IncomeStatement map[string]map[string]float64 `json:"incomeStatement"`
	CashFlow        map[string]map[string]float64 `json:"cashFlow"`
	Years           []string                      `json:"years"` // 可用年份列表（降序）
}

// ParseThsCSV 解析同花顺导出的 CSV 文件
func ParseThsCSV(filePath string) (map[string]map[string]float64, []string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("打开文件失败: %w", err)
	}
	defer f.Close()

	reader := csv.NewReader(f)
	reader.LazyQuotes = true
	records, err := reader.ReadAll()
	if err != nil {
		return nil, nil, fmt.Errorf("读取CSV失败: %w", err)
	}

	if len(records) < 3 {
		return nil, nil, fmt.Errorf("CSV数据行数不足")
	}

	// 找到真正的表头行（包含"科目\时间"）
	headerRowIdx := -1
	for i, row := range records {
		if len(row) > 0 && strings.Contains(row[0], "科目") && strings.Contains(row[0], "时间") {
			headerRowIdx = i
			break
		}
	}
	if headerRowIdx == -1 {
		return nil, nil, fmt.Errorf("未找到CSV表头行")
	}

	header := records[headerRowIdx]
	years := make([]string, 0, len(header)-1)
	for i := 1; i < len(header); i++ {
		year := strings.TrimSpace(header[i])
		if year != "" {
			years = append(years, year)
		}
	}

	data := make(map[string]map[string]float64)
	for i := headerRowIdx + 1; i < len(records); i++ {
		row := records[i]
		if len(row) == 0 {
			continue
		}
		itemName := strings.TrimSpace(row[0])
		if itemName == "" || strings.Contains(itemName, "报表") {
			continue // 跳过空行和分组标题行
		}

		// 标准化科目名
		stdName := normalizeItemName(itemName)
		if stdName == "" {
			stdName = itemName
		}

		yearData := make(map[string]float64)
		for j := 1; j < len(row) && j-1 < len(years); j++ {
			valStr := strings.TrimSpace(row[j])
			if valStr == "" || valStr == "--" {
				continue
			}
			val, err := strconv.ParseFloat(valStr, 64)
			if err != nil {
				continue // 忽略无法解析的数值
			}
			yearData[years[j-1]] = val
		}
		if len(yearData) > 0 {
			data[stdName] = yearData
		}
	}

	return data, years, nil
}

// normalizeItemName 将同花顺科目名标准化
func normalizeItemName(raw string) string {
	// 去掉首尾装饰字符和"(元)"后缀
	s := strings.TrimSpace(raw)
	s = strings.TrimPrefix(s, "*")
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "(元)")
	s = strings.TrimSuffix(s, "")
	s = strings.TrimSpace(s)

	// 映射表：同花顺原始名 -> 标准名
	aliases := map[string]string{
		// 资产负债表
		"资产合计": "资产合计",
		"负债合计": "负债合计",
		"所有者权益（或股东权益）合计": "所有者权益合计",
		"归属于母公司所有者权益合计": "归母所有者权益合计",
		"货币资金": "货币资金",
		"交易性金融资产": "交易性金融资产",
		"应收票据及应收账款": "应收票据及应收账款",
		"应收账款": "应收账款",
		"预付款项": "预付款项",
		"存货": "存货",
		"其他流动资产": "其他流动资产",
		"流动资产合计": "流动资产合计",
		"固定资产": "固定资产",
		"固定资产合计": "固定资产合计",
		"在建工程": "在建工程",
		"在建工程合计": "在建工程合计",
		"工程物资": "工程物资",
		"无形资产": "无形资产",
		"商誉": "商誉",
		"长期股权投资": "长期股权投资",
		"其他权益工具投资": "其他权益工具投资",
		"其他非流动金融资产": "其他非流动金融资产",
		"可供出售金融资产": "可供出售金融资产",
		"持有至到期投资": "持有至到期投资",
		"投资性房地产": "投资性房地产",
		"非流动资产合计": "非流动资产合计",
		"短期借款": "短期借款",
		"一年内到期的非流动负债": "一年内到期的非流动负债",
		"长期借款": "长期借款",
		"应付债券": "应付债券",
		"长期应付款": "长期应付款",
		"长期应付款合计": "长期应付款合计",
		"应付票据及应付账款": "应付票据及应付账款",
		"应付账款": "应付账款",
		"预收款项": "预收款项",
		"合同负债": "合同负债",
		"应付职工薪酬": "应付职工薪酬",
		"应交税费": "应交税费",
		"其他应付款合计": "其他应付款合计",
		"其他应付款": "其他应付款",
		"流动负债合计": "流动负债合计",
		"非流动负债合计": "非流动负债合计",
		"递延所得税资产": "递延所得税资产",
		"递延所得税负债": "递延所得税负债",
		"实收资本（或股本）": "实收资本",
		"资本公积": "资本公积",
		"盈余公积": "盈余公积",
		"未分配利润": "未分配利润",

		// 利润表
		"营业总收入": "营业总收入",
		"营业收入": "营业收入",
		"营业总成本": "营业总成本",
		"营业成本": "营业成本",
		"营业税金及附加": "税金及附加",
		"销售费用": "销售费用",
		"管理费用": "管理费用",
		"研发费用": "研发费用",
		"财务费用": "财务费用",
		"其中：利息费用": "利息费用",
		"利息收入": "利息收入",
		"资产减值损失": "资产减值损失",
		"信用减值损失": "信用减值损失",
		"投资收益": "投资收益",
		"其中：联营企业和合营企业的投资收益": "对联营企业和合营企业的投资收益",
		"公允价值变动收益": "公允价值变动收益",
		"资产处置收益": "资产处置收益",
		"其他收益": "其他收益",
		"营业利润": "营业利润",
		"三、营业利润": "营业利润",
		"营业外收入": "营业外收入",
		"营业外支出": "营业外支出",
		"利润总额": "利润总额",
		"所得税费用": "所得税费用",
		"净利润": "净利润",
		"归属于母公司所有者的净利润": "归母净利润",
		"少数股东损益": "少数股东损益",
		"扣除非经常性损益后的净利润": "扣非净利润",
		"基本每股收益": "基本每股收益",
		"稀释每股收益": "稀释每股收益",
		"其他综合收益": "其他综合收益",
		"综合收益总额": "综合收益总额",

		// 现金流量表
		"现金及现金等价物净增加额": "现金及现金等价物净增加额",
		"经营活动产生的现金流量净额": "经营活动现金流量净额",
		"投资活动产生的现金流量净额": "投资活动现金流量净额",
		"筹资活动产生的现金流量净额": "筹资活动现金流量净额",
		"期末现金及现金等价物余额": "期末现金及现金等价物余额",
		"期初现金及现金等价物余额": "期初现金及现金等价物余额",
		"销售商品、提供劳务收到的现金": "销售商品提供劳务收到的现金",
		"购买商品、接受劳务支付的现金": "购买商品接受劳务支付的现金",
		"支付给职工以及为职工支付的现金": "支付给职工以及为职工支付的现金",
		"支付的各项税费": "支付的各项税费",
		"支付其他与经营活动有关的现金": "支付其他与经营活动有关的现金",
		"收回投资收到的现金": "收回投资收到的现金",
		"取得投资收益收到的现金": "取得投资收益收到的现金",
		"处置固定资产、无形资产和其他长期资产收回的现金净额": "处置固定资产无形资产和其他长期资产收回的现金净额",
		"购建固定资产、无形资产和其他长期资产支付的现金": "购建固定资产无形资产和其他长期资产支付的现金",
		"投资支付的现金": "投资支付的现金",
		"取得子公司及其他营业单位支付的现金净额": "取得子公司及其他营业单位支付的现金净额",
		"吸收投资收到的现金": "吸收投资收到的现金",
		"取得借款收到的现金": "取得借款收到的现金",
		"偿还债务支付的现金": "偿还债务支付的现金",
		"分配股利、利润或偿付利息支付的现金": "分配股利利润或偿付利息支付的现金",
		"其中：子公司支付给少数股东的股利、利润": "子公司支付给少数股东的股利利润",
		"支付其他与筹资活动有关的现金": "支付其他与筹资活动有关的现金",
		"汇率变动对现金及现金等价物的影响": "汇率变动对现金及现金等价物的影响",
	}

	if alias, ok := aliases[s]; ok {
		return alias
	}
	return s
}

// GetReportYearList 从解析后的数据中提取可用年份列表（降序）
func GetReportYearList(data map[string]map[string]float64) []string {
	yearMap := make(map[string]bool)
	for _, yearData := range data {
		for year := range yearData {
			yearMap[year] = true
		}
	}
	var years []string
	for year := range yearMap {
		years = append(years, year)
	}
	// 简单冒泡降序排序
	for i := 0; i < len(years); i++ {
		for j := i + 1; j < len(years); j++ {
			if years[i] < years[j] {
				years[i], years[j] = years[j], years[i]
			}
		}
	}
	return years
}

// GetValue 安全获取某个科目某年的值，如果不存在返回 0 和 false
func GetValue(data map[string]map[string]float64, item string, year string) (float64, bool) {
	if yearData, ok := data[item]; ok {
		if val, ok := yearData[year]; ok {
			return val, true
		}
	}
	return 0, false
}

// ParseThsExcel 解析同花顺导出的 Excel 文件
func ParseThsExcel(filePath string) (map[string]map[string]float64, []string, error) {
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("打开Excel失败: %w", err)
	}
	defer f.Close()

	sheetName := f.GetSheetName(0)
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, nil, fmt.Errorf("读取Excel失败: %w", err)
	}
	if len(rows) < 3 {
		return nil, nil, fmt.Errorf("Excel数据行数不足")
	}

	headerRowIdx := -1
	for i, row := range rows {
		if len(row) > 0 && strings.Contains(row[0], "科目") && strings.Contains(row[0], "时间") {
			headerRowIdx = i
			break
		}
	}
	if headerRowIdx == -1 {
		return nil, nil, fmt.Errorf("未找到Excel表头行")
	}

	header := rows[headerRowIdx]
	years := make([]string, 0, len(header)-1)
	for i := 1; i < len(header); i++ {
		year := strings.TrimSpace(header[i])
		if year != "" {
			years = append(years, year)
		}
	}

	data := make(map[string]map[string]float64)
	for i := headerRowIdx + 1; i < len(rows); i++ {
		row := rows[i]
		if len(row) == 0 {
			continue
		}
		itemName := strings.TrimSpace(row[0])
		if itemName == "" || strings.Contains(itemName, "报表") {
			continue
		}

		stdName := normalizeItemName(itemName)
		if stdName == "" {
			stdName = itemName
		}

		yearData := make(map[string]float64)
		for j := 1; j < len(row) && j-1 < len(years); j++ {
			valStr := strings.TrimSpace(row[j])
			if valStr == "" || valStr == "--" {
				continue
			}
			val, err := strconv.ParseFloat(valStr, 64)
			if err != nil {
				continue
			}
			yearData[years[j-1]] = val
		}
		if len(yearData) > 0 {
			data[stdName] = yearData
		}
	}

	return data, years, nil
}

// detectReportTypeByContent 通过文件内容判断报表类型（支持 CSV 和 Excel）
func detectReportTypeByContent(filePath string) (string, error) {
	ext := strings.ToLower(filepath.Ext(filePath))
	var rows [][]string

	if ext == ".xlsx" {
		f, err := excelize.OpenFile(filePath)
		if err != nil {
			return "", err
		}
		defer f.Close()
		sheetName := f.GetSheetName(0)
		rows, err = f.GetRows(sheetName)
		if err != nil {
			return "", err
		}
	} else {
		f, err := os.Open(filePath)
		if err != nil {
			return "", err
		}
		defer f.Close()
		reader := csv.NewReader(f)
		reader.LazyQuotes = true
		rows, err = reader.ReadAll()
		if err != nil {
			return "", err
		}
	}

	if len(rows) < 4 {
		return "", fmt.Errorf("数据行数不足")
	}

	headerIdx := -1
	for i, row := range rows {
		if len(row) > 0 && strings.Contains(row[0], "科目") {
			headerIdx = i
			break
		}
	}
	if headerIdx == -1 || headerIdx+1 >= len(rows) {
		return "", fmt.Errorf("未找到表头行")
	}

	keywords := make(map[string]int)
	for i := headerIdx + 1; i < len(rows) && i < headerIdx+15; i++ {
		if len(rows[i]) > 0 {
			k := strings.ToLower(rows[i][0])
			keywords[k]++
		}
	}

	for k := range keywords {
		if strings.Contains(k, "净利润") || strings.Contains(k, "营业收入") || strings.Contains(k, "营业成本") || strings.Contains(k, "营业利润") || strings.Contains(k, "利润总额") {
			return "income", nil
		}
		if strings.Contains(k, "现金流量") || strings.Contains(k, "现金及现金等价物") || strings.Contains(k, "经营活动产生") {
			return "cash", nil
		}
		if strings.Contains(k, "资产合计") || strings.Contains(k, "负债合计") || strings.Contains(k, "所有者权益") || strings.Contains(k, "股东权益") {
			return "balance", nil
		}
	}
	return "", fmt.Errorf("无法识别报表类型")
}
