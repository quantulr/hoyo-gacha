package main

import (
	"math/big"
	"path/filepath"
	"sort"

	"github.com/egoist/mygo/ui"
)

// gachaAccounts 返回一款游戏数据目录里已有导出文件的 UID，按数字升序。
func gachaAccounts(root, game string) []string {
	matches, err := filepath.Glob(filepath.Join(gameDataDir(root, game), "gacha-list-*.json"))
	if err != nil {
		return nil
	}
	uids := make([]string, 0, len(matches))
	for _, path := range matches {
		if uid := gachaUIDFromPath(path); uid != "" {
			uids = append(uids, uid)
		}
	}
	sort.Slice(uids, func(i, j int) bool {
		a, okA := new(big.Int).SetString(uids[i], 10)
		b, okB := new(big.Int).SetString(uids[j], 10)
		if okA && okB {
			return a.Cmp(b) < 0
		}
		return uids[i] < uids[j]
	})
	return uids
}

// switchGachaAccount 读入指定账号并交给界面；读入成功会更新统一配置的 current。
func switchGachaAccount(root string, a *app, g *game, uid string) error {
	d, err := loadGachaAccount(root, g.id, uid)
	if err != nil {
		return err
	}
	if g.apply != nil {
		g.apply(a, d, nil)
	}
	return nil
}

// switchAccount 切换当前游戏的账号，失败时保留原来的记录并提示原因。
func (a *app) switchAccount(g *game, uid string) {
	if err := switchGachaAccount(a.dataRoot, a, g, uid); err != nil {
		a.toast = "切换账号失败：" + err.Error()
	}
}

// accountMenuOpen 返回账号菜单的展开状态；菜单需要跨帧保持同一个指针。
func (a *app) accountMenuOpen(game string) *bool {
	if a.accountOpen == nil {
		a.accountOpen = map[string]*bool{}
	}
	open := a.accountOpen[game]
	if open == nil {
		open = new(bool)
		a.accountOpen[game] = open
	}
	return open
}

// accountMenu 在标题栏显示当前账号，展开后列出该游戏已有的账号。
func (a *app) accountMenu(c *ui.Context, pal *palette, g *game, d *gachaData) {
	if a.dataRoot == "" || d == nil || d.UID == "" {
		return
	}
	open := a.accountMenuOpen(g.id)
	m := ui.MenuBase(c.Key("account-"+g.id), open)
	trigger := m.Trigger.Height(24).Shrink(0).Padding(0, 8).Gap(4).
		Radius(6).AlignItems(ui.Center).Label("切换账号").Tooltip("切换账号")
	bg, fg := pal.chip, pal.textMuted
	switch {
	case *open:
		bg, fg = pal.rowSelected, pal.text
	case trigger.Hovered():
		bg, fg = pal.rowHover, pal.text
	}
	trigger.Background(bg)
	trigger.Children(func() {
		ui.Text(c, "UID "+d.UID).FontSize(12).FontWeight(500).TextColor(fg).SingleLine()
		alignIconText(ui.Icon(c, chevronDownIcon).Size(10, 10).TextColor(fg).Shrink(0))
	})
	m.Popup(func(panel ui.Element) {
		panel.MinWidth(148).Padding(4).Radius(9).Background(pal.card).
			Border(1, pal.text.Alpha(0.12)).
			Shadow(0, 8, 24, 0, ui.RGBA(0, 0, 0, 0.18))
		for _, uid := range gachaAccounts(a.dataRoot, g.id) {
			item := m.RadioItem(uid == d.UID).Padding(7, 10).Radius(6)
			if item.Highlighted() {
				item.Background(pal.rowHover)
			}
			item.Children(func() {
				ui.Text(c, "UID "+uid).FontSize(12.5).TextColor(pal.text).SingleLine().Grow(1)
				if uid == d.UID {
					alignIconText(ui.Icon(c, checkIcon).Size(12, 12).TextColor(pal.accent).Shrink(0))
				}
			})
			if item.Clicked() && uid != d.UID {
				a.switchAccount(g, uid)
			}
		}
	})
}
