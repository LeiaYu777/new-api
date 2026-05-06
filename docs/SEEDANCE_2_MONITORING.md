# Seedance 2.0 计费监控接入手册

## 1. 目标

本手册用于把 NewAPI Seedance 2.0 计费指标接入 Prometheus、Alertmanager 和 Grafana，覆盖客户最关心的生产风险：

1. 用户充值后能否正常扣费。
2. Seedance 任务失败或超时后是否退款。
3. 任务轮询 worker 是否滞后。
4. 上游错误、余额不足、退款率是否异常。
5. 后台账单汇总和监控指标是否能相互印证。

## 2. 前置条件

1. NewAPI 已部署并可访问 `/api/billing/metrics`。
2. 已创建管理员 access token。
3. 已确认管理员用户 ID，用于 `New-Api-User` 请求头。
4. Prometheus 版本支持 `authorization` 和 `http_headers` 配置。
5. 监控网络可以访问 NewAPI，但该指标端点不应暴露到公网。

Prometheus 的 `authorization` 和自定义 `http_headers` 用法可参考官方配置文档：

```text
https://prometheus.io/docs/prometheus/latest/configuration/configuration/
```

管理员 token 建议放到 Prometheus secret 文件：

```text
/etc/prometheus/secrets/newapi_admin_access_token
/etc/prometheus/secrets/newapi_admin_user_id
```

文件内容示例：

```text
# /etc/prometheus/secrets/newapi_admin_access_token
sk-admin-access-token

# /etc/prometheus/secrets/newapi_admin_user_id
1
```

## 3. 接口验证

先用 curl 验证指标端点：

```bash
curl -H "Authorization: Bearer ${ADMIN_ACCESS_TOKEN}" \
  -H "New-Api-User: ${ADMIN_USER_ID}" \
  "${BASE_URL}/api/billing/metrics?model_name=doubao-seedance-2-0%"
```

返回体应包含：

```text
newapi_billing_net_quota
newapi_billing_task_failure_rate
newapi_billing_worker_lag_seconds
```

也可以使用预检脚本：

```bash
CHECK_BILLING_METRICS=true \
BASE_URL=https://new-api.example.com \
ADMIN_ACCESS_TOKEN=sk-admin-access-token \
ADMIN_USER_ID=1 \
scripts/seedance-billing-preflight.sh
```

## 4. Prometheus 配置

样例文件：

```text
deploy/observability/seedance-billing-prometheus.yml
```

关键配置：

```yaml
scrape_configs:
  - job_name: newapi-seedance-billing
    scheme: https
    metrics_path: /api/billing/metrics
    params:
      model_name:
        - doubao-seedance-2-0%
    authorization:
      type: Bearer
      credentials_file: /etc/prometheus/secrets/newapi_admin_access_token
    http_headers:
      New-Api-User:
        files:
          - /etc/prometheus/secrets/newapi_admin_user_id
```

部署步骤：

1. 将 `deploy/observability/seedance-billing-prometheus.yml` 合并到 Prometheus 主配置。
2. 将 `deploy/observability/seedance-billing-alert-rules.yml` 放到 Prometheus rule 目录。
3. 把 `rule_files` 指向实际 rule 文件路径。
4. 重新加载 Prometheus。
5. 在 Prometheus Targets 页面确认 `newapi-seedance-billing` 为 `UP`。

## 5. 告警规则

样例文件：

```text
deploy/observability/seedance-billing-alert-rules.yml
```

内置告警：

1. `NewAPISeedanceRefundRateHigh`：退款率高于 20%，且窗口内消费记录超过 5 条。
2. `NewAPISeedanceTaskFailureRateHigh`：任务失败率高于 30%，且已完成任务超过 5 条。
3. `NewAPISeedanceTimedOutTasks`：存在超时未完成任务。
4. `NewAPISeedanceWorkerLagHigh`：worker 滞后超过 900 秒。
5. `NewAPISeedancePendingTasksHigh`：待处理任务超过 20 条。
6. `NewAPISeedanceUpstreamErrors`：出现上游错误信号。
7. `NewAPISeedanceInsufficientBalance`：出现余额或订阅额度不足信号。
8. `NewAPISeedanceNetQuotaNegative`：净扣费为负，提示可能存在重复退款或筛选口径问题。

阈值可以按业务量调整。刚上线时建议先用 warning，观察 1 到 2 个账期后再收紧。

## 6. Grafana 面板建议

建议创建一个 `Seedance Billing` Dashboard，至少包含以下面板：

| 面板 | PromQL 示例 | 说明 |
| --- | --- | --- |
| 净扣费 | `newapi_billing_net_quota{feature="seedance-billing"}` | 当前筛选窗口净扣费 |
| 退款率 | `newapi_billing_refund_rate{feature="seedance-billing"}` | 退款数 / 消费数 |
| 任务失败率 | `newapi_billing_task_failure_rate{feature="seedance-billing"}` | 失败任务 / 已完成任务 |
| Pending 任务 | `newapi_billing_pending_task_count{feature="seedance-billing"}` | 未进入最终状态的任务 |
| 超时任务 | `newapi_billing_timed_out_task_count{feature="seedance-billing"}` | 超过配置阈值未完成 |
| Worker 滞后 | `newapi_billing_worker_lag_seconds{feature="seedance-billing"}` | 最旧未完成任务距上次更新时间 |
| 上游错误 | `newapi_billing_upstream_error_count{feature="seedance-billing"}` | 上游失败信号 |
| 余额不足 | `newapi_billing_insufficient_balance_count{feature="seedance-billing"}` | 充值或订阅额度不足信号 |

## 7. 日常排障

### 7.1 Prometheus 抓取 401 或 403

处理步骤：

1. 确认 `authorization.credentials_file` 中只有 token 原文，不包含 `Bearer ` 前缀。
2. 确认 `http_headers.New-Api-User.files` 文件中是管理员数字用户 ID。
3. 确认该 access token 属于管理员账号。
4. 使用 curl 手动复现，并对比 Prometheus 配置。

### 7.2 指标存在但告警不触发

处理步骤：

1. 在 Prometheus Rules 页面确认规则已加载。
2. 在 Prometheus Graph 页面执行对应 PromQL。
3. 检查 `feature="seedance-billing"`、`job`、`instance` 标签是否与规则一致。
4. 检查 `for` 持续时间是否尚未满足。

### 7.3 Worker 滞后或超时任务告警

处理步骤：

1. 确认至少一个节点配置 `UPDATE_TASK=true`。
2. 检查任务轮询日志。
3. 检查数据库连接池和慢查询。
4. 检查火山方舟任务查询接口是否限流或异常。
5. 在 `/console/task` 里按任务状态和模型筛选问题任务。

### 7.4 退款率或失败率突然升高

处理步骤：

1. 在 `/console/billing` 按 `task_id` 抽样对账。
2. 查看 CSV 中 `billing_source`、`pre_consumed_quota`、`actual_quota`。
3. 检查素材 URL allowlist 是否误拦截。
4. 检查模型 ID、API Key 权限和上游余额。
5. 如果开启了 `SEEDANCE_BILLING_STRICT_USAGE=true`，确认上游稳定返回 `usage.total_tokens`。

## 8. 安全建议

1. 指标端点复用管理员鉴权，不要将 token 写入仓库。
2. Prometheus secret 文件权限建议为 `0400` 或等效权限。
3. 只允许监控网络访问 `/api/billing/metrics`。
4. 如使用反向代理，建议限制来源 IP 并启用 TLS。
5. 离职或交付完成后轮换管理员 access token。
