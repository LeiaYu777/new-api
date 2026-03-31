# NewAPI 多租户 AIGC 平台用户手册（基于现有代码）

本文档面向最终用户与平台管理员，介绍 NewAPI 的核心能力、部署初始化、控制台操作流程、API 使用方式、计量计费与充值订阅、任务队列（轮询）以及监控告警与常见问题排查。

适用对象：
1. 平台运维/管理员（Root/Admin）：部署系统、配置渠道与模型、配置支付与登录、查看全局日志与性能指标
2. 普通用户（Common）：创建令牌、调用 API、查看用量/账单、充值或订阅
3. 多租户集成方（SaaS/网关调用方）：通过租户 Header/参数隔离不同客户的调用链路与审计

说明：
1. 本手册基于仓库当前实现（`/Users/leia/Codex/new-api/new-api`）生成。
2. 当前代码提供了“租户上下文 + 写操作审计日志”的多租户基础能力；租户的“创建/禁用/配额”管理 UI/接口并未在路由中暴露（需要通过配置与数据库运维方式管理，或后续二次开发补齐）。

---

## 1. 概述

### 1.1 系统定位
NewAPI 是一个面向多模型/多渠道的 AIGC 网关与控制台系统：
1. 对外提供 OpenAI 风格兼容接口（如 `/v1/chat/completions` 等）用于统一接入
2. 对内提供控制台，用于用户/令牌/渠道/模型/倍率/计费/订阅/充值/日志/任务等管理
3. 通过 Redis 与本地/内存缓存提高吞吐，通过“任务轮询”支持部分异步类模型任务
4. 支持基础多租户隔离（`tenant_id` 链路）与租户写操作审计（`tenant_audit_logs`）

### 1.2 核心概念

**用户角色（RBAC 简化版）**
1. Root：平台最高权限（系统设置、渠道密钥读取等受保护动作）
2. Admin：平台管理员（管理用户/渠道/令牌/模型等大部分资源）
3. Common：普通用户（创建自己的令牌、查看自己的用量与账单、发起调用）
4. Guest：受限用户（具体能力取决于配置）

**令牌（Token / API Key）**
1. 用户在控制台创建“令牌”，用于对外 API 调用鉴权
2. 通常以 `Authorization: Bearer <token>` 方式使用

**渠道（Channel）**
1. 渠道代表一个上游服务配置（厂商类型、Base URL、API Key、模型能力、倍率等）
2. 控制台用于新增渠道并为模型启用/禁用

**模型（Models / Model Meta）**
1. 模型列表可来自系统内置或从上游同步
2. 可对不同模型设置倍率、限制与展示

**计量计费**
1. 系统记录调用日志与用量（Token/额度）
2. 支持钱包（充值）与订阅两类资金来源（实现以“计费会话”封装）

**多租户（Tenant）**
1. 在启用多租户后，每个请求会被解析出 `tenant_id` 并注入到请求上下文
2. 写操作（POST/PUT/PATCH/DELETE）可写入租户审计表
3. 目前没有对所有资源做“数据库层强制 tenant 过滤”，因此多租户更偏向“调用链路隔离 + 审计”，而不是完整的“强隔离 SaaS”。

---

## 2. 初始化部署

### 2.1 方式 A：Docker Compose（推荐）

文件位置：
1. `docker-compose.yml`：单实例（new-api + redis + postgres）
2. `.env.example`：环境变量示例

步骤（示例）：
1. 在部署目录准备 `.env`（参考 `.env.example`），至少建议设置：
   1. `POSTGRES_PASSWORD`：数据库密码
   2. `SESSION_SECRET`：会话密钥（必须是强随机字符串）
   3. `CRYPTO_SECRET`：敏感字段加密密钥（建议与 `SESSION_SECRET` 不同）
   4. `SETUP_TOKEN`：初始化令牌（用于远程初始化时保护 `/api/setup`）
   5. `CORS_ALLOW_ORIGINS`：控制台域名与开放 API 的允许来源
   6. `TRUSTED_PROXIES`：反向代理/网关 IP 段（如使用 Nginx/Ingress）
2. 启动：
```bash
cd /Users/leia/Codex/new-api/new-api
docker compose up -d
```
3. 打开控制台：`http://127.0.0.1:3000`

