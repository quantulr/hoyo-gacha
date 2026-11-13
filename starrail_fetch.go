package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

// 崩坏：星穹铁道跃迁记录接口：国服与国际服域名不同。
const (
	starRailAPICN        = "https://public-operation-hkrpg.mihoyo.com"
	starRailAPIOS        = "https://public-operation-hkrpg-sg.hoyoverse.com"
	starRailGachaPath    = "/common/hkrpg_gacha_record/api/getGachaLog"
	starRailLdGachaPath  = "/common/hkrpg_gacha_record/api/getLdGachaLog"
	starRailPageSize     = 20
	starRailFetchTries   = 5
	starRailRetryWait    = 5 * time.Second
	starRailPageWait     = 300 * time.Millisecond
	starRailIntervalWait = time.Second
)

// starRailChannelOrder 是抓取和展示跃迁频段的固定顺序。
var starRailChannelOrder = []string{"11", "12", "1", "2", "21", "22"}

// starRailChannelNames 是各跃迁频段在简体中文下的名称。
var starRailChannelNames = map[string]string{
	"11": "角色活动跃迁",
	"12": "光锥活动跃迁",
	"1":  "常驻跃迁",
	"2":  "新手跃迁",
	"21": "角色联动跃迁",
	"22": "光锥联动跃迁",
}

// 测试里替换掉真实的网络和等待。
var (
	starRailHTTPClient = &http.Client{Timeout: 15 * time.Second}
	starRailSleep      = time.Sleep
)

var (
	errStarRailAuthkey = errors.New("authkey 已失效，请先在游戏内打开「跃迁记录」页，再点刷新")
	// 日志里的游戏安装路径，例如 C:/Program Files/Star Rail/StarRail_Data/。
	starRailGamePathRE = regexp.MustCompile(`(?i)[a-z]:[/\\].*?[/\\]StarRail_Data[/\\]`)
	// 网页缓存里的完整链接；查询参数里可能有 % 编码的逗号等字符。
	starRailURLRE = regexp.MustCompile(`https://[A-Za-z0-9\-._~:/?#\[\]@!$&'()*+,;=%]+`)
	// authkey 里未编码的等号会让参数解析截断，需要先转义。
	starRailAuthkeyRE = regexp.MustCompile(`authkey=([^&]+)`)
	// 链接里的生成时间戳，用于在缓存和日志之间选较新的一条。
	starRailTimestampRE = regexp.MustCompile(`[?&]timestamp=(\d+)`)
)

// starRailAPIURL 是从缓存链接解析出的请求信息。
type starRailAPIURL struct {
	domain string // 接口域名，国服或国际服
	query  string // 已编码的公共参数，不含分页参数
	lang   string
}

// refreshStarRailData 从游戏日志与网页缓存取得临时链接，抓取全部跃迁并写回。
func refreshStarRailData(progress func(string)) (*gachaRefreshResult, error) {
	root, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		return nil, err
	}
	return refreshStarRailRoot(root, progress)
}

func refreshStarRailRoot(root string, progress func(string)) (*gachaRefreshResult, error) {
	raw, err := findStarRailLogURL(starRailLogPaths())
	if err != nil {
		return nil, err
	}
	api, err := parseStarRailLogURL(raw)
	if err != nil {
		return nil, err
	}
	return fetchStarRailAll(root, api, progress)
}

// starRailLogPaths 返回国服与国际服客户端可能写入的玩家日志路径。
func starRailLogPaths() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	root := filepath.Join(home, "AppData", "LocalLow")
	return []string{
		filepath.Join(root, "miHoYo", "崩坏：星穹铁道", "Player.log"),
		filepath.Join(root, "Cognosphere", "Star Rail", "Player.log"),
	}
}

// findStarRailLogURL 从玩家日志里的安装路径找到游戏的网页缓存，再取出最后一条
// 含 authkey 的跃迁记录页链接。新版游戏会独占缓存文件，读不到时退回日志里
// 记录的链接。
func findStarRailLogURL(paths []string) (string, error) {
	locked := false
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		text := string(data)
		gamePath := starRailGamePathRE.FindString(text)
		if gamePath == "" {
			continue
		}
		cacheRaw, cacheLocked, _ := findStarRailCacheURL(gamePath)
		if cacheLocked {
			locked = true
		}
		logRaw := starRailCacheURL(text)
		switch {
		case cacheRaw != "" && (logRaw == "" || starRailURLTime(cacheRaw) >= starRailURLTime(logRaw)):
			return cacheRaw, nil
		case logRaw != "":
			// 缓存被游戏占用时，日志里通常还留着最近一次打开跃迁记录页的链接。
			return logRaw, nil
		}
	}
	if locked {
		return "", errors.New("跃迁记录缓存被游戏占用，无法读取；请先在游戏内打开「跃迁记录」，然后退出游戏再刷新")
	}
	return "", errors.New("没有在游戏日志或缓存里找到跃迁记录链接，请先在游戏内打开「跃迁记录」页再刷新")
}

