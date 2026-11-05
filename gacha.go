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
	Lost bool // S 级不是当期 UP；常驻、邦布或缺少卡池数据时为 false
}

// channelStats 是一个频段的统计。
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

// zzzData 是绝区零抽卡数据的汇总。
type zzzData struct {
	UID        string
	LastTime   string
	TotalPulls int
	TotalS     int
	TotalA     int
	Channels   []channelStats
	SRecords   []sPull // 按时间从新到旧
}

func (d *zzzData) SRate() float64 {
	if d.TotalPulls == 0 {
		return 0
	}
	return float64(d.TotalS) / float64(d.TotalPulls)
}

func (d *zzzData) ARate() float64 {
	if d.TotalPulls == 0 {
		return 0
	}
	return float64(d.TotalA) / float64(d.TotalPulls)
}

func (d *zzzData) Average() float64 {
	if d.TotalS == 0 {
		return 0
	}
	return float64(d.TotalPulls) / float64(d.TotalS)
}

// loadZZZData 从应用数据目录读取绝区零的抽卡数据。
func loadZZZData() (*zzzData, error) {
	root, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		return nil, err
	}
	return loadZZZDir(filepath.Join(root, "data", "zzz"))
}

// loadZZZDir 从 dir 读取 config.json 指向的抽卡文件。
func loadZZZDir(dir string) (*zzzData, error) {
	path, err := zzzFilePath(dir)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	d, err := parseZZZ(data)
	if err != nil {
		return nil, err
	}
	// 卡池数据只用于标记歪，读取失败不影响抽卡记录本身。
	if set, err := loadZZZBanners(); err == nil {
		annotateZZZLost(d, set.Banners)
	}
	return d, nil
}

// zzzFilePath 找到要读取的抽卡文件：config.json 的 current 账号，
// 否则取最近修改的 gacha-list-*.json。
func zzzFilePath(dir string) (string, error) {
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

// parseZZZ 解析抽卡导出文件并汇总统计。
func parseZZZ(data []byte) (*zzzData, error) {
	var exp gachaExport
	if err := json.Unmarshal(data, &exp); err != nil {
		return nil, err
	}
	return newZZZData(&exp), nil
}

// newZZZData 按频段汇总记录，并算出保底与出金历史。
func newZZZData(exp *gachaExport) *zzzData {
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

	d := &zzzData{UID: exp.UID}
	for _, typ := range order {
		recs := groups[typ]
		if len(recs) == 0 {
			continue
		}
		ch := channelStats{Type: typ, Name: names[typ], HardPity: hardPity(typ)}
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
			ch.Records = append(ch.Records, pullRecord{gachaRecord: r, Pity: pity})
			if r.Time > d.LastTime {
				d.LastTime = r.Time
			}
			switch r.RankType {
			case "4":
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
			case "3":
				ch.A++
				d.TotalA++
			case "2":
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

// hardPity 是各频段的 S 级保底抽数。
func hardPity(typ string) int {
	switch typ {
	case "3", "103", "5": // 音擎、音擎回响、邦布
		return 80
	}
	return 90
}
