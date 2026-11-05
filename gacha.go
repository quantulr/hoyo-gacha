package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/egoist/mygo"
)

// gachaRecord 是导出文件里的一条抽卡记录。
type gachaRecord struct {
	ID        string `json:"id"`
	ItemID    string `json:"item_id"`
	ItemType  string `json:"item_type"`
	Name      string `json:"name"`
	RankType  string `json:"rank_type"` // 4 = S 级，3 = A 级，2 = B 级
	Time      string `json:"time"`      // 本地时间，YYYY-MM-DD HH:MM:SS
	GachaType string `json:"gacha_type"`
	Count     string `json:"count,omitempty"`
}

// gachaGroup 是 result 里的一组记录：频段类型和它的记录。
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
	Pity int  // 这一抽是距上一个 S 级后的第几抽
	Tier int  // 0 = S，1 = A，2 = B
	Lost bool // S 级不是当期 UP；常驻、邦布或缺少卡池数据时为 false
}

// 记录页的筛选模式。
const (
	pullFilterAll = iota // 全部
	pullFilterSA         // S + A
	pullFilterS          // 仅 S
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
	Type     string
	Name     string
	Pulls    int
	S        int
	A        int
	B        int
	Pity     int // 距上一个 S 级已经抽了多少次
	HardPity int
	Average  float64      // 平均多少抽出一个 S 级
	Records  []pullRecord // 按时间从新到旧
}

// sPull 是一次出 S 级的记录及当时用掉的抽数。
type sPull struct {
	ID       string
	Name     string
	ItemType string
	Channel  string
	Time     string
	Pity     int
}

// gachaSpec 是一款游戏的导出等级与保底规则。
type gachaSpec struct {
	sRank string
	aRank string
	bRank string
	pity  map[string]int // 各频段的 S 级保底抽数，缺省 90
}

var gachaSpecs = map[string]gachaSpec{
	"zzz": {
		sRank: "4",
		aRank: "3",
		bRank: "2",
		pity:  map[string]int{"3": 80, "103": 80, "5": 80},
	},
	"starrail": {
		sRank: "5",
		aRank: "4",
		bRank: "3",
		pity:  map[string]int{"12": 80, "22": 80, "2": 50},
	},
}

// gachaData 是一款游戏抽卡数据的汇总。
type gachaData struct {
	UID        string
	LastTime   string
	TotalPulls int
	TotalS     int
	TotalA     int
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

// loadGameData 从应用数据目录读取一款游戏的抽卡数据。
func loadGameData(game string) (*gachaData, error) {
	root, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		return nil, err
	}
	return loadGachaDir(filepath.Join(root, "data", game), game)
}

// loadZZZData 从应用数据目录读取绝区零的抽卡数据。
func loadZZZData() (*gachaData, error) {
	return loadGameData("zzz")
}

// loadZZZDir 从 dir 读取 config.json 指向的绝区零抽卡文件。
func loadZZZDir(dir string) (*gachaData, error) {
	return loadGachaDir(dir, "zzz")
}

// loadGachaDir 从 dir 读取 config.json 指向的抽卡文件。
func loadGachaDir(dir, game string) (*gachaData, error) {
	path, err := gachaFilePath(dir)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	d, err := parseGacha(data, game)
	if err != nil {
		return nil, err
	}
	// 卡池数据只用于标记歪，读取失败不影响抽卡记录本身。
	if game == "zzz" {
		if set, err := loadZZZBanners(); err == nil {
			annotateZZZLost(d, set.Banners)
		}
	}
	return d, nil
}

// gachaFilePath 找到要读取的抽卡文件：config.json 的 current 账号，
// 否则取最近修改的 gacha-list-*.json。
func gachaFilePath(dir string) (string, error) {
	var cfg struct {
		Current string `json:"current"`
	}
	if data, err := os.ReadFile(filepath.Join(dir, "config.json")); err == nil {
		if json.Unmarshal(data, &cfg) == nil && cfg.Current != "" {
			p := filepath.Join(dir, "gacha-list-"+cfg.Current+".json")
			if _, err := os.Stat(p); err == nil {
				return p, nil
			}
		}
	}
	matches, err := filepath.Glob(filepath.Join(dir, "gacha-list-*.json"))
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("在 %s 中没有找到 gacha-list-*.json", dir)
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
	var exp gachaExport
	if err := json.Unmarshal(data, &exp); err != nil {
		return nil, err
	}
	return newGachaData(&exp, game), nil
}

// parseZZZ 解析绝区零抽卡导出文件。
func parseZZZ(data []byte) (*gachaData, error) {
	return parseGacha(data, "zzz")
}

// parseStarRail 解析崩坏：星穹铁道抽卡导出文件。
func parseStarRail(data []byte) (*gachaData, error) {
	return parseGacha(data, "starrail")
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

	spec := gachaSpecs[game]
	d := &gachaData{UID: exp.UID}
	for _, typ := range order {
		recs := groups[typ]
		if len(recs) == 0 {
			continue
		}
		ch := channelStats{Type: typ, Name: names[typ], HardPity: hardPity(game, typ)}
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
			case spec.sRank:
				tier = 0
			case spec.aRank:
				tier = 1
			case spec.bRank:
				tier = 2
			}
			ch.Records = append(ch.Records, pullRecord{gachaRecord: r, Pity: pity, Tier: tier})
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

// newZZZData 按绝区零频段汇总记录。
func newZZZData(exp *gachaExport) *gachaData {
	return newGachaData(exp, "zzz")
}

// hardPity 是一款游戏各频道的 S 级保底抽数。
func hardPity(game, typ string) int {
	if p, ok := gachaSpecs[game].pity[typ]; ok {
		return p
	}
	return 90
}
