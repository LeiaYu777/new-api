package middleware

import (
	"crypto/subtle"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
)

func SetupGuard() gin.HandlerFunc {
	setupToken := strings.TrimSpace(os.Getenv("SETUP_TOKEN"))
	allowPrivateIP := common.GetEnvOrDefaultBool("SETUP_ALLOW_PRIVATE_IP", true)
	return func(c *gin.Context) {
		if constant.Setup {
			c.Next()
			return
		}

		if setupToken != "" {
			provided := strings.TrimSpace(c.GetHeader("X-Setup-Token"))
			if provided == "" {
				provided = strings.TrimSpace(c.Query("setup_token"))
			}
			if subtle.ConstantTimeCompare([]byte(provided), []byte(setupToken)) != 1 {
				c.JSON(http.StatusForbidden, gin.H{
					"success": false,
					"message": "setup token is required",
				})
				c.Abort()
				return
			}
			c.Next()
			return
		}

		if allowPrivateIP && isPrivateOrLoopbackIP(c.ClientIP()) {
			c.Next()
			return
		}

		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "setup endpoint is restricted, set SETUP_TOKEN to enable remote setup",
		})
		c.Abort()
	}
}

func isPrivateOrLoopbackIP(rawIP string) bool {
	ip := net.ParseIP(strings.TrimSpace(rawIP))
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate()
}
