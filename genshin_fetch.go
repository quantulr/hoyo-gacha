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
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"
)

// 原神祈愿记录接口：国服与国际服域名不同。
const (
	genshinAPICN           = "https://public-operation-hk4e.mihoyo.com"
	genshinAPIOS           = "https://public-operation-hk4e-sg.hoyoverse.com"
	genshinGachaPath       = "/gacha_info/api/getGachaLog"
	genshinBeyondGachaPath = "/gacha_info/api/getBeyondGachaLog"
	genshinPageSize        = 20
	genshinBeyondPageSize  = 20
	genshinProbeSize       = 6
	genshinFetchTries      = 5
	genshinRetryWait       = 5 * time.Second
	genshinPageWait        = 300 * time.Millisecond
	genshinIntervalWait    = time.Second
)

// genshinChannelOrder 是抓取和展示祈愿频段的固定顺序。
var genshinChannelOrder = []string{"301", "302", "500", "200", "100", "2000", "1000"}

// genshinNormalChannelOrder 是普通祈愿接口的频段。
var genshinNormalChannelOrder = []string{"301", "302", "500", "200", "100"}

// genshinBeyondChannelOrder 是千星奇域颂愿接口的频段。活动颂愿接口返回
// 20011/20012/20021/20022 等子池记录，统一归入 2000。
var genshinBeyondChannelOrder = []string{"2000", "1000"}

// genshinChannelNames 是各祈愿频段在简体中文下的名称。
var genshinChannelNames = map[string]string{
	"301":  "角色活动祈愿",
	"302":  "武器活动祈愿",
	"500":  "集录祈愿",
	"200":  "常驻祈愿",
	"100":  "新手祈愿",
	"2000": "千星奇域·活动颂愿",
	"1000": "千星奇域·常驻颂愿",
}

// 测试里替换掉真实的网络和等待。
var (
	genshinHTTPClient = &http.Client{Timeout: 15 * time.Second}
	genshinSleep      = time.Sleep
)

var (
	errGenshinAuthkey = errors.New("authkey 已失效，请先在游戏内打开「祈愿记录」或「颂愿」页，再点刷新")
	// 日志里的游戏安装路径，例如 C:/Program Files/Genshin Impact/GenshinImpact_Data。
	genshinGamePathRE = regexp.MustCompile(`(?i)[a-z]:[/\\].+?(GenshinImpact_Data|YuanShen_Data)`)
	// 网页缓存里的完整链接；查询参数里可能有 % 编码的逗号等字符。
	genshinURLRE = regexp.MustCompile(`https://[A-Za-z0-9\-._~:/?#\[\]@!$&'()*+,;=%]+`)
	// authkey 里未编码的等号会让参数解析截断，需要先转义。
	genshinAuthkeyRE = regexp.MustCompile(`authkey=([^&]+)`)
	// 链接里的生成时间戳，用于在缓存和日志之间选较新的一条。
	genshinTimestampRE = regexp.MustCompile(`[?&]timestamp=(\d+)`)
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
func refreshGenshinData(progress func(string)) (*gachaRefreshResult, error) {
	root, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		return nil, err
	}
	return refreshGenshinRoot(root, progress)
}

