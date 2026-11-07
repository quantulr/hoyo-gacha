package main

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestGenshinBannerData(t *testing.T) {
	set, err := loadGenshinBanners()
	if err != nil {
		t.Fatal(err)
	}
	if set.Source == "" || set.Timezone != "UTC+8" || set.Updated == "" {
		t.Errorf("元数据 %+v", set)
	}
	if len(set.Banners) != 300 {
		t.Fatalf("祈愿 %d 个，期望 300", len(set.Banners))
	}

	channels := make(map[string]int)
	versions := make(map[string]bool)
	phases := make(map[string]map[string]int)
	var prev time.Time
	for i, b := range set.Banners {
		channels[b.Channel]++
		versions[b.Version] = true
		switch b.Phase {
		case "上半", "中", "下半":
		default:
			t.Errorf("第 %d 个祈愿阶段 %q", i+1, b.Phase)
		}
		if b.Title == "" || len(b.UP) == 0 {
			t.Errorf("第 %d 个祈愿缺少标题或 UP：%+v", i+1, b)
		}
		wantUP := 1
		if b.Channel == "weapon" {
			wantUP = 2
		}
		if len(b.UP) != wantUP {
			t.Errorf("第 %d 个祈愿有 %d 个 UP，期望 %d：%+v", i+1, len(b.UP), wantUP, b)
		}
		start, err := genshinBannerTime(b.Start)
		if err != nil {
			t.Errorf("第 %d 个祈愿开始时间：%v", i+1, err)
			continue
		}
		end, err := genshinBannerTime(b.End)
		if err != nil {
			t.Errorf("第 %d 个祈愿结束时间：%v", i+1, err)
			continue
		}
		if !end.After(start) {
			t.Errorf("第 %d 个祈愿结束时间不晚于开始时间", i+1)
		}
		if start.Before(prev) {
			t.Errorf("第 %d 个祈愿未按时间排序：%s 早于 %s", i+1, b.Start, prev.Format(genshinBannerLayout))
		}
		prev = start
		key := b.Version + b.Phase
		if phases[key] == nil {
			phases[key] = make(map[string]int)
		}
		phases[key][b.Channel]++
	}
	if channels["character"] != 192 || channels["weapon"] != 108 {
		t.Errorf("频道统计 %v，期望角色 192、武器 108", channels)
	}
	if len(versions) != 54 {
		t.Errorf("版本 %d 个，期望 54", len(versions))
	}
	if len(phases) != 109 {
		t.Errorf("阶段 %d 个，期望 109", len(phases))
	}
	for key, c := range phases {
		if c["character"] == 0 {
			t.Errorf("%s 缺少角色祈愿：%v", key, c)
		}
		if key != "1.3中" && c["weapon"] == 0 {
			t.Errorf("%s 缺少武器祈愿：%v", key, c)
		}
	}
	if c := phases["1.3中"]; c["weapon"] != 0 {
		t.Errorf("1.3中 不应有武器祈愿：%v", c)
	}

	for major, maxMinor := range map[int]int{1: 6, 2: 8, 3: 8, 4: 8, 5: 8} {
		for minor := 0; minor <= maxMinor; minor++ {
			v := fmt.Sprintf("%d.%d", major, minor)
			if !versions[v] {
				t.Errorf("缺少 %s 版本祈愿", v)
			}
		}
	}
	for _, v := range []string{"月之一", "月之二", "月之三", "月之四", "月之五", "月之六", "月之七", "月之八"} {
		if !versions[v] {
			t.Errorf("缺少 %s 版本祈愿", v)
		}
	}
	for minor := 0; minor <= 2; minor++ {
		v := fmt.Sprintf("7.%d", minor)
		if !versions[v] {
			t.Errorf("缺少 %s 版本祈愿", v)
		}
	}
}

