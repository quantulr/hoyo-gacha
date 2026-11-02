package main

import "github.com/egoist/mygo/ui"

// icon 画一个 16 网格上的线性图标。
func icon(shapes string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.35" stroke-linecap="round" stroke-linejoin="round">` + shapes + `</svg>`))
}

// 三个游戏的侧边栏图标，均使用 currentColor，跟随文字颜色。
var (
	// 原神：四芒星。
	genshinIcon = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24">
		<path fill="currentColor" d="M12 2.6c.75 4.75 3.65 7.65 8.4 8.4-4.75.75-7.65 3.65-8.4 8.4-.75-4.75-3.65-7.65-8.4-8.4 4.75-.75 7.65-3.65 8.4-8.4Z"/>
		<path fill="currentColor" d="M18.8 16.4c.3 1.6 1.2 2.5 2.8 2.8-1.6.3-2.5 1.2-2.8 2.8-.3-1.6-1.2-2.5-2.8-2.8 1.6-.3 2.5-1.2 2.8-2.8Z"/>
	</svg>`))

	// 崩坏：星穹铁道：五芒星。
	starRailIcon = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24">
		<path fill="currentColor" d="M12 3 14.35 8.76 20.56 9.22 15.8 13.24 17.29 19.28 12 16 6.71 19.28 8.2 13.24 3.44 9.22 9.65 8.76Z"/>
	</svg>`))

	// 绝区零：锐利的 Z。
	zzzIcon = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24">
		<path fill="none" stroke="currentColor" stroke-width="2.2" stroke-linejoin="round" d="M6.5 5h11l-11 14h11"/>
	</svg>`))

	// 应用标记：圆环。
	logoIcon = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24">
		<circle cx="12" cy="12" r="7" fill="none" stroke="currentColor" stroke-width="3.4"/>
	</svg>`))
)

// 自绘的窗口控制按钮图标，形状与 Kero 一致。
var (
	minimizeIcon = icon(`<path d="M3.5 8h9"/>`)
	maximizeIcon = icon(`<rect x="3.5" y="3.5" width="9" height="9" rx="1.5"/>`)
	restoreIcon  = icon(`<rect x="3.5" y="5.5" width="7" height="7" rx="1.2"/><path d="M5.5 3.5h5.5a1.5 1.5 0 0 1 1.5 1.5v5.5"/>`)
	closeIcon    = icon(`<path d="M4 4 12 12M12 4 4 12"/>`)
)
