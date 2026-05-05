package controller

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/model"
)

func TestBillingCSVCellEscapesSpreadsheetFormula(t *testing.T) {
	tests := map[string]string{
		"":               "",
		"normal content": "normal content",
		"=1+1":           "'=1+1",
		"+SUM(A1:A2)":    "'+SUM(A1:A2)",
		"-10":            "'-10",
		"@cmd":           "'@cmd",
	}

	for input, want := range tests {
		if got := billingCSVCell(input); got != want {
			t.Fatalf("billingCSVCell(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestFormatBillingPrometheusMetrics(t *testing.T) {
	metrics := &model.BillingAlertMetrics{
		WindowStart:              100,
		WindowEnd:                200,
		ConsumeQuota:             1000,
		RefundQuota:              250,
		NetQuota:                 750,
		RequestCount:             2,
		RefundCount:              1,
		RefundRate:               0.5,
		TaskSuccessCount:         1,
		TaskFailureCount:         1,
		TaskFailureRate:          0.5,
		PendingTaskCount:         3,
		TimedOutTaskCount:        1,
		WorkerLagSeconds:         900,
		InsufficientBalanceCount: 1,
		UpstreamErrorCount:       1,
		Alerts: []model.BillingAlertItem{
			{Key: "worker_lag", Severity: model.BillingAlertSeverityCritical},
		},
	}
	filter := model.BillingAlertFilter{
		ModelName:     `doubao-seedance-2-0"prod`,
		BillingSource: "wallet",
		Group:         "vip",
		Channel:       45,
		UserId:        7,
	}

	output := formatBillingPrometheusMetrics(metrics, filter)
	required := []string{
		"# TYPE newapi_billing_net_quota gauge",
		`newapi_billing_net_quota{model_name="doubao-seedance-2-0\"prod",billing_source="wallet",group="vip",channel_id="45",user_id="7"} 750`,
		`newapi_billing_refund_rate{model_name="doubao-seedance-2-0\"prod",billing_source="wallet",group="vip",channel_id="45",user_id="7"} 0.5`,
		`newapi_billing_alert_active{model_name="doubao-seedance-2-0\"prod",billing_source="wallet",group="vip",channel_id="45",user_id="7",alert_key="worker_lag",severity="critical"} 1`,
	}
	for _, expected := range required {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected prometheus output to contain %q, got:\n%s", expected, output)
		}
	}
}