func TestGenshinBannerSamples(t *testing.T) {
	set, err := loadGenshinBanners()
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		version, phase, channel, up string
		start, end                  string
	}{
		{"1.0", "上半", "character", "温迪", "2020-09-28 12:00:00", "2020-10-18 17:59:59"},
		{"1.0", "上半", "weapon", "风鹰剑", "2020-09-28 12:00:00", "2020-10-18 17:59:59"},
		{"1.3", "中", "character", "刻晴", "2021-02-17 18:00:00", "2021-03-02 15:59:59"},
		{"2.0", "上半", "character", "神里绫华", "2021-07-20 18:00:00", "2021-08-10 17:59:59"},
		{"3.0", "上半", "character", "钟离", "2022-08-24 11:00:00", "2022-09-09 17:59:59"},
		{"4.0", "上半", "character", "夜兰", "2023-08-15 15:00:00", "2023-09-05 17:59:59"},
		{"7.2", "上半", "character", "米提亚", "2026-11-04 06:00:00", "2026-11-24 17:59:59"},
		{"7.2", "上半", "weapon", "秘星典谕", "2026-11-04 06:00:00", "2026-11-24 17:59:59"},
	}
	for _, tt := range tests {
		b := findGenshinBanner(set.Banners, tt.version, tt.channel, tt.up)
		if b == nil {
			t.Errorf("找不到 %s %s %s", tt.version, tt.channel, tt.up)
			continue
		}
		if b.Phase != tt.phase || b.Start != tt.start || b.End != tt.end {
			t.Errorf("%s %s = %+v，期望 %s %s ~ %s", tt.version, tt.up, b, tt.phase, tt.start, tt.end)
		}
	}
}

func TestGenshinBannersAt(t *testing.T) {
	set, err := loadGenshinBanners()
	if err != nil {
		t.Fatal(err)
	}
	at := func(s string) time.Time {
		t.Helper()
		tm, err := genshinBannerTime(s)
		if err != nil {
			t.Fatal(err)
		}
		return tm
	}
	hasUP := func(bs []genshinBanner, name string) bool {
		for _, b := range bs {
			if slices.Contains(b.UP, name) {
				return true
			}
		}
		return false
	}

	if got := genshinBannersAt(set.Banners, "character", at("2020-09-27 12:00:00")); len(got) != 0 {
		t.Errorf("开服前返回了祈愿：%+v", got)
	}
	got := genshinBannersAt(set.Banners, "character", at("2020-10-01 12:00:00"))
	if !hasUP(got, "温迪") {
		t.Errorf("1.0 上半未找到温迪：%+v", got)
	}
	got = genshinBannersAt(set.Banners, "weapon", at("2020-10-01 12:00:00"))
	if !hasUP(got, "风鹰剑") || !hasUP(got, "阿莫斯之弓") {
		t.Errorf("1.0 上半武器 UP 不全：%+v", got)
	}
	got = genshinBannersAt(set.Banners, "character", at("2020-10-25 12:00:00"))
	if hasUP(got, "温迪") || !hasUP(got, "可莉") {
		t.Errorf("1.0 下半角色祈愿 %+v", got)
	}
	got = genshinBannersAt(set.Banners, "weapon", at("2020-10-25 12:00:00"))
	if hasUP(got, "风鹰剑") || !hasUP(got, "四风原典") || !hasUP(got, "狼的末路") {
		t.Errorf("1.0 下半武器祈愿 %+v", got)
	}

	// 1.3 中期刻晴祈愿开放时，上一期武器祈愿仍在进行。
	got = genshinBannersAt(set.Banners, "character", at("2021-02-20 12:00:00"))
	if !hasUP(got, "刻晴") {
		t.Errorf("1.3 中未找到刻晴：%+v", got)
	}
	got = genshinBannersAt(set.Banners, "weapon", at("2021-02-20 12:00:00"))
	if !hasUP(got, "和璞鸢") || !hasUP(got, "磐岩结绿") {
		t.Errorf("1.3 中武器祈愿 %+v", got)
	}

	// 4.0 上半同时有两期角色祈愿。
	got = genshinBannersAt(set.Banners, "character", at("2023-08-20 12:00:00"))
	if !hasUP(got, "林尼") || !hasUP(got, "夜兰") {
		t.Errorf("4.0 上半角色祈愿 %+v", got)
	}
	got = genshinBannersAt(set.Banners, "weapon", at("2023-08-20 12:00:00"))
	if !hasUP(got, "最初的大魔术") || !hasUP(got, "若水") {
		t.Errorf("4.0 上半武器祈愿 %+v", got)
	}

	// 当前 7.2 上半与即将到来的下半。
	got = genshinBannersAt(set.Banners, "character", at("2026-11-06 12:00:00"))
	if !hasUP(got, "米提亚") || !hasUP(got, "芙宁娜") || hasUP(got, "兹白") {
		t.Errorf("7.2 上半角色祈愿 %+v", got)
	}
	got = genshinBannersAt(set.Banners, "weapon", at("2026-11-06 12:00:00"))
	if !hasUP(got, "秘星典谕") || !hasUP(got, "静水流涌之辉") {
		t.Errorf("7.2 上半武器祈愿 %+v", got)
	}
	got = genshinBannersAt(set.Banners, "character", at("2026-11-25 12:00:00"))
	if hasUP(got, "米提亚") || hasUP(got, "芙宁娜") || !hasUP(got, "兹白") || !hasUP(got, "莉奈娅") {
		t.Errorf("7.2 下半角色祈愿 %+v", got)
	}
}

