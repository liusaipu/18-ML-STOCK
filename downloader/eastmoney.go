package downloader

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

const (
	hsf10BaseURL = "https://emweb.securities.eastmoney.com/PC_HSF10/NewFinanceAnalysis"
	dcBaseURL    = "https://datacenter-web.eastmoney.com/api/data/v1/get"
)

// FinancialReportData 下载后的三张表数据
type FinancialReportData struct {
	Symbol          string
	Years           []string
	BalanceSheet    map[string]map[string]float64
	IncomeStatement map[string]map[string]float64
	CashFlow        map[string]map[string]float64
}

// DownloadFinancialReports 从东方财富下载指定股票的所有年报财务数据
func DownloadFinancialReports(market, code string) (*FinancialReportData, error) {
	if strings.ToUpper(market) == "HK" {
		return nil, fmt.Errorf("港股财报暂不支持网络下载，请手动导入 CSV")
	}
	fullCode := market + code // e.g. SH603501

	// 1. 分别为三张表确定 companyType（保险/金融股各表可能不同）
	bsCT, err := getCompanyTypeForEndpoint("zcfzbAjaxNew", fullCode)
	if err != nil {
		return nil, fmt.Errorf("无法确定资产负债表companyType: %w", err)
	}
	isCT, err := getCompanyTypeForEndpoint("lrbAjaxNew", fullCode)
	if err != nil {
		return nil, fmt.Errorf("无法确定利润表companyType: %w", err)
	}
	cfCT, err := getCompanyTypeForEndpoint("xjllbAjaxNew", fullCode)
	if err != nil {
		return nil, fmt.Errorf("无法确定现金流量表companyType: %w", err)
	}

	// 2. 获取年报日期列表
	dates, err := getAnnualReportDates(code)
	if err != nil {
		return nil, fmt.Errorf("获取年报日期列表失败: %w", err)
	}
	if len(dates) == 0 {
		return nil, fmt.Errorf("未找到任何年报数据")
	}

	// 3. 并发下载三张表
	result := &FinancialReportData{
		Symbol:          code,
		Years:           dates,
		BalanceSheet:    make(map[string]map[string]float64),
		IncomeStatement: make(map[string]map[string]float64),
		CashFlow:        make(map[string]map[string]float64),
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var downloadErr error

	for _, date := range dates {
		// balance sheet
		wg.Add(1)
		go func(d string) {
			defer wg.Done()
			data, err := fetchHSF10("zcfzbAjaxNew", bsCT, d, fullCode)
			if err != nil {
				mu.Lock()
				if downloadErr == nil {
					downloadErr = err
				}
				mu.Unlock()
				return
			}
			mu.Lock()
			mergeBalanceSheet(result.BalanceSheet, data, d)
			mu.Unlock()
		}(date)

		// income statement
		wg.Add(1)
		go func(d string) {
			defer wg.Done()
			data, err := fetchHSF10("lrbAjaxNew", isCT, d, fullCode)
			if err != nil {
				mu.Lock()
				if downloadErr == nil {
					downloadErr = err
				}
				mu.Unlock()
				return
			}
			mu.Lock()
			mergeIncomeStatement(result.IncomeStatement, data, d)
			mu.Unlock()
		}(date)

		// cash flow
		wg.Add(1)
		go func(d string) {
			defer wg.Done()
			data, err := fetchHSF10("xjllbAjaxNew", cfCT, d, fullCode)
			if err != nil {
				mu.Lock()
				if downloadErr == nil {
					downloadErr = err
				}
				mu.Unlock()
				return
			}
			mu.Lock()
			mergeCashFlow(result.CashFlow, data, d)
			mu.Unlock()
		}(date)
	}

	wg.Wait()
	if downloadErr != nil {
		return nil, downloadErr
	}

	return result, nil
}

// getCompanyTypeForEndpoint 尝试多个 companyType 直到指定 endpoint 返回有效数据
func getCompanyTypeForEndpoint(endpoint, fullCode string) (int, error) {
	for _, ct := range []int{4, 1, 2, 3, 5, 6, 7, 8} {
		data, err := fetchHSF10(endpoint, ct, "2024-12-31", fullCode)
		if err != nil {
			continue
		}
		if data != nil && len(data) > 0 {
			return ct, nil
		}
	}
	return 0, fmt.Errorf("未找到匹配的companyType")
}

// getAnnualReportDates 通过 datacenter-web API 获取所有年报日期
func getAnnualReportDates(code string) ([]string, error) {
	url := fmt.Sprintf("%s?sortColumns=REPORT_DATE&sortTypes=-1&pageSize=500&pageNumber=1&reportName=RPT_DMSK_FN_BALANCE&columns=REPORT_DATE&source=WEB&filter=(SECURITY_CODE=\"%s\")", dcBaseURL, code)
	url = strings.ReplaceAll(url, `"`, "%22")
	resp, err := httpGetWithReferer(url, "https://data.eastmoney.com/")
	if err != nil {
		return nil, err
	}

	var result dcResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}
	if !result.Success || result.Result == nil {
		return nil, fmt.Errorf("datacenter-web API 返回失败")
	}

	dateSet := make(map[string]struct{})
	for _, item := range result.Result.Data {
		dateStr, ok := item["REPORT_DATE"].(string)
		if !ok {
			continue
		}
		dateStr = strings.TrimSpace(dateStr)
		if strings.Contains(dateStr, "12-31") {
			// 提取 yyyy-mm-dd 部分
			parts := strings.Fields(dateStr)
			if len(parts) > 0 {
				dateSet[parts[0]] = struct{}{}
			}
		}
	}

	dates := make([]string, 0, len(dateSet))
	for d := range dateSet {
		dates = append(dates, d)
	}
	// 降序排序
	for i := 0; i < len(dates); i++ {
		for j := i + 1; j < len(dates); j++ {
			if dates[i] < dates[j] {
				dates[i], dates[j] = dates[j], dates[i]
			}
		}
	}
	// 只保留最近5年
	if len(dates) > 5 {
		dates = dates[:5]
	}
	return dates, nil
}

