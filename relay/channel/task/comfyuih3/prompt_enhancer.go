package comfyuih3

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/internalchat"
	openaidto "github.com/QuantumNous/new-api/relaykit/dto"
	relaytypes "github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/gin-gonic/gin"
)

const (
	promptEnhancementContextKey = "comfyui_h3_final_prompt"
	maxEnhancerResponseBytes    = 1024 * 1024
	maxPromptEnhancerImageBytes = 10 * 1024 * 1024
	maxPromptEnhancerTotalBytes = 32 * 1024 * 1024
	h3PromptEnhancerFPS         = 24
	// Prompt enhancement is optional, but multimodal requests can legitimately
	// take longer than a normal text completion. The administrator-configured
	// timeout is honored up to the same 300-second limit exposed by the settings
	// API; the request context still cancels it when the downstream request ends.
	promptEnhancerDefaultTimeout = 30 * time.Second
	promptEnhancerMaxTimeout     = 300 * time.Second
	promptEnhancerMaxAttempts    = 3
)

type promptEnhancerHTTPError struct {
	statusCode int
	status     string
	message    string
}

func (e *promptEnhancerHTTPError) Error() string {
	if e == nil {
		return "prompt enhancer HTTP request failed"
	}
	if e.message == "" {
		return fmt.Sprintf("prompt enhancer returned %s", e.status)
	}
	return fmt.Sprintf("prompt enhancer returned %s: %s", e.status, e.message)
}

var enhanceH3Prompt = requestEnhancedH3Prompt
var executeInternalH3PromptChat = internalchat.ExecutePreservingSystemRole

// Kept as a variable so tests can exercise cancellation without sleeping for
// the production timeout. It is a safety ceiling, not a fixed 8-second wait.
var promptEnhancerMaxWait = promptEnhancerMaxTimeout

func (a *TaskAdaptor) applyPromptEnhancement(c *gin.Context, req relaycommon.TaskSubmitReq, selector h3Selector) relaycommon.TaskSubmitReq {
	if c != nil {
		if cached, exists := c.Get(promptEnhancementContextKey); exists {
			if finalPrompt, ok := cached.(string); ok && strings.TrimSpace(finalPrompt) != "" {
				req.Prompt = finalPrompt
				return req
			}
		}
	}

	finalPrompt := req.Prompt
	settings := model_setting.GetComfyUIH3PromptEnhancerSettings()
	providerReady := strings.TrimSpace(settings.BaseURL) != ""
	if model_setting.NormalizeComfyUIH3PromptEnhancerProviderMode(settings.ProviderMode) == model_setting.ComfyUIH3PromptEnhancerProviderChannel {
		providerReady = settings.ChannelID > 0
	}
	if req.ShouldEnhancePrompt() && settings.Enabled && providerReady && strings.TrimSpace(settings.Model) != "" {
		ctx := context.Background()
		if c != nil && c.Request != nil {
			ctx = c.Request.Context()
		}
		enhancementCtx, cancel := context.WithTimeout(ctx, promptEnhancerRequestTimeout(settings))
		defer cancel()

		images, err := a.promptEnhancerImages(enhancementCtx, c, req)
		if err == nil {
			if enhanced, enhanceErr := enhanceH3Prompt(enhancementCtx, req, selector, images, settings, service.GetHttpClient()); enhanceErr == nil && strings.TrimSpace(enhanced) != "" {
				finalPrompt = enhanced
			} else if enhanceErr != nil {
				logger.LogWarn(ctx, fmt.Sprintf("ComfyUI H3 prompt enhancement failed; using original prompt: %v", enhanceErr))
			}
		} else {
			logger.LogWarn(context.Background(), fmt.Sprintf("ComfyUI H3 reference preparation for prompt enhancement failed; using original prompt: %v", err))
		}
	}

	req.Prompt = finalPrompt
	if c != nil {
		c.Set(promptEnhancementContextKey, finalPrompt)
	}
	return req
}

