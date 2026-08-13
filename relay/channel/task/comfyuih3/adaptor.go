package comfyuih3

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"net/url"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	taskdto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	openaidto "github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	hosttypes "github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

const (
	channelName             = "comfyui_h3"
	modelName               = "minimax_h3"
	promptPath              = "/prompt"
	historyPath             = "/history/"
	uploadPath              = "/upload/image"
	h3NodeID                = "167"
	paramsNodeID            = "205"
	promptNodeID            = "209"
	outputNodeID            = "210"
	memoryProfileNodeID     = "227"
	defaultAspect           = "9:16 (Portrait Widescreen)"
	defaultMegapixels       = 1.0
	defaultMultiple         = 32
	maxImages               = 9
	maxVideos               = 3
	maxVideoAudios          = 3
	maxAudios               = 3
	minMultiple             = 8
	maxMultiple             = 128
	multipleStep            = 4
	maxDurationSeconds      = 300
	maxAspectRatioGap       = 0.06
	maxComfyErrorReasonSize = 1000
	workerQueueTimeout      = 3 * time.Second
	workerReservationTTL    = 30 * time.Second
)

//go:embed workflow.json
var workflowTemplate []byte

var h3SupportedAspectLabels = []string{
	"1:1 (Square)",
	"2:3 (Portrait Photo)",
	"3:2 (Photo)",
	"3:4 (Portrait Standard)",
	"4:3 (Standard)",
	"9:16 (Portrait Widescreen)",
	"16:9 (Widescreen)",
	"21:9 (Ultrawide)",
}

type TaskAdaptor struct {
	taskcommon.BaseBilling
	baseURL            string
	proxy              string
	workerSelectionErr error
	workerReserved     bool
}

type h3WorkerReservation struct {
	count     int
	expiresAt time.Time
}

var h3WorkerReservations = struct {
	sync.Mutex
	byURL map[string]h3WorkerReservation
}{byURL: make(map[string]h3WorkerReservation)}

type comfyPromptRequest struct {
	Prompt   map[string]any `json:"prompt"`
	ClientID string         `json:"client_id"`
}

type comfyPromptResponse struct {
	PromptID   string         `json:"prompt_id"`
	Error      any            `json:"error"`
	NodeErrors map[string]any `json:"node_errors"`
}

type h3Selector struct {
	AspectRatio string
	Megapixels  float64
	Multiple    int
}

type h3MegapixelPreset struct {
	Value        float64
	OutputPixels int
}

type h3AspectPreset struct {
	Ratio string
	Value float64
}

type h3GroupPrice struct {
	Price768P  float64
	Price2K    float64
	BoundGroup string
}

// RequiredNodeChoice describes a static workflow value which must be offered
// by the matching ComfyUI node before this channel can accept video requests.
type RequiredNodeChoice struct {
	NodeType  string
	InputName string
	Value     string
}

type referenceInput struct {
	Name string
	Data []byte
}

type referenceSources struct {
	images          []string
	videos          []string
	videoAudios     []string
	audios          []string
	imageFiles      []*multipart.FileHeader
	videoFiles      []*multipart.FileHeader
	videoAudioFiles []*multipart.FileHeader
	audioFiles      []*multipart.FileHeader
}

type references struct {
	Images      []string
	Videos      []string
	VideoAudios []string
	Audios      []string
}

// H3 exposes these megapixel choices with multiple=32. Use their documented
// 16:9 output areas so standard downstream resolutions select the closest
// workflow preset.
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
	{Value: 2.0, OutputPixels: 1920 * 1088},
}

var h3AspectPresets = []h3AspectPreset{
	{Ratio: "1:1 (Square)", Value: 1.0},
	{Ratio: "2:3 (Portrait Photo)", Value: 2.0 / 3.0},
	{Ratio: "3:2 (Photo)", Value: 3.0 / 2.0},
	{Ratio: "3:4 (Portrait Standard)", Value: 3.0 / 4.0},
	{Ratio: "4:3 (Standard)", Value: 4.0 / 3.0},
	{Ratio: "9:16 (Portrait Widescreen)", Value: 9.0 / 16.0},
	{Ratio: "16:9 (Widescreen)", Value: 16.0 / 9.0},
	{Ratio: "21:9 (Ultrawide)", Value: 21.0 / 9.0},
}

var lookupRunningHubH3GroupPrice = func(group string) (h3GroupPrice, bool) {
	price, ok := ratio_setting.GetRunningHubH3GroupPrice(group)
	if !ok {
		return h3GroupPrice{}, false
	}
	return h3GroupPrice{Price768P: price.Price768P, Price2K: price.Price2K, BoundGroup: price.BoundGroup}, true
}

// selectH3Worker chooses one equivalent GPU-bound ComfyUI process within a
// single NewAPI channel. The selected worker is reserved briefly while the
// prompt is being uploaded/submitted so concurrent requests do not pile onto
// the same apparently idle GPU.
func selectH3Worker(configuredURLs []string, fallbackURL, proxy string) (string, error) {
	workers := h3WorkerURLs(configuredURLs, fallbackURL)
	if len(workers) == 0 {
		return "", fmt.Errorf("ComfyUI H3 channel has no valid worker URL")
	}

	type candidate struct {
		url   string
		load  int
		index int
	}
	type probeResult struct {
		candidate candidate
		err       error
	}
	results := make(chan probeResult, len(workers))
	for index, workerURL := range workers {
		go func(index int, workerURL string) {
			queueLoad, err := h3WorkerQueueLoad(workerURL, proxy)
			results <- probeResult{candidate: candidate{url: workerURL, load: queueLoad, index: index}, err: err}
		}(index, workerURL)
	}
	candidates := make([]candidate, 0, len(workers))
	for range workers {
		result := <-results
		if result.err != nil {
			continue
		}
		candidates = append(candidates, result.candidate)
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf("all configured ComfyUI H3 workers are unavailable")
	}

	// Probe results are a snapshot. Choose and reserve under one lock so a burst
	// of requests sees prior selections immediately instead of all choosing the
	// first idle worker before any /prompt response is visible in /queue.
	h3WorkerReservations.Lock()
	defer h3WorkerReservations.Unlock()
	cleanupH3WorkerReservationsLocked(time.Now())
	for index := range candidates {
		candidates[index].load += h3WorkerReservations.byURL[candidates[index].url].count
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].load == candidates[j].load {
			return candidates[i].index < candidates[j].index
		}
		return candidates[i].load < candidates[j].load
	})
	selected := candidates[0].url
	reservation := h3WorkerReservations.byURL[selected]
	reservation.count++
	reservation.expiresAt = time.Now().Add(workerReservationTTL)
	h3WorkerReservations.byURL[selected] = reservation
	return selected, nil
}