func fetchHSF10(endpoint string, companyType int, date, fullCode string) (map[string]any, error) {
	url := fmt.Sprintf("%s/%s?companyType=%d&reportDateType=0&reportType=1&dates=%s&code=%s", hsf10BaseURL, endpoint, companyType, date, fullCode)
	resp, err := httpGetWithReferer(url, "https://emweb.securities.eastmoney.com/")
	if err != nil {
		return nil, err
	}

	var hsr hsf10Response
	if err := json.Unmarshal(resp, &hsr); err != nil {
		return nil, err
	}
	if hsr.Data == nil || len(hsr.Data) == 0 {
		return nil, fmt.Errorf("no data")
	}
	return hsr.Data[0], nil
}

func httpGetWithReferer(url, referer string) ([]byte, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")
	req.Header.Set("Referer", referer)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	// 去除可能的 UTF-8 BOM
	if len(body) >= 3 && body[0] == 0xEF && body[1] == 0xBB && body[2] == 0xBF {
		body = body[3:]
	}
	return body, nil
}

type dcResponse struct {
	Success bool `json:"success"`
	Result  *struct {
		Data []map[string]any `json:"data"`
	} `json:"result"`
}

type hsf10Response struct {
	Pages int              `json:"pages"`
	Count int              `json:"count"`
	Data  []map[string]any `json:"data"`
}

// StockProfile 股票基本资料
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
	PoliticalAffiliation string  `json:"politicalAffiliation"` // blue/green/空
}

// StockQuote 股票实时行情
type StockQuote struct {
	CurrentPrice         float64 `json:"currentPrice"`
	ChangePercent        float64 `json:"changePercent"`
	ChangeAmount         float64 `json:"changeAmount"`
	Volume               float64 `json:"volume"`
	TurnoverAmount       float64 `json:"turnoverAmount"`
	TurnoverRate         float64 `json:"turnoverRate"`
	Amplitude            float64 `json:"amplitude"`
	High                 float64 `json:"high"`
	Low                  float64 `json:"low"`
	Open                 float64 `json:"open"`
	PreviousClose        float64 `json:"previousClose"`
	CirculatingMarketCap float64 `json:"circulatingMarketCap"`
	VolumeRatio          float64 `json:"volumeRatio"`
	PE                   float64 `json:"pe"`
	PB                   float64 `json:"pb"`
	MarketCap            float64 `json:"marketCap"`
	QuoteTime            string  `json:"quoteTime"`
}

