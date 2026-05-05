package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"

	"github.com/bytedance/gopkg/util/gopool"
)

const billingStatementAutoDefaultModelFilter = "doubao-seedance-2-0%"

var (
	billingStatementAutoOnce            sync.Once
	billingStatementAutoRunning         atomic.Bool
	billingStatementAutoLastPeriodStart atomic.Int64
)

func StartBillingStatementAutoTask() {
	billingStatementAutoOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		if !common.GetEnvOrDefaultBool("BILLING_STATEMENT_AUTO_ENABLED", false) {
			common.SysLog("billing statement auto task disabled by BILLING_STATEMENT_AUTO_ENABLED")
			return
		}

		intervalMinutes := common.GetEnvOrDefault("BILLING_STATEMENT_AUTO_CHECK_INTERVAL_MINUTES", 60)
		if intervalMinutes <= 0 {
			intervalMinutes = 60
		}
		tickInterval := time.Duration(intervalMinutes) * time.Minute

		gopool.Go(func() {
			ctx := context.Background()
			logger.LogInfo(ctx, fmt.Sprintf("billing statement auto task started: tick=%s", tickInterval))
			ticker := time.NewTicker(tickInterval)
			defer ticker.Stop()

			runBillingStatementAutoTaskOnce(time.Now())
			for now := range ticker.C {
				runBillingStatementAutoTaskOnce(now)
			}
		})
	})
}

func runBillingStatementAutoTaskOnce(now time.Time) {
	loc := billingStatementAutoLocation()
	localNow := now.In(loc)
	if !billingStatementAutoDue(localNow) {
		return
	}

	periodStart, periodEnd := billingStatementPreviousMonthPeriod(localNow)
	if billingStatementAutoLastPeriodStart.Load() == periodStart {
		return
	}
	if !billingStatementAutoRunning.CompareAndSwap(false, true) {
		return
	}
	defer billingStatementAutoRunning.Store(false)

	result, err := runBillingStatementAutoJob(localNow)
	if err != nil {
		logger.LogWarn(context.Background(), fmt.Sprintf("billing statement auto task failed: %v", err))
		return
	}
	billingStatementAutoLastPeriodStart.Store(periodStart)
	logger.LogInfo(
		context.Background(),
		fmt.Sprintf(
			"billing statement auto task generated: period_start=%d, period_end=%d, generated_count=%d",
			periodStart,
			periodEnd,
			result.GeneratedCount,
		),
	)
}

func runBillingStatementAutoJob(now time.Time) (*model.BillingStatementGenerateResult, error) {
	periodStart, periodEnd := billingStatementPreviousMonthPeriod(now)
	modelFilter := strings.TrimSpace(common.GetEnvOrDefaultString("BILLING_STATEMENT_AUTO_MODEL_FILTER", billingStatementAutoDefaultModelFilter))
	billingSource := strings.TrimSpace(common.GetEnvOrDefaultString("BILLING_STATEMENT_AUTO_BILLING_SOURCE", ""))
	return model.GenerateBillingStatements(model.BillingStatementFilter{
		PeriodStart:   periodStart,
		PeriodEnd:     periodEnd,
		ModelName:     modelFilter,
		BillingSource: billingSource,
	})
}

func billingStatementAutoLocation() *time.Location {
	timezone := strings.TrimSpace(common.GetEnvOrDefaultString("BILLING_STATEMENT_AUTO_TIMEZONE", ""))
	if timezone == "" {
		return time.Local
	}
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		common.SysLog(fmt.Sprintf("invalid BILLING_STATEMENT_AUTO_TIMEZONE=%s: %v, fallback to local timezone", timezone, err))
		return time.Local
	}
	return loc
}

func billingStatementAutoDay() int {
	day := common.GetEnvOrDefault("BILLING_STATEMENT_AUTO_DAY", 1)
	if day < 1 {
		return 1
	}
	if day > 28 {
		return 28
	}
	return day
}

func billingStatementAutoHour() int {
	hour := common.GetEnvOrDefault("BILLING_STATEMENT_AUTO_HOUR", 2)
	if hour < 0 || hour > 23 {
		return 2
	}
	return hour
}

func billingStatementAutoDue(now time.Time) bool {
	localNow := now.In(billingStatementAutoLocation())
	return localNow.Day() == billingStatementAutoDay() && localNow.Hour() >= billingStatementAutoHour()
}

func billingStatementPreviousMonthPeriod(now time.Time) (int64, int64) {
	localNow := now.In(now.Location())
	currentMonthStart := time.Date(localNow.Year(), localNow.Month(), 1, 0, 0, 0, 0, localNow.Location())
	previousMonthStart := currentMonthStart.AddDate(0, -1, 0)
	previousMonthEnd := currentMonthStart.Add(-time.Second)
	return previousMonthStart.Unix(), previousMonthEnd.Unix()
}
