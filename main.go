package main

import (
	"fmt"
	"log"
	"runtime"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater/native"
	"github.com/egoist/mygo/ui"
)

// ownControls 表示由界面绘制窗口按钮，而不是交给系统标题栏。
var ownControls = runtime.GOOS != "darwin"

// gachaView 是抽卡页的视图状态。
type gachaView struct {
	page     int    // 0 概览，1 记录
	chanType string // 记录页选中的卡池频道
	filter   int    // 记录页筛选：0 仅 S，1 S+A，2 全部
}

// app 是窗口展示的状态：当前选中的游戏、窗口本身和已读入的数据。
type app struct {
	selected string
	win      *mygo.Window

	settingsWin  *mygo.Window
	settingsPage string // 设置窗口当前页：general 或 about

	importing bool   // 正在选择或写入导入文件
	toast     string // 待显示的导入结果，显示后清空

	genshin       *gachaData
	genshinErr    error
	genshinLoaded bool
	genshinView   gachaView

	genshinFetching bool   // 正在从游戏接口刷新
	genshinStatus   string // 刷新进度或结果
	genshinToast    string // 待显示的请求错误，显示后清空

	starrail       *gachaData
	starrailErr    error
	starrailLoaded bool
	starrailView   gachaView

	starrailFetching bool   // 正在从游戏接口刷新
	starrailStatus   string // 刷新进度或结果
	starrailToast    string // 待显示的请求错误，显示后清空

	zzz       *gachaData
	zzzErr    error
	zzzLoaded bool
	zzzView   gachaView

	zzzFetching bool   // 正在从游戏接口刷新
	zzzStatus   string // 刷新进度或结果
	zzzToast    string // 待显示的请求错误，显示后清空
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
	if a.genshinToast != "" {
		c.Toast(a.genshinToast)
		a.genshinToast = ""
	}
	if a.zzzToast != "" {
		// 每个请求错误只弹一次，避免每帧重复播报。
		c.Toast(a.zzzToast)
		a.zzzToast = ""
	}
	if a.starrailToast != "" {
		c.Toast(a.starrailToast)
		a.starrailToast = ""
	}
	if a.toast != "" {
		c.Toast(a.toast)
		a.toast = ""
	}
	a.shortcuts(c)

	base, glow := backdrop(pal)
	c.Root().LinearGradient(base)
	row := ui.Row(c).Fill().AlignItems(ui.Stretch).LinearGradient(glow)
	row.Children(func() {
		a.sidebar(c, pal)
		a.content(c, pal, a.current())
	})
	if status, ok := a.refreshStatus(); ok {
		a.refreshOverlay(c, pal, status)
	}
}

