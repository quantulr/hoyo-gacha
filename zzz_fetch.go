package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/egoist/mygo"
)

// 绝区零调频记录接口：国服与国际服域名不同，路径和参数一致。
const (
	zzzAPICN        = "https://public-operation-nap.mihoyo.com"
	zzzAPIOS        = "https://public-operation-nap-sg.hoyoverse.com"
	zzzAPIPath      = "/common/gacha_record/api/getGachaLog"
	zzzPageSize     = 20
	zzzFetchTries   = 5
	zzzRetryWait    = 5 * time.Second
	zzzPageWait     = 300 * time.Millisecond
	zzzIntervalWait = time.Second
)

// zzzChannelOrder 是抓取和展示频段的固定顺序。
var zzzChannelOrder = []string{"2", "3", "102", "103", "1", "5"}

// zzzChannelNames 是各频段在简体中文下的名称。
var zzzChannelNames = map[string]string{
	"2":   "独家频段",
	"3":   "音擎频段",
	"102": "独家重映",
	"103": "音擎回响",
	"1":   "常驻频段",
	"5":   "邦布频段",
}

// zzzServerZones 是各服务器记录的本地时间与 UTC 的时差。
var zzzServerZones = map[string]int{
	"prod_gf_cn": 8,
	"prod_gf_jp": 8,
	"prod_gf_us": -5,
	"prod_gf_eu": 1,
	"prod_gf_sg": 8,
}

// 测试里替换掉真实的网络和等待。
var (
	zzzHTTPClient = &http.Client{Timeout: 15 * time.Second}
	zzzSleep      = time.Sleep
)

var errZZZAuthkey = errors.New("authkey 已失效，请先在游戏内打开「调频记录」页，再点刷新")

// 日志里的调频记录页链接，以 #/info 结尾。
var zzzInfoRE = regexp.MustCompile(`https://[^\s"]+/info`)

// zzzAPIURL 是从日志链接解析出的请求信息。
type zzzAPIURL struct {
	domain string // 接口域名，国服或国际服
	query  string // 已编码的公共参数，不含分页参数
	lang   string
}

// refreshZZZData 从游戏日志取得临时链接，抓取全部频段并合并写回应用数据目录。
func refreshZZZData(progress func(string)) (*gachaRefreshResult, error) {
	root, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		return nil, err
	}
	return refreshZZZDir(filepath.Join(root, "data", "zzz"), progress)
}

func refreshZZZDir(dir string, progress func(string)) (*gachaRefreshResult, error) {
	raw, err := findZZZLogURL(zzzLogPaths())
	if err != nil {
		return nil, err
	}
	api, err := parseZZZLogURL(raw)
	if err != nil {
		return nil, err
	}
	return fetchZZZAll(dir, api, progress)
}

// zzzLogPaths 返回国服与国际服客户端可能写入的玩家日志路径。
func zzzLogPaths() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	root := filepath.Join(home, "AppData", "LocalLow", "miHoYo")
	return []string{
		filepath.Join(root, "绝区零", "Player.log"),
		filepath.Join(root, "ZenlessZoneZero", "Player.log"),
	}
}

// findZZZLogURL 在玩家日志里找最后一条调频记录页链接。两份日志都存在时
// 优先修改时间更新的那份；没有 authkey 的 /info 链接会被跳过。
func findZZZLogURL(paths []string) (string, error) {
	var bestURL string
	var bestMod time.Time
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if bestURL != "" && !info.ModTime().After(bestMod) {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		matches := zzzInfoRE.FindAllString(string(data), -1)
		for i := len(matches) - 1; i >= 0; i-- {
			if strings.Contains(matches[i], "authkey=") {
				bestURL, bestMod = matches[i], info.ModTime()
				break
			}
		}
	}
	if bestURL == "" {
		return "", errors.New("没有在游戏日志里找到调频记录链接，请先在游戏内打开「调频记录」页再刷新")
	}
	return bestURL, nil
}

// parseZZZLogURL 从日志链接里取出 authkey 等公共参数，并选定接口域名。
func parseZZZLogURL(raw string) (*zzzAPIURL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, fmt.Errorf("无法解析调频记录链接：%w", sanitizeURLError(err))
	}
	q, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return nil, fmt.Errorf("调频记录链接的参数无效：%w", err)
	}
	if q.Get("authkey") == "" {
		return nil, errors.New("调频记录链接里缺少 authkey，请重新打开游戏内的「调频记录」页")
	}
	for _, key := range []string{"page", "size", "real_gacha_type", "end_id"} {
		q.Del(key)
	}
	domain := zzzAPICN
	host := strings.ToLower(u.Host)
	if strings.Contains(host, "hoyoverse.com") || strings.Contains(host, "webstatic-sea") {
		domain = zzzAPIOS
	}
	lang := q.Get("lang")
	if lang == "" {
		lang = "zh-cn"
	}
	return &zzzAPIURL{domain: domain, query: q.Encode(), lang: lang}, nil
}

// setLang 把请求语言换成账号已有数据的语言，避免新旧记录语言混杂。
func (api *zzzAPIURL) setLang(lang string) {
	q, err := url.ParseQuery(api.query)
	if err != nil {
		return
	}
	q.Set("lang", lang)
	api.lang = lang
	api.query = q.Encode()
}

// zzzFetchUID 先抽一页，确定账号 UID 与服务器区域。
func zzzFetchUID(api *zzzAPIURL) (string, string, error) {
	var region string
	for _, typ := range zzzChannelOrder {
		data, err := zzzRequestRetry(api, typ, 1, "0", 5)
		if err != nil {
			return "", region, err
		}
		if region == "" {
			region = data.Region
		}
		if len(data.List) > 0 && data.List[0].UID != "" {
			return data.List[0].UID, data.Region, nil
		}
	}
	return "", region, nil
}

