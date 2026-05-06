# Seedance 2.0 充值扣费验收手册

## 1. 验收目标

本手册用于验收客户要求的最小闭环：

1. 用户可以充值或购买订阅。
2. 用户可以调用字节/火山方舟 Seedance 2.0 视频生成模型。
3. 系统可以按任务消耗扣除钱包余额或订阅额度。
4. 任务失败、超时或严格 usage 校验失败时可以退款。
5. 管理员可以在后台查看汇总账单并导出流水。

## 2. 前置条件

### 2.1 服务配置

生产或预发环境必须完成以下配置：

```env
SEEDANCE_BILLING_BY_USAGE=true
SEEDANCE_BILLING_STRICT_USAGE=false
SEEDANCE_REQUIRE_PRICE_CONFIRMATION=true
SEEDANCE_PRICE_CONFIRMED=true
SEEDANCE_DEFAULT_DURATION=5
SEEDANCE_MAX_DURATION=60
SEEDANCE_MAX_IMAGES=8
SEEDANCE_MAX_REFERENCE_VIDEOS=3
SEEDANCE_REMOTE_URL_ALLOWLIST=cdn.example.com,*.oss-cn-hangzhou.aliyuncs.com
SEEDANCE_CALLBACK_URL_ALLOWLIST=hooks.example.com
```

建议先用固定价格上线，再根据上游 usage 稳定性决定是否按 token 差额结算。

配置完成后，先执行生产预检脚本：

```bash
CONFIG_FILE=.env.production \
MODEL=doubao-seedance-2-0 \
scripts/seedance-billing-preflight.sh
```

如果当前节点就是异步任务 worker，可追加：

```bash
REQUIRE_LOCAL_WORKER=true scripts/seedance-billing-preflight.sh
```

如需同时验证服务端 readiness API 和 Prometheus 指标端点，可追加管理员 access token 和用户 ID：

```bash
CHECK_BILLING_READINESS=true \
CHECK_BILLING_METRICS=true \
BASE_URL=https://your-domain \
ADMIN_ACCESS_TOKEN=sk-admin-access-token \
ADMIN_USER_ID=1 \
scripts/seedance-billing-preflight.sh
```

预检脚本不会提交真实任务，只检查 worker、usage 策略、素材域名 allowlist、月结、告警阈值，以及可选的服务端 readiness 和指标抓取等上线前配置。

### 2.2 渠道配置

管理员进入控制台：

```text
/console/channel
```

配置项：

1. 渠道类型选择 Doubao Video 或火山方舟对应视频渠道。
2. Base URL 使用火山方舟控制台提供的正式地址。
3. API Key 使用客户火山方舟账号的有效密钥。
4. 模型列表包含 `doubao-seedance-2-0` 或通过模型映射指向真实上游模型 ID。
5. 渠道启用并通过一次渠道连通性检查。

### 2.3 价格配置

管理员进入模型/倍率设置，至少配置一种价格策略：

1. 固定价格：`doubao-seedance-2-0` 设置为每次任务固定扣费。
2. 倍率价格：`doubao-seedance-2-0` 配置模型倍率，等待上游返回 usage 后差额结算。

上线建议：

1. 预发环境可以启用 `SEEDANCE_BILLING_BY_USAGE=true` 观察 usage。
2. 生产首日建议保留固定价格兜底。
3. 如果上游 usage 经常缺失，不建议打开 `SEEDANCE_BILLING_STRICT_USAGE=true`，否则成功视频任务可能因为 usage 缺失被判失败并退款。
4. 生产建议开启 `SEEDANCE_REQUIRE_PRICE_CONFIRMATION=true`，并在客户价格配置和业务审批完成后设置 `SEEDANCE_PRICE_CONFIRMED=true`；否则 Seedance 2.0 任务会被本地拒绝，避免未定价成本外流。

### 2.4 远程素材 URL 安全配置

如客户要通过 `image` 或 `images` 传外部素材 URL，生产环境建议完成以下配置：

1. 系统 Fetch/SSRF 防护保持开启。
2. 不允许私有 IP、内网域名、链路本地地址或非 HTTP(S) URL。
3. 只允许客户对象存储、CDN 或素材域名作为 allowlist。
4. 不在素材 URL 中携带账号密码、长期签名或敏感 Token。

Seedance 2.0 提交前会校验图片、视频和 `callback_url`，非法 URL 应在本地返回 400，不应产生上游任务和扣费。

## 3. 钱包扣费验收

### 3.1 准备用户和令牌

1. 创建普通用户。
2. 给用户充值。
3. 创建 API 令牌。
4. 用户计费偏好设置为 `wallet_first` 或 `wallet_only`。

