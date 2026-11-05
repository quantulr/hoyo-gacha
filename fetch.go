package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// apiGachaRecord 是接口返回的一条记录，比导出文件多一个 uid 字段。
type apiGachaRecord struct {
	UID string `json:"uid"`
	gachaRecord
}

// UnmarshalJSON 分开解析 uid 和记录字段。gachaRecord 自带 UnmarshalJSON，
// 嵌入后会推广到整个结构体，直接解析会丢掉 uid。
func (r *apiGachaRecord) UnmarshalJSON(data []byte) error {
	var meta struct {
		UID string `json:"uid"`
	}
	if err := json.Unmarshal(data, &meta); err != nil {
		return err
	}
	var rec gachaRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return err
	}
	r.UID, r.gachaRecord = meta.UID, rec
	return nil
}

// gachaLogData 是接口响应里的 data 部分。
type gachaLogData struct {
	List           []apiGachaRecord `json:"list"`
	Region         string           `json:"region"`
	RegionTimeZone int              `json:"region_time_zone"`
}

// gachaRefreshResult 是一次刷新的结果，用来提示用户更新了多少条。
type gachaRefreshResult struct {
	UID   string
	Added int
	Total int
}

func (r *gachaRefreshResult) Message() string {
	if r.Added == 0 {
		return fmt.Sprintf("已是最新，共 %s 条记录", formatInt(r.Total))
	}
	return fmt.Sprintf("已更新 %s 条，共 %s 条记录", formatInt(r.Added), formatInt(r.Total))
}

// gachaLatestIDs 返回本地各频段最大的记录 id。
func gachaLatestIDs(exp *gachaExport) map[string]*big.Int {
	out := make(map[string]*big.Int)
	if exp == nil {
		return out
	}
	for _, group := range exp.Result {
		for _, rec := range group.Records {
			v, ok := new(big.Int).SetString(rec.ID, 10)
			if !ok {
				continue
			}
			if cur, ok := out[group.Type]; !ok || v.Cmp(cur) > 0 {
				out[group.Type] = v
			}
		}
	}
	return out
}

func gachaIDEquals(id string, n *big.Int) bool {
	v, ok := new(big.Int).SetString(id, 10)
	return ok && v.Cmp(n) == 0
}

// convertGachaTime 把服务器本地时间换算到账号记录的时区。
func convertGachaTime(s string, from, to int) string {
	if from == to {
		return s
	}
	t, err := time.Parse("2006-01-02 15:04:05", s)
	if err != nil {
		return s
	}
	t = t.Add(-time.Duration(from) * time.Hour).Add(time.Duration(to) * time.Hour)
	return t.Format("2006-01-02 15:04:05")
}

// loadGachaLocal 读取同一 UID 已有的导出文件，没有则返回 nil。
func loadGachaLocal(dir, uid string) (*gachaExport, error) {
	path := filepath.Join(dir, "gacha-list-"+uid+".json")
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var exp gachaExport
	if err := json.Unmarshal(data, &exp); err != nil {
		return nil, fmt.Errorf("本地数据 %s 无法解析：%w", filepath.Base(path), err)
	}
	if exp.UID == "" {
		exp.UID = uid
	}
	return &exp, nil
}

