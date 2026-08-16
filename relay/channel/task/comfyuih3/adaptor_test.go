package comfyuih3

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func resetH3WorkerReservations(t *testing.T) {
	t.Helper()
	h3WorkerReservations.Lock()
	h3WorkerReservations.byURL = make(map[string]h3WorkerReservation)
	h3WorkerReservations.nextIndexByPool = make(map[string]int)
	h3WorkerReservations.unavailableByURL = make(map[string]h3WorkerUnavailable)
	h3WorkerReservations.Unlock()
}

func withH3ReferenceVideoDurationProbe(t *testing.T, probe func(referenceInput) (float64, error)) {
	t.Helper()
	previous := probeH3ReferenceVideoDuration
	probeH3ReferenceVideoDuration = probe
	t.Cleanup(func() {
		probeH3ReferenceVideoDuration = previous
	})
}

func TestH3WorkerURLsDeduplicateAndFallBackToChannelBaseURL(t *testing.T) {
	workers := h3WorkerURLs([]string{
		"http://worker-a:5900/",
		"http://worker-a:5900",
		"not-a-url",
		"https://worker-b:5900/comfy/",
	}, "http://fallback:5900/")
	require.Equal(t, []string{
		"http://worker-a:5900",
		"https://worker-b:5900/comfy",
		"http://fallback:5900",
	}, workers)
}

func TestSelectH3WorkerUsesLeastQueueAndReservation(t *testing.T) {
	resetH3WorkerReservations(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/queue", r.URL.Path)
		_, _ = w.Write([]byte(`{"queue_running":[],"queue_pending":[]}`))
	}))
	defer server.Close()

	selected, err := selectH3Worker([]string{server.URL}, "", "")
	require.NoError(t, err)
	require.Equal(t, server.URL, selected)
	require.Equal(t, 1, activeH3WorkerReservations(server.URL))
	releaseH3WorkerReservation(server.URL)
}

func TestSelectH3WorkerSpreadsConcurrentIdleWorkers(t *testing.T) {
	resetH3WorkerReservations(t)
	newWorker := func() *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "/queue", r.URL.Path)
			_, _ = w.Write([]byte(`{"queue_running":[],"queue_pending":[]}`))
		}))
	}
	first := newWorker()
	defer first.Close()
	second := newWorker()
	defer second.Close()

	const requests = 2
	selected := make(chan string, requests)
	errs := make(chan error, requests)
	for range requests {
		go func() {
			worker, err := selectH3Worker([]string{first.URL, second.URL}, "", "")
			if err != nil {
				errs <- err
				return
			}
			selected <- worker
		}()
	}
	seen := map[string]bool{}
	for range requests {
		select {
		case err := <-errs:
			require.NoError(t, err)
		case worker := <-selected:
			seen[worker] = true
		}
	}
	require.Equal(t, map[string]bool{first.URL: true, second.URL: true}, seen)
	releaseH3WorkerReservation(first.URL)
	releaseH3WorkerReservation(second.URL)
}

func TestSelectH3WorkerRoundRobinsSequentialIdleWorkers(t *testing.T) {
	resetH3WorkerReservations(t)
	newWorker := func() *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "/queue", r.URL.Path)
			_, _ = w.Write([]byte(`{"queue_running":[],"queue_pending":[]}`))
		}))
	}
	first := newWorker()
	defer first.Close()
	second := newWorker()
	defer second.Close()

	firstSelected, err := selectH3Worker([]string{first.URL, second.URL}, "", "")
	require.NoError(t, err)
	releaseH3WorkerReservation(firstSelected)
	secondSelected, err := selectH3Worker([]string{first.URL, second.URL}, "", "")
	require.NoError(t, err)
	releaseH3WorkerReservation(secondSelected)

	require.NotEqual(t, firstSelected, secondSelected)
}

func TestH3WorkerQueueLoadRejectsMalformedQueueResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	_, err := h3WorkerQueueLoad(server.URL, "")
	require.ErrorContains(t, err, "queue_running")
}

