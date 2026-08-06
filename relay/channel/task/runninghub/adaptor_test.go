package runninghub

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSelectorFromRequestMapsStandardSizesAndMetadataOverrides(t *testing.T) {
	selector, err := selectorFromRequest(relaycommon.TaskSubmitReq{Size: "1280x720"})
	require.NoError(t, err)
	require.Equal(t, h3Selector{AspectRatio: "16:9 (Widescreen)", Megapixels: 0.9, Multiple: defaultMultiple}, selector)

	selector, err = selectorFromRequest(relaycommon.TaskSubmitReq{
		Size: "1024x1024",
		Metadata: map[string]any{
			"aspect_ratio": "portrait",
			"megapixels":   "2",
			"multiple":     2,
		},
	})
	require.NoError(t, err)
	require.Equal(t, "9:16 (Portrait Widescreen)", selector.AspectRatio)
	require.Equal(t, 2.0, selector.Megapixels)
	require.Equal(t, "2", selector.Multiple)

	_, err = selectorFromRequest(relaycommon.TaskSubmitReq{Size: "2048x512"})
	require.ErrorContains(t, err, "unsupported runninghub size")
}

func TestSelectorFromRequestMapsResolutionAndClarityToH3Megapixels(t *testing.T) {
	tests := []struct {
		name    string
		req     relaycommon.TaskSubmitReq
		aspect  string
		megapix float64
	}{
		{
			name:    "table 720p landscape",
			req:     relaycommon.TaskSubmitReq{Size: "1280x736"},
			aspect:  "16:9 (Widescreen)",
			megapix: 0.9,
		},
		{
			name:    "standard 1080p landscape",
			req:     relaycommon.TaskSubmitReq{Size: "1920x1080"},
			aspect:  "16:9 (Widescreen)",
			megapix: 2.0,
		},
		{
			name:    "standard 720p portrait",
			req:     relaycommon.TaskSubmitReq{Size: "720x1280"},
			aspect:  "9:16 (Portrait Widescreen)",
			megapix: 0.9,
		},
		{
			name:    "custom resolution field",
			req:     relaycommon.TaskSubmitReq{Metadata: map[string]any{"resolution": "1344x768"}},
			aspect:  "16:9 (Widescreen)",
			megapix: 0.98,
		},
		{
			name:    "standard 720p resolution label",
			req:     relaycommon.TaskSubmitReq{Metadata: map[string]any{"resolution": "720P"}},
			aspect:  "16:9 (Widescreen)",
			megapix: 0.9,
		},
		{
			name:    "clarity field",
			req:     relaycommon.TaskSubmitReq{Metadata: map[string]any{"clarity": "1.5"}},
			aspect:  defaultAspect,
			megapix: 1.5,
		},
		{
			name:    "clarity resolution label",
			req:     relaycommon.TaskSubmitReq{Metadata: map[string]any{"clarity": "1080P"}},
			aspect:  "16:9 (Widescreen)",
			megapix: 2.0,
		},
		{
			name:    "explicit megapixels overrides resolution",
			req:     relaycommon.TaskSubmitReq{Size: "1920x1080", Metadata: map[string]any{"megapixels": "0.5"}},
			aspect:  "16:9 (Widescreen)",
			megapix: 0.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selector, err := selectorFromRequest(tt.req)
			require.NoError(t, err)
			require.Equal(t, tt.aspect, selector.AspectRatio)
			require.Equal(t, tt.megapix, selector.Megapixels)
		})
	}

	_, err := selectorFromRequest(relaycommon.TaskSubmitReq{Metadata: map[string]any{"clarity": "high"}})
	require.ErrorContains(t, err, "unsupported runninghub megapixels")
}

