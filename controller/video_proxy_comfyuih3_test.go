package controller

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/require"
)

func TestVideoProxyPreservesRangeRequestAndResponseHeaders(t *testing.T) {
	upstreamRequestHeaders := http.Header{}
	upstreamRequestHeaders.Set("Range", "bytes=10-19")
	upstreamRequestHeaders.Set("If-Range", `"etag-1"`)
	destinationRequestHeaders := http.Header{}
	forwardVideoRangeHeaders(upstreamRequestHeaders, destinationRequestHeaders)
	require.Equal(t, "bytes=10-19", destinationRequestHeaders.Get("Range"))
	require.Equal(t, `"etag-1"`, destinationRequestHeaders.Get("If-Range"))

	upstreamResponseHeaders := http.Header{}
	upstreamResponseHeaders.Set("Accept-Ranges", "bytes")
	upstreamResponseHeaders.Set("Content-Range", "bytes 10-19/100")
	upstreamResponseHeaders.Set("Content-Length", "10")
	upstreamResponseHeaders.Set("Content-Type", "video/mp4")
	destinationResponseHeaders := http.Header{}
	copyVideoResponseHeaders(upstreamResponseHeaders, destinationResponseHeaders)
	require.Equal(t, "bytes", destinationResponseHeaders.Get("Accept-Ranges"))
	require.Equal(t, "bytes 10-19/100", destinationResponseHeaders.Get("Content-Range"))
	require.Equal(t, "10", destinationResponseHeaders.Get("Content-Length"))
	require.Equal(t, "video/mp4", destinationResponseHeaders.Get("Content-Type"))
	require.True(t, isVideoContentSuccessStatus(http.StatusOK))
	require.True(t, isVideoContentSuccessStatus(http.StatusPartialContent))
	require.False(t, isVideoContentSuccessStatus(http.StatusBadGateway))
}

func TestShouldRetryComfyUIH3GatewayVideoWithoutRange(t *testing.T) {
	request, err := http.NewRequest(http.MethodGet, "http://gateway.example/view", nil)
	require.NoError(t, err)
	request.Header.Set("Range", "bytes=0-63")

	missingContentRange := &http.Response{StatusCode: http.StatusPartialContent, Header: http.Header{}}
	require.True(t, shouldRetryComfyUIH3GatewayVideoWithoutRange(request, missingContentRange))

	withContentRange := &http.Response{StatusCode: http.StatusPartialContent, Header: http.Header{"Content-Range": []string{"bytes 0-63/1024"}}}
	require.False(t, shouldRetryComfyUIH3GatewayVideoWithoutRange(request, withContentRange))

	missingContentRange.StatusCode = http.StatusOK
	require.False(t, shouldRetryComfyUIH3GatewayVideoWithoutRange(request, missingContentRange))

	request.Header.Del("Range")
	missingContentRange.StatusCode = http.StatusPartialContent
	require.False(t, shouldRetryComfyUIH3GatewayVideoWithoutRange(request, missingContentRange))
}

func TestFullVideoRequestDropsRangeHeaders(t *testing.T) {
	request, err := http.NewRequest(http.MethodGet, "http://gateway.example/view", nil)
	require.NoError(t, err)
	request.Header.Set("Range", "bytes=0-63")
	request.Header.Set("If-Range", `"etag-1"`)

	fallback := fullVideoRequest(request)
	require.Equal(t, http.MethodGet, fallback.Method)
	require.Empty(t, fallback.Header.Get("Range"))
	require.Empty(t, fallback.Header.Get("If-Range"))
	require.Equal(t, "bytes=0-63", request.Header.Get("Range"))
}

func TestNormalizeComfyUIH3GatewayVideoContentDisposition(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "missing disposition type", input: `filename="video.mp4"`, want: `inline; filename="video.mp4"`},
		{name: "attachment", input: `attachment; filename="video.mp4"`, want: `inline; filename="video.mp4"`},
		{name: "already inline", input: `inline; filename="video.mp4"`, want: `inline; filename="video.mp4"`},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			headers := http.Header{"Content-Disposition": []string{testCase.input}}
			normalizeComfyUIH3GatewayVideoContentDisposition(headers)
			require.Equal(t, testCase.want, headers.Get("Content-Disposition"))
		})
	}
}

