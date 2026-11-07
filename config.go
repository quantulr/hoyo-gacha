package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"github.com/pelletier/go-toml/v2"
)

// gameConfigName 是应用数据根目录下的统一配置文件。
const gameConfigName = "config.toml"

// gameConfigMu 串行化配置的读取—修改—写入。各游戏在启动时并发 load，
// 读改写不加锁会互相覆盖 current。
var gameConfigMu sync.Mutex

// gameDataDir 返回一款游戏的抽卡数据目录。
func gameDataDir(root, game string) string {
	return filepath.Join(root, "data", game)
}

// gameConfigPath 返回统一配置文件路径。
func gameConfigPath(root string) string {
	return filepath.Join(root, gameConfigName)
}

// loadGameConfig 读取统一配置；文件不存在时返回空配置。
func loadGameConfig(root string) (map[string]any, error) {
	data, err := os.ReadFile(gameConfigPath(root))
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]any{}, nil
	}
	if err != nil {
		return nil, err
	}
	cfg := map[string]any{}
	if len(bytes.TrimSpace(data)) == 0 {
		return cfg, nil
	}
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("%s: %w", gameConfigName, err)
	}
	return cfg, nil
}

// configCurrent 返回某款游戏保存的当前账号。
func configCurrent(cfg map[string]any, game string) string {
	games, _ := cfg["games"].(map[string]any)
	entry, _ := games[game].(map[string]any)
	current, _ := entry["current"].(string)
	return current
}

// setConfigCurrent 写入某款游戏的当前账号，保留其他配置项。
func setConfigCurrent(cfg map[string]any, game, uid string) {
	games, _ := cfg["games"].(map[string]any)
	if games == nil {
		games = map[string]any{}
		cfg["games"] = games
	}
	entry, _ := games[game].(map[string]any)
	if entry == nil {
		entry = map[string]any{}
		games[game] = entry
	}
	entry["current"] = uid
}

// saveGameCurrent 更新统一配置中的当前账号。
func saveGameCurrent(root, game, uid string) error {
	gameConfigMu.Lock()
	defer gameConfigMu.Unlock()
	cfg, err := loadGameConfig(root)
	if err != nil {
		return err
	}
	setConfigCurrent(cfg, game, uid)
	return writeGameConfig(root, cfg)
}

// saveGameCurrentIfChanged 在 current 还不是 uid 时写入配置，
// 避免每次启动都重写没有变化的文件。
func saveGameCurrentIfChanged(root, game, uid string) error {
	gameConfigMu.Lock()
	defer gameConfigMu.Unlock()
	cfg, err := loadGameConfig(root)
	if err != nil {
		return err
	}
	if configCurrent(cfg, game) == uid {
		return nil
	}
	setConfigCurrent(cfg, game, uid)
	return writeGameConfig(root, cfg)
}

// writeGameConfig 原子写入统一配置。
func writeGameConfig(root string, cfg map[string]any) error {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	data, err := toml.Marshal(cfg)
	if err != nil {
		return err
	}
	return writeFileAtomic(gameConfigPath(root), data)
}
