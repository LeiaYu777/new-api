package model

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	BillingAlertSeverityWarning  = "warning"
	BillingAlertSeverityCritical = "critical"
)

type BillingAlertFilter struct {
	StartTimestamp     int64
	EndTimestamp       int64
	ModelName          string
	Username           string
	UserId             int
	Channel            int
	Group              string
	BillingSource      string
	TaskTimeoutSeconds int64
}

type BillingAlertThresholds struct {
	RefundRatePercent  int   `json:"refund_rate_percent"`
	FailureRatePercent int   `json:"failure_rate_percent"`
	RefundCount        int64 `json:"refund_count"`
	PendingTaskCount   int64 `json:"pending_task_count"`
	TaskTimeoutSeconds int64 `json:"task_timeout_seconds"`
	WorkerLagSeconds   int64 `json:"worker_lag_seconds"`
}

type BillingAlertItem struct {
	Key            string  `json:"key"`
	Severity       string  `json:"severity"`
	Message        string  `json:"message"`
	Recommendation string  `json:"recommendation"`
	Value          float64 `json:"value"`
	Threshold      float64 `json:"threshold"`
}

type BillingAlertMetrics struct {
	WindowStart              int64                  `json:"window_start"`
	WindowEnd                int64                  `json:"window_end"`
	ConsumeQuota             int64                  `json:"consume_quota"`
	RefundQuota              int64                  `json:"refund_quota"`
	NetQuota                 int64                  `json:"net_quota"`
	RequestCount             int64                  `json:"request_count"`
	RefundCount              int64                  `json:"refund_count"`
	RefundRate               float64                `json:"refund_rate"`
	TaskSuccessCount         int64                  `json:"task_success_count"`
	TaskFailureCount         int64                  `json:"task_failure_count"`
	TaskFailureRate          float64                `json:"task_failure_rate"`
	PendingTaskCount         int64                  `json:"pending_task_count"`
	TimedOutTaskCount        int64                  `json:"timed_out_task_count"`
	WorkerLagSeconds         int64                  `json:"worker_lag_seconds"`
	InsufficientBalanceCount int64                  `json:"insufficient_balance_count"`
	UpstreamErrorCount       int64                  `json:"upstream_error_count"`
	Thresholds               BillingAlertThresholds `json:"thresholds"`
	Alerts                   []BillingAlertItem     `json:"alerts"`
}

func billingAlertThresholds(timeoutSeconds int64) BillingAlertThresholds {
	if timeoutSeconds <= 0 {
		timeoutSeconds = int64(common.GetEnvOrDefault("BILLING_ALERT_TASK_TIMEOUT_SECONDS", 3600))
	}
	return BillingAlertThresholds{
		RefundRatePercent:  common.GetEnvOrDefault("BILLING_ALERT_REFUND_RATE_PERCENT", 20),
		FailureRatePercent: common.GetEnvOrDefault("BILLING_ALERT_FAILURE_RATE_PERCENT", 30),
		RefundCount:        int64(common.GetEnvOrDefault("BILLING_ALERT_REFUND_COUNT", 5)),
		PendingTaskCount:   int64(common.GetEnvOrDefault("BILLING_ALERT_PENDING_TASK_COUNT", 20)),
		TaskTimeoutSeconds: timeoutSeconds,
		WorkerLagSeconds:   int64(common.GetEnvOrDefault("BILLING_ALERT_WORKER_LAG_SECONDS", 900)),
	}
}

func taskPropertiesTextExpr() string {
	if common.UsingPostgreSQL {
		return "tasks.properties::text"
	}
	if common.UsingMySQL {
		return "CAST(tasks.properties AS CHAR)"
	}
	return "CAST(tasks.properties AS TEXT)"
}

func taskGroupSQLExpr() string {
	if common.UsingPostgreSQL {
		return `tasks."group"`
	}
	return "tasks.`group`"
}

func taskModelLikePattern(modelName string) (string, error) {
	pattern, err := sanitizeLikePattern(modelName)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(pattern, "%") {
		pattern = "%" + pattern
	}
	if !strings.HasSuffix(pattern, "%") {
		pattern += "%"
	}
	return pattern, nil
}

func applyBillingTaskFilters(tx *gorm.DB, filter BillingAlertFilter, includeWindow bool) (*gorm.DB, error) {
	if includeWindow {
		if filter.StartTimestamp > 0 {
			tx = tx.Where("(tasks.submit_time >= ? OR tasks.finish_time >= ?)", filter.StartTimestamp, filter.StartTimestamp)
		}
		if filter.EndTimestamp > 0 {
			tx = tx.Where("(tasks.submit_time <= ? OR tasks.finish_time <= ? OR tasks.finish_time = 0)", filter.EndTimestamp, filter.EndTimestamp)
		}
	}
	if filter.ModelName != "" {
		modelPattern, err := taskModelLikePattern(filter.ModelName)
		if err != nil {
			return nil, err
		}
		tx = tx.Where(taskPropertiesTextExpr()+" LIKE ? ESCAPE '!'", modelPattern)
	}
	if filter.UserId > 0 {
		tx = tx.Where("tasks.user_id = ?", filter.UserId)
	}
	if filter.Channel != 0 {
		tx = tx.Where("tasks.channel_id = ?", filter.Channel)
	}
	if filter.Group != "" {
		tx = tx.Where(taskGroupSQLExpr()+" = ?", filter.Group)
	}
	return tx, nil
}

