package doubao

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
)

// ============================
// Request / Response structures
// ============================

type ContentItem struct {
	Type     string          `json:"type"`                // "text", "image_url" or "video"
	Text     string          `json:"text,omitempty"`      // for text type
	ImageURL *ImageURL       `json:"image_url,omitempty"` // for image_url type
	Video    *VideoReference `json:"video,omitempty"`     // for video (sample) type
	Role     string          `json:"role,omitempty"`      // reference_image / first_frame / last_frame
}

type ImageURL struct {
	URL string `json:"url"`
}

type VideoReference struct {
	URL string `json:"url"` // Draft video URL
}

type requestPayload struct {
	Model                 string         `json:"model"`
	Content               []ContentItem  `json:"content"`
	CallbackURL           string         `json:"callback_url,omitempty"`
	ReturnLastFrame       *dto.BoolValue `json:"return_last_frame,omitempty"`
	ServiceTier           string         `json:"service_tier,omitempty"`
	ExecutionExpiresAfter dto.IntValue   `json:"execution_expires_after,omitempty"`
	GenerateAudio         *dto.BoolValue `json:"generate_audio,omitempty"`
	Draft                 *dto.BoolValue `json:"draft,omitempty"`
	Resolution            string         `json:"resolution,omitempty"`
	Ratio                 string         `json:"ratio,omitempty"`
	Duration              dto.IntValue   `json:"duration,omitempty"`
	Frames                dto.IntValue   `json:"frames,omitempty"`
	Seed                  dto.IntValue   `json:"seed,omitempty"`
	CameraFixed           *dto.BoolValue `json:"camera_fixed,omitempty"`
	Watermark             *dto.BoolValue `json:"watermark,omitempty"`
}

type responsePayload struct {
	ID string `json:"id"` // task_id
}

type responseTask struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Status  string `json:"status"`
	Content struct {
		VideoURL string `json:"video_url"`
	} `json:"content"`
	Seed            int    `json:"seed"`
	Resolution      string `json:"resolution"`
	Duration        int    `json:"duration"`
	Ratio           string `json:"ratio"`
	FramesPerSecond int    `json:"framespersecond"`
	ServiceTier     string `json:"service_tier"`
	Usage           struct {
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	CreatedAt int64 `json:"created_at"`
	UpdatedAt int64 `json:"updated_at"`
}

// ============================
// Adaptor implementation
// ============================

type TaskAdaptor struct {
	taskcommon.BaseBilling
	ChannelType int
	apiKey      string
	baseURL     string
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.ChannelType = info.ChannelType
	a.baseURL = info.ChannelBaseUrl
	a.apiKey = info.ApiKey
}

// ValidateRequestAndSetAction parses body, validates fields and sets default action.
func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) (taskErr *dto.TaskError) {
	// Accept only POST /v1/video/generations as "generate" action.
	if taskErr = relaycommon.ValidateBasicTaskRequest(c, info, constant.TaskActionGenerate); taskErr != nil {
		return taskErr
	}
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	if IsSeedance2Model(req.Model) {
		return a.validateSeedance2Request(&req)
	}
	return nil
}

// BuildRequestURL constructs the upstream URL.
func (a *TaskAdaptor) BuildRequestURL(info *relaycommon.RelayInfo) (string, error) {
	return fmt.Sprintf("%s/api/v3/contents/generations/tasks", a.baseURL), nil
}

// BuildRequestHeader sets required headers.
func (a *TaskAdaptor) BuildRequestHeader(c *gin.Context, req *http.Request, info *relaycommon.RelayInfo) error {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	return nil
}

// BuildRequestBody converts request into Doubao specific format.
func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil, err
	}

	body, err := a.convertToRequestPayload(&req)
	if err != nil {
		return nil, errors.Wrap(err, "convert request payload failed")
	}
	if info.IsModelMapped {
		body.Model = info.UpstreamModelName
	} else {
		info.UpstreamModelName = body.Model
	}
	if IsSeedance2Model(body.Model) {
		if taskErr := a.validateSeedance2Request(&req); taskErr != nil {
			return nil, taskErr.Error
		}
	}
	data, err := common.Marshal(body)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}

