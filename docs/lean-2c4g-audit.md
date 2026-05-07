# NewAPI 2C4G 精简部署审计与瘦身方案

> 审计目标：在不修改业务代码、不删除文件的前提下，分析当前 NewAPI 在 2 vCPU / 4GB RAM 云服务器上的启动链路、内存风险、可禁用模块、可裁剪依赖，并给出面向“用户充值、调用字节 Seedance 2.0、按消耗扣余额或订阅额度、后台看账”的 lean 版本落地方案。
>
> 本文档只做分析和实施建议。代码级改造需在后续分支中按阶段执行。

## 1. 当前项目启动链路

### 1.1 后端启动入口

入口文件：`main.go`

当前启动过程可以概括为：

```text
main()
  -> InitResources()
     -> 加载 .env
     -> common.InitEnv()
     -> logger.SetupLogger()
     -> ratio_setting.InitRatioSettings()
     -> service.InitHttpClient()
     -> service.InitTokenEncoders()
     -> model.InitDB()
     -> model.CheckSetup()
     -> model.InitOptionMap()
     -> common.CleanupOldCacheFiles()
     -> model.GetPricing()
     -> model.InitLogDB()
     -> common.InitRedisClient()
     -> common.StartSystemMonitor()
     -> i18n.Init()
     -> oauth.LoadCustomProviders()
  -> 可选/固定后台任务启动
  -> gin.New()
  -> 中间件注册
  -> router.SetRouter()
  -> server.Run(:PORT)
```

### 1.2 启动时加载的资源、服务与后台任务

| 启动项 | 位置 | 默认行为 | 内存/资源影响 | 是否与目标功能必要 | 建议 |
|---|---|---:|---|---:|---|
| 环境变量初始化 | `common/init.go` | 固定加载 | 设置大量运行参数，默认部分参数偏大 | 是 | 保留，但为 2C4G 调小默认值 |
| 日志初始化 | `logger.SetupLogger()` | 固定加载 | 低 | 是 | 保留 |
| HTTP 客户端 | `service/http_client.go` | 固定加载，默认 `MaxIdleConns=500`、`MaxIdleConnsPerHost=100` | 中，空闲连接池偏大 | 是，调用字节 API 需要 | 保留，但调小连接池 |
| Token Encoder | `service/tokenizer.go` | 固定加载 `cl100k_base` tokenizer | 中，tokenizer 常驻内存 | 账户计量可能需要 | 保留；若 Seedance 只按任务 usage 计费，可后续延迟加载 |
| 主数据库 | `model/main.go` | 固定加载，默认 `SQL_MAX_IDLE_CONNS=100`、`SQL_MAX_OPEN_CONNS=1000` | 高，连接池对 2C4G 过大 | 是 | 保留，但必须调小连接池 |
| AutoMigrate | `model/main.go` | master 节点启动时自动迁移全部表 | 中，启动时间和 DB 压力增加 | 是，但表过多 | MVP 保留；后续拆 lean migration |
| Setup 检查 | `model.CheckSetup()` | 固定检查 | 低 | 是 | 保留 |
| OptionMap | `model.InitOptionMap()` | 固定从 DB 加载系统配置 | 中低 | 是 | 保留，减少无关 Option 后可瘦身 |
| Pricing | `model.GetPricing()` | 固定加载模型价格 | 低 | 是，账单需要 | 保留 |
| 日志数据库 | `model.InitLogDB()` | 固定初始化；无 `LOG_SQL_DSN` 时复用主库 | 中；如单独日志库则双连接池 | 是，账单/审计依赖日志 | 保留，建议复用主库并调小连接池 |
| Redis | `common.InitRedisClient()` | compose 默认开启 Redis；启用 Redis 后强制内存缓存 | 中高，额外容器 + 应用内缓存 | 否，单机可不需要 | 2C4G 默认关闭 |
| 系统监控 goroutine | `common.StartSystemMonitor()` | 固定启动；未启用监控时每 30 秒休眠 | 低 | 可选 | 保留或后续加开关 |
| i18n | `i18n.Init()` | 固定加载多语言资源 | 中低 | 否，单语言后台不需要全部语言 | 后续裁剪语言包 |
| 自定义 OAuth Provider | `oauth.LoadCustomProviders()` | 固定加载 | 低 | 否 | 后续配置关闭或移除第三方登录 |
| Channel 内存缓存 | `model.InitChannelCache()` | `MEMORY_CACHE_ENABLED=true` 或 Redis 开启后加载 | 中高，加载 channel/ability/multikey | 可选 | 2C4G 关闭 Redis 与内存缓存 |
| Channel 缓存同步 | `model.SyncChannelCache()` | 内存缓存开启时循环同步 | 中 | 可选 | 关闭 |
| Option 同步 | `model.SyncOptions()` | 固定 goroutine，默认 60 秒 | 低 | 是 | 保留，可调低频率 |
| 数据看板缓存落库 | `model.UpdateQuotaData()` | 固定 goroutine；`DataExportEnabled=true` 时每 5 分钟 flush | 中，内存 map 随用户/模型/小时增长 | 账单可选，统计看板需要 | MVP 可保留；小机器建议拉长间隔或关闭看板缓存 |
| 自动渠道更新 | `controller.AutomaticallyUpdateChannels()` | 仅设置 `CHANNEL_UPDATE_FREQUENCY` 时启动 | 中 | 否 | 不配置 |
| 自动渠道测试 | `controller.AutomaticallyTestChannels()` | 固定启动 | 中，可能扫描渠道 | 否 | 后续加开关关闭 |
| Codex 凭证自动刷新 | `service.StartCodexCredentialAutoRefreshTask()` | 固定调用，master 执行 | 低中，扫描 Codex 渠道 | 否 | 后续加开关关闭 |
| 订阅额度重置 | `service.StartSubscriptionQuotaResetTask()` | 固定调用，master 每 1 分钟检查 | 中低 | 若使用订阅额度则需要 | 保留或按是否启用订阅配置关闭 |
| 自动出账单 | `service.StartBillingStatementAutoTask()` | 默认关闭，`BILLING_STATEMENT_AUTO_ENABLED=true` 开启 | 低中 | 可选，后台看账需要月结时需要 | MVP 可手动，生产开启 |
| 上游模型同步 | `controller.StartChannelUpstreamModelUpdateTask()` | 默认开启，除非显式 `CHANNEL_UPSTREAM_MODEL_UPDATE_TASK_ENABLED=false` | 中高，会访问渠道同步模型 | 否 | 必须关闭 |
| Midjourney 轮询 | `controller.UpdateMidjourneyTaskBulk()` | `UPDATE_TASK=true` 时启动 | 中，15 秒轮询未完成 MJ 任务 | 否 | 后续拆开关闭；当前若不使用 MJ 应避免相关任务 |
| 通用任务轮询 | `service.TaskPollingLoop()` | `UPDATE_TASK=true` 时启动 | 中，15 秒查询未完成异步任务，默认 limit 1000 | Seedance 异步任务需要 | 保留但将 `TASK_QUERY_LIMIT` 调小 |
| 批量更新器 | `model.InitBatchUpdater()` | compose 默认 `BATCH_UPDATE_ENABLED=true` | 中，多个批量 map 常驻 | 可选 | 2C4G 关闭，低并发先直接更新 DB |
| pprof | `ENABLE_PPROF=true` 才启动 | 默认关闭 | 低/中 | 否 | 保持关闭 |
| Pyroscope | `PYROSCOPE_URL` 非空才启动 | 默认不启动 | 中 | 否 | 保持关闭 |
| Gin + 中间件 | `main.go`、`middleware/*` | 固定加载 | 中低 | 是 | 保留核心鉴权/日志/限流，裁剪非必要中间件 |
| 路由注册 | `router.SetRouter()` | API、Dashboard、Relay、Video、Web 全量注册 | 中低，但功能面暴露大 | 部分需要 | 后续按功能开关裁剪 |

