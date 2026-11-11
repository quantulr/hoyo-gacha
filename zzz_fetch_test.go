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
)

func TestFindZZZLogURL(t *testing.T) {
	dir := t.TempDir()
	older := filepath.Join(dir, "older.log")
	newer := filepath.Join(dir, "newer.log")
	if err := os.WriteFile(older, []byte(`OnGetWebViewPageFinish:https://webstatic.mihoyo.com/zzz/index.html?authkey=OLD&lang=zh-cn#/info`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newer, []byte(`https://example.com/info
https://webstatic.mihoyo.com/zzz/index.html?authkey=STALE&lang=zh-cn#/info
https://webstatic.mihoyo.com/zzz/index.html?authkey=NEW&lang=zh-cn#/info`), 0o644); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().Add(-time.Hour)
	if err := os.Chtimes(older, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	got, err := findZZZLogURL([]string{older, newer})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "authkey=NEW") {
		t.Errorf("应取最新日志里最后一条链接，得到 %q", got)
	}

	// 更新的日志里没有 authkey 时，回退到旧日志。
	if err := os.WriteFile(newer, []byte(`https://example.com/info`), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err = findZZZLogURL([]string{older, newer})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "authkey=OLD") {
		t.Errorf("应回退到含 authkey 的日志，得到 %q", got)
	}

	if _, err := findZZZLogURL([]string{filepath.Join(dir, "missing.log")}); err == nil {
		t.Error("没有日志时没有报错")
	}
}

func TestParseZZZLogURL(t *testing.T) {
	raw := "https://webstatic.mihoyo.com/zzz/event/index.html?authkey=abc%2B%3D&game_biz=nap_cn&lang=zh-cn&region=prod_gf_cn&page=2&size=20&real_gacha_type=2&end_id=123&begin_id=456#/info"
	api, err := parseZZZLogURL(raw)
	if err != nil {
		t.Fatal(err)
	}
	if api.domain != zzzAPICN || api.lang != "zh-cn" {
		t.Errorf("域名 %q，语言 %q", api.domain, api.lang)
	}
	q, err := url.ParseQuery(api.query)
	if err != nil {
		t.Fatal(err)
	}
	if q.Get("authkey") != "abc+=" {
		t.Errorf("authkey 变成 %q", q.Get("authkey"))
	}
	for _, key := range []string{"page", "size", "real_gacha_type", "end_id", "begin_id"} {
		if q.Has(key) {
			t.Errorf("分页参数 %q 应被移除", key)
		}
	}

	osAPI, err := parseZZZLogURL("https://webstatic-sea.hoyoverse.com/zzz/index.html?authkey=OS&lang=en-us#/info")
	if err != nil {
		t.Fatal(err)
	}
	if osAPI.domain != zzzAPIOS {
		t.Errorf("国际服域名 %q", osAPI.domain)
	}

	if _, err := parseZZZLogURL("https://webstatic.mihoyo.com/zzz/index.html#/info"); err == nil {
		t.Error("缺少 authkey 时没有报错")
	}
}

func TestAPIURLSetLang(t *testing.T) {
	api := &zzzAPIURL{query: "authkey=test&lang=en-us", lang: "en-us"}
	api.setLang("zh-cn")
	q, err := url.ParseQuery(api.query)
	if err != nil {
		t.Fatal(err)
	}
	if api.lang != "zh-cn" || q.Get("lang") != "zh-cn" {
		t.Errorf("语言 %q，参数 %q", api.lang, q.Get("lang"))
	}
	if q.Get("authkey") != "test" {
		t.Errorf("authkey 丢失：%q", api.query)
	}
}

func TestMergeZZZExport(t *testing.T) {
	local := &gachaExport{
		UID:            "1",
		Lang:           "zh-cn",
		RegionTimeZone: 8,
		TypeMap:        [][2]string{{"2", "独家频段"}},
		Result: []gachaGroup{{Type: "2", Records: []gachaRecord{
			{ID: "9", Name: "旧九", Count: "1"},
			{ID: "10", Name: "旧十", Count: "1"},
		}}},
	}
	fetched := &gachaExport{
		UID:            "1",
		Lang:           "zh-cn",
		RegionTimeZone: 8,
		TypeMap:        [][2]string{{"2", "独家频段"}, {"1", "常驻频段"}},
		Result: []gachaGroup{
			{Type: "2", Records: []gachaRecord{{ID: "10", Name: "新十"}, {ID: "11", Name: "新十一"}}},
			{Type: "1", Records: nil},
		},
	}
	merged, added := mergeZZZExport(local, fetched)
	if added != 1 { // 只有 11 是新的
		t.Errorf("新增 %d，期望 1", added)
	}
	if merged.UID != "1" || merged.RegionTimeZone != 8 {
		t.Errorf("合并结果 %+v", merged)
	}
	if len(merged.Result) != 6 || merged.Result[0].Type != "2" || merged.Result[4].Type != "1" {
		t.Fatalf("频段顺序 %+v", merged.Result)
	}
	got := merged.Result[0].Records
	if len(got) != 3 || got[0].ID != "9" || got[1].ID != "10" || got[2].ID != "11" {
		t.Fatalf("记录顺序 %+v", got)
	}
	if got[1].Name != "新十" {
		t.Errorf("重复 id 应保留新数据，得到 %q", got[1].Name)
	}
	if got[2].Count != "1" {
		t.Errorf("缺失的 count 应补 1，得到 %q", got[2].Count)
	}
}

func TestSaveZZZExport(t *testing.T) {
	root := t.TempDir()
	writeTestConfig(t, root, `[games.zzz]
current = "1"
proxyPort = 8325
`)
	exp := &gachaExport{
		UID:            "42",
		Lang:           "zh-cn",
		RegionTimeZone: 8,
		TypeMap:        [][2]string{{"2", "独家频段"}},
		Result: []gachaGroup{{Type: "2", Records: []gachaRecord{
			{ID: "1", Name: "甲", RankType: "2", Time: "2024-01-01 10:00:00", GachaType: "2", Count: "1"},
		}}},
	}
	if err := saveZZZExport(root, exp); err != nil {
		t.Fatal(err)
	}
	d, err := loadZZZGame(root)
	if err != nil {
		t.Fatal(err)
	}
	if d.UID != "42" || d.TotalPulls != 1 {
		t.Errorf("读回 %+v", d)
	}
	cfg := readTestConfig(t, root)
	if got := configCurrent(cfg, "zzz"); got != "42" {
		t.Errorf("current = %q", got)
	}
	games, _ := cfg["games"].(map[string]any)
	zzz, _ := games["zzz"].(map[string]any)
	if zzz["proxyPort"] != int64(8325) {
		t.Errorf("其他配置被丢弃：%#v", zzz)
	}
}

func TestFetchZZZAll(t *testing.T) {
	root := t.TempDir()
	local := &gachaExport{
		UID:            "42",
		Lang:           "zh-cn",
		RegionTimeZone: 8,
		TypeMap:        [][2]string{{"2", "独家频段"}},
		Result: []gachaGroup{{Type: "2", Records: []gachaRecord{
			{ID: "999", Name: "旧甲", RankType: "2", Time: "2024-01-01 10:00:00", GachaType: "2", Count: "1"},
			{ID: "1001", Name: "旧乙", RankType: "3", Time: "2024-01-02 10:00:00", GachaType: "2", Count: "1"},
		}}},
	}
	if err := saveZZZExport(root, local); err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != zzzAPIPath {
			t.Errorf("请求路径 %q", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("authkey") != "test" {
			t.Errorf("authkey = %q", q.Get("authkey"))
		}
		typ, page, size, endID := q.Get("real_gacha_type"), q.Get("page"), q.Get("size"), q.Get("end_id")
		mu.Lock()
		requests = append(requests, fmt.Sprintf("%s/%s/%s/%s", typ, page, size, endID))
		mu.Unlock()

		var list []apiGachaRecord
		if typ == "2" {
			switch {
			case size == "5": // 预检
				list = []apiGachaRecord{{UID: "42", gachaRecord: gachaRecord{ID: "1003"}}}
			case page == "1" && endID == "0":
				list = []apiGachaRecord{
					{UID: "42", gachaRecord: gachaRecord{ID: "1003", Name: "新丙", RankType: "4", Time: "2024-01-03 10:00:00", GachaType: "2", Count: "1"}},
					{UID: "42", gachaRecord: gachaRecord{ID: "1002", Name: "新丁", RankType: "2", Time: "2024-01-01 12:00:00", GachaType: "2", Count: "1"}},
				}
			case page == "2" && endID == "1002":
				list = []apiGachaRecord{
					{UID: "42", gachaRecord: gachaRecord{ID: "1001", Name: "旧乙", RankType: "3", Time: "2024-01-02 10:00:00", GachaType: "2", Count: "1"}},
				}
			default:
				t.Errorf("意外的分页请求 typ=%s page=%s end=%s", typ, page, endID)
			}
		}
		json.NewEncoder(w).Encode(map[string]any{
			"retcode": 0,
			"message": "OK",
			"data":    map[string]any{"list": list, "region": "prod_gf_cn"},
		})
	}))
	defer server.Close()

	oldClient, oldSleep := zzzHTTPClient, zzzSleep
	zzzHTTPClient, zzzSleep = server.Client(), func(time.Duration) {}
	defer func() { zzzHTTPClient, zzzSleep = oldClient, oldSleep }()

	api := &zzzAPIURL{domain: server.URL, query: "authkey=test&lang=zh-cn&game_biz=nap_cn", lang: "zh-cn"}
	res, err := fetchZZZAll(root, api, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.UID != "42" || res.Added != 2 || res.Total != 4 {
		t.Errorf("结果 %+v", res)
	}

	d, err := loadZZZGame(root)
	if err != nil {
		t.Fatal(err)
	}
	if d.TotalPulls != 4 || d.TotalS != 1 || d.TotalA != 1 {
		t.Errorf("统计 %+v", d)
	}
	if len(d.Channels) != 1 || d.Channels[0].Pulls != 4 {
		t.Fatalf("频段 %+v", d.Channels)
	}
	// 展示时按时间最新在前。
	want := []string{"1003", "1001", "1002", "999"}
	for i, id := range want {
		if d.Channels[0].Records[i].ID != id {
			t.Errorf("第 %d 条 id %q，期望 %q", i, d.Channels[0].Records[i].ID, id)
		}
	}

	// 命中本地最新 id 后不应继续翻页。
	mu.Lock()
	defer mu.Unlock()
	for _, req := range requests {
		if strings.HasPrefix(req, "2/3/") {
			t.Errorf("不应请求第 3 页：%q", req)
		}
	}
}

func TestFetchZZZAuthkeyExpired(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"retcode": -100, "message": "authkey timeout"})
	}))
	defer server.Close()

	oldClient, oldSleep := zzzHTTPClient, zzzSleep
	zzzHTTPClient, zzzSleep = server.Client(), func(time.Duration) {}
	defer func() { zzzHTTPClient, zzzSleep = oldClient, oldSleep }()

	api := &zzzAPIURL{domain: server.URL, query: "authkey=test", lang: "zh-cn"}
	_, err := fetchZZZAll(t.TempDir(), api, nil)
	if !errors.Is(err, errZZZAuthkey) {
		t.Fatalf("错误 = %v，期望 authkey 失效", err)
	}
}
