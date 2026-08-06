package runninghub

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	taskdto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	openaidto "github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
)

const (
	channelName       = "runninghub"
	modelName         = "minimax_h3"
	createPath        = "/task/openapi/create"
	queryPath         = "/openapi/v2/query"
	uploadPath        = "/openapi/v2/media/upload/binary"
	defaultAspect     = "9:16 (Portrait Widescreen)"
	defaultMegapixels = 1
	defaultMultiple   = 32
	maxImages         = 9
	maxAudios         = 3
	maxAspectRatioGap = 0.06
)

var imageNodeIDs = []string{"137", "618", "617", "619", "627", "626", "625", "624", "623"}
var audioNodeIDs = []string{"628", "630", "629"}

type TaskAdaptor struct {
	taskcommon.BaseBilling
	baseURL    string
	workflowID string
	proxy      string
}

type nodeInfo struct {
	NodeID     string `json:"nodeId"`
	FieldName  string `json:"fieldName"`
	FieldValue any    `json:"fieldValue"`
}

type createRequest struct {
	APIKey       string     `json:"apiKey"`
	WorkflowID   string     `json:"workflowId"`
	NodeInfoList []nodeInfo `json:"nodeInfoList"`
}

type createResponse struct {
	Code    int    `json:"code"`
	Msg     string `json:"msg"`
	Message string `json:"message"`
	Data    struct {
		TaskID string `json:"taskId"`
	} `json:"data"`
}

type uploadResponse struct {
	Code    int    `json:"code"`
	Msg     string `json:"msg"`
	Message string `json:"message"`
	Data    struct {
		FileName string `json:"fileName"`
		Filename string `json:"filename"`
	} `json:"data"`
}

type h3Selector struct {
	AspectRatio string
	Megapixels  any
	Multiple    any
}

type h3MegapixelPreset struct {
	Value        float64
	OutputPixels int
}

type h3AspectPreset struct {
	Ratio string
	Value float64
}

// H3 exposes these megapixel choices with multiple=32. Use their documented
// 16:9 output areas so standard downstream resolutions choose the same preset.
var h3MegapixelPresets = []h3MegapixelPreset{
	{Value: 0.2, OutputPixels: 608 * 352},
	{Value: 0.3, OutputPixels: 736 * 416},
	{Value: 0.4, OutputPixels: 864 * 480},
	{Value: 0.5, OutputPixels: 960 * 544},
	{Value: 0.6, OutputPixels: 1056 * 608},
	{Value: 0.7, OutputPixels: 1152 * 640},
	{Value: 0.8, OutputPixels: 1216 * 672},
	{Value: 0.9, OutputPixels: 1280 * 736},
	{Value: 0.98, OutputPixels: 1344 * 768},
	{Value: 1.0, OutputPixels: 1376 * 768},
	{Value: 1.2, OutputPixels: 1504 * 832},
	{Value: 1.5, OutputPixels: 1664 * 928},
	{Value: 1.8, OutputPixels: 1824 * 1024},
	{Value: 2.0, OutputPixels: 1920 * 1080},
}

var h3AspectPresets = []h3AspectPreset{
	{Ratio: "9:16 (Portrait Widescreen)", Value: 9.0 / 16.0},
	{Ratio: "16:9 (Landscape Widescreen)", Value: 16.0 / 9.0},
	{Ratio: "1:1 (Square)", Value: 1.0},
}

type referenceInput struct {
	Name string
	Data []byte
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.baseURL = strings.TrimRight(info.ChannelBaseUrl, "/")
	a.workflowID = strings.TrimSpace(info.ChannelOtherSettings.RunningHubWorkflowID)
	a.proxy = info.ChannelSetting.Proxy
}

func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) *taskdto.TaskError {
	if taskErr := relaycommon.ValidateBasicTaskRequest(c, info, constant.TaskActionGenerate); taskErr != nil {
		return taskErr
	}
	if err := validateMultipartFields(c); err != nil {
		return service.TaskErrorWrapperLocal(err, "unsupported_multipart_field", http.StatusBadRequest)
	}
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	if err := a.validateRequestInput(c, req); err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	return nil
}

