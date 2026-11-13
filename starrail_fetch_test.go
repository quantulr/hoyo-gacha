package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// writeStarRailCache 在游戏目录里写入一份网页缓存，返回 data_2 路径。
func writeStarRailCache(t *testing.T, gameDir, version, text string) string {
	t.Helper()
	path := filepath.Join(gameDir, "StarRail_Data", "webCaches", version, "Cache", "Cache_Data", "data_2")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestFindStarRailLogURL(t *testing.T) {
	dir := t.TempDir()
	gameDir := filepath.Join(dir, "Star Rail Game")
	oldURL := "https://webstatic.mihoyo.com/hkrpg/event/x.html?auth_appid=webview_gacha&authkey=OLD&game_biz=hkrpg_cn"
	newURL := "https://webstatic.mihoyo.com/hkrpg/event/x.html?auth_appid=webview_gacha&authkey=NEW&game_biz=hkrpg_cn"
	oldCache := writeStarRailCache(t, gameDir, "1.0.0", "junk\x00"+oldURL+"\x00junk")
	newCache := writeStarRailCache(t, gameDir, "2.0.0", "junk\x00"+newURL+"\x00junk")
	oldTime := time.Now().Add(-time.Hour)
	if err := os.Chtimes(oldCache, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(newCache, time.Now(), time.Now()); err != nil {
		t.Fatal(err)
	}

	logPath := filepath.Join(dir, "Player.log")
	logText := "Loading player data from " + filepath.ToSlash(filepath.Join(gameDir, "StarRail_Data")) +
		"/data.unity3d\r\nTelemetryInterface path:" + filepath.Join(gameDir, "StarRail_Data") + `\SDKCaches` + "\r\n"
	if err := os.WriteFile(logPath, []byte(logText), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := findStarRailLogURL([]string{logPath})
	if err != nil {
		t.Fatal(err)
	}
	if got != newURL {
		t.Errorf("应取最新缓存里的链接，得到 %q", got)
	}

	// 缓存里没有完整链接时不返回。
	if err := os.WriteFile(newCache, []byte("https://example.com/"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err = findStarRailLogURL([]string{logPath})
	if err != nil {
		t.Fatal(err)
	}
	if got != oldURL {
		t.Errorf("应回退到旧缓存，得到 %q", got)
	}

	// 日志里没有游戏路径时报错。
	if err := os.WriteFile(logPath, []byte("nothing here"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := findStarRailLogURL([]string{logPath}); err == nil {
		t.Error("没有游戏路径时没有报错")
	}
	if _, err := findStarRailLogURL([]string{filepath.Join(dir, "missing.log")}); err == nil {
		t.Error("没有日志时没有报错")
	}
}

func TestFindStarRailLogURLFallsBackToLogWhenCacheLocked(t *testing.T) {
	dir := t.TempDir()
	gameDir := filepath.Join(dir, "Star Rail Game")
	gamePath := filepath.ToSlash(filepath.Join(gameDir, "StarRail_Data"))
	oldURL := "https://webstatic.mihoyo.com/hkrpg/event/x.html?auth_appid=webview_gacha&authkey=OLD&game_biz=hkrpg_cn&timestamp=100"
	logURL := "https://webstatic.mihoyo.com/hkrpg/event/x.html?auth_appid=webview_gacha&authkey=LOG&game_biz=hkrpg_cn&timestamp=200"

	oldCache := writeStarRailCache(t, gameDir, "2.53.0.0", "junk\x00"+oldURL+"\x00junk")
	oldTime := time.Now().Add(-time.Hour)
	if err := os.Chtimes(oldCache, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	// 最新的 data_2 建成目录，模拟游戏独占缓存导致读取失败。
	locked := filepath.Join(gameDir, "StarRail_Data", "webCaches", "2.54.0.0", "Cache", "Cache_Data", "data_2")
	if err := os.MkdirAll(locked, 0o755); err != nil {
		t.Fatal(err)
	}

	logPath := filepath.Join(dir, "Player.log")
	logText := "Loading player data from " + gamePath + "/data.unity3d\r\n" + logURL + "\r\n"
	if err := os.WriteFile(logPath, []byte(logText), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := findStarRailLogURL([]string{logPath})
	if err != nil {
		t.Fatal(err)
	}
	if got != logURL {
		t.Errorf("缓存被占用时应使用日志里的链接，得到 %q", got)
	}

	// 日志里没有链接时，不应回退到旧版本目录里已经过期的链接。
	logText = "Loading player data from " + gamePath + "/data.unity3d\r\n"
	if err := os.WriteFile(logPath, []byte(logText), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, err := findStarRailLogURL([]string{logPath}); err == nil || !strings.Contains(err.Error(), "占用") {
		t.Errorf("缓存被占用且日志没有链接时错误 = %v，链接 = %q", err, got)
	}
}

func TestFindStarRailLogURLPrefersNewerCacheURL(t *testing.T) {
	dir := t.TempDir()
	gameDir := filepath.Join(dir, "Star Rail Game")
	gamePath := filepath.ToSlash(filepath.Join(gameDir, "StarRail_Data"))
	cacheURL := "https://webstatic.mihoyo.com/hkrpg/event/x.html?auth_appid=webview_gacha&authkey=CACHE&game_biz=hkrpg_cn&timestamp=300"
	logURL := "https://webstatic.mihoyo.com/hkrpg/event/x.html?auth_appid=webview_gacha&authkey=LOG&game_biz=hkrpg_cn&timestamp=200"
	writeStarRailCache(t, gameDir, "2.54.0.0", "junk\x00"+cacheURL+"\x00junk")

	logPath := filepath.Join(dir, "Player.log")
	logText := "Loading player data from " + gamePath + "/data.unity3d\r\n" + logURL + "\r\n"
	if err := os.WriteFile(logPath, []byte(logText), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := findStarRailLogURL([]string{logPath})
	if err != nil {
		t.Fatal(err)
	}
	if got != cacheURL {
		t.Errorf("应优先使用缓存里较新的链接，得到 %q", got)
	}
}

func TestParseStarRailLogURL(t *testing.T) {
	raw := "https://webstatic.mihoyo.com/hkrpg/event/index.html?authkey=abc+def=ghi&game_biz=hkrpg_cn&lang=zh-cn&page=2&size=20&gacha_type=11&end_id=123&begin_id=456"
	api, err := parseStarRailLogURL(raw)
	if err != nil {
		t.Fatal(err)
	}
	if api.domain != starRailAPICN || api.lang != "zh-cn" {
		t.Errorf("域名 %q，语言 %q", api.domain, api.lang)
	}
	q, err := url.ParseQuery(api.query)
	if err != nil {
		t.Fatal(err)
	}
	if q.Get("authkey") != "abc+def=ghi" {
		t.Errorf("authkey 变成 %q", q.Get("authkey"))
	}
	for _, key := range []string{"page", "size", "gacha_type", "end_id", "begin_id"} {
		if q.Has(key) {
			t.Errorf("分页参数 %q 应被移除", key)
		}
	}

	osAPI, err := parseStarRailLogURL("https://webstatic-sea.hoyoverse.com/hkrpg/index.html?authkey=OS&lang=en-us")
	if err != nil {
		t.Fatal(err)
	}
	if osAPI.domain != starRailAPIOS || osAPI.lang != "en-us" {
		t.Errorf("国际服域名 %q，语言 %q", osAPI.domain, osAPI.lang)
	}

	if _, err := parseStarRailLogURL("https://webstatic.mihoyo.com/hkrpg/index.html"); err == nil {
		t.Error("缺少 authkey 时没有报错")
	}
}

func TestFetchStarRailAll(t *testing.T) {
	root := t.TempDir()
	local := &gachaExport{
		UID:            "42",
		Lang:           "zh-cn",
		RegionTimeZone: 8,
		TypeMap:        [][2]string{{"11", "角色活动跃迁"}},
		Result: []gachaGroup{{Type: "11", Records: []gachaRecord{
			{ID: "999", Name: "旧甲", RankType: "3", Time: "2024-01-01 10:00:00", GachaType: "11", Count: "1"},
			{ID: "1001", Name: "旧乙", RankType: "4", Time: "2024-01-02 10:00:00", GachaType: "11", Count: "1"},
		}}},
	}
	if err := saveGachaExport(root, "starrail", local); err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("authkey") != "test" {
			t.Errorf("authkey = %q", q.Get("authkey"))
		}
		typ, page, size, endID := q.Get("gacha_type"), q.Get("page"), q.Get("size"), q.Get("end_id")
		if typ == "21" || typ == "22" {
			if r.URL.Path != starRailLdGachaPath {
				t.Errorf("联动跃迁 %s 的路径 %q", typ, r.URL.Path)
			}
		} else if r.URL.Path != starRailGachaPath {
			t.Errorf("跃迁 %s 的路径 %q", typ, r.URL.Path)
		}
		mu.Lock()
		requests = append(requests, fmt.Sprintf("%s/%s/%s/%s", typ, page, size, endID))
		mu.Unlock()

		var list []apiGachaRecord
		if typ == "11" {
			switch {
			case size == "5": // 预检
				list = []apiGachaRecord{{UID: "42", gachaRecord: gachaRecord{ID: "1003"}}}
			case page == "1" && endID == "0":
				list = []apiGachaRecord{
					{UID: "42", gachaRecord: gachaRecord{ID: "1003", Name: "新丙", RankType: "5", Time: "2024-01-03 10:00:00", GachaType: "11", Count: "1"}},
					{UID: "42", gachaRecord: gachaRecord{ID: "1002", Name: "新丁", RankType: "3", Time: "2024-01-01 12:00:00", GachaType: "11", Count: "1"}},
				}
			case page == "2" && endID == "1002":
				list = []apiGachaRecord{
					{UID: "42", gachaRecord: gachaRecord{ID: "1001", Name: "旧乙", RankType: "4", Time: "2024-01-02 10:00:00", GachaType: "11", Count: "1"}},
				}
			default:
				t.Errorf("意外的分页请求 typ=%s page=%s end=%s", typ, page, endID)
			}
		}
		json.NewEncoder(w).Encode(map[string]any{
			"retcode": 0,
			"message": "OK",
			"data": map[string]any{
				"list":             list,
				"region":           "prod_gf_cn",
				"region_time_zone": 8,
			},
		})
	}))
	defer server.Close()

	oldClient, oldSleep := starRailHTTPClient, starRailSleep
	starRailHTTPClient, starRailSleep = server.Client(), func(time.Duration) {}
	defer func() { starRailHTTPClient, starRailSleep = oldClient, oldSleep }()

	api := &starRailAPIURL{domain: server.URL, query: "authkey=test&lang=zh-cn&game_biz=hkrpg_cn", lang: "zh-cn"}
	res, err := fetchStarRailAll(root, api, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.UID != "42" || res.Added != 2 || res.Total != 4 {
		t.Errorf("结果 %+v", res)
	}

	d, err := loadGachaGame(root, "starrail")
	if err != nil {
		t.Fatal(err)
	}
	if d.TotalPulls != 4 || d.TotalS != 1 || d.TotalA != 1 {
		t.Errorf("统计 %+v", d)
	}
	if len(d.Channels) != 1 || d.Channels[0].Type != "11" || d.Channels[0].Pulls != 4 {
		t.Fatalf("跃迁 %+v", d.Channels)
	}
	want := []string{"1003", "1001", "1002", "999"}
	for i, id := range want {
		if d.Channels[0].Records[i].ID != id {
			t.Errorf("第 %d 条 id %q，期望 %q", i, d.Channels[0].Records[i].ID, id)
		}
	}

	saved, err := loadGachaLocal(gameDataDir(root, "starrail"), "42")
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.Result) != len(starRailChannelOrder) {
		t.Fatalf("保存的跃迁分组 %+v", saved.Result)
	}
	for i, typ := range starRailChannelOrder {
		if saved.Result[i].Type != typ {
			t.Errorf("第 %d 组 %q，期望 %q", i, saved.Result[i].Type, typ)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	sawLd := false
	for _, req := range requests {
		if strings.HasPrefix(req, "11/3/") {
			t.Errorf("不应请求第 3 页：%q", req)
		}
		if strings.HasPrefix(req, "21/") || strings.HasPrefix(req, "22/") {
			sawLd = true
		}
	}
	if !sawLd {
		t.Error("没有请求联动跃迁")
	}
}

func TestStarRailFetchChannelBackfillsLateRecord(t *testing.T) {
	var mu sync.Mutex
	var pages []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		page, endID := q.Get("page"), q.Get("end_id")
		mu.Lock()
		pages = append(pages, page+"/"+endID)
		mu.Unlock()
		var list []apiGachaRecord
		switch page + "/" + endID {
		case "1/0":
			list = []apiGachaRecord{
				{UID: "42", gachaRecord: gachaRecord{ID: "1004"}},
				{UID: "42", gachaRecord: gachaRecord{ID: "1003"}},
			}
		case "2/1003":
			list = []apiGachaRecord{
				// 本地最新一条（1003）之后才写入服务器的旧记录。
				{UID: "42", gachaRecord: gachaRecord{ID: "1002"}},
				{UID: "42", gachaRecord: gachaRecord{ID: "1001"}},
			}
		case "3/1001":
			// 空页，结束。
		default:
			t.Errorf("意外的分页请求 %s/%s", page, endID)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"retcode": 0,
			"message": "OK",
			"data":    map[string]any{"list": list},
		})
	}))
	defer server.Close()

	oldClient, oldSleep := starRailHTTPClient, starRailSleep
	starRailHTTPClient, starRailSleep = server.Client(), func(time.Duration) {}
	defer func() { starRailHTTPClient, starRailSleep = oldClient, oldSleep }()

	api := &starRailAPIURL{domain: server.URL, query: "authkey=test", lang: "zh-cn"}
	localIDs := map[string]bool{"1001": true, "1003": true}
	recs, err := starRailFetchChannel(api, "11", "角色活动跃迁", localIDs, nil)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, rec := range recs {
		got = append(got, rec.ID)
	}
	want := []string{"1004", "1003", "1002", "1001"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("记录 %v，期望 %v", got, want)
	}
}

func TestFetchStarRailAuthkeyExpired(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"retcode": -100, "message": "authkey timeout"})
	}))
	defer server.Close()

	oldClient, oldSleep := starRailHTTPClient, starRailSleep
	starRailHTTPClient, starRailSleep = server.Client(), func(time.Duration) {}
	defer func() { starRailHTTPClient, starRailSleep = oldClient, oldSleep }()

	api := &starRailAPIURL{domain: server.URL, query: "authkey=test", lang: "zh-cn"}
	_, err := fetchStarRailAll(t.TempDir(), api, nil)
	if !errors.Is(err, errStarRailAuthkey) {
		t.Fatalf("错误 = %v，期望 authkey 失效", err)
	}
}

func TestStarRailRefreshOverlay(t *testing.T) {
	a := newApp()
	a.selected = "starrail"
	a.starrail = testStarRailData(t)
	a.starrailLoaded = true
	a.starrailFetching = true
	a.starrailStatus = "正在读取角色活动跃迁（第 2 页）…"
	tt := ui.NewTester(a.view, 960, 680)
	for _, text := range []string{"正在刷新抽卡记录", "正在读取角色活动跃迁（第 2 页）…"} {
		if !tt.HasText(text) {
			t.Errorf("缺少文本 %q，现有 %q", text, tt.Texts())
		}
	}
}

func TestStarRailRefreshButton(t *testing.T) {
	a := newApp()
	a.selected = "starrail"
	a.starrail = testStarRailData(t)
	a.starrailLoaded = true
	a.starrailStatus = "已是最新，共 6 条记录"
	tt := ui.NewTester(a.view, 960, 680)
	if !tt.HasText("刷新") {
		t.Errorf("缺少刷新按钮，现有 %q", tt.Texts())
	}
	if tt.HasText("已是最新，共 6 条记录") {
		t.Errorf("刷新结果不应显示在页面顶部，现有 %q", tt.Texts())
	}

	a.starrailStatus = ""
	a.starrailToast = "已是最新，共 6 条记录"
	tt.Frame()
	if !tt.HasText("已是最新，共 6 条记录") {
		t.Errorf("缺少刷新结果 toast，现有 %q", tt.Texts())
	}
	if a.starrailToast != "" {
		t.Error("toast 应只显示一次")
	}
}