// FetchStockProfile 获取股票基本资料（东方财富公司概况 + 腾讯行情补充数值）
func FetchStockProfile(market, code string) (*StockProfile, error) {
	profile := &StockProfile{}

	// 数据源1：HSF10 公司概况（行业、董事长、实控人、上市日期）
	csCode := toCsCode(market, code)
	csURL := fmt.Sprintf("https://emweb.securities.eastmoney.com/PC_HSF10/CompanySurvey/CompanySurveyAjax?code=%s", csCode)
	if csBody, csErr := httpGetWithReferer(csURL, "https://emweb.securities.eastmoney.com/"); csErr == nil {
		var csResp struct {
			Jbzl struct {
				Sshy string `json:"sshy"`
				Zjl  string `json:"zjl"`
				Dsz  string `json:"dsz"`
				Gsjj string `json:"gsjj"`
			} `json:"jbzl"`
			Fxxg struct {
				Ssrq string `json:"ssrq"`
			} `json:"fxxg"`
		}
		if json.Unmarshal(csBody, &csResp); csResp.Jbzl.Sshy != "" {
			profile.Industry = csResp.Jbzl.Sshy
		}
		if csResp.Jbzl.Dsz != "" {
			profile.Chairman = csResp.Jbzl.Dsz
		} else if csResp.Jbzl.Zjl != "" {
			profile.Chairman = csResp.Jbzl.Zjl
		}
		if csResp.Jbzl.Gsjj != "" {
			profile.Controller = extractController(csResp.Jbzl.Gsjj)
		}
		if csResp.Fxxg.Ssrq != "" {
			profile.ListingDate = csResp.Fxxg.Ssrq
		}
	}

	// 数据源2：高管管理信息（国籍）
	mgmtURL := fmt.Sprintf("https://emweb.securities.eastmoney.com/PC_HSF10/CompanyManagement/CompanyManagementAjax?code=%s", csCode)
	if mgmtBody, mgmtErr := httpGetWithReferer(mgmtURL, "https://emweb.securities.eastmoney.com/"); mgmtErr == nil {
		var mgmtResp struct {
			RptManagerList []struct {
				XM string `json:"xm"`
				ZW string `json:"zw"`
				JJ string `json:"jj"`
			} `json:"RptManagerList"`
		}
		if json.Unmarshal(mgmtBody, &mgmtResp); len(mgmtResp.RptManagerList) > 0 {
			// 需要匹配的人名：优先实控人，其次董事长/总经理
			targetName := profile.Controller
			if targetName == "" {
				targetName = profile.Chairman
			}
			for _, m := range mgmtResp.RptManagerList {
				if m.XM == targetName && m.JJ != "" {
					profile.ChairmanNationality = extractNationality(m.JJ)
					profile.PoliticalAffiliation = extractPoliticalAffiliation(m.JJ)
					break
				}
			}
			// 如果按姓名没匹配到，再按职务匹配（董事长/总经理）
			if profile.ChairmanNationality == "" {
				for _, m := range mgmtResp.RptManagerList {
					if strings.Contains(m.ZW, "董事长") || strings.Contains(m.ZW, "总经理") || strings.Contains(m.ZW, "法定代表人") {
						if m.JJ != "" {
							nat := extractNationality(m.JJ)
							if nat != "" {
								profile.ChairmanNationality = nat
								profile.PoliticalAffiliation = extractPoliticalAffiliation(m.JJ)
								break
							}
						}
					}
				}
			}
			// 若已判定为中国台湾但蓝绿属性仍为空，尝试内置映射表及百度百科推断
			if profile.ChairmanNationality == "中国台湾" && profile.PoliticalAffiliation == "" && targetName != "" {
				if pa, ok := knownTaiwanPoliticalMap[targetName]; ok && pa != "" {
					profile.PoliticalAffiliation = pa
				} else {
					profile.PoliticalAffiliation = inferPoliticalAffiliationFromBaike(targetName)
				}
			}
			// 若籍属仍为空，尝试从百度百科查找
			if profile.ChairmanNationality == "" && targetName != "" {
				profile.ChairmanNationality = inferNationalityFromBaike(targetName)
			}
		}
	}

	// 数据源3：腾讯行情接口补充市值、PE、PB 等数值字段
	if q, err := fetchQuoteFromTencent(market, code); err == nil && q != nil {
		if profile.MarketCap == 0 && q.MarketCap > 0 {
			profile.MarketCap = q.MarketCap
		}
		if profile.PE == 0 && q.PE > 0 {
			profile.PE = q.PE
		}
		if profile.PB == 0 && q.PB > 0 {
			profile.PB = q.PB
		}
		// 成交量字段作为总股本近似（腾讯接口 parts[55]/[56] 为股本，但通用性不强，暂用成交量字段占位）
		// 若后续有稳定总股本接口，可再替换
	}

	// 港股 fallback：若 HSF10 无数据，尝试 akshare
	if strings.ToUpper(market) == "HK" && profile.Industry == "" && profile.ListingDate == "" && profile.Chairman == "" {
		if hkProfile, err := fetchHKProfileFromPython(code); err == nil && hkProfile != nil {
			if profile.Industry == "" {
				profile.Industry = hkProfile.Industry
			}
			if profile.Chairman == "" {
				profile.Chairman = hkProfile.Chairman
			}
			if profile.ListingDate == "" {
				profile.ListingDate = hkProfile.ListingDate
			}
		}
	}

	// 只要有至少一项核心数据，即视为成功
	if profile.Industry != "" || profile.ListingDate != "" || profile.MarketCap > 0 || profile.PE > 0 || profile.Chairman != "" {
		return profile, nil
	}

	return nil, fmt.Errorf("未获取到股票资料数据")
}

