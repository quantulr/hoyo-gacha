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
	load:         loadGenshinData,
	apply:        (*app).applyGenshinData,
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
	a.gachaBody(c, pal, g, a.genshin, a.genshinErr, a.genshinLoaded, &a.genshinView,
		"把 gacha-list-*.json 放到应用数据目录的 genshin 文件夹后重启即可。")
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