func countBillingTasks(filter BillingAlertFilter, includeWindow bool, query string, args ...interface{}) (int64, error) {
	tx := DB.Model(&Task{})
	var err error
	tx, err = applyBillingTaskFilters(tx, filter, includeWindow)
	if err != nil {
		return 0, err
	}
	if query != "" {
		tx = tx.Where(query, args...)
	}
	var count int64
	err = tx.Count(&count).Error
	return count, err
}

func countBillingLogSignals(filter BillingAlertFilter, query string, args ...interface{}) (int64, error) {
	tx := LOG_DB.Model(&Log{})
	if filter.StartTimestamp > 0 {
		tx = tx.Where("logs.created_at >= ?", filter.StartTimestamp)
	}
	if filter.EndTimestamp > 0 {
		tx = tx.Where("logs.created_at <= ?", filter.EndTimestamp)
	}
	if filter.ModelName != "" {
		modelNamePattern, err := sanitizeLikePattern(filter.ModelName)
		if err != nil {
			return 0, err
		}
		tx = tx.Where("logs.model_name LIKE ? ESCAPE '!'", modelNamePattern)
	}
	if filter.Username != "" {
		tx = tx.Where("logs.username = ?", filter.Username)
	}
	if filter.UserId > 0 {
		tx = tx.Where("logs.user_id = ?", filter.UserId)
	}
	if filter.Channel != 0 {
		tx = tx.Where("logs.channel_id = ?", filter.Channel)
	}
	if filter.Group != "" {
		tx = tx.Where("logs."+logGroupCol+" = ?", filter.Group)
	}
	var err error
	tx, err = applyBillingSourceFilter(tx, filter.BillingSource)
	if err != nil {
		return 0, err
	}
	if query != "" {
		tx = tx.Where(query, args...)
	}
	var count int64
	err = tx.Count(&count).Error
	return count, err
}

func appendBillingAlert(alerts []BillingAlertItem, key string, severity string, message string, recommendation string, value float64, threshold float64) []BillingAlertItem {
	return append(alerts, BillingAlertItem{
		Key:            key,
		Severity:       severity,
		Message:        message,
		Recommendation: recommendation,
		Value:          value,
		Threshold:      threshold,
	})
}

