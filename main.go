package main

import (
	"log"
	"runtime"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// game 是一款由本应用管理抽卡记录的游戏。
type game struct {
	id      string
	name    string
	tagline string
	art     *ui.Bitmap
}

// games 是侧边栏里的全部游戏，顺序即显示顺序。
var games = []game{
	{
		id:      "genshin",
		name:    "原神",
		tagline: "提瓦特大陆的旅途记录",
		art:     gameArt("genshin.png"),
	},
	{
		id:      "starrail",
		name:    "崩坏：星穹铁道",
		tagline: "开拓者的星穹列车手记",
		art:     gameArt("starrail.png"),
	},
	{
		id:      "zzz",
		name:    "绝区零",
		tagline: "新艾利都的委托档案",
		art:     gameArt("zzz.png"),
	},
}

// ownControls 表示由界面绘制窗口按钮，而不是交给系统标题栏。
var ownControls = runtime.GOOS != "darwin"

// app 是窗口展示的状态：当前选中的游戏与窗口本身。
type app struct {
	selected string
	win      *mygo.Window
}

// newApp 返回默认选中第一款游戏的应用状态。
func newApp() *app {
	return &app{selected: games[0].id}
}

// current 返回当前选中的游戏；选中项无效时退回第一款游戏。
func (a *app) current() *game {
	for i := range games {
		if games[i].id == a.selected {
			return &games[i]
		}
	}
	return &games[0]
}

func (a *app) view(c *ui.Context) {
	pal := paletteFor(c)
	applyTheme(c, pal)
	a.shortcuts(c)

	base, glow := backdrop(pal)
	c.Root().LinearGradient(base)
	row := ui.Row(c).Fill().AlignItems(ui.Stretch).LinearGradient(glow)
	row.Children(func() {
		a.sidebar(c, pal)
		a.content(c, pal, a.current())
	})
}

// shortcuts 让 Cmd/Ctrl+1、+2、+3 切换游戏，和 Kero 的会话快捷键一致。
func (a *app) shortcuts(c *ui.Context) {
	for i := range games {
		if c.Shortcut(ui.Cmd, ui.Key1+ui.Key(i)) {
			a.selected = games[i].id
		}
	}
}

// sidebar 是透明的侧边栏：顶部品牌、游戏列表，底色由全窗渐变透出。
func (a *app) sidebar(c *ui.Context, pal *palette) {
	left := float32(16)
	if !ownControls {
		// 让开 macOS 的交通灯按钮。
		left = 80
	}
	ui.Column(c).Width(268).Shrink(0).AlignItems(ui.Stretch).Children(func() {
		ui.Row(c).Height(60).Shrink(0).Padding(0, 12, 0, left).Gap(10).
			AlignItems(ui.Center).DragWindow().Children(func() {
			ui.Box(c).Size(26, 26).Shrink(0).Radius(8).Center().
				LinearGradient(ui.LinearGradient{Angle: 135, From: ui.Hex("#5b7cfa"), To: ui.Hex("#9a6cf0")}).
				Children(func() {
					ui.Icon(c, logoIcon).Size(14, 14).TextColor(ui.Hex("#ffffff"))
				})
			ui.Column(c).Gap(1).Children(func() {
				ui.Text(c, "hoyo-gacha").FontSize(13.5).FontWeight(600).TextColor(pal.text).SingleLine()
				ui.Text(c, "抽卡记录管理").FontSize(11).TextColor(pal.textFaint).SingleLine()
			})
		})
		ui.Row(c).Height(28).Shrink(0).Margin(8, 0, 4, 0).Padding(0, 10, 0, 12).
			AlignItems(ui.Center).Children(func() {
			ui.Text(c, "游戏").FontSize(12.5).FontWeight(600).TextColor(pal.textMuted).Grow(1)
			ui.Textf(c, "%d", len(games)).FontSize(12.5).TextColor(pal.textFaint).FontFeatures("tnum")
		})
		ui.Column(c).Shrink(0).PaddingX(10).Gap(1).Children(func() {
			for i := range games {
				a.gameRow(c, pal, &games[i])
			}
		})
		ui.Spacer(c)
	})
}

// gameRow 是侧边栏里的一行游戏；选中时像 Kero 的列表一样抬起为白色卡片。
func (a *app) gameRow(c *ui.Context, pal *palette, g *game) {
	selected := a.selected == g.id
	row := ui.ButtonBase(c.Key("game-"+g.id)).Height(38).Shrink(0).Margin(0, 0, 1, 0).
		Padding(0, 10, 0, 12).Gap(10).Radius(8).Justify(ui.Start).AlignItems(ui.Center).Label(g.name)
	bg := ui.Transparent
	switch {
	case selected:
		bg = pal.rowSelected
	case row.Hovered():
		bg = pal.rowHover
	}
	row.Background(row.AnimateColor("bg", bg, ui.Animation{Duration: 110 * time.Millisecond}))
	if selected {
		chosen(row, pal)
	}
	row.Children(func() {
		gameImage(c, g, 18, pal)
		ui.Text(c, g.name).FontSize(13.5).FontWeight(500).TextColor(pal.text).
			SingleLine().Ellipsis("…").Shrink(1).MinWidth(0)
	})
	if row.Clicked() && !selected {
		a.selected = g.id
	}
}

// gameImage 显示游戏的官方图标，圆角与源图一致，并加一条发丝边，
// 让浅色图标在白色卡片上也有清晰轮廓。
func gameImage(c *ui.Context, g *game, size float32, pal *palette) ui.Element {
	e := ui.Image(c, g.art).Size(size, size).Shrink(0).Radius(size * 0.22)
	if pal.dark {
		return e.Border(0.5, ui.RGBA(255, 255, 255, 0.12))
	}
	return e.Border(0.5, ui.RGBA(0, 0, 0, 0.10))
}

// content 是右侧的大卡片：顶部是当前游戏，主体是记录占位区。
func (a *app) content(c *ui.Context, pal *palette, g *game) {
	ui.Column(c).Grow(1).MinWidth(0).Padding(8).Children(func() {
		card := ui.Column(c).Grow(1).MinHeight(0).Radius(14).Background(pal.card).Clip()
		paned(card, pal)
		card.Children(func() {
			ui.Row(c).Height(44).Shrink(0).Padding(0, 10, 0, 20).Gap(8).
				AlignItems(ui.Center).DragWindow().Children(func() {
				gameImage(c, g, 18, pal)
				ui.Text(c, g.name).FontSize(14).FontWeight(600).TextColor(pal.text).SingleLine()
				ui.Text(c, "· "+g.tagline).FontSize(12.5).TextColor(pal.textFaint).
					SingleLine().Ellipsis("…").Shrink(1).MinWidth(0)
				ui.Spacer(c)
				a.windowControls(c, pal)
			})
			ui.Column(c).Grow(1).MinHeight(0).Center().Padding(24).Children(func() {
				gameImage(c, g, 56, pal).Margin(0, 0, 18, 0).
					Shadow(0, 2, 8, 0, ui.RGBA(0, 0, 0, 0.12))
				ui.Text(c, "还没有抽卡记录").FontSize(19).FontWeight(650).TextColor(pal.text)
				ui.Text(c, "导入该游戏的抽卡记录后，统计和保底进度会显示在这里。").
					FontSize(13.5).TextColor(pal.textMuted).MaxWidth(380).
					TextAlign(ui.Center).LineHeight(1.45).Margin(8, 0, 0, 0)
			})
		})
	})
}

// windowControls 画出最小化、最大化/还原和关闭按钮。
func (a *app) windowControls(c *ui.Context, pal *palette) {
	if !ownControls || a.win == nil {
		return
	}
	ui.Row(c).Gap(2).AlignItems(ui.Center).Shrink(0).Children(func() {
		a.controlButton(c, pal, "minimize", "最小化", minimizeIcon, false, a.win.Minimize)
		maximized := a.win.IsMaximized()
		svg, label := maximizeIcon, "最大化"
		if maximized {
			svg, label = restoreIcon, "还原"
		}
		b := ui.ButtonBase(c.Key("maximize")).Size(28, 26).Radius(6).Center().
			TextColor(pal.textMuted).Label(label).Tooltip(label).MaximizeButton()
		styleControl(b, pal, false)
		b.Children(func() { ui.Icon(c, svg).Size(16, 16) })
		if b.Clicked() {
			if maximized {
				a.win.Unmaximize()
			} else {
				a.win.Maximize()
			}
		}
		a.controlButton(c, pal, "close-window", "关闭", closeIcon, true, a.win.Close)
	})
}

func (a *app) controlButton(c *ui.Context, pal *palette, key, label string, svg *ui.SVG, danger bool, fn func()) {
	b := ui.ButtonBase(c.Key(key)).Size(28, 26).Radius(6).Center().
		TextColor(pal.textMuted).Label(label).Tooltip(label)
	styleControl(b, pal, danger)
	b.Children(func() { ui.Icon(c, svg).Size(16, 16) })
	if b.Clicked() {
		fn()
	}
}

func styleControl(b ui.Element, pal *palette, danger bool) {
	switch {
	case danger && (b.Hovered() || b.Pressed()):
		b.Background(pal.danger).TextColor(ui.Hex("#ffffff"))
	case b.Pressed():
		b.Background(pal.rowHover.Alpha(0.12))
	case b.Hovered():
		b.Background(pal.rowHover).TextColor(pal.text)
	}
}

func main() {
	a := newApp()
	mygo.App.WhenReady(func() {
		opts := mygo.WindowOptions{
			Title:     "hoyo-gacha",
			Width:     960,
			Height:    680,
			MinWidth:  720,
			MinHeight: 480,
			// 记住上次关闭时的窗口位置。
			StateKey: "main",
			Content:  ui.View(a.view),
		}
		if ownControls {
			// 隐藏系统标题栏和按钮，由界面自己绘制，保留阴影、圆角和缩放边。
			opts.TitleBarStyle = mygo.TitleBarHidden
			opts.HideWindowButtons = true
			opts.AutoHideMenuBar = true
		} else {
			// macOS 保留交通灯，把它嵌进界面。
			opts.TitleBarStyle = mygo.TitleBarHiddenInset
			opts.TitleBarHeight = 60
		}
		a.win = mygo.NewWindow(opts)
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
