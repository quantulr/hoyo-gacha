package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// 绝区零限定频段历史，数据取自 Biligame wiki「调频」页。
//
//go:embed assets/zzz-banners.json
var zzzBannerJSON []byte

// zzzBanner 是一期限定频段（独家频段或音擎频段）。
type zzzBanner struct {
	Version string   `json:"version"` // 1.0、3.3 等
	Phase   string   `json:"phase"`   // 上半、下半、全版本
	Channel string   `json:"channel"` // character（独家）或 engine（音擎）
	Title   string   `json:"title"`
	Start   string   `json:"start"` // UTC+8，YYYY-MM-DD HH:MM:SS
	End     string   `json:"end"`   // UTC+8
	UP      []string `json:"up"`
}

// zzzBannerSet 是嵌入的卡池数据文件。
type zzzBannerSet struct {
	Source   string      `json:"source"`
	Timezone string      `json:"timezone"`
	Updated  string      `json:"updated"`
	Note     string      `json:"note"`
	Banners  []zzzBanner `json:"banners"`
}

const (
	zzzBannerLayout = "2006-01-02 15:04:05"
	// 国服偶尔提前开服，抽卡记录可能早于官方开池时间数小时。
	zzzBannerEarlyOpen = 6 * time.Hour
)

var zzzBannerLocation = time.FixedZone("UTC+8", 8*60*60)

// loadZZZBanners 解析并校验内嵌的卡池数据。
func loadZZZBanners() (*zzzBannerSet, error) {
	var set zzzBannerSet
	if err := json.Unmarshal(zzzBannerJSON, &set); err != nil {
		return nil, fmt.Errorf("解析卡池数据：%w", err)
	}
	if len(set.Banners) == 0 {
		return nil, fmt.Errorf("卡池数据为空")
	}
	for i, b := range set.Banners {
		start, err := zzzBannerTime(b.Start)
		if err != nil {
			return nil, fmt.Errorf("第 %d 个卡池开始时间：%w", i+1, err)
		}
		end, err := zzzBannerTime(b.End)
		if err != nil {
			return nil, fmt.Errorf("第 %d 个卡池结束时间：%w", i+1, err)
		}
		if !end.After(start) {
			return nil, fmt.Errorf("第 %d 个卡池结束时间不晚于开始时间", i+1)
		}
		if b.Channel != "character" && b.Channel != "engine" {
			return nil, fmt.Errorf("第 %d 个卡池频道 %q 无效", i+1, b.Channel)
		}
		if len(b.UP) == 0 {
			return nil, fmt.Errorf("第 %d 个卡池没有 UP", i+1)
		}
	}
	return &set, nil
}

func zzzBannerTime(s string) (time.Time, error) {
	return time.ParseInLocation(zzzBannerLayout, s, zzzBannerLocation)
}

// zzzBannersAt 返回 channel 在 t 时刻开放的卡池。若 t 落在版本更新前的
// 空档、且距离开池不足 zzzBannerEarlyOpen，则按提前开服算作已开放。
func zzzBannersAt(banners []zzzBanner, channel string, t time.Time) []zzzBanner {
	var active []zzzBanner
	var early []zzzBanner
	var earliest time.Time
	for _, b := range banners {
		if b.Channel != channel {
			continue
		}
		start, err := zzzBannerTime(b.Start)
		if err != nil {
			continue
		}
		end, err := zzzBannerTime(b.End)
		if err != nil {
			continue
		}
		if !t.Before(start) && !t.After(end) {
			active = append(active, b)
			continue
		}
		if t.Before(start) && start.Sub(t) <= zzzBannerEarlyOpen {
			if earliest.IsZero() || start.Before(earliest) {
				earliest = start
				early = early[:0]
			}
			if start.Equal(earliest) {
				early = append(early, b)
			}
		}
	}
	if len(active) > 0 {
		return active
	}
	return early
}

// zzzBannerChannel 把抽卡频道映射到卡池频道。常驻、邦布频段没有 UP，返回空。
func zzzBannerChannel(typ string) string {
	switch typ {
	case "2", "102": // 独家频段、独家重映
		return "character"
	case "3", "103": // 音擎频段、音擎回响
		return "engine"
	}
	return ""
}

// zzzBannerUP 判断 name 在 t 时刻是否为 channel 的当期 UP。
// ok 为 false 表示当时没有卡池数据，无法判断。
func zzzBannerUP(banners []zzzBanner, typ, name string, t time.Time) (up, ok bool) {
	channel := zzzBannerChannel(typ)
	if channel == "" {
		return false, false
	}
	active := zzzBannersAt(banners, channel, t)
	if len(active) == 0 {
		return false, false
	}
	name = zzzBannerName(name)
	for _, b := range active {
		for _, u := range b.UP {
			if zzzBannerName(u) == name {
				return true, true
			}
		}
	}
	return false, true
}

// zzzBannerName 去掉导出记录里可能包裹名字的「」。
func zzzBannerName(s string) string {
	return strings.Trim(strings.TrimSpace(s), "「」")
}

// annotateZZZLost 标记限定频段里不是当期 UP 的 S 级记录。
func annotateZZZLost(d *gachaData, banners []zzzBanner) {
	for i := range d.Channels {
		ch := &d.Channels[i]
		if zzzBannerChannel(ch.Type) == "" {
			continue
		}
		guaranteed := false
		// 记录最新在前，倒序遍历才能按时间顺序判断大保底。
		for j := len(ch.Records) - 1; j >= 0; j-- {
			r := &ch.Records[j]
			if r.Tier != 0 {
				continue
			}
			var up, known bool
			if t, err := zzzBannerTime(r.Time); err == nil {
				up, known = zzzBannerUP(banners, ch.Type, r.Name, t)
			}
			guaranteed = markBannerPull(r, guaranteed, known, up)
		}
		ch.countLost()
	}
}
