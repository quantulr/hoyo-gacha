package main

import "github.com/egoist/mygo/ui"

// genshinGame 是原神在应用里的入口。
var genshinGame = game{
	id:           "genshin",
	name:         "原神",
	tagline:      "提瓦特大陆的旅途记录",
	art:          gameArt("genshin.png"),
	allTerm:      "全部祈愿",
	pityTerm:     "祈愿保底",
	spec:         genshinSpec,
	body:         (*app).genshinBody,
	channelColor: genshinChannelColor,
	refresh:      (*app).refreshGenshin,
	fetching:     (*app).genshinFetchState,
	load:         loadGenshinData,
	apply:        (*app).applyGenshinData,
	annotate:     annotateGenshinLoaded,
}

// genshinSpec 是原神的等级与保底规则。
var genshinSpec = gachaSpec{
	sRank: "5",
	aRank: "4",
	bRank: "3",
	pity:  map[string]int{"302": 80, "100": 20},
}

// loadGenshinData 从应用数据目录读取原神的抽卡数据。
func loadGenshinData() (*gachaData, error) {
	return loadGameData("genshin")
}

func (a *app) applyGenshinData(d *gachaData, err error) {
	a.genshin, a.genshinErr, a.genshinLoaded = d, err, true
}

// genshinBody 按数据状态展示原神的抽卡概览。
func (a *app) genshinBody(c *ui.Context, pal *palette, g *game) {
	if a.genshinStatus != "" && !a.genshinFetching {
		a.statusRow(c, pal, a.genshinStatus)
	}
	a.gachaBody(c, pal, g, a.genshin, a.genshinErr, a.genshinLoaded, &a.genshinView,
		"先在游戏内打开「祈愿记录」页，再点右上角「刷新」即可读取最新记录。")
}

func (a *app) genshinFetchState() (string, bool) {
	return a.genshinStatus, a.genshinFetching
}

// refreshGenshin 在后台读取日志与网页缓存并抓取记录，完成后刷新界面。
func (a *app) refreshGenshin() {
	if a.genshinFetching {
		return
	}
	a.genshinFetching = true
	a.genshinStatus = "正在读取游戏日志与缓存…"
	go func() {
		res, err := refreshGenshinData(func(msg string) {
			a.publish(func() { a.genshinStatus = msg })
		})
		a.publish(func() {
			a.genshinFetching = false
			if err != nil {
				a.genshinStatus = ""
				a.genshinToast = "刷新失败：" + err.Error()
				return
			}
			d, loadErr := loadGenshinData()
			a.applyGenshinData(d, loadErr)
			if loadErr != nil {
				a.genshinStatus = ""
				a.genshinToast = "刷新成功，但读取数据失败：" + loadErr.Error()
				return
			}
			a.genshinStatus = res.Message()
		})
	}()
}

// genshinChannelColor 返回原神各祈愿频段的标识色。
func genshinChannelColor(typ string, pal *palette) ui.Color {
	purple, teal := channelHues(pal)
	switch typ {
	case "301": // 角色活动祈愿
		return pal.attention
	case "302": // 武器活动祈愿
		return purple
	case "500": // 集录祈愿
		return pal.accent
	case "100": // 新手祈愿
		return teal
	default: // 常驻祈愿
		return pal.textMuted
	}
}
