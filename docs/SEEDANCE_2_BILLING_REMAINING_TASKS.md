# Seedance 2.0 充值扣费剩余任务清单

## 1. 当前状态

本分支已经完成客户最小闭环所需的主要代码改造：

1. 内置 Seedance 2.0 系列模型名与模型识别逻辑。
2. 增加 Seedance 2.0 任务参数解析、白名单校验和固定价格兜底。
3. 打通钱包余额、订阅额度、Token 额度的任务预扣、补扣、退款链路。
4. 增加账单汇总 API、账单 CSV 导出和管理员账单页面。
5. 增加任务计费审计字段，包括资金来源、订阅 ID、预扣额度和实际额度。
6. 增加 Seedance 2.0 图片、视频和 callback URL 的基础 SSRF 校验。
7. 增加 Seedance 2.0 验收手册和真实接口 smoke test 脚本。

当前代码已经可以进入预发环境做联调验收，但还不建议在未完成 P0 事项前直接对客户生产放量。

## 2. P0 上线前必须完成

| 任务 | 状态 | 说明 | 验收标准 |
| --- | --- | --- | --- |
| 火山方舟真实接口联调 | 未完成，等待客户 API Key 和渠道信息 | 目前代码支持提交与轮询链路，但还没有用客户真实 Seedance 2.0 账号完成端到端调用。 | 通过 `scripts/seedance-billing-smoke.sh` 提交任务，拿到真实 `task_id`、最终状态和结果 URL。 |
| 确认官方模型 ID | 未完成，等待控制台确认 | 当前已内置常用 Seedance 2.0 命名，并支持模型映射兜底。最终生产模型名必须以火山方舟控制台开通后的真实 ID 为准。 | 后台渠道模型映射中可用真实模型 ID 成功提交任务。 |
| 确认官方返回结构和 usage 字段 | 未完成，等待真实任务返回 | 代码已支持固定价格兜底和可选 usage 差额结算，但是否稳定返回 `usage.total_tokens` 需要实测。 | 至少完成成功、失败、超时三类任务，确认 usage 缺失时不会多扣费。 |
| 配置真实价格 | 未完成，需业务定价 | 当前代码提供默认固定价格和倍率入口，但生产价格需要按客户成本、毛利和计费口径配置。 | 模型价格不为 0，成功任务净扣费等于业务定价，失败任务净扣费为 0。 |
| 验证充值入口 | 未完成，需客户支付方式 | 代码复用现有充值订单和管理员补单能力，但客户实际支付渠道需要单独验收。 | 用户充值到账后余额增加，充值流水导出包含 `content` 并可被财务核对。 |
| 验证任务轮询 worker | 未完成，需部署环境 | Seedance 是异步任务，生产必须启用任务轮询，否则失败退款和最终结算不会及时发生。 | `UPDATE_TASK=true` 的 worker 正常运行，任务最终状态会更新并触发结算或退款。 |
| 完整验收手册执行 | 未完成 | 需要按 `docs/SEEDANCE_2_BILLING_ACCEPTANCE.md` 跑钱包、订阅、余额不足、非法参数、usage 缺失场景。 | 验收证据齐全，包括请求、任务结果、余额前后、账单页截图和 CSV。 |

## 3. P1 建议上线前完成

