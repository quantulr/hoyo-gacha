package main

import (
	"fmt"
	"regexp"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestStarRailBannerData(t *testing.T) {
	set, err := loadStarRailBanners()
	if err != nil {
		t.Fatal(err)
	}
	if set.Source == "" || set.Timezone != "UTC+8" || set.Updated == "" {
		t.Errorf("元数据 %+v", set)
	}
	if len(set.Banners) != 236 {
		t.Fatalf("卡池 %d 个，期望 236", len(set.Banners))
	}

	versionRE := regexp.MustCompile(`^\d+\.\d+$`)
	versions := make(map[string]bool)
	phaseChannels := make(map[string]map[string]int)
	channels := make(map[string]int)
	permanent := 0
	var prev time.Time
	for i, b := range set.Banners {
		if !versionRE.MatchString(b.Version) {
			t.Errorf("第 %d 个卡池版本 %q", i+1, b.Version)
		}
		versions[b.Version] = true
		channels[b.Channel]++
		switch b.Phase {
		case "上半", "下半", "额外":
		default:
			t.Errorf("第 %d 个卡池阶段 %q", i+1, b.Phase)
		}
		if b.Title == "" || len(b.UP) == 0 {
			t.Errorf("第 %d 个卡池缺少标题或 UP：%+v", i+1, b)
		}
		start, err := starRailBannerTime(b.Start)
		if err != nil {
			t.Errorf("第 %d 个卡池开始时间：%v", i+1, err)
			continue
		}
		end, err := starRailBannerTime(b.End)
		if err != nil {
			t.Errorf("第 %d 个卡池结束时间：%v", i+1, err)
			continue
		}
		if !end.After(start) {
			t.Errorf("第 %d 个卡池结束时间不晚于开始时间", i+1)
		}
		if start.Before(prev) {
			t.Errorf("第 %d 个卡池未按时间排序：%s 早于 %s", i+1, b.Start, prev.Format(starRailBannerLayout))
		}
		prev = start
		if b.End == "9999-12-31 23:59:59" {
			permanent++
		}
		key := b.Version + b.Phase
		if phaseChannels[key] == nil {
			phaseChannels[key] = make(map[string]int)
		}
		phaseChannels[key][b.Channel]++
	}
	if channels["character"] != 118 || channels["lightcone"] != 118 {
		t.Errorf("频道统计 %v，期望各 118 个", channels)
	}
	if permanent != 8 {
		t.Errorf("长期联动跃迁 %d 个，期望 8", permanent)
	}
	for major, maxMinor := range map[int]int{1: 6, 2: 7, 3: 8, 4: 7} {
		for minor := 0; minor <= maxMinor; minor++ {
			v := fmt.Sprintf("%d.%d", major, minor)
			if !versions[v] {
				t.Errorf("缺少 %s 版本卡池", v)
				continue
			}
			for _, phase := range []string{"上半", "下半"} {
				c := phaseChannels[v+phase]
				if c["character"] == 0 || c["lightcone"] == 0 {
					t.Errorf("%s%s 缺少角色或光锥卡池：%v", v, phase, c)
				}
			}
		}
	}
	if c := phaseChannels["3.8额外"]; c["character"] == 0 || c["lightcone"] == 0 {
		t.Errorf("3.8额外 缺少角色或光锥卡池：%v", c)
	}
	if len(phaseChannels) != 65 {
		t.Errorf("阶段 %d 个，期望 65", len(phaseChannels))
	}
}

func TestStarRailBannerSamples(t *testing.T) {
	set, err := loadStarRailBanners()
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		version, phase, channel, up string
		start, end                  string
	}{
		{"1.0", "上半", "character", "希儿", "2023-04-26 10:00:00", "2023-05-17 17:59:59"},
		{"1.0", "下半", "character", "景元", "2023-05-17 18:00:00", "2023-06-06 15:00:00"},
		{"3.4", "上半", "character", "Saber", "2025-07-11 12:00:00", "9999-12-31 23:59:59"},
		{"3.4", "下半", "character", "流萤", "2025-07-23 12:00:00", "2025-08-12 15:00:00"},
		{"4.2", "下半", "lightcone", "邂逅于下一个花季", "2026-05-13 12:00:00", "2026-06-01 03:59:59"},
		{"4.7", "上半", "character", "银狼LV.999", "2026-11-10 15:00:01", "2026-12-02 11:59:59"},
	}
	for _, tt := range tests {
		b := findStarRailBanner(set.Banners, tt.version, tt.channel, tt.up)
		if b == nil {
			t.Errorf("找不到 %s %s %s", tt.version, tt.channel, tt.up)
			continue
		}
		if b.Phase != tt.phase || b.Start != tt.start || b.End != tt.end {
			t.Errorf("%s %s = %+v，期望 %s %s ~ %s", tt.version, tt.up, b, tt.phase, tt.start, tt.end)
		}
	}
}