// shortcuts 让 Cmd/Ctrl+1、+2、+3 切换游戏，和 Kero 的会话快捷键一致。
func (a *app) shortcuts(c *ui.Context) {
	for i := range games {
		if c.Shortcut(ui.Cmd, ui.Key1+ui.Key(i)) {
			a.selected = games[i].id
		}
	}
	if c.Shortcut(ui.Cmd, ui.KeyComma) {
		a.openSettings()
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
			alignIconText(ui.Image(c, appIcon).Size(26, 26).Shrink(0).Radius(6).
				Border(0.5, ui.RGBA(255, 255, 255, 0.18)))
			ui.Column(c).Gap(1).Children(func() {
				ui.Text(c, "Hoyo Gacha").FontSize(13.5).FontWeight(600).TextColor(pal.text).SingleLine()
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
		// 侧边栏底部与 Kero 一样，放一个安静的工具按钮。
		ui.Row(c).Height(36).Shrink(0).Padding(0, 12, 4, 12).Gap(2).
			AlignItems(ui.Center).DragWindow().Children(func() {
			iconButton(c, "settings", settingsIcon, "设置", pal, a.openSettings)
		})
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
		alignIconText(gameImage(c, g, 18, pal))
		ui.Text(c, g.name).FontSize(13.5).FontWeight(500).TextColor(pal.text).
			SingleLine().Ellipsis("…").Shrink(1).MinWidth(0)
	})
	if row.Clicked() && !selected {
		a.selected = g.id
	}
}

// alignIconText 把图标下移 1 DIP，补偿字体行框中偏高的视觉中心，
// 让图标与同一行的文字在视觉上垂直居中。
func alignIconText(e ui.Element) ui.Element {
	return e.Translate(0, 1)
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
				alignIconText(gameImage(c, g, 18, pal))
				ui.Text(c, g.name).FontSize(14).FontWeight(600).TextColor(pal.text).SingleLine()
				ui.Text(c, "· "+g.tagline).FontSize(12.5).TextColor(pal.textFaint).
					SingleLine().Ellipsis("…").Shrink(1).MinWidth(0)
				ui.Spacer(c)
				if g.load != nil && g.apply != nil {
					a.importButton(c, pal, g)
				}
				if g.refresh != nil && g.fetching != nil {
					_, fetching := g.fetching(a)
					a.refreshButton(c, pal, g.id+"-refresh", fetching, func() { g.refresh(a) })
				}
				a.windowControls(c, pal)
			})
			if g.body != nil {
				g.body(a, c, pal, g)
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

// gachaBody 按数据状态展示一款游戏的抽卡概览和记录。
func (a *app) gachaBody(c *ui.Context, pal *palette, g *game, d *gachaData, err error, loaded bool,
	view *gachaView, emptyTip string) {
	switch {
	case !loaded:
		ui.Column(c).Grow(1).MinHeight(0).Center().Gap(12).Children(func() {
			ui.Spinner(c).Label("正在读取抽卡记录")
			ui.Textf(c, "正在读取%s抽卡记录…", g.name).FontSize(13).TextColor(pal.textMuted)
		})
	case err != nil:
		a.messageBody(c, pal, g, "无法读取"+g.name+"抽卡数据", err.Error(), nil)
	case d == nil || d.TotalPulls == 0:
		a.messageBody(c, pal, g, "还没有抽卡记录", emptyTip, func() { a.importGacha(g) })
	default:
		ui.Column(c).Grow(1).MinHeight(0).Children(func() {
			ui.Row(c).Shrink(0).Padding(14, 28, 8, 28).Children(func() {
				viewSwitch(c, pal, &view.page)
			})
			if view.page == 1 {
				a.gachaRecords(c, pal, g, d, view)
			} else {
				a.gachaOverview(c, pal, g, d)
			}
		})
	}
}

// headerButton 是标题栏里的安静按钮。
func (a *app) headerButton(c *ui.Context, pal *palette, key, label, busyLabel string, busy bool, action func()) {
	text := label
	if busy {
		text = busyLabel
	}
	b := ui.ButtonBase(c.Key(key)).Height(24).Shrink(0).Padding(0, 10).
		Radius(6).AlignItems(ui.Center).Label(text).Disabled(busy)
	bg := pal.chip
	fg := pal.textMuted
	switch {
	case busy:
		fg = pal.textFaint
	case b.Hovered():
		bg = pal.rowHover
		fg = pal.text
	}
	b.Background(bg)
	b.Children(func() {
		ui.Text(c, text).FontSize(12).FontWeight(500).TextColor(fg).SingleLine()
	})
	if b.Clicked() {
		action()
	}
}

// refreshButton 重新从游戏日志抓取抽卡记录；抓取期间禁用。
func (a *app) refreshButton(c *ui.Context, pal *palette, key string, fetching bool, refresh func()) {
	a.headerButton(c, pal, key, "刷新", "刷新中…", fetching, refresh)
}

// importButton 从 gacha-list-*.json 导入当前游戏的抽卡记录。
func (a *app) importButton(c *ui.Context, pal *palette, g *game) {
	a.headerButton(c, pal, g.id+"-import", "导入", "导入中…", a.importing, func() {
		a.importGacha(g)
	})
}

// refreshStatus 返回正在进行的刷新状态；没有刷新时 ok 为 false。
func (a *app) refreshStatus() (string, bool) {
	for i := range games {
		if games[i].fetching == nil {
			continue
		}
		if status, ok := games[i].fetching(a); ok {
			return status, true
		}
	}
	return "", false
}

// refreshOverlay 在刷新期间盖住窗口，显示旋转动画和当前进度。
func (a *app) refreshOverlay(c *ui.Context, pal *palette, status string) {
	ui.Overlay(c, func() {
		ui.Box(c).Absolute().Left(0).Top(0).Right(0).Bottom(0).Center().
			Modal().Background(ui.RGBA(0, 0, 0, 0.35)).Children(func() {
			ui.Column(c).Role(ui.RoleDialog).Label("正在刷新抽卡记录").
				Width(360).Padding(18, 20).Gap(14).Radius(14).
				Background(pal.card).Shadow(0, 12, 36, 0, ui.RGBA(0, 0, 0, 0.28)).
				Children(func() {
					ui.Row(c).Gap(14).AlignItems(ui.Center).Children(func() {
						ui.Spinner(c).Size(26, 26).Shrink(0).TextColor(pal.accent).
							Label("正在刷新抽卡记录")
						ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
							ui.Text(c, "正在刷新抽卡记录").FontSize(14).FontWeight(600).
								TextColor(pal.text).SingleLine()
							ui.Text(c, status).FontSize(12.5).TextColor(pal.textMuted).
								SingleLine().Ellipsis("…")
						})
					})
				})
		})
	})
}