安全提示：
1. `docker-compose.yml` 默认绑定到 `127.0.0.1`，建议保持此策略，通过反向代理对外暴露
2. 生产环境务必替换所有默认/弱口令（包含数据库密码、会话密钥等）

### 2.2 方式 B：高可用角色拆分（控制面/转发面/Worker）

文件：`docker-compose.ha.yml`

角色说明（示意）：
1. Control：控制台与管理 API（用户、渠道、模型、计费配置等）
2. Relay：高频转发路径（对外 `/v1` 等模型转发）
3. Worker：后台任务（例如通知发送、异步轮询类任务的处理，取决于配置）

操作建议：
1. 使用相同的 `SQL_DSN`、`REDIS_CONN_STRING`、`SESSION_SECRET`、`CRYPTO_SECRET`
2. 按需对不同角色进行独立扩容（例如 Relay 多副本）
3. 通过网关/Ingress 将不同路径路由到不同角色（例如 `/api/*` 到 Control，`/v1/*` 到 Relay）

### 2.3 初始化接口（/api/setup）

接口：
1. `GET /api/setup`：查看初始化状态
2. `POST /api/setup`：执行初始化（创建 Root 用户、写入初始化记录、保存模式开关）

访问限制（重要）：
1. 系统未初始化时，`/api/setup` 默认仅允许内网/回环地址访问
2. 如设置 `SETUP_TOKEN`，则必须提供：
   1. Header：`X-Setup-Token: <SETUP_TOKEN>`
   2. 或 query：`?setup_token=<SETUP_TOKEN>`

初始化步骤（示例）：
1. 打开首页，按提示进入初始化页面（或直接调用接口）
2. 提交管理员账号与密码（密码至少 8 位）
3. 初始化成功后，`/api/setup` 将不可重复执行

### 2.4 本地开发（前端）

前端目录：`web/`

常见命令（示例）：
```bash
cd /Users/leia/Codex/new-api/new-api/web
npm install
npm run dev
```

说明：
1. 前端默认通过同源方式访问后端（开发环境可能需要代理配置，具体以 `web` 项目配置为准）
2. 生产部署通常由后端静态托管 `web/dist`（取决于构建流程）

---

## 3. 功能使用流程

本章节按控制台模块给出“从登录到操作完成”的步骤说明。不同版本的界面文案可能略有差异，但入口与资源概念一致。

### 3.0 控制台导航（页面与权限）

常见控制台路由（示意）：
1. `/console`：仪表盘（用户与管理员均可）
2. `/console/token`：令牌管理（用户与管理员均可，管理员可视情况管理更多）
3. `/console/channel`：渠道管理（管理员）
4. `/console/models`：模型管理（管理员）
5. `/console/topup`：充值（用户；管理员可在相关页面查看全局记录）
6. `/console/subscription`：订阅（用户；管理员可在后台管理计划与用户订阅）
7. `/console/log`：日志（用户可看个人日志；管理员可看全局日志）
8. `/console/task`：任务（用户可看个人任务；管理员可看全局任务）
9. `/console/user`：用户管理（管理员）
10. `/console/setting`：系统设置（Root 为主，部分项可能对 Admin 开放）
11. `/console/personal`：个人设置（所有登录用户）

“文本替代截图”（示意）：
1. 左侧菜单：仪表盘 | 渠道 | 模型 | 令牌 | 充值 | 订阅 | 日志 | 任务 | 用户 | 设置 | 个人中心
2. 页头区域：当前用户 | 退出登录 | 快捷入口（取决于配置）

### 3.1 登录与个人中心

#### 3.1.1 注册与登录
1. 打开网站首页
2. 若平台允许注册，点击“注册”，输入用户名/邮箱（取决于配置）与密码
3. 登录后进入控制台（通常在 `/console`）

#### 3.1.2 2FA 与 Passkey（可选）
1. 进入“个人中心/安全设置”
2. 按提示开启二步验证（2FA）或 Passkey
3. 保存备用恢复码（如系统提供）

#### 3.1.3 通知与告警（Webhook/Bark/Gotify）
1. 进入“个人中心/通知设置”
2. 选择通知方式：
   1. Webhook：填写 `Webhook URL` 与 `Webhook Secret`
   2. Bark：填写 Bark 推送地址
   3. Gotify：填写 Gotify 地址与 Token，并设置优先级
3. 保存后，系统在余额不足/异常等场景会按配置推送（具体触发逻辑以平台策略为准）

