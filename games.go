package main

import "github.com/egoist/mygo/ui"

// game 是一款由本应用管理抽卡记录的游戏。各游戏的实现在同名文件里。
type game struct {
	id           string
	name         string
	tagline      string
	art          *ui.Bitmap
	allTerm      string    // 总抽数的说明，如“全部跃迁”
	pityTerm     string    // 保底标题，如“跃迁保底”
	spec         gachaSpec // 等级与保底规则
	body         func(a *app, c *ui.Context, pal *palette, g *game)
	channelColor func(typ string, pal *palette) ui.Color
	refresh      func(a *app)                          // 不支持自动获取时为空
	fetching     func(a *app) (status string, ok bool) // 刷新状态
	load         func() (*gachaData, error)
	apply        func(a *app, d *gachaData, err error)
	annotate     func(d *gachaData) // 读入后的补充处理，如标记歪
}

// games 是侧边栏里的全部游戏，顺序即显示顺序。在 init 里赋值，避免各游戏
// 的读盘函数经由 gameByID 形成初始化循环。
var games []game

func init() {
	games = []game{genshinGame, starRailGame, zzzGame}
}

// gameByID 返回 id 对应的游戏；没有时返回 nil。
func gameByID(id string) *game {
	for i := range games {
		if games[i].id == id {
			return &games[i]
		}
	}
	return nil
}
