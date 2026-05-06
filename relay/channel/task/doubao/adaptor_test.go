package doubao

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/system_setting"
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

func TestValidateSeedance2RequestRejectsUnconfirmedProductionPrice(t *testing.T) {
	t.Setenv("SEEDANCE_REQUIRE_PRICE_CONFIRMATION", "true")
	t.Setenv("SEEDANCE_PRICE_CONFIRMED", "false")
	adaptor := &TaskAdaptor{}

	err := adaptor.validateSeedance2Request(&relaycommon.TaskSubmitReq{
		Model:  "doubao-seedance-2-0",
		Prompt: "make a short video",
	})
	if err == nil {
		t.Fatalf("validateSeedance2Request() expected error")
	}
	if err.Code != "seedance_price_not_confirmed" {
		t.Fatalf("error code = %q", err.Code)
	}
	if err.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status code = %d", err.StatusCode)
	}
}

func TestValidateSeedance2RequestAllowsConfirmedProductionPrice(t *testing.T) {
	t.Setenv("SEEDANCE_REQUIRE_PRICE_CONFIRMATION", "true")
	t.Setenv("SEEDANCE_PRICE_CONFIRMED", "true")
	adaptor := &TaskAdaptor{}

	err := adaptor.validateSeedance2Request(&relaycommon.TaskSubmitReq{
		Model:  "doubao-seedance-2-0",
		Prompt: "make a short video",
	})
	if err != nil {
		t.Fatalf("validateSeedance2Request() unexpected error = %v", err)
	}
}

func TestValidateSeedance2RequestRejectsMappedSeedanceWhenPriceUnconfirmed(t *testing.T) {
	t.Setenv("SEEDANCE_REQUIRE_PRICE_CONFIRMATION", "true")
	t.Setenv("SEEDANCE_PRICE_CONFIRMED", "false")
	adaptor := &TaskAdaptor{}

	err := adaptor.validateSeedance2RequestForModel(&relaycommon.TaskSubmitReq{
		Model:  "customer-seedance-alias",
		Prompt: "make a short video",
	}, "doubao-seedance-2-0")
	if err == nil {
		t.Fatalf("validateSeedance2RequestForModel() expected error")
	}
	if err.Code != "seedance_price_not_confirmed" {
		t.Fatalf("error code = %q", err.Code)
	}
}

func TestConvertToRequestPayloadUsesSeedanceReferenceContent(t *testing.T) {
	adaptor := &TaskAdaptor{}

	payload, err := adaptor.convertToRequestPayload(&relaycommon.TaskSubmitReq{
		Model:              "doubao-seedance-2-0",
		Prompt:             "make a short video",
		Images:             []string{" https://cdn.example.com/base.png "},
		ReferenceImageURL:  "https://cdn.example.com/ref-1.png",
		ReferenceImageURLs: []string{"https://cdn.example.com/ref-2.png"},
		FirstFrameURL:      "https://cdn.example.com/first.png",
		LastFrameURL:       "https://cdn.example.com/last.png",
		ReferenceVideoURL:  "https://cdn.example.com/ref-1.mp4",
		ReferenceVideoURLs: []string{"https://cdn.example.com/ref-2.mp4"},
	})
	if err != nil {
		t.Fatalf("convertToRequestPayload() error = %v", err)
	}

	assertContentItem(t, payload.Content, "image_url", "", "https://cdn.example.com/base.png")
	assertContentItem(t, payload.Content, "image_url", "reference_image", "https://cdn.example.com/ref-1.png")
	assertContentItem(t, payload.Content, "image_url", "reference_image", "https://cdn.example.com/ref-2.png")
	assertContentItem(t, payload.Content, "image_url", "first_frame", "https://cdn.example.com/first.png")
	assertContentItem(t, payload.Content, "image_url", "last_frame", "https://cdn.example.com/last.png")
	assertContentItem(t, payload.Content, "video", "reference_video", "https://cdn.example.com/ref-1.mp4")
	assertContentItem(t, payload.Content, "video", "reference_video", "https://cdn.example.com/ref-2.mp4")
}

func TestConvertToRequestPayloadUsesSeedanceMetadataReferences(t *testing.T) {
	adaptor := &TaskAdaptor{}

	payload, err := adaptor.convertToRequestPayload(&relaycommon.TaskSubmitReq{
		Model:  "doubao-seedance-2-0",
		Prompt: "make a short video",
		Metadata: map[string]interface{}{
			"first_frame_url":      "https://cdn.example.com/first.png",
			"reference_video_urls": []interface{}{"https://cdn.example.com/ref.mp4"},
		},
	})
	if err != nil {
		t.Fatalf("convertToRequestPayload() error = %v", err)
	}

	assertContentItem(t, payload.Content, "image_url", "first_frame", "https://cdn.example.com/first.png")
	assertContentItem(t, payload.Content, "video", "reference_video", "https://cdn.example.com/ref.mp4")
}

func TestValidateSeedance2RequestRejectsPrivateImageURL(t *testing.T) {
	withSeedanceFetchSetting(t)
	adaptor := &TaskAdaptor{}

	err := adaptor.validateSeedance2Request(&relaycommon.TaskSubmitReq{
		Model:  "doubao-seedance-2-0",
		Prompt: "make a short video",
		Images: []string{"http://127.0.0.1/admin"},
	})
	if err == nil {
		t.Fatalf("validateSeedance2Request() expected error")
	}
	if err.Code != "invalid_image_url" {
		t.Fatalf("error code = %q", err.Code)
	}
}

