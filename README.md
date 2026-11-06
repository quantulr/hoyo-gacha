<p align="center">
  <img src="resources/icon.png" width="96" height="96" alt="hoyo-gacha">
</p>

<h1 align="center">hoyo-gacha</h1>

<p align="center">原神 · 崩坏：星穹铁道 · 绝区零 抽卡记录管理器</p>

hoyo-gacha 是一款用 Go 和 [MyGo](https://github.com/egoist/mygo) 编写的原生桌面应用。它把三款游戏的抽卡记录集中到一个窗口里，展示抽数、出金率、保底进度和完整记录，并能从本机游戏日志/网页缓存中自动读取最新记录。界面全部由 Go 描述，没有 Web 前端，也没有前端构建步骤。

> 本项目是非官方工具，与米哈游 / HoYoverse 无关联。

## 功能

- **三款游戏**：原神、崩坏：星穹铁道、绝区零，侧边栏切换，`Ctrl/Cmd+1/2/3` 快捷键。
- **抽卡概览**：总抽数、S/A 数量与出货率、平均出金、各卡池保底进度，以及最近 12 次出金。
- **完整记录**：按卡池频段查看，最新在前；支持“仅 S / S+A / 全部”筛选，默认只看 S。
- **自动刷新**：从本机日志/缓存中提取临时 authkey，分页请求官方接口，并按记录 id 与本地数据合并。
- **歪卡标记**：内置历史卡池数据；限定卡池的 S 级如果不是当期 UP，会在记录里显示“歪”。
- **本地优先**：记录保存为 `gacha-list-<uid>.json`，采用原子写入；应用不会上传抽卡数据。

## 支持的游戏

| 游戏 | 游戏内记录页 | 自动刷新 | 歪卡判定范围 |
| --- | --- | :---: | --- |
| 原神 | 「祈愿记录」 | 支持 | 角色活动祈愿、武器活动祈愿 |
| 崩坏：星穹铁道 | 「跃迁记录」 | 支持 | 角色活动跃迁、光锥活动跃迁（含联动） |
| 绝区零 | 「调频记录」 | 支持 | 独家频段、音擎频段 |

常驻、新手和绝区零邦布频段没有 UP 概念；原神集录祈愿的卡池数据暂未收录。以上频段均不参与歪卡判定。

## 使用

### 自动刷新

1. 在游戏内打开上表对应的记录页。这一步会让客户端在日志或网页缓存里写入带临时 authkey 的链接。
2. 切回 hoyo-gacha，选择游戏，点右上角「刷新」。
3. 刷新会依次确认账号、读取本地数据、按频段翻页抓取并合并；完成后概览与记录页立即更新。

如果提示 authkey 失效，重新打开游戏内记录页再刷新即可。authkey 只在本次抓取期间使用，不会被保存。

### 手动导入

如果已有 `gacha-list-<uid>.json`（例如由 genshin-wish-export、star-rail-warp-export 等工具生成），也可以直接放入对应游戏的数据目录：

| 游戏 | 生产版目录 |
| --- | --- |
| 原神 | `%APPDATA%\hoyo-gacha\data\genshin\` |
| 崩坏：星穹铁道 | `%APPDATA%\hoyo-gacha\data\starrail\` |
| 绝区零 | `%APPDATA%\hoyo-gacha\data\zzz\` |

开发版（`go tool mygo dev`）把路径中的 `hoyo-gacha` 换成 `hoyo-gacha Dev`。放入文件后重新启动应用。

同一游戏有多个账号时，每个 UID 各保存一份 `gacha-list-<uid>.json`。应用优先读取 `config.json` 中 `current` 指向的账号；没有配置时读取最近修改的文件。自动刷新与手动导入可以混用，刷新按记录 id 合并，不会丢弃本地已有的其他频段。

## 歪卡判定

对限定卡池中抽到的 S 级（原神/星穹铁道为 5 星，绝区零为 S 级），应用会取记录时间点所有并行开放的同期卡池，把它们的 UP 角色/武器合并成一个集合。抽到的名字不在这个集合里，就标记为「歪」。

- 只判定限定卡池；常驻、新手、原神集录祈愿、绝区零邦布频段不判定。
- 同一时间有多期并行卡池时按并集判断，例如原神 4.0 上半的两期角色祈愿、武器池的两把 UP 武器。
- 星穹铁道的长期联动跃迁按长期开放处理。
- 记录时间不在内置卡池数据覆盖范围内，或当时没有对应卡池数据时，不标记。
- 内置卡池数据是静态的。新版本上线后需要更新对应的 `assets/*-banners.json` 并重新构建，否则新卡池期间的记录不会被标记。
- 卡池时间按 UTC+8 维护；非 UTC+8 服务器的记录在卡池边界附近可能有偏差。

截至 2026-11-06，内置数据覆盖原神 1.0–7.2、崩坏：星穹铁道 1.0–4.7、绝区零 1.0–3.3，包含部分前瞻版本。

## 数据与隐私

| 版本 | 数据根目录 |
| --- | --- |
| 生产构建 | `%APPDATA%\hoyo-gacha` |
| `mygo dev` | `%APPDATA%\hoyo-gacha Dev` |

每款游戏的数据位于 `data\<game>\`：

- `gacha-list-<uid>.json`：抽卡记录。
- `config.json`：当前账号 UID 等配置。

安全说明：

- authkey 是临时凭据，只在刷新时从本机日志/缓存读取并用于官方抽卡记录接口，不打印、不写日志、不持久化；网络错误信息会去掉完整 URL。
- 除官方抽卡记录接口外，应用不会把记录上传到其他服务。
- 仓库不包含任何真实用户数据；测试使用构造数据和假 authkey。
- `gacha-list-<uid>.json` 包含 UID 与完整抽卡历史。分享日志或提交 issue 前请先删除个人数据。

## 构建与开发

需要 Go 1.27.3 或更高版本。自动刷新依赖 Windows 版游戏的日志/缓存目录，建议在 Windows 10/11 上使用。

```sh
go mod download
go tool mygo dev        # 开发模式，热重载；终端按 q 退出
go tool mygo build      # 生产构建，输出到 build/
go test -count=1 ./...  # 全部测试
go tool mygo vet        # go vet + MyGo UI 生命周期检查
```

代码结构：

- `main.go`：应用状态、窗口和共享 UI；`games.go`：游戏注册表。
- `gacha.go`：记录模型、保底、等级与统计；`fetch.go`：共享抓取、合并和原子写入。
- `<game>.go`：游戏描述符与页面；`<game>_fetch.go`：日志/缓存定位与分页抓取。
- `<game>_banners.go`：歪卡判定；`assets/*-banners.json`：内置历史卡池数据。

## 数据来源

- 抽卡记录：米哈游官方抽卡记录接口，通过本机游戏日志/缓存中的临时 authkey 访问。
- 历史卡池：
  - 原神：[Biligame「往期祈愿」](https://wiki.biligame.com/ys/%E5%BE%80%E6%9C%9F%E7%A5%88%E6%84%BF)
  - 崩坏：星穹铁道：[Biligame「历史跃迁」](https://wiki.biligame.com/sr/%E5%8E%86%E5%8F%B2%E8%B7%83%E8%BF%81)
  - 绝区零：[Biligame「调频」](https://wiki.biligame.com/zzz/%E8%B0%83%E9%A2%91)
- 卡池数据以 JSON 内嵌（`go:embed`），运行时不联网获取 wiki。

## 已知限制

- 原神集录祈愿、常驻、新手，星穹铁道常驻/新手，绝区零常驻/邦布频段不判定歪。
- 内置卡池数据不会自动更新；新版本需要更新数据文件后重新构建。
- 自动刷新目前主要针对 Windows 客户端，其他平台未验证。
- 同一游戏多账号时，应用启动读取 `config.json` 的 `current` 账号，界面内暂不提供账号切换。

## 致谢

- [MyGo](https://github.com/egoist/mygo)：原生 Go UI 框架。
- [Biligame Wiki](https://wiki.biligame.com/)：历史卡池数据。
- [genshin-wish-export](https://github.com/biuuu/genshin-wish-export)、[star-rail-warp-export](https://github.com/biuuu/star-rail-warp-export)：抓取流程参考。

本项目仅供个人管理抽卡记录使用。请遵守游戏服务条款；因使用本工具产生的风险由使用者自行承担。