// publish 把后台结果交给界面线程；没有窗口时（测试）直接执行。
func (a *app) publish(fn func()) {
	if a.win != nil {
		a.win.Update(fn)
		return
	}
	fn()
}

// segmented 是一组分段选择按钮。
func segmented(c *ui.Context, pal *palette, sel *int, label string, labels []string) {
	parts := ui.SegmentedBase(c, sel, len(labels))
	parts.Track.Padding(2).Radius(9).Background(pal.chip).Label(label)
	parts.Track.Children(func() {
		for i, text := range labels {
			s := parts.Segment(i).Height(26).Padding(0, 12).Radius(7).Label(text)
			on := i == *sel
			switch {
			case on:
				s.Background(pal.rowSelected)
				chosen(s, pal)
			case s.Hovered():
				s.Background(pal.rowHover)
			}
			color := pal.textMuted
			if on {
				color = pal.text
			}
			s.Children(func() {
				ui.Text(c, text).FontSize(12).FontWeight(500).TextColor(color).SingleLine()
			})
		}
	})
}

// viewSwitch 是概览与记录之间的切换。
func viewSwitch(c *ui.Context, pal *palette, sel *int) {
	segmented(c, pal, sel, "视图", []string{"概览", "记录"})
}

// rankFilter 是记录页的等级筛选。原神/星穹铁道用当前频段的星级，
// 绝区零用 S/A/B。
func rankFilter(c *ui.Context, pal *palette, sel *int, g *game, ch *channelStats) {
	labels := []string{"仅 S", "S+A", "全部"}
	if g.stars {
		rank := g.spec.rankFor(ch.Type)
		labels = []string{"仅 " + rank.s + " 星", rank.s + "+" + rank.a + " 星", "全部"}
	}
	segmented(c, pal, sel, "筛选", labels)
}

func (a *app) messageBody(c *ui.Context, pal *palette, g *game, title, detail string, action func()) {
	ui.Column(c).Grow(1).MinHeight(0).Center().Padding(24).Children(func() {
		gameImage(c, g, 56, pal).Margin(0, 0, 18, 0).
			Shadow(0, 2, 8, 0, ui.RGBA(0, 0, 0, 0.12))
		ui.Text(c, title).FontSize(19).FontWeight(650).TextColor(pal.text)
		ui.Text(c, detail).FontSize(13).TextColor(pal.textMuted).
			MaxWidth(420).TextAlign(ui.Center).LineHeight(1.45).Margin(8, 0, 0, 0)
		if action != nil {
			importLabel := "导入记录"
			if a.importing {
				importLabel = "导入中…"
			}
			b := ui.ButtonBase(c.Key("import-"+g.id)).Height(30).Shrink(0).Padding(0, 16).
				Radius(7).Center().AlignItems(ui.Center).Label(importLabel).
				Disabled(a.importing).Margin(18, 0, 0, 0)
			bg := pal.accent
			switch {
			case b.Pressed():
				bg = bg.Mix(ui.Hex("#000000"), 0.08)
			case b.Hovered():
				bg = bg.Mix(ui.Hex("#ffffff"), 0.08)
			}
			b.Background(bg)
			raised(b, pal)
			b.Children(func() {
				ui.Text(c, importLabel).FontSize(13).FontWeight(500).
					TextColor(ui.Hex("#ffffff")).SingleLine()
			})
			if b.Clicked() {
				action()
			}
		}
	})
}