// mergeGachaExport 按记录 id 去重合并本地与抓取到的数据，按 id 升序排列。
// order 与 names 是这款游戏的固定频段顺序和名称；返回值里 Added 是抓取到的、
// 本地没有的记录数。
func mergeGachaExport(local, fetched *gachaExport, baseOrder []string, baseNames map[string]string,
	fallbackPrefix string) (*gachaExport, int) {
	if fetched == nil {
		fetched = &gachaExport{}
	}
	names := make(map[string]string, len(baseNames))
	for typ, name := range baseNames {
		names[typ] = name
	}
	for _, pair := range fetched.TypeMap {
		if len(pair) == 2 && pair[1] != "" {
			names[pair[0]] = pair[1]
		}
	}
	if local != nil {
		for _, pair := range local.TypeMap {
			if len(pair) == 2 && pair[1] != "" {
				names[pair[0]] = pair[1]
			}
		}
	}

	order := append([]string{}, baseOrder...)
	known := make(map[string]bool, len(order))
	for _, typ := range order {
		known[typ] = true
	}
	var extra []string
	addType := func(typ string) {
		if typ == "" || known[typ] {
			return
		}
		known[typ] = true
		extra = append(extra, typ)
	}
	if local != nil {
		for _, group := range local.Result {
			addType(group.Type)
		}
	}
	for _, group := range fetched.Result {
		addType(group.Type)
	}
	sort.Strings(extra)
	order = append(order, extra...)

	localGroups := map[string][]gachaRecord{}
	if local != nil {
		for _, group := range local.Result {
			localGroups[group.Type] = append(localGroups[group.Type], group.Records...)
		}
	}
	fetchedGroups := map[string][]gachaRecord{}
	for _, group := range fetched.Result {
		fetchedGroups[group.Type] = append(fetchedGroups[group.Type], group.Records...)
	}
	localIDs := make(map[string]map[string]bool, len(localGroups))
	for typ, recs := range localGroups {
		set := make(map[string]bool, len(recs))
		for _, rec := range recs {
			set[rec.ID] = true
		}
		localIDs[typ] = set
	}

	merged := &gachaExport{
		UID:            fetched.UID,
		Lang:           fetched.Lang,
		RegionTimeZone: fetched.RegionTimeZone,
	}
	if local != nil {
		if merged.UID == "" {
			merged.UID = local.UID
		}
		if merged.Lang == "" {
			merged.Lang = local.Lang
		}
		if merged.RegionTimeZone == 0 {
			merged.RegionTimeZone = local.RegionTimeZone
		}
	}

	added := 0
	for _, typ := range order {
		seen := map[string]bool{}
		var list []gachaRecord
		for _, rec := range fetchedGroups[typ] {
			if rec.ID != "" && seen[rec.ID] {
				continue
			}
			if rec.ID != "" {
				seen[rec.ID] = true
			}
			if rec.Count == "" {
				rec.Count = "1"
			}
			if localIDs[typ] == nil || !localIDs[typ][rec.ID] {
				added++
			}
			list = append(list, rec)
		}
		for _, rec := range localGroups[typ] {
			if rec.ID != "" && seen[rec.ID] {
				continue
			}
			if rec.ID != "" {
				seen[rec.ID] = true
			}
			if rec.Count == "" {
				rec.Count = "1"
			}
			list = append(list, rec)
		}
		sortGachaRecords(list)
		name := names[typ]
		if name == "" {
			name = fallbackPrefix + typ
		}
		merged.Result = append(merged.Result, gachaGroup{Type: typ, Records: list})
		merged.TypeMap = append(merged.TypeMap, [2]string{typ, name})
	}
	return merged, added
}

// sortGachaRecords 按数字大小的 id 升序排列；解析失败时退回字符串比较。
func sortGachaRecords(recs []gachaRecord) {
	sort.SliceStable(recs, func(i, j int) bool {
		a, okA := new(big.Int).SetString(recs[i].ID, 10)
		b, okB := new(big.Int).SetString(recs[j].ID, 10)
		if okA && okB {
			return a.Cmp(b) < 0
		}
		return recs[i].ID < recs[j].ID
	})
}

// saveGachaExport 写入 gacha-list-<uid>.json，并把 config.json 的 current 指向该账号。
func saveGachaExport(dir string, exp *gachaExport) error {
	if exp.UID == "" {
		return errors.New("抽卡数据缺少 UID")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(exp); err != nil {
		return err
	}
	if err := writeFileAtomic(filepath.Join(dir, "gacha-list-"+exp.UID+".json"), buf.Bytes()); err != nil {
		return err
	}
	return saveGachaCurrent(dir, exp.UID)
}

// saveGachaCurrent 更新 config.json 的当前账号，保留其他配置项。
func saveGachaCurrent(dir, uid string) error {
	path := filepath.Join(dir, "config.json")
	cfg := map[string]any{}
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &cfg)
	}
	cfg["current"] = uid
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	return writeFileAtomic(path, data)
}

// sanitizeURLError 剥掉 net/url 错误里的完整链接，避免 authkey 出现在界面或日志里。
func sanitizeURLError(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		return ue.Err
	}
	return err
}

// writeFileAtomic 先写临时文件再改名，避免中途失败留下半个文件。
func writeFileAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".gacha-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}