func TestSelectorFromRequestMapsEveryH3LandscapePreset(t *testing.T) {
	tests := []struct {
		size      string
		megapixel float64
	}{
		{size: "608x352", megapixel: 0.2},
		{size: "736x416", megapixel: 0.3},
		{size: "864x480", megapixel: 0.4},
		{size: "960x544", megapixel: 0.5},
		{size: "1056x608", megapixel: 0.6},
		{size: "1152x640", megapixel: 0.7},
		{size: "1216x672", megapixel: 0.8},
		{size: "1280x736", megapixel: 0.9},
		{size: "1344x768", megapixel: 0.98},
		{size: "1376x768", megapixel: 1.0},
		{size: "1504x832", megapixel: 1.2},
		{size: "1664x928", megapixel: 1.5},
		{size: "1824x1024", megapixel: 1.8},
		{size: "1920x1080", megapixel: 2.0},
	}

	for _, tt := range tests {
		t.Run(tt.size, func(t *testing.T) {
			selector, err := selectorFromRequest(relaycommon.TaskSubmitReq{Size: tt.size})
			require.NoError(t, err)
			require.Equal(t, "16:9 (Widescreen)", selector.AspectRatio)
			require.Equal(t, tt.megapixel, selector.Megapixels)
		})
	}
}

func TestSelectorFromRequestUsesTopLevelResolution(t *testing.T) {
	tests := []struct {
		body       string
		megapixels float64
	}{
		{body: `{"model":"minimax_h3","prompt":"video","resolution":"1824x1024"}`, megapixels: 1.8},
		{body: `{"model":"minimax_h3","prompt":"video","resolution":"720P"}`, megapixels: 0.9},
	}

	for _, tt := range tests {
		var req relaycommon.TaskSubmitReq
		require.NoError(t, json.Unmarshal([]byte(tt.body), &req))

		selector, err := selectorFromRequest(req)
		require.NoError(t, err)
		require.Equal(t, "16:9 (Widescreen)", selector.AspectRatio)
		require.Equal(t, tt.megapixels, selector.Megapixels)
	}
}