func (a *TaskAdaptor) validateRequestInput(c *gin.Context, req relaycommon.TaskSubmitReq) error {
	if strings.TrimSpace(a.workflowID) == "" {
		return fmt.Errorf("runninghub workflow id is not configured")
	}
	if _, err := selectorFromRequest(req); err != nil {
		return err
	}
	_, _, _, _, err := referenceInputs(c, req)
	return err
}

func (a *TaskAdaptor) EstimateBilling(c *gin.Context, _ *relaycommon.RelayInfo) map[string]float64 {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil
	}
	return map[string]float64{"seconds": float64(requestSeconds(req))}
}

func (a *TaskAdaptor) BuildRequestURL(_ *relaycommon.RelayInfo) (string, error) {
	return a.baseURL + createPath, nil
}

func (a *TaskAdaptor) BuildRequestHeader(_ *gin.Context, req *http.Request, info *relaycommon.RelayInfo) error {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+info.ApiKey)
	return nil
}

func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil, err
	}
	body, err := a.convertRequest(c, req, info.ApiKey)
	if err != nil {
		return nil, err
	}
	data, err := common.Marshal(body)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}

func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (string, []byte, *taskdto.TaskError) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
	}
	var result createResponse
	if err := common.Unmarshal(body, &result); err != nil {
		return "", nil, service.TaskErrorWrapper(errors.Wrapf(err, "body: %s", body), "unmarshal_response_body_failed", http.StatusInternalServerError)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices || !isRunningHubSuccessCode(result.Code) {
		message := runningHubResponseMessage(result.Msg, result.Message)
		if message == "" {
			message = resp.Status
		}
		return "", nil, service.TaskErrorWrapper(fmt.Errorf("runninghub create failed: %s", message), strconv.Itoa(result.Code), resp.StatusCode)
	}
	if strings.TrimSpace(result.Data.TaskID) == "" {
		return "", nil, service.TaskErrorWrapper(fmt.Errorf("runninghub create response missing taskId"), "invalid_response", http.StatusInternalServerError)
	}

	publicTaskID := result.Data.TaskID
	modelForClient := modelName
	if info != nil {
		if info.TaskRelayInfo != nil && strings.TrimSpace(info.PublicTaskID) != "" {
			publicTaskID = info.PublicTaskID
		}
		if strings.TrimSpace(info.OriginModelName) != "" {
			modelForClient = info.OriginModelName
		}
	}

	video := openaidto.NewOpenAIVideo()
	video.ID = publicTaskID
	video.TaskID = publicTaskID
	video.Model = modelForClient
	video.Status = openaidto.VideoStatusQueued
	video.CreatedAt = time.Now().Unix()
	c.JSON(http.StatusOK, video)
	return result.Data.TaskID, body, nil
}

func (a *TaskAdaptor) FetchTask(baseURL, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, _ := body["taskId"].(string)
	if taskID == "" {
		taskID, _ = body["task_id"].(string)
	}
	payload, err := common.Marshal(map[string]string{"taskId": taskID})
	if err != nil {
		return nil, err
	}
	requestURL := strings.TrimRight(baseURL, "/") + queryPath
	req, err := http.NewRequest(http.MethodPost, requestURL, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)
	client, err := service.GetHttpClientWithProxy(strings.TrimSpace(proxy))
	if err != nil {
		return nil, err
	}
	return client.Do(req)
}

func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	var top map[string]any
	if err := common.Unmarshal(respBody, &top); err != nil {
		return nil, errors.Wrap(err, "unmarshal runninghub task result failed")
	}

	code := intFromAny(top["code"])
	message := firstString(top, "msg", "message", "errorMessage", "error_msg")
	data := top
	if nested, ok := top["data"].(map[string]any); ok {
		data = nested
		if message == "" {
			message = firstString(nested, "msg", "message", "errorMessage", "error_msg")
		}
	}

	taskInfo := &relaycommon.TaskInfo{Code: code}
	if !isRunningHubSuccessCode(code) {
		taskInfo.Status = string(model.TaskStatusFailure)
		taskInfo.Reason = message
		taskInfo.Progress = "100%"
		return taskInfo, nil
	}

	status := firstString(data, "status", "taskStatus", "task_status", "state")
	taskStatus, progress := convertStatus(status)
	taskInfo.Status = string(taskStatus)
	taskInfo.Progress = progress
	if taskStatus == model.TaskStatusFailure {
		taskInfo.Reason = firstString(data, "error", "errorCode", "error_code", "errorMsg", "error_msg", "errorMessage", "msg", "message", "failReason")
		if taskInfo.Reason == "" {
			taskInfo.Reason = message
		}
	}
	if taskStatus == model.TaskStatusSuccess {
		taskInfo.Url = selectResultURL(data)
	}
	return taskInfo, nil
}