记录验收前数据：

1. 用户余额。
2. Token 剩余额度。
3. `/console/billing` 当前净扣费。

### 3.2 提交 Seedance 2.0 任务

使用脚本：

```bash
API_KEY=sk-xxx \
BASE_URL=https://your-domain \
MODEL=doubao-seedance-2-0 \
DURATION=5 \
RESOLUTION=720p \
RATIO=16:9 \
GENERATE_AUDIO=false \
./scripts/seedance-billing-smoke.sh
```

如需验收图生视频，可追加：

```bash
IMAGE_URL=https://cdn.example.com/reference.png ./scripts/seedance-billing-smoke.sh
```

如需验收首尾帧或参考视频，可追加：

```bash
FIRST_FRAME_URL=https://cdn.example.com/first.png \
LAST_FRAME_URL=https://cdn.example.com/last.png \
REFERENCE_VIDEO_URL=https://cdn.example.com/reference.mp4 \
./scripts/seedance-billing-smoke.sh
```

预期结果：

1. 返回 `task_id`。
2. 脚本输出 `/console/my-billing?task_id=...&model_name=...` 自助账单深链，登录该用户后打开会自动按本次任务筛选。
3. 用户余额在任务提交后发生预扣。
4. Token 剩余额度同步预扣。
5. 任务完成后状态为 `completed` 或 `SUCCESS`。
6. 任务失败时状态为 `failed` 或 `FAILURE`，余额和 Token 额度退款。

### 3.3 后台看账

管理员进入：

```text
/console/billing
```

筛选条件：

1. 模型：`doubao-seedance-2-0%`
2. 用户名或用户 ID：本次测试用户
3. 时间：本次测试时间窗口
4. 如需核对单笔任务：填写本次返回的 `task_id`

预期结果：

1. 成功任务出现消费扣费。
2. 失败任务出现退款返还。
3. 净扣费 = 消费扣费 - 退款返还。
4. CSV 导出包含 `billing_source=wallet`。
5. CSV 导出包含 `task_id`、`pre_consumed_quota`、`actual_quota`。
6. 填写 `task_id` 后，汇总与 CSV 导出只包含该任务的消费、退款和净扣费。
7. 如导出类型选择“充值”，CSV 应包含 `content`，用于核对充值、补单或兑换码说明。

## 4. 订阅扣费验收

### 4.1 准备订阅

1. 管理员创建订阅套餐。
2. 用户购买或被分配订阅。
3. 用户计费偏好设置为 `subscription_first` 或 `subscription_only`。
4. 记录订阅总额度和已用额度。

### 4.2 提交任务

使用同一个脚本提交任务：

```bash
API_KEY=sk-xxx BASE_URL=https://your-domain ./scripts/seedance-billing-smoke.sh
```

预期结果：

1. 任务提交后订阅额度被预扣。
2. 成功任务按实际额度差额结算。
3. 失败任务退还订阅预扣。
4. Token 额度与订阅额度保持一致变化。

### 4.3 后台看账

CSV 导出预期：

1. `billing_source=subscription`
2. `subscription_id` 不为空
3. `pre_consumed_quota` 不为空
4. `actual_quota` 成功任务为最终应扣额度，失败任务为 `0`

## 5. 异常场景验收

### 5.1 余额不足

操作：

1. 将用户余额调低到低于 Seedance 2.0 预扣额度。
2. 使用钱包模式提交任务。

预期结果：

1. 请求被拒绝。
2. 不创建上游任务。
3. 不产生额外消费日志。
4. 后台可看到错误提示或错误日志。

### 5.2 参数非法

操作：

```bash
API_KEY=sk-xxx \
BASE_URL=https://your-domain \
RESOLUTION=16k \
./scripts/seedance-billing-smoke.sh
```

预期结果：

1. 请求返回 `invalid_resolution`。
2. 不产生上游成本。
3. 不产生消费扣费。

### 5.3 usage 缺失

预发环境可临时开启：

```env
SEEDANCE_BILLING_STRICT_USAGE=true
```

预期结果：

1. 如果上游成功但没有返回 usage，系统将任务标记为失败。
2. 预扣额度退款。
3. `/console/billing` 出现退款记录。

### 5.4 非法远程素材 URL

操作：

```bash
API_KEY=sk-xxx \
BASE_URL=https://your-domain \
IMAGE_URL=http://127.0.0.1/admin \
EXPECT_SUBMIT_FAILURE=true \
./scripts/seedance-billing-smoke.sh
```

预期结果：

