package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type runningHubH3FallbackPollingAdaptor struct {
	mu           sync.Mutex
	results      map[string]*relaycommon.TaskInfo
	rawResponses map[string][]byte
}

func (a *runningHubH3FallbackPollingAdaptor) Init(_ *relaycommon.RelayInfo) {}

func (a *runningHubH3FallbackPollingAdaptor) FetchTask(_ string, _ string, body map[string]any, _ string) (*http.Response, error) {
	taskID, _ := body["task_id"].(string)
	a.mu.Lock()
	result := a.results[taskID]
	rawResponse := a.rawResponses[taskID]
	a.mu.Unlock()
	if len(rawResponse) > 0 {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(rawResponse))}, nil
	}
	if result == nil {
		return nil, assert.AnError
	}
	payload, err := common.Marshal(result)
	if err != nil {
		return nil, err
	}
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(payload))}, nil
}

func (a *runningHubH3FallbackPollingAdaptor) ParseTaskResult(body []byte) (*relaycommon.TaskInfo, error) {
	if bytes.Contains(body, []byte("torch.OutOfMemoryError")) {
		return &relaycommon.TaskInfo{Code: 805, Status: string(model.TaskStatusFailure), Reason: "torch.OutOfMemoryError"}, nil
	}
	result := &relaycommon.TaskInfo{}
	if err := common.Unmarshal(body, result); err != nil {
		return nil, err
	}
	return result, nil
}

func (a *runningHubH3FallbackPollingAdaptor) AdjustBillingOnComplete(_ *model.Task, _ *relaycommon.TaskInfo) int {
	return 0
}

func TestRunningHubH3FallbackDurations(t *testing.T) {
	assert.Equal(t, []int{5, 5, 5}, runningHubH3FallbackDurations(15))
	assert.Equal(t, []int{5, 5, 2}, runningHubH3FallbackDurations(12))
	assert.Nil(t, runningHubH3FallbackDurations(5))
	assert.Nil(t, runningHubH3FallbackDurations(runningHubH3FallbackMaxSegments*runningHubH3FallbackSegmentSeconds+1))
}

func TestIsRunningHubH3OOMResponseRequiresCodeAndMemorySignature(t *testing.T) {
	oom := []byte(`{"code":805,"data":{"status":"FAILED","failedReason":{"exception_type":"torch.OutOfMemoryError"}}}`)
	assert.True(t, isRunningHubH3OOMResponse(oom, &relaycommon.TaskInfo{Code: 805, Status: string(model.TaskStatusFailure)}))
	assert.True(t, isRunningHubH3OOMResponse([]byte(`{"errorCode":"805","failedReason":{"exception_type":"torch.OutOfMemoryError"}}`), &relaycommon.TaskInfo{Status: string(model.TaskStatusFailure)}))

	assert.False(t, isRunningHubH3OOMResponse([]byte(`{"code":805,"data":{"status":"FAILED","message":"invalid input"}}`), &relaycommon.TaskInfo{Code: 805}))
	assert.False(t, isRunningHubH3OOMResponse([]byte(`{"code":500,"data":{"status":"FAILED","message":"out of memory"}}`), &relaycommon.TaskInfo{Code: 500}))
}

func TestRunningHubH3FallbackRequestForDurationUpdatesNodeAndWorkflow(t *testing.T) {
	snapshot := testRunningHubH3FallbackRequest(15)
	request, err := runningHubH3FallbackRequestForDuration(snapshot, "test-key", 5)
	require.NoError(t, err)
	require.Equal(t, "test-key", request.APIKey)

	var duration any
	for _, node := range request.NodeInfoList {
		if node.NodeID == "132" && node.FieldName == "value" {
			duration = node.FieldValue
		}
	}
	require.Equal(t, 5, duration)

	var workflow map[string]any
	require.NoError(t, common.Unmarshal([]byte(request.Workflow), &workflow))
	inputs := workflow["132"].(map[string]any)["inputs"].(map[string]any)
	require.Equal(t, float64(5), inputs["value"])
}

