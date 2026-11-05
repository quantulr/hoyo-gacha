package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

const zzzFixture = `{
  "uid": "100",
  "lang": "zh-cn",
  "typeMap": [["2", "独家频段"], ["1", "常驻频段"]],
  "result": [
    ["2", [
      {"id":"1","item_id":"1","item_type":"音擎","name":"B1","rank_type":"2","time":"2024-01-01 10:00:00","gacha_type":"2"},
      {"id":"2","item_id":"2","item_type":"代理人","name":"A1","rank_type":"3","time":"2024-01-02 10:00:00","gacha_type":"2"},
      {"id":"3","item_id":"3","item_type":"代理人","name":"S1","rank_type":"4","time":"2024-01-03 10:00:00","gacha_type":"2"},
      {"id":"4","item_id":"4","item_type":"音擎","name":"B2","rank_type":"2","time":"2024-01-04 10:00:00","gacha_type":"2"}
    ]],
    ["1", [
      {"id":"5","item_id":"5","item_type":"代理人","name":"S2","rank_type":"4","time":"2024-01-05 10:00:00","gacha_type":"1"}
    ]]
  ]
}`

func testZZZData(t *testing.T) *zzzData {
	t.Helper()
	d, err := parseZZZ([]byte(zzzFixture))
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestZZZStats(t *testing.T) {
	d := testZZZData(t)
	if d.UID != "100" || d.TotalPulls != 5 || d.TotalS != 2 || d.TotalA != 1 {
		t.Fatalf("汇总 %+v", d)
	}
	if d.LastTime != "2024-01-05 10:00:00" {
		t.Errorf("最后记录 %q", d.LastTime)
	}
	if len(d.Channels) != 2 {
		t.Fatalf("频段 %+v", d.Channels)
	}
	limited := d.Channels[0]
	if limited.Type != "2" || limited.Pulls != 4 || limited.S != 1 || limited.A != 1 ||
		limited.Pity != 1 || limited.HardPity != 90 || limited.Average != 4 {
		t.Errorf("独家频段 %+v", limited)
	}
	stable := d.Channels[1]
	if stable.Type != "1" || stable.Pulls != 1 || stable.S != 1 || stable.Pity != 0 {
		t.Errorf("常驻频段 %+v", stable)
	}
	if len(limited.Records) != 4 {
		t.Fatalf("记录 %+v", limited.Records)
	}
	wantPity := []int{1, 3, 2, 1} // 最新在前：B2、S1、A1、B1
	for i, want := range wantPity {
		if limited.Records[i].Pity != want {
			t.Errorf("第 %d 条保底 %d，期望 %d", i, limited.Records[i].Pity, want)
		}
	}
	if limited.Records[0].Name != "B2" || limited.Records[1].Name != "S1" {
		t.Errorf("记录顺序 %+v", limited.Records)
	}
	if len(d.SRecords) != 2 {
		t.Fatalf("S 记录 %+v", d.SRecords)
	}
	if d.SRecords[0].Name != "S2" || d.SRecords[0].Pity != 1 ||
		d.SRecords[1].Name != "S1" || d.SRecords[1].Pity != 3 {
		t.Errorf("S 记录顺序 %+v", d.SRecords)
	}
	if d.SRate() != 0.4 || d.Average() != 2.5 {
		t.Errorf("概率 %.2f，平均 %.2f", d.SRate(), d.Average())
	}
}

func TestLoadZZZDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"current":"100"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "gacha-list-100.json"), []byte(zzzFixture), 0o644); err != nil {
		t.Fatal(err)
	}
	d, err := loadZZZDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if d.UID != "100" || d.TotalPulls != 5 {
		t.Errorf("读入 %+v", d)
	}
	if _, err := loadZZZDir(t.TempDir()); err == nil {
		t.Error("空目录没有报错")
	}
}

