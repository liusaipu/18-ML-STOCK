package downloader

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const tencentFinanceBaseURL = "https://web.ifzq.gtimg.cn/appstock/finance"

// TencentFinancialData 腾讯财经财务数据
type TencentFinancialData struct {
	Code   string
	Market string
	Years  []string
	Data   map[string]map[string]float64 // year -> field -> value
}

// DownloadFromTencent 从腾讯财经下载财务数据作为备用数据源
func DownloadFromTencent(market, code string) (*FinancialReportData, error) {
	// 腾讯财经代码格式: sh600531 或 sz000001
	tencentCode := strings.ToLower(market) + code
	
	// 尝试获取资产负债表
	balanceData, err := fetchTencentData(tencentCode, "f10_zcfzb")
	if err != nil {
		return nil, fmt.Errorf("腾讯财经资产负债表获取失败: %w", err)
	}
	
	// 尝试获取利润表
	incomeData, err := fetchTencentData(tencentCode, "f10_lrb")
	if err != nil {
		return nil, fmt.Errorf("腾讯财经利润表获取失败: %w", err)
	}
	
	// 尝试获取现金流量表
	cashflowData, err := fetchTencentData(tencentCode, "f10_xjllb")
	if err != nil {
		return nil, fmt.Errorf("腾讯财经现金流量表获取失败: %w", err)
	}
	
	// 转换数据格式
	result := &FinancialReportData{
		Symbol:          code,
		Years:           extractYears(balanceData),
		BalanceSheet:    convertTencentFormat(balanceData),
		IncomeStatement: convertTencentFormat(incomeData),
		CashFlow:        convertTencentFormat(cashflowData),
	}
	
	return result, nil
}

func fetchTencentData(code, endpoint string) (map[string]any, error) {
	url := fmt.Sprintf("%s/%s?code=%s", tencentFinanceBaseURL, endpoint, code)
	
	client := &http.Client{Timeout: 20 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://stock.finance.qq.com/")
	
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	
	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	
	return result, nil
}

func extractYears(data map[string]any) []string {
	// 从腾讯数据中提取年份列表
	years := make(map[string]bool)
	
	if items, ok := data["data"].(map[string]any); ok {
		for key := range items {
			// 腾讯数据的key通常是日期格式
			if len(key) >= 10 {
				year := key[:4]
				if _, err := time.Parse("2006", year); err == nil {
					years[year+"-12-31"] = true
				}
			}
		}
	}
	
	result := make([]string, 0, len(years))
	for y := range years {
		result = append(result, y)
	}
	return result
}

func convertTencentFormat(data map[string]any) map[string]map[string]float64 {
	result := make(map[string]map[string]float64)
	
	if items, ok := data["data"].(map[string]any); ok {
		for date, values := range items {
			year := date[:4] + "-12-31"
			if result[year] == nil {
				result[year] = make(map[string]float64)
			}
			
			if vmap, ok := values.(map[string]any); ok {
				for k, v := range vmap {
					switch val := v.(type) {
					case float64:
						result[year][k] = val
					case string:
						// 尝试解析字符串为数字
						if f, err := strconv.ParseFloat(val, 64); err == nil {
							result[year][k] = f
						}
					}
				}
			}
		}
	}
	
	return result
}
