package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

const starRailFixture = `{
  "uid": "101",
  "lang": "zh-cn",
  "typeMap": [["11", "角色活动跃迁"], ["12", "光锥活动跃迁"], ["1", "常驻跃迁"], ["2", "新手跃迁"], ["22", "光锥联动跃迁"]],
  "result": [
    ["11", [
      {"id": "1", "item_id": "1", "item_type": "光锥", "name": "B1", "rank_type": "3", "time": "2024-01-01 10:00:00", "gacha_type": "11"},
      {"id": "2", "item_id": "2", "item_type": "角色", "name": "A1", "rank_type": "4", "time": "2024-01-02 10:00:00", "gacha_type": "11"},
      {"id": "3", "item_id": "3", "item_type": "角色", "name": "S1", "rank_type": "5", "time": "2024-01-03 10:00:00", "gacha_type": "11"}
    ]],
    ["12", [
      {"id": "4", "item_id": "4", "item_type": "光锥", "name": "LC1", "rank_type": "5", "time": "2024-01-04 10:00:00", "gacha_type": "12"}
    ]],
    ["1", [
      {"id": "5", "item_id": "5", "item_type": "角色", "name": "STD1", "rank_type": "5", "time": "2024-01-05 10:00:00", "gacha_type": "1"}
    ]],
    ["2", [
      {"id": "6", "item_id": "6", "item_type": "角色", "name": "NOV1", "rank_type": "5", "time": "2024-01-06 10:00:00", "gacha_type": "2"}
    ]],
    ["22", []]
  ]
}`

func testStarRailData(t *testing.T) *gachaData {
	t.Helper()
	d, err := parseStarRail([]byte(starRailFixture))
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestStarRailStats(t *testing.T) {
	d := testStarRailData(t)
	if d.UID != "101" || d.TotalPulls != 6 || d.TotalS != 4 || d.TotalA != 1 {
		t.Fatalf("汇总 %+v", d)
	}
	if d.SRate() != float64(4)/6 || d.Average() != 1.5 {
		t.Errorf("概率 %.2f，平均 %.2f", d.SRate(), d.Average())
	}

	want := []struct {
		typ, name      string
		pulls, s, hard int
	}{
		{"11", "角色活动跃迁", 3, 1, 90},
		{"12", "光锥活动跃迁", 1, 1, 80},
		{"1", "常驻跃迁", 1, 1, 90},
		{"2", "新手跃迁", 1, 1, 50},
	}
	if len(d.Channels) != len(want) {
		t.Fatalf("跃迁 %d 个，期望 %d 个：%+v", len(d.Channels), len(want), d.Channels)
	}
	for i, w := range want {
		ch := d.Channels[i]
		if ch.Type != w.typ || ch.Name != w.name || ch.Pulls != w.pulls ||
			ch.S != w.s || ch.HardPity != w.hard {
			t.Errorf("第 %d 个跃迁 %+v，期望 %+v", i+1, ch, w)
		}
	}

	// 等级映射：5 = S、4 = A、3 = B。
	recs := d.Channels[0].Records
	if len(recs) != 3 || recs[0].Name != "S1" || recs[0].Tier != 0 ||
		recs[1].Name != "A1" || recs[1].Tier != 1 ||
		recs[2].Name != "B1" || recs[2].Tier != 2 {
		t.Errorf("角色活动跃迁记录 %+v", recs)
	}
	if recs[0].Pity != 3 || d.Channels[0].Pity != 0 {
		t.Errorf("保底计数 %+v", recs)
	}
}

func TestLoadStarRailDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"current":"101"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "gacha-list-101.json"), []byte(starRailFixture), 0o644); err != nil {
		t.Fatal(err)
	}
	d, err := loadGachaDir(dir, "starrail")
	if err != nil {
		t.Fatal(err)
	}
	if d.UID != "101" || d.TotalPulls != 6 || len(d.Channels) != 4 {
		t.Errorf("读入 %+v", d)
	}
}

func TestStarRailView(t *testing.T) {
	a := newApp()
	a.selected = "starrail"
	a.starrail = testStarRailData(t)
	a.starrailLoaded = true
	tt := ui.NewTester(a.view, 960, 680)
	for _, text := range []string{
		"崩坏：星穹铁道", "抽卡概览", "全部跃迁", "跃迁保底",
		"角色活动跃迁", "光锥活动跃迁", "新手跃迁", "最近出金", "保底 0 / 90",
		"5 星", "4 星",
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
		t.Errorf("记录页默认应只看 5 星，现有 %q", tt.Texts())
	}

	if err := tt.Click("全部"); err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"共 3 条 · 最新在前", "S1", "A1", "B1"} {
		if !tt.HasText(text) {
			t.Errorf("全部筛选缺少 %q，现有 %q", text, tt.Texts())
		}
	}
	for _, badge := range []string{"5星", "4星", "3星"} {
		if _, ok := tt.Find(badge); !ok {
			t.Errorf("缺少等级徽标 %q，现有 %q", badge, tt.Texts())
		}
	}

	if err := tt.Click("5+4 星"); err != nil {
		t.Fatal(err)
	}
	if tt.HasText("B1") || !tt.HasText("A1") || !tt.HasText("共 2 条 · 最新在前") {
		t.Errorf("5+4 星筛选结果 %q", tt.Texts())
	}
}