func TestConvertRequestBuildsH3NodesAndEnforcesLimits(t *testing.T) {
	adaptor := &TaskAdaptor{baseURL: "https://runninghub.example", imageWorkflowID: "wf-image", textWorkflowID: "wf-text"}
	ctx := &gin.Context{}

	body, err := adaptor.convertRequest(ctx, relaycommon.TaskSubmitReq{
		Prompt:   "make a video",
		Duration: 6,
		Size:     "720x1280",
		Image:    "uploaded-image-1.png",
		Images:   []string{"uploaded-image-2.png"},
		Metadata: map[string]any{"reference_audio": "uploaded-audio.mp3"},
	}, "secret-key")
	require.NoError(t, err)
	require.Equal(t, "secret-key", body.APIKey)
	require.Equal(t, "wf-image", body.WorkflowID)
	require.Contains(t, body.NodeInfoList, nodeInfo{NodeID: "138", FieldName: "value", FieldValue: "make a video"})
	require.Contains(t, body.NodeInfoList, nodeInfo{NodeID: "132", FieldName: "value", FieldValue: 6})
	require.Contains(t, body.NodeInfoList, nodeInfo{NodeID: "115", FieldName: "aspect_ratio", FieldValue: "9:16 (Portrait Widescreen)"})
	require.Contains(t, body.NodeInfoList, nodeInfo{NodeID: "115", FieldName: "megapixels", FieldValue: 0.9})
	require.Contains(t, body.NodeInfoList, nodeInfo{NodeID: "137", FieldName: "image", FieldValue: "uploaded-image-1.png"})
	require.Contains(t, body.NodeInfoList, nodeInfo{NodeID: "618", FieldName: "image", FieldValue: "uploaded-image-2.png"})
	require.Contains(t, body.NodeInfoList, nodeInfo{NodeID: "628", FieldName: "audio", FieldValue: "uploaded-audio.mp3"})

	body, err = adaptor.convertRequest(ctx, relaycommon.TaskSubmitReq{
		Prompt:         "deduplicate normalized input reference",
		InputReference: "same-image.png",
		Images:         []string{"same-image.png"},
	}, "secret-key")
	require.NoError(t, err)
	imageNodes := make([]nodeInfo, 0)
	for _, node := range body.NodeInfoList {
		if node.FieldName == "image" {
			imageNodes = append(imageNodes, node)
		}
	}
	require.Equal(t, []nodeInfo{{NodeID: "137", FieldName: "image", FieldValue: "same-image.png"}}, imageNodes)

	_, err = adaptor.convertRequest(ctx, relaycommon.TaskSubmitReq{Prompt: "x", Images: []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10"}}, "secret-key")
	require.ErrorContains(t, err, "at most 9 reference images")

	_, err = adaptor.convertRequest(ctx, relaycommon.TaskSubmitReq{Prompt: "x", Metadata: map[string]any{"reference_video": "video.mp4"}}, "secret-key")
	require.ErrorContains(t, err, "reference video")
}

func TestConvertRequestUsesTextWorkflowWithoutReferenceImages(t *testing.T) {
	adaptor := &TaskAdaptor{baseURL: "https://runninghub.example", imageWorkflowID: "wf-image", textWorkflowID: "wf-text"}

	body, err := adaptor.convertRequest(&gin.Context{}, relaycommon.TaskSubmitReq{
		Prompt:   "make a video from text",
		Duration: 5,
		Metadata: map[string]any{"reference_audio": "uploaded-audio.mp3"},
	}, "secret-key")
	require.NoError(t, err)
	require.Equal(t, "wf-text", body.WorkflowID)
	for _, node := range body.NodeInfoList {
		require.NotEqual(t, "image", node.FieldName)
	}
	require.Contains(t, body.NodeInfoList, nodeInfo{NodeID: "628", FieldName: "audio", FieldValue: "uploaded-audio.mp3"})
}

func TestWorkflowForRequestRecognizesMultipartReferenceURLs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var formBody bytes.Buffer
	writer := multipart.NewWriter(&formBody)
	require.NoError(t, writer.WriteField("input_reference", "https://cdn.example/reference.png"))
	require.NoError(t, writer.WriteField("reference_images", "stored-image.png"))
	require.NoError(t, writer.WriteField("reference_audio", "https://cdn.example/reference.mp3"))
	require.NoError(t, writer.Close())

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", &formBody)
	ctx.Request.Header.Set("Content-Type", writer.FormDataContentType())
	require.NoError(t, ctx.Request.ParseMultipartForm(1<<20))

	adaptor := &TaskAdaptor{imageWorkflowID: "wf-image", textWorkflowID: "wf-text"}
	workflowID, mode, err := adaptor.workflowForRequest(ctx, relaycommon.TaskSubmitReq{Prompt: "hello"})
	require.NoError(t, err)
	require.Equal(t, "wf-image", workflowID)
	require.Equal(t, common.RunningHubH3WorkflowImageToVideo, mode)

	images, audios, _, _, err := referenceInputs(ctx, relaycommon.TaskSubmitReq{Prompt: "hello"})
	require.NoError(t, err)
	require.Equal(t, []string{"https://cdn.example/reference.png", "stored-image.png"}, images)
	require.Equal(t, []string{"https://cdn.example/reference.mp3"}, audios)
}

func TestEstimateBillingUsesRunningHubDuration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Set("task_request", relaycommon.TaskSubmitReq{Seconds: "12"})

	ratios := (&TaskAdaptor{}).EstimateBilling(ctx, &relaycommon.RelayInfo{})
	require.Equal(t, map[string]float64{"seconds": 12}, ratios)
}

func TestRunningHubSecondsNormalizationIsSharedByBillingAndNode132(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	req := relaycommon.TaskSubmitReq{Seconds: "12"}
	ctx.Set("task_request", req)

	ratios := (&TaskAdaptor{}).EstimateBilling(ctx, &relaycommon.RelayInfo{})
	require.Equal(t, 12.0, ratios["seconds"])

	body, err := (&TaskAdaptor{baseURL: "https://runninghub.example", textWorkflowID: "wf-text"}).convertRequest(ctx, req, "secret-key")
	require.NoError(t, err)
	require.Contains(t, body.NodeInfoList, nodeInfo{NodeID: "132", FieldName: "value", FieldValue: 12})
}

