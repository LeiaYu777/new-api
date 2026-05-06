package relay

import (
	"net/http"
	"testing"
)

func TestValidateSeedanceTaskPriceConfirmationRejectsUnconfirmedMappedModel(t *testing.T) {
	t.Setenv("SEEDANCE_REQUIRE_PRICE_CONFIRMATION", "true")
	t.Setenv("SEEDANCE_PRICE_CONFIRMED", "false")

	taskErr := validateSeedanceTaskPriceConfirmation("customer-video-alias", "doubao-seedance-2-0")
	if taskErr == nil {
		t.Fatalf("validateSeedanceTaskPriceConfirmation() expected error")
	}
	if taskErr.Code != "seedance_price_not_confirmed" {
		t.Fatalf("error code = %q", taskErr.Code)
	}
	if taskErr.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status code = %d", taskErr.StatusCode)
	}
}

func TestValidateSeedanceTaskPriceConfirmationAllowsConfirmedModel(t *testing.T) {
	t.Setenv("SEEDANCE_REQUIRE_PRICE_CONFIRMATION", "true")
	t.Setenv("SEEDANCE_PRICE_CONFIRMED", "true")

	if taskErr := validateSeedanceTaskPriceConfirmation("doubao-seedance-2-0"); taskErr != nil {
		t.Fatalf("validateSeedanceTaskPriceConfirmation() unexpected error = %v", taskErr)
	}
}

func TestValidateSeedanceTaskPriceConfirmationIgnoresNonSeedanceModel(t *testing.T) {
	t.Setenv("SEEDANCE_REQUIRE_PRICE_CONFIRMATION", "true")
	t.Setenv("SEEDANCE_PRICE_CONFIRMED", "false")

	if taskErr := validateSeedanceTaskPriceConfirmation("gpt-4o-mini"); taskErr != nil {
		t.Fatalf("validateSeedanceTaskPriceConfirmation() unexpected error = %v", taskErr)
	}
}
