# Seedance 2.0 充值计费改造方案

## 1. 目标

本方案面向一个明确客户场景：

用户在平台充值或购买订阅后，通过 NewAPI 调用字节/火山方舟的 Seedance 2.0 视频生成模型；系统按实际消耗扣除余额或订阅额度，管理员可以在后台查看用户、模型、渠道、订单和账单明细。

本方案不做多方资金清分、提现、代理分润或财务代付。这里的“分账”定义为平台内部账本能力：充值入账、订阅额度、调用扣费、失败退款、账单可查。

## 2. 现有能力评估

当前代码已经具备以下可复用能力：

1. 钱包余额扣费：`service.WalletFunding` 支持预扣、补扣、退款。
2. 订阅额度扣费：`service.SubscriptionFunding` 支持订阅预扣、差额结算、失败退款。
3. 异步任务计费：`service/task_polling.go` 在任务完成后可根据 `total_tokens` 做二次结算。
4. 充值订单：`model.TopUp` 已支持充值订单、支付状态、管理员补单。
5. 订阅计划：`model.SubscriptionPlan`、`model.UserSubscription` 已支持套餐、周期、额度、用户订阅实例。
6. 消费日志：`model.Log` 可记录用户、模型、渠道、Token、额度、分组等账单字段。
7. Doubao/VolcEngine 视频任务适配基础：已有 `ChannelTypeDoubaoVideo`、`ChannelTypeVolcEngine` 和 `relay/channel/task/doubao` 异步任务适配器。

当前已完成的代码能力：

1. Seedance 2.0 系列模型名、模型识别和模型映射兜底已经加入。
2. Seedance 2.0 任务参数解析、参数白名单、图片数量限制和固定价格兜底已经加入。
3. 钱包余额、订阅额度、Token 额度的任务预扣、补扣、退款链路已经打通。
4. 管理员账单页、汇总 API、CSV 导出和资金来源审计字段已经加入。
5. Smoke test 脚本和验收手册已经加入。

当前剩余缺口：

1. Seedance 2.0 官方参数、返回结构、任务状态、usage 字段仍需使用客户真实火山方舟账号联调验证。
2. Seedance 2.0 生产价格、订阅套餐、充值支付方式仍需按客户业务配置。
3. 外部图片和 callback URL 已接入基础 SSRF 校验；生产环境仍建议配置客户素材域名 allowlist 或对象存储中转策略。
4. 客户账单周期报表、资金来源维度汇总和任务列表体验增强仍属于后续产品化事项。
5. 详细剩余任务见 `docs/SEEDANCE_2_BILLING_REMAINING_TASKS.md`。

## 3. MVP 范围

MVP 只实现客户现在需要的闭环：

1. 管理员配置火山方舟 Seedance 2.0 渠道。
2. 管理员配置 Seedance 2.0 模型价格。
3. 用户充值或购买订阅。
4. 用户调用 `/v1/video/generations` 或 `/v1/videos` 提交 Seedance 2.0 任务。
5. 系统先预扣额度。
6. 任务成功后按上游返回 usage 或平台计价规则差额结算。
7. 任务失败、取消或超时后退款。
8. 管理员可按用户、模型、渠道、时间查询账单。

## 4. 分支策略

建议从当前主线拉独立分支：

```bash
git checkout -b codex/seedance-billing-plan leia/main
```

所有改造先进入 `codex/seedance-billing-plan`，验证通过后再发 PR 合并到 `main`。

当前方案文档先放在：

```text
docs/SEEDANCE_2_BILLING_PLAN.md
```

## 5. 代码改造清单

### 5.1 Seedance 2.0 模型注册

文件：

```text
relay/channel/task/doubao/constants.go
relay/channel/volcengine/constants.go
```

改造：

1. 新增 Seedance 2.0 官方模型名。
2. 允许管理员通过模型映射把平台展示名映射到上游真实模型名。
3. 不在代码里写死价格，以后台倍率/固定价格配置为准。

示例：

```go
var ModelList = []string{
    "doubao-seedance-1-0-pro-250528",
    "doubao-seedance-1-5-pro-251215",
    "doubao-seedance-2-0",          // 以官方最终模型名为准
    "doubao-seedance-2-0-pro",      // 以官方最终模型名为准
}
```

注意：最终模型名必须以火山方舟控制台开通后的真实模型 ID 为准，不要仅凭公开文章写死。

