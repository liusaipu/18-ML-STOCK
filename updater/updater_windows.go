//go:build windows

package updater

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// ApplyUpdate 在 Windows 上应用更新：解压 zip，生成 update.bat，启动后退出
func ApplyUpdate(zipPath string) error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取可执行文件路径失败: %w", err)
	}
	exeDir := filepath.Dir(exePath)
	updateDir := filepath.Dir(zipPath)
	extractDir := filepath.Join(updateDir, "extracted")

	// 1. 解压 zip
	if err := ExtractZip(zipPath, extractDir); err != nil {
		return fmt.Errorf("解压更新包失败: %w", err)
	}

	// 2. 生成 update.bat
	batPath := filepath.Join(updateDir, "update.bat")
	batContent := fmt.Sprintf(`@echo off
timeout /t 2 /nobreak >nul
xcopy /E /Y "%s\*" "%s\" >nul 2>&1
rmdir /S /Q "%s" >nul 2>&1
del "%s" >nul 2>&1
start "" "%s"
del "%%~f0"
`,
		strings.ReplaceAll(extractDir, "/", "\\"),
		strings.ReplaceAll(exeDir, "/", "\\"),
		strings.ReplaceAll(extractDir, "/", "\\"),
		strings.ReplaceAll(zipPath, "/", "\\"),
		strings.ReplaceAll(exePath, "/", "\\"),
	)

	if err := os.WriteFile(batPath, []byte(batContent), 0644); err != nil {
		return fmt.Errorf("生成更新脚本失败: %w", err)
	}

	// 3. 启动 bat（隐藏窗口）
	cmd := exec.Command("cmd", "/c", batPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动更新脚本失败: %w", err)
	}

	// 4. 退出当前进程
	os.Exit(0)
	return nil // 不会执行到这里
}