func TestSelectH3WorkerSkipsRecentPromptSubmissionFailure(t *testing.T) {
	resetH3WorkerReservations(t)
	newWorker := func() *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"queue_running":[],"queue_pending":[]}`))
		}))
	}
	first := newWorker()
	defer first.Close()
	second := newWorker()
	defer second.Close()

	markH3WorkerUnavailable(first.URL, fmt.Errorf("prompt failed"), false)
	selected, err := selectH3Worker([]string{first.URL, second.URL}, "", "")
	require.NoError(t, err)
	require.Equal(t, second.URL, selected)
	releaseH3WorkerReservation(selected)
}

func TestH3PromptFailureQuarantineCannotBeDowngradedByQueueProbeFailure(t *testing.T) {
	resetH3WorkerReservations(t)
	workerURL := "http://worker.example:5900"
	markH3WorkerUnavailable(workerURL, fmt.Errorf("prompt failed"), false)
	markH3WorkerUnavailable(workerURL, fmt.Errorf("queue failed"), true)
	markH3WorkerHealthy(workerURL)

	h3WorkerReservations.Lock()
	_, stillUnavailable := h3WorkerReservations.unavailableByURL[workerURL]
	h3WorkerReservations.Unlock()
	require.True(t, stillUnavailable)
}

func TestShouldQuarantineH3WorkerAfterSubmitErrorSkipsCanceledClientRequest(t *testing.T) {
	ginCtx, _ := gin.CreateTestContext(httptest.NewRecorder())
	requestContext, cancelRequest := context.WithCancel(context.Background())
	ginCtx.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", nil).WithContext(requestContext)
	cancelRequest()

	require.False(t, shouldQuarantineH3WorkerAfterSubmitError(ginCtx, fmt.Errorf("do request failed: context canceled")))
	require.False(t, shouldQuarantineH3WorkerAfterSubmitError(ginCtx, fmt.Errorf("new proxy http client failed")))

	activeCtx, _ := gin.CreateTestContext(httptest.NewRecorder())
	activeCtx.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", nil)
	require.False(t, shouldQuarantineH3WorkerAfterSubmitError(activeCtx, fmt.Errorf("do request failed: connection reset")))
}

func TestTaskAdaptorInitClearsFailedAttemptWorkerBeforeFreshSelection(t *testing.T) {
	adaptor := &TaskAdaptor{}
	info := &relaycommon.RelayInfo{
		ChannelMeta:   &relaycommon.ChannelMeta{ChannelBaseUrl: "http://fallback:5900"},
		TaskRelayInfo: &relaycommon.TaskRelayInfo{SelectedBackendURL: "http://worker-a:5900/"},
	}
	adaptor.Init(info)
	require.NoError(t, adaptor.workerSelectionErr)
	require.Equal(t, "http://fallback:5900", adaptor.baseURL)
	require.Empty(t, info.SelectedBackendURL)
}

func TestBuildWorkflowMapsVideoParametersAndReferences(t *testing.T) {
	workflow, err := buildWorkflow(
		relaycommon.TaskSubmitReq{Prompt: "a dancer on a bright stage", Duration: 10},
		h3Selector{AspectRatio: "16:9 (Widescreen)", Megapixels: 2, Multiple: 32},
		references{
			Images:      []string{"image.png"},
			Videos:      []string{"motion.mp4"},
			VideoAudios: []string{"motion-audio.mp3"},
			Audios:      []string{"voice.mp3"},
		},
	)
	require.NoError(t, err)

	promptInputs, err := workflowInputs(workflow, promptNodeID, "value")
	require.NoError(t, err)
	require.Equal(t, "a dancer on a bright stage", promptInputs["value"])

	paramsInputs, err := workflowInputs(workflow, paramsNodeID, "aspect_ratio", "megapixels", "multiple", "duration")
	require.NoError(t, err)
	require.Equal(t, "16:9 (Widescreen)", paramsInputs["aspect_ratio"])
	require.Equal(t, 2.0, paramsInputs["megapixels"])
	require.Equal(t, 32, paramsInputs["multiple"])
	require.Equal(t, 10, paramsInputs["duration"])

	h3Inputs, err := workflowInputs(workflow, h3NodeID)
	require.NoError(t, err)
	imageNodeID := workflowReferenceNodeID(t, h3Inputs, "ref_images.ref_image_0")
	videoNodeID := workflowReferenceNodeID(t, h3Inputs, "ref_videos.ref_video_0")
	videoAudioNodeID := workflowReferenceNodeID(t, h3Inputs, "ref_video_audios.ref_video_audio_0")
	audioNodeID := workflowReferenceNodeID(t, h3Inputs, "ref_audios.ref_audio_0")
	require.Equal(t, []string{"229", "230", "231", "232"}, []string{imageNodeID, videoNodeID, videoAudioNodeID, audioNodeID})

	imageNode := workflow[imageNodeID].(map[string]any)
	require.Equal(t, "LoadImage", imageNode["class_type"])
	require.Equal(t, "image.png", imageNode["inputs"].(map[string]any)["image"])
	videoNode := workflow[videoNodeID].(map[string]any)
	require.Equal(t, "XB_VideoLoader", videoNode["class_type"])
	require.Equal(t, "motion.mp4", videoNode["inputs"].(map[string]any)["video"])
	require.Equal(t, "LoadAudio", workflow[videoAudioNodeID].(map[string]any)["class_type"])
	require.Equal(t, "LoadAudio", workflow[audioNodeID].(map[string]any)["class_type"])
	require.NotContains(t, workflow, "195")
}

func workflowReferenceNodeID(t *testing.T, h3Inputs map[string]any, inputName string) string {
	t.Helper()
	reference, ok := h3Inputs[inputName].([]any)
	require.Truef(t, ok, "invalid node reference for %s", inputName)
	require.Len(t, reference, 2)
	nodeID, ok := reference[0].(string)
	require.Truef(t, ok, "invalid node id for %s", inputName)
	require.EqualValues(t, 0, reference[1])
	return nodeID
}

func TestBuildWorkflowMapsAllNineReferenceImagesInOrder(t *testing.T) {
	images := make([]string, maxImages)
	for index := range images {
		images[index] = fmt.Sprintf("reference-%d.png", index+1)
	}

	workflow, err := buildWorkflow(
		relaycommon.TaskSubmitReq{Prompt: "combine all reference images", Seconds: "5"},
		h3Selector{AspectRatio: defaultAspect, Megapixels: 1, Multiple: 32},
		references{Images: images},
	)
	require.NoError(t, err)

	h3Inputs, err := workflowInputs(workflow, h3NodeID)
	require.NoError(t, err)
	for index, image := range images {
		referenceKey := fmt.Sprintf("ref_images.ref_image_%d", index)
		nodeRef, exists := h3Inputs[referenceKey]
		require.Truef(t, exists, "missing %s", referenceKey)
		nodeID, ok := nodeRef.([]any)[0].(string)
		require.Truef(t, ok, "invalid node reference for %s", referenceKey)
		imageNode, ok := workflow[nodeID].(map[string]any)
		require.Truef(t, ok, "missing LoadImage node for %s", referenceKey)
		require.Equal(t, "LoadImage", imageNode["class_type"])
		require.Equal(t, image, imageNode["inputs"].(map[string]any)["image"])
	}
	require.NotContains(t, h3Inputs, "ref_images.ref_image_9")
}

func TestBuildWorkflowSelectsEightStepReferenceWorkflowForImages(t *testing.T) {
	textWorkflow, err := buildWorkflow(
		relaycommon.TaskSubmitReq{Prompt: "a dancer", Seconds: "5"},
		h3Selector{AspectRatio: defaultAspect, Megapixels: 1, Multiple: 32},
		references{},
	)
	require.NoError(t, err)
	textUNET, err := workflowInputs(textWorkflow, "193", "unet_name")
	require.NoError(t, err)
	require.Equal(t, "minimax_h3_fl2va_pruned_int8_convrot.safetensors", textUNET["unet_name"])
	require.Equal(t, "LoraLoaderModelOnly", textWorkflow["220"].(map[string]any)["class_type"])
	require.Equal(t, "MiniMaxH3MemoryProfile", textWorkflow[memoryProfileNodeID].(map[string]any)["class_type"])

	referenceWorkflow, err := buildWorkflow(
		relaycommon.TaskSubmitReq{Prompt: "use <Picture 1> and <Picture 2>", Seconds: "5"},
		h3Selector{AspectRatio: defaultAspect, Megapixels: 1, Multiple: 32},
		references{Images: []string{"one.png", "two.png"}},
	)
	require.NoError(t, err)
	refUNET, err := workflowInputs(referenceWorkflow, "193", "unet_name")
	require.NoError(t, err)
	require.Equal(t, "minimax_h3_fl2va_pruned_int8_convrot.safetensors", refUNET["unet_name"])
	require.Equal(t, "LoraLoaderModelOnly", referenceWorkflow["220"].(map[string]any)["class_type"])
	require.Equal(t, "TESpeedMiniMaxH3", referenceWorkflow[teSpeedNodeID].(map[string]any)["class_type"])
	require.Equal(t, "SolAttnPatch", referenceWorkflow["228"].(map[string]any)["class_type"])
	require.Equal(t, "MiniMaxH3MemoryProfile", referenceWorkflow[memoryProfileNodeID].(map[string]any)["class_type"])

	samplerInputs, err := workflowInputs(referenceWorkflow, "184", "sampler_name")
	require.NoError(t, err)
	require.Equal(t, "euler", samplerInputs["sampler_name"])
	schedulerInputs, err := workflowInputs(referenceWorkflow, "185", "scheduler", "steps", "denoise")
	require.NoError(t, err)
	require.Equal(t, "beta", schedulerInputs["scheduler"])
	require.Equal(t, float64(8), schedulerInputs["steps"])
	require.Equal(t, float64(1), schedulerInputs["denoise"])

	clipInputs, err := workflowInputs(referenceWorkflow, "202", "clip_name")
	require.NoError(t, err)
	require.Equal(t, "qwen3vl_32b_minimax_h3_nvfp4_awq.safetensors", clipInputs["clip_name"])
	teSpeedInputs, err := workflowInputs(referenceWorkflow, teSpeedNodeID, "processing_control_value", "processing_percent_1", "processing_percent_2", "mcs", "device")
	require.NoError(t, err)
	require.InDelta(t, 0.12, teSpeedInputs["processing_control_value"], 0.000001)
	require.InDelta(t, 0.10, teSpeedInputs["processing_percent_1"], 0.000001)
	require.InDelta(t, 0.90, teSpeedInputs["processing_percent_2"], 0.000001)
	require.Equal(t, float64(2), teSpeedInputs["mcs"])
	require.Equal(t, "gpu", teSpeedInputs["device"])
	memoryInputs, err := workflowInputs(referenceWorkflow, memoryProfileNodeID, "profile")
	require.NoError(t, err)
	require.Equal(t, "1MP standard speed", memoryInputs["profile"])
}

func TestBuildWorkflowSelectsFirstLastFrameWorkflow(t *testing.T) {
	workflow, err := buildWorkflow(
		relaycommon.TaskSubmitReq{Prompt: "transition from the first frame to the last frame", Seconds: "5", Mode: firstLastFrameMode},
		h3Selector{AspectRatio: "16:9 (Widescreen)", Megapixels: 1, Multiple: 32},
		references{Images: []string{"first.png", "last.png"}},
	)
	require.NoError(t, err)

	require.Equal(t, "MiniMaxH3ImageToVideo", workflow[h3NodeID].(map[string]any)["class_type"])
	h3Inputs, err := workflowInputs(workflow, h3NodeID, "first_frame", "last_frame")
	require.NoError(t, err)
	require.Equal(t, firstFrameNodeID, workflowReferenceNodeID(t, h3Inputs, "first_frame"))
	require.Equal(t, lastFrameNodeID, workflowReferenceNodeID(t, h3Inputs, "last_frame"))

	firstFrameInputs, err := workflowInputs(workflow, firstFrameNodeID, "image")
	require.NoError(t, err)
	lastFrameInputs, err := workflowInputs(workflow, lastFrameNodeID, "image")
	require.NoError(t, err)
	require.Equal(t, "first.png", firstFrameInputs["image"])
	require.Equal(t, "last.png", lastFrameInputs["image"])
	for key := range h3Inputs {
		require.NotContains(t, key, "ref_images.ref_image_")
	}

	schedulerInputs, err := workflowInputs(workflow, "185", "scheduler", "steps", "denoise")
	require.NoError(t, err)
	require.Equal(t, "beta", schedulerInputs["scheduler"])
	require.Equal(t, float64(8), schedulerInputs["steps"])
	require.Equal(t, float64(1), schedulerInputs["denoise"])
	teSpeedInputs, err := workflowInputs(workflow, teSpeedNodeID, "processing_control_value", "processing_percent_1", "processing_percent_2", "mcs", "device")
	require.NoError(t, err)
	require.InDelta(t, 0.12, teSpeedInputs["processing_control_value"], 0.000001)
	require.InDelta(t, 0.10, teSpeedInputs["processing_percent_1"], 0.000001)
	require.InDelta(t, 0.90, teSpeedInputs["processing_percent_2"], 0.000001)
	require.Equal(t, float64(2), teSpeedInputs["mcs"])
	require.Equal(t, "gpu", teSpeedInputs["device"])
}

func TestBuildWorkflowFirstLastFrameModeMatchesTESpeedDeviceToFinalProfile(t *testing.T) {
	tests := []struct {
		name       string
		mode       string
		megapixels float64
		seconds    int
		profile    string
		device     string
	}{
		{name: "one mp standard profile uses gpu", mode: firstLastFrameMode, megapixels: 1, seconds: 5, profile: "1MP standard speed", device: "gpu"},
		{name: "two mp low vram profile uses cpu", mode: "first-last-frame", megapixels: 2, seconds: 10, profile: "2MP low VRAM", device: "cpu"},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			workflow, err := buildWorkflow(
				relaycommon.TaskSubmitReq{Prompt: "animate between frames", Duration: testCase.seconds, Mode: testCase.mode},
				h3Selector{AspectRatio: defaultAspect, Megapixels: testCase.megapixels, Multiple: 32},
				references{Images: []string{"first.png", "last.png"}},
			)
			require.NoError(t, err)
			memoryInputs, err := workflowInputs(workflow, memoryProfileNodeID, "profile")
			require.NoError(t, err)
			require.Equal(t, testCase.profile, memoryInputs["profile"])
			teSpeedInputs, err := workflowInputs(workflow, teSpeedNodeID, "device")
			require.NoError(t, err)
			require.Equal(t, testCase.device, teSpeedInputs["device"])
		})
	}
}

func TestBuildWorkflowFirstLastFrameModeValidatesReferences(t *testing.T) {
	tests := []struct {
		name       string
		references references
		errorText  string
	}{
		{name: "missing last frame", references: references{Images: []string{"first.png"}}, errorText: "requires exactly 2 images"},
		{name: "too many frame images", references: references{Images: []string{"first.png", "last.png", "extra.png"}}, errorText: "requires exactly 2 images"},
		{name: "reference video is unsupported", references: references{Images: []string{"first.png", "last.png"}, Videos: []string{"motion.mp4"}}, errorText: "supports only the 2 frame images"},
		{name: "reference audio is unsupported", references: references{Images: []string{"first.png", "last.png"}, Audios: []string{"music.mp3"}}, errorText: "supports only the 2 frame images"},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := buildWorkflow(
				relaycommon.TaskSubmitReq{Prompt: "animate between frames", Seconds: "5", Mode: firstLastFrameMode},
				h3Selector{AspectRatio: defaultAspect, Megapixels: 1, Multiple: 32},
				testCase.references,
			)
			require.ErrorContains(t, err, testCase.errorText)
		})
	}
}

func TestBuildWorkflowRejectsUnsupportedMode(t *testing.T) {
	_, err := buildWorkflow(
		relaycommon.TaskSubmitReq{Prompt: "animate between frames", Seconds: "5", Mode: "first_last_fram"},
		h3Selector{AspectRatio: defaultAspect, Megapixels: 1, Multiple: 32},
		references{Images: []string{"first.png", "last.png"}},
	)
	require.ErrorContains(t, err, "unsupported ComfyUI H3 mode")
}

func TestBuildWorkflowMatchesTESpeedDeviceToFinalMemoryProfile(t *testing.T) {
	tests := []struct {
		name       string
		megapixels float64
		seconds    int
		images     []string
		profile    string
		device     string
	}{
		{name: "one mp low load uses gpu", megapixels: 1, seconds: 5, images: []string{"one.png"}, profile: "1MP standard speed", device: "gpu"},
		{name: "two mp low load still uses gpu", megapixels: 2, seconds: 5, images: []string{"one.png"}, profile: "1MP standard speed", device: "gpu"},
		{name: "one mp high load uses cpu", megapixels: 1, seconds: 19, images: []string{"one.png"}, profile: "2MP low VRAM", device: "cpu"},
		{name: "two mp high load uses cpu", megapixels: 2, seconds: 10, images: []string{"one.png"}, profile: "2MP low VRAM", device: "cpu"},
		{name: "five references force cpu", megapixels: 1, seconds: 5, images: []string{"1", "2", "3", "4", "5"}, profile: "2MP low VRAM", device: "cpu"},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			workflow, err := buildWorkflow(
				relaycommon.TaskSubmitReq{Prompt: "reference video", Duration: testCase.seconds},
				h3Selector{AspectRatio: defaultAspect, Megapixels: testCase.megapixels, Multiple: 32},
				references{Images: testCase.images},
			)
			require.NoError(t, err)
			memoryInputs, err := workflowInputs(workflow, memoryProfileNodeID, "profile")
			require.NoError(t, err)
			require.Equal(t, testCase.profile, memoryInputs["profile"])
			teSpeedInputs, err := workflowInputs(workflow, teSpeedNodeID, "device")
			require.NoError(t, err)
			require.Equal(t, testCase.device, teSpeedInputs["device"])
		})
	}
}

func TestBuildWorkflowGeneratesFreshNoiseSeed(t *testing.T) {
	req := relaycommon.TaskSubmitReq{Prompt: "a dancer on a bright stage", Seconds: "5"}
	selector := h3Selector{AspectRatio: defaultAspect, Megapixels: defaultMegapixels, Multiple: defaultMultiple}

	firstWorkflow, err := buildWorkflow(req, selector, references{})
	require.NoError(t, err)
	secondWorkflow, err := buildWorkflow(req, selector, references{})
	require.NoError(t, err)

	firstInputs, err := workflowInputs(firstWorkflow, noiseNodeID, "noise_seed")
	require.NoError(t, err)
	secondInputs, err := workflowInputs(secondWorkflow, noiseNodeID, "noise_seed")
	require.NoError(t, err)
	firstSeed, ok := firstInputs["noise_seed"].(int64)
	require.True(t, ok)
	secondSeed, ok := secondInputs["noise_seed"].(int64)
	require.True(t, ok)

	require.NotEqual(t, int64(1059763349863698), firstSeed)
	require.NotEqual(t, firstSeed, secondSeed)
	require.GreaterOrEqual(t, firstSeed, int64(0))
	require.Less(t, firstSeed, maxH3NoiseSeed)
	require.GreaterOrEqual(t, secondSeed, int64(0))
	require.Less(t, secondSeed, maxH3NoiseSeed)
}

func TestOverridePriceDataStaleUserPricingPlanFallsBackToUsingGroup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	withComfyH3GroupPrices(t, map[string]h3GroupPrice{
		"using-group": {Price768P: 0.10, Price2K: 0.30},
	})
	withComfyUSDExchangeRate(t, 7.3)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Set("task_request", relaycommon.TaskSubmitReq{Seconds: "5"})

	priceData, ok, err := (&TaskAdaptor{}).OverridePriceData(ctx, &relaycommon.RelayInfo{
		OriginModelName: modelName,
		UsingGroup:      "using-group",
		UserSetting:     dto.UserSetting{RunningHubH3PriceGroup: "deleted-plan"},
	})

	require.NoError(t, err)
	require.True(t, ok)
	require.InDelta(t, 0.10/7.3, priceData.ModelPrice, 0.000001)
	require.Equal(t, 1.0, priceData.GroupRatioInfo.GroupRatio)
}

func TestOverridePriceDataMismatchedUserPricingPlanFallsBackToUsingGroup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	withComfyH3GroupPrices(t, map[string]h3GroupPrice{
		"using-group":   {Price768P: 0.10, Price2K: 0.30},
		"assigned-tier": {Price768P: 0.40, Price2K: 1.20, BoundGroup: "other-group"},
	})
	withComfyUSDExchangeRate(t, 7.3)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Set("task_request", relaycommon.TaskSubmitReq{Seconds: "5"})

	priceData, ok, err := (&TaskAdaptor{}).OverridePriceData(ctx, &relaycommon.RelayInfo{
		OriginModelName: modelName,
		UsingGroup:      "using-group",
		UserSetting:     dto.UserSetting{RunningHubH3PriceGroup: "assigned-tier"},
	})

	require.NoError(t, err)
	require.True(t, ok)
	require.InDelta(t, 0.10/7.3, priceData.ModelPrice, 0.000001)
}

func TestBuildWorkflowRemovesTemplateReferenceForTextOnlyRequest(t *testing.T) {
	workflow, err := buildWorkflow(
		relaycommon.TaskSubmitReq{Prompt: "a dancing woman", Seconds: "5"},
		h3Selector{AspectRatio: defaultAspect, Megapixels: defaultMegapixels, Multiple: defaultMultiple},
		references{},
	)
	require.NoError(t, err)
	h3Inputs, err := workflowInputs(workflow, h3NodeID)
	require.NoError(t, err)
	for key := range h3Inputs {
		require.NotContains(t, key, "ref_images.ref_image_")
	}
}

func TestBuildWorkflowSelectsMemoryProfileByRequestLoad(t *testing.T) {
	tests := []struct {
		name       string
		megapixels float64
		seconds    int
		references references
		profile    string
	}{
		{name: "equal threshold uses standard speed", megapixels: 1, seconds: 18, profile: "1MP standard speed"},
		{name: "above threshold uses low vram", megapixels: 1, seconds: 19, profile: "2MP low VRAM"},
		{name: "two mp at nine seconds uses standard speed", megapixels: 2, seconds: 9, profile: "1MP standard speed"},
		{name: "two mp above nine seconds uses low vram", megapixels: 2, seconds: 10, profile: "2MP low VRAM"},
		{name: "reference video uses low vram", megapixels: 1, seconds: 5, references: references{Videos: []string{"reference.mp4"}}, profile: "2MP low VRAM"},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			workflow, err := buildWorkflow(
				relaycommon.TaskSubmitReq{Duration: testCase.seconds},
				h3Selector{AspectRatio: defaultAspect, Megapixels: testCase.megapixels, Multiple: defaultMultiple},
				testCase.references,
			)
			require.NoError(t, err)
			memoryInputs, err := workflowInputs(workflow, memoryProfileNodeID, "profile")
			require.NoError(t, err)
			require.Equal(t, testCase.profile, memoryInputs["profile"])
		})
	}
}

func TestBuildWorkflowPreservesEveryUpstreamAspectRatioLabel(t *testing.T) {
	for _, aspectRatio := range []string{
		"1:1 (Square)",
		"2:3 (Portrait Photo)",
		"3:2 (Photo)",
		"3:4 (Portrait Standard)",
		"4:3 (Standard)",
		"9:16 (Portrait Widescreen)",
		"16:9 (Widescreen)",
		"21:9 (Ultrawide)",
	} {
		t.Run(aspectRatio, func(t *testing.T) {
			req := relaycommon.TaskSubmitReq{
				Duration: 5,
				Metadata: map[string]any{"aspect_ratio": aspectRatio},
			}
			selector, err := selectorFromRequest(req)
			require.NoError(t, err)
			require.Equal(t, aspectRatio, selector.AspectRatio)

			workflow, err := buildWorkflow(req, selector, references{})
			require.NoError(t, err)
			paramsInputs, err := workflowInputs(workflow, paramsNodeID, "aspect_ratio")
			require.NoError(t, err)
			require.Equal(t, aspectRatio, paramsInputs["aspect_ratio"])
		})
	}
}

func TestGetTaskOutputMetadataUsesFinalH3Parameters(t *testing.T) {
	tests := []struct {
		name     string
		req      relaycommon.TaskSubmitReq
		seconds  int
		expected string
	}{
		{
			name: "one mp widescreen",
			req: relaycommon.TaskSubmitReq{
				Seconds:  "5",
				Metadata: map[string]any{"aspect_ratio": "16:9", "megapixels": "1.0"},
			},
			seconds:  5,
			expected: "1376x768",
		},
		{
			name: "two mp widescreen",
			req: relaycommon.TaskSubmitReq{
				Duration: 15,
				Metadata: map[string]any{"aspect_ratio": "16:9", "megapixels": "2.0"},
			},
			seconds:  15,
			expected: "1920x1088",
		},
		{
			name: "duration overrides seconds and multiple is honored",
			req: relaycommon.TaskSubmitReq{
				Duration: 9,
				Seconds:  "20",
				Metadata: map[string]any{"aspect_ratio": "16:9", "megapixels": "1.0", "multiple": "64"},
			},
			seconds:  9,
			expected: "1344x768",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Set("task_request", testCase.req)

			seconds, size, err := (&TaskAdaptor{}).GetTaskOutputMetadata(ctx, nil)

			require.NoError(t, err)
			require.Equal(t, testCase.seconds, seconds)
			require.Equal(t, testCase.expected, size)
		})
	}
}

func TestH3OutputDimensionsCoversEveryAspectRatio(t *testing.T) {
	tests := []struct {
		aspect    string
		oneMPSize string
		twoMPSize string
	}{
		{aspect: "1:1 (Square)", oneMPSize: "1024x1024", twoMPSize: "1440x1440"},
		{aspect: "2:3 (Portrait Photo)", oneMPSize: "832x1248", twoMPSize: "1184x1760"},
		{aspect: "3:2 (Photo)", oneMPSize: "1248x832", twoMPSize: "1760x1184"},
		{aspect: "3:4 (Portrait Standard)", oneMPSize: "896x1184", twoMPSize: "1248x1664"},
		{aspect: "4:3 (Standard)", oneMPSize: "1184x896", twoMPSize: "1664x1248"},
		{aspect: "9:16 (Portrait Widescreen)", oneMPSize: "768x1376", twoMPSize: "1088x1920"},
		{aspect: "16:9 (Widescreen)", oneMPSize: "1376x768", twoMPSize: "1920x1088"},
		{aspect: "21:9 (Ultrawide)", oneMPSize: "1568x672", twoMPSize: "2208x960"},
	}

	for _, testCase := range tests {
		for _, preset := range []struct {
			name       string
			megapixels float64
			size       string
		}{
			{name: "1MP", megapixels: 1, size: testCase.oneMPSize},
			{name: "2MP", megapixels: 2, size: testCase.twoMPSize},
		} {
			t.Run(testCase.aspect+"/"+preset.name, func(t *testing.T) {
				width, height, ok := h3OutputDimensions(h3Selector{
					AspectRatio: testCase.aspect,
					Megapixels:  preset.megapixels,
					Multiple:    32,
				})
				require.True(t, ok)
				require.Equal(t, preset.size, fmt.Sprintf("%dx%d", width, height))
			})
		}
	}
}

func TestGetTaskOutputMetadataRejectsInvalidRequestParameters(t *testing.T) {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Set("task_request", relaycommon.TaskSubmitReq{
		Seconds:  "invalid",
		Metadata: map[string]any{"aspect_ratio": "16:9", "megapixels": "1.0"},
	})

	seconds, size, err := (&TaskAdaptor{}).GetTaskOutputMetadata(ctx, nil)

	require.Error(t, err)
	require.Zero(t, seconds)
	require.Empty(t, size)
}

func TestParseTaskResultExtractsVideoURL(t *testing.T) {
	adaptor := &TaskAdaptor{baseURL: "http://comfy.example"}
	taskInfo, err := adaptor.ParseTaskResult([]byte(`{
  "prompt-id": {
    "status": {"status_str": "success"},
    "outputs": {
      "210": {
        "gifs": [{"filename": "result.mp4", "subfolder": "videos/h3", "type": "output"}]
      }
    }
  }
}`))
	require.NoError(t, err)
	require.Equal(t, string(model.TaskStatusSuccess), taskInfo.Status)
	parsedURL, err := url.Parse(taskInfo.Url)
	require.NoError(t, err)
	require.Equal(t, "/view", parsedURL.Path)
	require.Equal(t, "result.mp4", parsedURL.Query().Get("filename"))
	require.Equal(t, "videos/h3", parsedURL.Query().Get("subfolder"))
	require.Equal(t, "output", parsedURL.Query().Get("type"))
}

func TestParseTaskResultReportsQueuedAndFailure(t *testing.T) {
	adaptor := &TaskAdaptor{baseURL: "http://comfy.example"}
	queued, err := adaptor.ParseTaskResult([]byte(`{}`))
	require.NoError(t, err)
	require.Equal(t, string(model.TaskStatusQueued), queued.Status)

	failed, err := adaptor.ParseTaskResult([]byte(`{
  "prompt-id": {"status": {"status_str": "error", "messages": [["execution_error", {"exception_message": "out of memory"}]]}}
}`))
	require.NoError(t, err)
	require.Equal(t, string(model.TaskStatusFailure), failed.Status)
	require.Contains(t, failed.Reason, "out of memory")
}

func TestParseTaskResultRequiresFinalVideoOutput(t *testing.T) {
	adaptor := &TaskAdaptor{baseURL: "http://comfy.example"}
	taskInfo, err := adaptor.ParseTaskResult([]byte(`{
  "prompt-id": {
    "status": {"status_str": "success"},
    "outputs": {
      "100": {"gifs": [{"filename": "intermediate.mp4", "type": "output"}]},
      "210": {"images": [{"filename": "preview.png", "type": "output"}]}
    }
  }
}`))
	require.NoError(t, err)
	require.Equal(t, string(model.TaskStatusFailure), taskInfo.Status)
	require.Contains(t, taskInfo.Reason, "node 210")
}

func TestFetchTaskUsesPromptHistoryEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/history/prompt-123", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	adaptor := &TaskAdaptor{}
	response, err := adaptor.FetchTask(server.URL, "ignored", map[string]any{"task_id": "prompt-123"}, "")
	require.NoError(t, err)
	defer response.Body.Close()
	require.Equal(t, http.StatusOK, response.StatusCode)
}

func TestGatewayBuildRequestUsesAuthenticatedEnvelopeAndStableRetryBody(t *testing.T) {
	disabled := false
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Set("task_request", relaycommon.TaskSubmitReq{
		Prompt:        "a dancer on a bright stage",
		Seconds:       "5",
		PromptEnhance: &disabled,
	})
	info := gatewayRelayInfo("http://gateway.example:8090", "gateway-token", "task_public_123")
	adaptor := &TaskAdaptor{}
	adaptor.Init(info)

	requestURL, err := adaptor.BuildRequestURL(info)
	require.NoError(t, err)
	require.Equal(t, "http://gateway.example:8090/api/gateway/tasks", requestURL)

	req, err := http.NewRequest(http.MethodPost, requestURL, nil)
	require.NoError(t, err)
	require.NoError(t, adaptor.BuildRequestHeader(ctx, req, info))
	require.Equal(t, "Bearer gateway-token", req.Header.Get("Authorization"))

	body, err := adaptor.BuildRequestBody(ctx, info)
	require.NoError(t, err)
	firstBody, err := io.ReadAll(body)
	require.NoError(t, err)
	var envelope map[string]any
	require.NoError(t, common.Unmarshal(firstBody, &envelope))
	require.Equal(t, "task_public_123", envelope["task_key"])
	require.Equal(t, "task_public_123", envelope["idempotency_key"])
	require.Empty(t, envelope["upload_ids"])
	workflow, ok := envelope["workflow"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "newapi-task_public_123", workflow["client_id"])

	retryBody, err := adaptor.BuildRequestBody(ctx, info)
	require.NoError(t, err)
	secondBody, err := io.ReadAll(retryBody)
	require.NoError(t, err)
	require.Equal(t, firstBody, secondBody)
}

func TestGatewayBuildRequestUploadsAllImagesAndSubmitsOneReferenceWorkflow(t *testing.T) {
	uploadCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, gatewayUploadsPath, r.URL.Path)
		require.Equal(t, "Bearer gateway-token", r.Header.Get("Authorization"))
		require.NoError(t, r.ParseMultipartForm(1024*1024))
		file, _, err := r.FormFile("file")
		require.NoError(t, err)
		_ = file.Close()
		uploadCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprintf(w, `{"upload_id":"upload-%d","file":{"name":"gateway-image-%d.png"}}`, uploadCount, uploadCount)
	}))
	defer server.Close()

	disabled := false
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Set("task_request", relaycommon.TaskSubmitReq{
		Prompt:        "use <Picture 1> and <Picture 2>",
		Seconds:       "10",
		Size:          "1920x1088",
		PromptEnhance: &disabled,
		Metadata: map[string]any{
			"reference_images": []any{
				"data:image/png;base64,b25l",
				"data:image/png;base64,dHdv",
			},
		},
	})
	info := gatewayRelayInfo(server.URL, "gateway-token", "task_public_multi_ref")
	adaptor := &TaskAdaptor{}
	adaptor.Init(info)

	body, err := adaptor.BuildRequestBody(ctx, info)
	require.NoError(t, err)
	payload, err := io.ReadAll(body)
	require.NoError(t, err)
	require.Equal(t, 2, uploadCount)

	var envelope map[string]any
	require.NoError(t, common.Unmarshal(payload, &envelope))
	require.Equal(t, []any{"upload-1", "upload-2"}, envelope["upload_ids"])
	workflowEnvelope := envelope["workflow"].(map[string]any)
	prompt := workflowEnvelope["prompt"].(map[string]any)
	unetInputs := prompt["193"].(map[string]any)["inputs"].(map[string]any)
	require.Equal(t, "minimax_h3_fl2va_pruned_int8_convrot.safetensors", unetInputs["unet_name"])
	schedulerInputs := prompt["185"].(map[string]any)["inputs"].(map[string]any)
	require.Equal(t, float64(8), schedulerInputs["steps"])
	teSpeedInputs := prompt[teSpeedNodeID].(map[string]any)["inputs"].(map[string]any)
	require.Equal(t, "cpu", teSpeedInputs["device"])
	memoryProfileInputs := prompt[memoryProfileNodeID].(map[string]any)["inputs"].(map[string]any)
	require.Equal(t, "2MP low VRAM", memoryProfileInputs["profile"])
	h3Inputs := prompt[h3NodeID].(map[string]any)["inputs"].(map[string]any)
	require.Contains(t, h3Inputs, "ref_images.ref_image_0")
	require.Contains(t, h3Inputs, "ref_images.ref_image_1")
	require.NotContains(t, h3Inputs, "ref_images.ref_image_2")
	require.NotContains(t, prompt, "195")
}

func TestGatewayBuildRequestMapsTwoUploadsToFirstAndLastFrames(t *testing.T) {
	uploadCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, gatewayUploadsPath, r.URL.Path)
		require.Equal(t, "Bearer gateway-token", r.Header.Get("Authorization"))
		require.NoError(t, r.ParseMultipartForm(1024*1024))
		file, _, err := r.FormFile("file")
		require.NoError(t, err)
		_ = file.Close()
		uploadCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprintf(w, `{"upload_id":"upload-%d","file":{"name":"gateway-frame-%d.png"}}`, uploadCount, uploadCount)
	}))
	defer server.Close()

	disabled := false
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Set("task_request", relaycommon.TaskSubmitReq{
		Prompt:        "move smoothly from the first frame to the last frame",
		Seconds:       "5",
		Size:          "1376x768",
		Mode:          firstLastFrameMode,
		PromptEnhance: &disabled,
		Images: []string{
			"data:image/png;base64,Zmlyc3Q=",
			"data:image/png;base64,bGFzdA==",
		},
	})
	info := gatewayRelayInfo(server.URL, "gateway-token", "task_public_first_last")
	adaptor := &TaskAdaptor{}
	adaptor.Init(info)

	body, err := adaptor.BuildRequestBody(ctx, info)
	require.NoError(t, err)
	payload, err := io.ReadAll(body)
	require.NoError(t, err)
	require.Equal(t, 2, uploadCount)

	var envelope map[string]any
	require.NoError(t, common.Unmarshal(payload, &envelope))
	require.Equal(t, []any{"upload-1", "upload-2"}, envelope["upload_ids"])
	prompt := envelope["workflow"].(map[string]any)["prompt"].(map[string]any)
	require.Equal(t, "MiniMaxH3ImageToVideo", prompt[h3NodeID].(map[string]any)["class_type"])
	require.Equal(t, "gateway-frame-1.png", prompt[firstFrameNodeID].(map[string]any)["inputs"].(map[string]any)["image"])
	require.Equal(t, "gateway-frame-2.png", prompt[lastFrameNodeID].(map[string]any)["inputs"].(map[string]any)["image"])
	h3Inputs := prompt[h3NodeID].(map[string]any)["inputs"].(map[string]any)
	require.Equal(t, firstFrameNodeID, workflowReferenceNodeID(t, h3Inputs, "first_frame"))
	require.Equal(t, lastFrameNodeID, workflowReferenceNodeID(t, h3Inputs, "last_frame"))
	for key := range h3Inputs {
		require.NotContains(t, key, "ref_images.ref_image_")
	}
}

func TestGatewayBuildRequestRejectsIncompleteFirstLastFramesBeforeUpload(t *testing.T) {
	uploadCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uploadCount++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Set("task_request", relaycommon.TaskSubmitReq{
		Prompt:  "move smoothly between frames",
		Seconds: "5",
		Mode:    firstLastFrameMode,
		Images:  []string{"data:image/png;base64,Zmlyc3Q="},
	})
	info := gatewayRelayInfo(server.URL, "gateway-token", "task_public_missing_last")
	adaptor := &TaskAdaptor{}
	adaptor.Init(info)

	_, err := adaptor.BuildRequestBody(ctx, info)
	require.ErrorContains(t, err, "requires exactly 2 images")
	require.Zero(t, uploadCount)
}

func TestGatewayBuildRequestRejectsUnsupportedModeBeforeUpload(t *testing.T) {
	uploadCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uploadCount++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Set("task_request", relaycommon.TaskSubmitReq{
		Prompt:  "move smoothly between frames",
		Seconds: "5",
		Mode:    "first_last_fram",
		Images: []string{
			"data:image/png;base64,Zmlyc3Q=",
			"data:image/png;base64,bGFzdA==",
		},
	})
	info := gatewayRelayInfo(server.URL, "gateway-token", "task_public_bad_mode")
	adaptor := &TaskAdaptor{}
	adaptor.Init(info)

	_, err := adaptor.BuildRequestBody(ctx, info)
	require.ErrorContains(t, err, "unsupported ComfyUI H3 mode")
	require.Zero(t, uploadCount)
}

func TestGatewayUploadReferenceUsesFileFieldAndTracksUploadID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, gatewayUploadsPath, r.URL.Path)
		require.Equal(t, "Bearer gateway-token", r.Header.Get("Authorization"))
		require.NoError(t, r.ParseMultipartForm(1024*1024))
		file, header, err := r.FormFile("file")
		require.NoError(t, err)
		defer file.Close()
		contents, err := io.ReadAll(file)
		require.NoError(t, err)
		require.Equal(t, []byte("image data"), contents)
		require.Equal(t, "input", r.FormValue("type"))
		require.Empty(t, r.FormValue("overwrite"))
		require.Equal(t, ".png", filepath.Ext(header.Filename))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"upload_id":"upload-123","file":{"name":"gateway-input.png"}}`))
	}))
	defer server.Close()

	adaptor := &TaskAdaptor{baseURL: server.URL, gatewayURL: server.URL, gatewayAPIKey: "gateway-token"}
	fileName, err := adaptor.uploadReference(referenceInput{Name: "image.png", Data: []byte("image data")})
	require.NoError(t, err)
	require.Equal(t, "gateway-input.png", fileName)
	require.Equal(t, []string{"upload-123"}, adaptor.gatewayUploadIDs)
}

