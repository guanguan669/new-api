package controller

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/gin-gonic/gin"
)

const videoProxyTimeout = 15 * time.Minute

// videoProxyError returns a standardized OpenAI-style error response.
func videoProxyError(c *gin.Context, status int, errType, message string) {
	c.JSON(status, gin.H{
		"error": gin.H{
			"message": message,
			"type":    errType,
		},
	})
}

func VideoProxy(c *gin.Context) {
	taskID := c.Param("task_id")
	if taskID == "" {
		videoProxyError(c, http.StatusBadRequest, "invalid_request_error", "task_id is required")
		return
	}

	userID := c.GetInt("id")
	task, exists, err := model.GetByTaskId(userID, taskID)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to query task %s: %s", taskID, err.Error()))
		videoProxyError(c, http.StatusInternalServerError, "server_error", "Failed to query task")
		return
	}
	if !exists || task == nil {
		videoProxyError(c, http.StatusNotFound, "invalid_request_error", "Task not found")
		return
	}

	if task.Status != model.TaskStatusSuccess {
		videoProxyError(c, http.StatusBadRequest, "invalid_request_error",
			fmt.Sprintf("Task is not completed yet, current status: %s", task.Status))
		return
	}
	channel, err := model.CacheGetChannel(task.ChannelId)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to get channel for task %s: %s", taskID, err.Error()))
		videoProxyError(c, http.StatusInternalServerError, "server_error", "Failed to retrieve channel information")
		return
	}
	baseURL := channel.GetBaseURL()
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}

	var videoURL string
	comfyUIH3GatewayMode := false
	proxy := channel.GetSetting().Proxy
	client := service.GetSSRFProtectedHTTPClient()
	if proxy != "" {
		// 渠道代理路径的连接由代理侧建立，无法做拨号时逐 IP 校验，
		// 因此后面对 videoURL 保留请求前的一次性 SSRF 校验。
		client, err = service.GetHttpClientWithProxy(proxy)
		if err != nil {
			logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to create proxy client for task %s: %s", taskID, err.Error()))
			videoProxyError(c, http.StatusInternalServerError, "server_error", "Failed to create proxy client")
			return
		}
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), videoProxyTimeout)
	defer cancel()
	method := c.Request.Method
	if method != http.MethodHead {
		method = http.MethodGet
	}
	req, err := http.NewRequestWithContext(ctx, method, "", nil)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to create request: %s", err.Error()))
		videoProxyError(c, http.StatusInternalServerError, "server_error", "Failed to create proxy request")
		return
	}
	forwardVideoRangeHeaders(c.Request.Header, req.Header)

	switch channel.Type {
	case constant.ChannelTypeGemini:
		apiKey := task.PrivateData.Key
		if apiKey == "" {
			logger.LogError(c.Request.Context(), fmt.Sprintf("Missing stored API key for Gemini task %s", taskID))
			videoProxyError(c, http.StatusInternalServerError, "server_error", "API key not stored for task")
			return
		}
		videoURL, err = getGeminiVideoURL(channel, task, apiKey)
		if err != nil {
			logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to resolve Gemini video URL for task %s: %s", taskID, err.Error()))
			videoProxyError(c, http.StatusBadGateway, "server_error", "Failed to resolve Gemini video URL")
			return
		}
		req.Header.Set("x-goog-api-key", apiKey)
	case constant.ChannelTypeVertexAi:
		videoURL, err = getVertexVideoURL(channel, task)
		if err != nil {
			logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to resolve Vertex video URL for task %s: %s", taskID, err.Error()))
			videoProxyError(c, http.StatusBadGateway, "server_error", "Failed to resolve Vertex video URL")
			return
		}
	case constant.ChannelTypeOpenAI, constant.ChannelTypeSora:
		videoURL = fmt.Sprintf("%s/v1/videos/%s/content", baseURL, task.GetUpstreamTaskID())
		req.Header.Set("Authorization", "Bearer "+channel.Key)
	case constant.ChannelTypeComfyUIH3:
		if gatewayVideoURL, apiKey, gatewayMode := comfyUIH3GatewayVideoRequest(channel, task); gatewayMode {
			comfyUIH3GatewayMode = true
			videoURL = gatewayVideoURL
			if apiKey == "" {
				logger.LogError(c.Request.Context(), fmt.Sprintf("Missing stored API key for ComfyUI H3 gateway task %s", taskID))
				videoProxyError(c, http.StatusInternalServerError, "server_error", "API key not stored for task")
				return
			}
			req.Header.Set("Authorization", "Bearer "+apiKey)
		} else {
			videoURL = task.GetResultURL()
		}
	default:
		// Video URL is stored in PrivateData.ResultURL (fallback to FailReason for old data)
		videoURL = task.GetResultURL()
	}
	if c.Request.Method == http.MethodHead && comfyUIH3GatewayMode {
		prepareComfyUIH3GatewayHeadRequest(req)
	}

	videoURL = strings.TrimSpace(videoURL)
	if videoURL == "" {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Video URL is empty for task %s", taskID))
		videoProxyError(c, http.StatusBadGateway, "server_error", "Failed to fetch video content")
		return
	}

	if strings.HasPrefix(videoURL, "data:") {
		if err := writeVideoDataURL(c, videoURL); err != nil {
			logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to decode video data URL for task %s: %s", taskID, err.Error()))
			videoProxyError(c, http.StatusBadGateway, "server_error", "Failed to fetch video content")
		}
		return
	}

	if isTrustedComfyUIH3GatewayResultURL(channel, task, videoURL) || isTrustedComfyUIH3ResultURL(channel, task.PrivateData.UpstreamBaseURL, videoURL) {
		// The URL was generated from a root-configured ComfyUI channel and is
		// constrained to its /view output endpoint. It may use a private host or
		// a non-default port, so keep the SSRF exception limited to this origin.
		if proxy == "" {
			client = service.GetHttpClient()
		}
	} else {
		var validateErr error
		if proxy == "" {
			validateErr = service.ValidateSSRFProtectedFetchURL(videoURL)
		} else {
			fetchSetting := system_setting.GetFetchSetting()
			validateErr = common.ValidateURLWithFetchSetting(videoURL, fetchSetting.EnableSSRFProtection, fetchSetting.AllowPrivateIp, fetchSetting.DomainFilterMode, fetchSetting.IpFilterMode, fetchSetting.DomainList, fetchSetting.IpList, fetchSetting.AllowedPorts, fetchSetting.ApplyIPFilterForDomain)
		}
		if validateErr != nil {
			logger.LogError(c.Request.Context(), fmt.Sprintf("Video URL blocked for task %s: %v", taskID, validateErr))
			videoProxyError(c, http.StatusForbidden, "server_error", fmt.Sprintf("request blocked: %v", validateErr))
			return
		}
	}

	req.URL, err = url.Parse(videoURL)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to parse URL %s: %s", videoURL, err.Error()))
		videoProxyError(c, http.StatusInternalServerError, "server_error", "Failed to create proxy request")
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to fetch video from %s: %s", videoURL, err.Error()))
		videoProxyError(c, http.StatusBadGateway, "server_error", "Failed to fetch video content")
		return
	}

	// Some ComfyUI gateway deployments respond to a valid Range request with
	// 206 but omit Content-Range. Browsers reject that malformed partial
	// response, so retry without Range and stream the complete 200 response.
	if comfyUIH3GatewayMode && c.Request.Method != http.MethodHead && shouldRetryComfyUIH3GatewayVideoWithoutRange(req, resp) {
		resp.Body.Close()
		fallbackRequest := fullVideoRequest(req)
		resp, err = client.Do(fallbackRequest)
		if err != nil {
			logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to fetch full ComfyUI H3 gateway video from %s: %s", videoURL, err.Error()))
			videoProxyError(c, http.StatusBadGateway, "server_error", "Failed to fetch video content")
			return
		}
	}
	defer resp.Body.Close()

	if !isVideoContentSuccessStatus(resp.StatusCode) {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Upstream returned status %d for %s", resp.StatusCode, videoURL))
		videoProxyError(c, http.StatusBadGateway, "server_error",
			fmt.Sprintf("Upstream service returned status %d", resp.StatusCode))
		return
	}

	copyVideoResponseHeaders(resp.Header, c.Writer.Header())
	if comfyUIH3GatewayMode {
		normalizeComfyUIH3GatewayVideoContentDisposition(c.Writer.Header())
	}
	responseStatus := resp.StatusCode
	if c.Request.Method == http.MethodHead && comfyUIH3GatewayMode {
		responseStatus = http.StatusOK
		normalizeGatewayHeadResponseHeaders(resp.StatusCode, resp.Header, c.Writer.Header())
	}

	c.Writer.Header().Set("Cache-Control", "private, max-age=86400")
	c.Writer.Header().Set("Vary", "Authorization")
	c.Writer.WriteHeader(responseStatus)
	if c.Request.Method == http.MethodHead {
		return
	}
	if _, err = io.Copy(c.Writer, resp.Body); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to stream video content: %s", err.Error()))
	}
}

