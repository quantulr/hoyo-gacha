package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// 设置窗口可以在没有真实窗口的测试中切换页面。
func TestSettingsPages(t *testing.T) {
	a := newApp()
	a.settingsPage = "general"
	tt := ui.NewTester(a.settingsView, 800, 640)

	for _, text := range []string{"通用", "关于", "检查更新", "当前版本", "数据目录"} {
		if !tt.HasText(text) {
			t.Fatalf("通用页缺少 %q，文本 %q", text, tt.Texts())
		}
	}

	if err := tt.Click("关于"); err != nil {
		t.Fatal(err)
	}
	if a.settingsPage != "about" {
		t.Fatalf("点击关于后页面为 %q", a.settingsPage)
	}
	for _, text := range []string{
		"致谢",
		"biuuu/genshin-wish-export",
		"Trrrrw/genshin-wish-export",
		"biuuu/star-rail-warp-export",
		"earthjasonlin/zzz-signal-search-export",
		"Biligame Wiki",
	} {
		if !tt.HasText(text) {
			t.Errorf("关于页缺少 %q，文本 %q", text, tt.Texts())
		}
	}
	if tt.HasText("MyGo") || tt.HasText("Kero") {
		t.Errorf("关于页仍在致谢 MyGo/Kero，文本 %q", tt.Texts())
	}

	if err := tt.Click("通用"); err != nil {
		t.Fatal(err)
	}
	if a.settingsPage != "general" || !tt.HasText("检查更新") {
		t.Fatalf("点击通用后页面为 %q，文本 %q", a.settingsPage, tt.Texts())
	}
}