func refreshGenshinRoot(root string, progress func(string)) (*gachaRefreshResult, error) {
	raw, err := findGenshinLogURL(genshinLogFiles())
	if err != nil {
		return nil, err
	}
	api, err := parseGenshinLogURL(raw)
	if err != nil {
		return nil, err
	}
	return fetchGenshinAll(root, api, progress)
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
// 云游戏日志里取出最后一条含 authkey 的祈愿记录页链接。新版游戏会独占
// 缓存文件，读不到时退回日志里记录的链接。
func findGenshinLogURL(files []genshinLogFile) (string, error) {
	locked := false
	for _, file := range files {
		data, err := os.ReadFile(file.path)
		if err != nil {
			continue
		}
		text := string(data)
		logRaw := genshinCacheURL(text)
		if file.direct {
			if logRaw != "" {
				return logRaw, nil
			}
			continue
		}
		gamePath := genshinGamePathRE.FindString(text)
		if gamePath == "" {
			continue
		}
		cacheRaw, cacheLocked, _ := findGenshinCacheURL(gamePath)
		if cacheLocked {
			locked = true
		}
		switch {
		case cacheRaw != "" && (logRaw == "" || genshinURLTime(cacheRaw) >= genshinURLTime(logRaw)):
			return cacheRaw, nil
		case logRaw != "":
			// 缓存被游戏占用时，日志里通常还留着最近一次打开祈愿记录页的链接。
			return logRaw, nil
		}
	}
	if locked {
		return "", errors.New("祈愿/颂愿缓存被游戏占用，无法读取；请先在游戏内打开「祈愿记录」或「颂愿」页，然后退出游戏再刷新")
	}
	return "", errors.New("没有在游戏日志或缓存里找到祈愿/颂愿记录链接，请先在游戏内打开「祈愿记录」或「颂愿」页再刷新")
}

// findGenshinCacheURL 按修改时间从新到旧读取 data_2 缓存文件，找不到时继续
// 试旧版本目录。locked 表示有更新的缓存文件读不到（通常是游戏运行中独占），
// 此时不再使用旧版本目录里的链接，因为其中的 authkey 通常已经过期。
func findGenshinCacheURL(gamePath string) (raw string, locked bool, err error) {
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
		return "", false, errors.New("没有找到游戏的网页缓存")
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
			locked = true
			continue
		}
		if raw := genshinCacheURL(string(data)); raw != "" {
			if locked {
				return "", true, nil
			}
			return raw, false, nil
		}
	}
	if locked {
		return "", true, nil
	}
	return "", false, errors.New("没有在游戏缓存里找到祈愿/颂愿记录链接")
}

