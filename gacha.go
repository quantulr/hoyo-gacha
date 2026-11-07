package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/egoist/mygo"
)

// gachaRecord 是导出文件里的一条抽卡记录。
type gachaRecord struct {
	ID        string `json:"id"`
	ItemID    string `json:"item_id"`
	ItemType  string `json:"item_type"`
	Name      string `json:"name"`
	RankType  string `json:"rank_type"` // 等级值随游戏而定：原神/星穹铁道为星级，绝区零为 2/3/4
	Time      string `json:"time"`      // 本地时间，YYYY-MM-DD HH:MM:SS
	GachaType string `json:"gacha_type"`
	Count     string `json:"count,omitempty"`
}

// UnmarshalJSON 同时接受对象记录和原神的紧凑数组记录：
// [时间, 名称, 类型, 等级] 或 [时间, 名称, 类型, 等级, 卡池类型, id]。
func (r *gachaRecord) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || trimmed[0] != '[' {
		type plain gachaRecord
		return json.Unmarshal(data, (*plain)(r))
	}
	var row []json.RawMessage
	if err := json.Unmarshal(trimmed, &row); err != nil {
		return err
	}
	if len(row) < 4 {
		return fmt.Errorf("抽卡记录有 %d 个字段，至少需要 4 个", len(row))
	}
	r.Time = rawString(row[0])
	r.Name = rawString(row[1])
	r.ItemType = rawString(row[2])
	r.RankType = rawString(row[3])
	if len(row) > 4 {
		r.GachaType = rawString(row[4])
	}
	if len(row) > 5 {
		r.ID = rawString(row[5])
	}
	return nil
}

// rawString 把 JSON 字符串或数字转成字符串。
func rawString(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var n json.Number
	if json.Unmarshal(raw, &n) == nil {
		return n.String()
	}
	return ""
}

// gachaGroup 是 result 里的一组记录：卡池类型和它的记录。
type gachaGroup struct {
	Type    string
	Records []gachaRecord
}

// UnmarshalJSON 解析 ["频段类型", [记录…]] 形式的分组。
func (g *gachaGroup) UnmarshalJSON(data []byte) error {
	var pair []json.RawMessage
	if err := json.Unmarshal(data, &pair); err != nil {
		return err
	}
	if len(pair) != 2 {
		return fmt.Errorf("抽卡分组有 %d 个部分，期望 2 个", len(pair))
	}
	if err := json.Unmarshal(pair[0], &g.Type); err != nil {
		return err
	}
	return json.Unmarshal(pair[1], &g.Records)
}

// MarshalJSON 把分组写回 ["频段类型", [记录…]] 形式。
// 没有记录的频段写成空数组，而不是 null，导出文件才能被其他工具读取。
func (g gachaGroup) MarshalJSON() ([]byte, error) {
	recs := g.Records
	if recs == nil {
		recs = []gachaRecord{}
	}
	return json.Marshal([]any{g.Type, recs})
}

// gachaExport 是 gacha-list-<uid>.json 的格式。
type gachaExport struct {
	UID            string       `json:"uid"`
	Lang           string       `json:"lang"`
	TypeMap        [][2]string  `json:"typeMap"`
	Result         []gachaGroup `json:"result"`
	RegionTimeZone int          `json:"region_time_zone,omitempty"`
	Time           int64        `json:"time,omitempty"` // 抓取时间，毫秒
}

// pullRecord 是一条带保底计数的抽卡记录。
type pullRecord struct {
	gachaRecord
	Pity        int  // 这一抽是距上一个 S 级后的第几抽
	Tier        int  // 0 = 最高档，1 = 次高，2 = 第三档
	Lost        bool // 最高档不是当期 UP；常驻、邦布或缺少卡池数据时为 false
	BannerKnown bool // 有当期卡池数据，Lost 的结果有效
	Guaranteed  bool // 上一次限定 S 歪后触发的大保底，不计入歪卡率
}

// 记录页的筛选模式。零值默认只看最高档。
const (
	pullFilterS   = iota // 仅最高档
	pullFilterSA         // 最高档 + 次高档
	pullFilterAll        // 全部
)

