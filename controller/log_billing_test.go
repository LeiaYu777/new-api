package controller

import "testing"

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
