package dto

import (
	"encoding/json"
)

type TaskError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	Data       any    `json:"data"`
	StatusCode int    `json:"-"`
	LocalError bool   `json:"-"`
	Error      error  `json:"-"`
}

type TaskData interface {
	SunoDataResponse | []SunoDataResponse | string | any
}

const TaskSuccessCode = "success"

type TaskResponse[T TaskData] struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

func (t *TaskResponse[T]) IsSuccess() bool {
	return t.Code == TaskSuccessCode
}

type TaskDto struct {
	ID         int64           `json:"id"`
	CreatedAt  int64           `json:"created_at"`
	UpdatedAt  int64           `json:"updated_at"`
	TaskID     string          `json:"task_id"`
	Platform   string          `json:"platform"`
	UserId     int             `json:"user_id"`
	Group      string          `json:"group"`
	ChannelId  int             `json:"channel_id"`
	Quota      int             `json:"quota"`
	Action     string          `json:"action"`
	Status     string          `json:"status"`
	FailReason string          `json:"fail_reason"`
	ResultURL  string          `json:"result_url,omitempty"` // 任务结果 URL（视频地址等）
	SubmitTime int64           `json:"submit_time"`
	StartTime  int64           `json:"start_time"`
	FinishTime int64           `json:"finish_time"`
	Progress   string          `json:"progress"`
	Properties any             `json:"properties"`
	Username   string          `json:"username,omitempty"`
	Data       json.RawMessage `json:"data"`
	Billing    *TaskBillingDto `json:"billing,omitempty"`
}

type TaskBillingDto struct {
	BillingSource     string             `json:"billing_source,omitempty"`
	SubscriptionId    int                `json:"subscription_id,omitempty"`
	TokenId           int                `json:"token_id,omitempty"`
	ModelName         string             `json:"model_name,omitempty"`
	SettlementStatus  string             `json:"settlement_status,omitempty"`
	PreConsumedQuota  int                `json:"pre_consumed_quota,omitempty"`
	ActualQuota       int                `json:"actual_quota,omitempty"`
	RefundQuota       int                `json:"refund_quota,omitempty"`
	PerCallBilling    bool               `json:"per_call_billing,omitempty"`
	ModelPrice        float64            `json:"model_price,omitempty"`
	GroupRatio        float64            `json:"group_ratio,omitempty"`
	ModelRatio        float64            `json:"model_ratio,omitempty"`
	OtherRatios       map[string]float64 `json:"other_ratios,omitempty"`
	UpstreamModelName string             `json:"upstream_model_name,omitempty"`
}

type FetchReq struct {
	IDs []string `json:"ids"`
}
