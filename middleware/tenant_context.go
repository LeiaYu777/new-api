package middleware

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

var tenantIDPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_\-]{1,63}$`)

func TenantContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !common.MultiTenantEnabled {
			c.Next()
			return
		}
		tenantID := strings.TrimSpace(c.GetString("tenant_id"))
		if tenantID == "" {
			tenantID = strings.TrimSpace(c.GetHeader(common.TenantHeaderKey))
		}
		if tenantID == "" {
			tenantID = strings.TrimSpace(c.Query("tenant_id"))
		}
		if tenantID == "" {
			tenantID = common.GetDefaultTenantID()
		}
		if !tenantIDPattern.MatchString(tenantID) {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "tenant_id 格式无效",
			})
			c.Abort()
			return
		}
		c.Set("tenant_id", tenantID)
		c.Next()
	}
}