func TestEstimateBillingUsesRunningHubMegapixelRatio(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Set("task_request", relaycommon.TaskSubmitReq{Seconds: "6", Size: "1920x1080"})

	ratios := (&TaskAdaptor{}).EstimateBilling(ctx, &relaycommon.RelayInfo{})
	require.Equal(t, 6.0, ratios["seconds"])
	require.Equal(t, 3.0, ratios["megapixels"])
}

func TestH3MegapixelBillingRatio(t *testing.T) {
	tests := []struct {
		megapixels any
		ratio      float64
	}{
		{megapixels: 0.2, ratio: 0.22},
		{megapixels: 0.98, ratio: 1.078},
		{megapixels: 1.0, ratio: 1.0},
		{megapixels: 1.2, ratio: 1.8},
		{megapixels: 2.0, ratio: 3.0},
	}

	for _, tt := range tests {
		require.InDelta(t, tt.ratio, h3MegapixelBillingRatio(tt.megapixels), 0.000001)
	}
}

func TestEstimateBillingIncludesOnlyReferenceImagesBeyondFive(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, tt := range []struct {
		name      string
		req       relaycommon.TaskSubmitReq
		wantRatio float64
	}{
		{
			name:      "five images are included",
			req:       relaycommon.TaskSubmitReq{Seconds: "5", Images: []string{"1", "2", "3", "4", "5"}},
			wantRatio: 1,
		},
		{
			name:      "six images at base quality",
			req:       relaycommon.TaskSubmitReq{Seconds: "5", Images: []string{"1", "2", "3", "4", "5", "6"}},
			wantRatio: 1.2,
		},
		{
			name: "six images at quality three",
			req: relaycommon.TaskSubmitReq{
				Seconds:  "5",
				Images:   []string{"1", "2", "3", "4", "5", "6"},
				Metadata: map[string]any{"megapixels": "2"},
			},
			wantRatio: 1 + 1.0/15.0,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(w)
			ctx.Set("task_request", tt.req)

			ratios := (&TaskAdaptor{}).EstimateBilling(ctx, &relaycommon.RelayInfo{})
			if tt.wantRatio == 1 {
				_, exists := ratios["reference_images"]
				require.False(t, exists)
				return
			}
			require.InDelta(t, tt.wantRatio, ratios["reference_images"], 0.000001)
		})
	}
}

func TestValidateRequestRejectsTooManyMultipartFilesBeforeUpload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var form bytes.Buffer
	writer := multipart.NewWriter(&form)
	require.NoError(t, writer.WriteField("prompt", "hello"))
	for i := 0; i < maxImages+1; i++ {
		part, err := writer.CreateFormFile("images", "local.png")
		require.NoError(t, err)
		_, err = part.Write([]byte("image-bytes"))
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", &form)
	ctx.Request.Header.Set("Content-Type", writer.FormDataContentType())

	adaptor := &TaskAdaptor{imageWorkflowID: "wf-image", textWorkflowID: "wf-text"}
	taskErr := adaptor.ValidateRequestAndSetAction(ctx, &relaycommon.RelayInfo{TaskRelayInfo: &relaycommon.TaskRelayInfo{}})
	require.NotNil(t, taskErr)
	require.Equal(t, http.StatusBadRequest, taskErr.StatusCode)
	require.Contains(t, taskErr.Message, "at most 9 reference images")
}

