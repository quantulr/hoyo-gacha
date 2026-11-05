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
	"sort"
	"strings"
	"time"

	"github.com/egoist/mygo"
)

// 原神祈愿记录接口：国服与国际服域名不同。
const (
	genshinAPICN        = "https://public-operation-hk4e.mihoyo.com"
	genshinAPIOS        = "https://public-operation-hk4e-sg.hoyoverse.com"
	genshinGachaPath    = "/gacha_info/api/getGachaLog"
	genshinPageSize     = 20
	genshinProbeSize    = 6
	genshinFetchTries   = 5
	genshinRetryWait    = 5 * time.Second
	genshinPageWait     = 300 * time.Millisecond
	genshinIntervalWait = time.Second
)

// genshinChannelOrder 是抓取和展示祈愿频段的固定顺序。
var genshinChannelOrder = []string{"301", "302", "500", "200", "100"}

// genshinChannelNames 是各祈愿频段在简体中文下的名称。
var genshinChannelNames = map[string]string{
	"301": "角色活动祈愿",
	"302": "武器活动祈愿",
	"500": "集录祈愿",
	"200": "常驻祈愿",
	"100": "新手祈愿",
}

// 测试里替换掉真实的网络和等待。
var (
	genshinHTTPClient = &http.Client{Timeout: 15 * time.Second}
	genshinSleep      = time.Sleep
)

var (
	errGenshinAuthkey = errors.New("authkey 已失效，请先在游戏内打开「祈愿记录」页，再点刷新")
	// 日志里的游戏安装路径，例如 C:/Program Files/Genshin Impact/GenshinImpact_Data。
	genshinGamePathRE = regexp.MustCompile(`(?i)[a-z]:[/\\].+?(GenshinImpact_Data|YuanShen_Data)`)
	// 网页缓存里的完整链接；查询参数里可能有 % 编码的逗号等字符。
	genshinURLRE = regexp.MustCompile(`https://[A-Za-z0-9\-._~:/?#\[\]@!$&'()*+,;=%]+`)
	// authkey 里未编码的等号会让参数解析截断，需要先转义。
	genshinAuthkeyRE = regexp.MustCompile(`authkey=([^&]+)`)
)

// genshinAPIURL 是从缓存链接解析出的请求信息。
type genshinAPIURL struct {
	domain string // 接口域名，国服或国际服
	query  string // 已编码的公共参数，不含分页参数
	lang   string
}

// genshinLogFile 是可能含有祈愿记录链接的日志。云游戏日志直接含链接，
// 普通客户端日志则要先找到安装目录，再去读网页缓存。
type genshinLogFile struct {
	path   string
	direct bool
}

// refreshGenshinData 从游戏日志与网页缓存取得临时链接，抓取全部祈愿并写回。
// ready 在 authkey 通过校验、开始翻页前调用，界面据此才显示刷新弹框。
func refreshGenshinData(progress func(string), ready func()) (*gachaRefreshResult, error) {
	root, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		return nil, err
	}
	return refreshGenshinDir(filepath.Join(root, "data", "genshin"), progress, ready)
}

func refreshGenshinDir(dir string, progress func(string), ready func()) (*gachaRefreshResult, error) {
	raw, err := findGenshinLogURL(genshinLogFiles())
	if err != nil {
		return nil, err
	}
	api, err := parseGenshinLogURL(raw)
	if err != nil {
		return nil, err
	}
	return fetchGenshinAll(dir, api, progress, ready)
}

// genshinLogFiles 返回国服、国际服客户端与云游戏可能写入的日志文件。
func genshinLogFiles() []genshinLogFile {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	localLow := filepath.Join(home, "AppData", "LocalLow")
	local := filepath.Join(home, "AppData", "Local")
	return []genshinLogFile{
		{path: filepath.Join(localLow, "miHoYo", "原神", "output_log.txt")},
		{path: filepath.Join(localLow, "miHoYo", "Genshin Impact", "output_log.txt")},
		{path: filepath.Join(local, "miHoYo", "GenshinImpactCloudGame", "config", "logs", "MiHoYoSDK.log"), direct: true},
	}
}

// findGenshinLogURL 从普通客户端日志里的安装路径找到网页缓存，或直接从
// 云游戏日志里取出最后一条含 authkey 的祈愿记录页链接。
func findGenshinLogURL(files []genshinLogFile) (string, error) {
	for _, file := range files {
		data, err := os.ReadFile(file.path)
		if err != nil {
			continue
		}
		text := string(data)
		if file.direct {
			if raw := genshinCacheURL(text); raw != "" {
				return raw, nil
			}
			continue
		}
		gamePath := genshinGamePathRE.FindString(text)
		if gamePath == "" {
			continue
		}
		if raw, err := findGenshinCacheURL(gamePath); err == nil && raw != "" {
			return raw, nil
		}
	}
	return "", errors.New("没有在游戏日志或缓存里找到祈愿记录链接，请先在游戏内打开「祈愿记录」页再刷新")
}