func forwardVideoRangeHeaders(source, destination http.Header) {
	for _, header := range []string{"Range", "If-Range"} {
		if value := source.Get(header); value != "" {
			destination.Set(header, value)
		}
	}
}

func shouldRetryComfyUIH3GatewayVideoWithoutRange(request *http.Request, response *http.Response) bool {
	return request != nil &&
		response != nil &&
		request.Method == http.MethodGet &&
		strings.TrimSpace(request.Header.Get("Range")) != "" &&
		response.StatusCode == http.StatusPartialContent &&
		strings.TrimSpace(response.Header.Get("Content-Range")) == ""
}

func fullVideoRequest(request *http.Request) *http.Request {
	fallback := request.Clone(request.Context())
	fallback.Method = http.MethodGet
	fallback.Header.Del("Range")
	fallback.Header.Del("If-Range")
	return fallback
}

func copyVideoResponseHeaders(source, destination http.Header) {
	for _, header := range []string{
		"Accept-Ranges",
		"Content-Disposition",
		"Content-Encoding",
		"Content-Length",
		"Content-Range",
		"Content-Type",
		"ETag",
		"Last-Modified",
	} {
		if value := source.Get(header); value != "" {
			destination.Set(header, value)
		}
	}
}

func normalizeComfyUIH3GatewayVideoContentDisposition(headers http.Header) {
	value := strings.TrimSpace(headers.Get("Content-Disposition"))
	if value == "" {
		return
	}
	lowerValue := strings.ToLower(value)
	if strings.HasPrefix(lowerValue, "inline") {
		return
	}
	if strings.HasPrefix(lowerValue, "attachment") {
		value = strings.TrimSpace(value[len("attachment"):])
	}
	if value != "" && !strings.HasPrefix(value, ";") {
		value = "; " + value
	}
	headers.Set("Content-Disposition", "inline"+value)
}

