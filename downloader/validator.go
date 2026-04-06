package downloader

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

// ValidationResult 单指标校验结果
type ValidationResult struct {
	Year        string  `json:"year"`
	Indicator   string  `json:"indicator"`
	HSF10Value  float64 `json:"hsf10Value"`
	DCValue     float64 `json:"dcValue"`
	DiffPercent float64 `json:"diffPercent"`
	Status      string  `json:"status"` // ok / warning / error
}

// ValidateWithDatacenter 用 datacenter-web API 校验 HSF10 下载的数据
func ValidateWithDatacenter(market, code string, data *FinancialReportData) ([]ValidationResult, error) {
	fullCode := market + code
	results := []ValidationResult{}

	// datacenter-web 三张表端点
	reports := []struct {
		name   string
		report string
		keys   []struct {
			hsField string
			dcField string
			label   string
		}
	}{
		{
			name:   "资产负债表",
			report: "RPT_DMSK_FN_BALANCE",
			keys: []struct {
				hsField string
				dcField string
				label   string
			}{
				{"资产合计", "TOTAL_ASSETS", "总资产"},
				{"负债合计", "TOTAL_LIABILITIES", "总负债"},
				{"所有者权益合计", "TOTAL_EQUITY", "所有者权益"},
				{"货币资金", "MONETARYFUNDS", "货币资金"},
			},
		},
		{
			name:   "利润表",
			report: "RPT_DMSK_FN_INCOME",
			keys: []struct {
				hsField string
				dcField string
				label   string
			}{
				{"营业收入", "TOTAL_OPERATE_INCOME", "营业收入"},
				{"营业成本", "OPERATE_COST", "营业成本"},
				{"营业利润", "OPERATE_PROFIT", "营业利润"},
				{"归属于母公司所有者的净利润", "PARENT_NETPROFIT", "归母净利润"},
			},
		},
		{
			name:   "现金流量表",
			report: "RPT_DMSK_FN_CASHFLOW",
			keys: []struct {
				hsField string
				dcField string
				label   string
			}{
				{"经营活动产生的现金流量净额", "NETCASH_OPERATE", "经营现金流"},
				{"购建固定资产无形资产和其他长期资产支付的现金", "CONSTRUCT_LONG_ASSET", "购建长期资产"},
			},
		},
	}

	for _, r := range reports {
		dcData, err := fetchDatacenter(r.report, code)
		if err != nil {
			continue // 跳过校验失败的表
		}
		for _, year := range data.Years {
			if len(results) >= 9 { // 最多校验最近3年的9个指标
				break
			}
			for _, k := range r.keys {
				hsVal := 0.0
				switch r.name {
				case "资产负债表":
					if m, ok := data.BalanceSheet[k.hsField]; ok {
						hsVal = m[year]
					}
				case "利润表":
					if m, ok := data.IncomeStatement[k.hsField]; ok {
						hsVal = m[year]
					}
				case "现金流量表":
					if m, ok := data.CashFlow[k.hsField]; ok {
						hsVal = m[year]
					}
				}
				dcVal := 0.0
				for _, row := range dcData {
					reportDate, _ := row["REPORT_DATE"].(string)
					if reportDate != "" && (reportDate == year || reportDate == year+" 00:00:00") {
						dcVal = extractFloat(row[k.dcField])
						break
					}
				}
				status := "ok"
				diffPct := 0.0
				if dcVal != 0 {
					diffPct = math.Abs(hsVal-dcVal) / math.Abs(dcVal) * 100
				} else if hsVal != 0 {
					diffPct = 100
				}
				if diffPct > 10 {
					status = "error"
				} else if diffPct > 5 {
					status = "warning"
				}
				results = append(results, ValidationResult{
					Year:        year,
					Indicator:   k.label,
					HSF10Value:  hsVal,
					DCValue:     dcVal,
					DiffPercent: diffPct,
					Status:      status,
				})
			}
		}
	}
	_ = fullCode // 避免未使用变量警告（后续如需可扩展）
	return results, nil
}

func fetchDatacenter(reportName, code string) ([]map[string]any, error) {
	url := fmt.Sprintf("%s?sortColumns=REPORT_DATE&sortTypes=-1&pageSize=500&pageNumber=1&reportName=%s&columns=ALL&source=WEB&filter=(SECURITY_CODE=\"%s\")", dcBaseURL, reportName, code)
	url = strings.ReplaceAll(url, `"`, "%22")
	body, err := httpGetWithReferer(url, "https://data.eastmoney.com/")
	if err != nil {
		return nil, err
	}

	var result dcResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	if !result.Success || result.Result == nil {
		return nil, fmt.Errorf("datacenter-web API failed")
	}
	return result.Result.Data, nil
}
