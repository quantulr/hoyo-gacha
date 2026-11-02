package main

import (
	"embed"

	"github.com/egoist/mygo/ui"
)

// 应用内使用的游戏图标，128×128 PNG，足以覆盖 16–56 DIP 的高分屏显示。
//
//go:embed assets/*.png
var assetFiles embed.FS

// gameArt 读取并解码内嵌的游戏图标。
func gameArt(name string) *ui.Bitmap {
	data, err := assetFiles.ReadFile("assets/" + name)
	if err != nil {
		panic("hoyo-gacha: " + err.Error())
	}
	b, err := ui.DecodeBitmap(data)
	if err != nil {
		panic("hoyo-gacha: " + err.Error())
	}
	return b
}