### 5.2 Seedance 2.0 请求参数适配

文件：

```text
relay/channel/task/doubao/adaptor.go
relay/common/relay_utils.go
```

现有适配器已经支持：

1. `prompt`
2. `image` / `images`
3. `resolution`
4. `ratio`
5. `duration`
6. `seed`
7. `generate_audio`
8. `watermark`
9. `camera_fixed`
10. `reference_image_url` / `reference_image_urls`
11. `first_frame_url` / `last_frame_url`
12. `reference_video_url` / `reference_video_urls`

需要补齐：

1. 官方 Seedance 2.0 真实请求样例的字段名和 `content.role` 取值仍需联调确认。
2. 虚拟人像 URI、音频参考等更细字段仍需以官方文档和客户开通能力为准。
3. 参数白名单与取值校验，避免客户提交不可计费或不可控参数。
4. 远程图片、视频和 callback URL 统一走系统 Fetch/SSRF 策略，拒绝私有 IP、危险端口、非 HTTP(S) 协议和带账号密码的 URL。

建议新增一个专门的转换函数：

```go
func (a *TaskAdaptor) convertSeedance2Request(req *relaycommon.TaskSubmitReq) (*requestPayload, error)
```

触发条件：

```go
strings.Contains(req.Model, "seedance-2")
```

### 5.3 Seedance 2.0 计价策略

文件：

```text
setting/ratio_setting/model_ratio.go
service/task_billing.go
service/task_polling.go
relay/channel/task/doubao/adaptor.go
```

推荐两种计价模式并存：

1. Token 计价：上游返回 `usage.total_tokens` 时，按模型倍率和用户分组倍率结算。
2. 固定/阶梯计价：上游不返回稳定 usage 时，按 `duration + resolution + generate_audio + service_tier` 计算平台价格。

MVP 推荐策略：

1. 先配置固定价格 `model_price` 作为预扣费，保证客户不会免费提交高成本任务。
2. 如果任务完成返回 `usage.total_tokens`，并且后台启用 `SEEDANCE_BILLING_BY_USAGE=true`，再按 usage 差额结算。
3. 如果 usage 缺失或异常，保留预扣金额，不做差额调整。

新增配置建议：

```text
SEEDANCE_BILLING_BY_USAGE=true
SEEDANCE_DEFAULT_RESOLUTION=1080p
SEEDANCE_DEFAULT_DURATION=5
SEEDANCE_DEFAULT_PRECONSUME_QUOTA=100000
SEEDANCE_BILLING_STRICT_USAGE=false
```

后台配置：

1. `model_price["doubao-seedance-2-0"] = 固定单次价格`
2. 或 `model_ratio["doubao-seedance-2-0"] = 官方 token 单价换算后的倍率`
3. `group_ratio["vip"]`、`group_ratio["enterprise"]` 用于给不同客户不同售价

### 5.4 预扣与结算链路

现有链路：

1. 请求进入 `controller.RelayTask`
2. 计算预扣额度
3. 创建 `BillingSession`
4. 任务提交成功后写入 `model.Task`
5. 轮询成功后 `settleTaskBillingOnComplete`
6. 失败或超时后退款

需要验证或增强：

1. Seedance 2.0 提交成功但任务最终失败时必须退款。
2. 上游返回 `succeeded` 且 `video_url` 有效时才做成功结算。
3. 上游返回内容审核失败、参数非法、额度不足时，需要记录错误日志并退款。
4. 任务超时后应进入失败状态，并根据预扣上下文退款。

### 5.5 账单查询与后台看账

现有字段已经满足 MVP：

1. `logs.user_id`
2. `logs.username`
3. `logs.model_name`
4. `logs.quota`
5. `logs.channel_id`
6. `logs.token_id`
7. `logs.group`
8. `logs.created_at`
9. `logs.type`

MVP 可先用现有日志页面查询：

1. 按用户筛选。
2. 按模型筛选 `doubao-seedance-2-0`。
3. 按时间范围筛选。
4. 按渠道筛选火山方舟渠道。

建议新增账单导出接口：

```text
GET /api/billing/export?user_id=&model_name=&start=&end=
```

同时新增账单汇总接口：

```text
GET /api/billing/summary?user_id=&model_name=&start_timestamp=&end_timestamp=
```

