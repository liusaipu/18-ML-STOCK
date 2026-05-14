package tray

import "context"

// Init 初始化系统托盘（平台特定实现）
func Init(ctx context.Context) {
	initTray(ctx)
}

// Quit 清理系统托盘
func Quit() {
	quitTray()
}

// UpdateTitle 更新 tray 标题（跨平台公共接口）
// title: 显示的文本，changePercent: 涨跌幅（用于决定颜色，A股涨红跌绿）
func UpdateTitle(title string, changePercent float64) {
	updateTrayTitle(title, changePercent)
}

// SetQuotesJSON 设置 tray 滚动显示的股票数据（JSON 格式字符串）
// 仅在 macOS 上生效，用于 menu bar 滚动字幕效果
func SetQuotesJSON(json string) {
	setTrayQuotesJSON(json)
}

// SetScrollEnabled 设置 tray 滚动字幕开关
func SetScrollEnabled(enabled bool) {
	setTrayScrollEnabled(enabled)
}

// SetIconVisible 设置 tray 菜单图标显示/隐藏
func SetIconVisible(visible bool) {
	setTrayIconVisible(visible)
}

// IsScrollEnabled 返回滚动字幕是否开启
func IsScrollEnabled() bool {
	return isTrayScrollEnabled()
}

// IsIconVisible 返回菜单图标是否显示
func IsIconVisible() bool {
	return isTrayIconVisible()
}

// 状态变更回调，用于同步应用菜单标题
var menuStateChangeCallback func(scrollEnabled, iconVisible bool)

// SetMenuStateChangeCallback 设置 tray 状态变更时的回调
func SetMenuStateChangeCallback(cb func(scrollEnabled, iconVisible bool)) {
	menuStateChangeCallback = cb
}
