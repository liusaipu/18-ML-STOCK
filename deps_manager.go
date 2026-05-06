package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"time"
)

// PythonPackage 需要检测/安装的 Python 包
type PythonPackage struct {
	Name       string `json:"name"`       // pip 安装名
	ModuleName string `json:"moduleName"` // Python import 名
	Display    string `json:"display"`    // 显示名称
	Required   bool   `json:"required"`   // 是否为核心必需包
	Installed  bool   `json:"installed"`  // 是否已安装
	Version    string `json:"version"`    // 已安装版本号
}

// PythonEnvResult Python 环境检测结果
type PythonEnvResult struct {
	PythonFound bool            `json:"pythonFound"`
	PythonPath  string          `json:"pythonPath"`
	Version     string          `json:"version"`
	Packages    []PythonPackage `json:"packages"`
	AllReady    bool            `json:"allReady"` // 所有必需包都已安装
	Ready       bool            `json:"ready"`    // 所有包（含可选）都已安装
	Missing     []string        `json:"missing"`  // 缺失的包名列表
}

// requiredPackages 定义需要检测的核心包列表
var requiredPackages = []PythonPackage{
	{Name: "onnxruntime", ModuleName: "onnxruntime", Display: "ONNX Runtime", Required: true},
	{Name: "scikit-learn", ModuleName: "sklearn", Display: "scikit-learn", Required: true},
	{Name: "numpy", ModuleName: "numpy", Display: "NumPy", Required: true},
	{Name: "pandas", ModuleName: "pandas", Display: "Pandas", Required: true},
	{Name: "akshare", ModuleName: "akshare", Display: "akshare", Required: true},
	{Name: "requests", ModuleName: "requests", Display: "Requests", Required: false},
	{Name: "openpyxl", ModuleName: "openpyxl", Display: "openpyxl", Required: false},
}

// appBundleParentDir 检测 macOS .app bundle 结构，返回 .app 的父目录
func appBundleParentDir(exeDir string) string {
	contentsDir := filepath.Dir(exeDir)
	if filepath.Base(contentsDir) != "Contents" {
		return ""
	}
	appDir := filepath.Dir(contentsDir)
	if !strings.HasSuffix(filepath.Base(appDir), ".app") {
		return ""
	}
	return filepath.Dir(appDir)
}