func TestStarRailBannersAt(t *testing.T) {
	set, err := loadStarRailBanners()
	if err != nil {
		t.Fatal(err)
	}
	at := func(s string) time.Time {
		t.Helper()
		tm, err := starRailBannerTime(s)
		if err != nil {
			t.Fatal(err)
		}
		return tm
	}
	hasUP := func(bs []starRailBanner, name string) bool {
		for _, b := range bs {
			if slices.Contains(b.UP, name) {
				return true
			}
		}
		return false
	}

	if got := starRailBannersAt(set.Banners, "character", at("2023-04-25 23:59:00")); len(got) != 0 {
		t.Errorf("开服前返回了卡池：%+v", got)
	}
	if got := starRailBannersAt(set.Banners, "character", at("2023-04-27 12:00:00")); !hasUP(got, "希儿") {
		t.Errorf("1.0 上半未找到希儿：%+v", got)
	}
	got := starRailBannersAt(set.Banners, "character", at("2023-05-18 12:00:00"))
	if hasUP(got, "希儿") || !hasUP(got, "景元") {
		t.Errorf("1.0 下半卡池 %+v", got)
	}

	// 3.4 上半同时开放常规跃迁和长期联动跃迁。
	got = starRailBannersAt(set.Banners, "character", at("2025-07-15 12:00:00"))
	for _, name := range []string{"白厄", "Saber", "Archer"} {
		if !hasUP(got, name) {
			t.Errorf("3.4 上半缺少 %s：%+v", name, got)
		}
	}
	// 4.4 上半的联动跃迁在版本中途开放，且长期有效。
	got = starRailBannersAt(set.Banners, "character", at("2026-08-01 12:00:00"))
	for _, name := range []string{"火花", "姬子•启行", "吉尔伽美什", "远坂凛"} {
		if !hasUP(got, name) {
			t.Errorf("4.4 上半缺少 %s：%+v", name, got)
		}
	}
	if hasUP(got, "刻律德菈") {
		t.Errorf("4.4 下半尚未开放：%+v", got)
	}
	// 4.6 和 4.7 的交接。
	got = starRailBannersAt(set.Banners, "character", at("2026-11-06 12:00:00"))
	if !hasUP(got, "真珠") || !hasUP(got, "千冶•刃") || hasUP(got, "银狼LV.999") {
		t.Errorf("4.6 卡池 %+v", got)
	}
	got = starRailBannersAt(set.Banners, "character", at("2026-11-11 12:00:00"))
	if !hasUP(got, "银狼LV.999") || !hasUP(got, "星神★阿哈") || hasUP(got, "真珠") {
		t.Errorf("4.7 卡池 %+v", got)
	}
	// 光锥联动跃迁也长期有效。
	got = starRailBannersAt(set.Banners, "lightcone", at("2025-08-01 12:00:00"))
	if !hasUP(got, "理想燃烧的地狱") || !hasUP(got, "梦应归于何处") {
		t.Errorf("3.4 下半光锥卡池 %+v", got)
	}
}

func TestStarRailBannerName(t *testing.T) {
	tests := map[string]string{
		"希儿":         "希儿",
		"「希儿」":       "希儿",
		"丹恒·饮月":      "丹恒•饮月",
		"丹恒・饮月":      "丹恒•饮月",
		"丹恒•饮月":      "丹恒•饮月",
		"托帕＆账账":      "托帕&账账",
		" 希儿　":       "希儿",
		"当第一声「阿哈」响起": "当第一声「阿哈」响起",
	}
	for in, want := range tests {
		if got := starRailBannerName(in); got != want {
			t.Errorf("starRailBannerName(%q) = %q，期望 %q", in, got, want)
		}
	}
}