func h3WorkerURLs(configuredURLs []string, fallbackURL string) []string {
	seen := make(map[string]struct{}, len(configuredURLs)+1)
	workers := make([]string, 0, len(configuredURLs)+1)
	appendWorker := func(rawURL string) {
		workerURL := normalizeH3WorkerURL(rawURL)
		if workerURL == "" {
			return
		}
		if _, exists := seen[workerURL]; exists {
			return
		}
		seen[workerURL] = struct{}{}
		workers = append(workers, workerURL)
	}
	for _, workerURL := range configuredURLs {
		appendWorker(workerURL)
	}
	appendWorker(fallbackURL)
	return workers
}

func normalizeH3WorkerURL(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return ""
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	parsed.RawPath = ""
	return strings.TrimRight(parsed.String(), "/")
}

func h3WorkerQueueLoad(workerURL, proxy string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), workerQueueTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, workerURL+"/queue", nil)
	if err != nil {
		return 0, err
	}
	request.Header.Set("Accept", "application/json")
	client, err := service.GetHttpClientWithProxy(strings.TrimSpace(proxy))
	if err != nil {
		return 0, err
	}
	response, err := client.Do(request)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("worker queue returned status %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 1024*1024))
	if err != nil {
		return 0, err
	}
	var queue struct {
		Running []any `json:"queue_running"`
		Pending []any `json:"queue_pending"`
	}
	if err := common.Unmarshal(body, &queue); err != nil {
		return 0, err
	}
	return len(queue.Running) + len(queue.Pending), nil
}

func reserveH3Worker(workerURL string) {
	if workerURL == "" {
		return
	}
	now := time.Now()
	h3WorkerReservations.Lock()
	defer h3WorkerReservations.Unlock()
	cleanupH3WorkerReservationsLocked(now)
	reservation := h3WorkerReservations.byURL[workerURL]
	reservation.count++
	reservation.expiresAt = now.Add(workerReservationTTL)
	h3WorkerReservations.byURL[workerURL] = reservation
}

func releaseH3WorkerReservation(workerURL string) {
	if workerURL == "" {
		return
	}
	now := time.Now()
	h3WorkerReservations.Lock()
	defer h3WorkerReservations.Unlock()
	cleanupH3WorkerReservationsLocked(now)
	reservation, exists := h3WorkerReservations.byURL[workerURL]
	if !exists {
		return
	}
	reservation.count--
	if reservation.count <= 0 {
		delete(h3WorkerReservations.byURL, workerURL)
		return
	}
	h3WorkerReservations.byURL[workerURL] = reservation
}

func activeH3WorkerReservations(workerURL string) int {
	now := time.Now()
	h3WorkerReservations.Lock()
	defer h3WorkerReservations.Unlock()
	cleanupH3WorkerReservationsLocked(now)
	return h3WorkerReservations.byURL[workerURL].count
}

func cleanupH3WorkerReservationsLocked(now time.Time) {
	for workerURL, reservation := range h3WorkerReservations.byURL {
		if reservation.count <= 0 || !reservation.expiresAt.After(now) {
			delete(h3WorkerReservations.byURL, workerURL)
		}
	}
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.baseURL = ""
	a.proxy = ""
	a.workerSelectionErr = nil
	a.workerReserved = false
	if info == nil {
		return
	}
	a.baseURL = normalizeH3WorkerURL(info.ChannelBaseUrl)
	a.proxy = info.ChannelSetting.Proxy
	if info.TaskRelayInfo != nil {
		// A failed attempt can be retried. Selection happens only after request
		// validation, so a retry gets a fresh queue snapshot instead of pinning
		// itself to the worker from the failed submission attempt.
		info.SelectedBackendURL = ""
	}
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
	selector, err := selectorFromRequest(req)
	if err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	if err := validateH3RequestParameters(req, selector); err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	if _, err := referenceInputs(c, req); err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	if _, err := newWorkflow(); err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_workflow", http.StatusBadGateway)
	}
	selected, err := selectH3Worker(info.ChannelOtherSettings.ComfyUIH3BackendURLs, a.baseURL, a.proxy)
	if err != nil {
		a.workerSelectionErr = err
		return service.TaskErrorWrapperLocal(err, "comfyui_h3_worker_unavailable", http.StatusServiceUnavailable)
	}
	a.baseURL = selected
	a.workerReserved = true
	if info != nil && info.TaskRelayInfo != nil {
		info.SelectedBackendURL = selected
	}
	return nil
}

func (a *TaskAdaptor) EstimateBilling(c *gin.Context, _ *relaycommon.RelayInfo) map[string]float64 {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil
	}
	ratios := map[string]float64{"seconds": float64(requestSeconds(req))}
	selector, err := selectorFromRequest(req)
	if err != nil {
		return ratios
	}
	qualityRatio := h3MegapixelBillingRatio(selector.Megapixels)
	if qualityRatio != 1 {
		ratios["megapixels"] = qualityRatio
	}
	if imageCount, err := referenceImageCount(c, req); err == nil {
		if imageRatio := h3ReferenceImageBillingRatio(requestSeconds(req), qualityRatio, imageCount); imageRatio != 1 {
			ratios["reference_images"] = imageRatio
		}
	}
	return ratios
}

func (a *TaskAdaptor) GetTaskOutputMetadata(c *gin.Context, _ *relaycommon.RelayInfo) (seconds int, size string, err error) {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return 0, "", err
	}
	selector, err := selectorFromRequest(req)
	if err != nil {
		return 0, "", err
	}
	if err := validateH3RequestParameters(req, selector); err != nil {
		return 0, "", err
	}

	seconds = requestSeconds(req)
	width, height, ok := h3OutputDimensions(selector)
	if !ok {
		return seconds, "", nil
	}
	return seconds, fmt.Sprintf("%dx%d", width, height), nil
}