| 任务 | 状态 | 说明 | 建议处理 |
| --- | --- | --- | --- |
| 远程图片和视频 URI 安全策略 | 基础完成，生产配置待完成 | 代码已接入系统 Fetch/SSRF 策略，会拒绝私有 IP、危险端口、非 HTTP(S) 协议和带账号密码的 URL。生产如果允许客户传外部 URL，仍建议限制到客户素材域名或对象存储域名。 | 配置 URL allowlist、禁止内网 IP、使用对象存储中转或安全代理拉取，并在预发验证非法 URL 会被 400 拦截。 |
| 补齐官方多模态字段 | 基础完成，官方细节待联调 | 当前支持 prompt、image/images、reference_image_url(s)、first_frame_url、last_frame_url、reference_video_url(s)、duration、resolution、ratio、seed、audio、watermark 等字段。虚拟人像 URI、音频参考和官方最终 `content.role` 取值仍需按客户开通能力联调。 | 以真实请求样例为准校正字段映射、校验和测试。 |
| 前端生产构建排查 | 已完成，仍有体积优化空间 | 已给 Vite 构建增加 Node heap 上限，并将 code-inspector 限制在 dev server；`npm run build` 已通过。构建仍提示部分 chunk 偏大。 | 后续可对 Mermaid、图表库、Semi UI 重页面继续做动态 import。 |
| 全量前端 lint 收敛 | 已完成 | 已增加 `.prettierignore` 排除 `dist` 等生成物，并格式化历史源码文件；`npm run lint` 已通过。 | CI 可直接执行 `cd web && npm run lint`。 |
| 生产监控与告警 | 未完成 | 需要监控任务失败率、超时率、退款量、余额不足、上游 4xx/5xx、worker 延迟。 | 增加 Grafana/Prometheus 告警或复用现有监控模块。 |
| 严格 usage 策略决策 | 未完成 | `SEEDANCE_BILLING_STRICT_USAGE=true` 会在 usage 缺失时失败退款，不适合未验证的生产环境。 | 预发观察 usage 稳定后再决定是否开启严格模式。 |
| 订阅套餐运营配置 | 未完成 | 订阅扣费链路已具备，但需要创建真实套餐、周期、额度和用户计费偏好。 | 在后台创建套餐，验证 `subscription_first`、`subscription_only`、钱包回退策略。 |

## 4. P2 后续产品化演进

| 任务 | 状态 | 说明 | 建议处理 |
| --- | --- | --- | --- |
| 月度账单报表表 | 未完成 | 当前可以查实时汇总和导出流水，但没有固化月结账单快照。 | 新增 `billing_statements` 表和定时汇总任务，支持财务月结。 |
| 任务列表体验增强 | 未完成 | 管理员账单页能看消费，但任务列表还可以增强展示视频任务状态、结果 URL、消耗和退款。 | 在任务日志/视频任务页面增加详情抽屉。 |
| 资金来源维度汇总 | 部分完成 | CSV 已导出 `billing_source`，但汇总 API 仍主要按用户、模型、渠道、分组聚合。 | 增加按钱包/订阅维度聚合和筛选。 |
| 更细粒度租户分账 | 未纳入当前 MVP | 当前“分账”定义为充值、订阅、扣费、退款、后台看账，不包含代理分润、提现、税务和多方清分。 | 如客户后续需要代理/渠道分润，另立清分账本和结算流程。 |
| AGPL 商用交付包 | 流程完成，实际发布待执行 | 已新增 Seedance 专用交付清单和交付打包脚本；如果直接把修改版 SaaS 提供给客户或网络用户使用，需要按 AGPL 提供源码、变更记录、版权声明和构建说明；闭源交付需要商业授权。 | 发布前执行 `scripts/package-seedance-delivery.sh`，按需生成 SBOM，或获取商业授权并保存凭证。 |
| Pull Request 和合并策略 | 未完成 | 当前改造分支未合并 main，按用户要求不影响 main。 | 验收通过后再开 PR，选择 merge/rebase/fast-forward 策略。 |

## 5. 建议下一步执行顺序

1. 在预发环境配置客户真实火山方舟 Seedance 2.0 渠道、模型映射和 API Key。
2. 配置真实固定价格，先不开启严格 usage 校验。
3. 用钱包模式执行 smoke test，保存账单页和 CSV 证据。
4. 用订阅模式执行 smoke test，验证订阅额度扣费与退款。
5. 执行余额不足、非法参数、任务失败或超时场景。
6. 根据真实返回校正多模态字段映射，并配置生产素材域名 allowlist 或对象存储中转。
7. 执行 `scripts/package-seedance-delivery.sh`，生成交付包；如闭源交付，保存商业授权凭证。

## 6. 相关文档

1. 改造方案：`docs/SEEDANCE_2_BILLING_PLAN.md`
2. 验收手册：`docs/SEEDANCE_2_BILLING_ACCEPTANCE.md`
3. 真实接口 smoke test：`scripts/seedance-billing-smoke.sh`
4. 交付清单：`compliance/SEEDANCE_2_BILLING_DELIVERY.md`
5. 交付打包脚本：`scripts/package-seedance-delivery.sh`
