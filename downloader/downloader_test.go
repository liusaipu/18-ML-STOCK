package downloader

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestDownload603501(t *testing.T) {
	data, err := DownloadFinancialReports("SH", "603501")
	if err != nil {
		t.Fatalf("download failed: %v", err)
	}

	fmt.Printf("Downloaded years: %v\n", data.Years)
	fmt.Printf("BalanceSheet keys: %d\n", len(data.BalanceSheet))
	fmt.Printf("IncomeStatement keys: %d\n", len(data.IncomeStatement))
	fmt.Printf("CashFlow keys: %d\n", len(data.CashFlow))

	// 验证关键字段
	if len(data.Years) == 0 {
		t.Fatal("no years downloaded")
	}
	latest := data.Years[0]

	bsKeys := []string{"资产合计", "负债合计", "货币资金", "应收账款", "固定资产", "应付票据及应付账款"}
	for _, k := range bsKeys {
		if _, ok := data.BalanceSheet[k]; !ok {
			t.Fatalf("missing balance sheet key: %s", k)
		}
		fmt.Printf("BS %s %s = %.0f\n", latest, k, data.BalanceSheet[k][latest])
	}

	isKeys := []string{"营业收入", "营业成本", "销售费用", "管理费用", "研发费用", "财务费用", "营业利润", "归属于母公司所有者的净利润"}
	for _, k := range isKeys {
		if _, ok := data.IncomeStatement[k]; !ok {
			t.Fatalf("missing income statement key: %s", k)
		}
		fmt.Printf("IS %s %s = %.0f\n", latest, k, data.IncomeStatement[k][latest])
	}

	cfKeys := []string{"经营活动产生的现金流量净额", "购建固定资产无形资产和其他长期资产支付的现金", "分配股利、利润或偿付利息支付的现金", "固定资产折旧、油气资产折耗、生产性生物资产折旧"}
	for _, k := range cfKeys {
		if _, ok := data.CashFlow[k]; !ok {
			t.Fatalf("missing cash flow key: %s", k)
		}
		fmt.Printf("CF %s %s = %.0f\n", latest, k, data.CashFlow[k][latest])
	}

	// 保存测试
	tmpDir := t.TempDir()
	if err := SaveAsJSON(data, tmpDir); err != nil {
		t.Fatalf("save json failed: %v", err)
	}
	for _, f := range []string{"balance_sheet.json", "income_statement.json", "cash_flow.json"} {
		path := filepath.Join(tmpDir, f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Fatalf("file not created: %s", f)
		}
	}

	// 校验测试
	validation, err := ValidateWithDatacenter("SH", "603501", data)
	if err != nil {
		t.Logf("validation error: %v", err)
	}
	for _, v := range validation {
		fmt.Printf("Validation %s %s: hsf10=%.0f dc=%.0f diff=%.2f%% status=%s\n",
			v.Year, v.Indicator, v.HSF10Value, v.DCValue, v.DiffPercent, v.Status)
	}
}
