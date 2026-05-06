package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

const (
	SeedanceReadinessStatusReady   = "ready"
	SeedanceReadinessStatusWarning = "warning"
	SeedanceReadinessStatusBlocked = "blocked"

	SeedanceReadinessCheckPass = "pass"
	SeedanceReadinessCheckWarn = "warn"
	SeedanceReadinessCheckFail = "fail"
	SeedanceReadinessCheckInfo = "info"
)

type SeedanceBillingReadinessOptions struct {
	ModelName                string
	UsageConfirmed           bool
	RequireLocalWorker       bool
	RequireCallbackAllowlist bool
}

type SeedanceBillingReadinessCheck struct {
	Key            string `json:"key"`
	Status         string `json:"status"`
	Blocking       bool   `json:"blocking"`
	Message        string `json:"message"`
	Recommendation string `json:"recommendation,omitempty"`
}

type SeedanceBillingReadiness struct {
	Feature         string                          `json:"feature"`
	ModelName       string                          `json:"model_name"`
	Status          string                          `json:"status"`
	Ready           bool                            `json:"ready"`
	ProductionReady bool                            `json:"production_ready"`
	GeneratedAt     int64                           `json:"generated_at"`
	Config          map[string]any                  `json:"config"`
	Checks          []SeedanceBillingReadinessCheck `json:"checks"`
}

func BuildSeedanceBillingReadiness(opts SeedanceBillingReadinessOptions) SeedanceBillingReadiness {
	modelName := strings.TrimSpace(opts.ModelName)
	if modelName == "" {
		modelName = "doubao-seedance-2-0"
	}

	config := map[string]any{
		"seedance_billing_by_usage":             common.GetEnvOrDefaultBool("SEEDANCE_BILLING_BY_USAGE", true),
		"seedance_billing_strict_usage":         common.GetEnvOrDefaultBool("SEEDANCE_BILLING_STRICT_USAGE", false),
		"seedance_require_price_confirmation":   common.GetEnvOrDefaultBool("SEEDANCE_REQUIRE_PRICE_CONFIRMATION", false),
		"seedance_price_confirmed":              common.GetEnvOrDefaultBool("SEEDANCE_PRICE_CONFIRMED", false),
		"seedance_default_duration":             common.GetEnvOrDefault("SEEDANCE_DEFAULT_DURATION", 5),
		"seedance_max_duration":                 common.GetEnvOrDefault("SEEDANCE_MAX_DURATION", 60),
		"seedance_max_images":                   common.GetEnvOrDefault("SEEDANCE_MAX_IMAGES", 8),
		"seedance_max_reference_videos":         common.GetEnvOrDefault("SEEDANCE_MAX_REFERENCE_VIDEOS", 3),
		"seedance_remote_url_allowlist_count":   csvItemCount(common.GetEnvOrDefaultString("SEEDANCE_REMOTE_URL_ALLOWLIST", "")),
		"seedance_callback_url_allowlist_count": csvItemCount(common.GetEnvOrDefaultString("SEEDANCE_CALLBACK_URL_ALLOWLIST", "")),
		"update_task":                           common.GetEnvOrDefaultBool("UPDATE_TASK", true),
		"node_type":                             common.GetEnvOrDefaultString("NODE_TYPE", "master"),
		"task_timeout_minutes":                  common.GetEnvOrDefault("TASK_TIMEOUT_MINUTES", 1440),
		"billing_statement_auto_enabled":        common.GetEnvOrDefaultBool("BILLING_STATEMENT_AUTO_ENABLED", false),
		"usage_confirmed":                       opts.UsageConfirmed,
		"require_local_worker":                  opts.RequireLocalWorker,
		"require_callback_allowlist":            opts.RequireCallbackAllowlist,
	}

	modelPrice, hasModelPrice := ratio_setting.GetModelPrice(modelName, false)
	modelRatio, hasModelRatio, _ := ratio_setting.GetModelRatio(modelName)
	config["model_price_configured"] = hasModelPrice
	if hasModelPrice {
		config["model_price"] = modelPrice
	}
	config["model_ratio_configured"] = hasModelRatio
	if hasModelRatio {
		config["model_ratio"] = modelRatio
	}

	checks := make([]SeedanceBillingReadinessCheck, 0, 12)
	checks = append(checks, seedancePricingCheck(modelName, hasModelPrice, modelPrice, hasModelRatio, modelRatio))
	checks = append(checks, seedancePriceConfirmationCheck())
	checks = append(checks, seedanceWorkerCheck(opts.RequireLocalWorker))
	checks = append(checks, seedanceUsageCheck(opts.UsageConfirmed))
	checks = append(checks, seedanceIntRangeCheck("task_timeout_minutes", "TASK_TIMEOUT_MINUTES", 1440, 1, 10080, "Seedance 异步任务需要超时退款保护，生产不建议关闭。"))
	checks = append(checks, seedanceIntRangeCheck("seedance_default_duration", "SEEDANCE_DEFAULT_DURATION", 5, 1, 600, "默认时长用于附加倍率估算，应与业务定价基准一致。"))
	checks = append(checks, seedanceIntRangeCheck("seedance_max_duration", "SEEDANCE_MAX_DURATION", 60, 1, 600, "最大时长过大会放大单次任务成本。"))
	checks = append(checks, seedanceIntRangeCheck("seedance_max_images", "SEEDANCE_MAX_IMAGES", 8, 1, 32, "限制参考图片数量可控制请求体体积和上游成本。"))
	checks = append(checks, seedanceIntRangeCheck("seedance_max_reference_videos", "SEEDANCE_MAX_REFERENCE_VIDEOS", 3, 0, 10, "限制参考视频数量可降低素材拉取和任务失败风险。"))
	checks = append(checks, seedanceAllowlistCheck("seedance_remote_url_allowlist", "SEEDANCE_REMOTE_URL_ALLOWLIST", true, "生产应限制图片/视频素材到客户 OSS/CDN 域名。"))
	checks = append(checks, seedanceAllowlistCheck("seedance_callback_url_allowlist", "SEEDANCE_CALLBACK_URL_ALLOWLIST", opts.RequireCallbackAllowlist || strings.TrimSpace(common.GetEnvOrDefaultString("CALLBACK_URL", "")) != "", "启用 callback_url 时应限制回调域名。"))
	checks = append(checks, seedanceStatementCheck())

	status, ready, productionReady := seedanceOverallReadiness(checks)
	return SeedanceBillingReadiness{
		Feature:         "seedance-billing",
		ModelName:       modelName,
		Status:          status,
		Ready:           ready,
		ProductionReady: productionReady,
		GeneratedAt:     time.Now().Unix(),
		Config:          config,
		Checks:          checks,
	}
}