func (a *TaskAdaptor) GetModelList() []string { return []string{modelName} }

func (a *TaskAdaptor) GetChannelName() string { return channelName }

func (a *TaskAdaptor) ConvertToOpenAIVideo(task *model.Task) ([]byte, error) {
	video := task.ToOpenAIVideo()
	video.Model = modelName
	if task.Status == model.TaskStatusFailure {
		message := task.FailReason
		if message == "" {
			message = "task failed"
		}
		video.Error = &openaidto.OpenAIVideoError{Message: message, Code: "task_failed"}
	}
	return common.Marshal(video)
}

func (a *TaskAdaptor) convertRequest(c *gin.Context, req relaycommon.TaskSubmitReq, apiKey string) (*createRequest, error) {
	workflowID := strings.TrimSpace(a.workflowID)
	if workflowID == "" {
		return nil, fmt.Errorf("runninghub workflow id is not configured")
	}
	selector, err := selectorFromRequest(req)
	if err != nil {
		return nil, err
	}
	images, audios, err := a.collectReferences(c, req, apiKey)
	if err != nil {
		return nil, err
	}

	nodes := []nodeInfo{
		{NodeID: "138", FieldName: "value", FieldValue: req.Prompt},
		{NodeID: "132", FieldName: "value", FieldValue: requestSeconds(req)},
		{NodeID: "115", FieldName: "aspect_ratio", FieldValue: selector.AspectRatio},
		{NodeID: "115", FieldName: "megapixels", FieldValue: selector.Megapixels},
		{NodeID: "115", FieldName: "multiple", FieldValue: selector.Multiple},
	}
	for i, image := range images {
		nodes = append(nodes, nodeInfo{NodeID: imageNodeIDs[i], FieldName: "image", FieldValue: image})
	}
	for i, audio := range audios {
		nodes = append(nodes, nodeInfo{NodeID: audioNodeIDs[i], FieldName: "audio", FieldValue: audio})
	}
	return &createRequest{APIKey: apiKey, WorkflowID: workflowID, NodeInfoList: nodes}, nil
}

func (a *TaskAdaptor) collectReferences(c *gin.Context, req relaycommon.TaskSubmitReq, apiKey string) ([]string, []string, error) {
	imageValues, audioValues, imageFiles, audioFiles, err := referenceInputs(c, req)
	if err != nil {
		return nil, nil, err
	}
	for _, fileHeader := range imageFiles {
		input, err := readMultipartFile(fileHeader)
		if err != nil {
			return nil, nil, err
		}
		fileName, err := a.uploadReference(apiKey, input)
		if err != nil {
			return nil, nil, err
		}
		imageValues = append(imageValues, fileName)
	}
	for _, fileHeader := range audioFiles {
		input, err := readMultipartFile(fileHeader)
		if err != nil {
			return nil, nil, err
		}
		fileName, err := a.uploadReference(apiKey, input)
		if err != nil {
			return nil, nil, err
		}
		audioValues = append(audioValues, fileName)
	}

	images, err := a.resolveReferenceValues(apiKey, imageValues)
	if err != nil {
		return nil, nil, err
	}
	audios, err := a.resolveReferenceValues(apiKey, audioValues)
	if err != nil {
		return nil, nil, err
	}
	return images, audios, nil
}