func TestGatewayCollectReferencesAcceptsMultipartInputReference(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, gatewayUploadsPath, r.URL.Path)
		require.Equal(t, "Bearer gateway-token", r.Header.Get("Authorization"))
		require.NoError(t, r.ParseMultipartForm(1024*1024))
		file, _, err := r.FormFile("file")
		require.NoError(t, err)
		defer file.Close()
		contents, err := io.ReadAll(file)
		require.NoError(t, err)
		require.Equal(t, []byte("reference image"), contents)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"upload_id":"upload-123","file":{"name":"gateway-input.png"}}`))
	}))
	defer server.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("input_reference", "reference.png")
	require.NoError(t, err)
	_, err = part.Write([]byte("reference image"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	request := httptest.NewRequest(http.MethodPost, "/v1/videos", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	require.NoError(t, request.ParseMultipartForm(1024*1024))
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = request

	adaptor := &TaskAdaptor{baseURL: server.URL, gatewayURL: server.URL, gatewayAPIKey: "gateway-token"}
	references, err := adaptor.collectReferences(ctx, relaycommon.TaskSubmitReq{})
	require.NoError(t, err)
	require.Equal(t, []string{"gateway-input.png"}, references.Images)
	require.Equal(t, []string{"upload-123"}, adaptor.gatewayUploadIDs)
}

func TestGatewayCollectReferencesRejectsReferenceVideoOverFifteenSecondsBeforeUpload(t *testing.T) {
	withH3ReferenceVideoDurationProbe(t, func(referenceInput) (float64, error) {
		return 60, nil
	})

	uploadCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uploadCalls++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("reference_video", "too-long.mp4")
	require.NoError(t, err)
	_, err = part.Write([]byte("reference video"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	request := httptest.NewRequest(http.MethodPost, "/v1/videos", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	require.NoError(t, request.ParseMultipartForm(1024*1024))
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = request

	adaptor := &TaskAdaptor{baseURL: server.URL, gatewayURL: server.URL, gatewayAPIKey: "gateway-token"}
	_, err = adaptor.collectReferences(ctx, relaycommon.TaskSubmitReq{})
	require.ErrorContains(t, err, "maximum is 15 seconds")
	require.Zero(t, uploadCalls)
}

func TestGatewayRejectsUnuploadedReferenceFileNames(t *testing.T) {
	adaptor := &TaskAdaptor{gatewayURL: "http://gateway.example:8090"}
	_, err := adaptor.resolveReferenceValues([]string{"old-worker-file.png"})
	require.ErrorContains(t, err, "URLs, data URLs, or multipart uploads")
}

func TestGatewayDoResponseAcceptsAsyncTask(t *testing.T) {
	writer := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(writer)
	adaptor := &TaskAdaptor{gatewayURL: "http://gateway.example:8090", baseURL: "http://gateway.example:8090"}
	info := gatewayRelayInfo("http://gateway.example:8090", "gateway-token", "task_public_456")
	response := &http.Response{
		StatusCode: http.StatusAccepted,
		Status:     "202 Accepted",
		Body:       io.NopCloser(bytes.NewBufferString(`{"task_id":"gateway-task-456","status":"waiting"}`)),
	}

	upstreamTaskID, taskData, taskErr := adaptor.DoResponse(ctx, response, info)
	require.Nil(t, taskErr)
	require.Equal(t, "gateway-task-456", upstreamTaskID)
	require.JSONEq(t, `{"task_id":"gateway-task-456","status":"waiting"}`, string(taskData))
	require.Equal(t, http.StatusOK, writer.Code)
	require.Contains(t, writer.Body.String(), "task_public_456")
}

func TestGatewayFetchTaskUsesAuthenticatedGatewayEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/api/gateway/tasks/gateway-task-789", r.URL.Path)
		require.Equal(t, "Bearer gateway-token", r.Header.Get("Authorization"))
		require.Equal(t, "application/json", r.Header.Get("Accept"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"task_id":"gateway-task-789","status":"queued"}`))
	}))
	defer server.Close()

	adaptor := &TaskAdaptor{gatewayURL: server.URL, gatewayAPIKey: "gateway-token"}
	response, err := adaptor.FetchTask(server.URL, "gateway-token", map[string]any{"task_id": "gateway-task-789"}, "")
	require.NoError(t, err)
	defer response.Body.Close()
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.Equal(t, "gateway-task-789", adaptor.gatewayPollingTaskID)
}