func seedanceCheck(key string, status string, blocking bool, message string, recommendation string) SeedanceBillingReadinessCheck {
	return SeedanceBillingReadinessCheck{Key: key, Status: status, Blocking: blocking, Message: message, Recommendation: recommendation}
}

func seedancePricingCheck(modelName string, hasPrice bool, price float64, hasRatio bool, ratio float64) SeedanceBillingReadinessCheck {
	if hasPrice && price > 0 {
		return seedanceCheck("pricing", SeedanceReadinessCheckPass, false, fmt.Sprintf("%s has model_price %.6f.", modelName, price), "确认该价格已按客户成本、毛利和计费周期审批。")
	}
	if hasRatio && ratio > 0 {
		return seedanceCheck("pricing", SeedanceReadinessCheckWarn, false, fmt.Sprintf("%s has model_ratio %.6f but no positive model_price.", modelName, ratio), "Seedance 生产首日建议配置固定 model_price 作为预扣兜底，usage 稳定后再使用倍率差额结算。")
	}
	return seedanceCheck("pricing", SeedanceReadinessCheckFail, true, fmt.Sprintf("%s has no positive model_price or model_ratio.", modelName), "在后台配置 model_price 或 model_ratio 后再开放客户调用。")
}

func seedancePriceConfirmationCheck() SeedanceBillingReadinessCheck {
	required := common.GetEnvOrDefaultBool("SEEDANCE_REQUIRE_PRICE_CONFIRMATION", false)
	confirmed := common.GetEnvOrDefaultBool("SEEDANCE_PRICE_CONFIRMED", false)
	if required && confirmed {
		return seedanceCheck("price_confirmation", SeedanceReadinessCheckPass, false, "Seedance production price confirmation gate is enabled and confirmed.", "保留价格审批记录与截图，随交付证据归档。")
	}
	if required {
		return seedanceCheck("price_confirmation", SeedanceReadinessCheckFail, true, "Seedance price confirmation gate is enabled but SEEDANCE_PRICE_CONFIRMED is not true.", "完成客户价格配置和业务审批后设置 SEEDANCE_PRICE_CONFIRMED=true。")
	}
	return seedanceCheck("price_confirmation", SeedanceReadinessCheckWarn, false, "Seedance price confirmation gate is disabled.", "生产建议设置 SEEDANCE_REQUIRE_PRICE_CONFIRMATION=true，并在价格审批后设置 SEEDANCE_PRICE_CONFIRMED=true。")
}

func seedanceWorkerCheck(requireLocalWorker bool) SeedanceBillingReadinessCheck {
	updateTask := common.GetEnvOrDefaultBool("UPDATE_TASK", true)
	nodeType := strings.ToLower(strings.TrimSpace(common.GetEnvOrDefaultString("NODE_TYPE", "master")))
	if !updateTask {
		status := SeedanceReadinessCheckWarn
		blocking := false
		if requireLocalWorker {
			status = SeedanceReadinessCheckFail
			blocking = true
		}
		return seedanceCheck("task_worker", status, blocking, "UPDATE_TASK=false on this node.", "Seedance 是异步任务，必须至少有一个 master/worker 节点启用 UPDATE_TASK=true。")
	}
	if nodeType == "slave" {
		status := SeedanceReadinessCheckWarn
		blocking := false
		if requireLocalWorker {
			status = SeedanceReadinessCheckFail
			blocking = true
		}
		return seedanceCheck("task_worker", status, blocking, "NODE_TYPE=slave, this node may not poll async tasks.", "确认至少一个 master 节点负责任务轮询、失败退款和最终结算。")
	}
	return seedanceCheck("task_worker", SeedanceReadinessCheckPass, false, "Task polling is enabled or defaults to enabled on this node.", "预发仍需通过真实 task_id 验证最终状态更新和退款。")
}

