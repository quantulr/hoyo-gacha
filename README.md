<p align="center">
  <img src="resources/icon.png" width="96" height="96" alt="Hoyo Gacha">
</p>

<h1 align="center">Hoyo Gacha</h1>

<p align="center">原神 · 崩坏：星穹铁道 · 绝区零 抽卡记录管理器</p>

Hoyo Gacha 是一款用 Go 和 [MyGo](https://github.com/egoist/mygo) 编写的原生桌面应用。它把三款游戏的抽卡记录集中到一个窗口里，展示抽数、出金率、保底进度和完整记录，并能从本机游戏日志/网页缓存中自动读取最新记录。界面全部由 Go 描述，没有 Web 前端，也没有前端构建步骤。

> 本项目是非官方工具，与米哈游 / HoYoverse 无关联。

## 功能

- **三款游戏**：原神、崩坏：星穹铁道、绝区零，侧边栏切换，`Ctrl/Cmd+1/2/3` 快捷键。
- **抽卡概览**：总抽数、5 星/4 星数量与出货率（绝区零为 S/A）、平均出金、各卡池保底进度，以及最近 12 次出金。
- **完整记录**：按卡池频段查看，最新在前；原神/星穹铁道支持“仅 5 星 / 5+4 星 / 全部”筛选，绝区零为“仅 S / S+A / 全部”，默认只看最高等级。
- **自动刷新**：从本机日志/缓存中提取临时 authkey，分页请求官方接口，并按记录 id 与本地数据合并。
- **一键导入**：点右上角「导入」选择 `gacha-list-<uid>.json`，应用会写入数据目录并把该账号设为当前账号；空状态也可以直接导入。
- **歪卡标记**：内置历史卡池数据；限定卡池的最高等级（原神/星穹铁道为 5 星，绝区零为 S 级）如果不是当期 UP，会在记录里显示“歪”，之后触发的大保底记录显示“大保底”；概览中统计总歪卡率和各卡池歪卡率（大保底不计入分母）。
- **自动更新**：启动后检查 GitHub Release 上的签名更新，下载后下次启动生效；发布与签名校验由 GitHub Actions 完成。
- **设置窗口**：侧边栏底部打开独立的设置窗口，可手动检查更新、打开数据目录，并在「关于」中查看开源项目致谢。
- **本地优先**：记录保存为 `gacha-list-<uid>.json`，采用原子写入；应用不会上传抽卡数据。

## 支持的游戏

| 游戏 | 游戏内记录页 | 自动刷新 | 歪卡判定范围 |
| --- | --- | :---: | --- |
| 原神 | 「祈愿记录」/「颂愿」 | 支持 | 角色活动祈愿、武器活动祈愿 |
| 崩坏：星穹铁道 | 「跃迁记录」 | 支持 | 角色活动跃迁、光锥活动跃迁（含联动） |
| 绝区零 | 「调频记录」 | 支持 | 独家频段、音擎频段 |

原神的千星奇域颂愿分为「千星奇域·活动颂愿」（5/4/3 星，最高等级保底 70 抽）和「千星奇域·常驻颂愿」（4/3/2 星，最高 4 星，保底 70 抽），两者也会随「刷新」一起抓取。

常驻、新手和绝区零邦布频段没有 UP 概念；原神集录祈愿的卡池数据暂未收录。以上频段以及千星奇域颂愿均不参与歪卡判定。

## 使用

### 自动刷新

1. 在游戏内打开上表对应的记录页。这一步会让客户端在日志或网页缓存里写入带临时 authkey 的链接。
2. 切回 Hoyo Gacha，选择游戏。还没有记录时点页面中央的「刷新记录」，已有记录时点右上角「刷新」。
3. 刷新会依次确认账号、读取本地数据、按频段翻页抓取并合并；完成后概览与记录页立即更新。

如果提示 authkey 失效，重新打开游戏内记录页再刷新即可。authkey 只在本次抓取期间使用，不会被保存。

### 手动导入

如果已有 `gacha-list-<uid>.json`（例如由 genshin-wish-export、star-rail-warp-export 等工具生成），选择对应游戏后点右上角「导入」并选中文件即可。应用会把文件写入该游戏的数据目录，并将 `config.toml` 中对应游戏的 `current` 指向文件中的 UID；同 UID 的旧文件会被覆盖。

也可以手动放入对应游戏的数据目录后重新启动：

| 游戏 | 生产版目录 |
| --- | --- |
| 原神 | `%APPDATA%\Hoyo Gacha\data\genshin\` |
| 崩坏：星穹铁道 | `%APPDATA%\Hoyo Gacha\data\starrail\` |
| 绝区零 | `%APPDATA%\Hoyo Gacha\data\zzz\` |

开发版（`go tool mygo dev`）使用 `%APPDATA%\Hoyo Gacha Dev`。应用会读取文件中的 UID，并写入数据根目录 `config.toml` 中对应游戏的 `current`。

`config.toml` 用 `[games.<game>]` 段统一保存各游戏配置：

```toml
[games.genshin]
current = "202"

[games.starrail]
current = "101"
```

旧版的 `data\<game>\config.json` 仍会被读取一次，并在下次保存时自动迁移到 `config.toml`。

0.3.0 及更早版本的数据目录是 `%APPDATA%\hoyo-gacha`。升级后把该目录里的文件复制到 `%APPDATA%\Hoyo Gacha` 即可保留已有记录。

如果数据目录里还没有 `gacha-list-*.json`，应用会提示导入文件，或先在游戏内打开记录页后点「刷新」获取。

同一游戏有多个账号时，每个 UID 各保存一份 `gacha-list-<uid>.json`。应用优先读取 `config.toml` 中 `current` 指向的账号；没有配置时读取最近修改的文件。自动刷新与手动导入可以混用，刷新按记录 id 合并，不会丢弃本地已有的其他频段。

## 自动更新与发布

生产版启动约 10 秒后检查一次更新，之后每天检查。发现新版本时会显示更新说明，可下载并重启应用；开发版和无法写入安装目录的构建不会自动更新。

更新只接受使用 `MYGO_UPDATER_PRIVATE_KEY` 对应私钥签名的包，公钥内嵌在 `mygo.json` 的 `updates.publicKey`。私钥只应保存在密码管理器和 GitHub Actions Secret 中，不要提交到仓库。

发布新版本：

1. 更新 `mygo.json` 的 `version`，并在 `CHANGELOG.md` 添加同版本段落。
2. 提交后创建并推送 tag，例如 `git tag v0.1.0 && git push origin v0.1.0`。
3. `.github/workflows/release.yml` 会构建 Windows amd64/arm64 包，上传到 draft release，并在全部构建成功后自动发布。

首次发布前需要添加仓库 Secret：

```sh
gh secret set MYGO_UPDATER_PRIVATE_KEY < path/to/mygo-update.key
```

`mygo keygen` 默认把密钥写到 `%APPDATA%\mygo\update-keys\`。没有该 Secret，工作流无法签名更新包。

## 歪卡判定

对限定卡池中抽到的最高等级（原神/星穹铁道为 5 星，绝区零为 S 级），应用会取记录时间点所有并行开放的同期卡池，把它们的 UP 角色/武器合并成一个集合。抽到的名字不在这个集合里，就标记为「歪」。

概览中的歪卡率只统计有内置卡池数据、可以判定的限定小保底；歪掉后触发的下一个大保底最高等级必然出 UP，不计入分母。覆盖范围外或没有同期卡池数据的记录同样不计入分母。「各卡池歪卡率」按限定频段分别展示，角色、武器/光锥、联动等频段互不混合。

- 只判定限定卡池；常驻、新手、原神集录祈愿、原神千星奇域颂愿、绝区零邦布频段不判定。
- 同一时间有多期并行卡池时按并集判断，例如原神 4.0 上半的两期角色祈愿、武器池的两把 UP 武器。
- 星穹铁道的长期联动跃迁按长期开放处理。
- 记录时间不在内置卡池数据覆盖范围内，或当时没有对应卡池数据时，不标记。
- 内置卡池数据是静态的。新版本上线后需要更新对应的 `assets/*-banners.json` 并重新构建，否则新卡池期间的记录不会被标记。
- 卡池时间按 UTC+8 维护；非 UTC+8 服务器的记录在卡池边界附近可能有偏差。

截至 2026-11-06，内置数据覆盖原神 1.0–7.2、崩坏：星穹铁道 1.0–4.7、绝区零 1.0–3.3，包含部分前瞻版本。

## 数据与隐私

| 版本 | 数据根目录 |
| --- | --- |
| 生产构建 | `%APPDATA%\Hoyo Gacha` |
| `mygo dev` | `%APPDATA%\Hoyo Gacha Dev` |

每款游戏的数据位于 `data\<game>\`：

- `gacha-list-<uid>.json`：抽卡记录。

数据根目录的 `config.toml` 保存各游戏当前账号 UID 等配置，例如 `[games.genshin] current = "202"`。

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

- 原神集录祈愿、常驻、新手、千星奇域颂愿，星穹铁道常驻/新手，绝区零常驻/邦布频段不判定歪。
- 内置卡池数据不会自动更新；新版本需要更新数据文件后重新构建。
- 自动刷新目前主要针对 Windows 客户端，其他平台未验证。
- 同一游戏多账号时，应用启动读取 `config.toml` 的 `current` 账号，界面内暂不提供账号切换。

## 致谢

- [MyGo](https://github.com/egoist/mygo)：原生 Go UI 框架。
- [Biligame Wiki](https://wiki.biligame.com/)：历史卡池数据。
- [biuuu/genshin-wish-export](https://github.com/biuuu/genshin-wish-export)、[Trrrrw/genshin-wish-export](https://github.com/Trrrrw/genshin-wish-export)、[biuuu/star-rail-warp-export](https://github.com/biuuu/star-rail-warp-export)、[earthjasonlin/zzz-signal-search-export](https://github.com/earthjasonlin/zzz-signal-search-export)：抓取流程、导出格式与千星奇域颂愿规则参考。

本项目仅供个人管理抽卡记录使用。请遵守游戏服务条款；因使用本工具产生的风险由使用者自行承担。