func referenceInputs(c *gin.Context, req relaycommon.TaskSubmitReq) ([]string, []string, []*multipart.FileHeader, []*multipart.FileHeader, error) {
	if hasMetadataValue(req.Metadata, "reference_video") || hasMetadataValue(req.Metadata, "reference_videos") {
		return nil, nil, nil, nil, fmt.Errorf("runninghub does not support reference video inputs")
	}

	// Validate the complete request before uploading any files. The common task
	// parser normalizes input_reference/image into Images, so de-duplicate these
	// equivalent representations while preserving their first supplied order.
	imageValues := appendDistinctNonEmpty(nil, req.InputReference, req.Image)
	imageValues = appendDistinctStrings(imageValues, req.Images...)
	imageValues = appendDistinctStrings(imageValues, metadataStrings(req.Metadata, "input_reference")...)
	imageValues = appendDistinctStrings(imageValues, metadataStrings(req.Metadata, "reference_images")...)
	audioValues := appendDistinctStrings(nil, metadataStrings(req.Metadata, "reference_audio")...)
	audioValues = appendDistinctStrings(audioValues, metadataStrings(req.Metadata, "reference_audios")...)

	var imageFiles, audioFiles []*multipart.FileHeader
	if c != nil && c.Request != nil && c.Request.MultipartForm != nil {
		form := c.Request.MultipartForm
		imageFiles = multipartFiles(form, "input_reference", "image", "images", "reference_images")
		audioFiles = multipartFiles(form, "reference_audio", "reference_audios")
	}
	if len(imageValues)+len(imageFiles) > maxImages {
		return nil, nil, nil, nil, fmt.Errorf("runninghub supports at most %d reference images", maxImages)
	}
	if len(audioValues)+len(audioFiles) > maxAudios {
		return nil, nil, nil, nil, fmt.Errorf("runninghub supports at most %d reference audio files", maxAudios)
	}
	return imageValues, audioValues, imageFiles, audioFiles, nil
}

func (a *TaskAdaptor) resolveReferenceValues(apiKey string, values []string) ([]string, error) {
	resolved := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if isHTTPURL(value) {
			input, err := downloadReference(value)
			if err != nil {
				return nil, err
			}
			fileName, err := a.uploadReference(apiKey, input)
			if err != nil {
				return nil, err
			}
			resolved = append(resolved, fileName)
			continue
		}
		resolved = append(resolved, value)
	}
	return resolved, nil
}

