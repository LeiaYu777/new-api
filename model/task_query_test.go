package model

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskQueryFiltersBillingSource(t *testing.T) {
	truncateTables(t)

	tasks := []*Task{
		{
			TaskID:     "task_wallet",
			Platform:   constant.TaskPlatform("54"),
			UserId:     1,
			ChannelId:  45,
			Action:     constant.TaskActionTextGenerate,
			Status:     TaskStatusSuccess,
			SubmitTime: time.Now().Unix(),
			Quota:      1000,
			Data:       json.RawMessage(`{}`),
			PrivateData: TaskPrivateData{
				BillingSource: billingSourceWallet,
			},
		},
		{
			TaskID:     "task_subscription",
			Platform:   constant.TaskPlatform("54"),
			UserId:     2,
			ChannelId:  45,
			Action:     constant.TaskActionTextGenerate,
			Status:     TaskStatusSuccess,
			SubmitTime: time.Now().Unix(),
			Quota:      2000,
			Data:       json.RawMessage(`{}`),
			PrivateData: TaskPrivateData{
				BillingSource:  billingSourceSubscription,
				SubscriptionId: 7,
			},
		},
		{
			TaskID:     "task_legacy_wallet",
			Platform:   constant.TaskPlatform("54"),
			UserId:     3,
			ChannelId:  45,
			Action:     constant.TaskActionGenerate,
			Status:     TaskStatusFailure,
			SubmitTime: time.Now().Unix(),
			Quota:      500,
			Data:       json.RawMessage(`{}`),
		},
	}
	for _, task := range tasks {
		insertTask(t, task)
	}

	subscriptionTasks := TaskGetAllTasks(0, 10, SyncTaskQueryParams{
		BillingSource: billingSourceSubscription,
	})
	require.Len(t, subscriptionTasks, 1)
	assert.Equal(t, "task_subscription", subscriptionTasks[0].TaskID)
	assert.Equal(t, int64(1), TaskCountAllTasks(SyncTaskQueryParams{
		BillingSource: billingSourceSubscription,
	}))

	walletTasks := TaskGetAllTasks(0, 10, SyncTaskQueryParams{
		BillingSource: billingSourceWallet,
	})
	require.Len(t, walletTasks, 2)
	taskIds := []string{walletTasks[0].TaskID, walletTasks[1].TaskID}
	assert.Contains(t, taskIds, "task_wallet")
	assert.Contains(t, taskIds, "task_legacy_wallet")
	assert.Equal(t, int64(2), TaskCountAllTasks(SyncTaskQueryParams{
		BillingSource: billingSourceWallet,
	}))
}

func TestTaskQueryFiltersActionStatusAndPlatform(t *testing.T) {
	truncateTables(t)

	for _, task := range []*Task{
		{
			TaskID:     "task_seedance_success",
			Platform:   constant.TaskPlatform("54"),
			UserId:     1,
			ChannelId:  45,
			Action:     constant.TaskActionTextGenerate,
			Status:     TaskStatusSuccess,
			SubmitTime: 100,
			Data:       json.RawMessage(`{}`),
		},
		{
			TaskID:     "task_seedance_failure",
			Platform:   constant.TaskPlatform("54"),
			UserId:     1,
			ChannelId:  45,
			Action:     constant.TaskActionGenerate,
			Status:     TaskStatusFailure,
			SubmitTime: 200,
			Data:       json.RawMessage(`{}`),
		},
		{
			TaskID:     "task_suno",
			Platform:   constant.TaskPlatformSuno,
			UserId:     1,
			ChannelId:  36,
			Action:     constant.SunoActionMusic,
			Status:     TaskStatusSuccess,
			SubmitTime: 300,
			Data:       json.RawMessage(`{}`),
		},
	} {
		insertTask(t, task)
	}

	tasks := TaskGetAllTasks(0, 10, SyncTaskQueryParams{
		Platform: constant.TaskPlatform("54"),
		Action:   constant.TaskActionTextGenerate,
		Status:   string(TaskStatusSuccess),
	})
	require.Len(t, tasks, 1)
	assert.Equal(t, "task_seedance_success", tasks[0].TaskID)
	assert.Equal(t, int64(1), TaskCountAllTasks(SyncTaskQueryParams{
		Platform: constant.TaskPlatform("54"),
		Action:   constant.TaskActionTextGenerate,
		Status:   string(TaskStatusSuccess),
	}))
}