### 1.3 当前路由加载情况

| 路由组 | 位置 | 当前能力 | 与目标关系 | 建议 |
|---|---|---|---|---|
| `/api/setup` | `router/api-router.go` | 初始化系统 | 初次部署需要 | 保留，但生产初始化后必须关闭或限制 token/IP |
| `/api/user/*` | `router/api-router.go` | 注册、登录、登出、个人信息、充值、支付、2FA、Passkey | 账户体系需要，部分可裁剪 | 保留登录/用户/余额/充值；裁剪第三方登录、Passkey、2FA 可选 |
| `/api/channel/*` | `router/api-router.go` | 渠道管理、多供应商 | 字节渠道配置需要 | 保留最小渠道管理，隐藏其他供应商 |
| `/api/token/*` | `router/api-router.go` | API Token 管理 | API 调用鉴权需要 | 保留 |
| `/api/log/*` | `router/api-router.go` | 调用日志 | 按账户统计需要 | 保留 |
| `/api/billing/*` | `router/api-router.go` | 账单汇总、导出、月结、监控指标 | 后台看账需要 | 保留 |
| `/api/data/*` | `router/api-router.go` | 数据看板 | 可选 | MVP 可只保留账单表格，后续关闭大屏 |
| `/v1/*` | `router/relay-router.go` | OpenAI 兼容文本/图片/音频/embedding/realtime 等 | 大部分不需要 | 后续按功能开关只保留必要 API |
| `/v1/video/generations` 等 | `router/video-router.go` | 视频任务提交/查询 | Seedance 2.0 需要 | 保留 Seedance/Doubao 路径，裁剪 Kling/Jimeng 等 |
| `/mj/*`、`/suno/*` | `router/relay-router.go` | Midjourney/Suno | 不需要 | 移除或配置关闭 |
| `/dashboard/*` | `router/dashboard.go` | 历史 dashboard 路由 | 可选 | 若前端已用 `/api/billing`，可后续移除 |
| Web SPA | `router/web-router.go` | 嵌入 `web/dist` 后台 | 最小后台需要 | 保留 lean 后台页面 |

## 2. 当前内存风险点

### 2.1 主要风险排行

| 风险点 | 严重度 | 现状 | 影响 | 立即建议 |
|---|---|---|---|---|
| 数据库连接池默认过大 | 高 | 主库与日志库默认 `100 idle / 1000 open` | 连接、prepared statement、DB 进程内存都可能被放大 | `SQL_MAX_IDLE_CONNS=2`、`SQL_MAX_OPEN_CONNS=20`、`SQL_MAX_LIFETIME=300` |
| docker-compose 默认启动 Redis | 高 | `REDIS_CONN_STRING=redis://redis`，且启用 Redis 后强制 `MemoryCacheEnabled=true` | 多一个 Redis 容器，应用还加载 channel cache | 2C4G 单机默认移除 Redis 服务和 env |
| 本地同时启动 Postgres + Redis + 全量 Go 服务 | 高 | 当前 compose 面向通用部署 | 4GB 内存容易被 DB/cache/app/系统页缓存挤满 | lean compose 只保留 app + tuned Postgres，或 DB 外置 |
| 批量更新器默认在 compose 开启 | 中 | `BATCH_UPDATE_ENABLED=true` | 常驻 map，低并发收益有限 | MVP 关闭 |
| 上游模型同步默认开启 | 中 | `CHANNEL_UPSTREAM_MODEL_UPDATE_TASK_ENABLED` 默认 true | 启动后立即扫描渠道并请求上游 | 设为 false |
| Midjourney/通用任务轮询绑定在 `UPDATE_TASK` | 中 | `UPDATE_TASK=true` 会同时启动 MJ 和通用 task loop | 使用 Seedance 需要 task loop，但不需要 MJ | 后续拆成 `SEEDANCE_TASK_WORKER_ENABLED` 与 `MIDJOURNEY_TASK_WORKER_ENABLED` |
| 请求体和流式扫描上限偏大 | 中 | `MAX_REQUEST_BODY_MB=128`、`STREAM_SCANNER_MAX_BUFFER_MB=128`、`MAX_FILE_DOWNLOAD_MB=64` | 单请求即可制造高内存压力 | Seedance MVP 调到 8/16/10 MB，素材只走 URL 白名单 |
| HTTP 空闲连接池偏大 | 中 | `RELAY_MAX_IDLE_CONNS=500`、`RELAY_MAX_IDLE_CONNS_PER_HOST=100` | 对单上游场景过大 | 调为 50/10 或更低 |
| 多语言与大前端包 | 中 | `web/dist` 约 18MB，`web/node_modules` 本地约 1.3GB | 运行态 dist 不算巨大，但构建态/镜像上下文风险高 | 不在服务器构建；只部署产物；后续裁剪 i18n/图表 |
| 图表、Markdown、Mermaid、KaTeX、图标库 | 中 | 前端依赖较重 | 增大构建产物和首屏资源 | lean 后台只保留表格/账单/用户/渠道 |
| 音频/视频/图片解析依赖 | 中 | Go 依赖包含 mp3/wav/flac/mp4/image 等 | 不一定启动即占用大量内存，但增加二进制体积和攻击面 | `GET_MEDIA_TOKEN=false`；后续移除不需要的媒体计量 |
| Pyroscope/pprof | 低/中 | 默认不启用 | 开启后有额外采样/端口风险 | 生产 2C4G 保持关闭 |

