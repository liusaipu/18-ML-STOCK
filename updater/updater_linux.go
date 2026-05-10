//go:build linux

package updater

import (
	"fmt"
)

// ApplyUpdate 在 Linux 上暂不支持自动安装
func ApplyUpdate(_ string) error {
	return fmt.Errorf("Linux 平台暂不支持自动更新，请手动下载安装")
}
