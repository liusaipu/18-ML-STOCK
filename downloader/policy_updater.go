package downloader

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
)

// PolicyUpdateResult Python脚本返回的更新结果
type PolicyUpdateResult struct {
	Success               bool     `json:"success"`
	Path                  string   `json:"path"`
	AddedIndustryKeywords int      `json:"added_industry_keywords"`
	AddedConceptKeywords  int      `json:"added_concept_keywords"`
	TotalIndustries       int      `json:"total_industries"`
	TotalConcepts         int      `json:"total_concepts"`
	Errors                []string `json:"errors"`
}

// updatePolicyScriptPath 返回 update_policy_library.py 绝对路径
func updatePolicyScriptPath() string {
	_, b, _, _ := runtime.Caller(0)
	base := filepath.Dir(b)
	root := findProjectRootByMarker(base, filepath.Join("scripts", "update_policy_library.py"))
	if root != "" {
		return filepath.Join(root, "scripts", "update_policy_library.py")
	}
	return filepath.Join(base, "..", "scripts", "update_policy_library.py")
}

// UpdatePolicyLibrary 调用 Python 脚本更新政策库 JSON
func UpdatePolicyLibrary(dataDir string) (*PolicyUpdateResult, error) {
	script := updatePolicyScriptPath()
	python := resolvePythonExecutable()
	cmd := exec.Command(python, script, dataDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("政策库更新脚本执行失败: %v, output: %s", err, string(output))
	}
	var result PolicyUpdateResult
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("解析政策库更新结果失败: %v, raw: %s", err, string(output))
	}
	if !result.Success {
		return &result, fmt.Errorf("政策库更新未成功")
	}
	return &result, nil
}