func extractController(gsjj string) string {
	// 匹配 "控股股东为XXX" 或 "实控人为XXX"
	patterns := []string{"控股股东为([^，。；、]+)", "实际控制人为([^，。；、]+)", "实控人为([^，。；、]+)", "控股股东是([^，。；、]+)", "由([^，。；、]+)作为主发起人"}
	for _, p := range patterns {
		re := regexp.MustCompile(p)
		if m := re.FindStringSubmatch(gsjj); len(m) > 1 && strings.TrimSpace(m[1]) != "" {
			return strings.TrimSpace(m[1])
		}
	}
	return ""
}

func extractNationality(jj string) string {
	jj = strings.ReplaceAll(jj, " ", "")
	// 港澳台优先
	if strings.Contains(jj, "中国香港") || strings.Contains(jj, "香港居民") || strings.Contains(jj, "香港永久性居民") {
		return "中国香港"
	}
	if strings.Contains(jj, "中国澳门") || strings.Contains(jj, "澳门居民") || strings.Contains(jj, "澳门永久性居民") {
		return "中国澳门"
	}
	if strings.Contains(jj, "中国台湾") || strings.Contains(jj, "台湾居民") {
		return "中国台湾"
	}
	if strings.Contains(jj, "台湾籍") || strings.Contains(jj, "台湾人") || strings.Contains(jj, "出生于台湾") {
		return "中国台湾"
	}
	// 若简介中出现“台湾”字样（如台湾公司、台湾大学等），优先判定为中国台湾
	if strings.Contains(jj, "台湾") {
		return "中国台湾"
	}
	// 常见外籍
	if strings.Contains(jj, "美国国籍") || strings.Contains(jj, "美籍") {
		return "美国"
	}
	if strings.Contains(jj, "加拿大国籍") || strings.Contains(jj, "加拿大籍") {
		return "加拿大"
	}
	if strings.Contains(jj, "新加坡") {
		return "新加坡"
	}
	if strings.Contains(jj, "英国") {
		return "英国"
	}
	if strings.Contains(jj, "日本") {
		return "日本"
	}
	if strings.Contains(jj, "澳大利亚") {
		return "澳大利亚"
	}
	if strings.Contains(jj, "德国") {
		return "德国"
	}
	// 默认中国（东方财富接口常返回“中国籍”而非“中国国籍”）
	if strings.Contains(jj, "中国国籍") || strings.Contains(jj, "中国籍") || strings.Contains(jj, "中国公民") ||
		strings.Contains(jj, "无境外永久居留权") || strings.Contains(jj, "无永久境外居留权") {
		return "中国"
	}
	// 兜底正则：仅匹配连续汉字（避免跨越英文标点）
	re := regexp.MustCompile(`([\p{Han}]{1,10})国籍`)
	if m := re.FindStringSubmatch(jj); len(m) > 1 {
		n := strings.TrimSpace(m[1])
		if n != "" && n != "先生" && n != "女士" && n != "男" && n != "女" {
			return n
		}
	}
	return ""
}

func extractPoliticalAffiliation(jj string) string {
	jj = strings.ReplaceAll(jj, " ", "")
	green := []string{"民进党", "民主进步党", "台湾团结联盟", "台联", "时代力量", "台湾基进", "绿营"}
	blue := []string{"国民党", "中国国民党", "亲民党", "新党", "统促党", "蓝营"}
	for _, k := range green {
		if strings.Contains(jj, k) {
			return "green"
		}
	}
	for _, k := range blue {
		if strings.Contains(jj, k) {
			return "blue"
		}
	}
	return ""
}

// 已知台湾籍企业家的蓝绿属性映射表（程序内置兜底）
var knownTaiwanPoliticalMap = map[string]string{
	"郭台铭": "blue",
	"王永庆": "blue",
	"王文洋": "blue",
	"严凯泰": "blue",
}

