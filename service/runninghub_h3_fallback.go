package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

const (
	runningHubH3FallbackCreatePath      = "/task/openapi/create"
	runningHubH3FallbackSegmentSeconds  = 5
	runningHubH3FallbackMaxSegments     = 12
	runningHubH3FallbackStateRunning    = "running"
	runningHubH3FallbackStateCompleted  = "completed"
	runningHubH3FallbackMaxSegmentBytes = int64(512 << 20)
	runningHubH3FallbackMediaDirEnv     = "RUNNINGHUB_H3_MEDIA_DIR"
	runningHubH3FallbackFFmpegEnv       = "RUNNINGHUB_H3_FFMPEG"
)

type runningHubH3FallbackNodeInfo struct {
	NodeID     string `json:"nodeId"`
	FieldName  string `json:"fieldName"`
	FieldValue any    `json:"fieldValue"`
}

type runningHubH3FallbackCreateRequest struct {
	APIKey       string                         `json:"apiKey"`
	WorkflowID   string                         `json:"workflowId"`
	NodeInfoList []runningHubH3FallbackNodeInfo `json:"nodeInfoList"`
	Workflow     string                         `json:"workflow"`
}

type runningHubH3FallbackCreateResponse struct {
	Code    int    `json:"code"`
	Msg     string `json:"msg"`
	Message string `json:"message"`
	Data    struct {
		TaskID string `json:"taskId"`
	} `json:"data"`
}

// isRunningHubH3OOMResponse is deliberately narrow. RunningHub code 805 by
// itself is not enough: automatic retries are only allowed for the observed
// CUDA-memory failure signatures, not arbitrary upstream errors.
func isRunningHubH3OOMResponse(responseBody []byte, taskResult *relaycommon.TaskInfo) bool {
	if taskResult != nil && taskResult.Code != 805 && !runningHubH3ResponseHasCode(responseBody, 805) {
		return false
	}
	if taskResult == nil && !runningHubH3ResponseHasCode(responseBody, 805) {
		return false
	}

	message := strings.ToLower(string(responseBody))
	return strings.Contains(message, "torch.outofmemoryerror") ||
		strings.Contains(message, "out of memory") ||
		strings.Contains(message, "显存不足")
}

func runningHubH3ResponseHasCode(responseBody []byte, expected int) bool {
	var body any
	if err := common.Unmarshal(responseBody, &body); err != nil {
		return false
	}

	var visit func(any) bool
	visit = func(value any) bool {
		switch typed := value.(type) {
		case map[string]any:
			for key, item := range typed {
				normalizedKey := strings.ToLower(strings.ReplaceAll(key, "_", ""))
				if (normalizedKey == "code" || normalizedKey == "errorcode") && runningHubH3ValueIsCode(item, expected) {
					return true
				}
				if visit(item) {
					return true
				}
			}
		case []any:
			for _, item := range typed {
				if visit(item) {
					return true
				}
			}
		}
		return false
	}
	return visit(body)
}

func runningHubH3ValueIsCode(value any, expected int) bool {
	switch typed := value.(type) {
	case float64:
		return int(typed) == expected && typed == float64(expected)
	case int:
		return typed == expected
	case int64:
		return typed == int64(expected)
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		return err == nil && parsed == expected
	default:
		return false
	}
}

