package downloader

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// RiskCrawlerData 非财务风险爬虫结果
type RiskCrawlerData struct {
	PledgeRatio        *float64 `json:"pledge_ratio"`
	InquiryCount1Y     *int     `json:"inquiry_count_1y"`
	ReductionCount1Y   *int     `json:"reduction_count_1y"`
	Error              string   `json:"error"`
}

func resolveRiskCrawlerPython() string {
	_, b, _, _ := runtime.Caller(0)
	base := filepath.Dir(b)
	root := findProjectRootByMarker(base, filepath.Join("ml_models", "risk_crawler.py"))
	if root != "" {
		venvPython := filepath.Join(root, ".venv", "bin", "python3")
		if _, err := os.Stat(venvPython); err == nil {
			return venvPython
		}
	}
	return "python3"
}

func riskCrawlerScriptPath() string {
	_, b, _, _ := runtime.Caller(0)
	base := filepath.Dir(b)
	root := findProjectRootByMarker(base, filepath.Join("ml_models", "risk_crawler.py"))
	if root != "" {
		return filepath.Join(root, "ml_models", "risk_crawler.py")
	}
	return filepath.Join(base, "..", "ml_models", "risk_crawler.py")
}

// FetchRiskCrawlerData 调用 Python 爬虫获取非财务风险数据
func FetchRiskCrawlerData(symbol string) (*RiskCrawlerData, error) {
	script := riskCrawlerScriptPath()
	if _, err := os.Stat(script); os.IsNotExist(err) {
		return nil, fmt.Errorf("爬虫脚本不存在: %s", script)
	}

	req := map[string]any{"symbol": symbol}
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	python := resolveRiskCrawlerPython()
	cmd := exec.Command(python, script)
	cmd.Env = append(os.Environ(), "TQDM_DISABLE=1", "PYTHONUNBUFFERED=1")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	go func() {
		_, _ = stdin.Write(reqBytes)
		_ = stdin.Close()
	}()

	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("爬虫失败: %s | stderr: %s", err, string(ee.Stderr))
		}
		return nil, err
	}

	var resp RiskCrawlerData
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, fmt.Errorf("解析爬虫结果失败: %w | raw: %s", err, string(out))
	}
	return &resp, nil
}