// findStarRailCacheURL 读取最新的 data_2 缓存文件，找不到时继续试旧版本目录。
// locked 表示有更新的缓存文件读不到（通常是游戏运行中独占），此时不再使用
// 旧版本目录里的链接，因为其中的 authkey 通常已经过期。
func findStarRailCacheURL(gamePath string) (raw string, locked bool, err error) {
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
		if raw := starRailCacheURL(string(data)); raw != "" {
			if locked {
				return "", true, nil
			}
			return raw, false, nil
		}
	}
	if locked {
		return "", true, nil
	}
	return "", false, errors.New("没有在游戏缓存里找到跃迁记录链接")
}

// starRailURLTime 返回链接里的 timestamp 参数（秒），缺失时返回 0。
func starRailURLTime(raw string) int64 {
	m := starRailTimestampRE.FindStringSubmatch(raw)
	if len(m) != 2 {
		return 0
	}
	n, err := strconv.ParseInt(m[1], 10, 64)
	if err != nil {
		return 0
	}
	return n
}

// starRailCacheURL 返回缓存文本里最后一条跃迁记录页链接。
func starRailCacheURL(text string) string {
	matches := starRailURLRE.FindAllString(text, -1)
	for i := len(matches) - 1; i >= 0; i-- {
		raw := matches[i]
		if strings.Contains(raw, "auth_appid=webview_gacha") &&
			strings.Contains(raw, "authkey=") &&
			strings.Contains(raw, "game_biz=hkrpg_") {
			return raw
		}
	}
	return ""
}

// fixStarRailAuthkey 转义 authkey 里未编码的等号。
func fixStarRailAuthkey(raw string) string {
	m := starRailAuthkeyRE.FindStringSubmatch(raw)
	if len(m) != 2 || !strings.Contains(m[1], "=") || strings.Contains(m[1], "%") {
		return raw
	}
	return strings.Replace(raw, m[0], "authkey="+url.QueryEscape(m[1]), 1)
}

// parseStarRailLogURL 从缓存链接里取出 authkey 等公共参数，并选定接口域名。
func parseStarRailLogURL(raw string) (*starRailAPIURL, error) {
	u, err := url.Parse(strings.TrimSpace(fixStarRailAuthkey(raw)))
	if err != nil {
		return nil, fmt.Errorf("无法解析跃迁记录链接：%w", sanitizeURLError(err))
	}
	q, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return nil, fmt.Errorf("跃迁记录链接的参数无效：%w", err)
	}
	if q.Get("authkey") == "" {
		return nil, errors.New("跃迁记录链接里缺少 authkey，请重新打开游戏内的「跃迁记录」页")
	}
	// begin_id 是网页缓存里滚动到底部时留下的游标；带着它会返回以该游标为界的
	// 旧页（其他频段甚至返回空列表），所以和分页参数一起去掉。
	for _, key := range []string{"page", "size", "gacha_type", "end_id", "begin_id"} {
		q.Del(key)
	}
	domain := starRailAPICN
	host := strings.ToLower(u.Host)
	if strings.Contains(host, "webstatic-sea") || strings.Contains(host, "hkrpg-api-os") ||
		strings.Contains(host, "api-os-takumi") || strings.Contains(host, "hoyoverse.com") {
		domain = starRailAPIOS
	}
	lang := q.Get("lang")
	if lang == "" {
		lang = "zh-cn"
	}
	return &starRailAPIURL{domain: domain, query: q.Encode(), lang: lang}, nil
}

// setLang 把请求语言换成账号已有数据的语言，避免新旧记录语言混杂。
func (api *starRailAPIURL) setLang(lang string) {
	q, err := url.ParseQuery(api.query)
	if err != nil {
		return
	}
	q.Set("lang", lang)
	api.lang = lang
	api.query = q.Encode()
}

