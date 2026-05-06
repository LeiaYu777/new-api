package service

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

func init() {
	ratio_setting.InitRatioSettings()
}

func TestBuildSeedanceBillingReadinessBlocksWhenPriceUnconfirmed(t *testing.T) {
	t.Setenv("SEEDANCE_REQUIRE_PRICE_CONFIRMATION", "true")
	t.Setenv("SEEDANCE_PRICE_CONFIRMED", "false")

	readiness := BuildSeedanceBillingReadiness(SeedanceBillingReadinessOptions{ModelName: "doubao-seedance-2-0"})
	if readiness.Status != SeedanceReadinessStatusBlocked {
		t.Fatalf("status = %q", readiness.Status)
	}
	if readiness.Ready {
		t.Fatalf("ready = true, expected false")
	}
	if !hasSeedanceReadinessCheck(readiness, "price_confirmation", SeedanceReadinessCheckFail) {
		t.Fatalf("missing failing price_confirmation check: %#v", readiness.Checks)
	}
}

func TestBuildSeedanceBillingReadinessWarningWhenGateDisabled(t *testing.T) {
	t.Setenv("SEEDANCE_REQUIRE_PRICE_CONFIRMATION", "false")
	t.Setenv("SEEDANCE_PRICE_CONFIRMED", "false")

	readiness := BuildSeedanceBillingReadiness(SeedanceBillingReadinessOptions{ModelName: "doubao-seedance-2-0"})
	if readiness.Status != SeedanceReadinessStatusWarning {
		t.Fatalf("status = %q", readiness.Status)
	}
	if !readiness.Ready {
		t.Fatalf("ready = false, expected true")
	}
	if readiness.ProductionReady {
		t.Fatalf("production_ready = true, expected false")
	}
}

func TestBuildSeedanceBillingReadinessBlocksLocalWorkerWhenRequired(t *testing.T) {
	t.Setenv("SEEDANCE_REQUIRE_PRICE_CONFIRMATION", "true")
	t.Setenv("SEEDANCE_PRICE_CONFIRMED", "true")
	t.Setenv("UPDATE_TASK", "false")

	readiness := BuildSeedanceBillingReadiness(SeedanceBillingReadinessOptions{ModelName: "doubao-seedance-2-0", RequireLocalWorker: true})
	if readiness.Status != SeedanceReadinessStatusBlocked {
		t.Fatalf("status = %q", readiness.Status)
	}
	if !hasSeedanceReadinessCheck(readiness, "task_worker", SeedanceReadinessCheckFail) {
		t.Fatalf("missing failing task_worker check: %#v", readiness.Checks)
	}
}

func TestBuildSeedanceBillingReadinessWarnsForStrictUsageWithoutConfirmation(t *testing.T) {
	t.Setenv("SEEDANCE_REQUIRE_PRICE_CONFIRMATION", "true")
	t.Setenv("SEEDANCE_PRICE_CONFIRMED", "true")
	t.Setenv("SEEDANCE_BILLING_STRICT_USAGE", "true")

	readiness := BuildSeedanceBillingReadiness(SeedanceBillingReadinessOptions{ModelName: "doubao-seedance-2-0"})
	if readiness.Status != SeedanceReadinessStatusWarning {
		t.Fatalf("status = %q", readiness.Status)
	}
	if !hasSeedanceReadinessCheck(readiness, "usage_strategy", SeedanceReadinessCheckWarn) {
		t.Fatalf("missing warning usage_strategy check: %#v", readiness.Checks)
	}
}

func TestBuildSeedanceBillingReadinessBlocksRemoteAllowlistWhenRequired(t *testing.T) {
	t.Setenv("SEEDANCE_REQUIRE_PRICE_CONFIRMATION", "true")
	t.Setenv("SEEDANCE_PRICE_CONFIRMED", "true")
	t.Setenv("SEEDANCE_REMOTE_URL_ALLOWLIST", "")

	readiness := BuildSeedanceBillingReadiness(SeedanceBillingReadinessOptions{
		ModelName:              "doubao-seedance-2-0",
		RequireRemoteAllowlist: true,
	})
	if readiness.Status != SeedanceReadinessStatusBlocked {
		t.Fatalf("status = %q", readiness.Status)
	}
	if !hasSeedanceReadinessCheck(readiness, "seedance_remote_url_allowlist", SeedanceReadinessCheckFail) {
		t.Fatalf("missing failing seedance_remote_url_allowlist check: %#v", readiness.Checks)
	}
}

func TestBuildSeedanceBillingReadinessBlocksCallbackAllowlistWhenRequired(t *testing.T) {
	t.Setenv("SEEDANCE_REQUIRE_PRICE_CONFIRMATION", "true")
	t.Setenv("SEEDANCE_PRICE_CONFIRMED", "true")
	t.Setenv("SEEDANCE_CALLBACK_URL_ALLOWLIST", "")

	readiness := BuildSeedanceBillingReadiness(SeedanceBillingReadinessOptions{
		ModelName:                "doubao-seedance-2-0",
		RequireCallbackAllowlist: true,
	})
	if readiness.Status != SeedanceReadinessStatusBlocked {
		t.Fatalf("status = %q", readiness.Status)
	}
	if !hasSeedanceReadinessCheck(readiness, "seedance_callback_url_allowlist", SeedanceReadinessCheckFail) {
		t.Fatalf("missing failing seedance_callback_url_allowlist check: %#v", readiness.Checks)
	}
}

func hasSeedanceReadinessCheck(readiness SeedanceBillingReadiness, key string, status string) bool {
	for _, check := range readiness.Checks {
		if check.Key == key && check.Status == status {
			return true
		}
	}
	return false
}