// findGenshinCacheURL 按修改时间从新到旧读取 data_2 缓存文件，找不到时继续
// 试旧版本目录。
func findGenshinCacheURL(gamePath string) (string, error) {
	root := filepath.FromSlash(strings.ReplaceAll(gamePath, `\`, "/"))
	var files []string
	for _, pattern := range []string{
		filepath.Join(root, "webCaches", "Cache", "Cache_Data", "data_2"),
		filepath.Join(root, "webCaches", "*", "Cache", "Cache_Data", "data_2"),
	} {
		matches, _ := filepath.Glob(pattern)
		files = append(files, matches...)
	}
	if len(files) == 0 {
		return "", errors.New("没有找到游戏的网页缓存")
	}
	sort.Slice(files, func(i, j int) bool {
		si, errI := os.Stat(files[i])
		sj, errJ := os.Stat(files[j])
		if errI != nil || errJ != nil {
			return errI == nil
		}
		return si.ModTime().After(sj.ModTime())
	})
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if raw := genshinCacheURL(string(data)); raw != "" {
			return raw, nil
		}
	}
	return "", errors.New("没有在游戏缓存里找到祈愿记录链接")
}

// genshinCacheURL 返回缓存文本里最后一条祈愿记录页链接。
func genshinCacheURL(text string) string {
	matches := genshinURLRE.FindAllString(text, -1)
	for i := len(matches) - 1; i >= 0; i-- {
		raw := matches[i]
		if strings.Contains(raw, "auth_appid=webview_gacha") &&
			strings.Contains(raw, "authkey=") &&
			strings.Contains(raw, "game_biz=hk4e_") {
			return raw
		}
	}
	return ""
}

// fixGenshinAuthkey 转义 authkey 里未编码的等号。
func fixGenshinAuthkey(raw string) string {
	m := genshinAuthkeyRE.FindStringSubmatch(raw)
	if len(m) != 2 || !strings.Contains(m[1], "=") || strings.Contains(m[1], "%") {
		return raw
	}
	return strings.Replace(raw, m[0], "authkey="+url.QueryEscape(m[1]), 1)
}

// parseGenshinLogURL 从缓存链接里取出 authkey 等公共参数，并选定接口域名。
func parseGenshinLogURL(raw string) (*genshinAPIURL, error) {
	u, err := url.Parse(strings.TrimSpace(fixGenshinAuthkey(raw)))
	if err != nil {
		return nil, fmt.Errorf("无法解析祈愿记录链接：%w", sanitizeURLError(err))
	}
	q, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return nil, fmt.Errorf("祈愿记录链接的参数无效：%w", err)
	}
	if q.Get("authkey") == "" {
		return nil, errors.New("祈愿记录链接里缺少 authkey，请重新打开游戏内的「祈愿记录」页")
	}
	for _, key := range []string{"page", "size", "gacha_type", "end_id"} {
		q.Del(key)
	}
	domain := genshinAPICN
	host := strings.ToLower(u.Host)
	if strings.Contains(host, "webstatic-sea") || strings.Contains(host, "hk4e-api-os") ||
		strings.Contains(host, "api-os-takumi") || strings.Contains(host, "hoyoverse.com") {
		domain = genshinAPIOS
	}
	lang := q.Get("lang")
	if lang == "" {
		lang = "zh-cn"
	}
	return &genshinAPIURL{domain: domain, query: q.Encode(), lang: lang}, nil
}

// setLang 把请求语言换成账号已有数据的语言，避免新旧记录语言混杂。
func (api *genshinAPIURL) setLang(lang string) {
	q, err := url.ParseQuery(api.query)
	if err != nil {
		return
	}
	q.Set("lang", lang)
	api.lang = lang
	api.query = q.Encode()
}

// genshinFetchUID 先抽一页，确定账号 UID 与服务器时区。
func genshinFetchUID(api *genshinAPIURL) (string, string, int, error) {
	var region string
	var zone int
	for _, typ := range genshinChannelOrder {
		data, err := genshinRequestRetry(api, typ, 1, "0", genshinProbeSize)
		if err != nil {
			return "", region, zone, err
		}
		if region == "" {
			region = data.Region
		}
		if zone == 0 {
			zone = data.RegionTimeZone
		}
		if len(data.List) > 0 && data.List[0].UID != "" {
			return data.List[0].UID, data.Region, zone, nil
		}
	}
	return "", region, zone, nil
}

// genshinRegionZone 按 UID 前缀返回服务器时区：6 为美服（UTC-5），
// 7 为欧服（UTC+1），其余（国服、亚服、台港澳服）为 UTC+8。
func genshinRegionZone(uid string) int {
	switch {
	case strings.HasPrefix(uid, "6"):
		return -5
	case strings.HasPrefix(uid, "7"):
		return 1
	default:
		return 8
	}
}

// fetchGenshinAll 抓取全部祈愿频段，与本地数据合并后写回 dir。ready 在
// authkey 通过校验、开始翻页前调用。
func fetchGenshinAll(dir string, api *genshinAPIURL, progress func(string),
	ready func()) (*gachaRefreshResult, error) {
	if progress == nil {
		progress = func(string) {}
	}
	progress("正在确认游戏账号…")
	uid, _, apiZone, err := genshinFetchUID(api)
	if err != nil {
		return nil, err
	}
	if uid == "" {
		return nil, errors.New("没有获取到祈愿记录，请先在游戏内打开「祈愿记录」页再刷新")
	}
	// 原神接口不返回 region_time_zone，按 UID 前缀判断服务器时区。
	serverZone := apiZone
	if serverZone == 0 {
		serverZone = genshinRegionZone(uid)
	}
	local, err := loadGachaLocal(dir, uid)
	if err != nil {
		return nil, err
	}
	if local != nil && local.Lang != "" && local.Lang != api.lang {
		api.setLang(local.Lang)
	}
	targetZone := serverZone
	if local != nil && local.RegionTimeZone != 0 {
		targetZone = local.RegionTimeZone
	}
	latest := gachaLatestIDs(local)
	if ready != nil {
		ready()
	}

	fetched := &gachaExport{UID: uid, Lang: api.lang, RegionTimeZone: targetZone}
	for _, typ := range genshinChannelOrder {
		name := genshinChannelNames[typ]
		recs, err := genshinFetchChannel(api, typ, name, latest[typ], progress)
		if err != nil {
			return nil, err
		}
		for i := range recs {
			recs[i].Time = convertGachaTime(recs[i].Time, serverZone, targetZone)
		}
		fetched.Result = append(fetched.Result, gachaGroup{Type: typ, Records: recs})
		fetched.TypeMap = append(fetched.TypeMap, [2]string{typ, name})
	}

	merged, added := mergeGachaExport(local, fetched, genshinChannelOrder, genshinChannelNames, "祈愿 ")
	merged.Time = time.Now().UnixMilli()
	if err := saveGachaExport(dir, merged); err != nil {
		return nil, err
	}
	total := 0
	for _, group := range merged.Result {
		total += len(group.Records)
	}
	return &gachaRefreshResult{UID: uid, Added: added, Total: total}, nil
}

// genshinFetchChannel 按 end_id 翻页抓取一个祈愿频段。latest 是本地最新的
// 记录 id，碰到它就说明后面的记录本地都有了。
func genshinFetchChannel(api *genshinAPIURL, typ, name string, latest *big.Int,
	progress func(string)) ([]gachaRecord, error) {
	var records []gachaRecord
	page := 1
	endID := "0"
	for {
		progress(fmt.Sprintf("正在读取 %s（第 %d 页）…", name, page))
		data, err := genshinRequestRetry(api, typ, page, endID, genshinPageSize)
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
		genshinSleep(genshinPageWait)
		if page%10 == 0 {
			// 每 10 页多歇一会，避免请求过快。
			genshinSleep(genshinIntervalWait)
		}
	}
	return records, nil
}

// genshinRequestRetry 请求一页记录，网络或接口错误时重试；authkey 失效直接返回。
func genshinRequestRetry(api *genshinAPIURL, typ string, page int, endID string,
	size int) (*gachaLogData, error) {
	var lastErr error
	for attempt := 0; attempt < genshinFetchTries; attempt++ {
		if attempt > 0 {
			genshinSleep(genshinRetryWait)
		}
		data, err := genshinRequest(api, typ, page, endID, size)
		if err == nil {
			return data, nil
		}
		if errors.Is(err, errGenshinAuthkey) {
			return nil, err
		}
		lastErr = err
	}
	return nil, lastErr
}

func genshinRequest(api *genshinAPIURL, typ string, page int, endID string, size int) (*gachaLogData, error) {
	endpoint := fmt.Sprintf("%s%s?%s&gacha_type=%s&page=%d&size=%d&end_id=%s",
		api.domain, genshinGachaPath, api.query, url.QueryEscape(typ), page, size, url.QueryEscape(endID))
	resp, err := genshinHTTPClient.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("请求祈愿记录失败：%w", sanitizeURLError(err))
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
		if payload.Retcode == -100 || payload.Retcode == -101 ||
			strings.Contains(lower, "authkey") || strings.Contains(lower, "登录") ||
			strings.Contains(lower, "过期") {
			return nil, errGenshinAuthkey
		}
		if payload.Message == "" {
			payload.Message = "接口请求失败"
		}
		return nil, fmt.Errorf("%s（retcode %d）", payload.Message, payload.Retcode)
	}
	return &payload.Data, nil
}