func TestDoResponseAndParseTaskResult(t *testing.T) {
	adaptor := &TaskAdaptor{}
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Body:       io.NopCloser(strings.NewReader(`{"code":0,"msg":"ok","data":{"taskId":"task-123"}}`)),
	}
	taskID, raw, taskErr := adaptor.DoResponse(ctx, resp, &relaycommon.RelayInfo{
		OriginModelName: "minimax_h3",
		TaskRelayInfo:   &relaycommon.TaskRelayInfo{PublicTaskID: "task_public_123"},
	})
	require.Nil(t, taskErr)
	require.Equal(t, "task-123", taskID)
	require.JSONEq(t, `{"code":0,"msg":"ok","data":{"taskId":"task-123"}}`, string(raw))
	require.Equal(t, http.StatusOK, w.Code)
	var publicVideo map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &publicVideo))
	require.Equal(t, "task_public_123", publicVideo["id"])
	require.Equal(t, "task_public_123", publicVideo["task_id"])

	info, err := adaptor.ParseTaskResult([]byte(`{"code":0,"msg":"ok","data":{"status":"SUCCESS","outputs":[{"url":"https://cdn.example/thumb.jpg"},{"video_url":"https://cdn.example/video.mp4"}]}}`))
	require.NoError(t, err)
	require.Equal(t, string(model.TaskStatusSuccess), info.Status)
	require.Equal(t, "100%", info.Progress)
	require.Equal(t, "https://cdn.example/video.mp4", info.Url)

	info, err = adaptor.ParseTaskResult([]byte(`{"taskId":"task-123","status":"SUCCESS","results":[{"url":"https://cdn.example/image.png","outputType":"jpg"},{"url":"https://cdn.example/download/opaque-video","outputType":"mp4"}]}`))
	require.NoError(t, err)
	require.Equal(t, string(model.TaskStatusSuccess), info.Status)
	require.Equal(t, "https://cdn.example/download/opaque-video", info.Url)

	info, err = adaptor.ParseTaskResult([]byte(`{"taskId":"task-zip","status":"SUCCESS","results":[{"url":"https://cdn.example/output.zip","outputType":"zip"}]}`))
	require.NoError(t, err)
	require.Equal(t, string(model.TaskStatusFailure), info.Status)
	require.Empty(t, info.Url)
	require.Contains(t, info.Reason, "without a video result")

	info, err = adaptor.ParseTaskResult([]byte(`{"code":0,"data":{"status":"FAILED","errorMsg":"bad input"}}`))
	require.NoError(t, err)
	require.Equal(t, string(model.TaskStatusFailure), info.Status)
	require.Equal(t, "bad input", info.Reason)
}

func TestPrepareWorkflowKeepsSaveVideoAndRemovesCompetingImageOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, apiFormatPath, r.URL.Path)
		var body map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		require.Equal(t, "rh-test-key", body["apiKey"])
		require.Equal(t, "wf-h3", body["workflowId"])
		_, _ = w.Write([]byte(`{"code":0,"data":{"prompt":{"138":{"inputs":{"value":"prompt"}},"132":{"inputs":{"value":5}},"115":{"inputs":{"aspect_ratio":"16:9","megapixels":1,"multiple":32}},"137":{"inputs":{"image":""}},"618":{"inputs":{"image":""}},"617":{"inputs":{"image":""}},"619":{"inputs":{"image":""}},"627":{"inputs":{"image":""}},"626":{"inputs":{"image":""}},"625":{"inputs":{"image":""}},"624":{"inputs":{"image":""}},"623":{"inputs":{"image":""}},"628":{"inputs":{"audio":""}},"630":{"inputs":{"audio":""}},"629":{"inputs":{"audio":""}},"92":{"class_type":"SaveVideo","inputs":{"video":["130",0]}},"603":{"class_type":"solarL_SaveImagesToZip","inputs":{"zip":["522",0]}}}}}`))
	}))
	defer server.Close()

	adaptor := &TaskAdaptor{baseURL: server.URL}
	workflow, err := adaptor.prepareWorkflow("rh-test-key", "wf-h3", common.RunningHubH3WorkflowImageToVideo)
	require.NoError(t, err)
	require.Contains(t, workflow, "92")
	require.NotContains(t, workflow, "603")
}

func TestDoResponseAndParseTaskResultAcceptV2SuccessCode(t *testing.T) {
	adaptor := &TaskAdaptor{}
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Body:       io.NopCloser(strings.NewReader(`{"code":200,"message":"ok","data":{"taskId":"task-200"}}`)),
	}
	taskID, _, taskErr := adaptor.DoResponse(ctx, resp, &relaycommon.RelayInfo{
		OriginModelName: "minimax_h3",
		TaskRelayInfo:   &relaycommon.TaskRelayInfo{PublicTaskID: "task_public_200"},
	})
	require.Nil(t, taskErr)
	require.Equal(t, "task-200", taskID)

	info, err := adaptor.ParseTaskResult([]byte(`{"code":200,"data":{"status":"SUCCESS","results":[{"url":"https://cdn.example/download/video","outputType":"mp4"}]}}`))
	require.NoError(t, err)
	require.Equal(t, string(model.TaskStatusSuccess), info.Status)
	require.Equal(t, "https://cdn.example/download/video", info.Url)
}

