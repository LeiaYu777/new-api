package controller

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

func GetAllLogs(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	logType, _ := strconv.Atoi(c.Query("type"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	username := c.Query("username")
	tokenName := c.Query("token_name")
	modelName := c.Query("model_name")
	channel, _ := strconv.Atoi(c.Query("channel"))
	group := c.Query("group")
	requestId := c.Query("request_id")
	logs, total, err := model.GetAllLogs(logType, startTimestamp, endTimestamp, modelName, username, tokenName, pageInfo.GetStartIdx(), pageInfo.GetPageSize(), channel, group, requestId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(logs)
	common.ApiSuccess(c, pageInfo)
	return
}

func GetUserLogs(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	userId := c.GetInt("id")
	logType, _ := strconv.Atoi(c.Query("type"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	tokenName := c.Query("token_name")
	modelName := c.Query("model_name")
	group := c.Query("group")
	requestId := c.Query("request_id")
	logs, total, err := model.GetUserLogs(userId, logType, startTimestamp, endTimestamp, modelName, tokenName, pageInfo.GetStartIdx(), pageInfo.GetPageSize(), group, requestId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(logs)
	common.ApiSuccess(c, pageInfo)
	return
}

// Deprecated: SearchAllLogs 已废弃，前端未使用该接口。
func SearchAllLogs(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": false,
		"message": "该接口已废弃",
	})
}

// Deprecated: SearchUserLogs 已废弃，前端未使用该接口。
func SearchUserLogs(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": false,
		"message": "该接口已废弃",
	})
}

func billingOtherValue(other map[string]interface{}, key string) string {
	if other == nil {
		return ""
	}
	if value, ok := other[key]; ok {
		return fmt.Sprint(value)
	}
	return ""
}

func billingCSVCell(value string) string {
	if value == "" {
		return ""
	}
	switch value[0] {
	case '=', '+', '-', '@':
		return "'" + value
	default:
		return value
	}
}

func ExportBillingLogs(c *gin.Context) {
	logType, _ := strconv.Atoi(c.Query("type"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	username := c.Query("username")
	userId, _ := strconv.Atoi(c.Query("user_id"))
	tokenName := c.Query("token_name")
	modelName := c.Query("model_name")
	channel, _ := strconv.Atoi(c.Query("channel"))
	group := c.Query("group")
	requestId := c.Query("request_id")
	taskId := c.Query("task_id")
	billingSource := c.Query("billing_source")
	limit, _ := strconv.Atoi(c.Query("limit"))

	logs, err := model.GetBillingExportLogs(logType, startTimestamp, endTimestamp, modelName, username, userId, tokenName, channel, group, requestId, taskId, billingSource, limit)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=billing-logs.csv")
	writer := csv.NewWriter(c.Writer)
	_ = writer.Write([]string{
		"created_at",
		"user_id",
		"username",
		"model_name",
		"channel_id",
		"token_id",
		"log_type",
		"quota",
		"group",
		"request_id",
		"task_id",
		"content",
		"billing_source",
		"subscription_id",
		"pre_consumed_quota",
		"actual_quota",
	})
	for _, log := range logs {
		other, _ := common.StrToMap(log.Other)
		_ = writer.Write([]string{
			time.Unix(log.CreatedAt, 0).Format(time.RFC3339),
			strconv.Itoa(log.UserId),
			log.Username,
			log.ModelName,
			strconv.Itoa(log.ChannelId),
			strconv.Itoa(log.TokenId),
			strconv.Itoa(log.Type),
			strconv.Itoa(log.Quota),
			log.Group,
			log.RequestId,
			billingOtherValue(other, "task_id"),
			billingCSVCell(log.Content),
			billingCSVCell(billingOtherValue(other, "billing_source")),
			billingCSVCell(billingOtherValue(other, "subscription_id")),
			billingCSVCell(billingOtherValue(other, "pre_consumed_quota")),
			billingCSVCell(billingOtherValue(other, "actual_quota")),
		})
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		common.ApiError(c, err)
	}
}

func parseBillingTimestampQuery(c *gin.Context, primary string, fallback string) int64 {
	value := c.Query(primary)
	if value == "" && fallback != "" {
		value = c.Query(fallback)
	}
	timestamp, _ := strconv.ParseInt(value, 10, 64)
	return timestamp
}

func parseBillingStatementFilter(c *gin.Context) model.BillingStatementFilter {
	userId, _ := strconv.Atoi(c.Query("user_id"))
	channel, _ := strconv.Atoi(c.Query("channel"))
	return model.BillingStatementFilter{
		PeriodStart:   parseBillingTimestampQuery(c, "period_start", "start_timestamp"),
		PeriodEnd:     parseBillingTimestampQuery(c, "period_end", "end_timestamp"),
		Username:      c.Query("username"),
		UserId:        userId,
		ModelName:     c.Query("model_name"),
		Channel:       channel,
		Group:         c.Query("group"),
		BillingSource: c.Query("billing_source"),
		Status:        c.Query("status"),
	}
}

func parseBillingAlertFilter(c *gin.Context) model.BillingAlertFilter {
	userId, _ := strconv.Atoi(c.Query("user_id"))
	channel, _ := strconv.Atoi(c.Query("channel"))
	timeoutSeconds, _ := strconv.ParseInt(c.Query("task_timeout_seconds"), 10, 64)
	return model.BillingAlertFilter{
		StartTimestamp:     parseBillingTimestampQuery(c, "start_timestamp", "period_start"),
		EndTimestamp:       parseBillingTimestampQuery(c, "end_timestamp", "period_end"),
		Username:           c.Query("username"),
		UserId:             userId,
		ModelName:          c.Query("model_name"),
		Channel:            channel,
		Group:              c.Query("group"),
		BillingSource:      c.Query("billing_source"),
		TaskTimeoutSeconds: timeoutSeconds,
	}
}

func GetBillingSummary(c *gin.Context) {
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	username := c.Query("username")
	userId, _ := strconv.Atoi(c.Query("user_id"))
	modelName := c.Query("model_name")
	channel, _ := strconv.Atoi(c.Query("channel"))
	group := c.Query("group")
	taskId := c.Query("task_id")
	billingSource := c.Query("billing_source")
	limit, _ := strconv.Atoi(c.Query("limit"))

	summary, err := model.GetBillingSummary(startTimestamp, endTimestamp, modelName, username, userId, channel, group, taskId, billingSource, limit)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, summary)
}

func GetBillingAlerts(c *gin.Context) {
	filter := parseBillingAlertFilter(c)
	metrics, err := model.GetBillingAlertMetrics(filter)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, metrics)
}

func GetBillingStatements(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	filter := parseBillingStatementFilter(c)
	statements, total, err := model.GetBillingStatements(filter, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(statements)
	common.ApiSuccess(c, pageInfo)
}

func GenerateBillingStatements(c *gin.Context) {
	filter := parseBillingStatementFilter(c)
	result, err := model.GenerateBillingStatements(filter)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func GetLogByKey(c *gin.Context) {
	tokenId := c.GetInt("token_id")
	if tokenId == 0 {
		c.JSON(200, gin.H{
			"success": false,
			"message": "无效的令牌",
		})
		return
	}
	logs, err := model.GetLogByTokenId(tokenId)
	if err != nil {
		c.JSON(200, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(200, gin.H{
		"success": true,
		"message": "",
		"data":    logs,
	})
}

func GetLogsStat(c *gin.Context) {
	logType, _ := strconv.Atoi(c.Query("type"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	tokenName := c.Query("token_name")
	username := c.Query("username")
	modelName := c.Query("model_name")
	channel, _ := strconv.Atoi(c.Query("channel"))
	group := c.Query("group")
	stat, err := model.SumUsedQuota(logType, startTimestamp, endTimestamp, modelName, username, tokenName, channel, group)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	//tokenNum := model.SumUsedToken(logType, startTimestamp, endTimestamp, modelName, username, "")
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"quota": stat.Quota,
			"rpm":   stat.Rpm,
			"tpm":   stat.Tpm,
		},
	})
	return
}

func GetLogsSelfStat(c *gin.Context) {
	username := c.GetString("username")
	logType, _ := strconv.Atoi(c.Query("type"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	tokenName := c.Query("token_name")
	modelName := c.Query("model_name")
	channel, _ := strconv.Atoi(c.Query("channel"))
	group := c.Query("group")
	quotaNum, err := model.SumUsedQuota(logType, startTimestamp, endTimestamp, modelName, username, tokenName, channel, group)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	//tokenNum := model.SumUsedToken(logType, startTimestamp, endTimestamp, modelName, username, tokenName)
	c.JSON(200, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"quota": quotaNum.Quota,
			"rpm":   quotaNum.Rpm,
			"tpm":   quotaNum.Tpm,
			//"token": tokenNum,
		},
	})
	return
}

func DeleteHistoryLogs(c *gin.Context) {
	targetTimestamp, _ := strconv.ParseInt(c.Query("target_timestamp"), 10, 64)
	if targetTimestamp == 0 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "target timestamp is required",
		})
		return
	}
	count, err := model.DeleteOldLog(c.Request.Context(), targetTimestamp, 100)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    count,
	})
	return
}