// filterPullRanks 按等级保留记录。
func filterPullRanks(recs []pullRecord, mode int) []pullRecord {
	if mode == pullFilterAll {
		return recs
	}
	out := make([]pullRecord, 0, len(recs))
	for _, r := range recs {
		if r.Tier == 0 || (mode == pullFilterSA && r.Tier == 1) {
			out = append(out, r)
		}
	}
	return out
}

// channelStats 是一个卡池频道的统计。
type channelStats struct {
	Type        string
	Name        string
	Pulls       int
	S           int
	A           int
	B           int
	Pity        int // 距上一个 S 级已经抽了多少次
	HardPity    int
	Average     float64      // 平均多少抽出一个 S 级
	LostS       int          // 小保底中歪掉的 S 级
	SmallPityS  int          // 计入歪卡率的小保底 S 级，不含大保底
	LostTracked bool         // 该频段支持歪卡判定（限定卡池）
	Records     []pullRecord // 按时间从新到旧
}

// LostRate 返回该频段小保底中的歪卡率。
func (ch channelStats) LostRate() float64 {
	if ch.SmallPityS == 0 {
		return 0
	}
	return float64(ch.LostS) / float64(ch.SmallPityS)
}

// countLost 汇总用于歪卡率的小保底次数与其中的歪卡数。
// 大保底由上一次歪触发，必然出 UP，不计入分母。
func (ch *channelStats) countLost() {
	ch.LostS, ch.SmallPityS = 0, 0
	for i := range ch.Records {
		r := &ch.Records[i]
		if !r.BannerKnown || r.Guaranteed {
			continue
		}
		ch.SmallPityS++
		if r.Lost {
			ch.LostS++
		}
	}
}

// markBannerPull 标记一次限定 S 的卡池结果，返回下一个 S 是否处于大保底。
// 调用方需按时间从早到晚处理同一频段的 S 级。
func markBannerPull(r *pullRecord, guaranteed, known, up bool) bool {
	r.BannerKnown = known
	r.Guaranteed = guaranteed
	r.Lost = known && !up
	return known && !up
}

// sPull 是一次出 S 级的记录及当时用掉的抽数。
type sPull struct {
	ID       string
	Name     string
	ItemType string
	Channel  string
	Time     string
	Rank     string // 原始等级，如原神/星穹铁道的 5、4、3 星
	Pity     int
}

// gachaRankSpec 是一个频段的等级映射。
type gachaRankSpec struct {
	s string
	a string
	b string
}

// gachaSpec 是一款游戏的导出等级与保底规则。
type gachaSpec struct {
	sRank string
	aRank string
	bRank string
	ranks map[string]gachaRankSpec // 按频段覆盖等级映射
	pity  map[string]int           // 各频段的 S 级保底抽数，缺省 90
}

// rankFor 返回频段使用的等级映射，未单独配置时用游戏默认值。
func (s gachaSpec) rankFor(typ string) gachaRankSpec {
	if r, ok := s.ranks[typ]; ok {
		return r
	}
	return gachaRankSpec{s: s.sRank, a: s.aRank, b: s.bRank}
}

// rankAt 返回等级映射中档位的原始值，如原神的 5、4、3。
func (r gachaRankSpec) rankAt(tier int) string {
	switch tier {
	case 0:
		return r.s
	case 1:
		return r.a
	default:
		return r.b
	}
}

// gachaSpecFor 返回游戏的等级与保底规则。
func gachaSpecFor(game string) gachaSpec {
	if g := gameByID(game); g != nil {
		return g.spec
	}
	return gachaSpec{}
}

// gachaData 是一款游戏抽卡数据的汇总。
type gachaData struct {
	UID        string
	LastTime   string
	TotalPulls int
	TotalS     int
	TotalA     int
	RankCounts map[string]int // 原始等级计数，如原神/星穹铁道的 5、4、3 星
	Channels   []channelStats
	SRecords   []sPull // 按时间从新到旧
}

func (d *gachaData) SRate() float64 {
	if d.TotalPulls == 0 {
		return 0
	}
	return float64(d.TotalS) / float64(d.TotalPulls)
}

func (d *gachaData) ARate() float64 {
	if d.TotalPulls == 0 {
		return 0
	}
	return float64(d.TotalA) / float64(d.TotalPulls)
}

