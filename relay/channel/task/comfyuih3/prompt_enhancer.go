package comfyuih3

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
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
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/gin-gonic/gin"
)

const (
	promptEnhancementContextKey = "comfyui_h3_final_prompt"
	maxEnhancerResponseBytes    = 1024 * 1024
	maxPromptEnhancerImageBytes = 10 * 1024 * 1024
	maxPromptEnhancerTotalBytes = 32 * 1024 * 1024
)

var enhanceH3Prompt = requestEnhancedH3Prompt
var executeInternalH3PromptChat = internalchat.Execute

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
		images, err := a.promptEnhancerImages(c, req)
		if err == nil {
			ctx := context.Background()
			if c != nil && c.Request != nil {
				ctx = c.Request.Context()
			}
			if enhanced, enhanceErr := enhanceH3Prompt(ctx, req, selector, images, settings, service.GetHttpClient()); enhanceErr == nil && strings.TrimSpace(enhanced) != "" {
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
	timeoutSeconds := settings.TimeoutSeconds
	if timeoutSeconds <= 0 {
		timeoutSeconds = 8
	}
	requestCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	mode := "text-to-video"
	if len(images) > 0 {
		mode = "image-to-video"
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
		content = append(content, openaidto.MediaContent{
			Type: openaidto.ContentTypeText,
			Text: fmt.Sprintf("Reference images: %d. They are provided below in <Picture N> order.", len(images)),
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
	payload.Messages[1].SetMediaContent(content)
	if model_setting.NormalizeComfyUIH3PromptEnhancerProviderMode(settings.ProviderMode) == model_setting.ComfyUIH3PromptEnhancerProviderChannel {
		result, err := executeInternalH3PromptChat(requestCtx, settings.ChannelID, &payload)
		if err != nil {
			return "", fmt.Errorf("request prompt enhancer channel %d: %w", settings.ChannelID, err)
		}
		if result == nil || len(result.Choices) == 0 {
			return "", fmt.Errorf("prompt enhancer response has no choices")
		}
		enhanced := cleanEnhancedPrompt(result.Choices[0].Message.StringContent())
		if enhanced == "" {
			return "", fmt.Errorf("prompt enhancer returned an empty prompt")
		}
		return enhanced, nil
	}
	body, err := common.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal prompt enhancer request: %w", err)
	}
	endpoint, err := promptEnhancerEndpoint(settings.BaseURL)
	if err != nil {
		return "", err
	}
	httpReq, err := http.NewRequestWithContext(requestCtx, http.MethodPost, endpoint, bytes.NewReader(body))
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
		return "", fmt.Errorf("prompt enhancer returned %s: %s", resp.Status, message)
	}
	var result openaidto.OpenAITextResponse
	if err := common.Unmarshal(responseBody, &result); err != nil {
		return "", fmt.Errorf("unmarshal prompt enhancer response: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("prompt enhancer response has no choices")
	}
	enhanced := cleanEnhancedPrompt(result.Choices[0].Message.StringContent())
	if enhanced == "" {
		return "", fmt.Errorf("prompt enhancer returned an empty prompt")
	}
	return enhanced, nil
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

func (a *TaskAdaptor) promptEnhancerImages(c *gin.Context, req relaycommon.TaskSubmitReq) ([]string, error) {
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
			input, err := downloadReferenceWithLimit(value, maxPromptEnhancerImageBytes)
			if err != nil {
				return nil, err
			}
			if err := appendImage(input); err != nil {
				return nil, err
			}
		default:
			fileName := filepath.Base(value)
			subfolder := strings.Trim(strings.TrimSuffix(value, fileName), "/\\")
			input, err := downloadComfyInputReference(a.baseURL, fileName, subfolder, a.proxy, maxPromptEnhancerImageBytes)
			if err != nil {
				return nil, err
			}
			if err := appendImage(input); err != nil {
				return nil, err
			}
		}
	}
	for _, header := range sources.imageFiles {
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

func downloadComfyInputReference(baseURL, fileName, subfolder, proxy string, maxBytes int64) (referenceInput, error) {
	requestURL := buildViewURL(baseURL, fileName, subfolder, "input")
	if requestURL == "" {
		return referenceInput{}, fmt.Errorf("invalid ComfyUI reference file name")
	}
	req, err := http.NewRequest(http.MethodGet, requestURL, nil)
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
