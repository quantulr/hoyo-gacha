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

// writeGenshinCache 在游戏数据目录里写入一份网页缓存，返回 data_2 路径。
func writeGenshinCache(t *testing.T, gameDataDir, version, text string) string {
	t.Helper()
	path := filepath.Join(gameDataDir, "webCaches", version, "Cache", "Cache_Data", "data_2")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestFindGenshinLogURL(t *testing.T) {
	dir := t.TempDir()
	gameDir := filepath.Join(dir, "Genshin Impact")
	dataDir := filepath.Join(gameDir, "GenshinImpact_Data")
	oldURL := "https://webstatic.mihoyo.com/hk4e/event/e20190909gacha/index.html?auth_appid=webview_gacha&authkey=OLD&game_biz=hk4e_cn"
	newURL := "https://webstatic.mihoyo.com/hk4e/event/e20190909gacha/index.html?auth_appid=webview_gacha&authkey=NEW&game_biz=hk4e_cn"
	oldCache := writeGenshinCache(t, dataDir, "1.0.0", "junk\x00"+oldURL+"\x00junk")
	newCache := writeGenshinCache(t, dataDir, "2.0.0", "junk\x00"+newURL+"\x00junk")
	oldTime := time.Now().Add(-time.Hour)
	if err := os.Chtimes(oldCache, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(newCache, time.Now(), time.Now()); err != nil {
		t.Fatal(err)
	}

	logPath := filepath.Join(dir, "output_log.txt")
	logText := "Loading player data from " + filepath.ToSlash(dataDir) +
		"/data.unity3d\r\nTelemetryInterface path:" + filepath.ToSlash(dataDir) + "/SDKCaches\r\n"
	if err := os.WriteFile(logPath, []byte(logText), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := findGenshinLogURL([]genshinLogFile{{path: logPath}})
	if err != nil {
		t.Fatal(err)
	}
	if got != newURL {
		t.Errorf("应取最新缓存里的链接，得到 %q", got)
	}

	// 最新缓存里没有完整链接时回退到旧缓存。
	if err := os.WriteFile(newCache, []byte("https://example.com/"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err = findGenshinLogURL([]genshinLogFile{{path: logPath}})
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
	if _, err := findGenshinLogURL([]genshinLogFile{{path: logPath}}); err == nil {
		t.Error("没有游戏路径时没有报错")
	}
	if _, err := findGenshinLogURL([]genshinLogFile{{path: filepath.Join(dir, "missing.log")}}); err == nil {
		t.Error("没有日志时没有报错")
	}

	// 云游戏日志里直接带链接。
	cloudPath := filepath.Join(dir, "MiHoYoSDK.log")
	if err := os.WriteFile(cloudPath, []byte("junk\x00"+oldURL+"\x00junk"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err = findGenshinLogURL([]genshinLogFile{{path: cloudPath, direct: true}})
	if err != nil {
		t.Fatal(err)
	}
	if got != oldURL {
		t.Errorf("云游戏日志里的链接 %q，期望 %q", got, oldURL)
	}
}

func TestParseGenshinLogURL(t *testing.T) {
	raw := "https://webstatic.mihoyo.com/hk4e/event/e20190909gacha/index.html?authkey=abc+def=ghi&game_biz=hk4e_cn&lang=zh-cn&page=2&size=20&gacha_type=301&end_id=123"
	api, err := parseGenshinLogURL(raw)
	if err != nil {
		t.Fatal(err)
	}
	if api.domain != genshinAPICN || api.lang != "zh-cn" {
		t.Errorf("域名 %q，语言 %q", api.domain, api.lang)
	}
	q, err := url.ParseQuery(api.query)
	if err != nil {
		t.Fatal(err)
	}
	if q.Get("authkey") != "abc+def=ghi" {
		t.Errorf("authkey 变成 %q", q.Get("authkey"))
	}
	for _, key := range []string{"page", "size", "gacha_type", "end_id"} {
		if q.Has(key) {
			t.Errorf("分页参数 %q 应被移除", key)
		}
	}

	osAPI, err := parseGenshinLogURL("https://webstatic-sea.hoyoverse.com/hk4e/event/index.html?authkey=OS&lang=en-us")
	if err != nil {
		t.Fatal(err)
	}
	if osAPI.domain != genshinAPIOS || osAPI.lang != "en-us" {
		t.Errorf("国际服域名 %q，语言 %q", osAPI.domain, osAPI.lang)
	}

	if _, err := parseGenshinLogURL("https://webstatic.mihoyo.com/hk4e/event/index.html"); err == nil {
		t.Error("缺少 authkey 时没有报错")
	}
}

func TestFetchGenshinAll(t *testing.T) {
	dir := t.TempDir()
	local := &gachaExport{
		UID:            "42",
		Lang:           "zh-cn",
		RegionTimeZone: 8,
		TypeMap:        [][2]string{{"301", "角色活动祈愿"}, {"1000", "千星奇域·常驻颂愿"}},
		Result: []gachaGroup{
			{Type: "301", Records: []gachaRecord{
				{ID: "999", Name: "旧甲", RankType: "3", Time: "2024-01-01 10:00:00", GachaType: "301", Count: "1"},
				{ID: "1001", Name: "旧乙", RankType: "4", Time: "2024-01-02 10:00:00", GachaType: "301", Count: "1"},
			}},
			// 本地已有的千星奇域频段不参与抓取，但合并后必须保留。
			{Type: "1000", Records: []gachaRecord{
				{ID: "777", Name: "奇域", RankType: "5", Time: "2024-01-01 09:00:00", GachaType: "1000", Count: "1"},
			}},
		},
	}
	if err := saveGachaExport(dir, local); err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("authkey") != "test" {
			t.Errorf("authkey = %q", q.Get("authkey"))
		}
		if r.URL.Path != genshinGachaPath {
			t.Errorf("请求路径 %q", r.URL.Path)
		}
		typ, page, size, endID := q.Get("gacha_type"), q.Get("page"), q.Get("size"), q.Get("end_id")
		mu.Lock()
		requests = append(requests, fmt.Sprintf("%s/%s/%s/%s", typ, page, size, endID))
		mu.Unlock()

		var list []apiGachaRecord
		if typ == "301" {
			switch {
			case size == "6": // 确认 UID
				list = []apiGachaRecord{{UID: "42", gachaRecord: gachaRecord{ID: "1003"}}}
			case page == "1" && endID == "0":
				list = []apiGachaRecord{
					{UID: "42", gachaRecord: gachaRecord{ID: "1003", Name: "新丙", RankType: "5", Time: "2024-01-03 10:00:00", GachaType: "301", Count: "1"}},
					{UID: "42", gachaRecord: gachaRecord{ID: "1002", Name: "新丁", RankType: "3", Time: "2024-01-01 12:00:00", GachaType: "301", Count: "1"}},
				}
			case page == "2" && endID == "1002":
				list = []apiGachaRecord{
					{UID: "42", gachaRecord: gachaRecord{ID: "1001", Name: "旧乙", RankType: "4", Time: "2024-01-02 10:00:00", GachaType: "301", Count: "1"}},
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

	oldClient, oldSleep := genshinHTTPClient, genshinSleep
	genshinHTTPClient, genshinSleep = server.Client(), func(time.Duration) {}
	defer func() { genshinHTTPClient, genshinSleep = oldClient, oldSleep }()

	api := &genshinAPIURL{domain: server.URL, query: "authkey=test&lang=zh-cn&game_biz=hk4e_cn", lang: "zh-cn"}
	res, err := fetchGenshinAll(dir, api, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.UID != "42" || res.Added != 2 || res.Total != 5 {
		t.Errorf("结果 %+v", res)
	}

	d, err := loadGachaDir(dir, "genshin")
	if err != nil {
		t.Fatal(err)
	}
	if d.TotalPulls != 5 || d.TotalS != 2 || d.TotalA != 1 {
		t.Errorf("统计 %+v", d)
	}
	if len(d.Channels) != 2 || d.Channels[0].Type != "301" || d.Channels[0].Pulls != 4 ||
		d.Channels[1].Type != "1000" || d.Channels[1].Pulls != 1 {
		t.Fatalf("祈愿 %+v", d.Channels)
	}
	// 记录页按时间从新到旧：1003、1001 的时间晚于 1002、999。
	want := []string{"1003", "1001", "1002", "999"}
	for i, id := range want {
		if d.Channels[0].Records[i].ID != id {
			t.Errorf("第 %d 条 id %q，期望 %q", i, d.Channels[0].Records[i].ID, id)
		}
	}

	saved, err := loadGachaLocal(dir, "42")
	if err != nil {
		t.Fatal(err)
	}
	wantTypes := append(append([]string{}, genshinChannelOrder...), "1000")
	if len(saved.Result) != len(wantTypes) {
		t.Fatalf("保存的祈愿分组 %+v", saved.Result)
	}
	for i, typ := range wantTypes {
		if saved.Result[i].Type != typ {
			t.Errorf("第 %d 组 %q，期望 %q", i, saved.Result[i].Type, typ)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	sawOther := false
	for _, req := range requests {
		if strings.HasPrefix(req, "301/3/") {
			t.Errorf("不应请求第 3 页：%q", req)
		}
		if strings.HasPrefix(req, "302/") {
			sawOther = true
		}
	}
	if !sawOther {
		t.Error("没有请求其他祈愿频段")
	}
}

func TestGenshinRegionZone(t *testing.T) {
	for uid, want := range map[string]int{
		"174095639": 8,  // 国服
		"612345678": -5, // 美服
		"712345678": 1,  // 欧服
		"812345678": 8,  // 亚服
	} {
		if got := genshinRegionZone(uid); got != want {
			t.Errorf("%s 的时区 %d，期望 %d", uid, got, want)
		}
	}
}

func TestFetchGenshinAuthkeyExpired(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"retcode": -100, "message": "authkey timeout"})
	}))
	defer server.Close()

	oldClient, oldSleep := genshinHTTPClient, genshinSleep
	genshinHTTPClient, genshinSleep = server.Client(), func(time.Duration) {}
	defer func() { genshinHTTPClient, genshinSleep = oldClient, oldSleep }()

	api := &genshinAPIURL{domain: server.URL, query: "authkey=test", lang: "zh-cn"}
	_, err := fetchGenshinAll(t.TempDir(), api, nil)
	if !errors.Is(err, errGenshinAuthkey) {
		t.Fatalf("错误 = %v，期望 authkey 失效", err)
	}
}

func TestGenshinRefreshOverlay(t *testing.T) {
	a := newApp()
	a.selected = "genshin"
	a.genshin = testGenshinData(t)
	a.genshinLoaded = true
	a.genshinFetching = true
	a.genshinStatus = "正在读取角色活动祈愿（第 2 页）…"
	tt := ui.NewTester(a.view, 960, 680)
	for _, text := range []string{"正在刷新抽卡记录", "正在读取角色活动祈愿（第 2 页）…"} {
		if !tt.HasText(text) {
			t.Errorf("缺少文本 %q，现有 %q", text, tt.Texts())
		}
	}
}

func TestGenshinRefreshButton(t *testing.T) {
	a := newApp()
	a.selected = "genshin"
	a.genshin = testGenshinData(t)
	a.genshinLoaded = true
	a.genshinStatus = "已是最新，共 7 条记录"
	tt := ui.NewTester(a.view, 960, 680)
	if !tt.HasText("刷新") {
		t.Errorf("缺少刷新按钮，现有 %q", tt.Texts())
	}
	if !tt.HasText("已是最新，共 7 条记录") {
		t.Errorf("缺少刷新状态，现有 %q", tt.Texts())
	}
}