### 2.2 当前构建/运行体积观察

| 项 | 当前观察 | 说明 |
|---|---:|---|
| `web/dist` | 约 18MB | 运行态被 Go binary 嵌入，体积可接受但仍可瘦身 |
| `web/node_modules` | 约 1.3GB | 不应出现在运行服务器；应在 CI 或本地构建后只部署镜像/二进制 |
| 后端 provider 目录 | 多供应商全量存在 | 运行注册不一定巨大，但二进制、依赖、测试和攻击面都偏大 |
| Docker compose 服务 | app + postgres + redis | 2C4G 上建议默认移除 Redis |

## 3. 必须保留的核心功能

针对当前业务目标，最小可用闭环如下：

```text
用户注册/管理员创建用户
  -> 用户充值或管理员发放余额/订阅额度
  -> 用户创建 API Token
  -> 用户调用 Seedance 2.0 视频生成接口
  -> 系统按任务预扣/完成后按 usage 或固定价格结算
  -> 写入 Log / Task / BillingStatement
  -> 管理后台查看用户余额、调用记录、账单汇总、月结快照
```

| 核心功能 | 必须保留模块 | 说明 |
|---|---|---|
| 用户/账户体系 | `model/user.go`、`controller/user.go`、`middleware/auth.go`、`router/api-router.go` 用户相关路由 | 保留注册/登录/登出/用户余额/管理员用户管理 |
| API Token 鉴权 | `model/token.go`、`controller/token.go`、`middleware/auth.go`、`middleware/distributor.go` | 外部客户调用 API 的基础鉴权 |
| 充值/余额 | `model/topup.go`、`model/redemption.go`、`controller/topup.go`、`controller/redemption.go` | 若暂不接第三方支付，可先保留兑换码/管理员充值 |
| 订阅额度 | `model/subscription*.go`、`controller/subscription.go`、`service/subscription_reset_task.go` | 如果客户需要“订阅额度”，保留；若只有余额扣费，可延后 |
| 调用量记录 | `model/log.go`、`model/usedata.go`、`controller/log.go` | 账单汇总和后台对账依赖 |
| 账单/月结 | `model/billing_statement.go`、`controller/log_billing.go`、`service/billing_statement_task.go`、`service/seedance_readiness.go` | 当前已有 Seedance 账单筛选和 readiness 检查，应保留 |
| Seedance 2.0 调用 | `relay/channel/task/doubao/*`、`relay/channel/volcengine/*`、`relay/relay_task.go`、`router/video-router.go` | 保留字节/豆包视频任务提交、查询、回调白名单、计费估算 |
| 异步任务轮询 | `service/task_polling.go`、`model/task.go`、`controller/task.go` | Seedance 视频生成是异步任务时需要；但轮询 limit 应调小 |
| 最小后台 | `web/src/pages/User`、`Token`、`TopUp`、`Channel`、`Log`、`Billing`、`SelfBilling`、`Setup`、`Setting` 的子集 | 保留管理用户、充值、渠道、日志、账单、初始化 |
| 基础日志/错误处理 | `logger/*`、`middleware/request-id.go`、`middleware/recover.go`、`common/logger.go` | 保留 |
| 安全配置 | `SESSION_SECRET`、`CRYPTO_SECRET`、`SETUP_TOKEN`、`CORS_ALLOW_ORIGINS`、`TRUSTED_PROXIES` | 生产必须配置环境变量，不写死 |

## 4. 可删除/可禁用模块清单

> 处理方式分为五类：
>
> - 保留：目标功能必须依赖。
> - 延迟加载：首次使用时再加载，避免启动常驻。
> - 配置关闭：短期无需改代码或只需极小开关即可关闭。
> - 移除：后续 lean 分支可删代码/依赖/页面，但需要编译回归。
> - 后续确认：业务口径未定，先不要删。