func isVideoContentSuccessStatus(status int) bool {
	return status == http.StatusOK || status == http.StatusPartialContent
}

func prepareComfyUIH3GatewayHeadRequest(req *http.Request) {
	// The gateway documents only GET for /view. Request one byte so the
	// Content-Range exposes the full size without transferring the whole video.
	req.Method = http.MethodGet
	req.Header.Set("Range", "bytes=0-0")
	req.Header.Del("If-Range")
}

func normalizeGatewayHeadResponseHeaders(status int, source, destination http.Header) {
	if status == http.StatusOK {
		destination.Del("Content-Range")
		return
	}

	destination.Del("Content-Length")
	contentRange := strings.TrimSpace(source.Get("Content-Range"))
	if slash := strings.LastIndex(contentRange, "/"); slash >= 0 {
		if total, err := strconv.ParseInt(strings.TrimSpace(contentRange[slash+1:]), 10, 64); err == nil && total >= 0 {
			destination.Set("Content-Length", strconv.FormatInt(total, 10))
		}
	}
	destination.Del("Content-Range")
}

func comfyUIH3GatewayVideoRequest(channel *model.Channel, task *model.Task) (videoURL string, apiKey string, gatewayMode bool) {
	if channel == nil || task == nil || channel.Type != constant.ChannelTypeComfyUIH3 {
		return "", "", false
	}
	if !task.PrivateData.ComfyUIH3Gateway {
		return "", "", false
	}
	gatewayURL := strings.TrimRight(strings.TrimSpace(task.PrivateData.UpstreamBaseURL), "/")
	if gatewayURL == "" {
		gatewayURL = strings.TrimRight(strings.TrimSpace(channel.GetOtherSettings().ComfyUIH3GatewayURL), "/")
	}
	if gatewayURL == "" {
		return "", "", true
	}
	apiKey = channel.ResolveComfyUIH3GatewayKey(gatewayURL, task.PrivateData.Key)
	return gatewayURL + "/api/gateway/tasks/" + url.PathEscape(task.GetUpstreamTaskID()) + "/view", apiKey, true
}

