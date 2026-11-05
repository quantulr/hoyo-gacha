package main

import (
	"fmt"
	"regexp"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestZZZBannerData(t *testing.T) {
	set, err := loadZZZBanners()
	if err != nil {
		t.Fatal(err)
	}
	if set.Source == "" || set.Timezone != "UTC+8" || set.Updated == "" {
		t.Errorf("元数据 %+v", set)
	}
	if len(set.Banners) != 142 {
		t.Fatalf("卡池 %d 个，期望 142", len(set.Banners))
	}

	versionRE := regexp.MustCompile(`^\d+\.\d+$`)
	versions := make(map[string]bool)
	channels := make(map[string]int)
	var prev time.Time
	for i, b := range set.Banners {
		if !versionRE.MatchString(b.Version) {
			t.Errorf("第 %d 个卡池版本 %q", i+1, b.Version)
		}
		versions[b.Version] = true
		channels[b.Channel]++
		switch b.Phase {
		case "上半", "下半", "全版本":
		default:
			t.Errorf("第 %d 个卡池阶段 %q", i+1, b.Phase)
		}
		if b.Title == "" || len(b.UP) == 0 {
			t.Errorf("第 %d 个卡池缺少标题或 UP：%+v", i+1, b)
		}
		start, err := zzzBannerTime(b.Start)
		if err != nil {
			t.Errorf("第 %d 个卡池开始时间：%v", i+1, err)
			continue
		}
		if start.Before(prev) {
			t.Errorf("第 %d 个卡池未按时间排序：%s 早于 %s", i+1, b.Start, prev.Format(zzzBannerLayout))
		}
		prev = start
	}
	if channels["character"] != 71 || channels["engine"] != 71 {
		t.Errorf("频道统计 %v，期望各 71 个", channels)
	}
	for major, maxMinor := range map[int]int{1: 7, 2: 8, 3: 3} {
		for minor := 0; minor <= maxMinor; minor++ {
			v := fmt.Sprintf("%d.%d", major, minor)
			if !versions[v] {
				t.Errorf("缺少 %s 版本卡池", v)
			}
		}
	}
}

func TestZZZBannerSamples(t *testing.T) {
	set, err := loadZZZBanners()
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		version, channel, up string
		phase, start, end    string
	}{
		{"1.0", "character", "艾莲", "上半", "2024-07-04 10:00:00", "2024-07-24 11:59:59"},
		{"1.0", "engine", "深海访客", "上半", "2024-07-04 10:00:00", "2024-07-24 11:59:59"},
		{"1.4", "character", "雅", "全版本", "2024-12-18 11:00:00", "2025-01-21 14:59:59"},
		{"1.4", "character", "悠真", "全版本", "2024-12-18 11:00:00", "2025-01-21 14:59:59"},
		{"3.3", "character", "菲欧妮", "上半", "2026-10-21 11:00:00", "2026-11-10 11:59:59"},
		{"3.3", "engine", "涅槃初翎", "上半", "2026-10-21 11:00:00", "2026-11-10 11:59:59"},
	}
	for _, tt := range tests {
		b := findZZZBanner(set.Banners, tt.version, tt.channel, tt.up)
		if b == nil {
			t.Errorf("找不到 %s %s %s", tt.version, tt.channel, tt.up)
			continue
		}
		if b.Phase != tt.phase || b.Start != tt.start || b.End != tt.end {
			t.Errorf("%s %s = %+v，期望 %s %s ~ %s", tt.version, tt.up, b, tt.phase, tt.start, tt.end)
		}
	}
}

func TestZZZBannersAt(t *testing.T) {
	set, err := loadZZZBanners()
	if err != nil {
		t.Fatal(err)
	}
	at := func(s string) time.Time {
		t.Helper()
		tm, err := zzzBannerTime(s)
		if err != nil {
			t.Fatal(err)
		}
		return tm
	}
	hasUP := func(bs []zzzBanner, name string) bool {
		for _, b := range bs {
			if slices.Contains(b.UP, name) {
				return true
			}
		}
		return false
	}

	// 1.4 实际提前开服，10:52 的抽卡应归入 11:00 开放的卡池。
	if got := zzzBannersAt(set.Banners, "character", at("2024-12-18 10:52:00")); !hasUP(got, "雅") {
		t.Errorf("提前开服未归入 1.4：%+v", got)
	}
	if got := zzzBannersAt(set.Banners, "engine", at("2024-12-18 10:52:00")); !hasUP(got, "霰落星殿") {
		t.Errorf("音擎频道提前开服未归入 1.4：%+v", got)
	}

	// 下半 12:00 开放，此时不应再算作上半。
	got := zzzBannersAt(set.Banners, "character", at("2024-07-24 12:30:00"))
	if len(got) != 1 || !hasUP(got, "朱鸢") || hasUP(got, "艾莲") {
		t.Errorf("下半卡池 %+v", got)
	}

	// 跨版本空档超过提前开服容差时，没有开放中的卡池。
	if got := zzzBannersAt(set.Banners, "character", at("2024-08-13 20:00:00")); len(got) != 0 {
		t.Errorf("空档期返回了卡池：%+v", got)
	}
}

