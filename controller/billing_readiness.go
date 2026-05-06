package controller

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

func boolQueryOrEnv(c *gin.Context, queryName string, envName string, defaultValue bool) bool {
	value := strings.TrimSpace(c.Query(queryName))
	if value == "" {
		return common.GetEnvOrDefaultBool(envName, defaultValue)
	}
	switch strings.ToLower(value) {
	case "true", "1", "yes", "y", "on":
		return true
	case "false", "0", "no", "n", "off":
		return false
	default:
		return defaultValue
	}
}

func GetSeedanceBillingReadiness(c *gin.Context) {
	modelName := strings.TrimSpace(c.Query("model_name"))
	if modelName == "" {
		modelName = strings.TrimSpace(c.Query("model"))
	}
	readiness := service.BuildSeedanceBillingReadiness(service.SeedanceBillingReadinessOptions{
		ModelName:                modelName,
		UsageConfirmed:           boolQueryOrEnv(c, "usage_confirmed", "USAGE_CONFIRMED", false),
		RequireLocalWorker:       boolQueryOrEnv(c, "require_local_worker", "REQUIRE_LOCAL_WORKER", false),
		RequireCallbackAllowlist: boolQueryOrEnv(c, "require_callback_allowlist", "REQUIRE_CALLBACK_ALLOWLIST", false),
	})
	common.ApiSuccess(c, readiness)
}