func isTrustedComfyUIH3GatewayResultURL(channel *model.Channel, task *model.Task, rawURL string) bool {
	if channel == nil || task == nil || channel.Type != constant.ChannelTypeComfyUIH3 || !task.PrivateData.ComfyUIH3Gateway || strings.TrimSpace(task.GetUpstreamTaskID()) == "" {
		return false
	}
	gatewayBaseURL := strings.TrimSpace(task.PrivateData.UpstreamBaseURL)
	if gatewayBaseURL == "" {
		gatewayBaseURL = channel.GetOtherSettings().ComfyUIH3GatewayURL
	}
	gatewayURL, err := url.Parse(strings.TrimRight(strings.TrimSpace(gatewayBaseURL), "/"))
	if err != nil || gatewayURL.Scheme == "" || gatewayURL.Host == "" || gatewayURL.User != nil || gatewayURL.RawQuery != "" || gatewayURL.Fragment != "" {
		return false
	}
	resultURL, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || resultURL.Scheme == "" || resultURL.Host == "" || resultURL.User != nil || resultURL.RawQuery != "" || resultURL.Fragment != "" {
		return false
	}
	expectedPath := strings.TrimRight(gatewayURL.Path, "/") + "/api/gateway/tasks/" + url.PathEscape(task.GetUpstreamTaskID()) + "/view"
	return strings.EqualFold(resultURL.Scheme, gatewayURL.Scheme) &&
		strings.EqualFold(resultURL.Host, gatewayURL.Host) &&
		resultURL.Path == expectedPath
}

func isTrustedComfyUIH3ResultURL(channel *model.Channel, selectedWorkerURL, rawURL string) bool {
	if channel == nil || channel.Type != constant.ChannelTypeComfyUIH3 {
		return false
	}
	resultURL, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || resultURL.Scheme == "" || resultURL.Host == "" || resultURL.User != nil || resultURL.Fragment != "" {
		return false
	}
	if strings.TrimSpace(selectedWorkerURL) != "" && !channel.IsConfiguredComfyUIH3WorkerURL(selectedWorkerURL) {
		return false
	}
	configuredOrigins := []string{channel.GetBaseURL()}
	configuredOrigins = append(configuredOrigins, channel.GetOtherSettings().ComfyUIH3BackendURLs...)
	trustedOrigin := false
	for _, configuredOrigin := range configuredOrigins {
		configuredURL, parseErr := url.Parse(strings.TrimSpace(configuredOrigin))
		if parseErr != nil || configuredURL.Scheme == "" || configuredURL.Host == "" || configuredURL.User != nil {
			continue
		}
		expectedPath := strings.TrimRight(configuredURL.Path, "/") + "/view"
		if strings.EqualFold(resultURL.Scheme, configuredURL.Scheme) && strings.EqualFold(resultURL.Host, configuredURL.Host) && resultURL.Path == expectedPath {
			trustedOrigin = true
			break
		}
	}
	if !trustedOrigin {
		return false
	}
	allowedQueryKeys := map[string]bool{"filename": true, "subfolder": true, "type": true}
	query := resultURL.Query()
	for key, values := range query {
		if !allowedQueryKeys[key] || len(values) != 1 {
			return false
		}
	}
	filename := strings.TrimSpace(query.Get("filename"))
	if filename == "" || strings.Contains(filename, "..") || strings.Contains(query.Get("subfolder"), "..") {
		return false
	}
	if fileType := strings.TrimSpace(query.Get("type")); fileType != "" && fileType != "output" {
		return false
	}
	return true
}

func writeVideoDataURL(c *gin.Context, dataURL string) error {
	parts := strings.SplitN(dataURL, ",", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid data url")
	}

	header := parts[0]
	payload := parts[1]
	if !strings.HasPrefix(header, "data:") || !strings.Contains(header, ";base64") {
		return fmt.Errorf("unsupported data url")
	}

	mimeType := strings.TrimPrefix(header, "data:")
	mimeType = strings.TrimSuffix(mimeType, ";base64")
	if mimeType == "" {
		mimeType = "video/mp4"
	}

	videoBytes, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		videoBytes, err = base64.RawStdEncoding.DecodeString(payload)
		if err != nil {
			return err
		}
	}

	c.Writer.Header().Set("Content-Type", mimeType)
	c.Writer.Header().Set("Cache-Control", "private, max-age=86400")
	c.Writer.WriteHeader(http.StatusOK)
	_, err = c.Writer.Write(videoBytes)
	return err
}