// gachaOverview 是仪表盘：概览、保底和最近出金。
func (a *app) gachaOverview(c *ui.Context, pal *palette, g *game, d *gachaData) {
	ui.Scroll(c).Grow(1).MinHeight(0).Padding(14, 28, 24, 28).Gap(22).Children(func() {
		ui.Column(c).Gap(10).Children(func() {
			meta := "最后记录 " + formatDate(d.LastTime)
			if d.UID != "" {
				meta = "UID " + d.UID + " · " + meta
			}
			sectionHeading(c, pal, "抽卡概览", meta)
			statCards(c, pal, g, d)
		})
		if d.HasLostChannels() {
			ui.Column(c).Gap(8).Children(func() {
				sectionHeading(c, pal, "各卡池歪卡率", "小保底 · 大保底不计入分母")
				for _, ch := range d.Channels {
					if ch.LostTracked {
						lostRateRow(c, pal, g, ch)
					}
				}
			})
		}
		ui.Column(c).Gap(8).Children(func() {
			// 各频段的最高等级不同（千星奇域常驻颂愿没有 5 星），这里不写具体星级。
			sectionHeading(c, pal, g.pityTerm, "自上次出金起")
			for _, ch := range d.Channels {
				channelRow(c, pal, g, ch)
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
					sPullRow(c, pal, g, s)
				}
			})
		})
	})
}

// gachaRecords 展示选中频段的完整抽卡记录。
func (a *app) gachaRecords(c *ui.Context, pal *palette, g *game, d *gachaData, view *gachaView) {
	if len(d.Channels) == 0 {
		return
	}
	ch := recordChannel(d, view)
	recs := filterPullRanks(ch.Records, view.filter)
	ui.Column(c).Grow(1).MinHeight(0).Padding(14, 28, 20, 28).Gap(12).Children(func() {
		sectionHeading(c, pal, "卡池记录", fmt.Sprintf("共 %s 条 · 最新在前", formatInt(len(recs))))
		ui.Row(c).AlignItems(ui.Start).Gap(12).Children(func() {
			a.channelChips(c, pal, g, d, ch, view).Grow(1)
			rankFilter(c, pal, &view.filter, g, ch)
		})
		panel := ui.Column(c).Grow(1).MinHeight(0).Background(pal.chip).Radius(12).
			Padding(4, 14).Clip()
		panel.Children(func() {
			key := fmt.Sprintf("gacha-records-%s-%s-%d", g.id, ch.Type, view.filter)
			list := ui.List(c.Key(key), nil, len(recs), func(i int) {
				recordRow(c, pal, g, recs[i])
			}).Grow(1).MinHeight(0).Label("抽卡记录")
			list.Children(func() {
				if len(recs) == 0 {
					msg := "没有记录"
					if len(ch.Records) > 0 {
						msg = "没有符合筛选的记录"
					}
					ui.Text(c, msg).FontSize(13).TextColor(pal.textFaint).Padding(12)
				}
			})
		})
	})
}

// recordChannel 返回记录页选中的频段，默认第一个。
func recordChannel(d *gachaData, view *gachaView) *channelStats {
	for i := range d.Channels {
		if d.Channels[i].Type == view.chanType {
			return &d.Channels[i]
		}
	}
	return &d.Channels[0]
}

