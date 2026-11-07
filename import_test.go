package main

import (
	"os"
	"path/filepath"
	"testing"
)

func writeImportSource(t *testing.T, name, data string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestImportGachaFile(t *testing.T) {
	src := writeImportSource(t, "genshin-export.json", genshinFixture)
	root := t.TempDir()
	added, err := importGachaFile(root, &genshinGame, src)
	if err != nil {
		t.Fatal(err)
	}
	if added != 7 {
		t.Errorf("导入 %d 条，期望 7 条", added)
	}
	d, err := loadGachaGame(root, "genshin")
	if err != nil {
		t.Fatal(err)
	}
	if d.UID != "202" || d.TotalPulls != 7 {
		t.Errorf("读入 %+v", d)
	}
	if got := configCurrent(readTestConfig(t, root), "genshin"); got != "202" {
		t.Errorf("config current = %q", got)
	}
}

func TestImportGachaFileUsesFilenameUID(t *testing.T) {
	fixture := `{
  "typeMap": [["301", "角色活动祈愿"]],
  "result": [["301", [["2024-01-01 10:00:00", "S1", "角色", 5]]]]
}`
	src := writeImportSource(t, "gacha-list-202.json", fixture)
	root := t.TempDir()
	if _, err := importGachaFile(root, &genshinGame, src); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(gameDataDir(root, "genshin"), "gacha-list-202.json")); err != nil {
		t.Errorf("没有写入 gacha-list-202.json：%v", err)
	}
}

func TestImportGachaFileRejectsOtherGame(t *testing.T) {
	src := writeImportSource(t, "zzz-export.json", zzzFixture)
	root := t.TempDir()
	if _, err := importGachaFile(root, &genshinGame, src); err == nil {
		t.Fatal("把绝区零文件导入原神时应报错")
	}
	if matches, _ := filepath.Glob(filepath.Join(gameDataDir(root, "genshin"), "gacha-list-*.json")); len(matches) != 0 {
		t.Errorf("失败时写入了文件：%v", matches)
	}
}

func TestImportGachaFileRejectsInvalidUID(t *testing.T) {
	fixture := `{
  "uid": "../../evil",
  "typeMap": [["301", "角色活动祈愿"]],
  "result": [["301", [["2024-01-01 10:00:00", "S1", "角色", 5]]]]
}`
	src := writeImportSource(t, "export.json", fixture)
	root := t.TempDir()
	if _, err := importGachaFile(root, &genshinGame, src); err == nil {
		t.Fatal("非法 UID 应报错")
	}
	if _, err := os.Stat(filepath.Join(gameDataDir(root, "genshin"), "gacha-list-../../evil.json")); !os.IsNotExist(err) {
		t.Errorf("非法 UID 产生了文件：%v", err)
	}
}
