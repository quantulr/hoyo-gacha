package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

// 原神导出使用紧凑数组记录：[时间, 名称, 类型, 等级, 卡池类型, id]。
const genshinFixture = `{
  "uid": "202",
  "lang": "zh-cn",
  "typeMap": [["301", "角色活动祈愿"], ["302", "武器活动祈愿"], ["200", "常驻祈愿"], ["500", "集录祈愿"], ["100", "新手祈愿"]],
  "result": [
    ["301", [
      ["2024-01-01 10:00:00", "B1", "武器", 3],
      ["2024-01-02 10:00:00", "A1", "角色", 4, "301", "2"],
      ["2024-01-03 10:00:00", "S1", "角色", 5, "400", "3"]
    ]],
    ["302", [["2024-01-04 10:00:00", "W1", "武器", 5, "302", "4"]]],
    ["200", [["2024-01-05 10:00:00", "STD1", "角色", 5, "200", "5"]]],
    ["500", [["2024-01-06 10:00:00", "CHR1", "角色", 5, "500", "6"]]],
    ["100", [["2024-01-07 10:00:00", "NOV1", "角色", 4, "100", "7"]]]
  ]
}`

func testGenshinData(t *testing.T) *gachaData {
	t.Helper()
	d, err := parseGacha([]byte(genshinFixture), "genshin")
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestGenshinStats(t *testing.T) {
	d := testGenshinData(t)
	if d.UID != "202" || d.TotalPulls != 7 || d.TotalS != 4 || d.TotalA != 2 {
		t.Fatalf("汇总 %+v", d)
	}
	if d.SRate() != float64(4)/7 || d.Average() != 1.75 {
		t.Errorf("概率 %.2f，平均 %.2f", d.SRate(), d.Average())
	}

	want := []struct {
		typ, name      string
		pulls, s, hard int
	}{
		{"301", "角色活动祈愿", 3, 1, 90},
		{"302", "武器活动祈愿", 1, 1, 80},
		{"200", "常驻祈愿", 1, 1, 90},
		{"500", "集录祈愿", 1, 1, 90},
		{"100", "新手祈愿", 1, 0, 20},
	}
	if len(d.Channels) != len(want) {
		t.Fatalf("祈愿 %d 个，期望 %d 个：%+v", len(d.Channels), len(want), d.Channels)
	}
	for i, w := range want {
		ch := d.Channels[i]
		if ch.Type != w.typ || ch.Name != w.name || ch.Pulls != w.pulls ||
			ch.S != w.s || ch.HardPity != w.hard {
			t.Errorf("第 %d 个祈愿 %+v，期望 %+v", i+1, ch, w)
		}
	}
	if d.Channels[4].Pity != 1 {
		t.Errorf("新手祈愿保底 %d，期望 1", d.Channels[4].Pity)
	}

	// 5 = S、4 = A、3 = B；301 组里 gacha_type 为 400 的记录也属于角色活动祈愿。
	recs := d.Channels[0].Records
	if len(recs) != 3 || recs[0].Name != "S1" || recs[0].Tier != 0 ||
		recs[1].Name != "A1" || recs[1].Tier != 1 ||
		recs[2].Name != "B1" || recs[2].Tier != 2 {
		t.Errorf("角色活动祈愿记录 %+v", recs)
	}
	if recs[0].GachaType != "400" || recs[0].ID != "3" {
		t.Errorf("数组记录解析 %+v", recs[0])
	}
	if recs[2].GachaType != "" || recs[2].ID != "" {
		t.Errorf("四字段记录 %+v", recs[2])
	}
}

func TestLoadGenshinDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"current":"202"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "gacha-list-202.json"), []byte(genshinFixture), 0o644); err != nil {
		t.Fatal(err)
	}
	d, err := loadGachaDir(dir, "genshin")
	if err != nil {
		t.Fatal(err)
	}
	if d.UID != "202" || d.TotalPulls != 7 || len(d.Channels) != 5 {
		t.Errorf("读入 %+v", d)
	}
}

// 没有导出文件时返回空数据，界面应提示导入或获取，而不是读取失败。
func TestLoadGenshinDirMissing(t *testing.T) {
	d, err := loadGachaDir(t.TempDir(), "genshin")
	if err != nil || d != nil {
		t.Fatalf("空目录返回 %+v, %v", d, err)
	}
}