// fetchZZZAll 抓取全部频段，与本地数据合并后写回 dir。
func fetchZZZAll(dir string, api *zzzAPIURL, progress func(string)) (*gachaRefreshResult, error) {
	if progress == nil {
		progress = func(string) {}
	}
	progress("正在确认游戏账号…")
	uid, region, err := zzzFetchUID(api)
	if err != nil {
		return nil, err
	}
	if uid == "" {
		return nil, errors.New("没有获取到抽卡记录，请先在游戏内打开「调频记录」页再刷新")
	}
	serverTZ, ok := zzzServerZones[region]
	if !ok {
		return nil, fmt.Errorf("不支持此服务器：%s", region)
	}
	local, err := loadGachaLocal(dir, uid)
	if err != nil {
		return nil, err
	}
	if local != nil && local.Lang != "" && local.Lang != api.lang {
		api.setLang(local.Lang)
	}
	targetTZ := serverTZ
	if local != nil && local.RegionTimeZone != 0 {
		targetTZ = local.RegionTimeZone
	}
	latest := gachaLatestIDs(local)

	fetched := &gachaExport{UID: uid, Lang: api.lang, RegionTimeZone: targetTZ}
	for _, typ := range zzzChannelOrder {
		name := zzzChannelNames[typ]
		recs, err := zzzFetchChannel(api, typ, name, latest[typ], progress)
		if err != nil {
			return nil, err
		}
		for i := range recs {
			recs[i].Time = convertGachaTime(recs[i].Time, serverTZ, targetTZ)
		}
		fetched.Result = append(fetched.Result, gachaGroup{Type: typ, Records: recs})
		fetched.TypeMap = append(fetched.TypeMap, [2]string{typ, name})
	}

	merged, added := mergeZZZExport(local, fetched)
	merged.Time = time.Now().UnixMilli()
	if err := saveZZZExport(dir, merged); err != nil {
		return nil, err
	}
	total := 0
	for _, group := range merged.Result {
		total += len(group.Records)
	}
	return &gachaRefreshResult{UID: uid, Added: added, Total: total}, nil
}

// zzzFetchChannel 按 end_id 翻页抓取一个频段。latest 是本地最新的记录 id，
// 碰到它就说明后面的记录本地都有了。
func zzzFetchChannel(api *zzzAPIURL, typ, name string, latest *big.Int, progress func(string)) ([]gachaRecord, error) {
	var records []gachaRecord
	page := 1
	endID := "0"
	for {
		progress(fmt.Sprintf("正在读取 %s（第 %d 页）…", name, page))
		data, err := zzzRequestRetry(api, typ, page, endID, zzzPageSize)
		if err != nil {
			return nil, err
		}
		if len(data.List) == 0 {
			break
		}
		hitLatest := false
		for _, item := range data.List {
			rec := item.gachaRecord
			if rec.Count == "" {
				rec.Count = "1"
			}
			records = append(records, rec)
			if latest != nil && gachaIDEquals(rec.ID, latest) {
				hitLatest = true
			}
		}
		if hitLatest {
			break
		}
		endID = data.List[len(data.List)-1].ID
		if endID == "" || endID == "0" {
			break
		}
		page++
		zzzSleep(zzzPageWait)
		if page%10 == 0 {
			// 每 10 页多歇一会，避免请求过快。
			zzzSleep(zzzIntervalWait)
		}
	}
	return records, nil
}

// zzzRequestRetry 请求一页记录，网络或接口错误时重试；authkey 失效直接返回。
func zzzRequestRetry(api *zzzAPIURL, typ string, page int, endID string, size int) (*gachaLogData, error) {
	var lastErr error
	for attempt := 0; attempt < zzzFetchTries; attempt++ {
		if attempt > 0 {
			zzzSleep(zzzRetryWait)
		}
		data, err := zzzRequest(api, typ, page, endID, size)
		if err == nil {
			return data, nil
		}
		if errors.Is(err, errZZZAuthkey) {
			return nil, err
		}
		lastErr = err
	}
	return nil, lastErr
}

func zzzRequest(api *zzzAPIURL, typ string, page int, endID string, size int) (*gachaLogData, error) {
	endpoint := fmt.Sprintf("%s%s?%s&real_gacha_type=%s&page=%d&size=%d&end_id=%s",
		api.domain, zzzAPIPath, api.query, url.QueryEscape(typ), page, size, url.QueryEscape(endID))
	resp, err := zzzHTTPClient.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("请求调频记录失败：%w", sanitizeURLError(err))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
		return nil, fmt.Errorf("接口返回 HTTP %d", resp.StatusCode)
	}
	var payload struct {
		Retcode int          `json:"retcode"`
		Message string       `json:"message"`
		Data    gachaLogData `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&payload); err != nil {
		return nil, fmt.Errorf("无法解析接口响应：%w", err)
	}
	if payload.Retcode != 0 {
		lower := strings.ToLower(payload.Message)
		if strings.Contains(lower, "authkey") || strings.Contains(lower, "登录") {
			return nil, errZZZAuthkey
		}
		if payload.Message == "" {
			payload.Message = "接口请求失败"
		}
		return nil, fmt.Errorf("%s（retcode %d）", payload.Message, payload.Retcode)
	}
	return &payload.Data, nil
}

// mergeZZZExport 按绝区零频段合并本地与抓取到的数据。
func mergeZZZExport(local, fetched *gachaExport) (*gachaExport, int) {
	return mergeGachaExport(local, fetched, zzzChannelOrder, zzzChannelNames, "频段 ")
}

// saveZZZExport 写入绝区零的抽卡导出文件。
func saveZZZExport(dir string, exp *gachaExport) error {
	return saveGachaExport(dir, exp)
}
