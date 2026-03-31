package middleware

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func TenantAudit() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if !common.MultiTenantEnabled || !common.TenantAuditEnabled {
			return
		}
		if c.Request.Method == http.MethodGet || c.Request.Method == http.MethodHead || c.Request.Method == http.MethodOptions {
			return
		}
		tenantID := strings.TrimSpace(c.GetString("tenant_id"))
		if tenantID == "" {
			tenantID = common.GetDefaultTenantID()
		}
		payload, _ := json.Marshal(map[string]any{
			"method": c.Request.Method,
			"path":   c.FullPath(),
			"status": c.Writer.Status(),
			"ip":     c.ClientIP(),
		})
		_ = model.CreateTenantAuditLog(&model.TenantAuditLog{
			TenantID:   tenantID,
			UserID:     c.GetInt("id"),
			Action:     c.Request.Method,
			Resource:   c.FullPath(),
			ResourceID: c.Param("id"),
			Detail:     string(payload),
		})
	}
}