// 手动导入 gacha-list-*.json 后，把文件里的 UID 写成当前账号。
func TestLoadGenshinDirImportsCurrent(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"proxyPort":8325}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "gacha-list-202.json"), []byte(genshinFixture), 0o644); err != nil {
		t.Fatal(err)
	}
	d, err := loadGachaDir(dir, "genshin")
	if err != nil {
		t.Fatal(err)
	}
	if d.UID != "202" {
		t.Fatalf("读入 UID %q", d.UID)
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
	if cfg["proxyPort"] != float64(8325) {
		t.Errorf("config 其他字段被覆盖：%v", cfg)
	}
}

func TestGenshinEmptyState(t *testing.T) {
	a := newApp()
	a.selected = "genshin"
	a.genshinLoaded = true
	tt := ui.NewTester(a.view, 960, 680)
	for _, text := range []string{"还没有抽卡记录", "导入 gacha-list-*.json", "导入记录", "刷新"} {
		if !tt.HasText(text) {
			t.Errorf("缺少文本 %q，现有 %q", text, tt.Texts())
		}
	}
	if _, ok := tt.Find("导入"); !ok {
		t.Errorf("标题栏缺少导入按钮，文本 %q", tt.Texts())
	}
	if tt.HasText("无法读取") {
		t.Errorf("没有文件时不应显示读取失败，现有 %q", tt.Texts())
	}
}

func TestGenshinView(t *testing.T) {
	a := newApp()
	a.selected = "genshin"
	a.genshin = testGenshinData(t)
	a.genshinLoaded = true
	tt := ui.NewTester(a.view, 960, 680)
	for _, text := range []string{
		"原神", "抽卡概览", "全部祈愿", "祈愿保底",
		"角色活动祈愿", "武器活动祈愿", "常驻祈愿", "集录祈愿", "新手祈愿",
		"最近出金", "保底 0 / 90", "保底 1 / 20", "5 星", "4 星",
	} {
		if !tt.HasText(text) {
			t.Errorf("缺少文本 %q，现有 %q", text, tt.Texts())
		}
	}

	if err := tt.Click("记录"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("仅 5 星") || !tt.HasText("5+4 星") {
		t.Errorf("缺少星级筛选，现有 %q", tt.Texts())
	}
	if !tt.HasText("S1") || tt.HasText("A1") || tt.HasText("B1") ||
		!tt.HasText("共 1 条 · 最新在前") {
		t.Errorf("记录页默认应只看最高档，现有 %q", tt.Texts())
	}

	if err := tt.Click("全部"); err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"共 3 条 · 最新在前", "S1", "A1", "B1"} {
		if !tt.HasText(text) {
			t.Errorf("全部筛选缺少 %q，现有 %q", text, tt.Texts())
		}
	}

	if err := tt.Click("5+4 星"); err != nil {
		t.Fatal(err)
	}
	if tt.HasText("B1") || !tt.HasText("A1") || !tt.HasText("共 2 条 · 最新在前") {
		t.Errorf("5+4 星筛选结果 %q", tt.Texts())
	}
}

// 千星奇域颂愿的等级刻度与普通祈愿不同：活动颂愿是 5/4/3，
// 常驻颂愿没有 5 星，最高 4 星。
const genshinUGCFixture = `{
  "uid": "202",
  "lang": "zh-cn",
  "typeMap": [["2000", "千星奇域·活动颂愿"], ["1000", "千星奇域·常驻颂愿"]],
  "result": [
    ["2000", [
      {"id":"11","item_id":"11","item_type":"装扮","name":"活动B","rank_type":"3","time":"2024-01-01 10:00:00","gacha_type":"20011"},
      {"id":"12","item_id":"12","item_type":"装扮","name":"活动A","rank_type":"4","time":"2024-01-02 10:00:00","gacha_type":"20021"},
      {"id":"13","item_id":"13","item_type":"装扮","name":"活动S","rank_type":"5","time":"2024-01-03 10:00:00","gacha_type":"20021"}
    ]],
    ["1000", [
      {"id":"21","item_id":"21","item_type":"装扮","name":"常驻B","rank_type":"2","time":"2024-01-04 10:00:00","gacha_type":"1000"},
      {"id":"22","item_id":"22","item_type":"装扮","name":"常驻A","rank_type":"3","time":"2024-01-05 10:00:00","gacha_type":"1000"},
      {"id":"23","item_id":"23","item_type":"装扮","name":"常驻S","rank_type":"4","time":"2024-01-06 10:00:00","gacha_type":"1000"}
    ]]
  ]
}`

