package analyzer

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// MLSentimentPrediction A 舆情+量价高频预警结果
type MLSentimentPrediction struct {
	MovementLabel string  `json:"movement_label"`
	MovementProb  float64 `json:"movement_prob"`
	AnomalyProb   float64 `json:"anomaly_prob"`
}

// MLFinancialPrediction 财务趋势预警结果
type MLFinancialPrediction struct {
	ROEDirection     string  `json:"roe_direction"`
	ROEProb          float64 `json:"roe_prob"`
	RevenueDirection string  `json:"revenue_direction"`
	RevenueProb      float64 `json:"revenue_prob"`
	MScoreDirection  string  `json:"mscore_direction"`
	MScoreProb       float64 `json:"mscore_prob"`
	HealthScore      float64 `json:"health_score"`
}

// mlInferenceScriptPath 返回推理脚本绝对路径
func mlInferenceScriptPath() string {
	_, b, _, _ := runtime.Caller(0)
	base := filepath.Dir(b)
	return filepath.Join(base, "..", "ml_models", "inference.py")
}

func resolveMLPythonExecutable() string {
	_, b, _, _ := runtime.Caller(0)
	base := filepath.Dir(b)
	venvPython := filepath.Join(base, "..", ".venv", "bin", "python3")
	if _, err := os.Stat(venvPython); err == nil {
		return venvPython
	}
	return "python3"
}

// callMLInference 调用 Python 推理脚本
func callMLInference(engine string, payload map[string]any) (map[string]any, error) {
	script := mlInferenceScriptPath()
	if _, err := os.Stat(script); os.IsNotExist(err) {
		return nil, fmt.Errorf("推理脚本不存在: %s", script)
	}

	req := map[string]any{
		"engine":  engine,
		"payload": payload,
	}
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	python := resolveMLPythonExecutable()
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
			return nil, fmt.Errorf("推理失败: %s | stderr: %s", err, string(ee.Stderr))
		}
		return nil, err
	}

	var resp map[string]any
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, fmt.Errorf("解析推理结果失败: %w | raw: %s", err, string(out))
	}
	if e, ok := resp["error"].(string); ok && e != "" {
		return nil, fmt.Errorf("推理错误: %s", e)
	}
	return resp, nil
}

// RunMLEngineA 运行引擎 A 推理
func RunMLEngineA(textSeq [][]float64, priceSeq [][]float64) (*MLSentimentPrediction, error) {
	resp, err := callMLInference("A", map[string]any{
		"text_seq":  textSeq,
		"price_seq": priceSeq,
	})
	if err != nil {
		return nil, err
	}
	result := &MLSentimentPrediction{}
	if d, ok := resp["direction"].(string); ok {
		result.MovementLabel = d
	}
	if p, ok := resp["direction_probs"].(map[string]any); ok {
		if up, ok := p["up"].(float64); ok {
			result.MovementProb = up
		} else if stay, ok := p["stay"].(float64); ok {
			result.MovementProb = stay
		} else if down, ok := p["down"].(float64); ok {
			result.MovementProb = down
		}
	}
	if a, ok := resp["abnormal_prob"].(float64); ok {
		result.AnomalyProb = a
	}
	return result, nil
}

// RunMLEngineB 运行引擎 B 推理
func RunMLEngineB(financialSeq [][]float64) (*MLFinancialPrediction, error) {
	resp, err := callMLInference("B", map[string]any{
		"financial_seq": financialSeq,
	})
	if err != nil {
		return nil, err
	}
	result := &MLFinancialPrediction{}
	parseDir := func(key string) (string, float64) {
		if m, ok := resp[key].(map[string]any); ok {
			var d string
			var c float64
			if dv, ok := m["direction"].(string); ok {
				d = dv
			}
			if cv, ok := m["confidence"].(float64); ok {
				c = cv
			}
			return d, c
		}
		return "", 0
	}
	result.ROEDirection, result.ROEProb = parseDir("roe")
	result.RevenueDirection, result.RevenueProb = parseDir("revenue")
	result.MScoreDirection, result.MScoreProb = parseDir("mscore")
	if h, ok := resp["health_score"].(float64); ok {
		result.HealthScore = h
	}
	return result, nil
}