func (a *TaskAdaptor) uploadReference(apiKey string, input referenceInput) (string, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", input.Name)
	if err != nil {
		return "", err
	}
	if _, err := part.Write(input.Data); err != nil {
		return "", err
	}
	if err := writer.Close(); err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodPost, a.baseURL+uploadPath, &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Accept", "application/json")
	client, err := service.GetHttpClientWithProxy(strings.TrimSpace(a.proxy))
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var result uploadResponse
	if err := common.Unmarshal(respBody, &result); err != nil {
		return "", errors.Wrap(err, "unmarshal runninghub upload response failed")
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices || !isRunningHubSuccessCode(result.Code) {
		message := runningHubResponseMessage(result.Msg, result.Message)
		if message == "" {
			message = resp.Status
		}
		return "", fmt.Errorf("runninghub upload failed: %s", message)
	}
	fileName := strings.TrimSpace(result.Data.FileName)
	if fileName == "" {
		fileName = strings.TrimSpace(result.Data.Filename)
	}
	if fileName == "" {
		return "", fmt.Errorf("runninghub upload response missing fileName")
	}
	return fileName, nil
}

func isRunningHubSuccessCode(code int) bool {
	return code == 0 || code == http.StatusOK
}

func runningHubResponseMessage(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func selectorFromRequest(req relaycommon.TaskSubmitReq) (h3Selector, error) {
	selector := h3Selector{AspectRatio: defaultAspect, Megapixels: defaultMegapixels, Multiple: defaultMultiple}
	if req.Size != "" {
		aspect, megapixels, err := h3ParametersFromSize(req.Size)
		if err != nil {
			return selector, err
		}
		selector.AspectRatio = aspect
		selector.Megapixels = megapixels
	}
	if v := metadataString(req.Metadata, "resolution"); v != "" {
		if megapixels, err := h3MegapixelsFromValue(v); err == nil {
			selector.Megapixels = megapixels
		} else {
			aspect, megapixels, sizeErr := h3ParametersFromSize(v)
			if sizeErr != nil {
				return selector, err
			}
			selector.AspectRatio = aspect
			selector.Megapixels = megapixels
		}
	}
	if v := metadataString(req.Metadata, "clarity"); v != "" {
		if megapixels, err := h3MegapixelsFromValue(v); err == nil {
			selector.Megapixels = megapixels
		} else {
			aspect, megapixels, sizeErr := h3ParametersFromSize(v)
			if sizeErr != nil {
				return selector, err
			}
			selector.AspectRatio = aspect
			selector.Megapixels = megapixels
		}
	}
	if v := metadataString(req.Metadata, "aspect_ratio"); v != "" {
		aspect, err := normalizeAspectRatio(v)
		if err != nil {
			return selector, err
		}
		selector.AspectRatio = aspect
	}
	if v := metadataString(req.Metadata, "megapixels"); v != "" {
		megapixels, err := h3MegapixelsFromValue(v)
		if err != nil {
			return selector, err
		}
		selector.Megapixels = megapixels
	}
	if v := metadataString(req.Metadata, "multiple"); v != "" {
		selector.Multiple = v
	}
	return selector, nil
}

func h3ParametersFromSize(size string) (string, any, error) {
	s := strings.ToLower(strings.TrimSpace(size))
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "*", "x")
	switch s {
	case "", "auto":
		return defaultAspect, defaultMegapixels, nil
	case "9:16", "portrait":
		return "9:16 (Portrait Widescreen)", defaultMegapixels, nil
	case "16:9", "landscape":
		return "16:9 (Landscape Widescreen)", defaultMegapixels, nil
	case "1:1", "square":
		return "1:1 (Square)", defaultMegapixels, nil
	}
	if strings.HasSuffix(s, "p") {
		height, err := strconv.Atoi(strings.TrimSuffix(s, "p"))
		if err == nil && height >= 352 && height <= 1080 {
			width := int(math.Round(float64(height) * 16.0 / 9.0))
			return "16:9 (Landscape Widescreen)", nearestH3Megapixels(width, height), nil
		}
	}

	width, height, ok := parseSizeDimensions(s)
	if !ok {
		return "", nil, fmt.Errorf("unsupported runninghub size %q; supported aspect ratios are 9:16, 16:9, and 1:1", size)
	}
	aspect, err := aspectRatioFromDimensions(width, height)
	if err != nil {
		return "", nil, fmt.Errorf("unsupported runninghub size %q; supported aspect ratios are 9:16, 16:9, and 1:1", size)
	}
	return aspect, nearestH3Megapixels(width, height), nil
}

func parseSizeDimensions(size string) (int, int, bool) {
	parts := strings.Split(size, "x")
	if len(parts) != 2 {
		return 0, 0, false
	}
	width, err := strconv.Atoi(parts[0])
	if err != nil || width <= 0 {
		return 0, 0, false
	}
	height, err := strconv.Atoi(parts[1])
	if err != nil || height <= 0 {
		return 0, 0, false
	}
	return width, height, true
}

func aspectRatioFromDimensions(width, height int) (string, error) {
	actual := float64(width) / float64(height)
	best := h3AspectPresets[0]
	bestGap := math.Abs(actual-best.Value) / best.Value
	for _, candidate := range h3AspectPresets[1:] {
		gap := math.Abs(actual-candidate.Value) / candidate.Value
		if gap < bestGap {
			best = candidate
			bestGap = gap
		}
	}
	if bestGap > maxAspectRatioGap {
		return "", fmt.Errorf("unsupported aspect ratio")
	}
	return best.Ratio, nil
}

func nearestH3Megapixels(width, height int) float64 {
	targetPixels := float64(width) * float64(height)
	best := h3MegapixelPresets[0]
	bestGap := math.Abs(targetPixels - float64(best.OutputPixels))
	for _, candidate := range h3MegapixelPresets[1:] {
		gap := math.Abs(targetPixels - float64(candidate.OutputPixels))
		if gap < bestGap {
			best = candidate
			bestGap = gap
		}
	}
	return best.Value
}

func h3MegapixelsFromValue(value string) (float64, error) {
	value = strings.TrimSpace(value)
	megapixels, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("unsupported runninghub megapixels %q", value)
	}
	for _, preset := range h3MegapixelPresets {
		if math.Abs(megapixels-preset.Value) < 0.000001 {
			return preset.Value, nil
		}
	}
	return 0, fmt.Errorf("unsupported runninghub megapixels %q; supported values are 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 0.98, 1.0, 1.2, 1.5, 1.8, and 2.0", value)
}