func TestGatewayParseTaskResultMapsStatesAndBuildsCanonicalResultURL(t *testing.T) {
	adaptor := &TaskAdaptor{
		baseURL:              "http://gateway.example:8090",
		gatewayURL:           "http://gateway.example:8090",
		gatewayPollingTaskID: "gateway-task-999",
	}
	tests := []struct {
		name          string
		payload       string
		status        model.TaskStatus
		progress      string
		resultURL     string
		reasonContain string
	}{
		{name: "waiting", payload: `{"task_id":"gateway-task-999","status":"waiting"}`, status: model.TaskStatusQueued, progress: "20%"},
		{name: "dispatching", payload: `{"task_id":"gateway-task-999","status":"dispatching"}`, status: model.TaskStatusQueued, progress: "20%"},
		{name: "queued", payload: `{"task_id":"gateway-task-999","status":"queued"}`, status: model.TaskStatusQueued, progress: "20%"},
		{name: "node unavailable", payload: `{"task_id":"gateway-task-999","status":"node_unavailable"}`, status: model.TaskStatusQueued, progress: "20%"},
		{name: "running", payload: `{"task_id":"gateway-task-999","status":"running"}`, status: model.TaskStatusInProgress, progress: "50%"},
		{name: "completed before result", payload: `{"task_id":"gateway-task-999","status":"completed","result_ready":false}`, status: model.TaskStatusInProgress, progress: "90%"},
		{name: "completed result", payload: `{"task_id":"gateway-task-999","status":"completed","result_ready":true,"view_endpoint":"http://untrusted.example/video"}`, status: model.TaskStatusSuccess, progress: "100%", resultURL: "http://gateway.example:8090/api/gateway/tasks/gateway-task-999/view"},
		{name: "failed", payload: `{"task_id":"gateway-task-999","status":"failed","message":"worker failed"}`, status: model.TaskStatusFailure, progress: "100%", reasonContain: "worker failed"},
		{name: "expired", payload: `{"task_id":"gateway-task-999","status":"expired"}`, status: model.TaskStatusFailure, progress: "100%", reasonContain: "expired"},
		{name: "cancelled", payload: `{"task_id":"gateway-task-999","status":"cancelled"}`, status: model.TaskStatusFailure, progress: "100%", reasonContain: "cancelled"},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			result, err := adaptor.ParseTaskResult([]byte(testCase.payload))
			require.NoError(t, err)
			require.Equal(t, string(testCase.status), result.Status)
			require.Equal(t, testCase.progress, result.Progress)
			require.Equal(t, testCase.resultURL, result.Url)
			if testCase.reasonContain != "" {
				require.Contains(t, result.Reason, testCase.reasonContain)
			}
		})
	}
}

