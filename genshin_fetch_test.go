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
			// 常驻颂愿最高只有 4 星，因此 4 在这里是 S 级。
			{Type: "1000", Records: []gachaRecord{
				{ID: "777", Name: "奇域", RankType: "4", Time: "2024-01-01 09:00:00", GachaType: "1000", Count: "1"},
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
		typ, page, size, endID := q.Get("gacha_type"), q.Get("page"), q.Get("size"), q.Get("end_id")
		mu.Lock()
		requests = append(requests, fmt.Sprintf("%s/%s/%s/%s/%s", r.URL.Path, typ, page, size, endID))
		mu.Unlock()

		data := map[string]any{"list": []any{}, "region": "prod_gf_cn", "region_time_zone": 8}
		switch r.URL.Path {
		case genshinGachaPath:
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
					t.Errorf("意外的祈愿分页请求 typ=%s page=%s end=%s", typ, page, endID)
				}
			}
			data["list"] = list
		case genshinBeyondGachaPath:
			data = map[string]any{"list": []any{}}
			var list []beyondGachaRecord
			switch typ {
			case "2000":
				switch {
				case page == "1" && endID == "0":
					list = []beyondGachaRecord{
						{ID: "20002", ItemName: "活动S", ItemType: "装扮", RankType: "5", Time: "2024-01-04 11:00:00", OpGachaType: "20021"},
						{ID: "20001", ItemName: "活动A", ItemType: "装扮", RankType: "4", Time: "2024-01-04 10:00:00", OpGachaType: "20021"},
					}
				case page == "2" && endID == "20001":
					list = []beyondGachaRecord{
						{ID: "20000", ItemName: "活动B", ItemType: "装扮", RankType: "3", Time: "2024-01-03 10:00:00", OpGachaType: "20011"},
					}
				case page == "3" && endID == "20000":
					// 最后一页为空，结束翻页。
				default:
					t.Errorf("意外的活动颂愿分页请求 page=%s end=%s", page, endID)
				}
			case "1000":
				switch {
				case page == "1" && endID == "0":
					// 第二页会包含本地最新的 777，抓到它后应停止翻页。
					list = []beyondGachaRecord{
						{ID: "10001", ItemName: "常驻S", ItemType: "装扮", RankType: "4", Time: "2024-01-05 10:00:00"},
						{ID: "777", ItemName: "奇域", ItemType: "装扮", RankType: "4", Time: "2024-01-01 09:00:00"},
					}
				default:
					t.Errorf("意外的常驻颂愿分页请求 page=%s end=%s", page, endID)
				}
			default:
				t.Errorf("意外的颂愿频段 %q", typ)
			}
			data["list"] = list
		default:
			t.Errorf("请求路径 %q", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"retcode": 0,
			"message": "OK",
			"data":    data,
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
	if res.UID != "42" || res.Added != 6 || res.Total != 9 {
		t.Errorf("结果 %+v", res)
	}

	d, err := loadGachaDir(dir, "genshin")
	if err != nil {
		t.Fatal(err)
	}
	if d.TotalPulls != 9 || d.TotalS != 4 || d.TotalA != 2 {
		t.Errorf("统计 %+v", d)
	}
	if len(d.Channels) != 3 {
		t.Fatalf("祈愿 %+v", d.Channels)
	}
	if d.Channels[0].Type != "301" || d.Channels[0].Pulls != 4 || d.Channels[0].HardPity != 90 ||
		d.Channels[1].Type != "2000" || d.Channels[1].Pulls != 3 || d.Channels[1].HardPity != 70 ||
		d.Channels[2].Type != "1000" || d.Channels[2].Pulls != 2 || d.Channels[2].HardPity != 70 {
		t.Fatalf("祈愿 %+v", d.Channels)
	}
	// 记录页按时间从新到旧：1003、1001 的时间晚于 1002、999。
	want := []string{"1003", "1001", "1002", "999"}
	for i, id := range want {
		if d.Channels[0].Records[i].ID != id {
			t.Errorf("第 %d 条 id %q，期望 %q", i, d.Channels[0].Records[i].ID, id)
		}
	}
	// 活动颂愿按 5/4/3 星映射 S/A/B；常驻颂愿最高 4 星，4 星就是 S。
	if got := d.Channels[1]; got.S != 1 || got.A != 1 || got.B != 1 || got.Records[0].Tier != 0 ||
		got.Records[1].Tier != 1 || got.Records[2].Tier != 2 {
		t.Errorf("活动颂愿 %+v", got)
	}
	if got := d.Channels[2]; got.S != 2 || got.A != 0 || got.B != 0 {
		t.Errorf("常驻颂愿 %+v", got)
	}
	// 接口返回的 op_gacha_type 保留在记录里，便于区分活动颂愿子池。
	if got := d.Channels[1].Records[0]; got.GachaType != "20021" || got.Name != "活动S" {
		t.Errorf("活动颂愿记录 %+v", got)
	}

	saved, err := loadGachaLocal(dir, "42")
	if err != nil {
		t.Fatal(err)
	}
	wantTypes := append([]string{}, genshinChannelOrder...)
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
	sawBeyondPage := false
	for _, req := range requests {
		if strings.HasPrefix(req, genshinGachaPath+"/301/3/") {
			t.Errorf("不应请求第 3 页：%q", req)
		}
		if strings.HasPrefix(req, genshinGachaPath+"/302/") {
			sawOther = true
		}
		if strings.HasPrefix(req, genshinBeyondGachaPath+"/2000/2/20/20001") {
			sawBeyondPage = true
		}
		if strings.HasPrefix(req, genshinBeyondGachaPath+"/1000/2/") {
			t.Errorf("命中本地最新 id 后不应继续翻页：%q", req)
		}
		if strings.HasPrefix(req, genshinBeyondGachaPath) && strings.Contains(req, "/6/") {
			t.Errorf("颂愿频段不应参与 UID 探测：%q", req)
		}
	}
	if !sawOther {
		t.Error("没有请求其他祈愿频段")
	}
	if !sawBeyondPage {
		t.Error("没有请求活动颂愿第 2 页")
	}
}

func TestFetchGenshinOnlyBeyond(t *testing.T) {
	dir := t.TempDir()
	var mu sync.Mutex
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("authkey") != "test" {
			t.Errorf("authkey = %q", q.Get("authkey"))
		}
		typ, page, size, endID := q.Get("gacha_type"), q.Get("page"), q.Get("size"), q.Get("end_id")
		lang := q.Get("lang")
		mu.Lock()
		requests = append(requests, fmt.Sprintf("%s/%s/%s/%s/%s/%s", r.URL.Path, typ, page, size, endID, lang))
		mu.Unlock()

		list := []any{}
		switch r.URL.Path {
		case genshinGachaPath:
			// 只玩颂愿的账号，普通祈愿接口全部为空。
			if lang != "en" {
				t.Errorf("普通祈愿 lang = %q，期望 en", lang)
			}
		case genshinBeyondGachaPath:
			if lang != "en-us" {
				t.Errorf("颂愿 lang = %q，期望 en-us", lang)
			}
			switch typ {
			case "2000":
				switch {
				case size == "6":
					list = []any{beyondGachaRecord{UID: "612345678", ID: "30002"}}
				case page == "1" && endID == "0":
					list = []any{beyondGachaRecord{
						UID: "612345678", ID: "30002", ItemName: "装扮S", ItemType: "装扮",
						RankType: "5", Time: "2024-01-02 10:00:00", OpGachaType: "20021",
					}}
				case page == "2" && endID == "30002":
					// 最后一页为空。
				default:
					t.Errorf("意外的活动颂愿分页请求 page=%s end=%s", page, endID)
				}
			case "1000":
				if page != "1" || endID != "0" {
					t.Errorf("意外的常驻颂愿分页请求 page=%s end=%s", page, endID)
				}
			default:
				t.Errorf("意外的颂愿频段 %q", typ)
			}
		default:
			t.Errorf("请求路径 %q", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"retcode": 0,
			"message": "OK",
			"data":    map[string]any{"list": list},
		})
	}))
	defer server.Close()

	oldClient, oldSleep := genshinHTTPClient, genshinSleep
	genshinHTTPClient, genshinSleep = server.Client(), func(time.Duration) {}
	defer func() { genshinHTTPClient, genshinSleep = oldClient, oldSleep }()

	api := &genshinAPIURL{domain: server.URL, query: "authkey=test&lang=en&game_biz=hk4e_os", lang: "en"}
	res, err := fetchGenshinAll(dir, api, nil)
	if err != nil {
		t.Fatal(err)
	}
	// 普通祈愿没有记录时，UID 应从颂愿接口取得；美服 UID 前缀 6。
	if res.UID != "612345678" || res.Added != 1 || res.Total != 1 {
		t.Errorf("结果 %+v", res)
	}
	d, err := loadGachaDir(dir, "genshin")
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Channels) != 1 || d.Channels[0].Type != "2000" || d.Channels[0].S != 1 ||
		d.Channels[0].HardPity != 70 || d.Channels[0].Records[0].Time != "2024-01-02 10:00:00" {
		t.Errorf("颂愿 %+v", d.Channels)
	}

	mu.Lock()
	defer mu.Unlock()
	sawBeyondProbe := false
	sawBeyondPage := false
	for _, req := range requests {
		if strings.HasPrefix(req, genshinBeyondGachaPath+"/2000/1/6/0/en-us") {
			sawBeyondProbe = true
		}
		if strings.HasPrefix(req, genshinBeyondGachaPath+"/2000/1/20/0/en-us") {
			sawBeyondPage = true
		}
	}
	if !sawBeyondProbe || !sawBeyondPage {
		t.Errorf("颂愿请求 %+v", requests)
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
	if tt.HasText("已是最新，共 7 条记录") {
		t.Errorf("刷新结果不应显示在页面顶部，现有 %q", tt.Texts())
	}

	a.genshinStatus = ""
	a.genshinToast = "已是最新，共 7 条记录"
	tt.Frame()
	if !tt.HasText("已是最新，共 7 条记录") {
		t.Errorf("缺少刷新结果 toast，现有 %q", tt.Texts())
	}
	if a.genshinToast != "" {
		t.Error("toast 应只显示一次")
	}
}
