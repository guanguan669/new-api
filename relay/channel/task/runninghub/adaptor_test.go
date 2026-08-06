package runninghub

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSelectorFromRequestMapsStandardSizesAndMetadataOverrides(t *testing.T) {
	selector, err := selectorFromRequest(relaycommon.TaskSubmitReq{Size: "1280x720"})
	require.NoError(t, err)
	require.Equal(t, h3Selector{AspectRatio: "16:9 (Landscape Widescreen)", Megapixels: defaultMegapixels, Multiple: defaultMultiple}, selector)

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
	require.Equal(t, "2", selector.Megapixels)
	require.Equal(t, "2", selector.Multiple)

	_, err = selectorFromRequest(relaycommon.TaskSubmitReq{Size: "2048x512"})
	require.ErrorContains(t, err, "unsupported runninghub size")
}

func TestConvertRequestBuildsH3NodesAndEnforcesLimits(t *testing.T) {
	adaptor := &TaskAdaptor{baseURL: "https://runninghub.example", workflowID: "wf-1"}
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
	require.Equal(t, "wf-1", body.WorkflowID)
	require.Contains(t, body.NodeInfoList, nodeInfo{NodeID: "138", FieldName: "value", FieldValue: "make a video"})
	require.Contains(t, body.NodeInfoList, nodeInfo{NodeID: "132", FieldName: "value", FieldValue: 6})
	require.Contains(t, body.NodeInfoList, nodeInfo{NodeID: "115", FieldName: "aspect_ratio", FieldValue: "9:16 (Portrait Widescreen)"})
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

func TestEstimateBillingUsesRunningHubDuration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Set("task_request", relaycommon.TaskSubmitReq{Seconds: "12"})

	ratios := (&TaskAdaptor{}).EstimateBilling(ctx, &relaycommon.RelayInfo{})
	require.Equal(t, map[string]float64{"seconds": 12}, ratios)
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

	adaptor := &TaskAdaptor{workflowID: "wf-1"}
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

	info, err = adaptor.ParseTaskResult([]byte(`{"code":0,"data":{"status":"FAILED","errorMsg":"bad input"}}`))
	require.NoError(t, err)
	require.Equal(t, string(model.TaskStatusFailure), info.Status)
	require.Equal(t, "bad input", info.Reason)
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

	adaptor := &TaskAdaptor{baseURL: server.URL, workflowID: "wf-1"}
	reader, err := adaptor.BuildRequestBody(ctx, &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ApiKey: "secret-key"}})
	require.NoError(t, err)
	payload, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.Equal(t, []byte("image-bytes"), uploadedBody)

	var body createRequest
	require.NoError(t, json.Unmarshal(payload, &body))
	require.Contains(t, body.NodeInfoList, nodeInfo{NodeID: "137", FieldName: "image", FieldValue: "rh-uploaded.png"})
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

	adaptor := &TaskAdaptor{baseURL: server.URL, workflowID: "wf-1"}
	_, err := adaptor.BuildRequestBody(ctx, &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ApiKey: "secret-key"}})
	require.ErrorContains(t, err, "at most 9 reference images")
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