func TestMultipartUploadBuildRequestBody(t *testing.T) {
	var uploadedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, uploadPath, r.URL.Path)
		require.Equal(t, "Bearer secret-key", r.Header.Get("Authorization"))
		require.NoError(t, r.ParseMultipartForm(1<<20))
		file, _, err := r.FormFile("file")
		require.NoError(t, err)
		defer file.Close()
		uploadedBody, err = io.ReadAll(file)
		require.NoError(t, err)
		_, _ = w.Write([]byte(`{"code":0,"msg":"ok","data":{"fileName":"rh-uploaded.png"}}`))
	}))
	defer server.Close()

	var form bytes.Buffer
	writer := multipart.NewWriter(&form)
	part, err := writer.CreateFormFile("input_reference", "local.png")
	require.NoError(t, err)
	_, err = part.Write([]byte("image-bytes"))
	require.NoError(t, err)
	require.NoError(t, writer.WriteField("prompt", "hello"))
	require.NoError(t, writer.Close())

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", &form)
	ctx.Request.Header.Set("Content-Type", writer.FormDataContentType())
	require.NoError(t, ctx.Request.ParseMultipartForm(1<<20))
	ctx.Set("task_request", relaycommon.TaskSubmitReq{Prompt: "hello"})
	workflow := map[string]any{
		"138": map[string]any{"inputs": map[string]any{"value": ""}},
		"132": map[string]any{"inputs": map[string]any{"value": 5}},
		"115": map[string]any{"inputs": map[string]any{"aspect_ratio": defaultAspect, "megapixels": defaultMegapixels, "multiple": defaultMultiple}},
		"92":  map[string]any{"class_type": "SaveVideo", "inputs": map[string]any{"video": []any{"130", 0}}},
	}
	for _, nodeID := range imageNodeIDs {
		workflow[nodeID] = map[string]any{"inputs": map[string]any{"image": ""}}
	}
	for _, nodeID := range audioNodeIDs {
		workflow[nodeID] = map[string]any{"inputs": map[string]any{"audio": ""}}
	}
	workflow["603"] = map[string]any{"class_type": "solarL_SaveImagesToZip", "inputs": map[string]any{"zip": []any{"522", 0}}}
	workflowBody, err := json.Marshal(map[string]any{"code": 0, "data": map[string]any{"prompt": workflow}})
	require.NoError(t, err)
	preparedWorkflow, err := common.PrepareRunningHubH3WorkflowForMode(workflowBody, common.RunningHubH3WorkflowImageToVideo)
	require.NoError(t, err)
	ctx.Set(preparedWorkflowContextKey, preparedWorkflow)

	adaptor := &TaskAdaptor{baseURL: server.URL, imageWorkflowID: "wf-image", textWorkflowID: "wf-text"}
	reader, err := adaptor.BuildRequestBody(ctx, &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ApiKey: "secret-key"}})
	require.NoError(t, err)
	payload, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.Equal(t, []byte("image-bytes"), uploadedBody)

	var body createRequest
	require.NoError(t, json.Unmarshal(payload, &body))
	require.Equal(t, "wf-image", body.WorkflowID)
	require.Contains(t, body.NodeInfoList, nodeInfo{NodeID: "137", FieldName: "image", FieldValue: "rh-uploaded.png"})
	var submittedWorkflow map[string]any
	require.NoError(t, json.Unmarshal([]byte(body.Workflow), &submittedWorkflow))
	require.Contains(t, submittedWorkflow, "92")
	require.NotContains(t, submittedWorkflow, "603")
	imageNode := submittedWorkflow["137"].(map[string]any)
	require.Equal(t, "rh-uploaded.png", imageNode["inputs"].(map[string]any)["image"])
}

