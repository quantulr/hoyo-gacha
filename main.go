package main

import (
	"fmt"
	"log"
	"runtime"
	"strings"
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

// app 是窗口展示的状态：当前选中的游戏、窗口本身和已读入的数据。
type app struct {
	selected  string
	win       *mygo.Window
	zzz       *zzzData
	zzzErr    error
	zzzLoaded bool
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
			if g.id == "zzz" {
				a.zzzBody(c, pal, g)
			} else {
				a.emptyBody(c, pal, g)
			}
		})
	})
}

// emptyBody 是还没有接入数据时的占位界面。
func (a *app) emptyBody(c *ui.Context, pal *palette, g *game) {
	ui.Column(c).Grow(1).MinHeight(0).Center().Padding(24).Children(func() {
		gameImage(c, g, 56, pal).Margin(0, 0, 18, 0).
			Shadow(0, 2, 8, 0, ui.RGBA(0, 0, 0, 0.12))
		ui.Text(c, "还没有抽卡记录").FontSize(19).FontWeight(650).TextColor(pal.text)
		ui.Text(c, "导入该游戏的抽卡记录后，统计和保底进度会显示在这里。").
			FontSize(13.5).TextColor(pal.textMuted).MaxWidth(380).
			TextAlign(ui.Center).LineHeight(1.45).Margin(8, 0, 0, 0)
	})
}

// recentSCount 是“最近出金”显示的条数。
const recentSCount = 12

// zzzBody 按数据状态展示绝区零的抽卡概览。
func (a *app) zzzBody(c *ui.Context, pal *palette, g *game) {
	switch {
	case !a.zzzLoaded:
		ui.Column(c).Grow(1).MinHeight(0).Center().Gap(12).Children(func() {
			ui.Spinner(c).Label("正在读取抽卡记录")
			ui.Text(c, "正在读取绝区零抽卡记录…").FontSize(13).TextColor(pal.textMuted)
		})
	case a.zzzErr != nil:
		a.messageBody(c, pal, g, "无法读取绝区零抽卡数据", a.zzzErr.Error())
	case a.zzz == nil || a.zzz.TotalPulls == 0:
		a.messageBody(c, pal, g, "还没有抽卡记录", "数据文件里没有可显示的记录。")
	default:
		a.zzzOverview(c, pal, a.zzz)
	}
}

func (a *app) messageBody(c *ui.Context, pal *palette, g *game, title, detail string) {
	ui.Column(c).Grow(1).MinHeight(0).Center().Padding(24).Children(func() {
		gameImage(c, g, 56, pal).Margin(0, 0, 18, 0).
			Shadow(0, 2, 8, 0, ui.RGBA(0, 0, 0, 0.12))
		ui.Text(c, title).FontSize(19).FontWeight(650).TextColor(pal.text)
		ui.Text(c, detail).FontSize(13).TextColor(pal.textMuted).
			MaxWidth(420).TextAlign(ui.Center).LineHeight(1.45).Margin(8, 0, 0, 0)
	})
}

// zzzOverview 是绝区零的仪表盘：概览、频段保底和最近出金。
func (a *app) zzzOverview(c *ui.Context, pal *palette, d *zzzData) {
	ui.Scroll(c).Grow(1).MinHeight(0).Padding(24, 28).Gap(22).Children(func() {
		ui.Column(c).Gap(10).Children(func() {
			meta := "最后记录 " + formatDate(d.LastTime)
			if d.UID != "" {
				meta = "UID " + d.UID + " · " + meta
			}
			sectionHeading(c, pal, "抽卡概览", meta)
			statCards(c, pal, d)
		})
		ui.Column(c).Gap(8).Children(func() {
			sectionHeading(c, pal, "频段保底", "自上次 S 级起")
			for _, ch := range d.Channels {
				channelRow(c, pal, ch)
			}
		})
		ui.Column(c).Gap(8).Children(func() {
			note := fmt.Sprintf("最近 %d 个 · 共 %d 个", min(len(d.SRecords), recentSCount), d.TotalS)
			sectionHeading(c, pal, "最近出金", note)
			list := ui.Column(c).Background(pal.chip).Radius(12).Padding(4, 14).Gap(2).
				Dividers(1, pal.divider)
			list.Children(func() {
				for i, s := range d.SRecords {
					if i >= recentSCount {
						break
					}
					sPullRow(c, pal, s)
				}
			})
		})
	})
}

func sectionHeading(c *ui.Context, pal *palette, title, note string) {
	ui.Row(c).AlignItems(ui.Center).Children(func() {
		ui.Text(c, title).FontSize(14).FontWeight(600).TextColor(pal.text).SingleLine()
		ui.Spacer(c)
		if note != "" {
			ui.Text(c, note).FontSize(11.5).TextColor(pal.textFaint).
				SingleLine().Ellipsis("…").Shrink(1).MinWidth(0)
		}
	})
}