func TestConvertToOpenAIVideoPreservesSignedContentURLQuerySeparators(t *testing.T) {
	adaptor := &TaskAdaptor{}
	task := &model.Task{
		TaskID: "task_signed_url",
		Status: model.TaskStatusSuccess,
		PrivateData: model.TaskPrivateData{
			ResultURL: "https://api.example.com/v1/videos/task_signed_url/content?expires=1&signature=abc&user_id=42",
		},
	}

	data, err := adaptor.ConvertToOpenAIVideo(task)
	require.NoError(t, err)
	require.Contains(t, string(data), "&signature=abc&user_id=42")
	require.NotContains(t, string(data), `\u0026`)
}

func gatewayRelayInfo(gatewayURL, apiKey, publicTaskID string) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl:       "http://direct-worker.example:5900",
			ApiKey:               apiKey,
			ChannelOtherSettings: dto.ChannelOtherSettings{ComfyUIH3GatewayURL: gatewayURL},
		},
		TaskRelayInfo:   &relaycommon.TaskRelayInfo{PublicTaskID: publicTaskID},
		OriginModelName: modelName,
	}
}

func TestReferenceInputsAcceptsVideoAndAudioValues(t *testing.T) {
	sources, err := referenceInputs(nil, relaycommon.TaskSubmitReq{Metadata: map[string]any{
		"reference_images":       []any{"one.png", "two.png"},
		"reference_videos":       []any{"reference.mp4"},
		"reference_video_audios": []any{"paired.mp3"},
		"reference_audios":       []any{"voice.mp3"},
	}})
	require.NoError(t, err)
	require.Equal(t, []string{"one.png", "two.png"}, sources.images)
	require.Equal(t, []string{"reference.mp4"}, sources.videos)
	require.Equal(t, []string{"paired.mp3"}, sources.videoAudios)
	require.Equal(t, []string{"voice.mp3"}, sources.audios)
}