// startRunningHubH3OOMFallback submits independent five-second H3 tasks after
// the original long-running task has failed with a confirmed CUDA OOM.
func startRunningHubH3OOMFallback(ctx context.Context, baseURL, key, proxy string, task *model.Task) (*relaycommon.TaskInfo, bool, error) {
	if task == nil || task.PrivateData.RunningHubH3Fallback == nil {
		return nil, false, nil
	}
	state := task.PrivateData.RunningHubH3Fallback
	if state.State != "" {
		return nil, false, nil
	}
	if state.Request == nil || state.Request.Duration <= runningHubH3FallbackSegmentSeconds {
		return nil, false, nil
	}
	if strings.TrimSpace(state.Request.WorkflowID) == "" || strings.TrimSpace(state.Request.Workflow) == "" {
		return relaycommon.FailTaskInfo("RunningHub H3 OOM fallback is unavailable because the accepted workflow snapshot is incomplete"), true, nil
	}

	durations := runningHubH3FallbackDurations(state.Request.Duration)
	if len(durations) == 0 {
		return relaycommon.FailTaskInfo(fmt.Sprintf("RunningHub H3 OOM fallback supports at most %d segments", runningHubH3FallbackMaxSegments)), true, nil
	}

	state.State = runningHubH3FallbackStateRunning
	state.Segments = make([]model.RunningHubH3FallbackSegment, len(durations))
	for index, duration := range durations {
		upstreamTaskID, err := submitRunningHubH3FallbackSegment(ctx, baseURL, key, proxy, state.Request, duration)
		if err != nil {
			state.State = "failed"
			return relaycommon.FailTaskInfo(fmt.Sprintf("RunningHub H3 OOM fallback could not submit segment %d: %s", index+1, err.Error())), true, nil
		}
		state.Segments[index] = model.RunningHubH3FallbackSegment{
			Duration:       duration,
			UpstreamTaskID: upstreamTaskID,
			Status:         model.TaskStatusQueued,
			Progress:       taskcommon.ProgressQueued,
		}
	}

	return &relaycommon.TaskInfo{
		Status:   string(model.TaskStatusQueued),
		Progress: taskcommon.ProgressQueued,
	}, true, nil
}

func runningHubH3FallbackDurations(duration int) []int {
	if duration <= runningHubH3FallbackSegmentSeconds {
		return nil
	}
	count := (duration + runningHubH3FallbackSegmentSeconds - 1) / runningHubH3FallbackSegmentSeconds
	if count > runningHubH3FallbackMaxSegments {
		return nil
	}

	segments := make([]int, 0, count)
	for remaining := duration; remaining > 0; {
		segment := runningHubH3FallbackSegmentSeconds
		if remaining < segment {
			segment = remaining
		}
		segments = append(segments, segment)
		remaining -= segment
	}
	return segments
}

