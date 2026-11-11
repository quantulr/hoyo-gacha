# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目概览

Hoyo Gacha 是一个用 Go 1.27.3 + MyGo v0.8.1（`github.com/egoist/mygo`）构建的原生桌面抽卡记录管理器，支持原神（含千星奇域颂愿）、崩坏：星穹铁道、绝区零。界面全部由 Go 的 `mygo/ui` 描述，没有前端构建步骤。

抽卡记录有两种来源：导入 `gacha-list-<uid>.json`，或从本机游戏日志/网页缓存中提取临时 authkey 链接后请求官方接口。应用数据位于 `mygo.App.Path(mygo.PathUserData)/data/<game>/`；`PathUserData` 由 MyGo 的 `mygo.json` `name` 决定，修改 `name` 会改变数据目录。Windows 生产版在 `%APPDATA%\Hoyo Gacha`，开发版在 `%APPDATA%\Hoyo Gacha Dev`；0.3.0 及更早版本使用 `%APPDATA%\hoyo-gacha`，升级后需手动迁移。

数据目录里没有 `gacha-list-*.json` 时，`loadGachaGame` 返回 `nil, nil`，界面显示导入或刷新引导，而不是读取失败。手动导入会校验 UID（仅数字、最多 20 位）和卡池频道，写入 `data/<game>/gacha-list-<uid>.json`，并把 UID 写入统一配置 `config.toml` 的 `[games.<game>] current`；同 UID 导入会覆盖原文件。

统一配置位于 `PathUserData/config.toml`（不在 `data/<game>/` 下），由 `config.go` 读写：

```toml
[games.genshin]
current = "202"

[games.starrail]
current = "101"
```

配置用 `map[string]any` 解析，未知键（如 `proxyPort`）在写入时保留。旧版每游戏一份的 `data/<game>/config.json` 只作为 `current` 的回退读取，读入成功后会写回 `config.toml`，无需手动迁移。

## 常用命令

```sh
go tool mygo dev        # 开发版，热重载；在终端按 q 退出
go tool mygo build      # 生产构建，输出到 build/
go tool mygo vet        # go vet + MyGo 原生 UI 生命周期检查
go test -count=1 ./...  # 全部测试
go test -run TestGenshinStats ./...  # 单个测试
go build -o /tmp/hoyo-gacha-check .  # 仅编译检查；用 -o 指到临时目录，避免在仓库根留下 exe
```

工作区混合 LF/CRLF。`gofmt -w` 会把 CRLF 文件整体改写，改动前先用 `file <file>.go` 确认；CRLF 文件可转成临时 LF 副本格式化后再比对。

## 架构

