package relay

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddTaskOutputMetadata(t *testing.T) {
	tests := []struct {
		name        string
		privateData model.TaskPrivateData
		input       json.RawMessage
		want        string
	}{
		{
			name: "merges normalized metadata into upstream object",
			privateData: model.TaskPrivateData{
				OutputSeconds: 5,
				OutputSize:    "1376x768",
			},
			input: json.RawMessage(`{"provider":"comfyui","seconds":99,"size":"raw"}`),
			want:  `{"provider":"comfyui","seconds":5,"size":"1376x768"}`,
		},
		{
			name: "replaces non object upstream data",
			privateData: model.TaskPrivateData{
				OutputSeconds: 10,
				OutputSize:    "1920x1088",
			},
			input: json.RawMessage(`"upstream-task-id"`),
			want:  `{"seconds":10,"size":"1920x1088"}`,
		},
		{
			name: "includes available fields only",
			privateData: model.TaskPrivateData{
				OutputSeconds: 5,
			},
			input: json.RawMessage(`null`),
			want:  `{"seconds":5}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &model.Task{Status: model.TaskStatusSuccess, PrivateData: tt.privateData}
			taskDto := &dto.TaskDto{Data: append(json.RawMessage(nil), tt.input...)}

			require.NoError(t, addTaskOutputMetadata(task, taskDto))
			assert.JSONEq(t, tt.want, string(taskDto.Data))
		})
	}
}

func TestAddTaskOutputMetadataLeavesUnaffectedTasksUnchanged(t *testing.T) {
	tests := []struct {
		name        string
		status      model.TaskStatus
		privateData model.TaskPrivateData
	}{
		{
			name:   "non success task",
			status: model.TaskStatusInProgress,
			privateData: model.TaskPrivateData{
				OutputSeconds: 5,
				OutputSize:    "1376x768",
			},
		},
		{
			name:   "legacy success task",
			status: model.TaskStatusSuccess,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := json.RawMessage(`{"provider":"unchanged"}`)
			task := &model.Task{Status: tt.status, PrivateData: tt.privateData}
			taskDto := &dto.TaskDto{Data: append(json.RawMessage(nil), original...)}

			require.NoError(t, addTaskOutputMetadata(task, taskDto))
			assert.Equal(t, original, taskDto.Data)
		})
	}
}

func TestAddTaskOutputMetadataRejectsMalformedObjectData(t *testing.T) {
	task := &model.Task{
		Status: model.TaskStatusSuccess,
		PrivateData: model.TaskPrivateData{
			OutputSeconds: 5,
		},
	}
	taskDto := &dto.TaskDto{Data: json.RawMessage(`{"provider":`)}

	err := addTaskOutputMetadata(task, taskDto)
	require.Error(t, err)
	assert.Equal(t, "object", common.GetJsonType(taskDto.Data))
}

func TestTaskOutputMetadataSuccessResponseContract(t *testing.T) {
	task := &model.Task{
		TaskID:     "task-contract",
		Status:     model.TaskStatusSuccess,
		Data:       json.RawMessage(`{"provider":"comfyui"}`),
		FailReason: "",
		PrivateData: model.TaskPrivateData{
			ResultURL:     "https://example.com/video.mp4",
			OutputSeconds: 5,
			OutputSize:    "1376x768",
		},
	}
	taskDto := TaskModel2Dto(task)
	require.NoError(t, addTaskOutputMetadata(task, taskDto))

	encoded, err := common.Marshal(dto.TaskResponse[any]{Code: dto.TaskSuccessCode, Data: taskDto})
	require.NoError(t, err)

	var response map[string]any
	require.NoError(t, common.Unmarshal(encoded, &response))
	assert.Equal(t, dto.TaskSuccessCode, response["code"])
	publicTask, ok := response["data"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, string(model.TaskStatusSuccess), publicTask["status"])
	assert.Equal(t, "https://example.com/video.mp4", publicTask["result_url"])
	publicData, ok := publicTask["data"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(5), publicData["seconds"])
	assert.Equal(t, "1376x768", publicData["size"])
	assert.Equal(t, "comfyui", publicData["provider"])
}

func TestTaskOutputMetadataUsesFrozenValuesAfterPollingDataReplacement(t *testing.T) {
	task := &model.Task{
		Status: model.TaskStatusSuccess,
		Data:   json.RawMessage(`{"poll_status":"completed"}`),
		PrivateData: model.TaskPrivateData{
			OutputSeconds: 5,
			OutputSize:    "1376x768",
		},
	}
	taskDto := TaskModel2Dto(task)

	require.NoError(t, addTaskOutputMetadata(task, taskDto))
	assert.JSONEq(t, `{"poll_status":"completed","seconds":5,"size":"1376x768"}`, string(taskDto.Data))
}