func TestValidateH3ReferenceVideoDurationAcceptsFifteenSecondsAndRejectsLonger(t *testing.T) {
	withH3ReferenceVideoDurationProbe(t, func(referenceInput) (float64, error) {
		return 15, nil
	})
	require.NoError(t, validateH3ReferenceVideoDuration(referenceInput{Name: "fifteen.mp4", Data: []byte("video")}))

	probeH3ReferenceVideoDuration = func(referenceInput) (float64, error) {
		return 15.11, nil
	}
	err := validateH3ReferenceVideoDuration(referenceInput{Name: "too-long.mp4", Data: []byte("video")})
	require.ErrorContains(t, err, "maximum is 15 seconds")
}

func TestUploadReferenceUsesUniqueFileNames(t *testing.T) {
	var uploadedNames []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseMultipartForm(1024*1024))
		file, header, err := r.FormFile("image")
		require.NoError(t, err)
		_ = file.Close()
		uploadedNames = append(uploadedNames, header.Filename)
		require.Equal(t, "false", r.FormValue("overwrite"))
		_, _ = w.Write([]byte(`{"name":"` + header.Filename + `"}`))
	}))
	defer server.Close()

	adaptor := &TaskAdaptor{baseURL: server.URL}
	first, err := adaptor.uploadReference(referenceInput{Name: "image.png", Data: []byte("one")})
	require.NoError(t, err)
	second, err := adaptor.uploadReference(referenceInput{Name: "image.png", Data: []byte("two")})
	require.NoError(t, err)
	require.Len(t, uploadedNames, 2)
	require.NotEqual(t, uploadedNames[0], uploadedNames[1])
	require.NotEqual(t, first, second)
	require.Equal(t, ".png", filepath.Ext(uploadedNames[0]))
	require.Equal(t, ".png", filepath.Ext(uploadedNames[1]))
}