func TestNormalizeGatewayHeadResponseHeadersUsesFullVideoLength(t *testing.T) {
	source := http.Header{}
	source.Set("Content-Range", "bytes 0-0/123456")
	source.Set("Content-Length", "1")
	destination := http.Header{}
	copyVideoResponseHeaders(source, destination)

	normalizeGatewayHeadResponseHeaders(http.StatusPartialContent, source, destination)

	require.Equal(t, "123456", destination.Get("Content-Length"))
	require.Empty(t, destination.Get("Content-Range"))
}

func TestNormalizeGatewayHeadResponseHeadersDropsPartialLengthWithoutTotal(t *testing.T) {
	source := http.Header{}
	source.Set("Content-Range", "bytes 0-0/*")
	source.Set("Content-Length", "1")
	destination := http.Header{}
	copyVideoResponseHeaders(source, destination)

	normalizeGatewayHeadResponseHeaders(http.StatusPartialContent, source, destination)

	require.Empty(t, destination.Get("Content-Length"))
	require.Empty(t, destination.Get("Content-Range"))
}

func TestNormalizeGatewayHeadResponseHeadersKeepsFullLengthWhenRangeIsIgnored(t *testing.T) {
	source := http.Header{}
	source.Set("Content-Length", "123456")
	destination := http.Header{}
	copyVideoResponseHeaders(source, destination)

	normalizeGatewayHeadResponseHeaders(http.StatusOK, source, destination)

	require.Equal(t, "123456", destination.Get("Content-Length"))
	require.Empty(t, destination.Get("Content-Range"))
}

func TestPrepareComfyUIH3GatewayHeadRequestUsesRangeMetadataGet(t *testing.T) {
	req, err := http.NewRequest(http.MethodHead, "http://gateway.example/view", nil)
	require.NoError(t, err)
	req.Header.Set("Range", "bytes=10-19")
	req.Header.Set("If-Range", `"etag-1"`)

	prepareComfyUIH3GatewayHeadRequest(req)

	require.Equal(t, http.MethodGet, req.Method)
	require.Equal(t, "bytes=0-0", req.Header.Get("Range"))
	require.Empty(t, req.Header.Get("If-Range"))
}

func TestTrustedComfyUIH3ResultURLRequiresConfiguredOutputOrigin(t *testing.T) {
	baseURL := "http://127.0.0.1:5900"
	channel := &model.Channel{Type: constant.ChannelTypeComfyUIH3, BaseURL: &baseURL}
	require.True(t, isTrustedComfyUIH3ResultURL(channel, "", "http://127.0.0.1:5900/view?filename=result.mp4&type=output"))
	require.True(t, isTrustedComfyUIH3ResultURL(channel, "", "http://127.0.0.1:5900/view?filename=result.mp4&subfolder=videos%2Fh3&type=output"))
	require.False(t, isTrustedComfyUIH3ResultURL(channel, "", "http://127.0.0.1:5900/view?filename=../secret.mp4&type=output"))
	require.False(t, isTrustedComfyUIH3ResultURL(channel, "", "http://127.0.0.1:5900/view?filename=result.mp4&type=input"))
	require.False(t, isTrustedComfyUIH3ResultURL(channel, "", "http://127.0.0.1:5901/view?filename=result.mp4&type=output"))
	require.False(t, isTrustedComfyUIH3ResultURL(channel, "", "http://127.0.0.1:5900/api/view?filename=result.mp4&type=output"))
	require.False(t, isTrustedComfyUIH3ResultURL(channel, "", "http://127.0.0.1:5900/view?filename=result.mp4&type=output&url=http://example.com"))
	baseURLWithPath := "http://127.0.0.1:5900/comfy"
	channel.BaseURL = &baseURLWithPath
	require.True(t, isTrustedComfyUIH3ResultURL(channel, "", "http://127.0.0.1:5900/comfy/view?filename=result.mp4&type=output"))
}