func TestGenshinBannerName(t *testing.T) {
	tests := map[string]string{
		"温迪":     "温迪",
		"「温迪」":   "温迪",
		" 温迪　":   "温迪",
		"风鹰剑":    "风鹰剑",
		"「风鹰剑」":  "风鹰剑",
		"阿莫斯之弓":  "阿莫斯之弓",
		"天空之刃":   "天空之刃",
		"最初的大魔术": "最初的大魔术",
	}
	for in, want := range tests {
		if got := genshinBannerName(in); got != want {
			t.Errorf("genshinBannerName(%q) = %q，期望 %q", in, got, want)
		}
	}
}

func TestAnnotateGenshinLost(t *testing.T) {
	set, err := loadGenshinBanners()
	if err != nil {
		t.Fatal(err)
	}
	d := newGenshinLostFixture()
	annotateGenshinLost(d, set.Banners)

	got := make(map[string]bool)
	for _, ch := range d.Channels {
		for _, r := range ch.Records {
			got[r.ID] = r.Lost
		}
	}
	want := map[string]bool{
		"1":  false, // 温迪是 1.0 上半 UP
		"2":  true,  // 迪卢克不是 UP
		"3":  false, // 可莉是 1.0 下半 UP
		"4":  true,  // 温迪在下半已不是 UP
		"5":  false, // 刻晴是 1.3 中 UP
		"6":  false, // 林尼是 4.0 上半 UP
		"7":  false, // 夜兰与林尼同期 UP
		"8":  false, // 米提亚是 7.2 上半 UP
		"9":  false, // 芙宁娜是 7.2 上半 UP
		"10": false, // A 级不判定歪
		"11": false, // 风鹰剑是 1.0 上半武器 UP
		"12": true,  // 天空之刃是常驻武器
		"13": false, // 阿莫斯之弓是同期另一把 UP
		"14": false, // 四风原典是 1.0 下半武器 UP
		"15": true,  // 风鹰剑在下半已不是 UP
		"16": false, // 和璞鸢在 1.3 中期仍开放
		"17": false, // 护摩之杖是 1.3 下半武器 UP
		"18": true,  // 天空之刃不是 1.3 下半 UP
		"19": false, // 集录祈愿不在本页数据里，不判定
		"20": false, // 常驻祈愿不判定
		"21": false, // 新手祈愿不判定
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

func TestGenshinLostStats(t *testing.T) {
	set, err := loadGenshinBanners()
	if err != nil {
		t.Fatal(err)
	}
	d := newGenshinLostFixture()
	annotateGenshinLost(d, set.Banners)

	lost, smallPity := d.LostStats()
	if lost != 5 || smallPity != 13 {
		t.Fatalf("歪卡统计 %d / %d，期望 5 / 13", lost, smallPity)
	}
	if d.LostRate() != float64(5)/13 {
		t.Errorf("歪卡率 %.4f，期望 %.4f", d.LostRate(), float64(5)/13)
	}
	if !d.HasLostChannels() {
		t.Error("限定频段应支持歪卡判定")
	}

	wantLost := map[string]struct {
		lost, smallPity int
		rate            float64
	}{
		"301": {2, 7, float64(2) / 7},
		"302": {3, 6, float64(3) / 6},
	}
	for _, ch := range d.Channels {
		want, tracks := wantLost[ch.Type]
		if ch.LostTracked != tracks {
			t.Errorf("频段 %s LostTracked = %v，期望 %v", ch.Type, ch.LostTracked, tracks)
		}
		if !tracks {
			continue
		}
		if ch.LostS != want.lost || ch.SmallPityS != want.smallPity {
			t.Errorf("频段 %s 歪卡统计 %d / %d，期望 %d / %d",
				ch.Type, ch.LostS, ch.SmallPityS, want.lost, want.smallPity)
		}
		if ch.LostRate() != want.rate {
			t.Errorf("频段 %s 歪卡率 %.4f，期望 %.4f", ch.Type, ch.LostRate(), want.rate)
		}
	}

	// 歪后的下一个 S 是大保底；它可能仍是 UP，但不参与 50/50 统计。
	wantGuaranteed := map[string]bool{"3": true, "5": true, "13": true, "16": true}
	for _, ch := range d.Channels {
		for _, r := range ch.Records {
			if r.Guaranteed != wantGuaranteed[r.ID] {
				t.Errorf("记录 %s 大保底 = %v，期望 %v", r.ID, r.Guaranteed, wantGuaranteed[r.ID])
			}
		}
	}
}

func TestGenshinLostRateCard(t *testing.T) {
	set, err := loadGenshinBanners()
	if err != nil {
		t.Fatal(err)
	}
	d := newGenshinLostFixture()
	annotateGenshinLost(d, set.Banners)

	a := newApp()
	a.selected = "genshin"
	a.genshin = d
	a.genshinLoaded = true
	tt := ui.NewTester(a.view, 960, 680)
	for _, text := range []string{
		"歪卡率", "38.46%", "5 歪 / 13 次小保底",
		"各卡池歪卡率", "28.57%", "2 歪 / 7 次小保底",
		"50.00%", "3 歪 / 6 次小保底",
	} {
		if !tt.HasText(text) {
			t.Errorf("缺少文本 %q，现有 %q", text, tt.Texts())
		}
	}
}

func TestGenshinLostRateWithoutBannerData(t *testing.T) {
	a := newApp()
	a.selected = "genshin"
	a.genshin = testGenshinData(t)
	a.genshinLoaded = true
	tt := ui.NewTester(a.view, 960, 680)
	for _, text := range []string{"歪卡率", "—", "暂无可判定卡池"} {
		if !tt.HasText(text) {
			t.Errorf("缺少文本 %q，现有 %q", text, tt.Texts())
		}
	}
	if tt.HasText("各卡池歪卡率") {
		t.Errorf("没有卡池数据时不应显示分池歪卡率，现有 %q", tt.Texts())
	}
}

func TestGenshinLostBadge(t *testing.T) {
	set, err := loadGenshinBanners()
	if err != nil {
		t.Fatal(err)
	}
	d := newGenshinLostFixture()
	annotateGenshinLost(d, set.Banners)

	a := newApp()
	a.selected = "genshin"
	a.genshin = d
	a.genshinLoaded = true
	tt := ui.NewTester(a.view, 960, 680)
	if err := tt.Click("记录"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("歪") {
		t.Errorf("记录页缺少歪标识，现有 %q", tt.Texts())
	}
	if !tt.HasText("温迪") || !tt.HasText("迪卢克") {
		t.Errorf("记录页缺少记录，现有 %q", tt.Texts())
	}
}

func TestLoadGenshinAnnotatesLost(t *testing.T) {
	dir := t.TempDir()
	exp := &gachaExport{
		UID:     "1",
		TypeMap: [][2]string{{"301", "角色活动祈愿"}},
		Result: []gachaGroup{{Type: "301", Records: []gachaRecord{
			{ID: "1", Name: "温迪", RankType: "5", Time: "2020-10-01 12:00:00"},
			{ID: "2", Name: "迪卢克", RankType: "5", Time: "2020-10-01 12:00:00"},
		}}},
	}
	if err := saveGachaExport(dir, exp); err != nil {
		t.Fatal(err)
	}
	d, err := loadGachaDir(dir, "genshin")
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
		t.Errorf("读取后歪标记 %v，期望温迪不歪、迪卢克歪", got)
	}
}

// newGenshinLostFixture 构造一组跨越多个版本的 5 星记录。
func newGenshinLostFixture() *gachaData {
	return newGachaData(&gachaExport{
		UID: "1",
		TypeMap: [][2]string{
			{"301", "角色活动祈愿"},
			{"302", "武器活动祈愿"},
			{"500", "集录祈愿"},
			{"200", "常驻祈愿"},
			{"100", "新手祈愿"},
		},
		Result: []gachaGroup{
			{Type: "301", Records: []gachaRecord{
				{ID: "1", Name: "温迪", RankType: "5", Time: "2020-10-01 12:00:00"},
				{ID: "2", Name: "迪卢克", RankType: "5", Time: "2020-10-01 12:00:00"},
				{ID: "3", Name: "可莉", RankType: "5", Time: "2020-10-25 12:00:00"},
				{ID: "4", Name: "温迪", RankType: "5", Time: "2020-10-25 12:00:00"},
				{ID: "5", Name: "刻晴", RankType: "5", Time: "2021-02-20 12:00:00"},
				{ID: "6", Name: "林尼", RankType: "5", Time: "2023-08-20 12:00:00"},
				{ID: "7", Name: "夜兰", RankType: "5", Time: "2023-08-20 12:00:00"},
				{ID: "8", Name: "米提亚", RankType: "5", Time: "2026-11-06 12:00:00"},
				{ID: "9", Name: "芙宁娜", RankType: "5", Time: "2026-11-06 12:00:00"},
				{ID: "10", Name: "迪卢克", RankType: "4", Time: "2020-10-01 12:00:00"},
			}},
			{Type: "302", Records: []gachaRecord{
				{ID: "11", Name: "风鹰剑", RankType: "5", Time: "2020-10-01 12:00:00"},
				{ID: "12", Name: "天空之刃", RankType: "5", Time: "2020-10-01 12:00:00"},
				{ID: "13", Name: "阿莫斯之弓", RankType: "5", Time: "2020-10-01 12:00:00"},
				{ID: "14", Name: "四风原典", RankType: "5", Time: "2020-10-25 12:00:00"},
				{ID: "15", Name: "风鹰剑", RankType: "5", Time: "2020-10-25 12:00:00"},
				{ID: "16", Name: "和璞鸢", RankType: "5", Time: "2021-02-20 12:00:00"},
				{ID: "17", Name: "护摩之杖", RankType: "5", Time: "2021-03-01 12:00:00"},
				{ID: "18", Name: "天空之刃", RankType: "5", Time: "2021-03-01 12:00:00"},
			}},
			{Type: "500", Records: []gachaRecord{
				{ID: "19", Name: "刻晴", RankType: "5", Time: "2026-11-06 12:00:00"},
			}},
			{Type: "200", Records: []gachaRecord{
				{ID: "20", Name: "迪卢克", RankType: "5", Time: "2020-10-01 12:00:00"},
			}},
			{Type: "100", Records: []gachaRecord{
				{ID: "21", Name: "迪卢克", RankType: "5", Time: "2020-10-01 12:00:00"},
			}},
		},
	}, "genshin")
}

func findGenshinBanner(banners []genshinBanner, version, channel, up string) *genshinBanner {
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