func TestStartRunningHubH3OOMFallbackSubmitsFiveSecondSegments(t *testing.T) {
	var durations []int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, runningHubH3FallbackCreatePath, r.URL.Path)
		var request runningHubH3FallbackCreateRequest
		require.NoError(t, common.DecodeJson(r.Body, &request))
		for _, node := range request.NodeInfoList {
			if node.NodeID == "132" && node.FieldName == "value" {
				durations = append(durations, int(node.FieldValue.(float64)))
			}
		}
		index := len(durations)
		_, _ = w.Write([]byte(`{"code":0,"data":{"taskId":"segment-` + strconv.Itoa(index) + `"}}`))
	}))
	defer server.Close()

	task := &model.Task{TaskID: "task_fallback_15", PrivateData: model.TaskPrivateData{
		RunningHubH3Fallback: &model.RunningHubH3FallbackState{Request: testRunningHubH3FallbackRequest(15)},
	}}
	result, started, err := startRunningHubH3OOMFallback(context.Background(), server.URL, "test-key", "", task)
	require.NoError(t, err)
	require.True(t, started)
	require.Equal(t, string(model.TaskStatusQueued), result.Status)
	require.Equal(t, []int{5, 5, 5}, durations)
	require.Equal(t, runningHubH3FallbackStateRunning, task.PrivateData.RunningHubH3Fallback.State)
	require.Len(t, task.PrivateData.RunningHubH3Fallback.Segments, 3)
	assert.Equal(t, "segment-1", task.PrivateData.RunningHubH3Fallback.Segments[0].UpstreamTaskID)
	assert.Equal(t, "segment-3", task.PrivateData.RunningHubH3Fallback.Segments[2].UpstreamTaskID)
}

func TestAdvanceRunningHubH3OOMFallbackCompletesOriginalTask(t *testing.T) {
	task := &model.Task{TaskID: "task_fallback_done", PrivateData: model.TaskPrivateData{
		RunningHubH3Fallback: &model.RunningHubH3FallbackState{
			State: runningHubH3FallbackStateRunning,
			Segments: []model.RunningHubH3FallbackSegment{
				{Duration: 5, UpstreamTaskID: "segment-1", Status: model.TaskStatusInProgress},
				{Duration: 5, UpstreamTaskID: "segment-2", Status: model.TaskStatusQueued},
			},
		},
	}}
	adaptor := &runningHubH3FallbackPollingAdaptor{results: map[string]*relaycommon.TaskInfo{
		"segment-1": {Status: string(model.TaskStatusSuccess), Progress: "100%", Url: "https://cdn.example/one.mp4"},
		"segment-2": {Status: string(model.TaskStatusSuccess), Progress: "100%", Url: "https://cdn.example/two.mp4"},
	}}

	result, handled, err := advanceRunningHubH3OOMFallbackWithConcat(context.Background(), adaptor, "https://runninghub.example", "test-key", "", task, func(_ context.Context, taskID string, urls []string) (string, error) {
		require.Equal(t, "task_fallback_done", taskID)
		require.Equal(t, []string{"https://cdn.example/one.mp4", "https://cdn.example/two.mp4"}, urls)
		return "task_fallback_done.mp4", nil
	})
	require.NoError(t, err)
	require.True(t, handled)
	assert.Equal(t, string(model.TaskStatusSuccess), result.Status)
	assert.Equal(t, runningHubH3FallbackStateCompleted, task.PrivateData.RunningHubH3Fallback.State)
	assert.Equal(t, "task_fallback_done.mp4", task.PrivateData.RunningHubH3Fallback.ResultFile)
}

func TestAdvanceRunningHubH3OOMFallbackFailsWhenASegmentFails(t *testing.T) {
	task := &model.Task{TaskID: "task_fallback_segment_failure", PrivateData: model.TaskPrivateData{
		RunningHubH3Fallback: &model.RunningHubH3FallbackState{
			State:    runningHubH3FallbackStateRunning,
			Segments: []model.RunningHubH3FallbackSegment{{Duration: 5, UpstreamTaskID: "segment-1"}},
		},
	}}
	adaptor := &runningHubH3FallbackPollingAdaptor{results: map[string]*relaycommon.TaskInfo{
		"segment-1": {Status: string(model.TaskStatusFailure), Reason: "bad segment"},
	}}

	result, handled, err := advanceRunningHubH3OOMFallback(context.Background(), adaptor, "https://runninghub.example", "test-key", "", task)
	require.NoError(t, err)
	require.True(t, handled)
	assert.Equal(t, string(model.TaskStatusFailure), result.Status)
	assert.Contains(t, result.Reason, "segment 1 failed")
	assert.Equal(t, "failed", task.PrivateData.RunningHubH3Fallback.State)
}