func testGenshinUGCData(t *testing.T) *gachaData {
	t.Helper()
	d, err := parseGacha([]byte(genshinUGCFixture), "genshin")
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestGenshinUGCStats(t *testing.T) {
	d := testGenshinUGCData(t)
	if d.UID != "202" || d.TotalPulls != 6 || d.TotalS != 2 || d.TotalA != 2 {
		t.Fatalf("汇总 %+v", d)
	}
	if len(d.Channels) != 2 {
		t.Fatalf("颂愿 %+v", d.Channels)
	}
	want := []struct {
		typ, name string
		s, a, b   int
		hard      int
	}{
		{"2000", "千星奇域·活动颂愿", 1, 1, 1, 70},
		{"1000", "千星奇域·常驻颂愿", 1, 1, 1, 70},
	}
	for i, w := range want {
		ch := d.Channels[i]
		if ch.Type != w.typ || ch.Name != w.name || ch.S != w.s || ch.A != w.a ||
			ch.B != w.b || ch.HardPity != w.hard || ch.Pity != 0 || ch.Average != 3 {
			t.Errorf("第 %d 个颂愿 %+v，期望 %+v", i+1, ch, w)
		}
	}
	// 常驻颂愿的 4/3/2 星分别映射为 S/A/B；按通用 5/4/3 规则会算错。
	recs := d.Channels[1].Records
	if len(recs) != 3 || recs[0].Name != "常驻S" || recs[0].Tier != 0 ||
		recs[1].Name != "常驻A" || recs[1].Tier != 1 ||
		recs[2].Name != "常驻B" || recs[2].Tier != 2 {
		t.Errorf("常驻颂愿记录 %+v", recs)
	}
	if len(d.SRecords) != 2 || d.SRecords[0].Name != "常驻S" || d.SRecords[1].Name != "活动S" {
		t.Errorf("颂愿出金 %+v", d.SRecords)
	}
}

func TestGenshinUGCView(t *testing.T) {
	a := newApp()
	a.selected = "genshin"
	a.genshin = testGenshinUGCData(t)
	a.genshinLoaded = true
	tt := ui.NewTester(a.view, 960, 680)
	for _, text := range []string{
		"千星奇域·活动颂愿", "千星奇域·常驻颂愿", "保底 0 / 70", "活动S", "常驻S",
	} {
		if !tt.HasText(text) {
			t.Errorf("缺少文本 %q，现有 %q", text, tt.Texts())
		}
	}

	if err := tt.Click("记录"); err != nil {
		t.Fatal(err)
	}
	// 记录页按频段分开显示，默认只列当前频段的 S 级。
	if !tt.HasText("活动S") || tt.HasText("活动A") || tt.HasText("常驻S") ||
		!tt.HasText("共 1 条 · 最新在前") {
		t.Errorf("记录页默认应只看 S，现有 %q", tt.Texts())
	}
	if err := tt.Click("全部"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("活动A") || !tt.HasText("活动B") || !tt.HasText("共 3 条 · 最新在前") {
		t.Errorf("全部筛选结果 %q", tt.Texts())
	}
	if err := tt.Click("千星奇域·常驻颂愿"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("常驻S") || !tt.HasText("常驻A") || !tt.HasText("常驻B") ||
		!tt.HasText("共 3 条 · 最新在前") {
		t.Errorf("常驻颂愿记录 %q", tt.Texts())
	}
	if !tt.HasText("仅 4 星") || !tt.HasText("4+3 星") {
		t.Errorf("常驻颂愿缺少星级筛选，现有 %q", tt.Texts())
	}
	if err := tt.Click("仅 4 星"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("常驻S") || tt.HasText("常驻A") || tt.HasText("常驻B") ||
		!tt.HasText("共 1 条 · 最新在前") {
		t.Errorf("常驻颂愿仅 4 星筛选结果 %q", tt.Texts())
	}
}
