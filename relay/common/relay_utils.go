package common

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
)

type HasPrompt interface {
	GetPrompt() string
}

type HasImage interface {
	HasImage() bool
}

func GetFullRequestURL(baseURL string, requestURL string, channelType int) string {
	fullRequestURL := fmt.Sprintf("%s%s", baseURL, requestURL)

	if strings.HasPrefix(baseURL, "https://gateway.ai.cloudflare.com") {
		switch channelType {
		case constant.ChannelTypeOpenAI:
			fullRequestURL = fmt.Sprintf("%s%s", baseURL, strings.TrimPrefix(requestURL, "/v1"))
		case constant.ChannelTypeAzure:
			fullRequestURL = fmt.Sprintf("%s%s", baseURL, strings.TrimPrefix(requestURL, "/openai/deployments"))
		}
	}
	return fullRequestURL
}

func GetAPIVersion(c *gin.Context) string {
	query := c.Request.URL.Query()
	apiVersion := query.Get("api-version")
	if apiVersion == "" {
		apiVersion = c.GetString("api_version")
	}
	return apiVersion
}

func createTaskError(err error, code string, statusCode int, localError bool) *dto.TaskError {
	return &dto.TaskError{
		Code:       code,
		Message:    err.Error(),
		StatusCode: statusCode,
		LocalError: localError,
		Error:      err,
	}
}

func storeTaskRequest(c *gin.Context, info *RelayInfo, action string, requestObj TaskSubmitReq) {
	info.Action = action
	c.Set("task_request", requestObj)
}
func GetTaskRequest(c *gin.Context) (TaskSubmitReq, error) {
	v, exists := c.Get("task_request")
	if !exists {
		return TaskSubmitReq{}, fmt.Errorf("request not found in context")
	}
	req, ok := v.(TaskSubmitReq)
	if !ok {
		return TaskSubmitReq{}, fmt.Errorf("invalid task request type")
	}
	return req, nil
}

func validatePrompt(prompt string) *dto.TaskError {
	if strings.TrimSpace(prompt) == "" {
		return createTaskError(fmt.Errorf("prompt is required"), "invalid_request", http.StatusBadRequest, true)
	}
	return nil
}

func validateMultipartTaskRequest(c *gin.Context, info *RelayInfo, action string) (TaskSubmitReq, error) {
	var req TaskSubmitReq
	if _, err := c.MultipartForm(); err != nil {
		return req, err
	}

	formData := c.Request.PostForm
	req = TaskSubmitReq{
		Prompt:            formData.Get("prompt"),
		Model:             formData.Get("model"),
		Mode:              formData.Get("mode"),
		Image:             formData.Get("image"),
		ReferenceImageURL: formData.Get("reference_image_url"),
		FirstFrameURL:     formData.Get("first_frame_url"),
		LastFrameURL:      formData.Get("last_frame_url"),
		ReferenceVideoURL: formData.Get("reference_video_url"),
		Size:              formData.Get("size"),
		Resolution:        formData.Get("resolution"),
		Ratio:             formData.Get("ratio"),
		ServiceTier:       formData.Get("service_tier"),
		CallbackURL:       formData.Get("callback_url"),
		Metadata:          make(map[string]interface{}),
	}

	if durationStr := formData.Get("seconds"); durationStr != "" {
		if duration, err := strconv.Atoi(durationStr); err == nil {
			req.Duration = duration
		}
	}
	if durationStr := formData.Get("duration"); durationStr != "" {
		if duration, err := strconv.Atoi(durationStr); err == nil {
			req.Duration = duration
		}
	}
	if framesStr := formData.Get("frames"); framesStr != "" {
		if frames, err := strconv.Atoi(framesStr); err == nil {
			req.Frames = dto.IntValue(frames)
		}
	}
	if seedStr := formData.Get("seed"); seedStr != "" {
		if seed, err := strconv.Atoi(seedStr); err == nil {
			req.Seed = dto.IntValue(seed)
		}
	}
	if expiresStr := formData.Get("execution_expires_after"); expiresStr != "" {
		if expires, err := strconv.Atoi(expiresStr); err == nil {
			req.ExecutionExpiresAfter = dto.IntValue(expires)
		}
	}
	if boolValue, ok := parseTaskBoolValue(formData.Get("return_last_frame")); ok {
		req.ReturnLastFrame = &boolValue
	}
	if boolValue, ok := parseTaskBoolValue(formData.Get("generate_audio")); ok {
		req.GenerateAudio = &boolValue
	}
	if boolValue, ok := parseTaskBoolValue(formData.Get("draft")); ok {
		req.Draft = &boolValue
	}
	if boolValue, ok := parseTaskBoolValue(formData.Get("camera_fixed")); ok {
		req.CameraFixed = &boolValue
	}
	if boolValue, ok := parseTaskBoolValue(formData.Get("watermark")); ok {
		req.Watermark = &boolValue
	}

	if images := formData["images"]; len(images) > 0 {
		req.Images = images
	}
	if referenceImageURLs := formData["reference_image_urls"]; len(referenceImageURLs) > 0 {
		req.ReferenceImageURLs = referenceImageURLs
	}
	if referenceVideoURLs := formData["reference_video_urls"]; len(referenceVideoURLs) > 0 {
		req.ReferenceVideoURLs = referenceVideoURLs
	}

	for key, values := range formData {
		if len(values) > 0 && !isKnownTaskField(key) {
			if intVal, err := strconv.Atoi(values[0]); err == nil {
				req.Metadata[key] = intVal
			} else if floatVal, err := strconv.ParseFloat(values[0], 64); err == nil {
				req.Metadata[key] = floatVal
			} else {
				req.Metadata[key] = values[0]
			}
		}
	}
	return req, nil
}