// OverridePriceData keeps the existing H3 group-price contract identical for
// direct ComfyUI and RunningHub-backed H3 channels.
func (a *TaskAdaptor) OverridePriceData(c *gin.Context, info *relaycommon.RelayInfo) (hosttypes.PriceData, bool, error) {
	if info == nil || info.OriginModelName != modelName {
		return hosttypes.PriceData{}, false, nil
	}

	helper.HandleGroupRatio(c, info)
	priceGroup := strings.TrimSpace(info.UserSetting.RunningHubH3PriceGroup)
	groupPrice, ok := lookupRunningHubH3GroupPrice(priceGroup)
	if ok && strings.TrimSpace(groupPrice.BoundGroup) != "" && strings.TrimSpace(groupPrice.BoundGroup) != strings.TrimSpace(info.UsingGroup) {
		ok = false
	}
	if priceGroup == "" || !ok {
		groupPrice, ok = lookupRunningHubH3GroupPrice(info.UsingGroup)
		if ok && strings.TrimSpace(groupPrice.BoundGroup) != "" && strings.TrimSpace(groupPrice.BoundGroup) != strings.TrimSpace(info.UsingGroup) {
			ok = false
		}
	}
	if !ok {
		return hosttypes.PriceData{}, false, nil
	}

	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return hosttypes.PriceData{}, true, err
	}
	selector, err := selectorFromRequest(req)
	if err != nil {
		return hosttypes.PriceData{}, true, err
	}

	seconds := requestSeconds(req)
	rateCNY, err := h3CustomBillingRateCNY(groupPrice.Price768P, groupPrice.Price2K, selector.Megapixels)
	if err != nil {
		return hosttypes.PriceData{}, true, err
	}
	imageCount, err := referenceImageCount(c, req)
	if err != nil {
		return hosttypes.PriceData{}, true, err
	}
	if imageCount > 5 {
		rateCNY += float64(imageCount-5) * 0.10 / float64(seconds)
	}
	if rateCNY < 0 {
		return hosttypes.PriceData{}, true, fmt.Errorf("comfyui h3 group price must be non-negative")
	}

	priceData := hosttypes.PriceData{
		FreeModel:  rateCNY == 0,
		ModelPrice: rateCNY / usdExchangeRate(),
		UsePrice:   true,
		GroupRatioInfo: hosttypes.GroupRatioInfo{
			GroupRatio: 1,
		},
	}
	priceData.AddOtherRatio("seconds", float64(seconds))
	quota, err := common.QuotaFromFloatStrict(priceData.ApplyOtherRatiosToFloat(priceData.ModelPrice * common.QuotaPerUnit))
	if err != nil {
		return hosttypes.PriceData{}, true, err
	}
	priceData.Quota = quota
	return priceData, true, nil
}

func (a *TaskAdaptor) BuildRequestURL(_ *relaycommon.RelayInfo) (string, error) {
	if a.baseURL == "" {
		return "", fmt.Errorf("ComfyUI channel base URL is empty")
	}
	return a.baseURL + promptPath, nil
}

func (a *TaskAdaptor) BuildRequestHeader(_ *gin.Context, req *http.Request, _ *relaycommon.RelayInfo) error {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	return nil
}

func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, _ *relaycommon.RelayInfo) (bodyReader io.Reader, err error) {
	defer func() {
		if err != nil {
			a.releaseWorkerReservation()
		}
	}()
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil, err
	}
	selector, err := selectorFromRequest(req)
	if err != nil {
		return nil, err
	}
	if err := validateH3RequestParameters(req, selector); err != nil {
		return nil, err
	}
	req = a.applyPromptEnhancement(c, req, selector)
	references, err := a.collectReferences(c, req)
	if err != nil {
		return nil, err
	}
	workflow, err := buildWorkflow(req, selector, references)
	if err != nil {
		return nil, err
	}
	clientID, err := common.GenerateRandomCharsKey(32)
	if err != nil {
		return nil, err
	}
	body, err := common.Marshal(comfyPromptRequest{Prompt: workflow, ClientID: clientID})
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(body), nil
}

func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	resp, err := channel.DoTaskApiRequest(a, c, info, requestBody)
	if err != nil || (resp != nil && (resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices)) {
		a.releaseWorkerReservation()
	}
	return resp, err
}

func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (string, []byte, *taskdto.TaskError) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		a.releaseWorkerReservation()
		return "", nil, service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
	}
	var result comfyPromptResponse
	if err := common.Unmarshal(body, &result); err != nil {
		a.releaseWorkerReservation()
		return "", nil, service.TaskErrorWrapper(fmt.Errorf("unmarshal ComfyUI create response: %w", err), "unmarshal_response_body_failed", http.StatusBadGateway)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices || hasComfyError(result.Error) || strings.TrimSpace(result.PromptID) == "" {
		message := compactComfyError(result.Error)
		if message == "" && len(result.NodeErrors) > 0 {
			message = compactComfyError(result.NodeErrors)
		}
		if message == "" && strings.TrimSpace(result.PromptID) == "" {
			message = "ComfyUI create response missing prompt_id"
		}
		if message == "" {
			message = resp.Status
		}
		a.releaseWorkerReservation()
		return "", nil, service.TaskErrorWrapper(fmt.Errorf("ComfyUI create failed: %s", message), "comfyui_create_failed", resp.StatusCode)
	}

	publicTaskID := result.PromptID
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
	return result.PromptID, body, nil
}

func (a *TaskAdaptor) releaseWorkerReservation() {
	if !a.workerReserved {
		return
	}
	releaseH3WorkerReservation(a.baseURL)
	a.workerReserved = false
}

// AbortTaskSubmission is used by the generic relay pipeline when an error is
// returned after worker selection but before a task has been accepted upstream.
func (a *TaskAdaptor) AbortTaskSubmission() {
	a.releaseWorkerReservation()
}

func (a *TaskAdaptor) FetchTask(baseURL, _ string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, _ := body["task_id"].(string)
	if taskID == "" {
		taskID, _ = body["taskId"].(string)
	}
	if strings.TrimSpace(taskID) == "" {
		return nil, fmt.Errorf("ComfyUI task ID is empty")
	}
	a.baseURL = strings.TrimRight(baseURL, "/")
	requestURL := a.baseURL + historyPath + url.PathEscape(taskID)
	req, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	client, err := service.GetHttpClientWithProxy(strings.TrimSpace(proxy))
	if err != nil {
		return nil, err
	}
	return client.Do(req)
}