| 模块/目录/文件 | 功能说明 | 是否与账户/账单/大模型调用相关 | 是否可以禁用 | 是否可以删除 | 删除风险 | 建议处理方式 |
|---|---|---:|---:|---:|---|---|
| `relay/channel/openai`、`claude`、`gemini`、`aws`、`cohere`、`mistral`、`zhipu` 等 | 多模型供应商支持 | 否，除字节外不需要 | 是 | 是 | 直接删会影响 adaptor registry、常量、前端渠道枚举 | 先配置隐藏；后续用 provider build tag 或 registry 白名单移除 |
| `relay/channel/volcengine` | 火山/字节模型适配 | 是 | 否 | 否 | 删除会失去字节模型能力 | 保留 |
| `relay/channel/task/doubao` | 豆包/Seedance 异步视频任务 | 是 | 否 | 否 | 删除会失去 Seedance 2.0 | 保留 |
| `relay/channel/task/kling`、`jimeng`、`suno`、`hailuo`、`vidu`、`sora`、`ali`、`gemini`、`vertex` | 其他视频/任务供应商 | 否 | 是 | 是 | 与 taskcommon、路由、前端任务页有耦合 | 后续移除，只保留 doubao + taskcommon |
| `router/relay-router.go` 中 `/mj`、`/suno`、realtime、audio、embedding、image、Gemini 等路由 | 全量 OpenAI 兼容和娱乐模型路由 | 大部分否 | 是 | 是 | 客户若误用旧 API 会断 | 后续增加功能开关，lean 只注册 Seedance 与必要 token/billing API |
| `router/video-router.go` | 视频生成路由 | Seedance 部分相关 | 部分可禁用 | 部分可删 | 需要区分 Doubao/Kling/Jimeng 映射 | 保留 Seedance/Doubao，移除其他映射 |
| `controller/midjourney.go`、`relay/channel/mj` 相关 | Midjourney 任务轮询与代理 | 否 | 应可禁用 | 是 | 当前 `UPDATE_TASK` 会同时启动 MJ 轮询，需先拆开关 | 加 `MIDJOURNEY_TASK_WORKER_ENABLED=false` 后移除 |
| `service/task_polling.go` | 通用异步任务轮询 | 是，Seedance 可能需要 | 不建议完全禁用 | 否 | 禁用会导致异步任务无法完成/退款 | 保留，但 `TASK_QUERY_LIMIT=50`，只轮询 Seedance 类型 |
| `controller/channel_upstream_update.go` | 自动同步上游模型 | 否 | 是 | 可删 | 关闭后需手动维护模型 | 立即配置 `CHANNEL_UPSTREAM_MODEL_UPDATE_TASK_ENABLED=false` |
| `controller/channel-test.go` 自动测试 | 自动测试渠道可用性 | 否 | 需要新增开关 | 可删 | 可能影响后台渠道状态自动更新 | 后续新增 `CHANNEL_AUTO_TEST_ENABLED=false` |
| `service/codex_credential_refresh_task.go` | Codex 凭证刷新 | 否 | 需要新增开关 | 是 | 删除需清理 Codex 渠道类型 | 后续新增 `CODEX_CREDENTIAL_REFRESH_ENABLED=false` 后移除 |
| `service/subscription_reset_task.go` | 订阅额度到期/重置 | 订阅额度相关 | 视业务而定 | 不建议先删 | 若客户买订阅包会需要 | 若 MVP 只余额扣费可配置关闭；订阅上线时保留 |
| `service/billing_statement_task.go` | 自动月结账单 | 是 | 是 | 否 | 关闭只影响自动月结，不影响手动生成 | MVP 手动生成；生产开启 |
| `model/usedata.go` | 数据看板小时聚合 | 间接相关 | 是 | 不建议先删 | 删除会影响 dashboard 图表 | 账单以 `logs` 为准；大屏可关闭 |
| `web/src/pages/Midjourney` | MJ 页面 | 否 | 是 | 是 | 需清理菜单/路由 | 移除 |
| `web/src/pages/Chat`、`Chat2Link`、`Playground` | 聊天/分享/调试台 | 否 | 是 | 是 | 若客户只 API 调用，无影响 | 移除或隐藏 |
| `web/src/pages/ModelDeployment`、`web/src/components/model-deployments` | 模型部署/供应商市场 | 否 | 是 | 是 | 需保留最小渠道配置替代 | 移除，保留字节渠道配置页 |
| `web/src/pages/Pricing` | 模型广场/价格展示 | 间接相关 | 是 | 可删 | 若客户门户要展示价格则需保留 | 后续确认；后台账单不依赖它 |
| `web/src/pages/Subscription` | 订阅计划管理 | 订阅额度相关 | 视业务而定 | 后续确认 | 客户若要订阅额度则需要 | 如果“订阅额度”是必需，保留精简版 |
| `web/src/pages/Billing`、`SelfBilling` | 管理员/用户账单 | 是 | 否 | 否 | 删除无法看账 | 保留 |
| `web/src/pages/Dashboard`、`components/dashboard/LazyVChart.jsx` | 数据分析大屏和图表 | 可选 | 是 | 是 | 删除影响图表，不影响账单表格 | MVP 隐藏，后续移除 `vchart` |
| `web/src/i18n/locales/*` | 多语言包 | 否 | 是 | 是 | 删除需固定语言和构建配置 | lean 只保留 `zh-CN` 或 `zh-CN + en` |
| `oauth/*`、`/api/oauth/*` | 第三方登录 | 否 | 是 | 是 | 客户若要求 SSO 则需另做企业 SSO | MVP 移除 GitHub/Discord/LinuxDO/Telegram/Wechat/OIDC |
| Passkey / WebAuthn | `go-webauthn/webauthn`、用户 passkey 路由 | 否 | 是 | 是 | 删除需调整用户安全设置页面 | MVP 移除，保留密码登录和管理员重置 |
| 2FA | `model/twofa*`、用户 2FA 路由 | 可选安全项 | 是 | 后续确认 | 删除降低安全能力 | 可保留；若极致瘦身再裁剪 |
| 邮件通知 | email/reset/notify 相关 controller/service | 否，除密码找回 | 是 | 是 | 删除会影响找回密码/邮件验证码 | MVP 可关闭注册邮件，管理员创建用户 |
| Webhook | `service/webhook.go`、支付/通知 webhook | 否 | 是 | 是 | 若支付回调或外部通知需要则不能删 | 不接在线支付时移除 |
| 第三方支付 | Stripe/Creem/Waffo/Epay 相关 controller/service/deps | 充值相关但非必须 | 是 | 后续确认 | 若客户需要线上支付会需要 | MVP 用管理员充值/兑换码；只保留一个最终支付渠道 |
| 文件上传/素材下载 | `service/file_service.go`、`common/body_storage.go`、`types/file_source.go` | Seedance 可能只需 URL 素材 | 是 | 部分可删 | Seedance 如果支持本地上传会受影响 | MVP 禁止大文件上传，只允许白名单 URL |
| 音频/图片 token 解析 | `common/audio.go`、媒体解析依赖 | 否 | 是 | 是 | 文本/多模态 token 计量可能受影响 | `GET_MEDIA_TOKEN=false`，后续删音频解析依赖 |
| Realtime WebSocket | `gorilla/websocket`、`/v1/realtime` | 否 | 是 | 是 | 删除会影响实时语音/对话 | 移除 |
| Redis 缓存 | `common/redis.go`、compose redis | 单机可不需要 | 是 | 后续确认 | 多节点/高并发缓存会需要 | 2C4G 关闭；规模扩大再外置 Redis |
| Electron | `electron/`、`web/package.json` electron dev deps | 否 | 是 | 是 | 无服务器影响 | 不进服务器构建上下文；后续移除 |
| 测试 Demo 数据 | `*_test.go`、测试 fixtures | 否 | 不参与运行 | 不建议从源码删 | 删除会降低回归能力 | 不打进运行镜像即可 |
| `deploy/observability` | 监控栈配置 | 可选 | 是 | 可删 | 生产运维可能需要 | 2C4G 不本机部署 Prometheus/Grafana；用云监控或轻量日志 |