func TestResolveReferenceValuesUploadsImageDataURL(t *testing.T) {
	var uploaded []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, uploadPath, r.URL.Path)
		require.NoError(t, r.ParseMultipartForm(1024*1024))
		file, header, err := r.FormFile("image")
		require.NoError(t, err)
		defer file.Close()
		uploaded, err = io.ReadAll(file)
		require.NoError(t, err)
		require.Equal(t, ".png", filepath.Ext(header.Filename))
		_, _ = w.Write([]byte(`{"name":"` + header.Filename + `"}`))
	}))
	defer server.Close()

	adaptor := &TaskAdaptor{baseURL: server.URL}
	resolved, err := adaptor.resolveReferenceValues([]string{
		"data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=",
	})
	require.NoError(t, err)
	require.Len(t, resolved, 1)
	require.NotEmpty(t, uploaded)
	require.Equal(t, []byte("\x89PNG\r\n\x1a\n"), uploaded[:8])
}

func TestValidateH3RequestParametersRejectsUnsupportedNodeRanges(t *testing.T) {
	selector := h3Selector{AspectRatio: defaultAspect, Megapixels: 1, Multiple: 32}
	require.NoError(t, validateH3RequestParameters(relaycommon.TaskSubmitReq{Duration: maxDurationSeconds}, selector))
	require.Error(t, validateH3RequestParameters(relaycommon.TaskSubmitReq{Duration: maxDurationSeconds + 1}, selector))
	require.Error(t, validateH3RequestParameters(relaycommon.TaskSubmitReq{Seconds: "abc"}, selector))
	require.Error(t, validateH3RequestParameters(relaycommon.TaskSubmitReq{Seconds: "0"}, selector))
	require.NoError(t, validateH3RequestParameters(relaycommon.TaskSubmitReq{Seconds: "1"}, selector))
	require.Error(t, validateH3RequestParameters(relaycommon.TaskSubmitReq{Duration: 5}, h3Selector{AspectRatio: defaultAspect, Megapixels: 1, Multiple: 6}))
	require.Error(t, validateH3RequestParameters(relaycommon.TaskSubmitReq{Duration: 5}, h3Selector{AspectRatio: defaultAspect, Megapixels: 1, Multiple: 10}))
}

