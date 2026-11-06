package main

import (
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

func TestGenshinView(t *testing.T) {
	a := newApp()
	a.selected = "genshin"
	a.genshin = testGenshinData(t)
	a.genshinLoaded = true
	tt := ui.NewTester(a.view, 960, 680)
	for _, text := range []string{
		"原神", "抽卡概览", "全部祈愿", "祈愿保底",
		"角色活动祈愿", "武器活动祈愿", "常驻祈愿", "集录祈愿", "新手祈愿",
		"最近出金", "保底 0 / 90", "保底 1 / 20",
	} {
		if !tt.HasText(text) {
			t.Errorf("缺少文本 %q，现有 %q", text, tt.Texts())
		}
	}

	if err := tt.Click("记录"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("S1") || tt.HasText("A1") || tt.HasText("B1") ||
		!tt.HasText("共 1 条 · 最新在前") {
		t.Errorf("记录页默认应只看 S，现有 %q", tt.Texts())
	}

	if err := tt.Click("全部"); err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"共 3 条 · 最新在前", "S1", "A1", "B1"} {
		if !tt.HasText(text) {
			t.Errorf("全部筛选缺少 %q，现有 %q", text, tt.Texts())
		}
	}

	if err := tt.Click("S+A"); err != nil {
		t.Fatal(err)
	}
	if tt.HasText("B1") || !tt.HasText("A1") || !tt.HasText("共 2 条 · 最新在前") {
		t.Errorf("S+A 筛选结果 %q", tt.Texts())
	}
}
