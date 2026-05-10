//go:build darwin

package updater

import (
	"fmt"
	"os/exec"
)

// ApplyUpdate 在 macOS 上应用更新：用 open 命令打开 dmg
func ApplyUpdate(dmgPath string) error {
	cmd := exec.Command("open", dmgPath)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("打开 DMG 失败: %w", err)
	}
	return nil
}