## 5. 运行时依赖审计

### 5.1 Go 依赖中可瘦身的大类

| 依赖/能力 | 当前来源 | 是否运行必需 | 建议 |
|---|---|---:|---|
| 多供应商 SDK/适配依赖 | `relay/channel/*`、`go.mod` | 否，除字节外 | 后续 provider 白名单，只保留 VolcEngine/Doubao 相关 |
| AWS Bedrock SDK | `github.com/aws/aws-sdk-go-v2/*` | 否 | 若不接 AWS，移除 |
| Redis 客户端 | `github.com/go-redis/redis/v8` | 2C4G 单机否 | 短期依赖可留，运行不配置 Redis；后续 build tag 移除 |
| Pyroscope | `github.com/grafana/pyroscope-go` | 否 | 运行不开；后续 lean 构建移除 |
| gopsutil | `github.com/shirou/gopsutil` | 可选 | 可保留轻量状态；极致瘦身则改为按需检查 |
| WebAuthn | `github.com/go-webauthn/webauthn` | 否 | 不使用 Passkey 时移除 |
| 支付 SDK | Stripe/Epay/Waffo/Creem | 后续确认 | MVP 管理员充值时移除；如果要线上支付只保留一个 |
| 音频/视频解析 | mp3/wav/flac/mp4/image 相关 | 否 | Seedance 只走 URL 和上游 usage 时移除 |
| WebSocket | `gorilla/websocket` | 否 | 不做 realtime 时移除 |
| SQLite 驱动 | `glebarez/sqlite` | 视 DB 方案 | 若 production 统一 Postgres 可移除；若极小单机试点可保留 SQLite |
| tokenizer | `tiktoken-go/tokenizer` | 可能需要 | 若 Seedance 全按任务 usage 计费，可延迟加载或只保留估算兜底 |

### 5.2 前端依赖中可瘦身的大类

| 依赖/能力 | 当前来源 | 是否目标必需 | 建议 |
|---|---|---:|---|
| 图表库 | `@visactor/vchart`、`@visactor/react-vchart`、`vchart-semi-theme` | 否，账单表格足够 | lean 后台移除大屏图表 |
| Mermaid/KaTeX/Markdown 高级渲染 | `mermaid`、`katex`、`rehype-*`、`remark-*` | 否 | 如不展示复杂文档，移除 |
| 大图标库 | `@lobehub/icons`、`lucide-react` | 部分可选 | 保留少量图标或改静态枚举 |
| 双 UI 体系 | `@douyinfe/semi-ui` + `antd` | 否 | 后续统一一套 UI，减少 bundle |
| 上传组件 | `react-dropzone` | 否 | 不开放文件上传时移除 |
| 第三方登录组件 | `react-telegram-login` | 否 | 移除 |
| Turnstile | `react-turnstile` | 可选安全项 | 若公网注册开放可保留；若管理员创建用户可移除 |
| i18n 全量语言 | `web/src/i18n/locales` | 否 | 只保留业务语言 |
| Electron | `electron`、`electron-builder` | 否 | 不参与 server 部署 |

### 5.3 Docker/启动脚本风险

| 文件 | 当前情况 | 风险 | 建议 |
|---|---|---|---|
| `docker-compose.yml` | 默认 app + Redis + Postgres | 2C4G 内存压力偏大；Redis 会触发 app 内存缓存 | 提供 `docker-compose.lean.yml` 或 profile，默认不启动 Redis |
| `Dockerfile` | 构建 web + Go，运行时 Debian slim | 构建阶段重，不适合 2C4G 服务器现场构建 | CI/本地构建镜像，服务器只 `docker compose pull/up` |
| `.env.example` | 默认示例包含大连接池、Redis、媒体 token、任务等 | 用户容易照抄导致小机内存不足 | 新增 `.env.lean.example` |
| `web/package.json` | 构建依赖大 | 服务器构建会消耗大量内存/磁盘 | 不在服务器 `npm/bun install` |
| `main.go` | 多后台任务固定启动 | 功能不用也会启动 goroutine | 后续补齐 task/worker 开关 |

## 6. 建议的最小架构

### 6.1 2C4G lean 运行拓扑

推荐优先级：