// starRailFetchUID 先抽一页，确定账号 UID 与服务器时区。
func starRailFetchUID(api *starRailAPIURL) (string, string, int, error) {
	var region string
	var zone int
	for _, typ := range []string{"11", "12", "1", "2"} {
		data, err := starRailRequestRetry(api, typ, 1, "0", 5)
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

// fetchStarRailAll 抓取全部跃迁频段，与本地数据合并后写回应用数据目录。
func fetchStarRailAll(root string, api *starRailAPIURL, progress func(string)) (*gachaRefreshResult, error) {
	dir := gameDataDir(root, "starrail")
	if progress == nil {
		progress = func(string) {}
	}
	progress("正在确认游戏账号…")
	uid, _, apiZone, err := starRailFetchUID(api)
	if err != nil {
		return nil, err
	}
	if uid == "" {
		return nil, errors.New("没有获取到跃迁记录，请先在游戏内打开「跃迁记录」页再刷新")
	}
	// 接口通常会返回服务器时区；缺失时按国服东八区处理。
	serverZone := apiZone
	if serverZone == 0 {
		serverZone = 8
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
	localIDs := gachaLocalIDs(local)

	fetched := &gachaExport{UID: uid, Lang: api.lang, RegionTimeZone: targetZone}
	for _, typ := range starRailChannelOrder {
		name := starRailChannelNames[typ]
		recs, err := starRailFetchChannel(api, typ, name, localIDs[typ], progress)
		if err != nil {
			return nil, err
		}
		for i := range recs {
			recs[i].Time = convertGachaTime(recs[i].Time, serverZone, targetZone)
		}
		fetched.Result = append(fetched.Result, gachaGroup{Type: typ, Records: recs})
		fetched.TypeMap = append(fetched.TypeMap, [2]string{typ, name})
	}

	merged, added := mergeGachaExport(local, fetched, starRailChannelOrder, starRailChannelNames, "跃迁 ")
	merged.Time = time.Now().UnixMilli()
	if err := saveGachaExport(root, "starrail", merged); err != nil {
		return nil, err
	}
	total := 0
	for _, group := range merged.Result {
		total += len(group.Records)
	}
	return &gachaRefreshResult{UID: uid, Added: added, Total: total}, nil
}

// starRailFetchChannel 按 end_id 翻页抓取一个跃迁频段。localIDs 是本地已有的
// 记录 id，整页都已在本地时说明后面的记录本地都有，可以停止。这样即使有记录
// 在本地最新一条之后才写入服务器（晚到的旧记录），也能补回来。
func starRailFetchChannel(api *starRailAPIURL, typ, name string, localIDs map[string]bool,
	progress func(string)) ([]gachaRecord, error) {
	if progress == nil {
		progress = func(string) {}
	}
	var records []gachaRecord
	page := 1
	endID := "0"
	for {
		progress(fmt.Sprintf("正在读取 %s（第 %d 页）…", name, page))
		data, err := starRailRequestRetry(api, typ, page, endID, starRailPageSize)
		if err != nil {
			return nil, err
		}
		if len(data.List) == 0 {
			break
		}
		pageHasNew := false
		for _, item := range data.List {
			rec := item.gachaRecord
			if rec.Count == "" {
				rec.Count = "1"
			}
			records = append(records, rec)
			if key, ok := gachaIDKey(rec.ID); !ok || !localIDs[key] {
				pageHasNew = true
			}
		}
		if !pageHasNew {
			break
		}
		endID = data.List[len(data.List)-1].ID
		if endID == "" || endID == "0" {
			break
		}
		page++
		starRailSleep(starRailPageWait)
		if page%10 == 0 {
			// 每 10 页多歇一会，避免请求过快。
			starRailSleep(starRailIntervalWait)
		}
	}
	return records, nil
}

// starRailRequestRetry 请求一页记录，网络或接口错误时重试；authkey 失效直接返回。
func starRailRequestRetry(api *starRailAPIURL, typ string, page int, endID string,
	size int) (*gachaLogData, error) {
	var lastErr error
	for attempt := 0; attempt < starRailFetchTries; attempt++ {
		if attempt > 0 {
			starRailSleep(starRailRetryWait)
		}
		data, err := starRailRequest(api, typ, page, endID, size)
		if err == nil {
			return data, nil
		}
		if errors.Is(err, errStarRailAuthkey) {
			return nil, err
		}
		lastErr = err
	}
	return nil, lastErr
}

func starRailRequest(api *starRailAPIURL, typ string, page int, endID string, size int) (*gachaLogData, error) {
	path := starRailGachaPath
	if typ == "21" || typ == "22" {
		// 联动跃迁使用单独的接口。
		path = starRailLdGachaPath
	}
	endpoint := fmt.Sprintf("%s%s?%s&gacha_type=%s&page=%d&size=%d&end_id=%s",
		api.domain, path, api.query, url.QueryEscape(typ), page, size, url.QueryEscape(endID))
	resp, err := starRailHTTPClient.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("请求跃迁记录失败：%w", sanitizeURLError(err))
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
			return nil, errStarRailAuthkey
		}
		if payload.Message == "" {
			payload.Message = "接口请求失败"
		}
		return nil, fmt.Errorf("%s（retcode %d）", payload.Message, payload.Retcode)
	}
	return &payload.Data, nil
}
