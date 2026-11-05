package main

import "github.com/egoist/mygo/ui"

// zzzGame 是绝区零在应用里的入口。
var zzzGame = game{
	id:           "zzz",
	name:         "绝区零",
	tagline:      "新艾利都的委托档案",
	art:          gameArt("zzz.png"),
	allTerm:      "全部频段",
	pityTerm:     "频段保底",
	spec:         zzzSpec,
	body:         (*app).zzzBody,
	channelColor: zzzChannelColor,
	refresh:      (*app).refreshZZZ,
	fetching:     (*app).zzzFetchState,
	busy:         (*app).zzzBusy,
	load:         loadZZZData,
	apply:        (*app).applyZZZData,
	annotate:     annotateZZZLoaded,
}

// zzzSpec 是绝区零的等级与保底规则。
var zzzSpec = gachaSpec{
	sRank: "4",
	aRank: "3",
	bRank: "2",
	pity:  map[string]int{"3": 80, "103": 80, "5": 80},
}

// loadZZZData 从应用数据目录读取绝区零的抽卡数据。
func loadZZZData() (*gachaData, error) {
	return loadGameData("zzz")
}

// loadZZZDir 从 dir 读取 config.json 指向的绝区零抽卡文件。
func loadZZZDir(dir string) (*gachaData, error) {
	return loadGachaDir(dir, "zzz")
}

// parseZZZ 解析绝区零抽卡导出文件。
func parseZZZ(data []byte) (*gachaData, error) {
	return parseGacha(data, "zzz")
}

// newZZZData 按绝区零频段汇总记录。
func newZZZData(exp *gachaExport) *gachaData {
	return newGachaData(exp, "zzz")
}

func (a *app) applyZZZData(d *gachaData, err error) {
	a.zzz, a.zzzErr, a.zzzLoaded = d, err, true
}

// zzzBody 按数据状态展示绝区零的抽卡概览。
func (a *app) zzzBody(c *ui.Context, pal *palette, g *game) {
	if a.zzzStatus != "" && !a.zzzFetching {
		a.statusRow(c, pal, a.zzzStatus)
	}
	a.gachaBody(c, pal, g, a.zzz, a.zzzErr, a.zzzLoaded, &a.zzzView,
		"先在游戏内打开「调频记录」页，再点右上角「刷新」即可读取最新记录。")
}

// zzzFetchState 返回刷新进度；只有 authkey 校验通过后才返回 ok，
// 避免链接失效时先闪一下进度弹框。
func (a *app) zzzFetchState() (string, bool) {
	return a.zzzStatus, a.zzzFetching
}

// zzzBusy 表示绝区零正在后台刷新，用于禁用刷新按钮。
func (a *app) zzzBusy() bool {
	return a.zzzChecking || a.zzzFetching
}

// refreshZZZ 在后台读取日志并抓取记录，完成后回到主线程刷新界面。
func (a *app) refreshZZZ() {
	if a.zzzBusy() {
		return
	}
	a.zzzChecking = true
	a.zzzStatus = "正在读取游戏日志…"
	go func() {
		res, err := refreshZZZData(
			func(msg string) {
				a.publish(func() { a.zzzStatus = msg })
			},
			func() {
				// authkey 已通过校验，从这里开始显示进度弹框。
				a.publish(func() {
					a.zzzChecking = false
					a.zzzFetching = true
				})
			},
		)
		a.publish(func() {
			a.zzzChecking = false
			a.zzzFetching = false
			if err != nil {
				a.zzzStatus = ""
				a.zzzToast = "刷新失败：" + err.Error()
				return
			}
			d, loadErr := loadZZZData()
			a.applyZZZData(d, loadErr)
			if loadErr != nil {
				a.zzzStatus = ""
				a.zzzToast = "刷新成功，但读取数据失败：" + loadErr.Error()
				return
			}
			a.zzzStatus = res.Message()
		})
	}()
}

// annotateZZZLoaded 给刚读入的绝区零记录标记歪。
func annotateZZZLoaded(d *gachaData) {
	if set, err := loadZZZBanners(); err == nil {
		annotateZZZLost(d, set.Banners)
	}
}

// zzzChannelColor 返回绝区零各频段的标识色。
func zzzChannelColor(typ string, pal *palette) ui.Color {
	purple, teal := channelHues(pal)
	switch typ {
	case "2", "102":
		return pal.attention
	case "3", "103":
		return purple
	case "5":
		return teal
	default:
		return pal.textMuted
	}
}