func submitRunningHubH3FallbackSegment(ctx context.Context, baseURL, key, proxy string, snapshot *model.RunningHubH3FallbackRequest, duration int) (string, error) {
	requestBody, err := runningHubH3FallbackRequestForDuration(snapshot, key, duration)
	if err != nil {
		return "", err
	}
	body, err := common.Marshal(requestBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+runningHubH3FallbackCreatePath, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	client, err := GetHttpClientWithProxy(strings.TrimSpace(proxy))
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var response runningHubH3FallbackCreateResponse
	if err := common.Unmarshal(responseBody, &response); err != nil {
		return "", fmt.Errorf("decode RunningHub create response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices || (response.Code != 0 && response.Code != http.StatusOK) {
		message := strings.TrimSpace(response.Msg)
		if message == "" {
			message = strings.TrimSpace(response.Message)
		}
		if message == "" {
			message = resp.Status
		}
		return "", fmt.Errorf("RunningHub create failed: %s", message)
	}
	if taskID := strings.TrimSpace(response.Data.TaskID); taskID != "" {
		return taskID, nil
	}
	return "", fmt.Errorf("RunningHub create response missing taskId")
}

func runningHubH3FallbackRequestForDuration(snapshot *model.RunningHubH3FallbackRequest, apiKey string, duration int) (*runningHubH3FallbackCreateRequest, error) {
	if snapshot == nil || duration <= 0 {
		return nil, fmt.Errorf("invalid RunningHub H3 fallback request")
	}

	nodes := make([]runningHubH3FallbackNodeInfo, 0, len(snapshot.NodeInfoList))
	foundDuration := false
	for _, node := range snapshot.NodeInfoList {
		value := node.FieldValue
		if node.NodeID == "132" && node.FieldName == "value" {
			value = duration
			foundDuration = true
		}
		nodes = append(nodes, runningHubH3FallbackNodeInfo{
			NodeID:     node.NodeID,
			FieldName:  node.FieldName,
			FieldValue: value,
		})
	}
	if !foundDuration {
		return nil, fmt.Errorf("RunningHub H3 workflow snapshot is missing node 132 value")
	}

	var workflow map[string]any
	if err := common.Unmarshal([]byte(snapshot.Workflow), &workflow); err != nil {
		return nil, fmt.Errorf("decode RunningHub H3 workflow snapshot: %w", err)
	}
	workflowNode, ok := workflow["132"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("RunningHub H3 workflow snapshot is missing node 132")
	}
	inputs, ok := workflowNode["inputs"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("RunningHub H3 workflow snapshot is missing node 132 inputs")
	}
	if _, ok := inputs["value"]; !ok {
		return nil, fmt.Errorf("RunningHub H3 workflow snapshot is missing node 132 value")
	}
	inputs["value"] = duration
	serializedWorkflow, err := common.Marshal(workflow)
	if err != nil {
		return nil, err
	}

	return &runningHubH3FallbackCreateRequest{
		APIKey:       apiKey,
		WorkflowID:   snapshot.WorkflowID,
		NodeInfoList: nodes,
		Workflow:     string(serializedWorkflow),
	}, nil
}

// advanceRunningHubH3OOMFallback polls the hidden segment tasks, then joins
// their outputs into one local MP4 while preserving the original public task.
func advanceRunningHubH3OOMFallback(ctx context.Context, adaptor TaskPollingAdaptor, baseURL, key, proxy string, task *model.Task) (*relaycommon.TaskInfo, bool, error) {
	return advanceRunningHubH3OOMFallbackWithConcat(ctx, adaptor, baseURL, key, proxy, task, concatenateRunningHubH3Segments)
}

func advanceRunningHubH3OOMFallbackWithConcat(ctx context.Context, adaptor TaskPollingAdaptor, baseURL, key, proxy string, task *model.Task, concat func(context.Context, string, []string) (string, error)) (*relaycommon.TaskInfo, bool, error) {
	if task == nil || task.PrivateData.RunningHubH3Fallback == nil {
		return nil, false, nil
	}
	state := task.PrivateData.RunningHubH3Fallback
	if state.State == "" {
		return nil, false, nil
	}
	if state.State == runningHubH3FallbackStateCompleted && state.ResultFile != "" {
		return &relaycommon.TaskInfo{
			Status:   string(model.TaskStatusSuccess),
			Progress: taskcommon.ProgressComplete,
			Url:      taskcommon.BuildProxyURL(task.TaskID),
		}, true, nil
	}
	if state.State != runningHubH3FallbackStateRunning {
		return relaycommon.FailTaskInfo("RunningHub H3 OOM fallback entered an invalid state"), true, nil
	}
	if adaptor == nil || len(state.Segments) == 0 {
		state.State = "failed"
		return relaycommon.FailTaskInfo("RunningHub H3 OOM fallback has no segment tasks to poll"), true, nil
	}

	for index := range state.Segments {
		segment := &state.Segments[index]
		if segment.Status == model.TaskStatusSuccess && strings.TrimSpace(segment.ResultURL) != "" {
			continue
		}
		if strings.TrimSpace(segment.UpstreamTaskID) == "" {
			state.State = "failed"
			return relaycommon.FailTaskInfo(fmt.Sprintf("RunningHub H3 OOM fallback segment %d has no upstream task ID", index+1)), true, nil
		}

		resp, err := adaptor.FetchTask(baseURL, key, map[string]any{"task_id": segment.UpstreamTaskID}, proxy)
		if err != nil {
			return nil, true, fmt.Errorf("poll RunningHub H3 fallback segment %d: %w", index+1, err)
		}
		responseBody, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return nil, true, fmt.Errorf("read RunningHub H3 fallback segment %d: %w", index+1, readErr)
		}
		result, err := adaptor.ParseTaskResult(responseBody)
		if err != nil {
			return nil, true, fmt.Errorf("parse RunningHub H3 fallback segment %d: %w", index+1, err)
		}

		segment.Status = model.TaskStatus(result.Status)
		if result.Progress != "" {
			segment.Progress = result.Progress
		}
		if result.Status == string(model.TaskStatusSuccess) {
			if strings.TrimSpace(result.Url) == "" {
				state.State = "failed"
				return relaycommon.FailTaskInfo(fmt.Sprintf("RunningHub H3 OOM fallback segment %d completed without a video URL", index+1)), true, nil
			}
			segment.ResultURL = result.Url
			segment.Progress = taskcommon.ProgressComplete
			continue
		}
		if result.Status == string(model.TaskStatusFailure) {
			state.State = "failed"
			segment.FailReason = result.Reason
			return relaycommon.FailTaskInfo(fmt.Sprintf("RunningHub H3 OOM fallback segment %d failed: %s", index+1, result.Reason)), true, nil
		}
	}

	urls := make([]string, 0, len(state.Segments))
	for _, segment := range state.Segments {
		if segment.Status != model.TaskStatusSuccess || strings.TrimSpace(segment.ResultURL) == "" {
			return &relaycommon.TaskInfo{
				Status:   string(model.TaskStatusInProgress),
				Progress: runningHubH3FallbackProgress(state.Segments),
			}, true, nil
		}
		urls = append(urls, segment.ResultURL)
	}

	resultFile, err := concat(ctx, task.TaskID, urls)
	if err != nil {
		state.State = "failed"
		return relaycommon.FailTaskInfo("RunningHub H3 OOM fallback could not concatenate segments: " + err.Error()), true, nil
	}
	state.State = runningHubH3FallbackStateCompleted
	state.ResultFile = resultFile
	return &relaycommon.TaskInfo{
		Status:   string(model.TaskStatusSuccess),
		Progress: taskcommon.ProgressComplete,
		Url:      taskcommon.BuildProxyURL(task.TaskID),
	}, true, nil
}

func runningHubH3FallbackProgress(segments []model.RunningHubH3FallbackSegment) string {
	if len(segments) == 0 {
		return taskcommon.ProgressQueued
	}

	total := 0
	for _, segment := range segments {
		if segment.Status == model.TaskStatusSuccess {
			total += 100
			continue
		}
		progress := strings.TrimSuffix(strings.TrimSpace(segment.Progress), "%")
		value, err := strconv.Atoi(progress)
		if err == nil && value > 0 {
			if value > 99 {
				value = 99
			}
			total += value
		}
	}
	progress := 10 + total*85/(len(segments)*100)
	if progress < 10 {
		progress = 10
	}
	if progress > 95 {
		progress = 95
	}
	return strconv.Itoa(progress) + "%"
}

func concatenateRunningHubH3Segments(ctx context.Context, taskID string, urls []string) (string, error) {
	if len(urls) < 2 {
		return "", fmt.Errorf("at least two segment URLs are required")
	}
	resultFile, err := runningHubH3FallbackResultFileName(taskID)
	if err != nil {
		return "", err
	}
	mediaDir, err := runningHubH3FallbackMediaDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(mediaDir, 0700); err != nil {
		return "", fmt.Errorf("create RunningHub H3 media directory: %w", err)
	}

	workDir, err := os.MkdirTemp(mediaDir, ".concat-")
	if err != nil {
		return "", fmt.Errorf("create RunningHub H3 concat directory: %w", err)
	}
	defer os.RemoveAll(workDir)

	inputs := make([]string, 0, len(urls))
	for index, videoURL := range urls {
		inputPath := filepath.Join(workDir, fmt.Sprintf("segment-%02d.mp4", index+1))
		if err := downloadRunningHubH3FallbackSegment(ctx, videoURL, inputPath); err != nil {
			return "", fmt.Errorf("download segment %d: %w", index+1, err)
		}
		inputs = append(inputs, inputPath)
	}

	concatList := filepath.Join(workDir, "segments.txt")
	var lines strings.Builder
	for _, inputPath := range inputs {
		lines.WriteString("file '")
		lines.WriteString(strings.ReplaceAll(inputPath, "'", "'\\\\''"))
		lines.WriteString("'\n")
	}
	if err := os.WriteFile(concatList, []byte(lines.String()), 0600); err != nil {
		return "", fmt.Errorf("write RunningHub H3 concat list: %w", err)
	}

	outputPath := filepath.Join(mediaDir, resultFile)
	temporaryOutput, err := os.CreateTemp(mediaDir, ".concat-result-*.mp4")
	if err != nil {
		return "", fmt.Errorf("create RunningHub H3 output: %w", err)
	}
	temporaryOutputPath := temporaryOutput.Name()
	if err := temporaryOutput.Close(); err != nil {
		os.Remove(temporaryOutputPath)
		return "", err
	}
	defer os.Remove(temporaryOutputPath)

	commandContext, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	command := exec.CommandContext(commandContext, runningHubH3FallbackFFmpegPath(), "-hide_banner", "-loglevel", "error", "-f", "concat", "-safe", "0", "-i", concatList, "-c", "copy", "-movflags", "+faststart", "-y", temporaryOutputPath)
	output, err := command.CombinedOutput()
	if err != nil {
		if commandContext.Err() != nil {
			return "", fmt.Errorf("ffmpeg concat timed out: %w", commandContext.Err())
		}
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		return "", fmt.Errorf("ffmpeg concat failed: %s", message)
	}
	info, err := os.Stat(temporaryOutputPath)
	if err != nil || info.Size() == 0 {
		if err != nil {
			return "", fmt.Errorf("read concatenated video: %w", err)
		}
		return "", fmt.Errorf("ffmpeg produced an empty video")
	}
	if err := os.Rename(temporaryOutputPath, outputPath); err != nil {
		return "", fmt.Errorf("publish concatenated video: %w", err)
	}
	return resultFile, nil
}

func downloadRunningHubH3FallbackSegment(ctx context.Context, videoURL, outputPath string) error {
	if err := ValidateSSRFProtectedFetchURL(videoURL); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, videoURL, nil)
	if err != nil {
		return err
	}
	resp, err := GetSSRFProtectedHTTPClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("upstream returned %s", resp.Status)
	}
	if resp.ContentLength > runningHubH3FallbackMaxSegmentBytes {
		return fmt.Errorf("video exceeds the %d MB segment limit", runningHubH3FallbackMaxSegmentBytes>>20)
	}

	file, err := os.OpenFile(outputPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	written, err := io.Copy(file, io.LimitReader(resp.Body, runningHubH3FallbackMaxSegmentBytes+1))
	if err != nil {
		return err
	}
	if written > runningHubH3FallbackMaxSegmentBytes {
		return fmt.Errorf("video exceeds the %d MB segment limit", runningHubH3FallbackMaxSegmentBytes>>20)
	}
	if written == 0 {
		return fmt.Errorf("upstream returned an empty video")
	}
	return nil
}

