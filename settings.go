package main

import (
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater"
	"github.com/egoist/mygo/ui"
)

// settingsNavItem 是设置窗口侧边栏的一项。
type settingsNavItem struct {
	id    string
	title string
	icon  *ui.SVG
}

var settingsNav = []settingsNavItem{
	{"general", "通用", settingsIcon},
	{"about", "关于", aboutIcon},
}

// openSettings 打开独立的设置窗口；已经打开时只把它带到前台。
func (a *app) openSettings() {
	if a.settingsWin != nil {
		a.settingsWin.Show()
		a.settingsWin.Focus()
		return
	}
	if a.settingsPage == "" {
		a.settingsPage = "general"
	}
	a.settingsWin = mygo.NewWindow(appWindowOptions(mygo.WindowOptions{
		Title:     "设置",
		Width:     760,
		Height:    560,
		MinWidth:  640,
		MinHeight: 440,
		StateKey:  "settings",
		Content:   ui.View(a.settingsView),
	}))
	a.settingsWin.OnClosed(func() {
		a.settingsWin = nil
	})
}

func (a *app) settingsView(c *ui.Context) {
	pal := paletteFor(c)
	applyTheme(c, pal)
	base, glow := backdrop(pal)
	c.Root().LinearGradient(base)
	row := ui.Row(c).Fill().AlignItems(ui.Stretch).LinearGradient(glow)
	row.Children(func() {
		a.settingsSidebar(c, pal)
		a.settingsContent(c, pal)
	})
}

// settingsSidebar 与主窗口一样是一条透明侧边栏，底部显示版本。
func (a *app) settingsSidebar(c *ui.Context, pal *palette) {
	top := float32(16)
	if !ownControls {
		// 让开 macOS 的交通灯按钮。
		top = 62
	}
	ui.Column(c).Width(200).Shrink(0).Padding(top, 10, 12, 10).Gap(2).
		DragWindow().Children(func() {
		for _, p := range settingsNav {
			a.settingsNavItem(c, pal, p)
		}
		ui.Spacer(c)
		ui.Text(c, versionText()).FontSize(11).TextColor(pal.textFaint).
			Padding(0, 10, 8, 10)
	})
}

func (a *app) settingsNavItem(c *ui.Context, pal *palette, p settingsNavItem) {
	selected := a.settingsPage == p.id
	item := ui.ButtonBase(c.Key("settings-"+p.id)).Height(32).Shrink(0).
		Padding(0, 10).Gap(10).Radius(8).Justify(ui.Start).AlignItems(ui.Center).Label(p.title)
	if pal.dark {
		item.Border(1, ui.Transparent) // 给选中项的描边留出位置
	}
	bg := ui.Transparent
	switch {
	case selected:
		bg = pal.rowSelected
		chosen(item, pal)
	case item.Hovered():
		bg = pal.rowHover
	}
	item.Background(item.AnimateColor("bg", bg, ui.Animation{Duration: 110 * time.Millisecond}))
	if item.Clicked() && !selected {
		a.settingsPage = p.id
	}
	item.Children(func() {
		ic := ui.Icon(c, p.icon).Size(15, 15).TextColor(pal.textMuted)
		if selected {
			ic.TextColor(pal.accent)
		}
		ui.Text(c, p.title).FontSize(13.5).TextColor(pal.text).SingleLine()
	})
}

// settingsContent 是设置窗口右侧的卡片：固定标题栏，内容在下面滚动。
func (a *app) settingsContent(c *ui.Context, pal *palette) {
	page := settingsNav[0]
	for _, p := range settingsNav {
		if p.id == a.settingsPage {
			page = p
		}
	}
	ui.Column(c).Grow(1).MinWidth(0).Padding(8).Children(func() {
		card := ui.Column(c).Grow(1).MinHeight(0).Radius(14).Background(pal.card).Clip()
		paned(card, pal)
		card.Children(func() {
			ui.Row(c).Height(48).Shrink(0).Padding(0, 10, 0, 20).Gap(8).
				AlignItems(ui.Center).DragWindow().Children(func() {
				ui.Text(c, page.title).FontSize(15).FontWeight(650).TextColor(pal.text)
				ui.Spacer(c)
				a.windowControlsFor(c, pal, a.settingsWin)
			})
			ui.Scroll(c).Grow(1).MinHeight(0).Padding(14, 28, 26, 28).Children(func() {
				ui.Column(c).Gap(22).MaxWidth(620).Children(func() {
					switch page.id {
					case "general":
						a.generalSettings(c, pal)
					case "about":
						a.aboutSettings(c, pal)
					}
				})
			})
		})
	})
}

func (a *app) generalSettings(c *ui.Context, pal *palette) {
	settingsGroup(c, pal, "更新", "", func() {
		settingRow(c, pal, "当前版本", "hoyo-gacha "+versionText()+"。启动后会自动检查更新，下载后下次启动生效。", func() {
			settingsButton(c, pal, "check-updates", "检查更新", nil, func() {
				updater.CheckForUpdates()
			})
		})
	})
	if dir, err := mygo.App.Path(mygo.PathUserData); err == nil && dir != "" {
		settingsGroup(c, pal, "数据", "", func() {
			settingRow(c, pal, "数据目录", dir, func() {
				settingsButton(c, pal, "open-data-dir", "打开文件夹", nil, func() {
					if err := mygo.Shell.OpenPath(dir); err != nil {
						c.Toast("无法打开数据目录")
					}
				})
			})
		})
	}
}