func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	var payload map[string]any
	if err := common.Unmarshal(respBody, &payload); err != nil {
		return nil, fmt.Errorf("unmarshal ComfyUI task result: %w", err)
	}
	if hasComfyError(payload["error"]) {
		return &relaycommon.TaskInfo{
			Status:   string(model.TaskStatusFailure),
			Progress: "100%",
			Reason:   compactComfyError(payload["error"]),
		}, nil
	}
	job := findHistoryJob(payload)
	if job == nil {
		return &relaycommon.TaskInfo{Status: string(model.TaskStatusQueued), Progress: "20%"}, nil
	}

	status := ""
	if statusInfo, ok := job["status"].(map[string]any); ok {
		status = firstString(statusInfo, "status_str", "status", "state")
	}
	if status == "" {
		status = firstString(job, "status_str", "status", "state")
	}
	status = strings.ToLower(strings.TrimSpace(status))
	switch status {
	case "error", "failed", "failure":
		return &relaycommon.TaskInfo{
			Status:   string(model.TaskStatusFailure),
			Progress: "100%",
			Reason:   comfyTaskFailureReason(job),
		}, nil
	case "success", "completed", "complete", "finished":
		resultURL := extractResultURL(a.baseURL, job)
		if resultURL == "" {
			return &relaycommon.TaskInfo{
				Status:   string(model.TaskStatusFailure),
				Progress: "100%",
				Reason:   "ComfyUI completed without a video result from node 210",
			}, nil
		}
		return &relaycommon.TaskInfo{Status: string(model.TaskStatusSuccess), Progress: "100%", Url: resultURL}, nil
	case "running", "executing", "processing":
		return &relaycommon.TaskInfo{Status: string(model.TaskStatusInProgress), Progress: "50%"}, nil
	}

	if _, ok := job["outputs"].(map[string]any); ok {
		resultURL := extractResultURL(a.baseURL, job)
		if resultURL != "" {
			return &relaycommon.TaskInfo{Status: string(model.TaskStatusSuccess), Progress: "100%", Url: resultURL}, nil
		}
	}
	return &relaycommon.TaskInfo{Status: string(model.TaskStatusInProgress), Progress: "30%"}, nil
}

func (a *TaskAdaptor) GetModelList() []string { return []string{modelName} }

func (a *TaskAdaptor) GetChannelName() string { return channelName }

func (a *TaskAdaptor) ConvertToOpenAIVideo(task *model.Task) ([]byte, error) {
	video := task.ToOpenAIVideo()
	if strings.TrimSpace(video.Model) == "" {
		video.Model = modelName
	}
	if task.Status == model.TaskStatusFailure {
		message := task.FailReason
		if message == "" {
			message = "task failed"
		}
		video.Error = &openaidto.OpenAIVideoError{Message: message, Code: "task_failed"}
	}
	return common.Marshal(video)
}

func newWorkflow() (map[string]any, error) {
	var workflow map[string]any
	if err := common.Unmarshal(workflowTemplate, &workflow); err != nil {
		return nil, fmt.Errorf("unmarshal embedded ComfyUI H3 workflow: %w", err)
	}
	if len(workflow) == 0 {
		return nil, fmt.Errorf("embedded ComfyUI H3 workflow is empty")
	}
	if node, ok := workflow[h3NodeID].(map[string]any); !ok || firstString(node, "class_type") != "MiniMaxH3ReferenceToVideo" {
		return nil, fmt.Errorf("embedded ComfyUI H3 workflow missing MiniMaxH3ReferenceToVideo node %s", h3NodeID)
	}
	if _, err := workflowInputs(workflow, promptNodeID, "value"); err != nil {
		return nil, err
	}
	if _, err := workflowInputs(workflow, paramsNodeID, "aspect_ratio", "megapixels", "multiple", "duration"); err != nil {
		return nil, err
	}
	if node, ok := workflow[outputNodeID].(map[string]any); !ok || firstString(node, "class_type") != "VHS_VideoCombine" {
		return nil, fmt.Errorf("embedded ComfyUI H3 workflow missing VHS_VideoCombine output node %s", outputNodeID)
	}
	return workflow, nil
}

// RequiredNodeTypes reports every custom node class required by the embedded
// workflow, including optional reference-input nodes injected at request time.
// Channel validation uses this before accepting a ComfyUI H3 server.
func RequiredNodeTypes() ([]string, error) {
	workflow, err := newWorkflow()
	if err != nil {
		return nil, err
	}
	nodeTypes := map[string]struct{}{
		"XB_VideoLoader": {},
		"LoadAudio":      {},
	}
	for _, value := range workflow {
		node, ok := value.(map[string]any)
		if !ok {
			continue
		}
		if classType := firstString(node, "class_type"); classType != "" {
			nodeTypes[classType] = struct{}{}
		}
	}
	result := make([]string, 0, len(nodeTypes))
	for classType := range nodeTypes {
		result = append(result, classType)
	}
	sort.Strings(result)
	return result, nil
}

// RequiredNodeChoices derives the model and profile selections baked into the
// embedded workflow. Channel tests use it to fail early when an otherwise
// installed ComfyUI node does not have the H3 assets required by this workflow.
func RequiredNodeChoices() ([]RequiredNodeChoice, error) {
	workflow, err := newWorkflow()
	if err != nil {
		return nil, err
	}
	choiceInputs := map[string]map[string]struct{}{
		"UNETLoader":             {"unet_name": {}},
		"CLIPLoader":             {"clip_name": {}},
		"VAELoader":              {"vae_name": {}},
		"LoraLoaderModelOnly":    {"lora_name": {}},
		"MiniMaxH3MemoryProfile": {"profile": {}},
	}
	choices := make([]RequiredNodeChoice, 0, 5)
	seen := make(map[RequiredNodeChoice]struct{})
	for _, value := range workflow {
		node, ok := value.(map[string]any)
		if !ok {
			continue
		}
		nodeType := firstString(node, "class_type")
		inputNames, ok := choiceInputs[nodeType]
		if !ok {
			continue
		}
		inputs, ok := node["inputs"].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("embedded ComfyUI H3 workflow missing %s inputs", nodeType)
		}
		for inputName := range inputNames {
			selectedValue, ok := inputs[inputName].(string)
			if !ok || strings.TrimSpace(selectedValue) == "" {
				return nil, fmt.Errorf("embedded ComfyUI H3 workflow missing %s input %s", nodeType, inputName)
			}
			choice := RequiredNodeChoice{NodeType: nodeType, InputName: inputName, Value: selectedValue}
			if _, exists := seen[choice]; !exists {
				seen[choice] = struct{}{}
				choices = append(choices, choice)
			}
		}
	}
	sort.Slice(choices, func(i, j int) bool {
		if choices[i].NodeType != choices[j].NodeType {
			return choices[i].NodeType < choices[j].NodeType
		}
		if choices[i].InputName != choices[j].InputName {
			return choices[i].InputName < choices[j].InputName
		}
		return choices[i].Value < choices[j].Value
	})
	return choices, nil
}

