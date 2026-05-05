package doubao

import "strings"

var ModelList = []string{
	"doubao-seedance-1-0-pro-250528",
	"doubao-seedance-1-0-lite-t2v",
	"doubao-seedance-1-0-lite-i2v",
	"doubao-seedance-1-5-pro-251215",
	"doubao-seedance-2-0",
	"doubao-seedance-2-0-pro",
	"doubao-seedance-2-0-lite",
	"seedance-2-0",
	"seedance-2-0-pro",
	"seedance-2-0-lite",
}

var ChannelName = "doubao-video"

func IsSeedance2Model(model string) bool {
	model = strings.ToLower(strings.TrimSpace(model))
	return strings.Contains(model, "seedance-2") || strings.Contains(model, "seedance2")
}
