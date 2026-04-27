//go:build windows

package downloader

import (
	"os/exec"
	"syscall"
)

// setHideWindow 在 Windows 上隐藏 CMD 窗口
func setHideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}