func TestBuildRequestBodyRejectsTooManyMultipartImagesBeforeUpload(t *testing.T) {
	uploadCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uploadCalls++
		http.Error(w, "upload should not be called", http.StatusInternalServerError)
	}))
	defer server.Close()

	var form bytes.Buffer
	writer := multipart.NewWriter(&form)
	for i := 0; i < maxImages+1; i++ {
		part, err := writer.CreateFormFile("input_reference", "image.png")
		require.NoError(t, err)
		_, err = part.Write([]byte("image-bytes"))
		require.NoError(t, err)
	}
	require.NoError(t, writer.WriteField("prompt", "hello"))
	require.NoError(t, writer.Close())

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", &form)
	ctx.Request.Header.Set("Content-Type", writer.FormDataContentType())
	require.NoError(t, ctx.Request.ParseMultipartForm(1<<20))
	ctx.Set("task_request", relaycommon.TaskSubmitReq{Prompt: "hello"})

	adaptor := &TaskAdaptor{baseURL: server.URL, imageWorkflowID: "wf-image", textWorkflowID: "wf-text"}
	_, err := adaptor.BuildRequestBody(ctx, &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ApiKey: "secret-key"}})
	require.ErrorContains(t, err, "at most 9 reference images")
	require.Zero(t, uploadCalls)
}

func TestDownloadReferenceRejectsOversizedContentLengthBeforeBodyRead(t *testing.T) {
	oldMax := constant.MaxFileDownloadMB
	constant.MaxFileDownloadMB = 1
	fetchSetting := system_setting.GetFetchSetting()
	oldFetchSetting := *fetchSetting
	fetchSetting.EnableSSRFProtection = false
	service.InitHttpClient()
	t.Cleanup(func() {
		constant.MaxFileDownloadMB = oldMax
		*fetchSetting = oldFetchSetting
		service.InitHttpClient()
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", strconv.FormatInt(maxReferenceBytes()+1, 10))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	_, err := downloadReference(server.URL + "/too-large.png")
	require.ErrorContains(t, err, "exceeds max size 1 MB")
}

func TestReferenceInputsRejectOversizedMultipartHeaderBeforeRead(t *testing.T) {
	oldMax := constant.MaxFileDownloadMB
	constant.MaxFileDownloadMB = 1
	t.Cleanup(func() { constant.MaxFileDownloadMB = oldMax })

	_, err := readMultipartFile(&multipart.FileHeader{Filename: "too-large.png", Size: maxReferenceBytes() + 1})
	require.ErrorContains(t, err, "exceeds max size 1 MB")
}

func TestUploadReferenceRejectsOversizedInputBeforeHTTP(t *testing.T) {
	oldMax := constant.MaxFileDownloadMB
	constant.MaxFileDownloadMB = 1
	t.Cleanup(func() { constant.MaxFileDownloadMB = oldMax })

	uploadCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uploadCalls++
		http.Error(w, "upload should not be called", http.StatusInternalServerError)
	}))
	defer server.Close()

	_, err := (&TaskAdaptor{baseURL: server.URL}).uploadReference("secret-key", referenceInput{Name: "too-large.mp3", Data: make([]byte, maxReferenceBytes()+1)})
	require.ErrorContains(t, err, "exceeds max size 1 MB")
	require.Zero(t, uploadCalls)
}

func TestUploadReferenceAcceptsV2FilenameResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, uploadPath, r.URL.Path)
		require.Equal(t, "Bearer secret-key", r.Header.Get("Authorization"))
		require.NoError(t, r.ParseMultipartForm(1<<20))
		file, _, err := r.FormFile("file")
		require.NoError(t, err)
		defer file.Close()
		body, err := io.ReadAll(file)
		require.NoError(t, err)
		require.Equal(t, []byte("audio-bytes"), body)
		_, _ = w.Write([]byte(`{"code":200,"message":"success","data":{"filename":"rh-uploaded.mp3"}}`))
	}))
	defer server.Close()

	adaptor := &TaskAdaptor{baseURL: server.URL}
	fileName, err := adaptor.uploadReference("secret-key", referenceInput{Name: "local.mp3", Data: []byte("audio-bytes")})
	require.NoError(t, err)
	require.Equal(t, "rh-uploaded.mp3", fileName)
}