func (a *app) channelChips(c *ui.Context, pal *palette, g *game, d *gachaData, selected *channelStats, view *gachaView) ui.Element {
	return ui.Row(c).Wrap().Gap(6).Children(func() {
		for i := range d.Channels {
			ch := &d.Channels[i]
			color := g.channelColor(ch.Type, pal)
			on := ch.Type == selected.Type
			b := ui.ButtonBase(c.Key("gacha-chan-"+g.id+"-"+ch.Type)).Height(28).Padding(0, 11).Gap(6).
				Radius(8).AlignItems(ui.Center).Label(ch.Name)
			bg := pal.chip
			fg := pal.textMuted
			weight := 500
			switch {
			case on:
				bg = color.Alpha(0.18)
				fg = pal.text
				weight = 600
				// 常驻频段的颜色接近灰色，仅靠浅色底不够明显。
				b.Border(1, color.Alpha(0.4))
			case b.Hovered():
				bg = pal.rowHover
				fg = pal.text
			}
			b.Background(bg)
			b.Children(func() {
				ui.Box(c).Size(6, 6).Shrink(0).Radius(3).Background(color)
				ui.Text(c, ch.Name).FontSize(12).FontWeight(weight).TextColor(fg).SingleLine()
				ui.Text(c, formatInt(ch.Pulls)).FontSize(11).TextColor(pal.textFaint).
					FontFeatures("tnum").SingleLine()
			})
			if b.Clicked() && !on {
				view.chanType = ch.Type
			}
		}
	})
}

func recordRow(c *ui.Context, pal *palette, g *game, r pullRecord) {
	fg, bg := rankColors(r.Tier, pal)
	ui.Row(c).Height(38).Shrink(0).Gap(10).AlignItems(ui.Center).
		BorderWidth(0, 0, 1, 0).BorderColor(pal.divider).Children(func() {
		ui.Box(c).Size(20, 20).Shrink(0).Radius(6).Background(bg).Center().Children(func() {
			ui.Text(c, g.rankBadge(r.RankType, r.Tier)).FontSize(10.5).FontWeight(700).TextColor(fg)
		})
		ui.Text(c, r.Name).FontSize(13.5).FontWeight(500).TextColor(pal.text).
			SingleLine().Ellipsis("…").Shrink(1).MinWidth(0)
		if r.Tier == 0 && r.Lost {
			ui.Box(c).Height(16).Shrink(0).Padding(0, 5).Radius(4).
				Background(pal.danger.Alpha(0.12)).Center().Children(func() {
				ui.Text(c, "歪").FontSize(10).FontWeight(650).TextColor(pal.danger).SingleLine()
			})
		}
		ui.Text(c, r.ItemType).FontSize(11.5).TextColor(pal.textFaint).SingleLine().Shrink(0)
		ui.Spacer(c)
		ui.Textf(c, "第 %d 抽", r.Pity).FontSize(11.5).TextColor(pal.textMuted).
			FontFeatures("tnum").Width(58).TextAlign(ui.End)
		ui.Text(c, formatDateTime(r.Time)).FontSize(11.5).TextColor(pal.textFaint).
			FontFeatures("tnum").SingleLine().Shrink(0)
	})
}

func rankColors(tier int, pal *palette) (fg, bg ui.Color) {
	switch tier {
	case 0:
		return pal.attention, pal.attention.Alpha(0.14)
	case 1:
		c := ui.Hex("#8b7bd8")
		if pal.dark {
			c = ui.Hex("#a99bf0")
		}
		return c, c.Alpha(0.14)
	default:
		return pal.textFaint, pal.text.Alpha(0.06)
	}
}

func rankLabel(tier int) string {
	switch tier {
	case 0:
		return "S"
	case 1:
		return "A"
	default:
		return "B"
	}
}

// rankName 返回等级在统计中的称呼，如原神/星穹铁道的「5 星」或绝区零的「S 级」。
func (g *game) rankName(tier int) string {
	if g.stars {
		return g.spec.rankFor("").rankAt(tier) + " 星"
	}
	return rankLabel(tier) + " 级"
}

// rankSymbol 返回频段等级在紧凑文本中的称呼，星级按频段规则换算。
func (g *game) rankSymbol(typ string, tier int) string {
	if g.stars {
		return g.spec.rankFor(typ).rankAt(tier) + " 星"
	}
	return rankLabel(tier)
}