func statCards(c *ui.Context, pal *palette, d *zzzData) {
	cards := []struct {
		label string
		value string
		note  string
		color ui.Color
	}{
		{"总抽数", formatInt(d.TotalPulls), "全部频段", pal.text},
		{"S 级", formatInt(d.TotalS), fmt.Sprintf("出货率 %.2f%%", d.SRate()*100), pal.attention},
		{"A 级", formatInt(d.TotalA), fmt.Sprintf("出货率 %.2f%%", d.ARate()*100), pal.text},
		{"平均出金", averageText(d.Average()), "抽 / S 级", pal.text},
	}
	width, _ := c.Size()
	if width >= 860 {
		ui.Row(c).Gap(10).Children(func() {
			for _, card := range cards {
				statCard(c, pal, card.label, card.value, card.note, card.color)
			}
		})
		return
	}
	for i := 0; i < len(cards); i += 2 {
		ui.Row(c).Gap(10).Children(func() {
			for _, card := range cards[i:min(i+2, len(cards))] {
				statCard(c, pal, card.label, card.value, card.note, card.color)
			}
		})
	}
}

func statCard(c *ui.Context, pal *palette, label, value, note string, color ui.Color) {
	ui.Column(c).Grow(1).MinWidth(0).Background(pal.chip).Radius(10).
		Padding(13, 16).Gap(3).Children(func() {
		ui.Text(c, label).FontSize(11.5).FontWeight(600).TextColor(pal.textMuted).SingleLine()
		ui.Text(c, value).FontSize(23).FontWeight(650).TextColor(color).
			FontFeatures("tnum").SingleLine()
		ui.Text(c, note).FontSize(11.5).TextColor(pal.textFaint).SingleLine().Ellipsis("…")
	})
}

func channelRow(c *ui.Context, pal *palette, ch channelStats) {
	color := channelColor(ch.Type, pal)
	ui.Row(c).Height(58).Shrink(0).Padding(0, 14).Gap(12).AlignItems(ui.Center).
		Background(pal.chip).Radius(10).Children(func() {
		ui.Box(c).Size(8, 8).Shrink(0).Radius(4).Background(color)
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			ui.Text(c, ch.Name).FontSize(13).FontWeight(600).TextColor(pal.text).SingleLine()
			ui.Textf(c, "%s 抽 · S %d · 平均 %s", formatInt(ch.Pulls), ch.S, averageText(ch.Average)).
				FontSize(11.5).TextColor(pal.textFaint).FontFeatures("tnum").
				SingleLine().Ellipsis("…")
		})
		ui.Column(c).Width(150).Shrink(0).AlignItems(ui.End).Gap(5).Children(func() {
			ui.Textf(c, "保底 %d / %d", ch.Pity, ch.HardPity).
				FontSize(11.5).TextColor(pal.textMuted).FontFeatures("tnum").SingleLine()
			pct := float32(ch.Pity) / float32(ch.HardPity)
			bar := ui.Box(c).Width(150).Height(5).Radius(2.5).Background(pal.text.Alpha(0.08))
			bar.Draw(func(p *ui.Painter, r ui.Rect) {
				if pct <= 0 {
					return
				}
				p.Fill(ui.Rect{X: r.X, Y: r.Y, W: r.W * min(pct, 1), H: r.H}, color, r.H/2)
			})
		})
	})
}

func sPullRow(c *ui.Context, pal *palette, s sPull) {
	meta := s.ItemType
	if s.Channel != "" {
		if meta != "" {
			meta += " · "
		}
		meta += s.Channel
	}
	ui.Row(c).Height(38).Shrink(0).Gap(10).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Size(20, 20).Shrink(0).Radius(6).Background(pal.attention.Alpha(0.14)).
			Center().Children(func() {
			ui.Text(c, "S").FontSize(10.5).FontWeight(700).TextColor(pal.attention)
		})
		ui.Text(c, s.Name).FontSize(13.5).FontWeight(500).TextColor(pal.text).
			SingleLine().Ellipsis("…").Shrink(1).MinWidth(0)
		ui.Text(c, meta).FontSize(11.5).TextColor(pal.textFaint).
			SingleLine().Ellipsis("…").Shrink(1).MinWidth(0)
		ui.Spacer(c)
		ui.Textf(c, "第 %d 抽 · %s", s.Pity, formatDate(s.Time)).
			FontSize(11.5).TextColor(pal.textFaint).FontFeatures("tnum").SingleLine().Shrink(0)
	})
}

func channelColor(typ string, pal *palette) ui.Color {
	switch typ {
	case "2", "102":
		return pal.attention
	case "3", "103":
		if pal.dark {
			return ui.Hex("#a99bf0")
		}
		return ui.Hex("#8b7bd8")
	case "5":
		if pal.dark {
			return ui.Hex("#7fc4b8")
		}
		return ui.Hex("#5f9e94")
	default:
		return pal.textMuted
	}
}

func averageText(avg float64) string {
	if avg == 0 {
		return "—"
	}
	return fmt.Sprintf("%.1f", avg)
}

func formatDate(s string) string {
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}

// formatInt 给数字加上千位分隔符。
func formatInt(n int) string {
	s := fmt.Sprint(n)
	if len(s) <= 3 {
		return s
	}
	var b strings.Builder
	pre := len(s) % 3
	if pre > 0 {
		b.WriteString(s[:pre])
	}
	for i := pre; i < len(s); i += 3 {
		if b.Len() > 0 {
			b.WriteByte(',')
		}
		b.WriteString(s[i : i+3])
	}
	return b.String()
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
		win := a.win
		// 数据文件在后台读取，读完再交给界面线程。
		go func() {
			d, err := loadZZZData()
			win.Update(func() {
				a.zzz, a.zzzErr, a.zzzLoaded = d, err, true
			})
		}()
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