// inferPoliticalAffiliationFromBaike 尝试从百度百科页面推断人物蓝绿属性
func inferPoliticalAffiliationFromBaike(name string) string {
	if name == "" {
		return ""
	}
	baikeURL := fmt.Sprintf("https://baike.baidu.com/item/%s", url.QueryEscape(name))
	body, err := httpGetWithReferer(baikeURL, "https://baike.baidu.com/")
	if err != nil {
		return ""
	}
	text := string(body)
	// 去掉 HTML 标签，只保留文本便于正则匹配
	reTag := regexp.MustCompile(`<[^>]+>`)
	text = reTag.ReplaceAllString(text, " ")
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "\t", " ")

	green := []string{"民进党", "民主进步党", "绿营", "泛绿", "深绿"}
	blue := []string{"国民党", "中国国民党", "蓝营", "泛蓝", "深蓝"}

	for _, k := range green {
		pat := regexp.MustCompile(regexp.QuoteMeta(name) + `.{0,40}` + regexp.QuoteMeta(k) + `|` + regexp.QuoteMeta(k) + `.{0,40}` + regexp.QuoteMeta(name))
		if pat.MatchString(text) {
			return "green"
		}
	}
	for _, k := range blue {
		pat := regexp.MustCompile(regexp.QuoteMeta(name) + `.{0,40}` + regexp.QuoteMeta(k) + `|` + regexp.QuoteMeta(k) + `.{0,40}` + regexp.QuoteMeta(name))
		if pat.MatchString(text) {
			return "blue"
		}
	}
	return ""
}

// inferNationalityFromBaike 尝试从百度百科页面推断人物国籍/籍属
func inferNationalityFromBaike(name string) string {
	if name == "" {
		return ""
	}
	baikeURL := fmt.Sprintf("https://baike.baidu.com/item/%s", url.QueryEscape(name))
	body, err := httpGetWithReferer(baikeURL, "https://baike.baidu.com/")
	if err != nil {
		return ""
	}
	text := string(body)
	// 去掉 HTML 标签，只保留文本便于正则匹配
	reTag := regexp.MustCompile(`<[^>]+>`)
	text = reTag.ReplaceAllString(text, " ")
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "\t", " ")

	// 【第一步】优先从明确的"国籍"字段提取（最准确）
	// 匹配 国籍 中国 / 国籍：中国 / 国籍,中国 等格式
	re := regexp.MustCompile(`国籍[\s,，:：]*([\p{Han}]{1,10})(?:\s|$|，|,)`)
	if m := re.FindStringSubmatch(text); len(m) > 1 {
		n := strings.TrimSpace(m[1])
		if n != "" && n != "未知" && n != "不详" && n != "暂无" {
			return n
		}
	}

	// 【第二步】检查港澳台（要求明确的"台湾籍/香港籍"或"台湾居民"等）
	if strings.Contains(text, "中国香港") || strings.Contains(text, "香港居民") || strings.Contains(text, "香港籍") {
		return "中国香港"
	}
	if strings.Contains(text, "中国澳门") || strings.Contains(text, "澳门居民") || strings.Contains(text, "澳门籍") {
		return "中国澳门"
	}
	// 台湾：必须是"台湾籍"、"台湾居民"或"中国台湾"，不能只是"台湾"（避免"东芝家电"等误匹配）
	if strings.Contains(text, "中国台湾") || strings.Contains(text, "台湾居民") || strings.Contains(text, "台湾籍") {
		return "中国台湾"
	}
	// "出生于台湾"或"籍贯台湾"才算
	if strings.Contains(text, "出生于台湾") || strings.Contains(text, "籍贯台湾") {
		return "中国台湾"
	}

	// 【第三步】常见外籍
	if strings.Contains(text, "美国国籍") || strings.Contains(text, "美籍") || strings.Contains(text, "美国籍") {
		return "美国"
	}
	if strings.Contains(text, "加拿大国籍") || strings.Contains(text, "加拿大籍") {
		return "加拿大"
	}
	if strings.Contains(text, "新加坡国籍") || strings.Contains(text, "新加坡籍") {
		return "新加坡"
	}
	if strings.Contains(text, "英国国籍") || strings.Contains(text, "英国籍") {
		return "英国"
	}
	if strings.Contains(text, "日本国籍") || strings.Contains(text, "日本籍") {
		return "日本"
	}
	if strings.Contains(text, "澳大利亚国籍") || strings.Contains(text, "澳大利亚籍") {
		return "澳大利亚"
	}
	if strings.Contains(text, "德国国籍") || strings.Contains(text, "德国籍") {
		return "德国"
	}
	if strings.Contains(text, "法国国籍") || strings.Contains(text, "法国籍") {
		return "法国"
	}

	// 【第四步】中国籍（兜底）
	if strings.Contains(text, "中国国籍") || strings.Contains(text, "中国籍") || strings.Contains(text, "中国公民") {
		return "中国"
	}

	return ""
}