func TestAnnotateStarRailLost(t *testing.T) {
	set, err := loadStarRailBanners()
	if err != nil {
		t.Fatal(err)
	}
	d := newStarRailLostFixture()
	annotateStarRailLost(d, set.Banners)

	got := make(map[string]bool)
	for _, ch := range d.Channels {
		for _, r := range ch.Records {
			got[r.ID] = r.Lost
		}
	}
	want := map[string]bool{
		"1":  false, // 希儿是 1.0 上半 UP
		"2":  true,  // 姬子不是 UP
		"3":  false, // 景元是 1.0 下半 UP
		"4":  true,  // 希儿在下半已不是 UP
		"5":  false, // 白厄是 3.4 上半 UP
		"6":  false, // Saber 是长期联动 UP
		"7":  true,  // 姬子不是 3.4 上半 UP
		"8":  false, // 银狼LV.999 是 4.7 上半 UP
		"9":  false, // 星神★阿哈 是 4.7 上半 UP
		"10": false, // 于夜色中是 1.0 上半光锥 UP
		"11": true,  // 银河铁道之夜是常驻光锥
		"12": false, // 邂逅于下一个花季是 4.2 下半光锥 UP
		"13": false, // 常驻跃迁不判定歪
		"14": false, // 新手跃迁不判定歪
		"15": false, // Archer 是长期联动 UP
		"16": false, // 理想燃烧的地狱是长期联动光锥 UP
		"17": false, // A 级不判定歪
		"18": false, // 去掉「」后仍是 UP
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

func TestStarRailLostBadge(t *testing.T) {
	set, err := loadStarRailBanners()
	if err != nil {
		t.Fatal(err)
	}
	d := newStarRailLostFixture()
	annotateStarRailLost(d, set.Banners)

	a := newApp()
	a.selected = "starrail"
	a.starrail = d
	a.starrailLoaded = true
	tt := ui.NewTester(a.view, 960, 680)
	if err := tt.Click("记录"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("歪") {
		t.Errorf("记录页缺少歪标识，现有 %q", tt.Texts())
	}
	if !tt.HasText("希儿") || !tt.HasText("姬子") {
		t.Errorf("记录页缺少记录，现有 %q", tt.Texts())
	}
}

func TestLoadStarRailAnnotatesLost(t *testing.T) {
	dir := t.TempDir()
	exp := &gachaExport{
		UID:     "1",
		TypeMap: [][2]string{{"11", "角色活动跃迁"}},
		Result: []gachaGroup{{Type: "11", Records: []gachaRecord{
			{ID: "1", Name: "希儿", RankType: "5", Time: "2023-04-27 12:00:00"},
			{ID: "2", Name: "姬子", RankType: "5", Time: "2023-04-27 12:00:00"},
		}}},
	}
	if err := saveGachaExport(dir, exp); err != nil {
		t.Fatal(err)
	}
	d, err := loadGachaDir(dir, "starrail")
	if err != nil {
		t.Fatal(err)
	}
	got := make(map[string]bool)
	for _, ch := range d.Channels {
		for _, r := range ch.Records {
			got[r.ID] = r.Lost
		}
	}
	if got["1"] || !got["2"] {
		t.Errorf("读取后歪标记 %v，期望希儿不歪、姬子歪", got)
	}
}

// newStarRailLostFixture 构造一组跨越多个版本的 S 级记录。
func newStarRailLostFixture() *gachaData {
	return newGachaData(&gachaExport{
		UID: "1",
		TypeMap: [][2]string{
			{"11", "角色活动跃迁"},
			{"12", "光锥活动跃迁"},
			{"1", "常驻跃迁"},
			{"2", "新手跃迁"},
			{"21", "角色联动跃迁"},
			{"22", "光锥联动跃迁"},
		},
		Result: []gachaGroup{
			{Type: "11", Records: []gachaRecord{
				{ID: "1", Name: "希儿", RankType: "5", Time: "2023-04-27 12:00:00"},
				{ID: "2", Name: "姬子", RankType: "5", Time: "2023-04-27 12:00:00"},
				{ID: "3", Name: "景元", RankType: "5", Time: "2023-05-18 12:00:00"},
				{ID: "4", Name: "希儿", RankType: "5", Time: "2023-05-18 12:00:00"},
				{ID: "5", Name: "白厄", RankType: "5", Time: "2025-07-15 12:00:00"},
				{ID: "6", Name: "Saber", RankType: "5", Time: "2025-07-15 12:00:00"},
				{ID: "7", Name: "姬子", RankType: "5", Time: "2025-07-15 12:00:00"},
				{ID: "8", Name: "银狼LV.999", RankType: "5", Time: "2026-11-11 12:00:00"},
				{ID: "9", Name: "星神★阿哈", RankType: "5", Time: "2026-11-11 12:00:00"},
				{ID: "17", Name: "姬子", RankType: "4", Time: "2023-04-27 12:00:00"},
				{ID: "18", Name: "「希儿」", RankType: "5", Time: "2023-04-27 12:00:00"},
			}},
			{Type: "12", Records: []gachaRecord{
				{ID: "10", Name: "于夜色中", RankType: "5", Time: "2023-04-27 12:00:00"},
				{ID: "11", Name: "银河铁道之夜", RankType: "5", Time: "2023-04-27 12:00:00"},
				{ID: "12", Name: "邂逅于下一个花季", RankType: "5", Time: "2026-05-20 12:00:00"},
			}},
			{Type: "1", Records: []gachaRecord{
				{ID: "13", Name: "姬子", RankType: "5", Time: "2023-04-27 12:00:00"},
			}},
			{Type: "2", Records: []gachaRecord{
				{ID: "14", Name: "姬子", RankType: "5", Time: "2023-04-27 12:00:00"},
			}},
			{Type: "21", Records: []gachaRecord{
				{ID: "15", Name: "Archer", RankType: "5", Time: "2025-08-01 12:00:00"},
			}},
			{Type: "22", Records: []gachaRecord{
				{ID: "16", Name: "理想燃烧的地狱", RankType: "5", Time: "2025-08-01 12:00:00"},
			}},
		},
	}, "starrail")
}

func findStarRailBanner(banners []starRailBanner, version, channel, up string) *starRailBanner {
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
