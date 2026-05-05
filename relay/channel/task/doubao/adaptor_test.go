package doubao

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
)

func TestConvertToRequestPayloadUsesTopLevelSeedanceFields(t *testing.T) {
	audio := dto.BoolValue(true)
	watermark := dto.BoolValue(false)
	adaptor := &TaskAdaptor{}

	payload, err := adaptor.convertToRequestPayload(&relaycommon.TaskSubmitReq{
		Model:         "doubao-seedance-2-0",
		Prompt:        "make a short video",
		Duration:      8,
		Resolution:    "1080p",
		Ratio:         "16:9",
		GenerateAudio: &audio,
		Watermark:     &watermark,
	})
	if err != nil {
		t.Fatalf("convertToRequestPayload() error = %v", err)
	}
	if payload.Model != "doubao-seedance-2-0" {
		t.Fatalf("model = %q", payload.Model)
	}
	if int(payload.Duration) != 8 {
		t.Fatalf("duration = %d", payload.Duration)
	}
	if payload.Resolution != "1080p" {
		t.Fatalf("resolution = %q", payload.Resolution)
	}
	if payload.Ratio != "16:9" {
		t.Fatalf("ratio = %q", payload.Ratio)
	}
	if payload.GenerateAudio == nil || !bool(*payload.GenerateAudio) {
		t.Fatalf("generate_audio not preserved")
	}
	if payload.Watermark == nil || bool(*payload.Watermark) {
		t.Fatalf("watermark not preserved")
	}
}

func TestValidateSeedance2RequestRejectsUnknownMetadata(t *testing.T) {
	adaptor := &TaskAdaptor{}
	err := adaptor.validateSeedance2Request(&relaycommon.TaskSubmitReq{
		Model:    "doubao-seedance-2-0",
		Prompt:   "make a short video",
		Metadata: map[string]interface{}{"unsafe_extra": true},
	})
	if err == nil {
		t.Fatalf("validateSeedance2Request() expected error")
	}
	if err.Code != "invalid_request" {
		t.Fatalf("error code = %q", err.Code)
	}
}

func TestEstimateBillingSeedance2Ratios(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	audio := dto.BoolValue(true)
	c.Set("task_request", relaycommon.TaskSubmitReq{
		Model:         "doubao-seedance-2-0",
		Prompt:        "make a short video",
		Duration:      10,
		Resolution:    "1080p",
		GenerateAudio: &audio,
	})

	adaptor := &TaskAdaptor{}
	ratios := adaptor.EstimateBilling(c, &relaycommon.RelayInfo{OriginModelName: "doubao-seedance-2-0"})
	if ratios["duration"] != 2 {
		t.Fatalf("duration ratio = %v", ratios["duration"])
	}
	if ratios["resolution"] != 1.6 {
		t.Fatalf("resolution ratio = %v", ratios["resolution"])
	}
	if ratios["audio"] != 1.2 {
		t.Fatalf("audio ratio = %v", ratios["audio"])
	}
}