### 3.2 租户管理（多租户基础能力）

#### 3.2.1 启用多租户
多租户由环境变量控制：
1. `MULTI_TENANT_ENABLED=true`
2. `DEFAULT_TENANT_ID=default`（可选）
3. `TENANT_HEADER_KEY=X-Tenant-Id`（可选）
4. `TENANT_AUDIT_ENABLED=true`（可选）

#### 3.2.2 客户端如何传递 tenant_id
当启用多租户后，系统会按优先级解析 `tenant_id`：
1. 请求上下文中的 `tenant_id`（上游中间件注入时）
2. Header：`X-Tenant-Id`（可通过 `TENANT_HEADER_KEY` 改名）
3. Query：`?tenant_id=xxx`
4. 否则使用默认租户 `DEFAULT_TENANT_ID`

示例（API 调用）：
```bash
curl -sS http://api.example.com/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer YOUR_TOKEN' \
  -H 'X-Tenant-Id: tenant_acme' \
  -d '{"model":"gpt-4.1-mini","messages":[{"role":"user","content":"hello"}]}'
```

#### 3.2.3 租户审计（写操作）
当启用 `TENANT_AUDIT_ENABLED=true` 且多租户开启时：
1. 对非 GET/HEAD/OPTIONS 的请求写入审计日志
2. 记录：`tenant_id`、`user_id`、方法、路由、状态码、IP 等

运维侧可在数据库中查询 `tenant_audit_logs` 进行审计与追踪。

说明（当前限制）：
1. 当前路由中未提供租户的“创建/禁用/配额调整”管理 API/UI
2. 多租户更适用于“请求链路隔离 + 审计”，如需强隔离（所有数据按 tenant 过滤），需要进行更深度的数据库与业务改造

运维侧“租户创建/禁用”的可落地做法（通过数据库）：
1. 创建租户（示例 SQL，字段名以实际数据库表为准）：
```sql
INSERT INTO tenants (id, name, status, created_time, updated_time)
VALUES ('tenant_acme', 'Acme Corp', 1, EXTRACT(EPOCH FROM NOW())::bigint, EXTRACT(EPOCH FROM NOW())::bigint);
```
2. 禁用租户（示例）：
```sql
UPDATE tenants
SET status = 2, updated_time = EXTRACT(EPOCH FROM NOW())::bigint
WHERE id = 'tenant_acme';
```
3. 查询租户写操作审计（示例）：
```sql
SELECT id, tenant_id, user_id, action, resource, status, created_at
FROM tenant_audit_logs
WHERE tenant_id = 'tenant_acme'
ORDER BY id DESC
LIMIT 50;
```

### 3.3 用户管理与 RBAC

#### 3.3.1 普通用户（Common）操作
1. 登录控制台
2. 创建令牌（见 3.4）
3. 绑定 OAuth/OIDC（如平台开启）
4. 查看用量/充值/订阅（见 3.6/3.7）

#### 3.3.2 管理员（Admin/Root）操作：创建与管理用户
入口通常在“控制台 -> 用户管理（/console/user）”：
1. 查看用户列表
2. 搜索用户
3. 创建用户（指定角色、状态、初始额度等）
4. 禁用/启用用户、重置安全绑定（2FA/Passkey 等，若界面提供）

### 3.4 令牌管理（API Key）

入口：控制台 “令牌（/console/token）”

常见流程：
1. 点击“新建令牌”
2. 设置令牌名称、权限/范围（若有）、额度/限速策略（若有）
3. 保存后复制令牌 Key（用于 API 调用）
4. 若怀疑泄露，立即禁用/删除并重新生成

只读用量查询接口（示例）：
1. `GET /api/usage/token`（通过 `TokenAuthReadOnly` 鉴权）

### 3.5 渠道管理（上游厂商接入）

入口：控制台 “渠道（/console/channel）”（管理员权限）

常见流程：
1. 点击“新增渠道”
2. 选择渠道类型（厂商）
3. 填写上游 API Key、Base URL、模型能力、权重/倍率等
4. 保存后执行“测试渠道/更新余额/拉取模型”（界面上通常提供）

从 0 到可用的最短路径（管理员操作建议）：
1. 新增渠道并保存
2. 在渠道列表中点击“测试”，确认连通性与鉴权正确
3. 同步模型（若渠道支持上游模型同步）
4. 进入“模型管理”确认模型已出现，并为目标模型设置倍率/启用状态
5. 在“用户管理”检查目标用户的分组/权限是否允许使用该模型（如果平台按分组控制模型）
6. 指导用户创建令牌并开始调用

