package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// 崩坏：星穹铁道限定跃迁历史，数据取自 Biligame wiki「历史跃迁」页。
//
//go:embed assets/starrail-banners.json
var starRailBannerJSON []byte

// starRailBanner 是一期限定跃迁（角色活动跃迁或光锥活动跃迁）。
type starRailBanner struct {
	Version string   `json:"version"` // 1.0、4.7 等
	Phase   string   `json:"phase"`   // 上半、下半、额外
	Channel string   `json:"channel"` // character（角色）或 lightcone（光锥）
	Title   string   `json:"title"`
	Start   string   `json:"start"` // UTC+8，YYYY-MM-DD HH:MM:SS
	End     string   `json:"end"`   // UTC+8；长期开放的联动跃迁记为 9999-12-31
	UP      []string `json:"up"`
}

// starRailBannerSet 是嵌入的卡池数据文件。
type starRailBannerSet struct {
	Source   string           `json:"source"`
	Timezone string           `json:"timezone"`
	Updated  string           `json:"updated"`
	Note     string           `json:"note"`
	Banners  []starRailBanner `json:"banners"`
}

const starRailBannerLayout = "2006-01-02 15:04:05"

var (
	starRailBannerLocation = time.FixedZone("UTC+8", 8*60*60)
	starRailVersionRE      = regexp.MustCompile(`^\d+\.\d+$`)
	// 导出记录与 wiki 对间隔号、全角符号的写法可能不同。
	starRailNameReplacer = strings.NewReplacer(
		" ", " ", // 不间断空格
		"　", " ", // 全角空格
		"·", "•", // ·
		"・", "•", // ・
		"‧", "•", // ‧
		"＆", "&", // ＆
	)
)

// loadStarRailBanners 解析并校验内嵌的卡池数据。
func loadStarRailBanners() (*starRailBannerSet, error) {
	var set starRailBannerSet
	if err := json.Unmarshal(starRailBannerJSON, &set); err != nil {
		return nil, fmt.Errorf("解析卡池数据：%w", err)
	}
	if len(set.Banners) == 0 {
		return nil, fmt.Errorf("卡池数据为空")
	}
	for i, b := range set.Banners {
		if !starRailVersionRE.MatchString(b.Version) {
			return nil, fmt.Errorf("第 %d 个卡池版本 %q 无效", i+1, b.Version)
		}
		switch b.Phase {
		case "上半", "下半", "额外":
		default:
			return nil, fmt.Errorf("第 %d 个卡池阶段 %q 无效", i+1, b.Phase)
		}
		start, err := starRailBannerTime(b.Start)
		if err != nil {
			return nil, fmt.Errorf("第 %d 个卡池开始时间：%w", i+1, err)
		}
		end, err := starRailBannerTime(b.End)
		if err != nil {
			return nil, fmt.Errorf("第 %d 个卡池结束时间：%w", i+1, err)
		}
		if !end.After(start) {
			return nil, fmt.Errorf("第 %d 个卡池结束时间不晚于开始时间", i+1)
		}
		if b.Channel != "character" && b.Channel != "lightcone" {
			return nil, fmt.Errorf("第 %d 个卡池频道 %q 无效", i+1, b.Channel)
		}
		if len(b.UP) == 0 {
			return nil, fmt.Errorf("第 %d 个卡池没有 UP", i+1)
		}
	}
	return &set, nil
}

func starRailBannerTime(s string) (time.Time, error) {
	return time.ParseInLocation(starRailBannerLayout, s, starRailBannerLocation)
}

// starRailBannersAt 返回 channel 在 t 时刻开放的卡池。同一时刻可能有多个
// 并行卡池，例如上半的两期角色跃迁，或长期开放的联动跃迁。
func starRailBannersAt(banners []starRailBanner, channel string, t time.Time) []starRailBanner {
	var active []starRailBanner
	for _, b := range banners {
		if b.Channel != channel {
			continue
		}
		start, err := starRailBannerTime(b.Start)
		if err != nil {
			continue
		}
		end, err := starRailBannerTime(b.End)
		if err != nil {
			continue
		}
		if !t.Before(start) && !t.After(end) {
			active = append(active, b)
		}
	}
	return active
}

// starRailBannerChannel 把跃迁频道映射到卡池频道。常驻、新手跃迁没有 UP，返回空。
func starRailBannerChannel(typ string) string {
	switch typ {
	case "11", "21": // 角色活动跃迁、角色联动跃迁
		return "character"
	case "12", "22": // 光锥活动跃迁、光锥联动跃迁
		return "lightcone"
	}
	return ""
}

// starRailBannerUP 判断 name 在 t 时刻是否为 channel 的当期 UP。
// ok 为 false 表示当时没有卡池数据，无法判断。
func starRailBannerUP(banners []starRailBanner, typ, name string, t time.Time) (up, ok bool) {
	channel := starRailBannerChannel(typ)
	if channel == "" {
		return false, false
	}
	active := starRailBannersAt(banners, channel, t)
	if len(active) == 0 {
		return false, false
	}
	name = starRailBannerName(name)
	for _, b := range active {
		for _, u := range b.UP {
			if starRailBannerName(u) == name {
				return true, true
			}
		}
	}
	return false, true
}

// starRailBannerName 统一导出记录与 wiki 里的名字：去掉包裹的「」，
// 并归一化不同写法的不间断空格、间隔号和全角 &。
func starRailBannerName(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "「」")
	s = starRailNameReplacer.Replace(s)
	return strings.Join(strings.Fields(s), " ")
}

// annotateStarRailLost 标记限定跃迁里不是当期 UP 的 S 级记录。
func annotateStarRailLost(d *gachaData, banners []starRailBanner) {
	for i := range d.Channels {
		ch := &d.Channels[i]
		if starRailBannerChannel(ch.Type) == "" {
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
			if t, err := starRailBannerTime(r.Time); err == nil {
				up, known = starRailBannerUP(banners, ch.Type, r.Name, t)
			}
			guaranteed = markBannerPull(r, guaranteed, known, up)
		}
		ch.countLost()
	}
}