func GetBillingAlertMetrics(filter BillingAlertFilter) (*BillingAlertMetrics, error) {
	thresholds := billingAlertThresholds(filter.TaskTimeoutSeconds)
	filter.TaskTimeoutSeconds = thresholds.TaskTimeoutSeconds

	summary, err := GetBillingSummary(
		filter.StartTimestamp,
		filter.EndTimestamp,
		filter.ModelName,
		filter.Username,
		filter.UserId,
		filter.Channel,
		filter.Group,
		"",
		filter.BillingSource,
		logSearchCountLimit,
	)
	if err != nil {
		return nil, err
	}

	successCount, err := countBillingTasks(filter, true, "tasks.status = ?", TaskStatusSuccess)
	if err != nil {
		return nil, err
	}
	failureCount, err := countBillingTasks(filter, true, "tasks.status = ?", TaskStatusFailure)
	if err != nil {
		return nil, err
	}
	pendingCount, err := countBillingTasks(
		filter,
		false,
		"tasks.status NOT IN ?",
		[]TaskStatus{TaskStatusSuccess, TaskStatusFailure},
	)
	if err != nil {
		return nil, err
	}

	now := common.GetTimestamp()
	timeoutCutoff := now - thresholds.TaskTimeoutSeconds
	timedOutCount, err := countBillingTasks(
		filter,
		false,
		"tasks.status NOT IN ? AND tasks.submit_time > 0 AND tasks.submit_time < ?",
		[]TaskStatus{TaskStatusSuccess, TaskStatusFailure},
		timeoutCutoff,
	)
	if err != nil {
		return nil, err
	}

	var oldestPending struct {
		UpdatedAt int64 `gorm:"column:updated_at"`
	}
	pendingTx := DB.Model(&Task{}).Select("MIN(tasks.updated_at) AS updated_at").
		Where("tasks.status NOT IN ?", []TaskStatus{TaskStatusSuccess, TaskStatusFailure})
	pendingTx, err = applyBillingTaskFilters(pendingTx, filter, false)
	if err != nil {
		return nil, err
	}
	if err = pendingTx.Scan(&oldestPending).Error; err != nil {
		return nil, err
	}

	insufficientBalanceCount, err := countBillingLogSignals(
		filter,
		"(logs.content LIKE ? OR logs.content LIKE ?)",
		"%余额不足%",
		"%额度不足%",
	)
	if err != nil {
		return nil, err
	}
	upstreamErrorCount, err := countBillingTasks(
		filter,
		true,
		"tasks.status = ? AND (tasks.fail_reason LIKE ? OR tasks.fail_reason LIKE ?)",
		TaskStatusFailure,
		"%upstream%",
		"%上游%",
	)
	if err != nil {
		return nil, err
	}

	metrics := &BillingAlertMetrics{
		WindowStart:              filter.StartTimestamp,
		WindowEnd:                filter.EndTimestamp,
		ConsumeQuota:             summary.ConsumeQuota,
		RefundQuota:              summary.RefundQuota,
		NetQuota:                 summary.NetQuota,
		RequestCount:             summary.RequestCount,
		RefundCount:              summary.RefundCount,
		TaskSuccessCount:         successCount,
		TaskFailureCount:         failureCount,
		PendingTaskCount:         pendingCount,
		TimedOutTaskCount:        timedOutCount,
		InsufficientBalanceCount: insufficientBalanceCount,
		UpstreamErrorCount:       upstreamErrorCount,
		Thresholds:               thresholds,
		Alerts:                   []BillingAlertItem{},
	}

	if metrics.RequestCount > 0 {
		metrics.RefundRate = float64(metrics.RefundCount) / float64(metrics.RequestCount)
	}
	finishedTasks := metrics.TaskSuccessCount + metrics.TaskFailureCount
	if finishedTasks > 0 {
		metrics.TaskFailureRate = float64(metrics.TaskFailureCount) / float64(finishedTasks)
	}
	if oldestPending.UpdatedAt > 0 && now > oldestPending.UpdatedAt {
		metrics.WorkerLagSeconds = now - oldestPending.UpdatedAt
	}

	if metrics.RefundRate*100 >= float64(thresholds.RefundRatePercent) && metrics.RequestCount > 0 {
		metrics.Alerts = appendBillingAlert(metrics.Alerts, "refund_rate", BillingAlertSeverityWarning,
			"退款率超过阈值",
			"检查上游任务失败、严格 usage 策略和价格配置，确认失败任务是否都已正确退款。",
			metrics.RefundRate*100,
			float64(thresholds.RefundRatePercent),
		)
	}
	if metrics.RefundCount >= thresholds.RefundCount {
		metrics.Alerts = appendBillingAlert(metrics.Alerts, "refund_count", BillingAlertSeverityWarning,
			"退款流水数量偏高",
			"按 request_id/task_id 抽样核对失败原因，重点查看 Seedance 任务状态和上游返回。",
			float64(metrics.RefundCount),
			float64(thresholds.RefundCount),
		)
	}
	if metrics.TaskFailureRate*100 >= float64(thresholds.FailureRatePercent) && finishedTasks > 0 {
		metrics.Alerts = appendBillingAlert(metrics.Alerts, "task_failure_rate", BillingAlertSeverityWarning,
			"异步任务失败率超过阈值",
			"检查渠道可用性、素材 URL 可访问性、模型参数和上游限流错误。",
			metrics.TaskFailureRate*100,
			float64(thresholds.FailureRatePercent),
		)
	}
	if metrics.TimedOutTaskCount > 0 {
		metrics.Alerts = appendBillingAlert(metrics.Alerts, "timed_out_tasks", BillingAlertSeverityCritical,
			"存在超时未完成任务",
			"确认 UPDATE_TASK=true 的 worker 正常运行，并检查任务轮询日志和渠道连通性。",
			float64(metrics.TimedOutTaskCount),
			0,
		)
	}
	if metrics.PendingTaskCount >= thresholds.PendingTaskCount {
		metrics.Alerts = appendBillingAlert(metrics.Alerts, "pending_tasks", BillingAlertSeverityWarning,
			"待处理任务积压",
			"增加 worker 副本或降低轮询间隔，排查上游排队和本地任务更新延迟。",
			float64(metrics.PendingTaskCount),
			float64(thresholds.PendingTaskCount),
		)
	}
	if metrics.WorkerLagSeconds >= thresholds.WorkerLagSeconds && metrics.PendingTaskCount > 0 {
		metrics.Alerts = appendBillingAlert(metrics.Alerts, "worker_lag", BillingAlertSeverityCritical,
			"worker 更新滞后",
			"检查 worker 进程、数据库连接池、任务轮询日志和上游 fetch 接口。",
			float64(metrics.WorkerLagSeconds),
			float64(thresholds.WorkerLagSeconds),
		)
	}
	if metrics.InsufficientBalanceCount > 0 {
		metrics.Alerts = appendBillingAlert(metrics.Alerts, "insufficient_balance", BillingAlertSeverityWarning,
			"出现余额或额度不足信号",
			"确认用户充值、订阅额度和扣费优先级配置是否符合客户预期。",
			float64(metrics.InsufficientBalanceCount),
			0,
		)
	}
	if metrics.UpstreamErrorCount > 0 {
		metrics.Alerts = appendBillingAlert(metrics.Alerts, "upstream_errors", BillingAlertSeverityWarning,
			"出现上游错误失败任务",
			"核对火山方舟渠道状态、API Key 权限、模型 ID 和素材域名 allowlist。",
			float64(metrics.UpstreamErrorCount),
			0,
		)
	}

	return metrics, nil
}