func (a *app) aboutSettings(c *ui.Context, pal *palette) {
	border := ui.RGBA(0, 0, 0, 0.10)
	if pal.dark {
		border = ui.RGBA(255, 255, 255, 0.12)
	}
	ui.Column(c).AlignItems(ui.Center).Padding(4, 0, 14, 0).Children(func() {
		ui.Image(c, appIcon).Size(72, 72).Radius(16).Border(0.5, border).
			Shadow(0, 6, 16, -8, ui.RGBA(0, 0, 0, 0.28))
		ui.Text(c, "hoyo-gacha").FontSize(21).FontWeight(650).TextColor(pal.text).
			Margin(14, 0, 2, 0)
		ui.Text(c, "原神 · 崩坏：星穹铁道 · 绝区零 抽卡记录管理器").
			FontSize(12.5).TextColor(pal.textMuted).TextAlign(ui.Center)
		ui.Text(c, versionText()).FontSize(11.5).TextColor(pal.textFaint).Margin(4, 0, 0, 0)
	})
	settingsGroup(c, pal, "致谢", "感谢这些项目提供的抓取流程与导出参考；历史卡池数据来自 Biligame Wiki。", func() {
		aboutProjectRow(c, pal, "biuuu/genshin-wish-export", "祈愿记录导出工具",
			"https://github.com/biuuu/genshin-wish-export")
		aboutProjectRow(c, pal, "Trrrrw/genshin-wish-export", "祈愿记录导出与千星奇域颂愿规则",
			"https://github.com/Trrrrw/genshin-wish-export")
		aboutProjectRow(c, pal, "biuuu/star-rail-warp-export", "跃迁记录导出与抓取流程",
			"https://github.com/biuuu/star-rail-warp-export")
		aboutProjectRow(c, pal, "earthjasonlin/zzz-signal-search-export", "绝区零调频记录导出与抓取流程",
			"https://github.com/earthjasonlin/zzz-signal-search-export")
	})
	ui.Text(c, "本项目是非官方工具，与米哈游 / HoYoverse 无关联。").
		FontSize(11.5).TextColor(pal.textFaint).TextAlign(ui.Center).Padding(0, 8)
}

func aboutProjectRow(c *ui.Context, pal *palette, name, detail, url string) {
	settingRow(c, pal, name, detail, func() {
		settingsButton(c, pal, "open-"+name, "查看", externalIcon, func() {
			c.OpenURL(url)
		})
	})
}

// settingsRows 统计当前分组已画出的行，用来在行间画分隔线。
var settingsRows int

// settingsGroup 是一组设置项，放在 Kero 风格的浅色面板上。
func settingsGroup(c *ui.Context, pal *palette, title, note string, fn func()) {
	ui.Column(c).Gap(8).Children(func() {
		if title != "" {
			ui.Text(c, title).FontSize(12).FontWeight(600).TextColor(pal.textMuted).Padding(0, 4)
		}
		if fn != nil {
			settingsRows = 0
			ui.Column(c).Radius(12).Background(pal.chip).Padding(4, 16).Children(fn)
		}
		if note != "" {
			ui.Text(c, note).FontSize(12).TextColor(pal.textFaint).LineHeight(1.4).Padding(0, 4)
		}
	})
}

// settingRow 是分组里的一行：左侧标签与说明，右侧控件。
func settingRow(c *ui.Context, pal *palette, label, detail string, control func()) {
	if settingsRows++; settingsRows > 1 {
		ui.Box(c).Height(1).Shrink(0).Background(pal.divider)
	}
	ui.Row(c).Padding(11, 0).Gap(16).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
			ui.Text(c, label).FontSize(13.5).TextColor(pal.text)
			if detail != "" {
				ui.Text(c, detail).FontSize(12).TextColor(pal.textMuted).LineHeight(1.35)
			}
		})
		if control != nil {
			ui.Row(c).Shrink(0).Gap(8).AlignItems(ui.Center).Children(control)
		}
	})
}

// settingsButton 是设置卡片上的按钮，外观与 Kero 的次级按钮一致。
func settingsButton(c *ui.Context, pal *palette, key, label string, iconSVG *ui.SVG, action func()) {
	b := ui.ButtonBase(c.Key(key)).Height(28).Shrink(0).Padding(0, 12).Gap(6).
		Radius(7).AlignItems(ui.Center).Label(label)
	bg, fg := pal.control, pal.text
	switch {
	case b.Pressed():
		bg = bg.Mix(ui.Hex("#000000"), 0.08)
	case b.Hovered():
		bg = pal.controlHover
	}
	b.Background(b.AnimateColor("bg", bg, ui.Animation{Duration: 90 * time.Millisecond})).TextColor(fg)
	raised(b, pal)
	b.Children(func() {
		ui.Text(c, label).FontSize(12.5).FontWeight(500).SingleLine()
		if iconSVG != nil {
			ui.Icon(c, iconSVG).Size(12, 12).TextColor(fg.Alpha(0.75))
		}
	})
	if b.Clicked() {
		action()
	}
}

// versionText 是界面里显示的版本号；测试和未打包运行时退回“开发版”。
func versionText() string {
	if v := mygo.App.Version(); v != "" {
		return "v" + v
	}
	return "开发版"
}
