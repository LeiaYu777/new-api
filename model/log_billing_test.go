package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupLogBillingTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	oldDB := DB
	oldLOGDB := LOG_DB
	oldUsingSQLite := common.UsingSQLite
	oldUsingMySQL := common.UsingMySQL
	oldUsingPostgreSQL := common.UsingPostgreSQL
	oldLogSqlType := common.LogSqlType
	oldLogGroupCol := logGroupCol

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.LogSqlType = common.DatabaseTypeSQLite
	logGroupCol = `"group"`

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	LOG_DB = db
	DB = db
	if err := db.AutoMigrate(&Log{}, &BillingStatement{}, &Task{}); err != nil {
		t.Fatalf("failed to migrate logs: %v", err)
	}
	t.Cleanup(func() {
		DB = oldDB
		LOG_DB = oldLOGDB
		common.UsingSQLite = oldUsingSQLite
		common.UsingMySQL = oldUsingMySQL
		common.UsingPostgreSQL = oldUsingPostgreSQL
		common.LogSqlType = oldLogSqlType
		logGroupCol = oldLogGroupCol
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func TestGetBillingSummaryAggregatesConsumeAndRefund(t *testing.T) {
	db := setupLogBillingTestDB(t)
	logs := []*Log{
		{UserId: 1, Username: "alice", CreatedAt: 10, Type: LogTypeConsume, ModelName: "doubao-seedance-2-0", Quota: 1000, ChannelId: 45, TokenId: 1, Group: "vip", PromptTokens: 10, CompletionTokens: 20},
		{UserId: 1, Username: "alice", CreatedAt: 20, Type: LogTypeRefund, ModelName: "doubao-seedance-2-0", Quota: 200, ChannelId: 45, TokenId: 1, Group: "vip"},
		{UserId: 2, Username: "bob", CreatedAt: 30, Type: LogTypeConsume, ModelName: "other-model", Quota: 500, ChannelId: 46, TokenId: 2, Group: "default"},
	}
	if err := db.Create(&logs).Error; err != nil {
		t.Fatalf("failed to seed logs: %v", err)
	}

	summary, err := GetBillingSummary(0, 0, "doubao-seedance-2-0", "", 0, 0, "", "", "", 100)
	if err != nil {
		t.Fatalf("GetBillingSummary() error = %v", err)
	}
	if len(summary.Items) != 1 {
		t.Fatalf("items len = %d", len(summary.Items))
	}
	item := summary.Items[0]
	if item.UserId != 1 || item.Username != "alice" || item.ModelName != "doubao-seedance-2-0" {
		t.Fatalf("unexpected item: %+v", item)
	}
	if item.BillingSource != billingSourceWallet {
		t.Fatalf("billing source = %s", item.BillingSource)
	}
	if item.ConsumeQuota != 1000 {
		t.Fatalf("consume quota = %d", item.ConsumeQuota)
	}
	if item.RefundQuota != 200 {
		t.Fatalf("refund quota = %d", item.RefundQuota)
	}
	if item.NetQuota != 800 {
		t.Fatalf("net quota = %d", item.NetQuota)
	}
	if item.RequestCount != 1 || item.RefundCount != 1 {
		t.Fatalf("counts = %d/%d", item.RequestCount, item.RefundCount)
	}
	if item.TotalTokens != 30 {
		t.Fatalf("total tokens = %d", item.TotalTokens)
	}
	if summary.NetQuota != 800 || summary.ConsumeQuota != 1000 || summary.RefundQuota != 200 {
		t.Fatalf("summary totals = %+v", summary)
	}
}

func TestGetBillingSummaryFiltersByUserAndChannel(t *testing.T) {
	db := setupLogBillingTestDB(t)
	logs := []*Log{
		{UserId: 1, Username: "alice", CreatedAt: 10, Type: LogTypeConsume, ModelName: "doubao-seedance-2-0", Quota: 1000, ChannelId: 45, Group: "vip"},
		{UserId: 1, Username: "alice", CreatedAt: 20, Type: LogTypeConsume, ModelName: "doubao-seedance-2-0", Quota: 300, ChannelId: 46, Group: "vip"},
		{UserId: 2, Username: "bob", CreatedAt: 30, Type: LogTypeConsume, ModelName: "doubao-seedance-2-0", Quota: 500, ChannelId: 45, Group: "vip"},
	}
	if err := db.Create(&logs).Error; err != nil {
		t.Fatalf("failed to seed logs: %v", err)
	}

	summary, err := GetBillingSummary(0, 0, "", "", 1, 45, "vip", "", "", 100)
	if err != nil {
		t.Fatalf("GetBillingSummary() error = %v", err)
	}
	if len(summary.Items) != 1 {
		t.Fatalf("items len = %d", len(summary.Items))
	}
	if summary.Items[0].NetQuota != 1000 {
		t.Fatalf("net quota = %d", summary.Items[0].NetQuota)
	}
}

func TestGetBillingSummaryGroupsAndFiltersByBillingSource(t *testing.T) {
	db := setupLogBillingTestDB(t)
	logs := []*Log{
		{UserId: 1, Username: "alice", CreatedAt: 10, Type: LogTypeConsume, ModelName: "doubao-seedance-2-0", Quota: 1000, ChannelId: 45, Group: "vip", Other: common.MapToJsonStr(map[string]interface{}{"billing_source": billingSourceWallet})},
		{UserId: 1, Username: "alice", CreatedAt: 20, Type: LogTypeConsume, ModelName: "doubao-seedance-2-0", Quota: 400, ChannelId: 45, Group: "vip", Other: common.MapToJsonStr(map[string]interface{}{"billing_source": billingSourceSubscription})},
		{UserId: 1, Username: "alice", CreatedAt: 30, Type: LogTypeRefund, ModelName: "doubao-seedance-2-0", Quota: 100, ChannelId: 45, Group: "vip", Other: common.MapToJsonStr(map[string]interface{}{"billing_source": billingSourceSubscription})},
	}
	if err := db.Create(&logs).Error; err != nil {
		t.Fatalf("failed to seed logs: %v", err)
	}

	summary, err := GetBillingSummary(0, 0, "", "", 1, 45, "vip", "", "", 100)
	if err != nil {
		t.Fatalf("GetBillingSummary() error = %v", err)
	}
	if len(summary.Items) != 2 {
		t.Fatalf("items len = %d", len(summary.Items))
	}
	netBySource := map[string]int64{}
	for _, item := range summary.Items {
		netBySource[item.BillingSource] = item.NetQuota
	}
	if netBySource[billingSourceWallet] != 1000 {
		t.Fatalf("wallet net quota = %d", netBySource[billingSourceWallet])
	}
	if netBySource[billingSourceSubscription] != 300 {
		t.Fatalf("subscription net quota = %d", netBySource[billingSourceSubscription])
	}

	filtered, err := GetBillingSummary(0, 0, "", "", 1, 45, "vip", "", billingSourceSubscription, 100)
	if err != nil {
		t.Fatalf("GetBillingSummary(subscription) error = %v", err)
	}
	if len(filtered.Items) != 1 {
		t.Fatalf("filtered items len = %d", len(filtered.Items))
	}
	if filtered.Items[0].BillingSource != billingSourceSubscription || filtered.NetQuota != 300 {
		t.Fatalf("filtered summary = %+v item=%+v", filtered, filtered.Items[0])
	}
}

func TestGetBillingExportLogsSanitizesModelFilter(t *testing.T) {
	db := setupLogBillingTestDB(t)
	logs := []*Log{
		{UserId: 1, Username: "alice", CreatedAt: 10, Type: LogTypeConsume, ModelName: "doubao-seedance-2-0", Quota: 1000, ChannelId: 45, Group: "vip"},
		{UserId: 2, Username: "bob", CreatedAt: 20, Type: LogTypeConsume, ModelName: "other-model", Quota: 500, ChannelId: 46, Group: "default"},
	}
	if err := db.Create(&logs).Error; err != nil {
		t.Fatalf("failed to seed logs: %v", err)
	}

	exported, err := GetBillingExportLogs(LogTypeUnknown, 0, 0, "doubao-seedance-2-0", "", 0, "", 0, "", "", "", "", 100)
	if err != nil {
		t.Fatalf("GetBillingExportLogs() error = %v", err)
	}
	if len(exported) != 1 || exported[0].ModelName != "doubao-seedance-2-0" {
		t.Fatalf("unexpected exported logs: %+v", exported)
	}

	if _, err := GetBillingExportLogs(LogTypeUnknown, 0, 0, "%%%%", "", 0, "", 0, "", "", "", "", 100); err == nil {
		t.Fatalf("expected invalid wildcard pattern error")
	}
}

func TestGetBillingExportLogsFiltersByBillingSource(t *testing.T) {
	db := setupLogBillingTestDB(t)
	logs := []*Log{
		{UserId: 1, Username: "alice", CreatedAt: 10, Type: LogTypeConsume, ModelName: "doubao-seedance-2-0", Quota: 1000, ChannelId: 45, Group: "vip", Other: common.MapToJsonStr(map[string]interface{}{"billing_source": billingSourceWallet})},
		{UserId: 1, Username: "alice", CreatedAt: 20, Type: LogTypeConsume, ModelName: "doubao-seedance-2-0", Quota: 400, ChannelId: 45, Group: "vip", Other: common.MapToJsonStr(map[string]interface{}{"billing_source": billingSourceSubscription})},
	}
	if err := db.Create(&logs).Error; err != nil {
		t.Fatalf("failed to seed logs: %v", err)
	}

	exported, err := GetBillingExportLogs(LogTypeConsume, 0, 0, "doubao-seedance-2-0", "", 0, "", 0, "", "", "", billingSourceSubscription, 100)
	if err != nil {
		t.Fatalf("GetBillingExportLogs(subscription) error = %v", err)
	}
	if len(exported) != 1 || exported[0].Quota != 400 {
		t.Fatalf("unexpected exported logs: %+v", exported)
	}

	if _, err := GetBillingExportLogs(LogTypeConsume, 0, 0, "", "", 0, "", 0, "", "", "", "unknown", 100); err == nil {
		t.Fatalf("expected unsupported billing source error")
	}
}

func TestGetUserBillingExportLogsForcesUserScope(t *testing.T) {
	db := setupLogBillingTestDB(t)
	logs := []*Log{
		{UserId: 1, Username: "alice", CreatedAt: 10, Type: LogTypeConsume, ModelName: "doubao-seedance-2-0", Quota: 1000, ChannelId: 45, Group: "vip", Other: common.MapToJsonStr(map[string]interface{}{"task_id": "task_alice", "billing_source": billingSourceWallet})},
		{UserId: 2, Username: "bob", CreatedAt: 20, Type: LogTypeConsume, ModelName: "doubao-seedance-2-0", Quota: 9000, ChannelId: 45, Group: "vip", Other: common.MapToJsonStr(map[string]interface{}{"task_id": "task_bob", "billing_source": billingSourceWallet})},
	}
	if err := db.Create(&logs).Error; err != nil {
		t.Fatalf("failed to seed logs: %v", err)
	}

	exported, err := GetUserBillingExportLogs(1, LogTypeUnknown, 0, 0, "doubao-seedance-2-0", "", 45, "vip", "", "", "", 100)
	if err != nil {
		t.Fatalf("GetUserBillingExportLogs() error = %v", err)
	}
	if len(exported) != 1 {
		t.Fatalf("exported len = %d, logs = %+v", len(exported), exported)
	}
	if exported[0].UserId != 1 || exported[0].Username != "alice" || exported[0].Quota != 1000 {
		t.Fatalf("unexpected exported log: %+v", exported[0])
	}
}

func TestGetBillingSummaryFiltersByTaskID(t *testing.T) {
	db := setupLogBillingTestDB(t)
	logs := []*Log{
		{UserId: 1, Username: "alice", CreatedAt: 10, Type: LogTypeConsume, ModelName: "doubao-seedance-2-0", Quota: 1000, ChannelId: 45, Group: "vip", Other: common.MapToJsonStr(map[string]interface{}{"task_id": "task_seedance_a", "billing_source": billingSourceWallet})},
		{UserId: 1, Username: "alice", CreatedAt: 20, Type: LogTypeRefund, ModelName: "doubao-seedance-2-0", Quota: 250, ChannelId: 45, Group: "vip", Other: common.MapToJsonStr(map[string]interface{}{"task_id": "task_seedance_a", "billing_source": billingSourceWallet})},
		{UserId: 1, Username: "alice", CreatedAt: 30, Type: LogTypeConsume, ModelName: "doubao-seedance-2-0", Quota: 400, ChannelId: 45, Group: "vip", Other: common.MapToJsonStr(map[string]interface{}{"task_id": "task_seedance_b", "billing_source": billingSourceWallet})},
	}
	if err := db.Create(&logs).Error; err != nil {
		t.Fatalf("failed to seed logs: %v", err)
	}

	summary, err := GetBillingSummary(0, 0, "", "", 1, 45, "vip", "task_seedance_a", "", 100)
	if err != nil {
		t.Fatalf("GetBillingSummary(task_id) error = %v", err)
	}
	if len(summary.Items) != 1 {
		t.Fatalf("items len = %d", len(summary.Items))
	}
	if summary.ConsumeQuota != 1000 || summary.RefundQuota != 250 || summary.NetQuota != 750 {
		t.Fatalf("summary totals = %+v", summary)
	}
	if summary.RequestCount != 1 || summary.RefundCount != 1 {
		t.Fatalf("summary counts = %+v", summary)
	}
}

func TestGetUserBillingSummaryForcesUserScope(t *testing.T) {
	db := setupLogBillingTestDB(t)
	logs := []*Log{
		{UserId: 1, Username: "alice", CreatedAt: 10, Type: LogTypeConsume, ModelName: "doubao-seedance-2-0", Quota: 1000, ChannelId: 45, Group: "vip", Other: common.MapToJsonStr(map[string]interface{}{"billing_source": billingSourceWallet})},
		{UserId: 1, Username: "alice", CreatedAt: 20, Type: LogTypeRefund, ModelName: "doubao-seedance-2-0", Quota: 250, ChannelId: 45, Group: "vip", Other: common.MapToJsonStr(map[string]interface{}{"billing_source": billingSourceWallet})},
		{UserId: 2, Username: "bob", CreatedAt: 30, Type: LogTypeConsume, ModelName: "doubao-seedance-2-0", Quota: 9000, ChannelId: 45, Group: "vip", Other: common.MapToJsonStr(map[string]interface{}{"billing_source": billingSourceWallet})},
	}
	if err := db.Create(&logs).Error; err != nil {
		t.Fatalf("failed to seed logs: %v", err)
	}

	summary, err := GetUserBillingSummary(1, 0, 0, "doubao-seedance-2-0", 45, "vip", "", "", 100)
	if err != nil {
		t.Fatalf("GetUserBillingSummary() error = %v", err)
	}
	if len(summary.Items) != 1 {
		t.Fatalf("items len = %d", len(summary.Items))
	}
	if summary.Items[0].UserId != 1 || summary.Items[0].Username != "alice" {
		t.Fatalf("unexpected summary item: %+v", summary.Items[0])
	}
	if summary.ConsumeQuota != 1000 || summary.RefundQuota != 250 || summary.NetQuota != 750 {
		t.Fatalf("summary totals = %+v", summary)
	}
}

func TestGetBillingExportLogsFiltersByTaskID(t *testing.T) {
	db := setupLogBillingTestDB(t)
	logs := []*Log{
		{UserId: 1, Username: "alice", CreatedAt: 10, Type: LogTypeConsume, ModelName: "doubao-seedance-2-0", Quota: 1000, ChannelId: 45, Group: "vip", Other: common.MapToJsonStr(map[string]interface{}{"task_id": "task_seedance_a", "billing_source": billingSourceWallet})},
		{UserId: 1, Username: "alice", CreatedAt: 20, Type: LogTypeRefund, ModelName: "doubao-seedance-2-0", Quota: 250, ChannelId: 45, Group: "vip", Other: common.MapToJsonStr(map[string]interface{}{"task_id": "task_seedance_a", "billing_source": billingSourceWallet})},
		{UserId: 1, Username: "alice", CreatedAt: 30, Type: LogTypeConsume, ModelName: "doubao-seedance-2-0", Quota: 400, ChannelId: 45, Group: "vip", Other: common.MapToJsonStr(map[string]interface{}{"task_id": "task_seedance_b", "billing_source": billingSourceWallet})},
	}
	if err := db.Create(&logs).Error; err != nil {
		t.Fatalf("failed to seed logs: %v", err)
	}

	exported, err := GetBillingExportLogs(LogTypeUnknown, 0, 0, "doubao-seedance-2-0", "", 0, "", 0, "", "", "task_seedance_a", "", 100)
	if err != nil {
		t.Fatalf("GetBillingExportLogs(task_id) error = %v", err)
	}
	if len(exported) != 2 {
		t.Fatalf("exported len = %d", len(exported))
	}
	for _, log := range exported {
		other, _ := common.StrToMap(log.Other)
		if fmt.Sprint(other["task_id"]) != "task_seedance_a" {
			t.Fatalf("unexpected exported task_id: %+v", exported)
		}
	}
}

func TestGenerateBillingStatementsCreatesSourceSnapshots(t *testing.T) {
	db := setupLogBillingTestDB(t)
	logs := []*Log{
		{UserId: 1, Username: "alice", CreatedAt: 10, Type: LogTypeConsume, ModelName: "doubao-seedance-2-0", Quota: 1000, ChannelId: 45, Group: "vip", Other: common.MapToJsonStr(map[string]interface{}{"billing_source": billingSourceWallet})},
		{UserId: 1, Username: "alice", CreatedAt: 20, Type: LogTypeConsume, ModelName: "doubao-seedance-2-0", Quota: 400, ChannelId: 45, Group: "vip", Other: common.MapToJsonStr(map[string]interface{}{"billing_source": billingSourceSubscription})},
		{UserId: 1, Username: "alice", CreatedAt: 30, Type: LogTypeRefund, ModelName: "doubao-seedance-2-0", Quota: 100, ChannelId: 45, Group: "vip", Other: common.MapToJsonStr(map[string]interface{}{"billing_source": billingSourceSubscription})},
	}
	if err := db.Create(&logs).Error; err != nil {
		t.Fatalf("failed to seed logs: %v", err)
	}

	result, err := GenerateBillingStatements(BillingStatementFilter{
		PeriodStart: 1,
		PeriodEnd:   40,
		ModelName:   "doubao-seedance-2-0",
		UserId:      1,
	})
	if err != nil {
		t.Fatalf("GenerateBillingStatements() error = %v", err)
	}
	if result.GeneratedCount != 2 {
		t.Fatalf("generated count = %d", result.GeneratedCount)
	}

	statements, total, err := GetBillingStatements(BillingStatementFilter{
		PeriodStart: 1,
		PeriodEnd:   40,
		UserId:      1,
	}, 0, 100)
	if err != nil {
		t.Fatalf("GetBillingStatements() error = %v", err)
	}
	if total != 2 || len(statements) != 2 {
		t.Fatalf("statements total/len = %d/%d", total, len(statements))
	}
	netBySource := map[string]int64{}
	for _, statement := range statements {
		netBySource[statement.BillingSource] = statement.NetQuota
	}
	if netBySource[billingSourceWallet] != 1000 {
		t.Fatalf("wallet net quota = %d", netBySource[billingSourceWallet])
	}
	if netBySource[billingSourceSubscription] != 300 {
		t.Fatalf("subscription net quota = %d", netBySource[billingSourceSubscription])
	}
}

func TestGetUserBillingStatementsForcesUserScope(t *testing.T) {
	db := setupLogBillingTestDB(t)
	statements := []*BillingStatement{
		{UserId: 1, Username: "alice", PeriodStart: 100, PeriodEnd: 200, ModelName: "doubao-seedance-2-0", ChannelId: 45, Group: "vip", BillingSource: billingSourceWallet, NetQuota: 750, ConsumeQuota: 1000, RefundQuota: 250},
		{UserId: 2, Username: "bob", PeriodStart: 100, PeriodEnd: 200, ModelName: "doubao-seedance-2-0", ChannelId: 45, Group: "vip", BillingSource: billingSourceWallet, NetQuota: 9000, ConsumeQuota: 9000},
	}
	if err := db.Create(&statements).Error; err != nil {
		t.Fatalf("failed to seed statements: %v", err)
	}

	got, total, err := GetUserBillingStatements(1, BillingStatementFilter{
		PeriodStart: 100,
		PeriodEnd:   200,
		ModelName:   "doubao-seedance-2-0",
		UserId:      2,
		Username:    "bob",
	}, 0, 100)
	if err != nil {
		t.Fatalf("GetUserBillingStatements() error = %v", err)
	}
	if total != 1 || len(got) != 1 {
		t.Fatalf("total=%d len=%d statements=%+v", total, len(got), got)
	}
	if got[0].UserId != 1 || got[0].Username != "alice" || got[0].NetQuota != 750 {
		t.Fatalf("unexpected statement: %+v", got[0])
	}
}

func TestGenerateBillingStatementsIsIdempotentForSamePeriod(t *testing.T) {
	db := setupLogBillingTestDB(t)
	logs := []*Log{
		{UserId: 1, Username: "alice", CreatedAt: 10, Type: LogTypeConsume, ModelName: "doubao-seedance-2-0", Quota: 1000, ChannelId: 45, Group: "vip"},
	}
	if err := db.Create(&logs).Error; err != nil {
		t.Fatalf("failed to seed logs: %v", err)
	}
	filter := BillingStatementFilter{
		PeriodStart: 1,
		PeriodEnd:   40,
		UserId:      1,
	}
	if _, err := GenerateBillingStatements(filter); err != nil {
		t.Fatalf("first GenerateBillingStatements() error = %v", err)
	}

	refund := &Log{UserId: 1, Username: "alice", CreatedAt: 20, Type: LogTypeRefund, ModelName: "doubao-seedance-2-0", Quota: 250, ChannelId: 45, Group: "vip"}
	if err := db.Create(refund).Error; err != nil {
		t.Fatalf("failed to seed refund: %v", err)
	}
	if _, err := GenerateBillingStatements(filter); err != nil {
		t.Fatalf("second GenerateBillingStatements() error = %v", err)
	}

	statements, total, err := GetBillingStatements(filter, 0, 100)
	if err != nil {
		t.Fatalf("GetBillingStatements() error = %v", err)
	}
	if total != 1 || len(statements) != 1 {
		t.Fatalf("statements total/len = %d/%d", total, len(statements))
	}
	if statements[0].NetQuota != 750 {
		t.Fatalf("net quota = %d", statements[0].NetQuota)
	}
}

func TestGetBillingAlertMetricsRaisesOperationalSignals(t *testing.T) {
	db := setupLogBillingTestDB(t)
	now := common.GetTimestamp()
	logs := []*Log{
		{UserId: 1, Username: "alice", CreatedAt: now - 100, Type: LogTypeConsume, ModelName: "doubao-seedance-2-0", Quota: 1000, ChannelId: 45, Group: "vip"},
		{UserId: 1, Username: "alice", CreatedAt: now - 90, Type: LogTypeRefund, ModelName: "doubao-seedance-2-0", Quota: 500, ChannelId: 45, Group: "vip"},
		{UserId: 1, Username: "alice", CreatedAt: now - 80, Type: LogTypeUnknown, ModelName: "doubao-seedance-2-0", Content: "用户余额不足", ChannelId: 45, Group: "vip"},
	}
	if err := db.Create(&logs).Error; err != nil {
		t.Fatalf("failed to seed logs: %v", err)
	}
	tasks := []*Task{
		{UserId: 1, SubmitTime: now - 100, FinishTime: now - 50, UpdatedAt: now - 50, Status: TaskStatusSuccess, ChannelId: 45, Group: "vip", Properties: Properties{OriginModelName: "doubao-seedance-2-0"}},
		{UserId: 1, SubmitTime: now - 100, FinishTime: now - 40, UpdatedAt: now - 40, Status: TaskStatusFailure, FailReason: "upstream returned error", ChannelId: 45, Group: "vip", Properties: Properties{OriginModelName: "doubao-seedance-2-0"}},
		{UserId: 1, SubmitTime: now - 7200, UpdatedAt: now - 3600, Status: TaskStatusInProgress, ChannelId: 45, Group: "vip", Properties: Properties{OriginModelName: "doubao-seedance-2-0"}},
	}
	if err := db.Create(&tasks).Error; err != nil {
		t.Fatalf("failed to seed tasks: %v", err)
	}

	metrics, err := GetBillingAlertMetrics(BillingAlertFilter{
		StartTimestamp:     now - 200,
		EndTimestamp:       now + 1,
		ModelName:          "doubao-seedance-2-0%",
		UserId:             1,
		Channel:            45,
		Group:              "vip",
		TaskTimeoutSeconds: 3600,
	})
	if err != nil {
		t.Fatalf("GetBillingAlertMetrics() error = %v", err)
	}
	if metrics.RefundCount != 1 || metrics.RequestCount != 1 {
		t.Fatalf("refund/request count = %d/%d", metrics.RefundCount, metrics.RequestCount)
	}
	if metrics.TaskSuccessCount != 1 || metrics.TaskFailureCount != 1 {
		t.Fatalf("task success/failure count = %d/%d", metrics.TaskSuccessCount, metrics.TaskFailureCount)
	}
	if metrics.PendingTaskCount != 1 || metrics.TimedOutTaskCount != 1 {
		t.Fatalf("pending/timedout count = %d/%d", metrics.PendingTaskCount, metrics.TimedOutTaskCount)
	}
	if metrics.UpstreamErrorCount != 1 || metrics.InsufficientBalanceCount != 1 {
		t.Fatalf("upstream/insufficient count = %d/%d", metrics.UpstreamErrorCount, metrics.InsufficientBalanceCount)
	}
	alertKeys := map[string]bool{}
	for _, alert := range metrics.Alerts {
		alertKeys[alert.Key] = true
	}
	for _, key := range []string{"refund_rate", "task_failure_rate", "timed_out_tasks", "worker_lag", "insufficient_balance", "upstream_errors"} {
		if !alertKeys[key] {
			t.Fatalf("expected alert %s in %+v", key, metrics.Alerts)
		}
	}
}