func buildWorkflow(req relaycommon.TaskSubmitReq, selector h3Selector, input references) (map[string]any, error) {
	workflow, err := newWorkflow()
	if err != nil {
		return nil, err
	}
	promptInputs, _ := workflowInputs(workflow, promptNodeID, "value")
	promptInputs["value"] = req.Prompt
	paramsInputs, _ := workflowInputs(workflow, paramsNodeID, "aspect_ratio", "megapixels", "multiple", "duration")
	paramsInputs["aspect_ratio"] = selector.AspectRatio
	paramsInputs["megapixels"] = selector.Megapixels
	paramsInputs["multiple"] = selector.Multiple
	paramsInputs["duration"] = requestSeconds(req)
	memoryProfileInputs, err := workflowInputs(workflow, memoryProfileNodeID, "profile")
	if err != nil {
		return nil, err
	}
	memoryProfileInputs["profile"] = h3MemoryProfile(selector.Megapixels, requestSeconds(req))

	h3Inputs, err := workflowInputs(workflow, h3NodeID)
	if err != nil {
		return nil, err
	}
	clearReferenceInputs(h3Inputs)
	nextID := nextNodeID(workflow)
	appendReference := func(prefix string, index int, classType string, inputs map[string]any, outputIndex int) {
		nodeID := strconv.Itoa(nextID)
		nextID++
		workflow[nodeID] = map[string]any{
			"class_type": classType,
			"inputs":     inputs,
		}
		h3Inputs[fmt.Sprintf("%s%d", prefix, index)] = []any{nodeID, outputIndex}
	}
	for index, fileName := range input.Images {
		appendReference("ref_images.ref_image_", index, "LoadImage", map[string]any{"image": fileName}, 0)
	}
	for index, fileName := range input.Videos {
		appendReference("ref_videos.ref_video_", index, "XB_VideoLoader", map[string]any{
			"video":             fileName,
			"force_rate":        0,
			"custom_width":      0,
			"custom_height":     0,
			"frame_load_cap":    0,
			"skip_first_frames": 0,
			"select_every_nth":  1,
			"format":            "AnimateDiff",
		}, 0)
	}
	for index, fileName := range input.VideoAudios {
		appendReference("ref_video_audios.ref_video_audio_", index, "LoadAudio", map[string]any{"audio": fileName}, 0)
	}
	for index, fileName := range input.Audios {
		appendReference("ref_audios.ref_audio_", index, "LoadAudio", map[string]any{"audio": fileName}, 0)
	}
	return workflow, nil
}

func h3MemoryProfile(megapixels float64, seconds int) string {
	if megapixels*float64(seconds) > 18 {
		return "2MP low VRAM"
	}
	return "1MP standard speed"
}

func workflowInputs(workflow map[string]any, nodeID string, requiredFields ...string) (map[string]any, error) {
	node, ok := workflow[nodeID].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("embedded ComfyUI H3 workflow missing node %s", nodeID)
	}
	inputs, ok := node["inputs"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("embedded ComfyUI H3 workflow missing node %s inputs", nodeID)
	}
	for _, field := range requiredFields {
		if _, ok := inputs[field]; !ok {
			return nil, fmt.Errorf("embedded ComfyUI H3 workflow missing node %s input %s", nodeID, field)
		}
	}
	return inputs, nil
}

func clearReferenceInputs(inputs map[string]any) {
	for key := range inputs {
		if strings.HasPrefix(key, "ref_images.ref_image_") ||
			strings.HasPrefix(key, "ref_videos.ref_video_") ||
			strings.HasPrefix(key, "ref_video_audios.ref_video_audio_") ||
			strings.HasPrefix(key, "ref_audios.ref_audio_") {
			delete(inputs, key)
		}
	}
}

func nextNodeID(workflow map[string]any) int {
	maxID := 0
	for key := range workflow {
		if id, err := strconv.Atoi(key); err == nil && id > maxID {
			maxID = id
		}
	}
	return maxID + 1
}

func (a *TaskAdaptor) collectReferences(c *gin.Context, req relaycommon.TaskSubmitReq) (references, error) {
	sources, err := referenceInputs(c, req)
	if err != nil {
		return references{}, err
	}
	for _, header := range sources.imageFiles {
		input, err := readMultipartFile(header)
		if err != nil {
			return references{}, err
		}
		fileName, err := a.uploadReference(input)
		if err != nil {
			return references{}, err
		}
		sources.images = append(sources.images, fileName)
	}
	for _, header := range sources.videoFiles {
		input, err := readMultipartFile(header)
		if err != nil {
			return references{}, err
		}
		fileName, err := a.uploadReference(input)
		if err != nil {
			return references{}, err
		}
		sources.videos = append(sources.videos, fileName)
	}
	for _, header := range sources.videoAudioFiles {
		input, err := readMultipartFile(header)
		if err != nil {
			return references{}, err
		}
		fileName, err := a.uploadReference(input)
		if err != nil {
			return references{}, err
		}
		sources.videoAudios = append(sources.videoAudios, fileName)
	}
	for _, header := range sources.audioFiles {
		input, err := readMultipartFile(header)
		if err != nil {
			return references{}, err
		}
		fileName, err := a.uploadReference(input)
		if err != nil {
			return references{}, err
		}
		sources.audios = append(sources.audios, fileName)
	}

	images, err := a.resolveReferenceValues(sources.images)
	if err != nil {
		return references{}, err
	}
	videos, err := a.resolveReferenceValues(sources.videos)
	if err != nil {
		return references{}, err
	}
	videoAudios, err := a.resolveReferenceValues(sources.videoAudios)
	if err != nil {
		return references{}, err
	}
	audios, err := a.resolveReferenceValues(sources.audios)
	if err != nil {
		return references{}, err
	}
	return references{Images: images, Videos: videos, VideoAudios: videoAudios, Audios: audios}, nil
}

