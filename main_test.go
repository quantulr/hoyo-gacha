package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// 视图可以在没有窗口的测试中运行，像用户一样点击和按键。
func TestSidebarChoosesGame(t *testing.T) {
	a := newApp()
	tt := ui.NewTester(a.view, 960, 680)

	// 默认展示第一款游戏，侧边栏底部有设置按钮。
	if a.selected != "genshin" || !tt.HasText("· 提瓦特大陆的旅途记录") {
		t.Fatalf("默认选中 %q，文本 %q", a.selected, tt.Texts())
	}
	if _, ok := tt.Find("设置"); !ok {
		t.Fatalf("侧边栏缺少设置按钮，文本 %q", tt.Texts())
	}

	if err := tt.Click("绝区零"); err != nil {
		t.Fatal(err)
	}
	if a.selected != "zzz" || !tt.HasText("· 新艾利都的委托档案") {
		t.Errorf("点击后选中 %q，文本 %q", a.selected, tt.Texts())
	}

	// Cmd/Ctrl+数字切换游戏。
	tt.Key(ui.Cmd, ui.Key2)
	if a.selected != "starrail" || !tt.HasText("· 开拓者的星穹列车手记") {
		t.Errorf("按快捷键后选中 %q，文本 %q", a.selected, tt.Texts())
	}
}

// 单次歪显示「歪」，连歪用中文数字标注是连续第几次。
func TestLostBadge(t *testing.T) {
	cases := map[int]string{
		0:   "歪",
		1:   "歪",
		2:   "二连歪",
		3:   "三连歪",
		10:  "十连歪",
		11:  "十一连歪",
		20:  "二十连歪",
		99:  "九十九连歪",
		100: "100连歪",
	}
	for streak, want := range cases {
		if got := lostBadge(streak); got != want {
			t.Errorf("lostBadge(%d) = %q，期望 %q", streak, got, want)
		}
	}
}

// 记录行徽标只显示星级数字，绝区零仍用 S/A/B。
func TestRankBadge(t *testing.T) {
	cases := []struct {
		game     *game
		rankType string
		tier     int
		want     string
	}{
		{&genshinGame, "5", 0, "5"},
		{&genshinGame, "4", 1, "4"},
		{&genshinGame, "3", 2, "3"},
		{&genshinGame, "4", 0, "4"}, // 千星奇域常驻颂愿的最高档是 4 星
		{&starRailGame, "5", 0, "5"},
		{&zzzGame, "4", 0, "S"},
		{&zzzGame, "3", 1, "A"},
		{&zzzGame, "2", 2, "B"},
	}
	for _, tc := range cases {
		if got := tc.game.rankBadge(tc.rankType, tc.tier); got != tc.want {
			t.Errorf("%s rankBadge(%q, %d) = %q，期望 %q",
				tc.game.id, tc.rankType, tc.tier, got, tc.want)
		}
	}
}