// FetchStockQuote 从东方财富获取股票实时行情，若失败则 fallback 到腾讯财经接口
func FetchStockQuote(market, code string) (*StockQuote, error) {
	var secid string
	switch strings.ToUpper(market) {
	case "SH":
		secid = "1." + code
	case "SZ":
		secid = "0." + code
	case "HK":
		secid = "116." + code
	default:
		secid = "0." + code
	}
	url := fmt.Sprintf("http://push2.eastmoney.com/api/qt/stock/get?secid=%s&fields=f2,f3,f4,f5,f6,f7,f8,f10,f15,f16,f17,f18,f20,f21,f9,f23", secid)
	body, err := httpGetWithReferer(url, "https://quote.eastmoney.com/")
	if err == nil {
		var resp struct {
			Data map[string]any `json:"data"`
		}
		if json.Unmarshal(body, &resp) == nil && resp.Data != nil {
			quote := &StockQuote{}
			quote.CurrentPrice = parseAnyFloat(resp.Data["f2"])
			quote.ChangePercent = parseAnyFloat(resp.Data["f3"])
			quote.ChangeAmount = parseAnyFloat(resp.Data["f4"])
			quote.Volume = parseAnyFloat(resp.Data["f5"])
			quote.TurnoverAmount = parseAnyFloat(resp.Data["f6"])
			quote.Amplitude = parseAnyFloat(resp.Data["f7"])
			quote.TurnoverRate = parseAnyFloat(resp.Data["f8"])
			quote.VolumeRatio = parseAnyFloat(resp.Data["f10"])
			quote.High = parseAnyFloat(resp.Data["f15"])
			quote.Low = parseAnyFloat(resp.Data["f16"])
			quote.Open = parseAnyFloat(resp.Data["f17"])
			quote.PreviousClose = parseAnyFloat(resp.Data["f18"])
			quote.MarketCap = parseAnyFloat(resp.Data["f20"])
			quote.CirculatingMarketCap = parseAnyFloat(resp.Data["f21"])
			quote.PE = parseAnyFloat(resp.Data["f9"])
			quote.PB = parseAnyFloat(resp.Data["f23"])
			if quote.CurrentPrice > 0 {
				return quote, nil
			}
		}
	}

	// fallback 到腾讯财经接口
	return fetchQuoteFromTencent(market, code)
}

func fetchQuoteFromTencent(market, code string) (*StockQuote, error) {
	prefix := strings.ToLower(market)
	url := fmt.Sprintf("http://qt.gtimg.cn/q=%s%s", prefix, code)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	// 腾讯接口返回 GB18030 编码
	utf8Body, err := decodeGB18030(body)
	if err != nil {
		return nil, err
	}

	text := strings.TrimSpace(string(utf8Body))
	// 格式：v_sh603501="1~豪威集团~603501~90.15~91.35~...";
	start := strings.Index(text, "=\"")
	end := strings.LastIndex(text, "\";")
	if start == -1 || end == -1 || start+2 >= end {
		return nil, fmt.Errorf("无法解析腾讯行情数据")
	}
	content := text[start+2 : end]
	parts := strings.Split(content, "~")
	if len(parts) < 47 {
		return nil, fmt.Errorf("腾讯行情数据字段不足")
	}

	isHK := strings.ToUpper(market) == "HK"

	quote := &StockQuote{}
	quote.CurrentPrice = parseStrFloat(parts[3])
	quote.PreviousClose = parseStrFloat(parts[4])
	quote.Open = parseStrFloat(parts[5])
	quote.Volume = parseStrFloat(parts[6])
	quote.ChangeAmount = parseStrFloat(parts[31])
	quote.ChangePercent = parseStrFloat(parts[32])
	quote.High = parseStrFloat(parts[33])
	quote.Low = parseStrFloat(parts[34])
	if isHK {
		// 港股：parts[37] 直接是成交额
		quote.TurnoverAmount = parseStrFloat(parts[37])
	} else {
		// A股：成交额在 parts[35] 中，格式：当前价/成交量/成交额（单位：元）
		if len(parts) > 35 {
			triplet := strings.Split(parts[35], "/")
			if len(triplet) >= 3 {
				quote.TurnoverAmount = parseStrFloat(triplet[2])
			}
		}
	}
	quote.TurnoverRate = parseStrFloat(parts[38])
	quote.PE = parseStrFloat(parts[39])
	quote.Amplitude = parseStrFloat(parts[43])
	// 流通市值 / 总市值
	if len(parts) > 44 {
		quote.CirculatingMarketCap = parseStrFloat(parts[44]) * 1e8
	}
	if len(parts) > 45 {
		quote.MarketCap = parseStrFloat(parts[45]) * 1e8
	}
	// PB 字段：A股在 parts[46]，港股因为插入了英文名称，PB 在 parts[47]
	if isHK {
		if len(parts) > 47 {
			quote.PB = parseStrFloat(parts[47])
		}
	} else {
		if len(parts) > 46 {
			quote.PB = parseStrFloat(parts[46])
		}
	}
	if !isHK && len(parts) > 49 {
		quote.VolumeRatio = parseStrFloat(parts[49])
	}
	// 时间格式：A股为 YYYYMMDDHHMMSS(14位)，港股为 YYYY/MM/DD HH:MM:SS
	if len(parts) > 30 && parts[30] != "" {
		timeStr := parts[30]
		if len(timeStr) == 14 {
			quote.QuoteTime = fmt.Sprintf("%s-%s-%s %s:%s:%s",
				timeStr[0:4], timeStr[4:6], timeStr[6:8],
				timeStr[8:10], timeStr[10:12], timeStr[12:14])
		} else if len(timeStr) == 19 && timeStr[4] == '/' && timeStr[7] == '/' {
			// 港股格式 2026/04/10 16:09:25 -> 2026-04-10 16:09:25
			quote.QuoteTime = strings.Replace(timeStr, "/", "-", 2)
		}
	}

	if quote.CurrentPrice == 0 {
		return nil, fmt.Errorf("腾讯行情数据无效")
	}
	return quote, nil
}

