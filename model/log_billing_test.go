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

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	logGroupCol = `"group"`

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	LOG_DB = db
	if err := db.AutoMigrate(&Log{}); err != nil {
		t.Fatalf("failed to migrate logs: %v", err)
	}
	t.Cleanup(func() {
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

	summary, err := GetBillingSummary(0, 0, "doubao-seedance-2-0", "", 0, 0, "", 100)
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

	summary, err := GetBillingSummary(0, 0, "", "", 1, 45, "vip", 100)
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

func TestGetBillingExportLogsSanitizesModelFilter(t *testing.T) {
	db := setupLogBillingTestDB(t)
	logs := []*Log{
		{UserId: 1, Username: "alice", CreatedAt: 10, Type: LogTypeConsume, ModelName: "doubao-seedance-2-0", Quota: 1000, ChannelId: 45, Group: "vip"},
		{UserId: 2, Username: "bob", CreatedAt: 20, Type: LogTypeConsume, ModelName: "other-model", Quota: 500, ChannelId: 46, Group: "default"},
	}
	if err := db.Create(&logs).Error; err != nil {
		t.Fatalf("failed to seed logs: %v", err)
	}

	exported, err := GetBillingExportLogs(LogTypeUnknown, 0, 0, "doubao-seedance-2-0", "", 0, "", 0, "", "", 100)
	if err != nil {
		t.Fatalf("GetBillingExportLogs() error = %v", err)
	}
	if len(exported) != 1 || exported[0].ModelName != "doubao-seedance-2-0" {
		t.Fatalf("unexpected exported logs: %+v", exported)
	}

	if _, err := GetBillingExportLogs(LogTypeUnknown, 0, 0, "%%%%", "", 0, "", 0, "", "", 100); err == nil {
		t.Fatalf("expected invalid wildcard pattern error")
	}
}