func TestValidateSeedance2RequestRejectsUnsafeCallbackURL(t *testing.T) {
	withSeedanceFetchSetting(t)
	adaptor := &TaskAdaptor{}

	err := adaptor.validateSeedance2Request(&relaycommon.TaskSubmitReq{
		Model:       "doubao-seedance-2-0",
		Prompt:      "make a short video",
		CallbackURL: "https://token:secret@example.com/callback",
	})
	if err == nil {
		t.Fatalf("validateSeedance2Request() expected error")
	}
	if err.Code != "invalid_callback_url" {
		t.Fatalf("error code = %q", err.Code)
	}
}

func TestValidateSeedance2RequestRejectsPrivateReferenceVideoURL(t *testing.T) {
	withSeedanceFetchSetting(t)
	adaptor := &TaskAdaptor{}

	err := adaptor.validateSeedance2Request(&relaycommon.TaskSubmitReq{
		Model:             "doubao-seedance-2-0",
		Prompt:            "make a short video",
		ReferenceVideoURL: "http://127.0.0.1/video.mp4",
	})
	if err == nil {
		t.Fatalf("validateSeedance2Request() expected error")
	}
	if err.Code != "invalid_video_url" {
		t.Fatalf("error code = %q", err.Code)
	}
}

func TestValidateSeedance2RequestRejectsTooManyReferenceVideos(t *testing.T) {
	withSeedanceFetchSetting(t)
	adaptor := &TaskAdaptor{}

	err := adaptor.validateSeedance2Request(&relaycommon.TaskSubmitReq{
		Model:             "doubao-seedance-2-0",
		Prompt:            "make a short video",
		ReferenceVideoURL: "https://cdn.example.com/ref-0.mp4",
		ReferenceVideoURLs: []string{
			"https://cdn.example.com/ref-1.mp4",
			"https://cdn.example.com/ref-2.mp4",
			"https://cdn.example.com/ref-3.mp4",
		},
	})
	if err == nil {
		t.Fatalf("validateSeedance2Request() expected error")
	}
	if err.Code != "invalid_videos" {
		t.Fatalf("error code = %q", err.Code)
	}
}

func TestValidateSeedance2RequestAllowsPublicImageURL(t *testing.T) {
	withSeedanceFetchSetting(t)
	adaptor := &TaskAdaptor{}

	err := adaptor.validateSeedance2Request(&relaycommon.TaskSubmitReq{
		Model:  "doubao-seedance-2-0",
		Prompt: "make a short video",
		Images: []string{"https://cdn.example.com/seedance/reference.png"},
	})
	if err != nil {
		t.Fatalf("validateSeedance2Request() unexpected error = %v", err)
	}
}

func TestValidateSeedance2RequestRejectsImageURLOutsideAllowlist(t *testing.T) {
	withSeedanceFetchSetting(t)
	t.Setenv("SEEDANCE_REMOTE_URL_ALLOWLIST", "cdn.example.com,*.trusted.example")
	adaptor := &TaskAdaptor{}

	err := adaptor.validateSeedance2Request(&relaycommon.TaskSubmitReq{
		Model:  "doubao-seedance-2-0",
		Prompt: "make a short video",
		Images: []string{"https://evil.example.com/seedance/reference.png"},
	})
	if err == nil {
		t.Fatalf("validateSeedance2Request() expected error")
	}
	if err.Code != "invalid_image_url" {
		t.Fatalf("error code = %q", err.Code)
	}
}

func TestValidateSeedance2RequestAllowsWildcardAllowlistedImageURL(t *testing.T) {
	withSeedanceFetchSetting(t)
	t.Setenv("SEEDANCE_REMOTE_URL_ALLOWLIST", "*.example.com")
	adaptor := &TaskAdaptor{}

	err := adaptor.validateSeedance2Request(&relaycommon.TaskSubmitReq{
		Model:  "doubao-seedance-2-0",
		Prompt: "make a short video",
		Images: []string{"https://media.example.com/seedance/reference.png"},
	})
	if err != nil {
		t.Fatalf("validateSeedance2Request() unexpected error = %v", err)
	}
}

func TestValidateSeedance2RequestRejectsCallbackURLOutsideAllowlist(t *testing.T) {
	withSeedanceFetchSetting(t)
	t.Setenv("SEEDANCE_CALLBACK_URL_ALLOWLIST", "hooks.example.com")
	adaptor := &TaskAdaptor{}

	err := adaptor.validateSeedance2Request(&relaycommon.TaskSubmitReq{
		Model:       "doubao-seedance-2-0",
		Prompt:      "make a short video",
		CallbackURL: "https://callback.example.com/seedance",
	})
	if err == nil {
		t.Fatalf("validateSeedance2Request() expected error")
	}
	if err.Code != "invalid_callback_url" {
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

func withSeedanceFetchSetting(t *testing.T) {
	t.Helper()

	fetchSetting := system_setting.GetFetchSetting()
	previous := *fetchSetting
	t.Cleanup(func() {
		*fetchSetting = previous
	})

	fetchSetting.EnableSSRFProtection = true
	fetchSetting.AllowPrivateIp = false
	fetchSetting.DomainFilterMode = false
	fetchSetting.IpFilterMode = false
	fetchSetting.DomainList = nil
	fetchSetting.IpList = nil
	fetchSetting.AllowedPorts = []string{"80", "443"}
	fetchSetting.ApplyIPFilterForDomain = false
}

func assertContentItem(t *testing.T, content []ContentItem, itemType string, role string, expectedURL string) {
	t.Helper()

	for _, item := range content {
		if item.Type != itemType || item.Role != role {
			continue
		}
		switch itemType {
		case "image_url":
			if item.ImageURL != nil && item.ImageURL.URL == expectedURL {
				return
			}
		case "video":
			if item.Video != nil && item.Video.URL == expectedURL {
				return
			}
		}
	}

	t.Fatalf("content item type=%q role=%q url=%q not found in %#v", itemType, role, expectedURL, content)
}
