package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// 原神限定祈愿历史，数据取自 Biligame wiki「往期祈愿」页。
// 该页只覆盖角色活动祈愿与武器活动祈愿，常驻、新手与集录祈愿不在其中。
//
//go:embed assets/genshin-banners.json
var genshinBannerJSON []byte

// genshinBanner 是一期限定祈愿（角色活动祈愿或武器活动祈愿）。
type genshinBanner struct {
	Version string   `json:"version"` // 1.0、月之一、7.2 等
	Phase   string   `json:"phase"`   // 上半、中、下半
	Channel string   `json:"channel"` // character（角色）或 weapon（武器）
	Title   string   `json:"title"`
	Start   string   `json:"start"` // UTC+8，YYYY-MM-DD HH:MM:SS
	End     string   `json:"end"`   // UTC+8
	UP      []string `json:"up"`
}

// genshinBannerSet 是嵌入的祈愿数据文件。
type genshinBannerSet struct {
	Source   string          `json:"source"`
	Timezone string          `json:"timezone"`
	Updated  string          `json:"updated"`
	Note     string          `json:"note"`
	Banners  []genshinBanner `json:"banners"`
}

const genshinBannerLayout = "2006-01-02 15:04:05"

var (
	genshinBannerLocation = time.FixedZone("UTC+8", 8*60*60)
	genshinNameReplacer   = strings.NewReplacer(
		" ", " ", // 不间断空格
		"　", " ", // 全角空格
	)
)

// loadGenshinBanners 解析并校验内嵌的祈愿数据。
func loadGenshinBanners() (*genshinBannerSet, error) {
	var set genshinBannerSet
	if err := json.Unmarshal(genshinBannerJSON, &set); err != nil {
		return nil, fmt.Errorf("解析祈愿数据：%w", err)
	}
	if len(set.Banners) == 0 {
		return nil, fmt.Errorf("祈愿数据为空")
	}
	for i, b := range set.Banners {
		if b.Version == "" {
			return nil, fmt.Errorf("第 %d 个祈愿缺少版本", i+1)
		}
		switch b.Phase {
		case "上半", "中", "下半":
		default:
			return nil, fmt.Errorf("第 %d 个祈愿阶段 %q 无效", i+1, b.Phase)
		}
		start, err := genshinBannerTime(b.Start)
		if err != nil {
			return nil, fmt.Errorf("第 %d 个祈愿开始时间：%w", i+1, err)
		}
		end, err := genshinBannerTime(b.End)
		if err != nil {
			return nil, fmt.Errorf("第 %d 个祈愿结束时间：%w", i+1, err)
		}
		if !end.After(start) {
			return nil, fmt.Errorf("第 %d 个祈愿结束时间不晚于开始时间", i+1)
		}
		if b.Channel != "character" && b.Channel != "weapon" {
			return nil, fmt.Errorf("第 %d 个祈愿频道 %q 无效", i+1, b.Channel)
		}
		if len(b.UP) == 0 {
			return nil, fmt.Errorf("第 %d 个祈愿没有 UP", i+1)
		}
	}
	return &set, nil
}

func genshinBannerTime(s string) (time.Time, error) {
	return time.ParseInLocation(genshinBannerLayout, s, genshinBannerLocation)
}

// genshinBannersAt 返回 channel 在 t 时刻开放的祈愿。同一时刻可能有多期
// 并行的角色祈愿，武器祈愿则在同一期里有两个 5 星 UP。
func genshinBannersAt(banners []genshinBanner, channel string, t time.Time) []genshinBanner {
	var active []genshinBanner
	for _, b := range banners {
		if b.Channel != channel {
			continue
		}
		start, err := genshinBannerTime(b.Start)
		if err != nil {
			continue
		}
		end, err := genshinBannerTime(b.End)
		if err != nil {
			continue
		}
		if !t.Before(start) && !t.After(end) {
			active = append(active, b)
		}
	}
	return active
}

// genshinBannerChannel 把抽卡频道映射到祈愿频道。500 集录祈愿不在本页数据里，
// 200 常驻和 100 新手没有 UP，均返回空。
func genshinBannerChannel(typ string) string {
	switch typ {
	case "301": // 角色活动祈愿
		return "character"
	case "302": // 武器活动祈愿
		return "weapon"
	}
	return ""
}

// genshinBannerUP 判断 name 在 t 时刻是否为 channel 的当期 UP。
// ok 为 false 表示当时没有祈愿数据，无法判断。
func genshinBannerUP(banners []genshinBanner, typ, name string, t time.Time) (up, ok bool) {
	channel := genshinBannerChannel(typ)
	if channel == "" {
		return false, false
	}
	active := genshinBannersAt(banners, channel, t)
	if len(active) == 0 {
		return false, false
	}
	name = genshinBannerName(name)
	for _, b := range active {
		for _, u := range b.UP {
			if genshinBannerName(u) == name {
				return true, true
			}
		}
	}
	return false, true
}

// genshinBannerName 统一导出记录与 wiki 里的名字：去掉包裹的「」，
// 并归一化不同写法的不间断空格与全角空格。
func genshinBannerName(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "「」")
	s = genshinNameReplacer.Replace(s)
	return strings.Join(strings.Fields(s), " ")
}

// annotateGenshinLoaded 给刚读入的原神记录标记歪。集录祈愿缺少本页数据，
// 暂不判定。
func annotateGenshinLoaded(d *gachaData) {
	if set, err := loadGenshinBanners(); err == nil {
		annotateGenshinLost(d, set.Banners)
	}
}

// annotateGenshinLost 标记限定祈愿里不是当期 UP 的 5 星记录。
func annotateGenshinLost(d *gachaData, banners []genshinBanner) {
	for i := range d.Channels {
		ch := &d.Channels[i]
		if genshinBannerChannel(ch.Type) == "" {
			continue
		}
		ch.LostTracked = true
		guaranteed := false
		// 记录最新在前，倒序遍历才能按时间顺序判断大保底。
		for j := len(ch.Records) - 1; j >= 0; j-- {
			r := &ch.Records[j]
			if r.Tier != 0 {
				continue
			}
			var up, known bool
			if t, err := genshinBannerTime(r.Time); err == nil {
				up, known = genshinBannerUP(banners, ch.Type, r.Name, t)
			}
			guaranteed = markBannerPull(r, guaranteed, known, up)
		}
		ch.countLost()
	}
}
