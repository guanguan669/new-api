package comfyuih3

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"

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
	h3WorkerReservations.Unlock()
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
	require.Equal(t, []any{"228", 0}, h3Inputs["ref_images.ref_image_0"])
	require.Equal(t, []any{"229", 0}, h3Inputs["ref_videos.ref_video_0"])
	require.Equal(t, []any{"230", 0}, h3Inputs["ref_video_audios.ref_video_audio_0"])
	require.Equal(t, []any{"231", 0}, h3Inputs["ref_audios.ref_audio_0"])

	imageNode := workflow["228"].(map[string]any)
	require.Equal(t, "LoadImage", imageNode["class_type"])
	require.Equal(t, "image.png", imageNode["inputs"].(map[string]any)["image"])
	videoNode := workflow["229"].(map[string]any)
	require.Equal(t, "XB_VideoLoader", videoNode["class_type"])
	require.Equal(t, "motion.mp4", videoNode["inputs"].(map[string]any)["video"])
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

func TestBuildWorkflowSelectsMemoryProfileByMegapixelSeconds(t *testing.T) {
	tests := []struct {
		name       string
		megapixels float64
		seconds    int
		profile    string
	}{
		{name: "equal threshold uses standard speed", megapixels: 1, seconds: 18, profile: "1MP standard speed"},
		{name: "above threshold uses low vram", megapixels: 1, seconds: 19, profile: "2MP low VRAM"},
		{name: "two mp at nine seconds uses standard speed", megapixels: 2, seconds: 9, profile: "1MP standard speed"},
		{name: "two mp above nine seconds uses low vram", megapixels: 2, seconds: 10, profile: "2MP low VRAM"},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			workflow, err := buildWorkflow(
				relaycommon.TaskSubmitReq{Duration: testCase.seconds},
				h3Selector{AspectRatio: defaultAspect, Megapixels: testCase.megapixels, Multiple: defaultMultiple},
				references{},
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
	require.Contains(t, nodeTypes, "XB_HailuoH3VideoParams")
	require.Contains(t, nodeTypes, "VHS_VideoCombine")
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