// genshinURLTime 返回链接里的 timestamp 参数（秒），缺失时返回 0。
func genshinURLTime(raw string) int64 {
	m := genshinTimestampRE.FindStringSubmatch(raw)
	if len(m) != 2 {
		return 0
	}
	n, err := strconv.ParseInt(m[1], 10, 64)
	if err != nil {
		return 0
	}
	return n
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
	// begin_id 是网页缓存里滚动到底部时留下的游标；带着它会返回以该游标为界的
	// 旧页（其他频段甚至返回空列表），所以和分页参数一起去掉。
	for _, key := range []string{"page", "size", "gacha_type", "end_id", "begin_id"} {
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

// genshinFetchUID 先抽一页，确定账号 UID 与服务器时区。只玩颂愿的账号
// 普通祈愿全为空，因此再探测颂愿频段。
func genshinFetchUID(api *genshinAPIURL) (string, string, int, error) {
	var region string
	var zone int
	for _, typ := range genshinNormalChannelOrder {
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
	beyond := genshinBeyondAPI(api)
	for _, typ := range genshinBeyondChannelOrder {
		data, err := genshinBeyondRequestRetry(&beyond, typ, 1, "0", genshinProbeSize)
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
			return data.List[0].UID, region, zone, nil
		}
	}
	return "", region, zone, nil
}

// genshinBeyondAPI 返回颂愿接口的请求信息。英文接口要求 en-us，
// 而普通祈愿使用 en。
func genshinBeyondAPI(api *genshinAPIURL) genshinAPIURL {
	out := *api
	if out.lang == "en" {
		out.setLang("en-us")
	}
	return out
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

// fetchGenshinAll 抓取全部祈愿频段，与本地数据合并后写回应用数据目录。
func fetchGenshinAll(root string, api *genshinAPIURL, progress func(string)) (*gachaRefreshResult, error) {
	dir := gameDataDir(root, "genshin")
	if progress == nil {
		progress = func(string) {}
	}
	progress("正在确认游戏账号…")
	uid, _, apiZone, err := genshinFetchUID(api)
	if err != nil {
		return nil, err
	}
	if uid == "" {
		return nil, errors.New("没有获取到祈愿或颂愿记录，请先在游戏内打开「祈愿记录」或「颂愿」页再刷新")
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

	fetched := &gachaExport{UID: uid, Lang: api.lang, RegionTimeZone: targetZone}
	for _, typ := range genshinNormalChannelOrder {
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

	// 千星奇域颂愿使用独立接口。
	beyondAPI := genshinBeyondAPI(api)
	for _, typ := range genshinBeyondChannelOrder {
		name := genshinChannelNames[typ]
		recs, err := genshinFetchBeyondChannel(&beyondAPI, typ, name, latest[typ], progress)
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
	if err := saveGachaExport(root, "genshin", merged); err != nil {
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

// beyondGachaRecord 是 getBeyondGachaLog 返回的一条颂愿记录。
type beyondGachaRecord struct {
	UID         string `json:"uid"`
	ID          string `json:"id"`
	ItemType    string `json:"item_type"`
	ItemID      string `json:"item_id"`
	ItemName    string `json:"item_name"`
	RankType    string `json:"rank_type"`
	Time        string `json:"time"`
	OpGachaType string `json:"op_gacha_type"`
}

// beyondGachaLogData 是颂愿接口响应里的 data 部分。
type beyondGachaLogData struct {
	List           []beyondGachaRecord `json:"list"`
	Region         string              `json:"region"`
	RegionTimeZone int                 `json:"region_time_zone"`
}

// toGachaRecord 把颂愿记录转成导出文件里的通用记录。gacha_type 保留
// 接口返回的 op_gacha_type，便于区分活动颂愿的实际子池。
func (r beyondGachaRecord) toGachaRecord(typ string) gachaRecord {
	gachaType := r.OpGachaType
	if gachaType == "" {
		gachaType = typ
	}
	return gachaRecord{
		ID:        r.ID,
		ItemID:    r.ItemID,
		ItemType:  r.ItemType,
		Name:      r.ItemName,
		RankType:  r.RankType,
		Time:      r.Time,
		GachaType: gachaType,
		Count:     "1",
	}
}

// genshinFetchBeyondChannel 按 end_id 翻页抓取一个颂愿频段。
func genshinFetchBeyondChannel(api *genshinAPIURL, typ, name string, latest *big.Int,
	progress func(string)) ([]gachaRecord, error) {
	var records []gachaRecord
	page := 1
	endID := "0"
	for {
		progress(fmt.Sprintf("正在读取 %s（第 %d 页）…", name, page))
		data, err := genshinBeyondRequestRetry(api, typ, page, endID, genshinBeyondPageSize)
		if err != nil {
			return nil, err
		}
		if len(data.List) == 0 {
			break
		}
		hitLatest := false
		for _, item := range data.List {
			rec := item.toGachaRecord(typ)
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

// genshinBeyondRequestRetry 请求一页颂愿记录，网络或接口错误时重试。
func genshinBeyondRequestRetry(api *genshinAPIURL, typ string, page int, endID string,
	size int) (*beyondGachaLogData, error) {
	var lastErr error
	for attempt := 0; attempt < genshinFetchTries; attempt++ {
		if attempt > 0 {
			genshinSleep(genshinRetryWait)
		}
		data, err := genshinBeyondRequest(api, typ, page, endID, size)
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

func genshinBeyondRequest(api *genshinAPIURL, typ string, page int, endID string, size int) (*beyondGachaLogData, error) {
	endpoint := fmt.Sprintf("%s%s?%s&gacha_type=%s&page=%d&size=%d&end_id=%s",
		api.domain, genshinBeyondGachaPath, api.query, url.QueryEscape(typ), page, size, url.QueryEscape(endID))
	resp, err := genshinHTTPClient.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("请求颂愿记录失败：%w", sanitizeURLError(err))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
		return nil, fmt.Errorf("接口返回 HTTP %d", resp.StatusCode)
	}
	var payload struct {
		Retcode int                `json:"retcode"`
		Message string             `json:"message"`
		Data    beyondGachaLogData `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&payload); err != nil {
		return nil, fmt.Errorf("无法解析接口响应：%w", err)
	}
	if err := genshinAPIError(payload.Retcode, payload.Message); err != nil {
		return nil, err
	}
	return &payload.Data, nil
}

// genshinAPIError 把接口错误转成统一的错误；authkey 相关错误单独返回。
func genshinAPIError(retcode int, message string) error {
	if retcode == 0 {
		return nil
	}
	lower := strings.ToLower(message)
	if retcode == -100 || retcode == -101 ||
		strings.Contains(lower, "authkey") || strings.Contains(lower, "登录") ||
		strings.Contains(lower, "过期") {
		return errGenshinAuthkey
	}
	if message == "" {
		message = "接口请求失败"
	}
	return fmt.Errorf("%s（retcode %d）", message, retcode)
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
	if err := genshinAPIError(payload.Retcode, payload.Message); err != nil {
		return nil, err
	}
	return &payload.Data, nil
}