// DoRequest delegates to common helper.
func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

// DoResponse handles upstream response, returns taskID etc.
func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (taskID string, taskData []byte, taskErr *dto.TaskError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		taskErr = service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
		return
	}
	_ = resp.Body.Close()

	// Parse Doubao response
	var dResp responsePayload
	if err := common.Unmarshal(responseBody, &dResp); err != nil {
		taskErr = service.TaskErrorWrapper(errors.Wrapf(err, "body: %s", responseBody), "unmarshal_response_body_failed", http.StatusInternalServerError)
		return
	}

	if dResp.ID == "" {
		taskErr = service.TaskErrorWrapper(fmt.Errorf("task_id is empty"), "invalid_response", http.StatusInternalServerError)
		return
	}

	ov := dto.NewOpenAIVideo()
	ov.ID = info.PublicTaskID
	ov.TaskID = info.PublicTaskID
	ov.CreatedAt = time.Now().Unix()
	ov.Model = info.OriginModelName

	c.JSON(http.StatusOK, ov)
	return dResp.ID, responseBody, nil
}

// FetchTask fetch task status
func (a *TaskAdaptor) FetchTask(baseUrl, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid task_id")
	}

	uri := fmt.Sprintf("%s/api/v3/contents/generations/tasks/%s", baseUrl, taskID)

	req, err := http.NewRequest(http.MethodGet, uri, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)

	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	return client.Do(req)
}

func (a *TaskAdaptor) GetModelList() []string {
	return ModelList
}

func (a *TaskAdaptor) GetChannelName() string {
	return ChannelName
}

func (a *TaskAdaptor) convertToRequestPayload(req *relaycommon.TaskSubmitReq) (*requestPayload, error) {
	r := requestPayload{
		Model:                 req.Model,
		Content:               []ContentItem{},
		CallbackURL:           strings.TrimSpace(req.CallbackURL),
		ReturnLastFrame:       req.ReturnLastFrame,
		ServiceTier:           req.ServiceTier,
		ExecutionExpiresAfter: req.ExecutionExpiresAfter,
		GenerateAudio:         req.GenerateAudio,
		Draft:                 req.Draft,
		Resolution:            req.Resolution,
		Ratio:                 req.Ratio,
		Duration:              dto.IntValue(req.Duration),
		Frames:                req.Frames,
		Seed:                  req.Seed,
		CameraFixed:           req.CameraFixed,
		Watermark:             req.Watermark,
	}
	if r.Duration == 0 && req.Seconds != "" {
		if seconds, err := strconv.Atoi(req.Seconds); err == nil {
			r.Duration = dto.IntValue(seconds)
		}
	}
	if r.Resolution == "" && req.Size != "" {
		r.Resolution = req.Size
	}

	// Add text prompt
	if req.Prompt != "" {
		r.Content = append(r.Content, ContentItem{
			Type: "text",
			Text: req.Prompt,
		})
	}

	// Add images if present
	if req.HasImage() {
		for _, imgURL := range req.Images {
			appendSeedanceImageContent(&r, imgURL, "")
		}
	}
	appendSeedanceImageContent(&r, req.ReferenceImageURL, "reference_image")
	for _, imgURL := range req.ReferenceImageURLs {
		appendSeedanceImageContent(&r, imgURL, "reference_image")
	}
	appendSeedanceImageContent(&r, req.FirstFrameURL, "first_frame")
	appendSeedanceImageContent(&r, req.LastFrameURL, "last_frame")
	appendSeedanceVideoContent(&r, req.ReferenceVideoURL, "reference_video")
	for _, videoURL := range req.ReferenceVideoURLs {
		appendSeedanceVideoContent(&r, videoURL, "reference_video")
	}

	metadata := req.Metadata
	if err := taskcommon.UnmarshalMetadata(metadata, &r); err != nil {
		return nil, errors.Wrap(err, "unmarshal metadata failed")
	}
	appendSeedanceMetadataReferences(&r, metadata)

	return &r, nil
}