func decodeGB18030(data []byte) ([]byte, error) {
	// 使用 golang.org/x/text/encoding/simplifiedchinese
	decoder := simplifiedchinese.GB18030.NewDecoder()
	return decoder.Bytes(data)
}

func toCsCode(market, code string) string {
	switch strings.ToUpper(market) {
	case "SH":
		return "SH" + code
	case "SZ":
		return "SZ" + code
	case "HK":
		return "HK" + code
	default:
		return "SZ" + code
	}
}

// hkProfileResult Python 脚本返回的港股资料
type hkProfileResult struct {
	Industry    string `json:"industry"`
	Chairman    string `json:"chairman"`
	ListingDate string `json:"listing_date"`
}

// fetchHKProfileScriptPath 返回 fetch_hk_profile.py 绝对路径
func fetchHKProfileScriptPath() string {
	_, b, _, _ := runtime.Caller(0)
	base := filepath.Dir(b)
	root := findProjectRootByMarker(base, filepath.Join("scripts", "fetch_hk_profile.py"))
	if root != "" {
		return filepath.Join(root, "scripts", "fetch_hk_profile.py")
	}
	return filepath.Join(base, "..", "scripts", "fetch_hk_profile.py")
}

// fetchHKProfileFromPython 调用 Python 脚本获取港股基本资料
func fetchHKProfileFromPython(code string) (*hkProfileResult, error) {
	script := fetchHKProfileScriptPath()
	python := resolvePythonExecutable()
	cmd := exec.Command(python, script, code)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("fetch_hk_profile.py 执行失败: %v, output: %s", err, string(output))
	}
	var result hkProfileResult
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("解析港股资料失败: %v, raw: %s", err, string(output))
	}
	return &result, nil
}

func parseStrFloat(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

func parseAnyFloat(v any) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case string:
		f, _ := strconv.ParseFloat(val, 64)
		return f
	}
	return 0
}

// KlineData 单根K线数据
type KlineData struct {
	Time   string  `json:"time"`
	Open   float64 `json:"open"`
	Close  float64 `json:"close"`
	Low    float64 `json:"low"`
	High   float64 `json:"high"`
	Volume float64 `json:"volume"`
	Amount float64 `json:"amount"` // 成交额（元）
}

// FetchStockKlines 获取股票历史K线数据（日K），先尝试东方财富，再腾讯财经，再网易财经
func FetchStockKlines(market, code string, limit int) ([]KlineData, error) {
	var secid string
	switch strings.ToUpper(market) {
	case "SH":
		secid = "1." + code
	case "SZ":
		secid = "0." + code
	case "HK":
		secid = "116." + code
	default:
		secid = "0." + code
	}

	// 1. 东方财富 HTTPS
	url := fmt.Sprintf("https://push2.eastmoney.com/api/qt/stock/kline/get?ut=fa5fd1943c7b386f172d6893dbfba10b&secid=%s&fields1=f1&fields2=f51,f52,f53,f54,f55,f56,f57,f58&klt=101&fqt=0&end=20500101&lmt=%d", secid, limit)
	body, err := httpGetWithReferer(url, "https://quote.eastmoney.com/")
	if err == nil {
		var resp struct {
			Data struct {
				Klines []string `json:"klines"`
			} `json:"data"`
		}
		if json.Unmarshal(body, &resp) == nil && len(resp.Data.Klines) > 0 {
			return parseEastMoneyKlines(resp.Data.Klines), nil
		}
	}

	// 2. 腾讯财经 HTTPS
	klines, err := fetchKlinesFromTencent(market, code, limit)
	if err == nil && len(klines) > 0 {
		return klines, nil
	}

	// 3. 网易财经 CSV
	return fetchKlinesFromNetEase(market, code, limit)
}