安全注意：
1. 渠道密钥属于高敏信息，通常仅 Root 可读取明文
2. 建议将渠道密钥由组织的密钥管理系统托管（以环境变量/密文注入方式），避免在浏览器侧或日志中泄露

### 3.6 模型管理（模型元信息/启用/倍率）

入口：控制台 “模型（/console/models）”（管理员权限）

常见流程：
1. 查看系统模型列表
2. 进行上游同步（preview/执行同步，视界面能力而定）
3. 对模型设置：
   1. 启用/禁用
   2. 价格/倍率显示
   3. 分组/标签（若支持）

模型上线检查清单（管理员）：
1. 模型名与上游一致（避免路由失败）
2. 倍率/价格展示正确（避免计费误解）
3. 该模型是否允许在用户分组中可见（若系统启用分组/白名单机制）
4. 在 Playground 页面进行一次真实调用验证（见 5.2）

### 3.7 计量计费、充值与账单

#### 3.7.1 用量查看（Usage）
入口：控制台 Dashboard 或用量页面

后端接口示例：
1. `GET /api/dashboard/billing/usage`
2. `GET /api/v1/dashboard/billing/usage`（兼容路径）

你可以查看：
1. 近期消耗
2. 模型维度消耗（取决于统计口径）
3. 用量日志（管理员可查看全局，用户可查看自身）

#### 3.7.2 充值（TopUp）
入口：控制台 “充值（/console/topup）”

流程（示例）：
1. 选择充值金额/充值档位
2. 选择支付方式（Stripe/Creem/Waffo/Epay 等，取决于平台启用）
3. 完成支付后查看充值记录（可在“充值历史”）

管理员侧：
1. 可在用户管理或充值管理页面查看所有充值记录
2. 部分支付方式可能需要管理员“确认完成”（取决于支付网关回调与平台策略）

支付网关配置（Root/管理员，入口通常在“设置 -> 支付”）：
1. Stripe：
   1. 填写 Stripe Key/Secret（敏感信息通常会被隐藏显示）
   2. 在 Stripe 控制台配置 Webhook 地址（示意）：`https://your-domain/api/stripe/webhook`
   3. 保存并进行一次小额测试
2. Creem/Waffo/Epay：
   1. 按页面提示填写回调地址与签名密钥
   2. 确保回调地址可被支付平台访问（公网可达 + HTTPS）
   3. 检查 `TRUSTED_REDIRECT_DOMAINS`（若平台启用回调 URL 域名校验）

#### 3.7.3 订阅（Subscription）
入口：控制台 “订阅（/console/subscription）”

流程（示例）：
1. 查看订阅计划（Plan）
2. 购买/续费
3. 配置计费偏好（subscription_first / wallet_first 等，取决于实现与界面）
4. 查看当前订阅状态与到期时间

说明：
1. 系统在扣费时会根据用户偏好选择资金来源，并支持必要的回退策略

### 3.8 API 调度与任务队列（轮询）

系统既支持同步请求，也对部分平台/模型提供任务提交与轮询获取结果的方式（例如音频/视频/图片等异步类任务，具体取决于渠道类型）。

控制台入口：
1. “任务（/console/task）”：查看任务状态、结果、错误信息（权限决定可见范围）

API 路由示例（不同平台的 task 路由可能不同）：
1. 提交任务：`POST /relay/.../submit/...`（由路由适配不同平台）
2. 获取结果：`GET/POST /relay/.../fetch...`

运维与性能建议：
1. 轮询模式强依赖 Redis 与内存缓存，否则性能会显著下降
2. 可通过环境变量调整轮询频率（`POLLING_INTERVAL`）与任务超时策略（如 `TASK_TIMEOUT_MINUTES`）

### 3.9 监控与告警

#### 3.9.1 服务健康与状态面板
1. `GET /api/status`：基础健康检查
2. 控制台仪表盘通常会展示：
   1. 调用成功率/错误率（取决于统计口径）
   2. 最近用量
   3. 外部状态面板集成（如 Uptime Kuma）

#### 3.9.2 Uptime Kuma 集成（可选）
用途：
1. 将你的服务可用性（或相关依赖）在控制台中以分组方式展示