func (d *gachaData) Average() float64 {
	if d.TotalS == 0 {
		return 0
	}
	return float64(d.TotalPulls) / float64(d.TotalS)
}

// LostStats 返回小保底中歪掉的 S 级数和小保底次数；大保底不计入分母。
func (d *gachaData) LostStats() (lost, smallPity int) {
	for _, ch := range d.Channels {
		lost += ch.LostS
		smallPity += ch.SmallPityS
	}
	return lost, smallPity
}

// LostRate 返回小保底中的歪卡率。
func (d *gachaData) LostRate() float64 {
	lost, smallPity := d.LostStats()
	if smallPity == 0 {
		return 0
	}
	return float64(lost) / float64(smallPity)
}

// HasLostChannels 表示数据里是否有支持歪卡判定的限定频段。
func (d *gachaData) HasLostChannels() bool {
	for _, ch := range d.Channels {
		if ch.LostTracked {
			return true
		}
	}
	return false
}

// loadGameData 从应用数据目录读取一款游戏的抽卡数据。
func loadGameData(game string) (*gachaData, error) {
	root, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		return nil, err
	}
	return loadGachaGame(root, game)
}

// loadGachaGame 从 root 下的统一配置和数据目录读取一款游戏。
// 还没有任何 gacha-list-*.json 时返回 nil, nil，让界面提示导入或获取。
func loadGachaGame(root, game string) (*gachaData, error) {
	dir := gameDataDir(root, game)
	current := ""
	if cfg, err := loadGameConfig(root); err == nil {
		current = configCurrent(cfg, game)
	}
	// 兼容旧版每款游戏各自的 config.json；读取后会在下面写回统一配置。
	if current == "" {
		current = legacyCurrent(dir)
	}
	return loadGachaAccount(root, game, current)
}

// loadGachaAccount 读取指定账号的抽卡数据；uid 为空时退回最近修改的文件。
// 读入成功会把统一配置的 current 指向文件里的 UID。
func loadGachaAccount(root, game, uid string) (*gachaData, error) {
	dir := gameDataDir(root, game)
	path, err := gachaFilePath(dir, uid)
	if errors.Is(err, errNoGachaFile) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	exp, err := parseGachaExport(data)
	if err != nil {
		return nil, err
	}
	if exp.UID == "" {
		exp.UID = gachaUIDFromPath(path)
	}
	d := newGachaData(exp, game)
	// 手动导入的文件可能还没有配置；记下这个账号，下次启动继续读取。
	// 配置写入失败不影响已经读入的记录。
	if exp.UID != "" {
		_ = saveGameCurrentIfChanged(root, game, exp.UID)
	}
	// 游戏的补充数据（如卡池）读取失败不影响抽卡记录本身。
	if g := gameByID(game); g != nil && g.annotate != nil {
		g.annotate(d)
	}
	return d, nil
}

// legacyCurrent 读取旧版游戏目录里的 config.json，用于迁移到统一配置。
func legacyCurrent(dir string) string {
	var cfg struct {
		Current string `json:"current"`
	}
	data, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil || json.Unmarshal(data, &cfg) != nil {
		return ""
	}
	return cfg.Current
}

// errNoGachaFile 表示目录里还没有任何导出的抽卡文件。
var errNoGachaFile = errors.New("没有 gacha-list-*.json")

// gachaUIDFromPath 从 gacha-list-<uid>.json 文件名中取出 UID。
func gachaUIDFromPath(path string) string {
	name := filepath.Base(path)
	if !strings.HasPrefix(name, "gacha-list-") || !strings.HasSuffix(name, ".json") {
		return ""
	}
	return strings.TrimSuffix(strings.TrimPrefix(name, "gacha-list-"), ".json")
}

// gachaFilePath 找到要读取的抽卡文件：统一配置里的当前账号，
// 否则取最近修改的 gacha-list-*.json。
func gachaFilePath(dir, current string) (string, error) {
	if current != "" {
		p := filepath.Join(dir, "gacha-list-"+current+".json")
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	matches, err := filepath.Glob(filepath.Join(dir, "gacha-list-*.json"))
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "", errNoGachaFile
	}
	sort.Slice(matches, func(i, j int) bool {
		si, errI := os.Stat(matches[i])
		sj, errJ := os.Stat(matches[j])
		if errI != nil || errJ != nil || si.ModTime().Equal(sj.ModTime()) {
			return matches[i] > matches[j]
		}
		return si.ModTime().After(sj.ModTime())
	})
	return matches[0], nil
}

