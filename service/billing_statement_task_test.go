package service

import (
	"testing"
	"time"
)

func TestBillingStatementPreviousMonthPeriod(t *testing.T) {
	loc := time.FixedZone("CST", 8*3600)
	now := time.Date(2026, time.May, 6, 3, 30, 0, 0, loc)

	start, end := billingStatementPreviousMonthPeriod(now)

	assertUnixTime(t, start, time.Date(2026, time.April, 1, 0, 0, 0, 0, loc))
	assertUnixTime(t, end, time.Date(2026, time.April, 30, 23, 59, 59, 0, loc))
}

func TestBillingStatementPreviousMonthPeriodCrossYear(t *testing.T) {
	loc := time.FixedZone("CST", 8*3600)
	now := time.Date(2026, time.January, 2, 3, 30, 0, 0, loc)

	start, end := billingStatementPreviousMonthPeriod(now)

	assertUnixTime(t, start, time.Date(2025, time.December, 1, 0, 0, 0, 0, loc))
	assertUnixTime(t, end, time.Date(2025, time.December, 31, 23, 59, 59, 0, loc))
}

func TestBillingStatementAutoDueUsesConfiguredDayAndHour(t *testing.T) {
	loc := time.FixedZone("CST", 8*3600)
	oldLocal := time.Local
	time.Local = loc
	t.Cleanup(func() {
		time.Local = oldLocal
	})
	t.Setenv("BILLING_STATEMENT_AUTO_TIMEZONE", "")
	t.Setenv("BILLING_STATEMENT_AUTO_DAY", "3")
	t.Setenv("BILLING_STATEMENT_AUTO_HOUR", "2")

	tests := []struct {
		name string
		now  time.Time
		want bool
	}{
		{
			name: "configured hour is due",
			now:  time.Date(2026, time.May, 3, 2, 30, 0, 0, loc),
			want: true,
		},
		{
			name: "later same day is still due",
			now:  time.Date(2026, time.May, 3, 12, 0, 0, 0, loc),
			want: true,
		},
		{
			name: "before configured hour is not due",
			now:  time.Date(2026, time.May, 3, 1, 59, 0, 0, loc),
			want: false,
		},
		{
			name: "different day is not due",
			now:  time.Date(2026, time.May, 2, 2, 30, 0, 0, loc),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := billingStatementAutoDue(tt.now); got != tt.want {
				t.Fatalf("billingStatementAutoDue() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBillingStatementAutoConfigClampsUnsafeValues(t *testing.T) {
	t.Setenv("BILLING_STATEMENT_AUTO_DAY", "31")
	t.Setenv("BILLING_STATEMENT_AUTO_HOUR", "24")

	if got := billingStatementAutoDay(); got != 28 {
		t.Fatalf("billingStatementAutoDay() = %d, want 28", got)
	}
	if got := billingStatementAutoHour(); got != 2 {
		t.Fatalf("billingStatementAutoHour() = %d, want 2", got)
	}
}

func assertUnixTime(t *testing.T, got int64, want time.Time) {
	t.Helper()
	if got != want.Unix() {
		t.Fatalf("unix time = %d (%s), want %d (%s)", got, time.Unix(got, 0).In(want.Location()), want.Unix(), want)
	}
}