- `main.go`：`app` 状态、主窗口与所有共享 UI。启动时遍历 `games`，在后台调用各游戏的 `load`，再回到主线程 `apply`。`content` 按 `g.body` 分发页面；顶栏刷新按钮和全窗进度弹框由 `game.refresh` / `game.fetching` 驱动。
- `settings.go`：Kero 风格的独立设置窗口，含「通用」「关于」两页、手动检查更新和数据目录入口；窗口控件与主窗口共用 `windowControlsFor`。
- `import.go`：右上角和空状态里的导入按钮，通过 `mygo.Dialog.Open` 选择 `gacha-list-*.json`；校验 UID 与文件所含频道是否属于当前游戏，原子写入数据目录并更新 `config.toml` 的 `[games.<game>] current`，然后重新 `load`/`apply`。导入结果通过 `a.toast` 在 UI 线程显示。
- `config.go`：统一 TOML 配置。`loadGameConfig`/`writeGameConfig` 读写 `PathUserData/config.toml`，`configCurrent`/`setConfigCurrent` 访问 `[games.<game>]` 段，`saveGameCurrent`/`saveGameCurrentIfChanged` 供导入与抓取更新当前 UID；`gameDataDir` 统一拼出 `data/<game>/`。
- `accounts.go`：账号切换。`gachaAccounts` 扫描 `data/<game>/gacha-list-*.json` 得到 UID 列表；`switchGachaAccount` 用 `loadGachaAccount` 读入指定账号后交给 `game.apply`，读入成功才会更新 `config.toml` 的 `current`。标题栏的 UID 菜单由 `accountMenu` 绘制，`game.data` 提供当前已读入的数据。
- `games.go`：`game` 描述结构与注册表。每个游戏文件提供元数据、`gachaSpec`、`load`/`apply`、页面 `body`、频道颜色，可选 `refresh`/`fetching`/`annotate`；`game.stars` 决定等级用星级还是 S/A/B 展示。`games` 在 `init()` 中赋值，避免“游戏变量 → load → gameByID → games”的初始化循环。
- `gacha.go`：共享数据模型与统计。`gachaRecord` 同时解析对象记录和原神紧凑数组；`gachaExport` 对应 `gacha-list-<uid>.json`；`newGachaData` 计算保底、等级和出金历史，并把记录翻成最新在前，同时按原始 `RankType` 累计 `RankCounts` 供星级统计卡片使用。`gachaSpecFor` 从游戏注册表取等级与保底规则；`gachaSpec.ranks` 按频道覆盖等级映射。`loadGachaGame` 在没有任何导出文件时返回 `nil, nil`，读入成功后用文件 UID 校正 `config.toml` 的 `[games.<game>] current`（保留其他键）；没有配置时回退读取旧版 `data/<game>/config.json` 并完成迁移。歪卡标注由各游戏的 `annotate` 按时间顺序用 `markBannerPull` 写入 `pullRecord.BannerKnown`/`Lost`/`Guaranteed` 并标记 `channelStats.LostTracked`；`channelStats.countLost` 只汇总小保底（跳过 `Guaranteed`），同时把连续歪卡序号写入 `pullRecord.LostStreak`；`channelStats.LostRate` 供「各卡池歪卡率」使用，`gachaData.LostStats` 与 `LostRate` 供总览卡片使用。
- `fetch.go`：共享抓取基础设施。API 响应结构、`gachaLatestIDs`/`gachaIDEquals`/`gachaLocalIDs`、时区换算、`loadGachaLocal`、`mergeGachaExport`（按 id 严格并集）、原子写 `saveGachaExport(root, game, exp)`、`sanitizeURLError`。
- 每个游戏两份文件：`<game>.go`（描述符与 UI）和 `<game>_fetch.go`（日志/缓存定位、链接解析、分页抓取）。ZZZ、星穹铁道和原神的卡池歪标记分别在 `zzz_banners.go`、`starrail_banners.go`、`genshin_banners.go`，对应的 `assets/*-banners.json` 通过 `go:embed` 内嵌；卡池时间按 UTC+8。原神卡池只覆盖角色/武器活动祈愿，集录祈愿和千星奇域颂愿暂不判定。
- UI 测试用 `ui.NewTester(a.view, 960, 680)` 配合 `HasText`/`Click`，不需要真实窗口。

## 抓取流程

每个游戏的 `refresh<Game>` 先进入刷新状态，然后在 goroutine 中调用 `refresh<Game>Data(progress)`：

1. `find<Game>LogURL` 在游戏日志/网页缓存中找最后一条含 authkey 的链接；
2. `parse<Game>LogURL` 删除 `page`/`size`/`gacha_type`/`end_id`/`begin_id` 等分页参数，保留 authkey 等公共参数，并按 host 选择国服/国际服域名；`begin_id` 是网页里的滚动游标，带给接口会返回以它为准的旧页，其他频段甚至返回空列表；
3. `<game>FetchUID` 用第一页确认 UID 和服务器时区；原神普通祈愿全空时继续探测颂愿接口；
4. `fetch<Game>All` 读取本地文件，按频道翻页；星穹铁道在整页记录都已在本地时提前停止（顺带补回晚于本地最新一条才写入服务器的旧记录），原神与绝区零命中 `gachaLatestIDs` 返回的本地最新 id 时提前停止；
5. `mergeGachaExport` 按 id 合并（不会丢弃本地独有的频道），`saveGachaExport` 原子写回并更新 `config.toml` 的 `[games.<game>] current`。