func requestEnhancedH3Prompt(
	ctx context.Context,
	req relaycommon.TaskSubmitReq,
	selector h3Selector,
	images []string,
	settings model_setting.ComfyUIH3PromptEnhancerSettings,
	client *http.Client,
) (string, error) {
	requestCtx, cancel := context.WithTimeout(ctx, promptEnhancerRequestTimeout(settings))
	defer cancel()

	mode := "text-to-video"
	if len(images) > 0 {
		mode = "image-to-video"
	}
	if isFirstLastFrameMode(req.Mode) {
		mode = "first-last-frame"
	}
	contextText := fmt.Sprintf(
		"User request:\n%s\n\nTarget video duration: %d seconds\nTarget aspect ratio: %s\nTarget resolution preset: %.2f MP\nGeneration mode: %s",
		req.Prompt,
		requestSeconds(req),
		selector.AspectRatio,
		selector.Megapixels,
		mode,
	)
	content := []openaidto.MediaContent{{Type: openaidto.ContentTypeText, Text: contextText}}
	if len(images) > 0 {
		referenceDescription := fmt.Sprintf("Reference images: %d. They are provided below in <Picture N> order.", len(images))
		if isFirstLastFrameMode(req.Mode) {
			referenceDescription = firstLastFramePromptContext(requestSeconds(req))
		}
		content = append(content, openaidto.MediaContent{
			Type: openaidto.ContentTypeText,
			Text: referenceDescription,
		})
		for index, imageURL := range images {
			if !strings.HasPrefix(strings.ToLower(imageURL), "data:image/") {
				return "", fmt.Errorf("prompt enhancer image %d is not a controlled data URL", index+1)
			}
			content = append(content,
				openaidto.MediaContent{Type: openaidto.ContentTypeText, Text: fmt.Sprintf("<Picture %d>", index+1)},
				openaidto.MediaContent{
					Type: openaidto.ContentTypeImageURL,
					ImageUrl: map[string]string{
						"url":    imageURL,
						"detail": "high",
					},
				},
			)
		}
	}
	stream := false
	payload := openaidto.GeneralOpenAIRequest{
		Model: strings.TrimSpace(settings.Model),
		Messages: []openaidto.Message{
			{Role: "system", Content: settings.SystemPrompt},
			{Role: "user"},
		},
		Stream: &stream,
	}
	// Keep pure text enhancement requests in the canonical OpenAI shape. Some
	// OpenAI-compatible gateways only accept a string for text-only messages;
	// multimodal requests still use the content-part array required for images.
	if len(images) == 0 {
		payload.Messages[1].SetStringContent(contextText)
	} else {
		payload.Messages[1].SetMediaContent(content)
	}

	providerMode := model_setting.NormalizeComfyUIH3PromptEnhancerProviderMode(settings.ProviderMode)
	var body []byte
	var endpoint string
	var err error
	if providerMode != model_setting.ComfyUIH3PromptEnhancerProviderChannel {
		body, err = common.Marshal(payload)
		if err != nil {
			return "", fmt.Errorf("marshal prompt enhancer request: %w", err)
		}
		endpoint, err = promptEnhancerEndpoint(settings.BaseURL)
		if err != nil {
			return "", err
		}
	}

	var lastErr error
	for attempt := 0; attempt < promptEnhancerMaxAttempts; attempt++ {
		if err := requestCtx.Err(); err != nil {
			if lastErr != nil {
				return "", lastErr
			}
			return "", err
		}
		deadline, hasDeadline := requestCtx.Deadline()
		if !hasDeadline {
			return "", errors.New("prompt enhancer request has no deadline")
		}
		remaining := time.Until(deadline)
		attemptsLeft := promptEnhancerMaxAttempts - attempt
		attemptTimeout := remaining / time.Duration(attemptsLeft)
		if attemptTimeout <= 0 {
			if lastErr != nil {
				return "", lastErr
			}
			return "", context.DeadlineExceeded
		}

		attemptCtx, cancelAttempt := context.WithTimeout(requestCtx, attemptTimeout)
		enhanced, err := requestEnhancedH3PromptOnce(attemptCtx, payload, settings, client, body, endpoint, providerMode)
		cancelAttempt()
		if err == nil {
			return enhanced, nil
		}
		lastErr = err
		if attempt == promptEnhancerMaxAttempts-1 || !shouldRetryPromptEnhancerError(err, requestCtx) {
			return "", err
		}
	}
	return "", lastErr
}

