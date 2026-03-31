package common

import "strings"

func GetDefaultTenantID() string {
	if strings.TrimSpace(DefaultTenantId) == "" {
		return "default"
	}
	return strings.TrimSpace(DefaultTenantId)
}