1. 请求返回 `invalid_image_url`。
2. 不创建上游任务。
3. 不产生消费扣费。
4. 如果传入 `CALLBACK_URL=https://token:secret@example.com/callback`，应返回 `invalid_callback_url`。

## 6. 验收证据

每次验收建议保存以下证据：

1. 提交任务的请求体和返回 `task_id`。
2. 任务最终查询结果。
3. 用户余额或订阅额度验收前后截图。
4. `/console/billing` 汇总截图，建议同时保存按 `task_id` 精确筛选后的截图。
5. 普通用户独立页面 `/console/my-billing` 或钱包/充值页“我的 Seedance 账单”截图，证明客户自己可以核对消费、退款和资金来源。
6. CSV 导出文件，建议同时保存管理员视角和用户自助视角按 `task_id` 精确筛选后的 CSV。
7. 服务端日志中与 `task_id` 对应的计费记录。
8. `/api/billing/readiness` 返回结果，状态不应为 `blocked`。
9. Prometheus 指标抓取结果，至少包含 `newapi_billing_net_quota`、`newapi_billing_task_failure_rate`、`newapi_billing_worker_lag_seconds`。

可使用证据归档脚本统一保存验收材料：

```bash
BASE_URL=https://your-domain \
ADMIN_ACCESS_TOKEN=sk-admin-access-token \
ADMIN_USER_ID=1 \
SELF_ACCESS_TOKEN=sk-user-access-token \
SELF_USER_ID=1001 \
API_KEY=sk-user-token \
TASK_ID=task_xxx \
scripts/seedance-billing-collect-evidence.sh
```

脚本会在 `compliance/evidence/seedance-<timestamp>/` 下保存服务端 readiness、管理员账单汇总、告警、月结快照、CSV 流水、Prometheus 指标、普通用户自助账单接口证据和任务查询结果。脚本不会把管理员 token、用户 access token 或用户 API key 写入证据目录。

如果提供 `SELF_ACCESS_TOKEN` 和 `SELF_USER_ID`，脚本会自动采集：

```text
billing-self-summary.json
billing-self-statements.json
billing-self-ledger.csv
billing-self-topups.csv
```

如需强制要求用户自助账单证据，可设置：

```bash
COLLECT_SELF_BILLING=true scripts/seedance-billing-collect-evidence.sh
```

归档完成后执行离线校验：

```bash
scripts/seedance-billing-verify-evidence.sh compliance/evidence/seedance-20260506T120000Z
```

校验脚本会检查必需文件、HTTP 状态、JSON 格式、readiness 状态、CSV 对账字段、Prometheus 指标名、用户自助账单是否只包含 `self_user_id` 的数据，以及 manifest 中 `task_id` 与 CSV 流水是否匹配。

指标抓取示例：

```bash
curl -H "Authorization: $ADMIN_ACCESS_TOKEN" \
  -H "New-Api-User: $ADMIN_USER_ID" \
  "$BASE_URL/api/billing/metrics?model_name=doubao-seedance-2-0%"
```

Prometheus 和 Grafana 接入可参考：

```text
docs/SEEDANCE_2_MONITORING.md
deploy/observability/seedance-billing-prometheus.yml
deploy/observability/seedance-billing-alert-rules.yml
deploy/observability/seedance-billing-grafana-dashboard.json
```

## 7. 通过标准

该功能可以交付给客户的最低通过标准：

1. 钱包扣费成功任务：预扣、完成、账单汇总、CSV 导出全部正确。
2. 钱包扣费失败任务：预扣后退款，无净扣费异常。
3. 订阅扣费成功任务：订阅已用额度增加，账单标记 `subscription`。
4. 订阅扣费失败任务：订阅预扣退还。
5. 非法参数不会发起上游调用。
6. 管理员可通过 `/console/billing` 查到用户、模型、渠道、任务 ID、净扣费。
7. 普通用户可通过 `/console/my-billing`、钱包/充值页“我的 Seedance 账单”或 `/api/billing/self/*` 查到自己的消费、退款、资金来源和月结快照，且不能查看其他用户账单。

## 8. 不通过处理

1. 查不到任务：确认渠道、模型映射和 API Key。
2. 没有扣费：确认模型价格或倍率不为 0。
3. 没有退款：确认任务轮询 worker 正常运行。
4. CSV 缺少资金来源：确认任务日志 `other` 字段包含 `billing_source`。
5. 任务长期处理中：检查上游任务状态、轮询间隔和任务超时配置。
6. 图片或 callback URL 被拒绝：检查 Fetch/SSRF 设置、端口白名单和素材域名 allowlist。