1. **首选**：2C4G 只运行 NewAPI + Nginx，Postgres 使用云数据库或另一台小规格数据库。
2. **可接受**：2C4G 同机运行 NewAPI + tuned Postgres，不启动 Redis，不在本机构建前端/镜像。
3. **仅试点**：SQLite 单文件 + NewAPI，适合极低并发验证，不建议正式商用。

```text
[Client]
  -> HTTPS / Nginx or Cloud LB
  -> NewAPI lean backend + embedded lean admin UI
  -> Postgres tuned small pool
  -> ByteDance Seedance 2.0 API
```

### 6.2 单机 2C4G 推荐参数

建议运行环境变量：

```env
PORT=3000
TZ=Asia/Shanghai
NODE_TYPE=master

# 数据库：本机 Postgres 或云数据库，连接池必须小
SQL_DSN=postgresql://newapi:strong-password@postgres:5432/newapi
LOG_SQL_DSN=
SQL_MAX_IDLE_CONNS=2
SQL_MAX_OPEN_CONNS=20
SQL_MAX_LIFETIME=300

# 单机关闭 Redis 和内存缓存
REDIS_CONN_STRING=
MEMORY_CACHE_ENABLED=false
SYNC_FREQUENCY=300
BATCH_UPDATE_ENABLED=false

# 后台任务瘦身
CHANNEL_UPSTREAM_MODEL_UPDATE_TASK_ENABLED=false
CHANNEL_UPDATE_FREQUENCY=
UPDATE_TASK=true
TASK_QUERY_LIMIT=50
TASK_TIMEOUT_MINUTES=1440
BILLING_STATEMENT_AUTO_ENABLED=false
BILLING_STATEMENT_AUTO_CHECK_INTERVAL_MINUTES=60

# HTTP/请求体限制
RELAY_MAX_IDLE_CONNS=50
RELAY_MAX_IDLE_CONNS_PER_HOST=10
RELAY_TIMEOUT=120
MAX_REQUEST_BODY_MB=8
STREAM_SCANNER_MAX_BUFFER_MB=16
MAX_FILE_DOWNLOAD_MB=10
GET_MEDIA_TOKEN=false
GET_MEDIA_TOKEN_NOT_STREAM=false

# 多租户和审计：单客户/单平台先关闭复杂多租户
MULTI_TENANT_ENABLED=false
TENANT_AUDIT_ENABLED=false

# 生产安全
SESSION_SECRET=replace-with-32-byte-random
CRYPTO_SECRET=replace-with-another-32-byte-random
SETUP_TOKEN=replace-with-one-time-setup-token
CORS_ALLOW_ORIGINS=https://your-domain.example
TRUSTED_PROXIES=127.0.0.1
TLS_INSECURE_SKIP_VERIFY=false
ENABLE_PPROF=false
PYROSCOPE_URL=

# Seedance 2.0 计费安全闸门
SEEDANCE_BILLING_BY_USAGE=true
SEEDANCE_BILLING_STRICT_USAGE=false
SEEDANCE_REQUIRE_PRICE_CONFIRMATION=true
SEEDANCE_PRICE_CONFIRMED=true
SEEDANCE_DEFAULT_DURATION=5
SEEDANCE_MAX_DURATION=60
SEEDANCE_MAX_IMAGES=4
SEEDANCE_MAX_REFERENCE_VIDEOS=1
SEEDANCE_REMOTE_URL_ALLOWLIST=cdn.example.com,*.oss-cn-hangzhou.aliyuncs.com
SEEDANCE_CALLBACK_URL_ALLOWLIST=hooks.example.com
REQUIRE_REMOTE_ALLOWLIST=true
REQUIRE_CALLBACK_ALLOWLIST=false
```

说明：字节/豆包 API Key 当前主要通过后台渠道配置保存，例如渠道密钥格式为 API Key 或 `AccessKey|SecretAccessKey|Region`。如果要求“API Key 必须通过环境变量配置，不能写死”，后续应新增 `SEEDANCE_API_KEY` 或 `VOLCENGINE_ACCESS_KEY/VOLCENGINE_SECRET_KEY/VOLCENGINE_REGION` 环境变量注入渠道的能力，避免在数据库明文保存上游密钥。

### 6.3 Postgres 小内存建议

若 Postgres 与应用同机，建议在 compose 或 `postgresql.conf` 中控制：

```conf
shared_buffers = 128MB
effective_cache_size = 1GB
work_mem = 2MB
maintenance_work_mem = 64MB
max_connections = 40
wal_buffers = 8MB
```

同时应用侧必须控制：

```env
SQL_MAX_IDLE_CONNS=2
SQL_MAX_OPEN_CONNS=20
SQL_MAX_LIFETIME=300
```

### 6.4 不建议在 2C4G 做的事

| 不建议事项 | 原因 |
|---|---|
| 在服务器上执行 `bun install` / `npm install` / 前端构建 | `node_modules` 已观察约 1.3GB，构建内存也高 |
| 同机运行 Redis、Prometheus、Grafana、向量库 | 与目标功能无关，会挤占内存 |
| 开启全量数据大屏和高频自动同步 | 小机器上后台任务比业务请求更容易造成抖动 |
| 本机部署大模型或模型推理服务 | 目标只调用字节 API，2C4G 不适合推理 |
| 开放大文件上传和大请求体 | 容易被单请求打爆内存 |

## 7. 需要修改的文件清单

> 以下是后续 lean 分支建议修改的文件，不是本轮已修改内容。

