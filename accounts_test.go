package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestGachaAccounts(t *testing.T) {
	root := t.TempDir()
	dir := testGameDir(t, root, "genshin")
	for _, uid := range []string{"1000", "202", "99"} {
		if err := os.WriteFile(filepath.Join(dir, "gacha-list-"+uid+".json"),
			[]byte(`{"uid":"`+uid+`"}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// 不属于账号的文件应被忽略。
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"current":"99"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got := gachaAccounts(root, "genshin")
	want := []string{"99", "202", "1000"}
	if len(got) != len(want) {
		t.Fatalf("账号 %v，期望 %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("第 %d 个账号 %q，期望 %q", i, got[i], want[i])
		}
	}
	if got := gachaAccounts(root, "zzz"); len(got) != 0 {
		t.Errorf("没有数据目录时返回 %v", got)
	}
}

// writeAccountFixtures 写入两个账号，返回根目录。
func writeAccountFixtures(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	dir := testGameDir(t, root, "genshin")
	other := strings.ReplaceAll(genshinFixture, `"uid": "202"`, `"uid": "303"`)
	for uid, data := range map[string]string{"202": genshinFixture, "303": other} {
		if err := os.WriteFile(filepath.Join(dir, "gacha-list-"+uid+".json"), []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeTestConfig(t, root, `[games.genshin]
current = "202"
`)
	return root
}

func TestSwitchGachaAccount(t *testing.T) {
	root := writeAccountFixtures(t)
	a := newApp()
	d, err := loadGachaGame(root, "genshin")
	if err != nil {
		t.Fatal(err)
	}
	a.applyGenshinData(d, err)
	if d == nil || d.UID != "202" {
		t.Fatalf("初始账号 %+v", d)
	}

	if err := switchGachaAccount(root, a, &genshinGame, "303"); err != nil {
		t.Fatal(err)
	}
	if a.genshin == nil || a.genshin.UID != "303" || a.genshin.TotalPulls != 7 {
		t.Errorf("切换后 %+v", a.genshin)
	}
	if got := configCurrent(readTestConfig(t, root), "genshin"); got != "303" {
		t.Errorf("切换后 current = %q", got)
	}

	if err := switchGachaAccount(root, a, &genshinGame, "202"); err != nil {
		t.Fatal(err)
	}
	if a.genshin == nil || a.genshin.UID != "202" {
		t.Errorf("切回后 %+v", a.genshin)
	}
}

func TestAccountMenu(t *testing.T) {
	root := writeAccountFixtures(t)
	a := newApp()
	a.dataRoot = root
	d, err := loadGachaGame(root, "genshin")
	if err != nil {
		t.Fatal(err)
	}
	a.applyGenshinData(d, err)
	tt := ui.NewTester(a.view, 960, 680)

	if !tt.HasText("UID 202") {
		t.Fatalf("标题栏缺少当前账号，现有 %q", tt.Texts())
	}
	if err := tt.Click("UID 202"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("UID 303") {
		t.Fatalf("菜单没有列出其他账号，现有 %q", tt.Texts())
	}
	if err := tt.Click("UID 303"); err != nil {
		t.Fatal(err)
	}
	if a.genshin == nil || a.genshin.UID != "303" {
		t.Fatalf("点击后账号 %+v", a.genshin)
	}
	if got := configCurrent(readTestConfig(t, root), "genshin"); got != "303" {
		t.Errorf("点击后 current = %q", got)
	}
	if !tt.HasText("UID 303") {
		t.Errorf("切换后标题栏未更新，现有 %q", tt.Texts())
	}
}