func seedanceUsageCheck(usageConfirmed bool) SeedanceBillingReadinessCheck {
	byUsage := common.GetEnvOrDefaultBool("SEEDANCE_BILLING_BY_USAGE", true)
	strictUsage := common.GetEnvOrDefaultBool("SEEDANCE_BILLING_STRICT_USAGE", false)
	if strictUsage && usageConfirmed {
		return seedanceCheck("usage_strategy", SeedanceReadinessCheckPass, false, "Strict usage mode is enabled and usage has been marked confirmed.", "保留真实成功、失败、超时任务的 usage 验收证据。")
	}
	if strictUsage {
		return seedanceCheck("usage_strategy", SeedanceReadinessCheckWarn, false, "Strict usage mode is enabled but usage is not confirmed.", "若上游成功任务缺少 usage.total_tokens，任务会被判失败并退款；生产前必须实测。")
	}
	if !byUsage {
		return seedanceCheck("usage_strategy", SeedanceReadinessCheckInfo, false, "Usage delta settlement is disabled; fixed-price billing will be used.", "这是 Seedance 上线早期的保守策略，但仍需确认固定价格覆盖成本。")
	}
	return seedanceCheck("usage_strategy", SeedanceReadinessCheckPass, false, "Usage delta settlement is enabled and strict missing-usage failure is disabled.", "预发观察 usage 稳定性后再决定是否开启严格模式。")
}

func seedanceIntRangeCheck(key string, envName string, defaultValue int, minValue int, maxValue int, recommendation string) SeedanceBillingReadinessCheck {
	value := common.GetEnvOrDefault(envName, defaultValue)
	if value < minValue || value > maxValue {
		return seedanceCheck(key, SeedanceReadinessCheckFail, true, fmt.Sprintf("%s=%d is outside expected range %d-%d.", envName, value, minValue, maxValue), recommendation)
	}
	return seedanceCheck(key, SeedanceReadinessCheckPass, false, fmt.Sprintf("%s=%d is within expected range %d-%d.", envName, value, minValue, maxValue), recommendation)
}

func seedanceAllowlistCheck(key string, envName string, required bool, recommendation string) SeedanceBillingReadinessCheck {
	value := strings.TrimSpace(common.GetEnvOrDefaultString(envName, ""))
	if value == "" {
		status := SeedanceReadinessCheckInfo
		if required {
			status = SeedanceReadinessCheckWarn
		}
		return seedanceCheck(key, status, false, fmt.Sprintf("%s is empty.", envName), recommendation)
	}
	for _, item := range strings.Split(value, ",") {
		trimmed := strings.TrimSpace(item)
		if trimmed == "*" || trimmed == "*." {
			return seedanceCheck(key, SeedanceReadinessCheckFail, true, fmt.Sprintf("%s contains too-broad wildcard item %q.", envName, trimmed), recommendation)
		}
		if strings.Contains(trimmed, " ") {
			return seedanceCheck(key, SeedanceReadinessCheckWarn, false, fmt.Sprintf("%s contains whitespace inside item %q.", envName, trimmed), recommendation)
		}
	}
	return seedanceCheck(key, SeedanceReadinessCheckPass, false, fmt.Sprintf("%s has %d configured item(s).", envName, csvItemCount(value)), recommendation)
}

func seedanceStatementCheck() SeedanceBillingReadinessCheck {
	if common.GetEnvOrDefaultBool("BILLING_STATEMENT_AUTO_ENABLED", false) {
		return seedanceCheck("billing_statement_auto", SeedanceReadinessCheckPass, false, "Automatic billing statement generation is enabled.", "确认账期日、小时和时区口径与客户合同一致。")
	}
	return seedanceCheck("billing_statement_auto", SeedanceReadinessCheckInfo, false, "Automatic billing statement generation is disabled.", "MVP 可先手动生成月结快照，生产规模化后建议开启自动月结。")
}

func seedanceOverallReadiness(checks []SeedanceBillingReadinessCheck) (status string, ready bool, productionReady bool) {
	hasWarn := false
	for _, check := range checks {
		if check.Blocking || check.Status == SeedanceReadinessCheckFail {
			return SeedanceReadinessStatusBlocked, false, false
		}
		if check.Status == SeedanceReadinessCheckWarn {
			hasWarn = true
		}
	}
	if hasWarn {
		return SeedanceReadinessStatusWarning, true, false
	}
	return SeedanceReadinessStatusReady, true, true
}

func csvItemCount(value string) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	count := 0
	for _, item := range strings.Split(value, ",") {
		if strings.TrimSpace(item) != "" {
			count++
		}
	}
	return count
}