func TestUpdateVideoSingleTaskStartsRunningHubFallbackWithoutRefund(t *testing.T) {
	truncate(t)

	var submitted []int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, runningHubH3FallbackCreatePath, r.URL.Path)
		var request runningHubH3FallbackCreateRequest
		require.NoError(t, common.DecodeJson(r.Body, &request))
		for _, node := range request.NodeInfoList {
			if node.NodeID == "132" && node.FieldName == "value" {
				submitted = append(submitted, int(node.FieldValue.(float64)))
			}
		}
		_, _ = w.Write([]byte(`{"code":0,"data":{"taskId":"fallback-` + strconv.Itoa(len(submitted)) + `"}}`))
	}))
	defer server.Close()

	baseURL := server.URL
	task := &model.Task{
		TaskID:    "task_fallback_no_refund",
		Platform:  constant.TaskPlatform("62"),
		ChannelId: 1,
		Status:    model.TaskStatusInProgress,
		Progress:  "50%",
		Quota:     900,
		CreatedAt: time.Now().Unix(),
		PrivateData: model.TaskPrivateData{
			Key:            "test-key",
			UpstreamTaskID: "original-oom",
			RunningHubH3Fallback: &model.RunningHubH3FallbackState{
				Request: testRunningHubH3FallbackRequest(15),
			},
		},
	}
	require.NoError(t, model.DB.Create(task).Error)

	adaptor := &runningHubH3FallbackPollingAdaptor{results: map[string]*relaycommon.TaskInfo{}}
	oomPayload := []byte(`{"code":805,"data":{"status":"FAILED","failedReason":{"exception_type":"torch.OutOfMemoryError"}}}`)
	adaptor.rawResponses = map[string][]byte{"original-oom": oomPayload}

	channel := &model.Channel{Id: 1, Type: constant.ChannelTypeRunningHub, Key: "test-key", BaseURL: &baseURL}
	require.NoError(t, updateVideoSingleTask(context.Background(), adaptor, channel, "original-oom", map[string]*model.Task{"original-oom": task}))

	var reloaded model.Task
	require.NoError(t, model.DB.First(&reloaded, task.ID).Error)
	assert.Equal(t, model.TaskStatus(model.TaskStatusQueued), reloaded.Status)
	assert.Equal(t, 900, reloaded.Quota)
	require.NotNil(t, reloaded.PrivateData.RunningHubH3Fallback)
	assert.Equal(t, runningHubH3FallbackStateRunning, reloaded.PrivateData.RunningHubH3Fallback.State)
	assert.Equal(t, []int{5, 5, 5}, submitted)
}

func TestUpdateVideoSingleTaskPersistsRunningHubFallbackSegmentProgress(t *testing.T) {
	truncate(t)

	task := &model.Task{
		TaskID:    "task_fallback_progress",
		Platform:  constant.TaskPlatform("62"),
		ChannelId: 2,
		Status:    model.TaskStatusInProgress,
		Progress:  "30%",
		CreatedAt: time.Now().Unix(),
		PrivateData: model.TaskPrivateData{
			Key:            "test-key",
			UpstreamTaskID: "original-oom",
			RunningHubH3Fallback: &model.RunningHubH3FallbackState{
				State: runningHubH3FallbackStateRunning,
				Segments: []model.RunningHubH3FallbackSegment{
					{Duration: 5, UpstreamTaskID: "segment-1", Status: model.TaskStatusQueued, Progress: "20%"},
				},
			},
		},
	}
	require.NoError(t, model.DB.Create(task).Error)

	adaptor := &runningHubH3FallbackPollingAdaptor{results: map[string]*relaycommon.TaskInfo{
		"segment-1": {Status: string(model.TaskStatusInProgress), Progress: "60%"},
	}}
	channel := &model.Channel{Id: 2, Type: constant.ChannelTypeRunningHub, Key: "test-key"}
	require.NoError(t, updateVideoSingleTask(context.Background(), adaptor, channel, "original-oom", map[string]*model.Task{"original-oom": task}))

	var reloaded model.Task
	require.NoError(t, model.DB.First(&reloaded, task.ID).Error)
	require.Equal(t, model.TaskStatus(model.TaskStatusInProgress), reloaded.Status)
	require.NotNil(t, reloaded.PrivateData.RunningHubH3Fallback)
	require.Len(t, reloaded.PrivateData.RunningHubH3Fallback.Segments, 1)
	assert.Equal(t, "60%", reloaded.PrivateData.RunningHubH3Fallback.Segments[0].Progress)
}

func TestOpenRunningHubH3FallbackResultOnlyAllowsDeterministicLocalFile(t *testing.T) {
	directory := t.TempDir()
	t.Setenv(runningHubH3FallbackMediaDirEnv, directory)

	resultFile, err := runningHubH3FallbackResultFileName("task_safe_result")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(directory, resultFile), []byte("video"), 0600))

	file, info, err := OpenRunningHubH3FallbackResult("task_safe_result", resultFile)
	require.NoError(t, err)
	require.Equal(t, int64(5), info.Size())
	require.NoError(t, file.Close())

	_, _, err = OpenRunningHubH3FallbackResult("task_safe_result", "../../etc/passwd")
	require.Error(t, err)
}

func testRunningHubH3FallbackRequest(duration int) *model.RunningHubH3FallbackRequest {
	return &model.RunningHubH3FallbackRequest{
		WorkflowID: "workflow-test",
		Duration:   duration,
		NodeInfoList: []model.RunningHubH3FallbackNode{
			{NodeID: "138", FieldName: "value", FieldValue: "a dancer"},
			{NodeID: "132", FieldName: "value", FieldValue: duration},
		},
		Workflow: `{"132":{"inputs":{"value":` + strconv.Itoa(duration) + `}},"138":{"inputs":{"value":"a dancer"}}}`,
	}
}