配置步骤（Root/管理员）：
1. 进入“设置 -> 仪表盘/监控（Uptime Kuma）”
2. 配置 Uptime Kuma 的访问地址与分组（最多 20 个）
3. 保存后回到仪表盘查看状态卡片

#### 3.9.3 性能监控与保护（拒绝新转发）
用途：
1. 当系统 CPU/内存/磁盘使用率超过阈值时，拒绝新的高频转发请求，以保护系统稳定性

配置步骤（Root）：
1. 进入“设置 -> 性能”
2. 启用“性能监控”
3. 配置阈值（CPU/内存/磁盘）
4. 保存后可在“性能指标”中查看统计，并可执行：
   1. 清理磁盘缓存
   2. 重置统计
   3. 触发 GC
   4. 下载/清理日志（取决于界面开关）

#### 3.9.4 告警通知（Webhook/Bark/Gotify）
用途：
1. 当额度不足、异常错误或其他事件触发时向用户推送通知（以平台策略为准）

配置步骤（用户）：
1. 进入“个人中心 -> 通知设置”
2. 填写通知渠道配置并保存
3. 进行一次测试（例如触发额度阈值，或管理员手动触发通知，具体能力取决于平台版本）

---

## 4. 注意事项与最佳实践

### 4.1 多租户隔离最佳实践
1. 统一在网关层注入 `X-Tenant-Id`，避免由浏览器端可控输入
2. 为每个租户单独签发令牌（Token），并在发放时绑定租户策略（如需强隔离，建议二次开发实现 DB 级过滤）
3. 开启 `TENANT_AUDIT_ENABLED=true` 记录写操作，配合数据库审计与集中日志（ELK/Loki）

### 4.2 Token 计量与账单周期
1. 生产环境建议明确账单周期（按月/按自然周/按订阅重置周期）
2. 对企业客户建议提供：
   1. 固定配额 + 超额计费
   2. 额度预警通知（Webhook/Bark/Gotify）
3. 建议使用只读 Token 用量接口为外部看板提供数据

### 4.3 队列与缓存策略
1. Redis 建议开启持久化策略（按你的 SLO 选择 AOF/RDB）
2. 热点元数据（模型倍率、渠道健康、路由策略）建议缓存命中率监控
3. 如果需要更高吞吐的异步任务，建议引入消息队列（Redis Streams/NATS/Kafka）替代轮询

### 4.4 权限控制
1. 渠道密钥读取等高危动作建议仅 Root 允许
2. 启用 2FA/Passkey，至少对 Admin/Root 强制
3. 与企业 IdP 集成时建议使用 OIDC 并关闭弱口令注册入口（视业务策略）

### 4.5 高并发与稳定性
1. 部署在反向代理后时正确配置 `TRUSTED_PROXIES`，避免客户端 IP 与限流策略失效
2. 生产环境严格设置 `CORS_ALLOW_ORIGINS`，避免任意来源调用控制台 API
3. 通过“性能监控”功能在资源超阈值时拒绝新转发请求，以保护系统稳定性（控制台可配置阈值）

---

## 5. 操作示例与命令

### 5.1 生成/获取用户 Access Token（用于网页会话之外的访问）
示例接口：`GET /api/user/self/token`（需要登录态）

### 5.2 OpenAI 兼容调用示例（Chat Completions）
```bash
curl -sS http://127.0.0.1:3000/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer YOUR_TOKEN' \
  -d '{
    "model":"gpt-4.1-mini",
    "messages":[{"role":"user","content":"用一句话介绍 NewAPI"}],
    "stream": false
  }'
```

### 5.3 多租户调用示例
```bash
curl -sS http://127.0.0.1:3000/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer YOUR_TOKEN' \
  -H 'X-Tenant-Id: tenant_demo' \
  -d '{"model":"gpt-4.1-mini","messages":[{"role":"user","content":"hello"}]}'
```

### 5.4 健康检查
1. `GET /api/status`：服务状态
2. Docker Compose healthcheck 也会请求 `http://localhost:3000/api/status`

---

## 6. 配置说明（环境变量与关键参数）

以下为常用关键项（以 `.env.example` 与 `docker-compose.yml` 为准）：

### 6.1 基础
1. `APP_PORT` / `PORT`：服务端口（容器内默认 3000）
2. `BIND_HOST`：对外绑定地址（compose 默认 `127.0.0.1`）