func appendSeedanceImageContent(payload *requestPayload, imageURL string, role string) {
	imageURL = strings.TrimSpace(imageURL)
	if imageURL == "" {
		return
	}
	payload.Content = append(payload.Content, ContentItem{
		Type: "image_url",
		ImageURL: &ImageURL{
			URL: imageURL,
		},
		Role: role,
	})
}

func appendSeedanceVideoContent(payload *requestPayload, videoURL string, role string) {
	videoURL = strings.TrimSpace(videoURL)
	if videoURL == "" {
		return
	}
	payload.Content = append(payload.Content, ContentItem{
		Type: "video",
		Video: &VideoReference{
			URL: videoURL,
		},
		Role: role,
	})
}

func appendSeedanceMetadataReferences(payload *requestPayload, metadata map[string]interface{}) {
	if metadata == nil {
		return
	}
	appendSeedanceImageContent(payload, seedanceMetadataString(metadata, "reference_image_url"), "reference_image")
	for _, imageURL := range seedanceMetadataStringSlice(metadata, "reference_image_urls") {
		appendSeedanceImageContent(payload, imageURL, "reference_image")
	}
	appendSeedanceImageContent(payload, seedanceMetadataString(metadata, "first_frame_url"), "first_frame")
	appendSeedanceImageContent(payload, seedanceMetadataString(metadata, "last_frame_url"), "last_frame")
	appendSeedanceVideoContent(payload, seedanceMetadataString(metadata, "reference_video_url"), "reference_video")
	for _, videoURL := range seedanceMetadataStringSlice(metadata, "reference_video_urls") {
		appendSeedanceVideoContent(payload, videoURL, "reference_video")
	}
}

func seedanceMetadataString(metadata map[string]interface{}, key string) string {
	if value, ok := metadata[key].(string); ok {
		return value
	}
	return ""
}

func seedanceMetadataStringSlice(metadata map[string]interface{}, key string) []string {
	value, ok := metadata[key]
	if !ok {
		return nil
	}
	switch typed := value.(type) {
	case []string:
		return typed
	case []interface{}:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			if raw, ok := item.(string); ok {
				result = append(result, raw)
			}
		}
		return result
	case string:
		if typed == "" {
			return nil
		}
		return []string{typed}
	default:
		return nil
	}
}

func (a *TaskAdaptor) EstimateBilling(c *gin.Context, info *relaycommon.RelayInfo) map[string]float64 {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil
	}
	modelName := req.Model
	if modelName == "" {
		modelName = info.OriginModelName
	}
	if !IsSeedance2Model(modelName) && !IsSeedance2Model(info.UpstreamModelName) {
		return nil
	}
	body, err := a.convertToRequestPayload(&req)
	if err != nil {
		return nil
	}

	ratios := map[string]float64{}
	defaultDuration := common.GetEnvOrDefault("SEEDANCE_DEFAULT_DURATION", 5)
	if defaultDuration <= 0 {
		defaultDuration = 5
	}
	if duration := int(body.Duration); duration > 0 && duration != defaultDuration {
		ratios["duration"] = float64(duration) / float64(defaultDuration)
	}
	if ratio, ok := seedanceResolutionRatio(body.Resolution); ok && ratio != 1 {
		ratios["resolution"] = ratio
	}
	if body.GenerateAudio != nil && bool(*body.GenerateAudio) {
		ratios["audio"] = 1.2
	}
	if len(ratios) == 0 {
		return nil
	}
	return ratios
}