func referenceInputs(c *gin.Context, req relaycommon.TaskSubmitReq) (referenceSources, error) {
	sources := referenceSources{}
	sources.images = appendDistinctNonEmpty(sources.images, req.InputReference, req.Image)
	sources.images = appendDistinctStrings(sources.images, req.Images...)
	sources.images = appendDistinctStrings(sources.images, metadataStrings(req.Metadata, "input_reference")...)
	sources.images = appendDistinctStrings(sources.images, metadataStrings(req.Metadata, "reference_images")...)
	sources.videos = appendDistinctStrings(sources.videos, metadataStrings(req.Metadata, "reference_video")...)
	sources.videos = appendDistinctStrings(sources.videos, metadataStrings(req.Metadata, "reference_videos")...)
	sources.videoAudios = appendDistinctStrings(sources.videoAudios, metadataStrings(req.Metadata, "reference_video_audio")...)
	sources.videoAudios = appendDistinctStrings(sources.videoAudios, metadataStrings(req.Metadata, "reference_video_audios")...)
	sources.audios = appendDistinctStrings(sources.audios, metadataStrings(req.Metadata, "reference_audio")...)
	sources.audios = appendDistinctStrings(sources.audios, metadataStrings(req.Metadata, "reference_audios")...)

	if c != nil && c.Request != nil && c.Request.MultipartForm != nil {
		form := c.Request.MultipartForm
		sources.images = appendDistinctStrings(sources.images, form.Value["input_reference"]...)
		sources.images = appendDistinctStrings(sources.images, form.Value["reference_images"]...)
		sources.videos = appendDistinctStrings(sources.videos, form.Value["reference_video"]...)
		sources.videos = appendDistinctStrings(sources.videos, form.Value["reference_videos"]...)
		sources.videoAudios = appendDistinctStrings(sources.videoAudios, form.Value["reference_video_audio"]...)
		sources.videoAudios = appendDistinctStrings(sources.videoAudios, form.Value["reference_video_audios"]...)
		sources.audios = appendDistinctStrings(sources.audios, form.Value["reference_audio"]...)
		sources.audios = appendDistinctStrings(sources.audios, form.Value["reference_audios"]...)
		sources.imageFiles = multipartFiles(form, "input_reference", "image", "images", "reference_images")
		sources.videoFiles = multipartFiles(form, "reference_video", "reference_videos")
		sources.videoAudioFiles = multipartFiles(form, "reference_video_audio", "reference_video_audios")
		sources.audioFiles = multipartFiles(form, "reference_audio", "reference_audios")
	}
	if len(sources.images)+len(sources.imageFiles) > maxImages {
		return referenceSources{}, fmt.Errorf("ComfyUI H3 supports at most %d reference images", maxImages)
	}
	if len(sources.videos)+len(sources.videoFiles) > maxVideos {
		return referenceSources{}, fmt.Errorf("ComfyUI H3 supports at most %d reference videos", maxVideos)
	}
	if len(sources.videoAudios)+len(sources.videoAudioFiles) > maxVideoAudios {
		return referenceSources{}, fmt.Errorf("ComfyUI H3 supports at most %d reference video audio files", maxVideoAudios)
	}
	if len(sources.audios)+len(sources.audioFiles) > maxAudios {
		return referenceSources{}, fmt.Errorf("ComfyUI H3 supports at most %d reference audio files", maxAudios)
	}
	return sources, nil
}

func referenceImageCount(c *gin.Context, req relaycommon.TaskSubmitReq) (int, error) {
	sources, err := referenceInputs(c, req)
	if err != nil {
		return 0, err
	}
	return len(sources.images) + len(sources.imageFiles), nil
}

func (a *TaskAdaptor) resolveReferenceValues(values []string) ([]string, error) {
	resolved := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if strings.HasPrefix(strings.ToLower(value), "data:image/") {
			input, err := promptEnhancerDataURLInput(value)
			if err != nil {
				return nil, err
			}
			fileName, err := a.uploadReference(input)
			if err != nil {
				return nil, err
			}
			resolved = append(resolved, fileName)
			continue
		}
		if isHTTPURL(value) {
			input, err := downloadReference(value)
			if err != nil {
				return nil, err
			}
			fileName, err := a.uploadReference(input)
			if err != nil {
				return nil, err
			}
			resolved = append(resolved, fileName)
			continue
		}
		if strings.Contains(value, "..") {
			return nil, fmt.Errorf("reference file name cannot contain '..'")
		}
		resolved = append(resolved, value)
	}
	return resolved, nil
}

func (a *TaskAdaptor) uploadReference(input referenceInput) (string, error) {
	if int64(len(input.Data)) > maxReferenceBytes() {
		return "", fmt.Errorf("file %s exceeds max size %d MB", input.Name, maxReferenceMB())
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	uploadName, err := uniqueUploadFileName(input.Name)
	if err != nil {
		return "", err
	}
	part, err := writer.CreateFormFile("image", uploadName)
	if err != nil {
		return "", err
	}
	if _, err := part.Write(input.Data); err != nil {
		return "", err
	}
	if err := writer.WriteField("type", "input"); err != nil {
		return "", err
	}
	if err := writer.WriteField("overwrite", "false"); err != nil {
		return "", err
	}
	if err := writer.Close(); err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodPost, a.baseURL+uploadPath, &body)
	if err != nil {
		return "", err
	}
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
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		message := strings.TrimSpace(string(responseBody))
		if len(message) > 200 {
			message = message[:200]
		}
		return "", fmt.Errorf("ComfyUI upload failed with status %s: %s", resp.Status, message)
	}
	var result struct {
		Name      string `json:"name"`
		Subfolder string `json:"subfolder"`
	}
	if err := common.Unmarshal(responseBody, &result); err != nil {
		return "", fmt.Errorf("unmarshal ComfyUI upload response: %w", err)
	}
	if strings.TrimSpace(result.Name) == "" {
		return "", fmt.Errorf("ComfyUI upload response missing name")
	}
	if subfolder := strings.Trim(strings.TrimSpace(result.Subfolder), "/"); subfolder != "" {
		return subfolder + "/" + result.Name, nil
	}
	return result.Name, nil
}

func uniqueUploadFileName(name string) (string, error) {
	name = safeFileName(name)
	randomPart, err := common.GenerateRandomCharsKey(24)
	if err != nil {
		return "", err
	}
	extension := filepath.Ext(name)
	return "newapi-" + randomPart + extension, nil
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
	for _, key := range []string{"resolution", "clarity"} {
		if value := metadataString(req.Metadata, key); value != "" {
			if megapixels, err := h3MegapixelsFromValue(value); err == nil {
				selector.Megapixels = megapixels
			} else {
				aspect, megapixels, sizeErr := h3ParametersFromSize(value)
				if sizeErr != nil {
					return selector, sizeErr
				}
				selector.AspectRatio = aspect
				selector.Megapixels = megapixels
			}
		}
	}
	if value := metadataString(req.Metadata, "aspect_ratio"); value != "" {
		aspect, err := normalizeAspectRatio(value)
		if err != nil {
			return selector, err
		}
		selector.AspectRatio = aspect
	}
	if value := metadataString(req.Metadata, "megapixels"); value != "" {
		megapixels, err := h3MegapixelsFromValue(value)
		if err != nil {
			return selector, err
		}
		selector.Megapixels = megapixels
	}
	if value := metadataString(req.Metadata, "multiple"); value != "" {
		multiple, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil || multiple < minMultiple || multiple > maxMultiple || multiple%multipleStep != 0 {
			return selector, fmt.Errorf("unsupported ComfyUI H3 multiple %q", value)
		}
		selector.Multiple = multiple
	}
	return selector, nil
}