func normalizeAspectRatio(value string) (string, error) {
	s := strings.ToLower(strings.TrimSpace(value))
	switch s {
	case "9:16", "9:16 (portrait widescreen)", "portrait":
		return "9:16 (Portrait Widescreen)", nil
	case "16:9", "16:9 (landscape widescreen)", "landscape":
		return "16:9 (Landscape Widescreen)", nil
	case "1:1", "1:1 (square)", "square":
		return "1:1 (Square)", nil
	default:
		return "", fmt.Errorf("unsupported runninghub aspect_ratio %q; supported values are 9:16, 16:9, and 1:1", value)
	}
}

func requestSeconds(req relaycommon.TaskSubmitReq) int {
	if req.Duration > 0 {
		return req.Duration
	}
	if req.Seconds != "" {
		if seconds, err := strconv.Atoi(req.Seconds); err == nil && seconds > 0 {
			return seconds
		}
	}
	return 5
}

func validateMultipartFields(c *gin.Context) error {
	if c == nil || c.Request == nil {
		return nil
	}
	form := c.Request.MultipartForm
	if form == nil {
		return nil
	}
	allowed := map[string]bool{
		"input_reference":  true,
		"image":            true,
		"images":           true,
		"reference_images": true,
		"reference_audio":  true,
		"reference_audios": true,
	}
	for field, files := range form.File {
		if len(files) == 0 || allowed[field] {
			continue
		}
		if strings.Contains(field, "video") {
			return fmt.Errorf("runninghub does not support reference video file field %q", field)
		}
		return fmt.Errorf("runninghub does not support multipart file field %q", field)
	}
	return nil
}

func maxReferenceMB() int {
	if constant.MaxFileDownloadMB > 0 {
		return constant.MaxFileDownloadMB
	}
	return 64
}

func maxReferenceBytes() int64 {
	return int64(maxReferenceMB()) * 1024 * 1024
}

func readMultipartFile(fileHeader *multipart.FileHeader) (referenceInput, error) {
	if fileHeader == nil {
		return referenceInput{}, fmt.Errorf("reference file is missing")
	}
	file, err := fileHeader.Open()
	if err != nil {
		return referenceInput{}, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxReferenceBytes()+1))
	if err != nil {
		return referenceInput{}, err
	}
	if int64(len(data)) > maxReferenceBytes() {
		return referenceInput{}, fmt.Errorf("file %s exceeds max size %d MB", fileHeader.Filename, maxReferenceMB())
	}
	return referenceInput{Name: safeFileName(fileHeader.Filename), Data: data}, nil
}

func downloadReference(rawURL string) (referenceInput, error) {
	if err := service.ValidateSSRFProtectedFetchURL(rawURL); err != nil {
		return referenceInput{}, err
	}
	resp, err := service.GetSSRFProtectedHTTPClient().Get(rawURL)
	if err != nil {
		return referenceInput{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return referenceInput{}, fmt.Errorf("download reference failed with status %s", resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxReferenceBytes()+1))
	if err != nil {
		return referenceInput{}, err
	}
	if int64(len(data)) > maxReferenceBytes() {
		return referenceInput{}, fmt.Errorf("reference file exceeds max size %d MB", maxReferenceMB())
	}
	parsed, _ := url.Parse(rawURL)
	return referenceInput{Name: safeFileName(filepath.Base(parsed.Path)), Data: data}, nil
}

func safeFileName(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	if name == "." || name == "/" || name == "" {
		return "reference.bin"
	}
	return name
}

func isHTTPURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}

func appendDistinctNonEmpty(values []string, candidates ...string) []string {
	for _, candidate := range candidates {
		if candidate = strings.TrimSpace(candidate); candidate != "" {
			values = appendDistinctStrings(values, candidate)
		}
	}
	return values
}

func appendDistinctStrings(values []string, candidates ...string) []string {
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		duplicate := false
		for _, value := range values {
			if strings.TrimSpace(value) == candidate {
				duplicate = true
				break
			}
		}
		if !duplicate {
			values = append(values, candidate)
		}
	}
	return values
}

func multipartFiles(form *multipart.Form, fields ...string) []*multipart.FileHeader {
	if form == nil {
		return nil
	}
	files := make([]*multipart.FileHeader, 0)
	for _, field := range fields {
		files = append(files, form.File[field]...)
	}
	return files
}