返回按用户、模型、渠道、分组聚合后的 `consume_quota`、`refund_quota`、`net_quota`、`request_count`、`refund_count`、`total_tokens`，用于后台快速对账。

管理员控制台已新增页面：

```text
/console/billing
```

该页面默认按 `doubao-seedance-2-0%` 筛选 Seedance 2.0 系列模型，支持按时间、用户、模型、渠道、分组查询汇总，并可导出消费/退款/充值流水 CSV。

生产监控可使用 Prometheus 文本格式接口：

```text
GET /api/billing/metrics?model_name=doubao-seedance-2-0%
```

该接口复用账单告警筛选条件和管理员鉴权，Prometheus 抓取时需要携带管理员 access token 与 `New-Api-User` 头。指标覆盖净扣费、消费/退款数量、退款率、异步任务成功/失败数、失败率、待处理任务、超时任务、worker 滞后、余额不足和上游错误信号。

返回 CSV 字段：

```text
created_at,user_id,username,model_name,channel_id,token_id,log_type,quota,group,request_id,task_id,content,billing_source,subscription_id,pre_consumed_quota,actual_quota
```

其中 `content` 用于保留充值、补单、兑换码等历史文本说明；`billing_source` 用于区分钱包扣费和订阅额度扣费；`pre_consumed_quota` / `actual_quota` 用于核对任务提交预扣与任务完成后的差额结算。

### 5.6 充值与订阅配置

现有充值能力可复用：

1. 用户充值：`/api/user/topup`
2. 用户充值记录：`/api/user/topup/self`
3. 管理员充值记录：`/api/user/topup`
4. 管理员补单：`/api/user/topup/complete`

现有订阅能力可复用：

1. 用户购买订阅计划。
2. 管理员创建订阅计划。
3. 订阅扣费优先级：`subscription_only`、`wallet_only`、`wallet_first`、`subscription_first`

推荐客户配置：

1. 个人用户：`wallet_first`
2. 企业客户：`subscription_first`
3. 大客户包月：订阅计划 + 超额钱包扣费

## 6. 数据库改造建议

MVP 可以不新增表，先复用 `logs`、`top_ups`、`subscription_plans`、`user_subscriptions`、`tasks`。

为了产品化账单，建议后续新增月账单汇总表：

```sql
CREATE TABLE billing_statements (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL,
  period_start BIGINT NOT NULL,
  period_end BIGINT NOT NULL,
  model_name VARCHAR(128) NOT NULL DEFAULT '',
  channel_id BIGINT NOT NULL DEFAULT 0,
  consume_quota BIGINT NOT NULL DEFAULT 0,
  refund_quota BIGINT NOT NULL DEFAULT 0,
  net_quota BIGINT NOT NULL DEFAULT 0,
  request_count BIGINT NOT NULL DEFAULT 0,
  task_count BIGINT NOT NULL DEFAULT 0,
  status VARCHAR(32) NOT NULL DEFAULT 'open',
  created_at BIGINT NOT NULL,
  updated_at BIGINT NOT NULL
);
```

索引：

```sql
CREATE INDEX idx_billing_statements_user_period ON billing_statements(user_id, period_start, period_end);
CREATE INDEX idx_billing_statements_model_period ON billing_statements(model_name, period_start, period_end);
```

## 7. 前端改造建议

MVP 阶段可以复用现有页面：

1. 渠道管理：新增 DoubaoVideo / VolcEngine 渠道。
2. 模型管理：新增 Seedance 2.0 模型元信息。
3. 价格/倍率设置：配置 Seedance 2.0 的固定价格或倍率。
4. 日志页面：筛选消费日志。
5. 充值页面：用户充值。
6. 订阅页面：用户购买订阅。

已新增：

1. 管理员账单页：`/console/billing`
2. 账单汇总 API：`GET /api/billing/summary`
3. 账单导出 API：`GET /api/billing/export`

后续建议新增：

1. 客户可见账单页，按月展示消费、充值、退款和订阅额度。
2. Seedance 任务列表增强：展示视频任务状态、消耗、退款、结果 URL。
3. 按钱包/订阅资金来源维度的汇总筛选。

## 8. API 调用示例

用户提交任务：

```bash
curl -sS https://your-domain/v1/video/generations \
  -H 'Authorization: Bearer USER_TOKEN' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "doubao-seedance-2-0",
    "prompt": "一只橘猫在江南古镇灯笼下奔跑，电影感镜头",
    "duration": 5,
    "resolution": "1080p",
    "ratio": "16:9",
    "generate_audio": true,
    "watermark": false
}'
```