// parseGacha 解析抽卡导出文件并汇总统计。
func parseGacha(data []byte, game string) (*gachaData, error) {
	exp, err := parseGachaExport(data)
	if err != nil {
		return nil, err
	}
	return newGachaData(exp, game), nil
}

func parseGachaExport(data []byte) (*gachaExport, error) {
	var exp gachaExport
	if err := json.Unmarshal(data, &exp); err != nil {
		return nil, err
	}
	return &exp, nil
}

// newGachaData 按频段汇总记录，并算出保底与出金历史。
func newGachaData(exp *gachaExport, game string) *gachaData {
	names := make(map[string]string, len(exp.TypeMap))
	order := make([]string, 0, len(exp.TypeMap))
	for _, p := range exp.TypeMap {
		if len(p) != 2 {
			continue
		}
		if _, seen := names[p[0]]; !seen {
			order = append(order, p[0])
		}
		names[p[0]] = p[1]
	}
	groups := make(map[string][]gachaRecord, len(exp.Result))
	for _, g := range exp.Result {
		groups[g.Type] = append(groups[g.Type], g.Records...)
		if _, ok := names[g.Type]; !ok {
			names[g.Type] = "频段 " + g.Type
			order = append(order, g.Type)
		}
	}

	spec := gachaSpecFor(game)
	d := &gachaData{UID: exp.UID, RankCounts: map[string]int{}}
	for _, typ := range order {
		recs := groups[typ]
		if len(recs) == 0 {
			continue
		}
		ch := channelStats{Type: typ, Name: names[typ], HardPity: hardPity(game, typ)}
		rank := spec.rankFor(typ)
		// 导出文件通常已按时间升序；排序保证保底计算正确。
		sort.SliceStable(recs, func(i, j int) bool {
			if recs[i].Time != recs[j].Time {
				return recs[i].Time < recs[j].Time
			}
			return recs[i].ID < recs[j].ID
		})
		pity := 0
		for _, r := range recs {
			pity++
			ch.Pulls++
			tier := 2
			switch r.RankType {
			case rank.s:
				tier = 0
			case rank.a:
				tier = 1
			case rank.b:
				tier = 2
			}
			ch.Records = append(ch.Records, pullRecord{gachaRecord: r, Pity: pity, Tier: tier})
			if r.RankType != "" {
				d.RankCounts[r.RankType]++
			}
			if r.Time > d.LastTime {
				d.LastTime = r.Time
			}
			switch tier {
			case 0:
				ch.S++
				d.TotalS++
				d.SRecords = append(d.SRecords, sPull{
					ID:       r.ID,
					Name:     r.Name,
					ItemType: r.ItemType,
					Channel:  ch.Name,
					Time:     r.Time,
					Rank:     r.RankType,
					Pity:     pity,
				})
				pity = 0
			case 1:
				ch.A++
				d.TotalA++
			default:
				ch.B++
			}
		}
		ch.Pity = pity
		if ch.S > 0 {
			ch.Average = float64(ch.Pulls) / float64(ch.S)
		}
		// 展示时最新在前。
		for i, j := 0, len(ch.Records)-1; i < j; i, j = i+1, j-1 {
			ch.Records[i], ch.Records[j] = ch.Records[j], ch.Records[i]
		}
		d.TotalPulls += ch.Pulls
		d.Channels = append(d.Channels, ch)
	}
	sort.SliceStable(d.SRecords, func(i, j int) bool {
		if d.SRecords[i].Time != d.SRecords[j].Time {
			return d.SRecords[i].Time > d.SRecords[j].Time
		}
		return d.SRecords[i].ID > d.SRecords[j].ID
	})
	return d
}

// hardPity 是一款游戏各频道的 S 级保底抽数。
func hardPity(game, typ string) int {
	if p, ok := gachaSpecFor(game).pity[typ]; ok {
		return p
	}
	return 90
}