func runningHubH3FallbackMediaDir() (string, error) {
	directory := strings.TrimSpace(os.Getenv(runningHubH3FallbackMediaDirEnv))
	if directory == "" {
		directory = "runninghub-h3"
	}
	return filepath.Abs(directory)
}

func runningHubH3FallbackFFmpegPath() string {
	if value := strings.TrimSpace(os.Getenv(runningHubH3FallbackFFmpegEnv)); value != "" {
		return value
	}
	return "ffmpeg"
}

func runningHubH3FallbackResultFileName(taskID string) (string, error) {
	if strings.TrimSpace(taskID) == "" || strings.ContainsAny(taskID, "/\\") || taskID != filepath.Base(taskID) {
		return "", fmt.Errorf("invalid RunningHub H3 task ID")
	}
	for _, character := range taskID {
		if (character < 'a' || character > 'z') && (character < 'A' || character > 'Z') && (character < '0' || character > '9') && character != '_' && character != '-' {
			return "", fmt.Errorf("invalid RunningHub H3 task ID")
		}
	}
	return taskID + ".mp4", nil
}

// OpenRunningHubH3FallbackResult opens the deterministic local result file for
// a completed task. The result file is never treated as a caller-provided path.
func OpenRunningHubH3FallbackResult(taskID, resultFile string) (*os.File, os.FileInfo, error) {
	expectedFile, err := runningHubH3FallbackResultFileName(taskID)
	if err != nil {
		return nil, nil, err
	}
	if resultFile != expectedFile {
		return nil, nil, fmt.Errorf("invalid RunningHub H3 result file")
	}
	mediaDir, err := runningHubH3FallbackMediaDir()
	if err != nil {
		return nil, nil, err
	}
	path := filepath.Join(mediaDir, expectedFile)
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, nil, err
	}
	if !info.Mode().IsRegular() {
		file.Close()
		return nil, nil, fmt.Errorf("RunningHub H3 result is not a regular file")
	}
	return file, info, nil
}
