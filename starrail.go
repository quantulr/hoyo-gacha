package main

import "github.com/egoist/mygo/ui"

// starRailGame 是崩坏：星穹铁道在应用里的入口。
var starRailGame = game{
	id:           "starrail",
	name:         "崩坏：星穹铁道",
	tagline:      "开拓者的星穹列车手记",
	art:          gameArt("starrail.png"),
	allTerm:      "全部跃迁",
	pityTerm:     "跃迁保底",
	stars:        true,
	spec:         starRailSpec,
	channels:     starRailChannelOrder,
	body:         (*app).starrailBody,
	channelColor: starRailChannelColor,
	refresh:      (*app).refreshStarRail,
	fetching:     (*app).starRailFetchState,
	load:         loadStarRailData,
	apply:        (*app).applyStarRailData,
	annotate:     annotateStarRailLoaded,
}

// starRailSpec 是崩坏：星穹铁道的等级与保底规则。
var starRailSpec = gachaSpec{
	sRank: "5",
	aRank: "4",
	bRank: "3",
	pity:  map[string]int{"12": 80, "22": 80, "2": 50},
}

// loadStarRailData 从应用数据目录读取崩坏：星穹铁道的抽卡数据。
func loadStarRailData() (*gachaData, error) {
	return loadGameData("starrail")
}

// parseStarRail 解析崩坏：星穹铁道抽卡导出文件。
func parseStarRail(data []byte) (*gachaData, error) {
	return parseGacha(data, "starrail")
}

func (a *app) applyStarRailData(d *gachaData, err error) {
	a.starrail, a.starrailErr, a.starrailLoaded = d, err, true
}

// starrailBody 按数据状态展示崩坏：星穹铁道的抽卡概览。
func (a *app) starrailBody(c *ui.Context, pal *palette, g *game) {
	a.gachaBody(c, pal, g, a.starrail, a.starrailErr, a.starrailLoaded, &a.starrailView,
		"导入 gacha-list-*.json 文件，或先在游戏内打开「跃迁记录」页，再点右上角「刷新」获取。")
}

func (a *app) starRailFetchState() (string, bool) {
	return a.starrailStatus, a.starrailFetching
}

// refreshStarRail 在后台读取日志与网页缓存并抓取记录，完成后刷新界面。
func (a *app) refreshStarRail() {
	if a.starrailFetching {
		return
	}
	a.starrailFetching = true
	a.starrailStatus = "正在读取游戏日志与缓存…"
	go func() {
		res, err := refreshStarRailData(func(msg string) {
			a.publish(func() { a.starrailStatus = msg })
		})
		a.publish(func() {
			a.starrailFetching = false
			if err != nil {
				a.starrailStatus = ""
				a.starrailToast = "刷新失败：" + err.Error()
				return
			}
			d, loadErr := loadStarRailData()
			a.applyStarRailData(d, loadErr)
			if loadErr != nil {
				a.starrailStatus = ""
				a.starrailToast = "刷新成功，但读取数据失败：" + loadErr.Error()
				return
			}
			a.starrailStatus = ""
			a.starrailToast = res.Message()
		})
	}()
}

// annotateStarRailLoaded 给刚读入的星穹铁道记录标记歪。
func annotateStarRailLoaded(d *gachaData) {
	if set, err := loadStarRailBanners(); err == nil {
		annotateStarRailLost(d, set.Banners)
	}
}

// starRailChannelColor 返回崩坏：星穹铁道各跃迁频段的标识色。
func starRailChannelColor(typ string, pal *palette) ui.Color {
	purple, teal := channelHues(pal)
	switch typ {
	case "11", "21": // 角色活动跃迁、角色联动跃迁
		return pal.attention
	case "12", "22": // 光锥活动跃迁、光锥联动跃迁
		return purple
	case "2": // 新手跃迁
		return teal
	default: // 常驻跃迁
		return pal.textMuted
	}
}
