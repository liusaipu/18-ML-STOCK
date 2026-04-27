//go:build !windows

package downloader

import "os/exec"

// setHideWindow 在非 Windows 平台上无操作
func setHideWindow(cmd *exec.Cmd) {}
