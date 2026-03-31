package middleware

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"os"
	"strings"
	"time"
)

func CORS() gin.HandlerFunc {
	config := cors.Config{
		AllowCredentials: common.GetEnvOrDefaultBool("CORS_ALLOW_CREDENTIALS", true),
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders: []string{
			"Authorization",
			"Content-Type",
			"X-Requested-With",
			"X-Setup-Token",
			"X-Newapi-Request-Id",
		},
		ExposeHeaders: []string{
			"Content-Length",
			"Content-Type",
			"X-Newapi-Request-Id",
		},
		MaxAge: 12 * time.Hour,
	}
	allowedOrigins := getAllowedOrigins()
	if len(allowedOrigins) > 0 {
		config.AllowOrigins = allowedOrigins
	} else {
		// No cross-origin requests are allowed unless CORS_ALLOW_ORIGINS / FRONTEND_BASE_URL is configured.
		config.AllowOriginFunc = func(string) bool {
			return false
		}
	}
	return cors.New(config)
}

func getAllowedOrigins() []string {
	raw := strings.TrimSpace(common.GetEnvOrDefaultString("CORS_ALLOW_ORIGINS", ""))
	if raw != "" {
		parts := strings.Split(raw, ",")
		origins := make([]string, 0, len(parts))
		for _, part := range parts {
			origin := strings.TrimSpace(part)
			if origin != "" {
				origins = append(origins, origin)
			}
		}
		return origins
	}
	frontendBaseURL := strings.TrimSpace(os.Getenv("FRONTEND_BASE_URL"))
	if frontendBaseURL != "" {
		return []string{strings.TrimSuffix(frontendBaseURL, "/")}
	}
	return nil
}

func PoweredBy() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-New-Api-Version", common.Version)
		c.Next()
	}
}