func requestEnhancedH3PromptOnce(
	ctx context.Context,
	payload openaidto.GeneralOpenAIRequest,
	settings model_setting.ComfyUIH3PromptEnhancerSettings,
	client *http.Client,
	body []byte,
	endpoint string,
	providerMode string,
) (string, error) {
	if providerMode == model_setting.ComfyUIH3PromptEnhancerProviderChannel {
		result, err := executeInternalH3PromptChat(ctx, settings.ChannelID, &payload)
		if err != nil {
			return "", fmt.Errorf("request prompt enhancer channel %d: %w", settings.ChannelID, err)
		}
		if result == nil || len(result.Choices) == 0 {
			return "", errors.New("prompt enhancer response has no choices")
		}
		enhanced := cleanEnhancedPrompt(result.Choices[0].Message.StringContent())
		if enhanced == "" {
			return "", errors.New("prompt enhancer returned an empty prompt")
		}
		return enhanced, nil
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create prompt enhancer request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	if apiKey := strings.TrimSpace(settings.APIKey); apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	}
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("request prompt enhancer: %w", err)
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, maxEnhancerResponseBytes+1))
	if err != nil {
		return "", fmt.Errorf("read prompt enhancer response: %w", err)
	}
	if len(responseBody) > maxEnhancerResponseBytes {
		return "", fmt.Errorf("prompt enhancer response exceeds %d bytes", maxEnhancerResponseBytes)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		message := strings.TrimSpace(string(responseBody))
		if len(message) > 300 {
			message = message[:300]
		}
		return "", &promptEnhancerHTTPError{statusCode: resp.StatusCode, status: resp.Status, message: message}
	}
	var result openaidto.OpenAITextResponse
	if err := common.Unmarshal(responseBody, &result); err != nil {
		return "", fmt.Errorf("unmarshal prompt enhancer response: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", errors.New("prompt enhancer response has no choices")
	}
	enhanced := cleanEnhancedPrompt(result.Choices[0].Message.StringContent())
	if enhanced == "" {
		return "", errors.New("prompt enhancer returned an empty prompt")
	}
	return enhanced, nil
}