图生视频 smoke test：

```bash
API_KEY=sk-xxx \
BASE_URL=https://your-domain \
IMAGE_URL=https://cdn.example.com/reference.png \
./scripts/seedance-billing-smoke.sh
```

首尾帧和参考视频 smoke test：

```bash
API_KEY=sk-xxx \
BASE_URL=https://your-domain \
FIRST_FRAME_URL=https://cdn.example.com/first.png \
LAST_FRAME_URL=https://cdn.example.com/last.png \
REFERENCE_VIDEO_URL=https://cdn.example.com/reference.mp4 \
./scripts/seedance-billing-smoke.sh
```

非法素材 URL 拦截 smoke test：

```bash
API_KEY=sk-xxx \
BASE_URL=https://your-domain \
IMAGE_URL=http://127.0.0.1/admin \
EXPECT_SUBMIT_FAILURE=true \
./scripts/seedance-billing-smoke.sh
```

用户查询任务：

```bash
curl -sS https://your-domain/v1/video/generations/TASK_ID \
  -H 'Authorization: Bearer USER_TOKEN'
```

管理员查消费日志：

```bash
curl -sS 'https://your-domain/api/log/?model_name=doubao-seedance-2-0' \
  -H 'Cookie: SESSION=ADMIN_SESSION'
```

## 9. 验收清单

完整上线验收步骤见：

```text
docs/SEEDANCE_2_BILLING_ACCEPTANCE.md
```

剩余任务清单见：

```text
docs/SEEDANCE_2_BILLING_REMAINING_TASKS.md
```

### 9.1 充值钱包

1. 用户充值成功后余额增加。
2. 用户提交 Seedance 2.0 任务后余额预扣。
3. 任务成功后余额按实际消耗结算。
4. 任务失败后余额退款。
5. 管理员可以看到充值记录和消费日志。

### 9.2 订阅额度

1. 用户购买订阅后产生可用订阅额度。
2. 用户设置 `subscription_first` 后优先扣订阅。
3. 订阅额度不足时可回退钱包扣费（如配置 `subscription_first`）。
4. 任务失败后订阅预扣额度退款。

### 9.3 Seedance 2.0 任务

1. 文生视频可提交成功。
2. 图生视频可提交成功。
3. 任务轮询能拿到 `video_url`。
4. 任务状态可从 queued/running/succeeded/failed 正确映射。
5. 上游 usage 正常时记录 token 消耗。
6. 上游 usage 缺失时不多扣费。

### 9.4 后台账单

1. 可按用户查看消费。
2. 可按模型查看 Seedance 2.0 消费。
3. 可按渠道查看火山方舟消费。
4. 可按日期导出或汇总账单。

## 10. 风险与控制

1. 官方模型名变化：使用后台模型映射兜底，不依赖代码固定名称。
2. usage 字段变化：结算逻辑保留固定价格兜底。
3. 视频任务成本高：必须启用预扣费，禁止 0 价格上线。
4. 异步任务延迟长：需要配置任务超时和失败退款。
5. 客户余额不足：提交前拦截，避免上游成本先发生。
6. 订阅和钱包并存：统一使用 `BillingSession`，避免重复扣费。

## 11. 推荐实施阶段

阶段 1：模型与渠道接入

1. 新增 Seedance 2.0 模型名。
2. 配置 DoubaoVideo / VolcEngine 渠道。
3. 完成一次真实提交与查询。

阶段 2：计价和扣费验证

1. 配置固定价格或倍率。
2. 验证钱包预扣、成功结算、失败退款。
3. 验证订阅优先与钱包回退。

阶段 3：后台账单

1. 复用日志页面筛选。
2. 新增账单导出接口。
3. 增加账单汇总任务。

阶段 4：生产加固

1. 增加 Seedance 2.0 参数白名单。
2. 增加异步任务超时告警。
3. 增加额度不足告警。
4. 增加官方 usage 变更监控。

## 12. 推荐 PR 拆分

1. `feat(seedance): add seedance 2.0 model registration`
2. `feat(seedance): support seedance 2.0 request metadata`
3. `feat(billing): add seedance task fixed price fallback`
4. `feat(billing): add billing export api`
5. `test(seedance): cover wallet and subscription task billing`
