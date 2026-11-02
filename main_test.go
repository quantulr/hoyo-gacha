package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// 视图可以在没有窗口的测试中运行，像用户一样点击和按键。
func TestSidebarChoosesGame(t *testing.T) {
	a := newApp()
	tt := ui.NewTester(a.view, 960, 680)

	// 默认展示第一款游戏。
	if a.selected != "genshin" || !tt.HasText("· 提瓦特大陆的旅途记录") {
		t.Fatalf("默认选中 %q，文本 %q", a.selected, tt.Texts())
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