### 6.2 数据库
1. `SQL_DSN`：数据库连接串（PostgreSQL/MySQL/SQLite 三选一）
2. `SQL_MAX_IDLE_CONNS` / `SQL_MAX_OPEN_CONNS` / `SQL_MAX_LIFETIME`：连接池参数（视压测调整）

### 6.3 Redis 与缓存
1. `REDIS_CONN_STRING`：Redis 连接串
2. `MEMORY_CACHE_ENABLED`：是否启用内存缓存

### 6.4 安全与初始化
1. `SESSION_SECRET`：会话密钥（必须强随机）
2. `CRYPTO_SECRET`：敏感字段加密密钥（建议独立于 `SESSION_SECRET`）
3. `SETUP_TOKEN`：初始化保护令牌（保护 `/api/setup` 远程访问）
4. `SETUP_ALLOW_PRIVATE_IP`：未设置 token 时是否允许内网 IP 初始化（默认 true）

关于 KMS/HSM：
1. 当前实现主要通过 `CRYPTO_SECRET` 做应用层对称加密
2. 推荐将 `CRYPTO_SECRET` 存储在组织的密钥管理系统中（KMS/HSM/Secrets Manager），以“运行时注入”的方式提供给容器，并建立轮换流程

### 6.5 CORS 与代理
1. `CORS_ALLOW_ORIGINS`：允许的来源域名（逗号分隔）
2. `TRUSTED_PROXIES`：可信代理 IP/网段（逗号分隔）

### 6.6 多租户
1. `MULTI_TENANT_ENABLED`：是否启用多租户
2. `DEFAULT_TENANT_ID`：默认租户 ID
3. `TENANT_HEADER_KEY`：租户 header 名称（默认 `X-Tenant-Id`）
4. `TENANT_AUDIT_ENABLED`：是否启用租户写操作审计

### 6.7 任务轮询与超时
1. `POLLING_INTERVAL`：轮询间隔（秒）
2. `TASK_TIMEOUT_MINUTES`：任务超时（分钟）

### 6.8 SSO（OIDC）
系统支持 OIDC 登录（例如 Okta/Auth0 等）：
1. 在控制台的系统设置中填写 OIDC 配置（Client ID/Secret、授权端点、Token 端点、UserInfo 端点或 Well-Known URL）
2. 启用后，登录页会出现 “使用 OIDC 继续”

---

## 7. 常见问题与解决方案

### 7.1 数据库连接失败
现象：
1. 控制台无法登录
2. `api/status` 报错或启动失败

排查：
1. 检查 `SQL_DSN` 是否正确（用户名、密码、host、db）
2. 确认数据库容器/实例可达（网络、端口、DNS）
3. 检查连接池是否过小导致大量超时（高并发下尤为明显）

建议：
1. 使用 PostgreSQL 时建议配合连接池（如 PgBouncer）并调优 `SQL_MAX_OPEN_CONNS`

### 7.2 Redis 连接异常 / 缓存失效
现象：
1. 轮询任务性能下降
2. 命中率低导致延迟增加

排查：
1. 检查 `REDIS_CONN_STRING`
2. 查看 Redis 是否 OOM、是否触发频繁淘汰
3. 检查容器网络连通性

建议：
1. 为 Redis 设置合理内存与淘汰策略
2. 关键路径启用内存缓存（如业务允许）

### 7.3 任务轮询失败 / 卡住
现象：
1. 控制台任务一直处于 pending/running
2. fetch 接口返回超时或无结果

排查：
1. 检查 Redis 是否可用（轮询模式依赖缓存）
2. 检查 `POLLING_INTERVAL` 是否过大
3. 查看任务相关日志与错误码

建议：
1. 对异步任务引入消息队列替代轮询（当任务量大时）

### 7.4 监控/告警不生效（Uptime Kuma/通知）
排查：
1. 确认已在控制台启用 Uptime Kuma 并配置分组
2. 确认 Webhook/Bark/Gotify 参数正确，且服务端可访问对应地址
3. 若启用 SSRF 防护，需确保目标地址符合允许策略

### 7.5 初始化接口无法访问（/api/setup）
原因：
1. 未设置 `SETUP_TOKEN` 且访问来自公网 IP（非内网/回环）

解决：
1. 在内网访问初始化，或设置 `SETUP_TOKEN` 并通过 `X-Setup-Token` 提供