func TestZZZView(t *testing.T) {
	a := newApp()
	a.selected = "zzz"
	a.zzz = testZZZData(t)
	a.zzzLoaded = true
	tt := ui.NewTester(a.view, 960, 680)
	for _, text := range []string{"抽卡概览", "频段保底", "最近出金", "独家频段", "保底 1 / 90", "第 3 抽 · 2024-01-03", "刷新"} {
		if !tt.HasText(text) {
			t.Errorf("缺少文本 %q，现有 %q", text, tt.Texts())
		}
	}

	// 切到记录页，显示每个卡池的完整记录。
	if err := tt.Click("记录"); err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"卡池记录", "共 4 条 · 最新在前", "B2", "S1", "A1"} {
		if !tt.HasText(text) {
			t.Errorf("记录页缺少 %q，现有 %q", text, tt.Texts())
		}
	}
	if err := tt.Click("常驻频段"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("S2") || !tt.HasText("共 1 条 · 最新在前") {
		t.Errorf("切换卡池后文本 %q", tt.Texts())
	}

	// 最小窗口下也能构建。
	tt.SetSize(720, 480)
	if !tt.HasText("卡池记录") || !tt.HasText("S2") {
		t.Errorf("最小尺寸缺少内容，文本 %q", tt.Texts())
	}
}

func TestZZZToast(t *testing.T) {
	a := newApp()
	a.selected = "zzz"
	a.zzz = testZZZData(t)
	a.zzzLoaded = true
	a.zzzToast = "刷新失败：没有找到调频记录链接"
	tt := ui.NewTester(a.view, 960, 680)
	if !tt.HasText("刷新失败：没有找到调频记录链接") {
		t.Errorf("缺少 toast，现有 %q", tt.Texts())
	}
	if a.zzzToast != "" {
		t.Error("toast 应只显示一次")
	}
}

func TestGachaGroupMarshal(t *testing.T) {
	empty, err := json.Marshal(gachaGroup{Type: "1"})
	if err != nil {
		t.Fatal(err)
	}
	if string(empty) != `["1",[]]` {
		t.Errorf("空频段 = %s", empty)
	}
	full, err := json.Marshal(gachaGroup{Type: "2", Records: []gachaRecord{{ID: "1"}}})
	if err != nil {
		t.Fatal(err)
	}
	if string(full) != `["2",[{"id":"1","item_id":"","item_type":"","name":"","rank_type":"","time":"","gacha_type":""}]]` {
		t.Errorf("有记录时 = %s", full)
	}
}

func TestFilterPullRanks(t *testing.T) {
	d := testZZZData(t)
	recs := d.Channels[0].Records // 最新在前：B2、S1、A1、B1
	if got := filterPullRanks(recs, zzzFilterAll); len(got) != 4 {
		t.Errorf("全部筛选 %d 条，期望 4 条", len(got))
	}
	sa := filterPullRanks(recs, zzzFilterSA)
	if len(sa) != 2 || sa[0].Name != "S1" || sa[1].Name != "A1" {
		t.Errorf("S+A 筛选 %+v", sa)
	}
	s := filterPullRanks(recs, zzzFilterS)
	if len(s) != 1 || s[0].Name != "S1" {
		t.Errorf("仅 S 筛选 %+v", s)
	}
}

func TestZZZRankFilter(t *testing.T) {
	a := newApp()
	a.selected = "zzz"
	a.zzz = testZZZData(t)
	a.zzzLoaded = true
	tt := ui.NewTester(a.view, 960, 680)
	if err := tt.Click("记录"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("B2") || !tt.HasText("A1") || !tt.HasText("S1") {
		t.Fatalf("默认应显示全部记录，现有 %q", tt.Texts())
	}

	if err := tt.Click("S+A"); err != nil {
		t.Fatal(err)
	}
	if tt.HasText("B2") || tt.HasText("B1") {
		t.Errorf("S+A 不应显示 B 级，现有 %q", tt.Texts())
	}
	if !tt.HasText("A1") || !tt.HasText("S1") || !tt.HasText("共 2 条 · 最新在前") {
		t.Errorf("S+A 筛选结果 %q", tt.Texts())
	}

	if err := tt.Click("仅 S"); err != nil {
		t.Fatal(err)
	}
	if tt.HasText("A1") || !tt.HasText("S1") || !tt.HasText("共 1 条 · 最新在前") {
		t.Errorf("仅 S 筛选结果 %q", tt.Texts())
	}

	if err := tt.Click("全部"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("B2") || !tt.HasText("共 4 条 · 最新在前") {
		t.Errorf("恢复全部后 %q", tt.Texts())
	}
}

func TestFormatInt(t *testing.T) {
	for n, want := range map[int]string{0: "0", 79: "79", 711: "711", 4698: "4,698", 1234567: "1,234,567"} {
		if got := formatInt(n); got != want {
			t.Errorf("formatInt(%d) = %q，期望 %q", n, got, want)
		}
	}
}