func h3ParametersFromSize(size string) (string, float64, error) {
	s := strings.ToLower(strings.TrimSpace(size))
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "*", "x")
	switch s {
	case "", "auto":
		return defaultAspect, defaultMegapixels, nil
	case "1:1", "1:1(square)", "square":
		return "1:1 (Square)", defaultMegapixels, nil
	case "2:3", "2:3(portraitphoto)", "portraitphoto":
		return "2:3 (Portrait Photo)", defaultMegapixels, nil
	case "3:2", "3:2(photo)", "photo":
		return "3:2 (Photo)", defaultMegapixels, nil
	case "3:4", "3:4(portraitstandard)", "portraitstandard":
		return "3:4 (Portrait Standard)", defaultMegapixels, nil
	case "4:3", "4:3(standard)", "standard":
		return "4:3 (Standard)", defaultMegapixels, nil
	case "9:16", "9:16(portraitwidescreen)", "portrait", "portraitwidescreen":
		return "9:16 (Portrait Widescreen)", defaultMegapixels, nil
	case "16:9", "16:9(widescreen)", "16:9(landscapewidescreen)", "landscape", "widescreen":
		return "16:9 (Widescreen)", defaultMegapixels, nil
	case "21:9", "21:9(ultrawide)", "ultrawide":
		return "21:9 (Ultrawide)", defaultMegapixels, nil
	}
	if strings.HasSuffix(s, "p") {
		height, err := strconv.Atoi(strings.TrimSuffix(s, "p"))
		if err == nil && height >= 352 && height <= 1080 {
			width := int(math.Round(float64(height) * 16.0 / 9.0))
			return "16:9 (Widescreen)", nearestH3Megapixels(width, height), nil
		}
	}
	width, height, ok := parseSizeDimensions(s)
	if !ok {
		return "", 0, fmt.Errorf("unsupported ComfyUI H3 size %q; supported aspect ratios are %s", size, strings.Join(h3SupportedAspectLabels, ", "))
	}
	aspect, err := aspectRatioFromDimensions(width, height)
	if err != nil {
		return "", 0, fmt.Errorf("unsupported ComfyUI H3 size %q; supported aspect ratios are %s", size, strings.Join(h3SupportedAspectLabels, ", "))
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

func h3OutputDimensions(selector h3Selector) (width, height int, ok bool) {
	if selector.Megapixels <= 0 || math.IsNaN(selector.Megapixels) || math.IsInf(selector.Megapixels, 0) || selector.Multiple <= 0 {
		return 0, 0, false
	}

	aspectRatio := 0.0
	for _, preset := range h3AspectPresets {
		if preset.Ratio == selector.AspectRatio {
			aspectRatio = preset.Value
			break
		}
	}
	if aspectRatio <= 0 {
		return 0, 0, false
	}

	pixelArea := selector.Megapixels * 1024 * 1024
	width = int(math.Round(math.Sqrt(pixelArea*aspectRatio)/float64(selector.Multiple))) * selector.Multiple
	height = int(math.Round(math.Sqrt(pixelArea/aspectRatio)/float64(selector.Multiple))) * selector.Multiple
	if width <= 0 || height <= 0 {
		return 0, 0, false
	}
	return width, height, true
}

func h3MegapixelsFromValue(value string) (float64, error) {
	megapixels, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return 0, fmt.Errorf("unsupported ComfyUI H3 megapixels %q", value)
	}
	for _, preset := range h3MegapixelPresets {
		if math.Abs(megapixels-preset.Value) < 0.000001 {
			return preset.Value, nil
		}
	}
	return 0, fmt.Errorf("unsupported ComfyUI H3 megapixels %q; supported values are 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 0.98, 1.0, 1.2, 1.5, 1.8, and 2.0", value)
}

func h3MegapixelBillingRatio(megapixels float64) float64 {
	if math.Abs(megapixels-1) < 0.000001 {
		return 1
	}
	if megapixels > 1 {
		return megapixels * 1.5
	}
	return megapixels * 1.1
}

func h3ReferenceImageBillingRatio(seconds int, qualityRatio float64, imageCount int) float64 {
	if imageCount <= 5 || seconds <= 0 || qualityRatio <= 0 {
		return 1
	}
	return 1 + float64(imageCount-5)/(float64(seconds)*qualityRatio)
}

func h3CustomBillingRateCNY(price768P float64, price2K float64, megapixels float64) (float64, error) {
	qualityRatio := h3MegapixelBillingRatio(megapixels)
	if megapixels > 1 {
		if price2K < 0 {
			return 0, fmt.Errorf("ComfyUI H3 group price must be non-negative")
		}
		return price2K * (qualityRatio / 3), nil
	}
	if price768P < 0 {
		return 0, fmt.Errorf("ComfyUI H3 group price must be non-negative")
	}
	return price768P * qualityRatio, nil
}

func usdExchangeRate() float64 {
	if operation_setting.USDExchangeRate > 0 {
		return operation_setting.USDExchangeRate
	}
	return ratio_setting.USD2RMB
}

func normalizeAspectRatio(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	for _, supported := range h3SupportedAspectLabels {
		if trimmed == supported {
			return trimmed, nil
		}
	}
	s := strings.ToLower(trimmed)
	switch s {
	case "1:1", "1:1 (square)", "square":
		return "1:1 (Square)", nil
	case "2:3", "2:3 (portrait photo)", "portrait photo":
		return "2:3 (Portrait Photo)", nil
	case "3:2", "3:2 (photo)", "photo":
		return "3:2 (Photo)", nil
	case "3:4", "3:4 (portrait standard)", "portrait standard":
		return "3:4 (Portrait Standard)", nil
	case "4:3", "4:3 (standard)", "standard":
		return "4:3 (Standard)", nil
	case "9:16", "9:16 (portrait widescreen)", "portrait", "portrait widescreen":
		return "9:16 (Portrait Widescreen)", nil
	case "16:9", "16:9 (widescreen)", "16:9 (landscape widescreen)", "landscape", "widescreen":
		return "16:9 (Widescreen)", nil
	case "21:9", "21:9 (ultrawide)", "ultrawide":
		return "21:9 (Ultrawide)", nil
	default:
		return "", fmt.Errorf("unsupported ComfyUI H3 aspect_ratio %q; supported values are %s", value, strings.Join(h3SupportedAspectLabels, ", "))
	}
}

func requestSeconds(req relaycommon.TaskSubmitReq) int {
	seconds, _ := parseRequestSeconds(req)
	return seconds
}

func parseRequestSeconds(req relaycommon.TaskSubmitReq) (int, error) {
	if req.Duration > 0 {
		return req.Duration, nil
	}
	if secondsText := strings.TrimSpace(req.Seconds); secondsText != "" {
		seconds, err := strconv.Atoi(secondsText)
		if err != nil || seconds < 1 {
			return 0, fmt.Errorf("ComfyUI H3 seconds must be a positive integer")
		}
		return seconds, nil
	}
	return 5, nil
}