func TestRequiredNodeTypesIncludeWorkflowAndDynamicReferenceNodes(t *testing.T) {
	nodeTypes, err := RequiredNodeTypes()
	require.NoError(t, err)
	require.Contains(t, nodeTypes, "MiniMaxH3ReferenceToVideo")
	require.Contains(t, nodeTypes, "MiniMaxH3ImageToVideo")
	require.Contains(t, nodeTypes, "XB_HailuoH3VideoParams")
	require.Contains(t, nodeTypes, "VHS_VideoCombine")
	require.Contains(t, nodeTypes, "TESpeedMiniMaxH3")
	require.Contains(t, nodeTypes, "SolAttnPatch")
	require.Contains(t, nodeTypes, "XB_VideoLoader")
	require.Contains(t, nodeTypes, "LoadAudio")
}

func TestRequiredNodeChoicesAreDerivedFromWorkflow(t *testing.T) {
	choices, err := RequiredNodeChoices()
	require.NoError(t, err)
	require.ElementsMatch(t, []RequiredNodeChoice{
		{NodeType: "UNETLoader", InputName: "unet_name", Value: "minimax_h3_fl2va_pruned_int8_convrot.safetensors"},
		{NodeType: "CLIPLoader", InputName: "clip_name", Value: "qwen3vl_32b_minimax_h3_nvfp4_awq.safetensors"},
		{NodeType: "VAELoader", InputName: "vae_name", Value: "minimax_h3_video_vae_fp16.safetensors"},
		{NodeType: "VAELoader", InputName: "vae_name", Value: "minimax_h3_audio_vae_fp32.safetensors"},
		{NodeType: "LoraLoaderModelOnly", InputName: "lora_name", Value: "minimax_h3_fl2v_lightx2v_turbo_4step_v0.1_comfy.safetensors"},
		{NodeType: "MiniMaxH3MemoryProfile", InputName: "profile", Value: "1MP standard speed"},
		{NodeType: "MiniMaxH3MemoryProfile", InputName: "profile", Value: "2MP low VRAM"},
	}, choices)
}

func withComfyH3GroupPrices(t *testing.T, configuredPrices map[string]h3GroupPrice) {
	t.Helper()
	original := lookupRunningHubH3GroupPrice
	lookupRunningHubH3GroupPrice = func(group string) (h3GroupPrice, bool) {
		price, ok := configuredPrices[group]
		if ok && price.BoundGroup == "" {
			price.BoundGroup = group
		}
		return price, ok
	}
	t.Cleanup(func() { lookupRunningHubH3GroupPrice = original })
}

func withComfyUSDExchangeRate(t *testing.T, rate float64) {
	t.Helper()
	original := operation_setting.USDExchangeRate
	operation_setting.USDExchangeRate = rate
	t.Cleanup(func() { operation_setting.USDExchangeRate = original })
}
