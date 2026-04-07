package downloader

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// RIMExternalData Python脚本返回的原始数据
type RIMExternalData struct {
	Symbol      string             `json:"symbol"`
	EPSForecast map[string]float64 `json:"eps_forecast"`
	EPSForecastError string        `json:"eps_forecast_error,omitempty"`
	Rf          float64            `json:"rf"`
	RfDate      string             `json:"rf_date,omitempty"`
	RfError     string             `json:"rf_error,omitempty"`
	Price       float64            `json:"price"`
	TotalShares float64            `json:"total_shares"`
	MarketCap   float64            `json:"market_cap"`
	PB          float64            `json:"pb"`
	Beta        float64            `json:"beta"`
	RmRf        float64            `json:"rm_rf"`
	Error       string             `json:"error,omitempty"`
}

// fetchRIMScriptPath 返回 fetch_rim_data.py 绝对路径
func fetchRIMScriptPath() string {
	_, b, _, _ := runtime.Caller(0)
	base := filepath.Dir(b)
	return filepath.Join(base, "..", "scripts", "fetch_rim_data.py")
}

// resolvePythonExecutable 优先使用项目 .venv/bin/python3
func resolvePythonExecutable() string {
	_, b, _, _ := runtime.Caller(0)
	base := filepath.Dir(b)
	venvPython := filepath.Join(base, "..", ".venv", "bin", "python3")
	if _, err := os.Stat(venvPython); err == nil {
		return venvPython
	}
	return "python3"
}

// FetchRIMExternalData 调用 Python 脚本获取 RIM 外部数据
func FetchRIMExternalData(symbol string) (*RIMExternalData, error) {
	script := fetchRIMScriptPath()
	if _, err := os.Stat(script); os.IsNotExist(err) {
		return nil, fmt.Errorf("fetch_rim_data.py not found: %s", script)
	}

	req := map[string]string{"symbol": symbol}
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	python := resolvePythonExecutable()
	cmd := exec.Command(python, script)
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
			return nil, fmt.Errorf("fetch rim data failed: %s | stderr: %s", err, string(ee.Stderr))
		}
		return nil, err
	}

	var resp RIMExternalData
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, fmt.Errorf("parse rim data failed: %w | raw: %s", err, string(out))
	}
	if resp.Error != "" {
		return nil, fmt.Errorf("fetch rim data error: %s", resp.Error)
	}
	return &resp, nil
}