func metadataString(metadata map[string]any, key string) string {
	values := metadataStrings(metadata, key)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func metadataStrings(metadata map[string]any, key string) []string {
	if metadata == nil {
		return nil
	}
	value, ok := metadata[key]
	if !ok || value == nil {
		return nil
	}
	switch typed := value.(type) {
	case string:
		if strings.TrimSpace(typed) == "" {
			return nil
		}
		return []string{typed}
	case []string:
		return typed
	case []any:
		values := make([]string, 0, len(typed))
		for _, item := range typed {
			if s := strings.TrimSpace(fmt.Sprint(item)); s != "" {
				values = append(values, s)
			}
		}
		return values
	default:
		if s := strings.TrimSpace(fmt.Sprint(typed)); s != "" {
			return []string{s}
		}
		return nil
	}
}

func hasMetadataValue(metadata map[string]any, key string) bool {
	return len(metadataStrings(metadata, key)) > 0
}

func firstString(data map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := data[key]; ok && value != nil {
			if text := strings.TrimSpace(fmt.Sprint(value)); text != "" && text != "<nil>" {
				return text
			}
		}
	}
	return ""
}

func convertStatus(status string) (model.TaskStatus, string) {
	s := strings.ToUpper(strings.TrimSpace(status))
	s = strings.ReplaceAll(s, "-", "_")
	s = strings.ReplaceAll(s, " ", "_")
	switch s {
	case "", "CREATED", "CREATE", "WAITING", "PENDING", "QUEUED", "IN_QUEUE":
		return model.TaskStatusQueued, "10%"
	case "RUNNING", "PROCESSING", "IN_PROGRESS", "EXECUTING":
		return model.TaskStatusInProgress, "50%"
	case "SUCCESS", "SUCCEEDED", "FINISHED", "COMPLETED", "COMPLETE":
		return model.TaskStatusSuccess, "100%"
	case "FAIL", "FAILED", "FAILURE", "ERROR", "CANCELED", "CANCELLED":
		return model.TaskStatusFailure, "100%"
	default:
		return model.TaskStatusInProgress, "30%"
	}
}

func intFromAny(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case string:
		parsed, _ := strconv.Atoi(strings.TrimSpace(typed))
		return parsed
	default:
		return 0
	}
}

func selectResultURL(data any) string {
	if dataMap, ok := data.(map[string]any); ok {
		if results, ok := dataMap["results"].([]any); ok {
			for _, item := range results {
				result, ok := item.(map[string]any)
				if !ok {
					continue
				}
				if isVideoOutputType(firstString(result, "outputType", "output_type", "fileType", "file_type", "type")) {
					if resultURL := firstString(result, "url", "fileUrl", "file_url", "downloadUrl", "download_url"); isHTTPURL(resultURL) {
						return resultURL
					}
				}
			}
		}
	}
	all := collectURLs(data)
	for _, candidate := range all {
		lower := strings.ToLower(candidate)
		if strings.Contains(lower, ".mp4") || strings.Contains(lower, ".mov") || strings.Contains(lower, ".webm") || strings.Contains(lower, "video") {
			return candidate
		}
	}
	if len(all) > 0 {
		return all[0]
	}
	return ""
}

func isVideoOutputType(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "video", "mp4", "mov", "webm", "mpeg", "mpg", "mkv", "avi", "video/mp4", "video/quicktime", "video/webm", "video/mpeg":
		return true
	default:
		return false
	}
}

func collectURLs(value any) []string {
	urls := make([]string, 0)
	var walk func(any)
	walk = func(current any) {
		switch typed := current.(type) {
		case string:
			if isHTTPURL(typed) {
				urls = append(urls, typed)
			}
		case []any:
			for _, item := range typed {
				walk(item)
			}
		case map[string]any:
			preferredKeys := []string{"video", "videoUrl", "video_url", "url", "fileUrl", "file_url", "downloadUrl", "download_url"}
			for _, key := range preferredKeys {
				if item, ok := typed[key]; ok {
					walk(item)
				}
			}
			for key, item := range typed {
				skip := false
				for _, preferred := range preferredKeys {
					if key == preferred {
						skip = true
						break
					}
				}
				if !skip {
					walk(item)
				}
			}
		}
	}
	walk(value)
	return urls
}
