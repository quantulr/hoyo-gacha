package main

import (
	"encoding/json"
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
	dir := t.TempDir()
	added, err := importGachaFile(dir, &genshinGame, src)
	if err != nil {
		t.Fatal(err)
	}
	if added != 7 {
		t.Errorf("导入 %d 条，期望 7 条", added)
	}
	d, err := loadGachaDir(dir, "genshin")
	if err != nil {
		t.Fatal(err)
	}
	if d.UID != "202" || d.TotalPulls != 7 {
		t.Errorf("读入 %+v", d)
	}
	data, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg["current"] != "202" {
		t.Errorf("config current = %v", cfg["current"])
	}
}

func TestImportGachaFileUsesFilenameUID(t *testing.T) {
	fixture := `{
  "typeMap": [["301", "角色活动祈愿"]],
  "result": [["301", [["2024-01-01 10:00:00", "S1", "角色", 5]]]]
}`
	src := writeImportSource(t, "gacha-list-202.json", fixture)
	dir := t.TempDir()
	if _, err := importGachaFile(dir, &genshinGame, src); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "gacha-list-202.json")); err != nil {
		t.Errorf("没有写入 gacha-list-202.json：%v", err)
	}
}

func TestImportGachaFileRejectsOtherGame(t *testing.T) {
	src := writeImportSource(t, "zzz-export.json", zzzFixture)
	dir := t.TempDir()
	if _, err := importGachaFile(dir, &genshinGame, src); err == nil {
		t.Fatal("把绝区零文件导入原神时应报错")
	}
	if matches, _ := filepath.Glob(filepath.Join(dir, "gacha-list-*.json")); len(matches) != 0 {
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
	dir := t.TempDir()
	if _, err := importGachaFile(dir, &genshinGame, src); err == nil {
		t.Fatal("非法 UID 应报错")
	}
	if _, err := os.Stat(filepath.Join(dir, "gacha-list-../../evil.json")); !os.IsNotExist(err) {
		t.Errorf("非法 UID 产生了文件：%v", err)
	}
}
