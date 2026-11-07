package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
)

// importGacha 选择并导入当前游戏的 gacha-list-*.json。
func (a *app) importGacha(g *game) {
	if a.importing || g.load == nil || g.apply == nil {
		return
	}
	a.importing = true
	a.toast = ""
	win := a.win
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent:      win,
			Title:       "导入" + g.name + "抽卡记录",
			ButtonLabel: "导入",
			Filters:     []mygo.FileFilter{{Name: "抽卡记录", Extensions: []string{"json"}}},
		})
		if err != nil {
			a.publish(func() {
				a.importing = false
				a.toast = "导入失败：" + err.Error()
			})
			return
		}
		if len(paths) == 0 {
			a.publish(func() { a.importing = false })
			return
		}
		root, err := mygo.App.Path(mygo.PathUserData)
		var added int
		if err == nil {
			added, err = importGachaFile(filepath.Join(root, "data", g.id), g, paths[0])
		}
		a.publish(func() {
			a.importing = false
			if err != nil {
				a.toast = "导入失败：" + err.Error()
				return
			}
			d, loadErr := g.load()
			g.apply(a, d, loadErr)
			if loadErr != nil {
				a.toast = "导入成功，但读取数据失败：" + loadErr.Error()
				return
			}
			a.toast = fmt.Sprintf("已导入 %d 条记录", added)
		})
	}()
}

// importGachaFile 读取 src，把其中的 UID 和记录写入 dir/gacha-list-<uid>.json，
// 并更新 config.json 的 current。返回导入的记录数。
func importGachaFile(dir string, g *game, src string) (int, error) {
	data, err := os.ReadFile(src)
	if err != nil {
		return 0, err
	}
	exp, err := parseGachaExport(data)
	if err != nil {
		return 0, fmt.Errorf("文件不是有效的抽卡记录：%w", err)
	}
	if exp.UID == "" {
		exp.UID = gachaUIDFromPath(src)
	}
	if !validGachaUID(exp.UID) {
		return 0, errors.New("文件中缺少有效的 UID")
	}
	if !g.acceptsImport(exp) {
		return 0, errors.New("文件中没有该游戏的抽卡记录，请确认选择了正确的文件")
	}
	d := newGachaData(exp, g.id)
	if err := saveGachaExport(dir, exp); err != nil {
		return 0, err
	}
	return d.TotalPulls, nil
}

// acceptsImport 检查文件里是否有该游戏支持的卡池记录，避免选错游戏的文件。
func (g *game) acceptsImport(exp *gachaExport) bool {
	known := make(map[string]bool, len(g.channels))
	for _, typ := range g.channels {
		known[typ] = true
	}
	if len(known) == 0 {
		return true
	}
	for _, group := range exp.Result {
		if len(group.Records) > 0 && known[group.Type] {
			return true
		}
	}
	return false
}

// validGachaUID 只接受数字 UID，避免用文件里的 UID 拼出意外路径。
func validGachaUID(uid string) bool {
	if uid == "" || len(uid) > 20 {
		return false
	}
	for i := 0; i < len(uid); i++ {
		if uid[i] < '0' || uid[i] > '9' {
			return false
		}
	}
	return true
}