| 文件/目录 | 修改目的 | 优先级 | 建议动作 |
|---|---|---:|---|
| `main.go` | 拆分后台任务开关 | P0 | 为自动渠道测试、Codex 刷新、MJ 轮询、Seedance worker 添加独立 env 开关 |
| `common/init.go` | 调整 lean 默认值 | P0 | 新增/调整 `CHANNEL_AUTO_TEST_ENABLED`、`CODEX_CREDENTIAL_REFRESH_ENABLED`、`MIDJOURNEY_TASK_WORKER_ENABLED`、`SEEDANCE_TASK_WORKER_ENABLED` |
| `docker-compose.yml` | 2C4G compose 不应默认 Redis | P0 | 新增 lean profile 或 `docker-compose.lean.yml`，只保留 app + tuned Postgres |
| `.env.example` | 给出小内存部署示例 | P0 | 新增 `.env.lean.example`，明确连接池、任务、请求体限制 |
| `router/relay-router.go` | 路由功能开关 | P1 | 只注册必要 relay/video 路由，移除 MJ/Suno/realtime/audio 等 |
| `router/video-router.go` | Seedance-only 视频路由 | P1 | 保留 Doubao/Seedance，隐藏 Kling/Jimeng/其他视频平台 |
| `relay/channel/*` | Provider 白名单 | P1 | 通过 registry 或 build tag 只编译 VolcEngine/Doubao |
| `controller/midjourney.go` | 移除 MJ 轮询 | P1 | 不启动、不注册、不编译 |
| `controller/channel-test.go` | 自动渠道测试开关 | P1 | 默认关闭，管理员手动测试 |
| `controller/channel_upstream_update.go` | 自动模型同步关闭 | P1 | 默认 false，后台手动同步 |
| `service/codex_credential_refresh_task.go` | Codex 刷新关闭 | P1 | 默认 false 或移除 |
| `service/task_polling.go` | 任务轮询限定范围 | P1 | 只查询 Seedance/Doubao 未完成任务，limit 默认 50 |
| `service/tokenizer.go` | token encoder 延迟加载 | P2 | 非文本模型只用 usage/固定价格，按需加载 tokenizer |
| `service/file_service.go`、`common/audio.go` | 禁用媒体下载/解析 | P2 | Seedance 只允许白名单 URL，不处理本地上传和媒体 token |
| `web/src/App.jsx` | 精简后台路由 | P1 | 只保留 Setup/Login/User/Token/TopUp/Channel/Log/Billing/SelfBilling/Setting 子集 |
| `web/src/components/layout/SiderBar.jsx` | 精简菜单 | P1 | 隐藏 Chat/Midjourney/Playground/ModelDeployment/Dashboard 等 |
| `web/src/pages/Billing` | 保留账单页并避免图表强依赖 | P1 | 账单表格优先，图表 lazy 或移除 |
| `web/src/i18n/locales` | 精简语言包 | P2 | lean 构建只保留中文或中英 |
| `web/package.json` | 删除重型前端依赖 | P2 | 移除 chart/mermaid/katex/electron/dropzone/telegram 等不需要依赖 |
| `go.mod` | 删除重型后端依赖 | P2 | provider 裁剪后移除 AWS、WebAuthn、音频解析、支付 SDK 等 |
| `Dockerfile` | lean 镜像 | P2 | 使用预构建 web/dist，运行层只放 binary + ca-certificates + tzdata |

## 8. 分阶段实施计划

### 阶段 0：只改部署配置，让 2C4G 先启动

1. 不在服务器构建前端和 Go。
2. 使用已构建镜像或本地/CI 构建后推送镜像。
3. 关闭 Redis：移除 `REDIS_CONN_STRING`，compose 不启动 Redis。
4. 调小数据库连接池：`SQL_MAX_IDLE_CONNS=2`、`SQL_MAX_OPEN_CONNS=20`。
5. 关闭上游模型同步：`CHANNEL_UPSTREAM_MODEL_UPDATE_TASK_ENABLED=false`。
6. 关闭批量更新：`BATCH_UPDATE_ENABLED=false`。
7. 限制请求体：`MAX_REQUEST_BODY_MB=8`、`STREAM_SCANNER_MAX_BUFFER_MB=16`。
8. Seedance 异步任务保留 `UPDATE_TASK=true`，但 `TASK_QUERY_LIMIT=50`。

验收标准：服务可启动，登录后台、创建用户、创建渠道、提交 Seedance 任务、查看账单均可用。

### 阶段 1：启动任务开关化

1. 新增 `CHANNEL_AUTO_TEST_ENABLED=false`。
2. 新增 `CODEX_CREDENTIAL_REFRESH_ENABLED=false`。
3. 拆分 `UPDATE_TASK` 为：
   - `SEEDANCE_TASK_WORKER_ENABLED=true`
   - `MIDJOURNEY_TASK_WORKER_ENABLED=false`
   - `OTHER_TASK_WORKER_ENABLED=false`
4. 通用任务轮询只查询 Seedance/Doubao 任务。

验收标准：启动日志中不再出现 MJ、Codex、上游同步等无关任务。

### 阶段 2：后端 provider 裁剪

1. 建立 provider registry 白名单，只注册 VolcEngine/Doubao。
2. 移除其他供应商 adaptor import。
3. 移除 MJ/Suno/Kling/Jimeng 等 task provider。
4. 回归 Seedance 任务提交、查询、失败退款、usage 差额结算。

验收标准：Go 二进制体积下降，`go test` 覆盖账单、Seedance adaptor、task polling。

### 阶段 3：前端 lean 后台

1. 精简路由：登录、初始化、用户、Token、充值/兑换码、渠道、日志、账单、个人账单、系统设置。
2. 删除或隐藏 Chat、Playground、Midjourney、模型市场、部署市场、大屏等页面。
3. 移除 chart/markdown/mermaid/electron/dropzone/telegram 等不需要依赖。
4. 只保留必要语言包。

验收标准：后台首屏更小，账单页可用，用户和渠道管理可用。

### 阶段 4：密钥与计费生产化

1. 新增从环境变量注入 Seedance 上游密钥的机制。
2. 上游密钥入库前必须加密，或 lean 模式下不入库。
3. 开启 `SEEDANCE_REQUIRE_PRICE_CONFIRMATION=true` 和 `SEEDANCE_PRICE_CONFIRMED=true`。
4. 配置素材 URL 白名单和 callback URL 白名单。
5. 明确固定价格兜底与 usage 差额结算策略。

验收标准：未确认价格无法上线；密钥不明文写入配置文件；账单可追溯。