func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	resTask := responseTask{}
	if err := common.Unmarshal(respBody, &resTask); err != nil {
		return nil, errors.Wrap(err, "unmarshal task result failed")
	}

	taskResult := relaycommon.TaskInfo{
		Code: 0,
	}

	// Map Doubao status to internal status
	switch resTask.Status {
	case "pending", "queued":
		taskResult.Status = model.TaskStatusQueued
		taskResult.Progress = "10%"
	case "processing", "running":
		taskResult.Status = model.TaskStatusInProgress
		taskResult.Progress = "50%"
	case "succeeded":
		taskResult.Status = model.TaskStatusSuccess
		taskResult.Progress = "100%"
		taskResult.Url = resTask.Content.VideoURL
		if IsSeedance2Model(resTask.Model) && common.GetEnvOrDefaultBool("SEEDANCE_BILLING_STRICT_USAGE", false) && resTask.Usage.TotalTokens <= 0 {
			taskResult.Status = model.TaskStatusFailure
			taskResult.Reason = "seedance usage missing"
			return &taskResult, nil
		}
		// 解析 usage 信息用于按倍率计费
		if shouldReportTaskUsage(resTask.Model) {
			taskResult.CompletionTokens = resTask.Usage.CompletionTokens
			taskResult.TotalTokens = resTask.Usage.TotalTokens
		}
	case "failed":
		taskResult.Status = model.TaskStatusFailure
		taskResult.Progress = "100%"
		taskResult.Reason = "task failed"
	default:
		// Unknown status, treat as processing
		taskResult.Status = model.TaskStatusInProgress
		taskResult.Progress = "30%"
	}

	return &taskResult, nil
}

func (a *TaskAdaptor) validateSeedance2Request(req *relaycommon.TaskSubmitReq) *dto.TaskError {
	for key := range req.Metadata {
		if !seedanceAllowedMetadata[key] {
			return service.TaskErrorWrapperLocal(fmt.Errorf("unsupported seedance metadata field: %s", key), "invalid_request", http.StatusBadRequest)
		}
	}
	body, err := a.convertToRequestPayload(req)
	if err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	maxDuration := common.GetEnvOrDefault("SEEDANCE_MAX_DURATION", 60)
	if maxDuration <= 0 {
		maxDuration = 60
	}
	if duration := int(body.Duration); duration < 0 || duration > maxDuration {
		return service.TaskErrorWrapperLocal(fmt.Errorf("duration must be between 1 and %d seconds", maxDuration), "invalid_duration", http.StatusBadRequest)
	}
	maxImages := common.GetEnvOrDefault("SEEDANCE_MAX_IMAGES", 8)
	if maxImages <= 0 {
		maxImages = 8
	}
	imageReferenceCount := countSeedanceContentItems(body, "image_url")
	if imageReferenceCount > maxImages {
		return service.TaskErrorWrapperLocal(fmt.Errorf("image references cannot exceed %d", maxImages), "invalid_images", http.StatusBadRequest)
	}
	maxReferenceVideos := common.GetEnvOrDefault("SEEDANCE_MAX_REFERENCE_VIDEOS", 3)
	if maxReferenceVideos <= 0 {
		maxReferenceVideos = 3
	}
	videoReferenceCount := countSeedanceContentItems(body, "video")
	if videoReferenceCount > maxReferenceVideos {
		return service.TaskErrorWrapperLocal(fmt.Errorf("reference videos cannot exceed %d", maxReferenceVideos), "invalid_videos", http.StatusBadRequest)
	}
	if body.Resolution != "" {
		if _, ok := seedanceResolutionRatio(body.Resolution); !ok {
			return service.TaskErrorWrapperLocal(fmt.Errorf("unsupported resolution: %s", body.Resolution), "invalid_resolution", http.StatusBadRequest)
		}
	}
	if body.Ratio != "" && !seedanceAllowedRatios[body.Ratio] {
		return service.TaskErrorWrapperLocal(fmt.Errorf("unsupported ratio: %s", body.Ratio), "invalid_ratio", http.StatusBadRequest)
	}
	if taskErr := validateSeedance2ExternalURLs(body); taskErr != nil {
		return taskErr
	}
	return nil
}

func validateSeedance2ExternalURLs(body *requestPayload) *dto.TaskError {
	if body.CallbackURL != "" {
		if err := validateSeedance2URL(body.CallbackURL); err != nil {
			return service.TaskErrorWrapperLocal(fmt.Errorf("unsafe callback_url: %w", err), "invalid_callback_url", http.StatusBadRequest)
		}
	}

	for index, item := range body.Content {
		if item.ImageURL != nil {
			if err := validateSeedance2URL(item.ImageURL.URL); err != nil {
				return service.TaskErrorWrapperLocal(fmt.Errorf("unsafe image URL at index %d: %w", index, err), "invalid_image_url", http.StatusBadRequest)
			}
		}
		if item.Video != nil {
			if err := validateSeedance2URL(item.Video.URL); err != nil {
				return service.TaskErrorWrapperLocal(fmt.Errorf("unsafe video URL at index %d: %w", index, err), "invalid_video_url", http.StatusBadRequest)
			}
		}
	}

	return nil
}