func shouldRetryPromptEnhancerError(err error, totalCtx context.Context) bool {
	if err == nil || (totalCtx != nil && totalCtx.Err() != nil) || errors.Is(err, context.Canceled) {
		return false
	}
	statusCode := promptEnhancerErrorStatusCode(err)
	if statusCode >= http.StatusBadRequest && statusCode < http.StatusInternalServerError && statusCode != http.StatusTooManyRequests {
		return false
	}
	lower := strings.ToLower(err.Error())
	for _, marker := range []string{
		"invalid api key",
		"api key is invalid",
		"authentication failed",
		"unauthorized",
		"forbidden",
		"model is required",
		"model not configured",
		"model not found",
		"llm model is required",
		"channel does not exist",
		"channel is disabled",
		"invalid prompt enhancer",
		"invalid request",
		"bad request",
	} {
		if strings.Contains(lower, marker) {
			return false
		}
	}
	if statusCode == http.StatusTooManyRequests || statusCode >= http.StatusInternalServerError {
		return true
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && (netErr.Timeout() || netErr.Temporary()) {
		return true
	}
	for _, marker := range []string{
		"unknown provider",
		"temporarily unavailable",
		"service unavailable",
		"upstream unavailable",
		"try again",
		"server busy",
		"overloaded",
		"connection reset",
		"connection refused",
		"broken pipe",
		"eof",
		"timeout",
		"deadline exceeded",
		"bad gateway",
		"gateway timeout",
		"upstream error",
		"response has no choices",
	} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func promptEnhancerErrorStatusCode(err error) int {
	var httpErr *promptEnhancerHTTPError
	if errors.As(err, &httpErr) && httpErr != nil {
		return httpErr.statusCode
	}
	var apiErr *relaytypes.NewAPIError
	if errors.As(err, &apiErr) && apiErr != nil {
		return apiErr.StatusCode
	}
	return 0
}

func firstLastFramePromptContext(seconds int) string {
	duration := float64(seconds)
	lastFrameTime := duration - 1.0/h3PromptEnhancerFPS
	if lastFrameTime < 0 {
		lastFrameTime = 0
	}
	return fmt.Sprintf(
		"First/last frame contract: <Picture 1> is the required first frame at 0.000 seconds. <Picture 2> is the required final rendered frame at %.3f seconds for a %.3f-second, %d FPS target. Any shot timestamp written by the enhanced prompt must remain strictly less than %.3f seconds. Treat the pictures as ordered endpoint frames, not interchangeable reference images.",
		lastFrameTime,
		duration,
		h3PromptEnhancerFPS,
		duration,
	)
}

func promptEnhancerRequestTimeout(settings model_setting.ComfyUIH3PromptEnhancerSettings) time.Duration {
	timeoutSeconds := settings.TimeoutSeconds
	if timeoutSeconds <= 0 {
		if promptEnhancerDefaultTimeout > promptEnhancerMaxWait {
			return promptEnhancerMaxWait
		}
		return promptEnhancerDefaultTimeout
	}
	timeout := time.Duration(timeoutSeconds) * time.Second
	if timeout > promptEnhancerMaxWait {
		return promptEnhancerMaxWait
	}
	return timeout
}

func promptEnhancerEndpoint(baseURL string) (string, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("invalid prompt enhancer base URL")
	}
	path := strings.TrimRight(parsed.Path, "/")
	if !strings.HasSuffix(path, "/chat/completions") {
		if strings.HasSuffix(path, "/v1") {
			path += "/chat/completions"
		} else {
			path += "/v1/chat/completions"
		}
	}
	parsed.Path = path
	parsed.RawPath = ""
	return parsed.String(), nil
}

func cleanEnhancedPrompt(prompt string) string {
	prompt = strings.TrimSpace(prompt)
	if !strings.HasPrefix(prompt, "```") || !strings.HasSuffix(prompt, "```") {
		return prompt
	}
	lines := strings.Split(prompt, "\n")
	if len(lines) < 3 {
		return prompt
	}
	return strings.TrimSpace(strings.Join(lines[1:len(lines)-1], "\n"))
}

func (a *TaskAdaptor) promptEnhancerImages(ctx context.Context, c *gin.Context, req relaycommon.TaskSubmitReq) ([]string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	sources, err := referenceInputs(c, req)
	if err != nil {
		return nil, err
	}
	images := make([]string, 0, len(sources.images)+len(sources.imageFiles))
	var totalBytes int64
	appendImage := func(input referenceInput) error {
		if err := reservePromptEnhancerImageBytes(&totalBytes, int64(len(input.Data))); err != nil {
			return err
		}
		dataURL, err := promptEnhancerImageDataURL(input)
		if err != nil {
			return err
		}
		images = append(images, dataURL)
		return nil
	}
	for _, value := range sources.images {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		value = strings.TrimSpace(value)
		switch {
		case value == "":
			continue
		case strings.HasPrefix(strings.ToLower(value), "data:image/"):
			input, err := promptEnhancerDataURLInputWithLimit(value, maxPromptEnhancerImageBytes)
			if err != nil {
				return nil, err
			}
			if err := appendImage(input); err != nil {
				return nil, err
			}
		case isHTTPURL(value):
			input, err := downloadReferenceWithLimitContext(ctx, value, maxPromptEnhancerImageBytes)
			if err != nil {
				return nil, err
			}
			if err := appendImage(input); err != nil {
				return nil, err
			}
		default:
			fileName := filepath.Base(value)
			subfolder := strings.Trim(strings.TrimSuffix(value, fileName), "/\\")
			input, err := downloadComfyInputReference(ctx, a.baseURL, fileName, subfolder, a.proxy, maxPromptEnhancerImageBytes)
			if err != nil {
				return nil, err
			}
			if err := appendImage(input); err != nil {
				return nil, err
			}
		}
	}
	for _, header := range sources.imageFiles {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		input, err := readMultipartFileWithLimit(header, maxPromptEnhancerImageBytes)
		if err != nil {
			return nil, err
		}
		if err := appendImage(input); err != nil {
			return nil, err
		}
	}
	return images, nil
}

func promptEnhancerDataURLInput(value string) (referenceInput, error) {
	return promptEnhancerDataURLInputWithLimit(value, maxReferenceBytes())
}

func promptEnhancerDataURLInputWithLimit(value string, maxBytes int64) (referenceInput, error) {
	comma := strings.IndexByte(value, ',')
	if comma <= len("data:") {
		return referenceInput{}, fmt.Errorf("invalid image data URL")
	}
	header := strings.TrimSpace(value[:comma])
	parts := strings.Split(header, ";")
	if len(parts) < 2 || !strings.HasPrefix(strings.ToLower(parts[0]), "data:image/") {
		return referenceInput{}, fmt.Errorf("invalid image data URL MIME type")
	}
	isBase64 := false
	for _, part := range parts[1:] {
		if strings.EqualFold(strings.TrimSpace(part), "base64") {
			isBase64 = true
			break
		}
	}
	if !isBase64 {
		return referenceInput{}, fmt.Errorf("image data URL must use base64 encoding")
	}
	decoder := base64.NewDecoder(base64.StdEncoding, strings.NewReader(strings.TrimSpace(value[comma+1:])))
	data, err := io.ReadAll(io.LimitReader(decoder, maxBytes+1))
	if err != nil {
		return referenceInput{}, fmt.Errorf("decode image data URL: %w", err)
	}
	if int64(len(data)) > maxBytes {
		return referenceInput{}, fmt.Errorf("image data URL exceeds max size %d MB", bytesToMegabytes(maxBytes))
	}
	extension, _ := mime.ExtensionsByType(strings.TrimPrefix(strings.TrimSpace(parts[0]), "data:"))
	name := "reference.img"
	if len(extension) > 0 {
		name = "reference" + extension[0]
	}
	return referenceInput{Name: name, Data: data}, nil
}

func promptEnhancerImageDataURL(input referenceInput) (string, error) {
	detected := strings.ToLower(strings.TrimSpace(http.DetectContentType(input.Data)))
	if !strings.HasPrefix(detected, "image/") {
		return "", fmt.Errorf("reference file %s is not a valid image", input.Name)
	}
	return "data:" + detected + ";base64," + base64.StdEncoding.EncodeToString(input.Data), nil
}

func reservePromptEnhancerImageBytes(totalBytes *int64, imageBytes int64) error {
	if imageBytes > maxPromptEnhancerImageBytes {
		return fmt.Errorf("prompt enhancer image exceeds max size %d MB", bytesToMegabytes(maxPromptEnhancerImageBytes))
	}
	if imageBytes > maxPromptEnhancerTotalBytes-*totalBytes {
		return fmt.Errorf("prompt enhancer images exceed aggregate size %d MB", bytesToMegabytes(maxPromptEnhancerTotalBytes))
	}
	*totalBytes += imageBytes
	return nil
}

func downloadComfyInputReference(ctx context.Context, baseURL, fileName, subfolder, proxy string, maxBytes int64) (referenceInput, error) {
	requestURL := buildViewURL(baseURL, fileName, subfolder, "input")
	if requestURL == "" {
		return referenceInput{}, fmt.Errorf("invalid ComfyUI reference file name")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return referenceInput{}, err
	}
	client, err := service.GetHttpClientWithProxy(strings.TrimSpace(proxy))
	if err != nil {
		return referenceInput{}, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return referenceInput{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return referenceInput{}, fmt.Errorf("read ComfyUI reference failed with status %s", resp.Status)
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
	return referenceInput{Name: fileName, Data: data}, nil
}