### 阶段 5：生产部署与观测

1. 使用云监控或轻量日志，不在 2C4G 本机部署完整观测栈。
2. 配置 Nginx/Caddy HTTPS 反代。
3. 开启数据库备份。
4. 使用 k6 做小规格压测：10/30/50 并发阶梯，不追求高峰值，验证内存稳定。
5. 设置 OOM 重启告警、磁盘告警、DB 连接数告警。

验收标准：连续 24 小时运行无 OOM，任务失败可退款，账单导出正确。

## 9. 风险点

| 风险 | 严重度 | 说明 | 缓解措施 |
|---|---|---|---|
| 只靠配置关闭仍有无关代码编译进 binary | 中 | 不影响启动太多，但二进制和攻击面仍偏大 | 阶段 2 做 provider 裁剪 |
| `UPDATE_TASK` 当前同时牵连 MJ 与通用任务 | 高 | Seedance 需要 worker，但 MJ 不需要 | 尽快拆任务开关 |
| 关闭 Redis 后高并发能力下降 | 中 | 单机 2C4G 本身不适合高并发 | MVP 接受；规模上来外置 Redis |
| 关闭批量更新后 DB 写入增加 | 中 | 低并发影响可控，高并发可能 DB 压力上升 | 先小流量；后续外置 Redis/队列/批处理 |
| 使用 SQLite 正式商用有一致性风险 | 高 | 并发写、备份、审计不如 Postgres | 正式客户建议 Postgres |
| 在线支付裁剪过早可能影响充值闭环 | 中 | 如果客户要微信/支付宝/Stripe 自动充值，需要保留对应支付 | MVP 先管理员充值，支付需求明确后只接一个 |
| 上游密钥当前通过后台渠道配置 | 高 | 若未加密或权限泄漏，API Key 风险高 | 使用 `CRYPTO_SECRET`，后续支持 env 注入和 KMS |
| 素材 URL 未做业务白名单会有 SSRF/成本风险 | 高 | Seedance 可能拉取图片/视频 URL | 必须配置 `SEEDANCE_REMOTE_URL_ALLOWLIST` |
| 大请求体仍可能造成 OOM | 高 | 视频/图片输入若开放 base64 或大文件 | 限制请求体、禁止本地上传、只允许 URL |
| AGPL/商业授权风险 | 中 | 开源项目商用 SaaS 修改版需遵守 AGPL 网络交互源码提供义务，闭源交付需商业授权 | 继续保留合规文档、变更记录、SBOM；闭源需授权凭证 |

## 10. 预计优化后的部署方式

### 10.1 推荐 lean compose 形态

后续建议新增 `docker-compose.lean.yml`，结构如下：

```yaml
services:
  new-api:
    image: leia/new-api:seedance-lean
    restart: unless-stopped
    ports:
      - "127.0.0.1:3000:3000"
    env_file:
      - .env.lean
    volumes:
      - ./logs:/app/logs
    depends_on:
      - postgres
    mem_limit: 768m

  postgres:
    image: postgres:15-alpine
    restart: unless-stopped
    environment:
      POSTGRES_USER: newapi
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
      POSTGRES_DB: newapi
    command:
      - "postgres"
      - "-c"
      - "max_connections=40"
      - "-c"
      - "shared_buffers=128MB"
      - "-c"
      - "work_mem=2MB"
      - "-c"
      - "maintenance_work_mem=64MB"
    volumes:
      - pg_data:/var/lib/postgresql/data
    mem_limit: 768m

volumes:
  pg_data:
```

说明：

- 不启动 Redis。
- 不启动向量库、模型服务、worker 独立进程、Prometheus/Grafana。
- 前端由 Go binary 嵌入或由 Nginx 静态托管，不在服务器安装 `node_modules`。
- 字节 API Key 通过环境变量或加密渠道配置注入，不能写死在源码和 compose 文件中。

### 10.2 预计资源占用

| 组件 | 目标内存范围 | 说明 |
|---|---:|---|
| NewAPI lean 后端 + 嵌入后台 | 150MB - 350MB | 取决于并发、请求体、tokenizer 和任务数量 |
| Postgres tuned | 200MB - 700MB | 取决于连接数、数据量和查询 |
| Nginx/Caddy | 20MB - 80MB | 可选 |
| OS + page cache | 800MB - 1500MB | 4GB 机器必须预留 |
| 预留峰值 | 1GB+ | 防止任务高峰和备份时 OOM |

### 10.3 最小上线验收清单

1. 服务启动后 RSS 稳定低于 500MB，Postgres 稳定低于 800MB。
2. `/api/status` 正常。
3. 管理员能登录后台。
4. 能创建普通用户并充值。
5. 能创建 API Token。
6. 能配置 Seedance 2.0 字节渠道，密钥不写死。
7. 能提交 Seedance 任务并查询状态。
8. 成功任务能扣费，失败任务能退款。
9. 管理员账单页能按用户/模型/时间查询。
10. 用户个人账单页能看到自己的消耗。
11. `/api/setup` 初始化完成后不可被公网滥用。
12. 24 小时无 OOM，无异常后台任务刷屏。

## 11. 结论

当前 NewAPI 已具备“账户、调用计量、充值/订阅额度、Seedance 2.0、账单后台”的业务基础，但默认形态是多供应商、多功能、全量后台的通用平台，不适合直接以默认 compose 部署在 2C4G 小机器上。

短期最小可行方案不是先删代码，而是先做配置级瘦身：关闭 Redis、关闭上游模型同步、关闭批量更新、调小数据库连接池、限制请求体、只保留 Seedance 任务轮询。这样可以较低风险让 2C4G 先稳定启动。

中期应在独立 lean 分支做代码级裁剪：拆后台任务开关、只注册字节/Seedance provider、精简前端后台、移除不必要依赖。完成后可形成专门面向“Seedance 计费 SaaS”的轻量发行版，内存、攻击面、维护成本都会明显降低。