func TestAnnotateZZZLost(t *testing.T) {
	set, err := loadZZZBanners()
	if err != nil {
		t.Fatal(err)
	}
	d := newZZZData(&gachaExport{
		UID:     "1",
		TypeMap: [][2]string{{"2", "独家频段"}, {"3", "音擎频段"}, {"1", "常驻频段"}},
		Result: []gachaGroup{
			{Type: "2", Records: []gachaRecord{
				{ID: "1", Name: "艾莲", RankType: "4", Time: "2024-07-10 12:00:00"},
				{ID: "2", Name: "猫又", RankType: "4", Time: "2024-07-11 12:00:00"},
				{ID: "3", Name: "朱鸢", RankType: "4", Time: "2024-07-25 12:00:00"},
				{ID: "4", Name: "艾莲", RankType: "4", Time: "2024-07-26 12:00:00"},
				{ID: "5", Name: "雅", RankType: "4", Time: "2024-12-18 10:52:00"},
				{ID: "6", Name: "「猫又」", RankType: "4", Time: "2024-12-18 10:52:00"},
			}},
			{Type: "3", Records: []gachaRecord{
				{ID: "7", Name: "深海访客", RankType: "4", Time: "2024-07-10 12:00:00"},
				{ID: "8", Name: "燃狱齿轮", RankType: "4", Time: "2024-07-10 12:00:00"},
			}},
			{Type: "1", Records: []gachaRecord{
				{ID: "9", Name: "猫又", RankType: "4", Time: "2024-07-10 12:00:00"},
			}},
		},
	})
	annotateZZZLost(d, set.Banners)

	got := make(map[string]bool)
	for _, ch := range d.Channels {
		for _, r := range ch.Records {
			got[r.ID] = r.Lost
		}
	}
	want := map[string]bool{
		"1": false, // 艾莲是 1.0 上半 UP
		"2": true,  // 猫又不是 UP
		"3": false, // 朱鸢是 1.0 下半 UP
		"4": true,  // 艾莲在下半已不是 UP
		"5": false, // 1.4 提前开服，雅仍是 UP
		"6": true,  // 提前开服也按 1.4 卡池判定
		"7": false, // 深海访客是音擎 UP
		"8": true,  // 燃狱齿轮是常驻音擎
		"9": false, // 常驻频段不判定歪
	}
	for id, wantLost := range want {
		gotLost, ok := got[id]
		if !ok {
			t.Errorf("缺少记录 %s", id)
			continue
		}
		if gotLost != wantLost {
			t.Errorf("记录 %s 歪 = %v，期望 %v", id, gotLost, wantLost)
		}
	}
}

func TestZZZLostBadge(t *testing.T) {
	set, err := loadZZZBanners()
	if err != nil {
		t.Fatal(err)
	}
	d := newZZZData(&gachaExport{
		UID:     "1",
		TypeMap: [][2]string{{"2", "独家频段"}},
		Result: []gachaGroup{{Type: "2", Records: []gachaRecord{
			{ID: "1", Name: "艾莲", RankType: "4", Time: "2024-07-10 12:00:00"},
			{ID: "2", Name: "猫又", RankType: "4", Time: "2024-07-11 12:00:00"},
		}}},
	})
	annotateZZZLost(d, set.Banners)

	a := newApp()
	a.selected = "zzz"
	a.zzz = d
	a.zzzLoaded = true
	tt := ui.NewTester(a.view, 960, 680)
	if err := tt.Click("记录"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("歪") {
		t.Errorf("记录页缺少歪标识，现有 %q", tt.Texts())
	}
	if !tt.HasText("猫又") || !tt.HasText("艾莲") {
		t.Errorf("记录页缺少记录，现有 %q", tt.Texts())
	}
}

func findZZZBanner(banners []zzzBanner, version, channel, up string) *zzzBanner {
	for i, b := range banners {
		if b.Version != version || b.Channel != channel {
			continue
		}
		if slices.Contains(b.UP, up) {
			return &banners[i]
		}
	}
	return nil
}