// rankBadge 返回记录行里的等级徽标，只显示星级数字。原神/星穹铁道用
// 记录的原始星级，这样千星奇域常驻颂愿的 4 星也能正确显示。
func (g *game) rankBadge(rankType string, tier int) string {
	if g.stars {
		if rankType != "" {
			return rankType
		}
		return g.spec.rankFor("").rankAt(tier)
	}
	return rankLabel(tier)
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

func statCards(c *ui.Context, pal *palette, g *game, d *gachaData) {
	lost, smallPity := d.LostStats()
	lostValue, lostNote, lostColor := "—", "暂无可判定卡池", pal.textFaint
	if smallPity > 0 {
		lostValue = fmt.Sprintf("%.2f%%", d.LostRate()*100)
		lostNote = fmt.Sprintf("%d 歪 / %d 次小保底", lost, smallPity)
		lostColor = pal.text
		if lost > 0 {
			lostColor = pal.danger
		}
	}
	sCount, aCount := d.TotalS, d.TotalA
	sRate, aRate := d.SRate(), d.ARate()
	average := d.Average()
	if g.stars {
		// 原神/星穹铁道按真实星级统计；千星奇域常驻颂愿的 4 星也归入 4 星。
		sCount = d.RankCounts[g.spec.sRank]
		aCount = d.RankCounts[g.spec.aRank]
		if d.TotalPulls > 0 {
			sRate = float64(sCount) / float64(d.TotalPulls)
			aRate = float64(aCount) / float64(d.TotalPulls)
		}
		average = 0
		if sCount > 0 {
			average = float64(d.TotalPulls) / float64(sCount)
		}
	}
	cards := []struct {
		label string
		value string
		note  string
		color ui.Color
	}{
		{"总抽数", formatInt(d.TotalPulls), g.allTerm, pal.text},
		{g.rankName(0), formatInt(sCount), fmt.Sprintf("出货率 %.2f%%", sRate*100), pal.attention},
		{g.rankName(1), formatInt(aCount), fmt.Sprintf("出货率 %.2f%%", aRate*100), pal.text},
		{"平均出金", averageText(average), "抽 / " + g.rankName(0), pal.text},
		{"歪卡率", lostValue, lostNote, lostColor},
	}
	cols := 2
	width, _ := c.Size()
	switch {
	case width >= 980:
		cols = len(cards)
	case width >= 600:
		cols = 3
	}
	ui.Grid(c).Columns(cols).Gap(10).Children(func() {
		for _, card := range cards {
			statCard(c, pal, card.label, card.value, card.note, card.color)
		}
	})
}

func statCard(c *ui.Context, pal *palette, label, value, note string, color ui.Color) {
	ui.Column(c).MinWidth(0).Background(pal.chip).Radius(10).
		Padding(13, 16).Gap(3).Children(func() {
		ui.Text(c, label).FontSize(11.5).FontWeight(600).TextColor(pal.textMuted).SingleLine()
		ui.Text(c, value).FontSize(23).FontWeight(650).TextColor(color).
			FontFeatures("tnum").SingleLine()
		ui.Text(c, note).FontSize(11.5).TextColor(pal.textFaint).SingleLine().Ellipsis("…")
	})
}

func channelRow(c *ui.Context, pal *palette, g *game, ch channelStats) {
	color := g.channelColor(ch.Type, pal)
	ui.Row(c).Height(58).Shrink(0).Padding(0, 14).Gap(12).AlignItems(ui.Center).
		Background(pal.chip).Radius(10).Children(func() {
		ui.Box(c).Size(8, 8).Shrink(0).Radius(4).Background(color)
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			ui.Text(c, ch.Name).FontSize(13).FontWeight(600).TextColor(pal.text).SingleLine()
			ui.Textf(c, "%s 抽 · %s %d · 平均 %s", formatInt(ch.Pulls), g.rankSymbol(ch.Type, 0), ch.S, averageText(ch.Average)).
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

func lostRateRow(c *ui.Context, pal *palette, g *game, ch channelStats) {
	color := g.channelColor(ch.Type, pal)
	value, detail, fg := "—", "暂无小保底记录", pal.textFaint
	rate := ch.LostRate()
	if ch.SmallPityS > 0 {
		value = fmt.Sprintf("%.2f%%", rate*100)
		detail = fmt.Sprintf("%d 歪 / %d 次小保底", ch.LostS, ch.SmallPityS)
		fg = pal.text
		if ch.LostS > 0 {
			fg = pal.danger
		}
	}
	ui.Row(c).Height(52).Shrink(0).Padding(0, 14).Gap(12).AlignItems(ui.Center).
		Background(pal.chip).Radius(10).Children(func() {
		ui.Box(c).Size(8, 8).Shrink(0).Radius(4).Background(color)
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			ui.Text(c, ch.Name).FontSize(13).FontWeight(600).TextColor(pal.text).SingleLine()
			ui.Text(c, detail).FontSize(11.5).TextColor(pal.textFaint).FontFeatures("tnum").
				SingleLine().Ellipsis("…")
		})
		ui.Column(c).Width(150).Shrink(0).AlignItems(ui.End).Gap(6).Children(func() {
			ui.Text(c, value).FontSize(13).FontWeight(650).TextColor(fg).
				FontFeatures("tnum").SingleLine()
			pct := float32(rate)
			bar := ui.Box(c).Width(150).Height(5).Radius(2.5).Background(pal.text.Alpha(0.08))
			bar.Draw(func(p *ui.Painter, r ui.Rect) {
				if pct <= 0 {
					return
				}
				p.Fill(ui.Rect{X: r.X, Y: r.Y, W: r.W * min(pct, 1), H: r.H}, pal.danger, r.H/2)
			})
		})
	})
}

func sPullRow(c *ui.Context, pal *palette, g *game, s sPull) {
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
			ui.Text(c, g.rankBadge(s.Rank, 0)).FontSize(10.5).FontWeight(700).TextColor(pal.attention)
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

func formatDateTime(s string) string {
	if len(s) >= 16 {
		return s[:16]
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

// iconButton 是工具栏里的安静方形图标按钮。
func iconButton(c *ui.Context, key string, svg *ui.SVG, label string, pal *palette, fn func()) {
	b := ui.ButtonBase(c.Key(key)).Size(28, 26).Radius(6).Center().
		TextColor(pal.textMuted).Label(label).Tooltip(label)
	if b.Pressed() {
		b.Background(pal.rowHover.Alpha(0.12))
	} else if b.Hovered() {
		b.Background(pal.rowHover)
		b.TextColor(pal.text)
	}
	b.Children(func() { ui.Icon(c, svg).Size(16, 16) })
	if b.Clicked() {
		fn()
	}
}

// windowControls 画出主窗口的最小化、最大化/还原和关闭按钮。
func (a *app) windowControls(c *ui.Context, pal *palette) {
	a.windowControlsFor(c, pal, a.win)
}

// windowControlsFor 画出指定窗口的按钮，供主窗口和设置窗口共用。
func (a *app) windowControlsFor(c *ui.Context, pal *palette, win *mygo.Window) {
	if !ownControls || win == nil {
		return
	}
	ui.Row(c).Gap(2).AlignItems(ui.Center).Shrink(0).Children(func() {
		a.controlButton(c, pal, "minimize", "最小化", minimizeIcon, false, win.Minimize)
		maximized := win.IsMaximized()
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
				win.Unmaximize()
			} else {
				win.Maximize()
			}
		}
		a.controlButton(c, pal, "close-window", "关闭", closeIcon, true, win.Close)
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

// appWindowOptions 统一主窗口和设置窗口的标题栏风格。
func appWindowOptions(opts mygo.WindowOptions) mygo.WindowOptions {
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
	return opts
}

func main() {
	// 原生更新窗口，避免为更新功能额外依赖 WebView。
	mygo.Use(native.Plugin)
	a := newApp()
	mygo.App.WhenReady(func() {
		opts := appWindowOptions(mygo.WindowOptions{
			Title:     "Hoyo Gacha",
			Width:     960,
			Height:    680,
			MinWidth:  720,
			MinHeight: 480,
			// 记住上次关闭时的窗口位置。
			StateKey: "main",
			Content:  ui.View(a.view),
		})
		a.win = mygo.NewWindow(opts)
		win := a.win
		// 数据文件在后台读取，读完再交给界面线程。
		go func() {
			type result struct {
				game *game
				data *gachaData
				err  error
			}
			results := make([]result, 0, len(games))
			for i := range games {
				g := &games[i]
				if g.load == nil {
					continue
				}
				d, err := g.load()
				results = append(results, result{g, d, err})
			}
			win.Update(func() {
				for _, r := range results {
					if r.game.apply != nil {
						r.game.apply(a, r.data, r.err)
					}
				}
			})
		}()
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