分页约定：每页 20 条、`end_id` 游标、网络错误重试 5 次（间隔 5s）、每页 300ms、每 10 页额外 1s。进度通过 `progress` 回调更新；UI 线程更新一律走 `a.publish`。原神千星奇域颂愿走独立的 `/gacha_info/api/getBeyondGachaLog`，活动颂愿统一用 `gacha_type=2000` 查询，响应里的 `op_gacha_type` 保留到记录中；英文接口的语言参数需要从 `en` 改成 `en-us`。

原神接口不返回 `region_time_zone`，`genshinRegionZone` 按 UID 前缀补：`6` = UTC-5，`7` = UTC+1，其余 = UTC+8。HSR 使用接口返回的时区，ZZZ 按 `region` 查 `zzzServerZones`。

### 频道与规则

| 游戏 | 频道 | 等级 | 保底 |
|---|---|---|---|
| 原神 | 301 角色 / 302 武器 / 500 集录 / 200 常驻 / 100 新手 / 2000 活动颂愿 / 1000 常驻颂愿 | 5/4/3；1000 为 4/3/2 | 302=80，100=20，2000/1000=70，其余 90 |
| 星穹铁道 | 11/12/1/2/21/22 | 5/4/3 | 12/22=80，2=50，其余 90 |
| 绝区零 | 2/102 角色 / 3/103 音擎 / 1 常驻 / 5 邦布 | 4/3/2 | 3/103/5=80，其余 90 |

等级在 `pullRecord.Tier` 中统一成 0=最高、1=次高、2=第三档；规则集中在各游戏的 `gachaSpec`，`hardPity` 缺省 90。展示由 `game.stars` 决定：原神/星穹铁道显示 3/4/5 星，记录徽标用原始 `RankType`（千星奇域常驻颂愿为 4/3/2 星），筛选标签按当前频段的 `gachaSpec.rankFor` 生成；绝区零显示 S/A/B。千星奇域常驻颂愿没有 5 星，4 星作为最高档参与保底与出金统计。

### 新接入一款游戏

1. 新建 `<game>.go`：定义 `var <game>Game = game{...}`、`<game>Spec`、`load<Game>Data`、`apply<Game>Data`、`<game>Body`、`<game>ChannelColor`。
2. 在 `games.go` 的 `init()` 中注册。共享文件（`gacha.go`/`fetch.go`/`main.go`）不得出现具体游戏分支；游戏差异放在各自文件或 `game` 字段里。
3. 需要自动抓取时新建 `<game>_fetch.go`，设置 `refresh`/`fetching`；沿用 `mergeGachaExport`、`saveGachaExport` 和既有错误脱敏，不要改成位置式合并。
4. 需要读入后补充处理（如歪标记）时实现 `annotate`。

## 数据与安全

- 用户抽卡数据、真实 UID、authkey 链接都不属于仓库，不能提交；测试只用假 authkey 和构造数据。
- authkey 是日志/缓存里的临时凭据：只允许在抓取时读取，禁止打印、写入日志或持久化；网络错误已由 `sanitizeURLError` 剥掉完整 URL。
- 不要用真实用户数据做测试输入；需要验证日志解析时，只输出是否找到、长度/域名等非敏感信息。

## Git 工作流

- 用户说“提交”才 commit；说“合并”才执行 `git fetch origin && git merge --ff-only origin/main && git push origin main`。其他时候不要推送。
- 提交信息使用 Conventional Commits，简洁说明 why；沿用仓库现有的 Co-Authored-By 尾注。

## 发布与自动更新

- `mygo.json` 的 `updates` 指向 GitHub Release；`updates.publicKey` 是签名公钥，私钥只保存在 GitHub Actions Secret `MYGO_UPDATER_PRIVATE_KEY`，禁止提交、打印或写入 CI 日志。
- `.github/workflows/release.yml` 在推送 `v*` tag 时校验 tag 与 `mygo.json` 的 `version` 一致，构建 Windows amd64/arm64，上传 draft release，全部成功后自动发布；带连字符的 tag 发布为 prerelease。
- 发版前更新 `mygo.json` 的 `version` 和 `CHANGELOG.md` 对应版本段落；更新窗口的说明来自 CHANGELOG。
- 应用通过 `plugins/updater/native` 在启动后后台检查更新；开发构建和不可写安装目录不会自更新。