// findPythonExecutable 查找系统中可用的 Python 可执行文件
func findPythonExecutable() string {
	// 1. 先查找 .venv（开发/打包环境）
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		candidates := []string{
			filepath.Join(exeDir, ".venv", "Scripts", "python.exe"),
			filepath.Join(exeDir, ".venv", "bin", "python3"),
		}
		for _, p := range candidates {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
		// macOS .app bundle: 检查 .app 同级目录下的 .venv
		if parent := appBundleParentDir(exeDir); parent != "" {
			venvPython := filepath.Join(parent, ".venv", "bin", "python3")
			if _, err := os.Stat(venvPython); err == nil {
				return venvPython
			}
		}
	}

	// 2. Windows 常见安装路径
	if runtime.GOOS == "windows" {
		var pythonPaths []string
		// python.org 官方安装器默认路径（用户级）
		if home, err := os.UserHomeDir(); err == nil {
			for _, ver := range []string{"Python314", "Python313", "Python312", "Python311", "Python310", "Python39"} {
				pythonPaths = append(pythonPaths, filepath.Join(home, "AppData", "Local", "Programs", "Python", ver, "python.exe"))
			}
		}
		// 系统级路径
		pythonPaths = append(pythonPaths,
			`C:\Python314\python.exe`,
			`C:\Python313\python.exe`,
			`C:\Python312\python.exe`,
			`C:\Python311\python.exe`,
			`C:\Python310\python.exe`,
			`C:\Python39\python.exe`,
			`C:\Program Files\Python314\python.exe`,
			`C:\Program Files\Python313\python.exe`,
			`C:\Program Files\Python312\python.exe`,
			`C:\Program Files\Python311\python.exe`,
			`C:\Program Files\Python310\python.exe`,
			`C:\Program Files\Python39\python.exe`,
			`C:\Program Files (x86)\Python314\python.exe`,
			`C:\Program Files (x86)\Python313\python.exe`,
			`C:\Program Files (x86)\Python312\python.exe`,
			`C:\Program Files (x86)\Python311\python.exe`,
			`C:\Program Files (x86)\Python310\python.exe`,
			`C:\Program Files (x86)\Python39\python.exe`,
		)
		for _, p := range pythonPaths {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}

		// where 命令查找，优先非 Windows Store 路径
		if out, err := exec.Command("where", "python").Output(); err == nil {
			paths := strings.Split(strings.TrimSpace(string(out)), "\n")
			var fallback string
			for _, p := range paths {
				p = strings.TrimSpace(p)
				if p == "" {
					continue
				}
				// Windows Store 的 python.exe 是 redirector/shim，
				// exec.Command 调用时无法正确执行 -m pip，会导致 exit status 9009
				if strings.Contains(strings.ToLower(p), `windowsapps`) {
					if fallback == "" {
						fallback = p
					}
					continue
				}
				return p
			}
			if fallback != "" {
				return fallback
			}
		}

		// PATH 中查找，同样过滤 Windows Store
		for _, name := range []string{"python", "python3", "py"} {
			if p, err := exec.LookPath(name); err == nil {
				if !strings.Contains(strings.ToLower(p), `windowsapps`) {
					return p
				}
			}
		}
		return ""
	}

	// 3. macOS / Linux
	for _, name := range []string{"python3", "python"} {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	return ""
}

// getPythonVersion 获取 Python 版本
func getPythonVersion(python string) string {
	out, err := exec.Command(python, "--version").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// checkPythonPackage 检测单个 Python 包是否已安装
func checkPythonPackage(python, moduleName string) (bool, string) {
	script := fmt.Sprintf(
		"import sys, importlib; "+
			"m = importlib.import_module('%s'); "+
			"print(getattr(m, '__version__', getattr(m, 'VERSION', 'unknown'))); "+
			"sys.exit(0)",
		moduleName,
	)
	out, err := exec.Command(python, "-c", script).Output()
	if err != nil {
		return false, ""
	}
	return true, strings.TrimSpace(string(out))
}

// CheckPythonEnv 检测 Python 环境及依赖包状态
func CheckPythonEnv() *PythonEnvResult {
	result := &PythonEnvResult{
		Packages: make([]PythonPackage, len(requiredPackages)),
		Missing:  []string{},
	}
	copy(result.Packages, requiredPackages)

	python := findPythonExecutable()
	if python == "" {
		return result
	}
	result.PythonFound = true
	result.PythonPath = python
	result.Version = getPythonVersion(python)

	allReady := true
	ready := true
	for i := range result.Packages {
		pkg := &result.Packages[i]
		installed, version := checkPythonPackage(python, pkg.ModuleName)
		pkg.Installed = installed
		pkg.Version = version
		if !installed {
			result.Missing = append(result.Missing, pkg.Name)
			ready = false
			if pkg.Required {
				allReady = false
			}
		}
	}
	result.AllReady = allReady
	result.Ready = ready
	return result
}

// InstallPythonPackages 安装指定的 Python 包，返回实时输出
func InstallPythonPackages(python string, packages []string, onOutput func(string)) error {
	args := append([]string{"-m", "pip", "install"}, packages...)
	cmd := exec.Command(python, args...)
	cmd.Env = append(os.Environ(), "PYTHONIOENCODING=utf-8")

	setHideWindow(cmd)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := stdout.Read(buf)
			if n > 0 && onOutput != nil {
				onOutput(string(buf[:n]))
			}
			if err != nil {
				break
			}
		}
	}()
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := stderr.Read(buf)
			if n > 0 && onOutput != nil {
				onOutput(string(buf[:n]))
			}
			if err != nil {
				break
			}
		}
	}()

	return cmd.Wait()
}

// depsMarkerPath 依赖检查标记文件路径
func depsMarkerPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "stock-analyzer", "deps_checked")
}

// hasCheckedDeps 是否已经检查过依赖（标记文件 7 天后过期）
func hasCheckedDeps() bool {
	p := depsMarkerPath()
	if p == "" {
		return false
	}
	info, err := os.Stat(p)
	if err != nil {
		return false
	}
	// 7 天后过期，重新检测
	return time.Since(info.ModTime()) < 7*24*time.Hour
}

// markDepsChecked 标记依赖已检查
func markDepsChecked() error {
	p := depsMarkerPath()
	if p == "" {
		return nil
	}
	dir := filepath.Dir(p)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(p, []byte(time.Now().Format(time.RFC3339)), 0644)
}