func validateSeedance2URL(rawURL string) error {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return fmt.Errorf("url is required")
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("unsupported URL scheme: %s", parsedURL.Scheme)
	}
	if parsedURL.Host == "" {
		return fmt.Errorf("URL host is required")
	}
	if parsedURL.User != nil {
		return fmt.Errorf("URL credentials are not allowed")
	}

	fetchSetting := system_setting.GetFetchSetting()
	if err := common.ValidateURLWithFetchSetting(rawURL, fetchSetting.EnableSSRFProtection, fetchSetting.AllowPrivateIp, fetchSetting.DomainFilterMode, fetchSetting.IpFilterMode, fetchSetting.DomainList, fetchSetting.IpList, fetchSetting.AllowedPorts, fetchSetting.ApplyIPFilterForDomain); err != nil {
		return err
	}
	return nil
}

func countSeedanceContentItems(body *requestPayload, itemType string) int {
	count := 0
	for _, item := range body.Content {
		if item.Type == itemType {
			count++
		}
	}
	return count
}

var seedanceAllowedMetadata = map[string]bool{
	"callback_url":            true,
	"return_last_frame":       true,
	"service_tier":            true,
	"execution_expires_after": true,
	"generate_audio":          true,
	"draft":                   true,
	"resolution":              true,
	"ratio":                   true,
	"duration":                true,
	"frames":                  true,
	"seed":                    true,
	"camera_fixed":            true,
	"watermark":               true,
	"reference_image_url":     true,
	"reference_image_urls":    true,
	"first_frame_url":         true,
	"last_frame_url":          true,
	"reference_video_url":     true,
	"reference_video_urls":    true,
}

var seedanceAllowedRatios = map[string]bool{
	"16:9": true,
	"9:16": true,
	"1:1":  true,
	"4:3":  true,
	"3:4":  true,
	"21:9": true,
}

func seedanceResolutionRatio(resolution string) (float64, bool) {
	switch strings.ToLower(strings.TrimSpace(resolution)) {
	case "":
		return 1, true
	case "480p":
		return 0.6, true
	case "720p":
		return 1, true
	case "1080p":
		return 1.6, true
	case "2k", "1440p":
		return 2.4, true
	case "4k", "2160p":
		return 3.2, true
	default:
		return 0, false
	}
}

func shouldReportTaskUsage(modelName string) bool {
	if !IsSeedance2Model(modelName) {
		return true
	}
	return common.GetEnvOrDefaultBool("SEEDANCE_BILLING_BY_USAGE", true)
}

func (a *TaskAdaptor) ConvertToOpenAIVideo(originTask *model.Task) ([]byte, error) {
	var dResp responseTask
	if err := common.Unmarshal(originTask.Data, &dResp); err != nil {
		return nil, errors.Wrap(err, "unmarshal doubao task data failed")
	}

	openAIVideo := dto.NewOpenAIVideo()
	openAIVideo.ID = originTask.TaskID
	openAIVideo.TaskID = originTask.TaskID
	openAIVideo.Status = originTask.Status.ToVideoStatus()
	openAIVideo.SetProgressStr(originTask.Progress)
	openAIVideo.SetMetadata("url", dResp.Content.VideoURL)
	openAIVideo.CreatedAt = originTask.CreatedAt
	openAIVideo.CompletedAt = originTask.UpdatedAt
	openAIVideo.Model = originTask.Properties.OriginModelName

	if dResp.Status == "failed" {
		openAIVideo.Error = &dto.OpenAIVideoError{
			Message: "task failed",
			Code:    "failed",
		}
	}

	return common.Marshal(openAIVideo)
}
