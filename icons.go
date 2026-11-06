package main

import "github.com/egoist/mygo/ui"

// icon 画一个 16 网格上的线性图标。
func icon(shapes string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.35" stroke-linecap="round" stroke-linejoin="round">` + shapes + `</svg>`))
}

var (
	// settingsIcon 是侧边栏和设置页的滑杆图标。
	settingsIcon = icon(`<path d="M3 4.5h10M3 8h10M3 11.5h10"/><circle cx="6" cy="4.5" r="1.4"/><circle cx="10" cy="8" r="1.4"/><circle cx="5.5" cy="11.5" r="1.4"/>`)
	aboutIcon    = icon(`<circle cx="8" cy="8" r="6"/><path d="M8 7.2v4"/><path d="M8 4.7v.1"/>`)
	externalIcon = icon(`<path d="M9.5 3.5H13v3.5M13 3.5 7.5 9M12.5 9.5v2a1 1 0 0 1-1 1h-7a1 1 0 0 1-1-1v-7a1 1 0 0 1 1-1h2"/>`)

	// 自绘的窗口控制按钮图标，形状与 Kero 一致。
	minimizeIcon = icon(`<path d="M3.5 8h9"/>`)
	maximizeIcon = icon(`<rect x="3.5" y="3.5" width="9" height="9" rx="1.5"/>`)
	restoreIcon  = icon(`<rect x="3.5" y="5.5" width="7" height="7" rx="1.2"/><path d="M5.5 3.5h5.5a1.5 1.5 0 0 1 1.5 1.5v5.5"/>`)
	closeIcon    = icon(`<path d="M4 4 12 12M12 4 4 12"/>`)
)
