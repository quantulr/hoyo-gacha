package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// testGameDir 创建 root/data/<game> 并返回该目录。
func testGameDir(t *testing.T, root, game string) string {
	t.Helper()
	dir := gameDataDir(root, game)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// writeTestConfig 直接写入一段 TOML 配置。
func writeTestConfig(t *testing.T, root, text string) {
	t.Helper()
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(gameConfigPath(root), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readTestConfig(t *testing.T, root string) map[string]any {
	t.Helper()
	cfg, err := loadGameConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestGameConfigCurrent(t *testing.T) {
	root := t.TempDir()
	writeTestConfig(t, root, `[games.genshin]
current = "202"
proxyPort = 8325
`)
	if got := configCurrent(readTestConfig(t, root), "genshin"); got != "202" {
		t.Fatalf("genshin current = %q", got)
	}

	if err := saveGameCurrent(root, "starrail", "101"); err != nil {
		t.Fatal(err)
	}
	cfg := readTestConfig(t, root)
	if configCurrent(cfg, "genshin") != "202" || configCurrent(cfg, "starrail") != "101" {
		t.Errorf("写入后配置 %#v", cfg)
	}
	games, _ := cfg["games"].(map[string]any)
	genshin, _ := games["genshin"].(map[string]any)
	if genshin["proxyPort"] != int64(8325) {
		t.Errorf("其他配置被覆盖：%#v", genshin)
	}
}

// 各游戏启动时并发读入，统一配置的读改写必须串行，避免互相覆盖。
func TestSaveGameCurrentConcurrent(t *testing.T) {
	root := t.TempDir()
	ids := []string{"genshin", "starrail", "zzz"}
	var wg sync.WaitGroup
	for i, id := range ids {
		wg.Add(1)
		go func(id string, n int) {
			defer wg.Done()
			if err := saveGameCurrent(root, id, fmt.Sprintf("10%d", n)); err != nil {
				t.Errorf("%s: %v", id, err)
			}
		}(id, i)
	}
	wg.Wait()
	cfg := readTestConfig(t, root)
	for i, id := range ids {
		if got := configCurrent(cfg, id); got != fmt.Sprintf("10%d", i) {
			t.Errorf("%s current = %q", id, got)
		}
	}
}

// 旧版每款游戏的 config.json 会被读入，并在下次保存时迁移到统一配置。
func TestLoadGachaGameMigratesLegacyConfig(t *testing.T) {
	root := t.TempDir()
	dir := testGameDir(t, root, "genshin")
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"current":"202"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "gacha-list-202.json"), []byte(genshinFixture), 0o644); err != nil {
		t.Fatal(err)
	}
	d, err := loadGachaGame(root, "genshin")
	if err != nil {
		t.Fatal(err)
	}
	if d == nil || d.UID != "202" {
		t.Fatalf("读入 %+v", d)
	}
	if got := configCurrent(readTestConfig(t, root), "genshin"); got != "202" {
		t.Errorf("迁移后的 current = %q", got)
	}
}
