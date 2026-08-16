package relay

import (
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
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

func TestTaskWithPublicResultURLHidesLoopbackWorker(t *testing.T) {
	setTaskTestServerAddress(t, "")
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("GET", "http://103.36.63.156:3000/v1/videos/task_private", nil)
	ctx.Request.Host = "103.36.63.156:3000"

	task := &model.Task{
		TaskID: "task_private",
		UserId: 42,
		PrivateData: model.TaskPrivateData{
			ResultURL: "http://127.0.0.1:5901/view?filename=result.mp4&type=output",
		},
	}

	publicTask := taskWithPublicResultURL(ctx, task)
	require.NotSame(t, task, publicTask)
	assert.Equal(t, "http://127.0.0.1:5901/view?filename=result.mp4&type=output", task.GetResultURL())
	assertSignedTaskContentURL(t, publicTask.GetResultURL(), task.TaskID, task.UserId, "http://103.36.63.156:3000/v1/videos/task_private/content")
}

func TestTaskWithPublicResultURLHidesSuccessfulGatewayResult(t *testing.T) {
	setTaskTestServerAddress(t, "https://api.example.com/")
	task := &model.Task{
		TaskID: "task/gateway",
		UserId: 42,
		Status: model.TaskStatusSuccess,
		PrivateData: model.TaskPrivateData{
			ComfyUIH3Gateway: true,
			ResultURL:        "https://gateway.example.com/view?filename=result.mp4&token=secret",
		},
	}
	task.Data = []byte(`{"status":"completed","result_ready":true,"view_endpoint":"/api/gateway/tasks/upstream/view"}`)

	publicTask := taskWithPublicResultURL(nil, task)
	require.NotSame(t, task, publicTask)
	assert.Equal(t, "https://gateway.example.com/view?filename=result.mp4&token=secret", task.GetResultURL())
	assertSignedTaskContentURL(t, publicTask.GetResultURL(), task.TaskID, task.UserId, "https://api.example.com/v1/videos/task%2Fgateway/content")
	assert.JSONEq(t, `{"status":"completed","result_ready":true}`, string(publicTask.Data))
	assert.Contains(t, string(task.Data), "view_endpoint")
}

func TestTaskModel2PublicDtoUsesSignedGatewayContentURL(t *testing.T) {
	setTaskTestServerAddress(t, "https://api.example.com")
	task := &model.Task{
		TaskID: "task/log-gateway",
		UserId: 42,
		Status: model.TaskStatusSuccess,
		PrivateData: model.TaskPrivateData{
			ComfyUIH3Gateway: true,
			ResultURL:        "http://gateway.internal:8090/api/gateway/tasks/upstream/view",
		},
	}

	taskDTO := TaskModel2PublicDto(nil, task)
	assertSignedTaskContentURL(t, taskDTO.ResultURL, task.TaskID, task.UserId, "https://api.example.com/v1/videos/task%2Flog-gateway/content")
	assert.NotContains(t, taskDTO.ResultURL, "gateway.internal")
	assert.Equal(t, "http://gateway.internal:8090/api/gateway/tasks/upstream/view", task.GetResultURL())
}

func assertSignedTaskContentURL(t *testing.T, rawURL, taskID string, userID int, wantContentURL string) {
	t.Helper()
	parsedURL, err := url.Parse(rawURL)
	require.NoError(t, err)
	parsedContentURL, err := url.Parse(wantContentURL)
	require.NoError(t, err)
	assert.Equal(t, parsedContentURL.Scheme, parsedURL.Scheme)
	assert.Equal(t, parsedContentURL.Host, parsedURL.Host)
	assert.Equal(t, parsedContentURL.EscapedPath(), parsedURL.EscapedPath())
	verifiedUserID, err := service.VerifySignedVideoContentURL(taskID, parsedURL.Query().Get("user_id"), parsedURL.Query().Get("expires"), parsedURL.Query().Get("signature"))
	require.NoError(t, err)
	assert.Equal(t, userID, verifiedUserID)
}

func TestTaskWithPublicResultURLLeavesExternalResultsUntouched(t *testing.T) {
	setTaskTestServerAddress(t, "https://api.example.com")

	tests := []struct {
		name string
		task *model.Task
	}{
		{
			name: "direct task",
			task: &model.Task{
				TaskID: "task_external",
				Status: model.TaskStatusSuccess,
				PrivateData: model.TaskPrivateData{
					ResultURL: "https://videos.example.com/result.mp4",
				},
			},
		},
		{
			name: "unfinished gateway task",
			task: &model.Task{
				TaskID: "task_gateway_pending",
				Status: model.TaskStatusInProgress,
				PrivateData: model.TaskPrivateData{
					ComfyUIH3Gateway: true,
					ResultURL:        "https://gateway.example.com/view?filename=result.mp4&token=secret",
				},
				Data: json.RawMessage(`{"status":"running","view_endpoint":"/api/gateway/tasks/upstream/view"}`),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			publicTask := taskWithPublicResultURL(nil, tt.task)
			if tt.task.PrivateData.ComfyUIH3Gateway {
				require.NotSame(t, tt.task, publicTask)
				assert.JSONEq(t, `{"status":"running"}`, string(publicTask.Data))
				assert.Contains(t, string(tt.task.Data), "view_endpoint")
				return
			}
			assert.Same(t, tt.task, publicTask)
		})
	}
}

func TestTaskWithPublicResultURLDropsMalformedGatewayData(t *testing.T) {
	task := &model.Task{
		TaskID: "task_gateway_malformed",
		Status: model.TaskStatusInProgress,
		PrivateData: model.TaskPrivateData{
			ComfyUIH3Gateway: true,
		},
		Data: json.RawMessage(`{"view_endpoint":`),
	}

	publicTask := taskWithPublicResultURL(nil, task)

	require.NotSame(t, task, publicTask)
	assert.JSONEq(t, `{}`, string(publicTask.Data))
	assert.Equal(t, `{"view_endpoint":`, string(task.Data))
}

func TestBuildPublicTaskContentURLSelectsPublicBase(t *testing.T) {
	tests := []struct {
		name          string
		serverAddress string
		requestURL    string
		want          string
	}{
		{
			name:          "public configured address takes priority",
			serverAddress: "https://api.example.com/",
			requestURL:    "http://103.36.63.156:3000/v1/videos/task_private",
			want:          "https://api.example.com/v1/videos/task_private/content",
		},
		{
			name:          "localhost configured address falls back to public request",
			serverAddress: "http://localhost:3000",
			requestURL:    "http://103.36.63.156:3000/v1/videos/task_private",
			want:          "http://103.36.63.156:3000/v1/videos/task_private/content",
		},
		{
			name:          "loopback IP configured address falls back to TLS request",
			serverAddress: "127.0.0.1:3000",
			requestURL:    "https://request.example.com/v1/videos/task_private",
			want:          "https://request.example.com/v1/videos/task_private/content",
		},
		{
			name:       "empty configured address falls back to request",
			requestURL: "https://request.example.com/v1/videos/task_private",
			want:       "https://request.example.com/v1/videos/task_private/content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setTaskTestServerAddress(t, tt.serverAddress)
			gin.SetMode(gin.TestMode)
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = httptest.NewRequest("GET", tt.requestURL, nil)

			assert.Equal(t, tt.want, buildPublicTaskContentURL(ctx, "task_private"))
		})
	}
}

func setTaskTestServerAddress(t *testing.T, address string) {
	t.Helper()
	previousAddress := system_setting.ServerAddress
	system_setting.ServerAddress = address
	t.Cleanup(func() {
		system_setting.ServerAddress = previousAddress
	})
}