func ValidateMultipartDirect(c *gin.Context, info *RelayInfo) *dto.TaskError {
	var prompt string
	var model string
	var seconds int
	var size string
	var hasInputReference bool

	var req TaskSubmitReq
	if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		return createTaskError(err, "invalid_json", http.StatusBadRequest, true)
	}

	prompt = req.Prompt
	model = req.Model
	size = req.Size
	seconds, _ = strconv.Atoi(req.Seconds)
	if seconds == 0 {
		seconds = req.Duration
	}
	if req.InputReference != "" {
		req.Images = []string{req.InputReference}
	}

	if strings.TrimSpace(req.Model) == "" {
		return createTaskError(fmt.Errorf("model field is required"), "missing_model", http.StatusBadRequest, true)
	}

	if req.HasImage() {
		hasInputReference = true
	}

	if taskErr := validatePrompt(prompt); taskErr != nil {
		return taskErr
	}

	action := constant.TaskActionTextGenerate
	if hasInputReference {
		action = constant.TaskActionGenerate
	}
	if strings.HasPrefix(model, "sora-2") {

		if size == "" {
			size = "720x1280"
		}

		if seconds <= 0 {
			seconds = 4
		}

		if model == "sora-2" && !lo.Contains([]string{"720x1280", "1280x720"}, size) {
			return createTaskError(fmt.Errorf("sora-2 size is invalid"), "invalid_size", http.StatusBadRequest, true)
		}
		if model == "sora-2-pro" && !lo.Contains([]string{"720x1280", "1280x720", "1792x1024", "1024x1792"}, size) {
			return createTaskError(fmt.Errorf("sora-2 size is invalid"), "invalid_size", http.StatusBadRequest, true)
		}
		// OtherRatios 已移到 Sora adaptor 的 EstimateBilling 中设置
	}

	storeTaskRequest(c, info, action, req)

	return nil
}

func isKnownTaskField(field string) bool {
	knownFields := map[string]bool{
		"prompt":                  true,
		"model":                   true,
		"mode":                    true,
		"image":                   true,
		"images":                  true,
		"reference_image_url":     true,
		"reference_image_urls":    true,
		"first_frame_url":         true,
		"last_frame_url":          true,
		"reference_video_url":     true,
		"reference_video_urls":    true,
		"size":                    true,
		"duration":                true,
		"seconds":                 true,
		"input_reference":         true, // Sora 特有字段
		"callback_url":            true,
		"return_last_frame":       true,
		"service_tier":            true,
		"execution_expires_after": true,
		"generate_audio":          true,
		"draft":                   true,
		"resolution":              true,
		"ratio":                   true,
		"frames":                  true,
		"seed":                    true,
		"camera_fixed":            true,
		"watermark":               true,
	}
	return knownFields[field]
}

func parseTaskBoolValue(raw string) (dto.BoolValue, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "true", "1", "yes", "on":
		v := dto.BoolValue(true)
		return v, true
	case "false", "0", "no", "off":
		v := dto.BoolValue(false)
		return v, true
	default:
		return false, false
	}
}

func ValidateBasicTaskRequest(c *gin.Context, info *RelayInfo, action string) *dto.TaskError {
	var err error
	contentType := c.GetHeader("Content-Type")
	var req TaskSubmitReq
	if strings.HasPrefix(contentType, "multipart/form-data") {
		req, err = validateMultipartTaskRequest(c, info, action)
		if err != nil {
			return createTaskError(err, "invalid_multipart_form", http.StatusBadRequest, true)
		}
	} else if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		return createTaskError(err, "invalid_request", http.StatusBadRequest, true)
	}

	if taskErr := validatePrompt(req.Prompt); taskErr != nil {
		return taskErr
	}

	if len(req.Images) == 0 && strings.TrimSpace(req.Image) != "" {
		// 兼容单图上传
		req.Images = []string{req.Image}
	}

	storeTaskRequest(c, info, action, req)
	return nil
}