func TestTrustedComfyUIH3ResultURLAcceptsConfiguredWorkerOnly(t *testing.T) {
	baseURL := "http://127.0.0.1:5900"
	channel := &model.Channel{Type: constant.ChannelTypeComfyUIH3, BaseURL: &baseURL}
	channel.SetOtherSettings(dto.ChannelOtherSettings{ComfyUIH3BackendURLs: []string{"http://127.0.0.1:5901"}})
	require.True(t, isTrustedComfyUIH3ResultURL(channel, "http://127.0.0.1:5901", "http://127.0.0.1:5901/view?filename=result.mp4&type=output"))
	require.False(t, isTrustedComfyUIH3ResultURL(channel, "http://127.0.0.1:5999", "http://127.0.0.1:5999/view?filename=result.mp4&type=output"))
}

func TestTrustedComfyUIH3GatewayResultURLIsExact(t *testing.T) {
	baseURL := "http://127.0.0.1:5900"
	channel := &model.Channel{Type: constant.ChannelTypeComfyUIH3, BaseURL: &baseURL}
	channel.SetOtherSettings(dto.ChannelOtherSettings{ComfyUIH3GatewayURL: "http://127.0.0.1:8090/gateway"})
	task := &model.Task{PrivateData: model.TaskPrivateData{UpstreamTaskID: "upstream-1", ComfyUIH3Gateway: true}}

	require.True(t, isTrustedComfyUIH3GatewayResultURL(channel, task, "http://127.0.0.1:8090/gateway/api/gateway/tasks/upstream-1/view"))
	require.False(t, isTrustedComfyUIH3GatewayResultURL(channel, task, "http://127.0.0.1:8090/gateway/api/gateway/tasks/upstream-2/view"))
	require.False(t, isTrustedComfyUIH3GatewayResultURL(channel, task, "http://127.0.0.1:8090/gateway/api/gateway/tasks/upstream-1/view?url=http://example.com"))
	require.False(t, isTrustedComfyUIH3GatewayResultURL(channel, task, "http://127.0.0.1:8091/gateway/api/gateway/tasks/upstream-1/view"))

	task.PrivateData.ComfyUIH3Gateway = false
	require.False(t, isTrustedComfyUIH3GatewayResultURL(channel, task, "http://127.0.0.1:8090/gateway/api/gateway/tasks/upstream-1/view"))
}

func TestComfyUIH3GatewayVideoRequestUsesCurrentKeyForSameGateway(t *testing.T) {
	baseURL := "http://127.0.0.1:5900"
	channel := &model.Channel{Type: constant.ChannelTypeComfyUIH3, BaseURL: &baseURL, Key: "current-key"}
	channel.SetOtherSettings(dto.ChannelOtherSettings{ComfyUIH3GatewayURL: "http://127.0.0.1:8090/"})
	task := &model.Task{TaskID: "task-public", PrivateData: model.TaskPrivateData{
		UpstreamTaskID:   "upstream-1",
		Key:              "stored-task-key",
		ComfyUIH3Gateway: true,
	}}

	videoURL, apiKey, gatewayMode := comfyUIH3GatewayVideoRequest(channel, task)
	require.True(t, gatewayMode)
	require.Equal(t, "http://127.0.0.1:8090/api/gateway/tasks/upstream-1/view", videoURL)
	require.Equal(t, "current-key", apiKey)

	task.PrivateData.Key = ""
	_, apiKey, gatewayMode = comfyUIH3GatewayVideoRequest(channel, task)
	require.True(t, gatewayMode)
	require.Equal(t, "current-key", apiKey)

	channel.SetOtherSettings(dto.ChannelOtherSettings{ComfyUIH3GatewayURL: "http://127.0.0.1:8091/"})
	task.PrivateData.Key = "stored-task-key"
	task.PrivateData.UpstreamBaseURL = "http://127.0.0.1:8090"
	_, apiKey, gatewayMode = comfyUIH3GatewayVideoRequest(channel, task)
	require.True(t, gatewayMode)
	require.Equal(t, "stored-task-key", apiKey)

	task.PrivateData.ComfyUIH3Gateway = false
	_, _, gatewayMode = comfyUIH3GatewayVideoRequest(channel, task)
	require.False(t, gatewayMode)
}