func validateH3RequestParameters(req relaycommon.TaskSubmitReq, selector h3Selector) error {
	seconds, err := parseRequestSeconds(req)
	if err != nil {
		return err
	}
	if seconds < 1 || seconds > maxDurationSeconds {
		return fmt.Errorf("ComfyUI H3 seconds must be between 1 and %d", maxDurationSeconds)
	}
	if selector.Multiple < minMultiple || selector.Multiple > maxMultiple || selector.Multiple%multipleStep != 0 {
		return fmt.Errorf("ComfyUI H3 multiple must be between %d and %d in increments of %d", minMultiple, maxMultiple, multipleStep)
	}
	return nil
}

func validateMultipartFields(c *gin.Context) error {
	if c == nil || c.Request == nil || c.Request.MultipartForm == nil {
		return nil
	}
	allowed := map[string]bool{
		"input_reference":        true,
		"image":                  true,
		"images":                 true,
		"reference_images":       true,
		"reference_video":        true,
		"reference_videos":       true,
		"reference_video_audio":  true,
		"reference_video_audios": true,
		"reference_audio":        true,
		"reference_audios":       true,
	}
	for field, files := range c.Request.MultipartForm.File {
		if len(files) > 0 && !allowed[field] {
			return fmt.Errorf("ComfyUI H3 does not support multipart file field %q", field)
		}
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
	return readMultipartFileWithLimit(fileHeader, maxReferenceBytes())
}

func readMultipartFileWithLimit(fileHeader *multipart.FileHeader, maxBytes int64) (referenceInput, error) {
	if fileHeader == nil {
		return referenceInput{}, fmt.Errorf("reference file is missing")
	}
	if fileHeader.Size > maxBytes {
		return referenceInput{}, fmt.Errorf("file %s exceeds max size %d MB", fileHeader.Filename, bytesToMegabytes(maxBytes))
	}
	file, err := fileHeader.Open()
	if err != nil {
		return referenceInput{}, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		return referenceInput{}, err
	}
	if int64(len(data)) > maxBytes {
		return referenceInput{}, fmt.Errorf("file %s exceeds max size %d MB", fileHeader.Filename, bytesToMegabytes(maxBytes))
	}
	return referenceInput{Name: safeFileName(fileHeader.Filename), Data: data}, nil
}

func downloadReference(rawURL string) (referenceInput, error) {
	return downloadReferenceWithLimit(rawURL, maxReferenceBytes())
}

func downloadReferenceWithLimit(rawURL string, maxBytes int64) (referenceInput, error) {
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
	if resp.ContentLength > maxBytes {
		return referenceInput{}, fmt.Errorf("reference file exceeds max size %d MB", bytesToMegabytes(maxBytes))
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return referenceInput{}, err
	}
	if int64(len(data)) > maxBytes {
		return referenceInput{}, fmt.Errorf("reference file exceeds max size %d MB", bytesToMegabytes(maxBytes))
	}
	parsed, _ := url.Parse(rawURL)
	return referenceInput{Name: safeFileName(filepath.Base(parsed.Path)), Data: data}, nil
}

func bytesToMegabytes(value int64) int64 {
	const megabyte = int64(1024 * 1024)
	return (value + megabyte - 1) / megabyte
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
			if value := strings.TrimSpace(fmt.Sprint(item)); value != "" {
				values = append(values, value)
			}
		}
		return values
	default:
		if value := strings.TrimSpace(fmt.Sprint(typed)); value != "" {
			return []string{value}
		}
		return nil
	}
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

func hasComfyError(value any) bool {
	if value == nil {
		return false
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed) != ""
	case map[string]any:
		return len(typed) > 0
	case []any:
		return len(typed) > 0
	default:
		return strings.TrimSpace(fmt.Sprint(typed)) != "" && fmt.Sprint(typed) != "<nil>"
	}
}

func compactComfyError(value any) string {
	if !hasComfyError(value) {
		return ""
	}
	data, err := common.Marshal(value)
	message := ""
	if err == nil {
		message = string(data)
	} else {
		message = fmt.Sprint(value)
	}
	message = strings.TrimSpace(message)
	if len(message) > maxComfyErrorReasonSize {
		message = message[:maxComfyErrorReasonSize]
	}
	return message
}

func findHistoryJob(payload map[string]any) map[string]any {
	if _, ok := payload["outputs"]; ok {
		return payload
	}
	for _, value := range payload {
		job, ok := value.(map[string]any)
		if !ok {
			continue
		}
		if _, ok := job["outputs"]; ok {
			return job
		}
		if _, ok := job["status"]; ok {
			return job
		}
	}
	return nil
}

func comfyTaskFailureReason(job map[string]any) string {
	if status, ok := job["status"].(map[string]any); ok {
		if messages, ok := status["messages"]; ok && hasComfyError(messages) {
			return compactComfyError(messages)
		}
	}
	if message := compactComfyError(job["error"]); message != "" {
		return message
	}
	return "ComfyUI task failed"
}

func extractResultURL(baseURL string, job map[string]any) string {
	outputs, ok := job["outputs"].(map[string]any)
	if !ok {
		return ""
	}
	output, ok := outputs[outputNodeID].(map[string]any)
	if !ok {
		return ""
	}
	return outputVideoURL(baseURL, output)
}

func outputVideoURL(baseURL string, output map[string]any) string {
	items, ok := output["gifs"].([]any)
	if !ok {
		return ""
	}
	for _, item := range items {
		file, ok := item.(map[string]any)
		if !ok {
			continue
		}
		filename := firstString(file, "filename", "name")
		if !isVideoFileName(filename) {
			continue
		}
		fileType := firstString(file, "type")
		if fileType != "" && fileType != "output" {
			continue
		}
		return buildViewURL(baseURL, filename, firstString(file, "subfolder"), "output")
	}
	return ""
}

func isVideoFileName(fileName string) bool {
	switch strings.ToLower(filepath.Ext(strings.TrimSpace(fileName))) {
	case ".mp4", ".mov", ".webm", ".mkv", ".avi":
		return true
	default:
		return false
	}
}

func buildViewURL(baseURL, filename, subfolder, fileType string) string {
	if strings.TrimSpace(baseURL) == "" || strings.TrimSpace(filename) == "" {
		return ""
	}
	query := url.Values{}
	query.Set("filename", filename)
	if strings.TrimSpace(subfolder) != "" {
		query.Set("subfolder", subfolder)
	}
	if strings.TrimSpace(fileType) != "" {
		query.Set("type", fileType)
	} else {
		query.Set("type", "output")
	}
	return strings.TrimRight(baseURL, "/") + "/view?" + query.Encode()
}