func parseEastMoneyKlines(lines []string) []KlineData {
	var result []KlineData
	for _, line := range lines {
		parts := strings.Split(line, ",")
		if len(parts) < 6 {
			continue
		}
		amount := 0.0
		if len(parts) > 6 {
			amount = parseStrFloat(parts[6])
		}
		result = append(result, KlineData{
			Time:   parts[0],
			Open:   parseStrFloat(parts[1]),
			Close:  parseStrFloat(parts[2]),
			Low:    parseStrFloat(parts[3]),
			High:   parseStrFloat(parts[4]),
			Volume: parseStrFloat(parts[5]),
			Amount: amount,
		})
	}
	return result
}

func fetchKlinesFromTencent(market, code string, limit int) ([]KlineData, error) {
	prefix := strings.ToLower(market)
	url := fmt.Sprintf("https://web.ifzq.gtimg.cn/appstock/app/fqkline/get?param=%s%s,day,,,%d,qfq", prefix, code, limit)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Code int `json:"code"`
		Data map[string]struct {
			QfqDay [][]any `json:"qfqday"`
			Day    [][]any `json:"day"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	key := prefix + code
	stockData, ok := result.Data[key]
	if !ok {
		return nil, fmt.Errorf("腾讯K线数据为空")
	}

	days := stockData.QfqDay
	if len(days) == 0 {
		days = stockData.Day
	}
	if len(days) == 0 {
		return nil, fmt.Errorf("腾讯K线数据为空")
	}

	var klines []KlineData
	for _, item := range days {
		if len(item) < 6 {
			continue
		}
		date, _ := item[0].(string)
		close := parseAnyFloat(item[2])
		volume := parseAnyFloat(item[5])
		amount := close * volume * 100 // 腾讯无成交额字段，用收盘价×成交量(手)×100 估算
		klines = append(klines, KlineData{
			Time:   date,
			Open:   parseAnyFloat(item[1]),
			Close:  close,
			High:   parseAnyFloat(item[3]),
			Low:    parseAnyFloat(item[4]),
			Volume: volume,
			Amount: amount,
		})
	}
	return klines, nil
}

func fetchKlinesFromNetEase(market, code string, limit int) ([]KlineData, error) {
	if strings.ToUpper(market) == "HK" {
		return nil, fmt.Errorf("网易财经暂不支持港股K线")
	}
	var neteaseCode string
	if strings.ToUpper(market) == "SH" {
		neteaseCode = "0" + code
	} else {
		neteaseCode = "1" + code
	}
	end := time.Now().Format("20060102")
	start := time.Now().AddDate(0, -6, 0).Format("20060102")
	url := fmt.Sprintf("http://quotes.money.163.com/service/chddata.html?code=%s&start=%s&end=%s&fields=TCLOSE;HIGH;LOW;TOPEN;LCLOSE;VOTURNOVER;VATURNOVER", neteaseCode, start, end)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// 网易返回 GB2312 编码的 CSV
	reader := transform.NewReader(resp.Body, simplifiedchinese.GBK.NewDecoder())
	csvReader := csv.NewReader(reader)
	csvReader.LazyQuotes = true
	records, err := csvReader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) <= 1 {
		return nil, fmt.Errorf("网易K线数据为空")
	}

	var klines []KlineData
	// 跳过表头，从最后一行往前取 limit 条
	for i := len(records) - 1; i >= 1; i-- {
		row := records[i]
		if len(row) < 11 {
			continue
		}
		// 日期,股票代码,名称,收盘价,最高价,最低价,开盘价,前收盘,涨跌额,涨跌幅,成交量,成交金额
		date := strings.TrimSpace(row[0])
		if date == "" {
			continue
		}
		// 转换日期格式 2024-01-01 -> 20240101（Time 字段统一使用 YYYY-MM-DD）
		date = strings.ReplaceAll(date, "-", "")
		if len(date) == 8 {
			date = date[:4] + "-" + date[4:6] + "-" + date[6:]
		}
		amount := 0.0
		if len(row) > 11 {
			amount = parseStrFloat(row[11]) // 成交金额（元）
		}
		klines = append(klines, KlineData{
			Time:   date,
			Open:   parseStrFloat(row[6]),
			High:   parseStrFloat(row[4]),
			Low:    parseStrFloat(row[5]),
			Close:  parseStrFloat(row[3]),
			Volume: parseStrFloat(row[10]) / 100, // 网易成交量是股数，转为手
			Amount: amount,
		})
		if len(klines) >= limit {
			break
		}
	}
	// 恢复时间正序
	for i, j := 0, len(klines)-1; i < j; i, j = i+1, j-1 {
		klines[i], klines[j] = klines[j], klines[i]
	}
	if len(klines) == 0 {
		return nil, fmt.Errorf("网易K线数据为空")
	}
	return klines, nil
}
