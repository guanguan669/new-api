package comfyuih3

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	openaidto "github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type promptEnhancerRoundTripper func(*http.Request) (*http.Response, error)

func (f promptEnhancerRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func setPromptEnhancerMaxWait(t *testing.T, wait time.Duration) {
	t.Helper()
	previous := promptEnhancerMaxWait
	promptEnhancerMaxWait = wait
	t.Cleanup(func() {
		promptEnhancerMaxWait = previous
	})
}

func TestRequestEnhancedH3PromptSendsContextAndImages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/chat/completions", r.URL.Path)
		require.Equal(t, "Bearer secret", r.Header.Get("Authorization"))
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NotContains(t, string(body), "MimeType")
		require.NotContains(t, string(body), "mime_type")
		var request openaidto.GeneralOpenAIRequest
		require.NoError(t, common.Unmarshal(body, &request))
		require.Equal(t, "vision-model", request.Model)
		require.Len(t, request.Messages, 2)
		require.Equal(t, "system", request.Messages[0].Role)
		require.Equal(t, model_setting.DefaultComfyUIH3ContextIRSystemPrompt, request.Messages[0].StringContent())
		require.Equal(t, "user", request.Messages[1].Role)
		content := request.Messages[1].ParseContent()
		require.Len(t, content, 4)
		require.Contains(t, content[0].Text, "Target video duration: 10 seconds")
		require.Contains(t, content[0].Text, "16:9 (Widescreen)")
		require.Equal(t, "<Picture 1>", content[2].Text)
		require.Equal(t, "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=", content[3].GetImageMedia().Url)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"integrated_multimodal_description: enhanced\noverall_soundscape: N/A\nnon_diegetic_music: N/A"}}]}`))
	}))
	defer server.Close()

	enhanced, err := requestEnhancedH3Prompt(
		context.Background(),
		relaycommon.TaskSubmitReq{Prompt: "舞者跳舞", Seconds: "10"},
		h3Selector{AspectRatio: "16:9 (Widescreen)", Megapixels: 2, Multiple: 32},
		[]string{"data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII="},
		model_setting.ComfyUIH3PromptEnhancerSettings{
			BaseURL:        server.URL,
			APIKey:         "secret",
			Model:          "vision-model",
			TimeoutSeconds: 1,
			SystemPrompt:   model_setting.DefaultComfyUIH3ContextIRSystemPrompt,
		},
		server.Client(),
	)
	require.NoError(t, err)
	require.Contains(t, enhanced, "integrated_multimodal_description: enhanced")
}

func TestRequestEnhancedH3PromptLabelsFirstAndLastFrameRoles(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var request openaidto.GeneralOpenAIRequest
		require.NoError(t, common.Unmarshal(body, &request))
		content := request.Messages[1].ParseContent()
		require.Len(t, content, 6)
		require.Contains(t, content[0].Text, "Generation mode: first-last-frame")
		require.Contains(t, content[1].Text, "<Picture 1> is the required first frame at 0.000 seconds")
		require.Contains(t, content[1].Text, "<Picture 2> is the required final rendered frame at 4.958 seconds")
		require.Contains(t, content[1].Text, "strictly less than 5.000 seconds")
		require.NotContains(t, content[1].Text, "last frame at 5.00 seconds")
		require.Equal(t, "<Picture 1>", content[2].Text)
		require.Equal(t, "<Picture 2>", content[4].Text)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"integrated_multimodal_description: enhanced\noverall_soundscape: N/A\nnon_diegetic_music: N/A"}}]}`))
	}))
	defer server.Close()

	image := "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII="
	_, err := requestEnhancedH3Prompt(
		context.Background(),
		relaycommon.TaskSubmitReq{Prompt: "transition between frames", Seconds: "5", Mode: firstLastFrameMode},
		h3Selector{AspectRatio: "16:9 (Widescreen)", Megapixels: 1, Multiple: 32},
		[]string{image, image},
		model_setting.ComfyUIH3PromptEnhancerSettings{
			BaseURL: server.URL, Model: "vision-model", TimeoutSeconds: 1, SystemPrompt: "system prompt",
		},
		server.Client(),
	)
	require.NoError(t, err)
}

func TestFirstLastFramePromptContextAlwaysUsesTimestampBeforeDuration(t *testing.T) {
	cases := []struct {
		seconds  int
		lastTime string
	}{
		{seconds: 1, lastTime: "0.958"},
		{seconds: 5, lastTime: "4.958"},
		{seconds: 10, lastTime: "9.958"},
		{seconds: 15, lastTime: "14.958"},
	}
	for _, testCase := range cases {
		contextText := firstLastFramePromptContext(testCase.seconds)
		require.Contains(t, contextText, fmt.Sprintf("final rendered frame at %s seconds", testCase.lastTime))
		require.Contains(t, contextText, fmt.Sprintf("strictly less than %d.000 seconds", testCase.seconds))
	}
}

func TestRequestEnhancedH3PromptUsesConfiguredChannel(t *testing.T) {
	originalExecute := executeInternalH3PromptChat
	executeInternalH3PromptChat = func(ctx context.Context, channelID int, request *openaidto.GeneralOpenAIRequest) (*openaidto.OpenAITextResponse, error) {
		require.Equal(t, 27, channelID)
		require.Equal(t, "vision-model", request.Model)
		require.Len(t, request.Messages, 2)
		require.Equal(t, "system prompt", request.Messages[0].StringContent())
		content := request.Messages[1].ParseContent()
		require.NotEmpty(t, content)
		require.Contains(t, content[0].Text, "Target video duration: 5 seconds")
		return &openaidto.OpenAITextResponse{Choices: []openaidto.OpenAITextResponseChoice{{
			Message: openaidto.Message{Role: "assistant", Content: "integrated_multimodal_description: channel enhanced\noverall_soundscape: N/A\nnon_diegetic_music: N/A"},
		}}}, nil
	}
	t.Cleanup(func() { executeInternalH3PromptChat = originalExecute })

	enhanced, err := requestEnhancedH3Prompt(
		context.Background(),
		relaycommon.TaskSubmitReq{Prompt: "舞者跳舞", Seconds: "5"},
		h3Selector{AspectRatio: defaultAspect, Megapixels: 1, Multiple: 32},
		nil,
		model_setting.ComfyUIH3PromptEnhancerSettings{
			ProviderMode: model_setting.ComfyUIH3PromptEnhancerProviderChannel,
			ChannelID:    27, Model: "vision-model", TimeoutSeconds: 1, SystemPrompt: "system prompt",
		},
		nil,
	)
	require.NoError(t, err)
	require.Contains(t, enhanced, "channel enhanced")
}

func TestApplyPromptEnhancementChannelModeDoesNotRequireDirectURL(t *testing.T) {
	originalSettings := model_setting.GetComfyUIH3PromptEnhancerSettings()
	model_setting.ReplaceComfyUIH3PromptEnhancerSettings(model_setting.ComfyUIH3PromptEnhancerSettings{
		Enabled: true, ProviderMode: model_setting.ComfyUIH3PromptEnhancerProviderChannel,
		ChannelID: 27, Model: "vision-model", SystemPrompt: "system", TimeoutSeconds: 1,
	})
	t.Cleanup(func() { model_setting.ReplaceComfyUIH3PromptEnhancerSettings(originalSettings) })

	originalEnhancer := enhanceH3Prompt
	calls := 0
	enhanceH3Prompt = func(_ context.Context, _ relaycommon.TaskSubmitReq, _ h3Selector, _ []string, settings model_setting.ComfyUIH3PromptEnhancerSettings, _ *http.Client) (string, error) {
		calls++
		require.Equal(t, 27, settings.ChannelID)
		return "channel prompt", nil
	}
	t.Cleanup(func() { enhanceH3Prompt = originalEnhancer })

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", nil)
	result := (&TaskAdaptor{baseURL: "http://comfy.example"}).applyPromptEnhancement(
		ctx,
		relaycommon.TaskSubmitReq{Prompt: "original", Seconds: "5"},
		h3Selector{AspectRatio: defaultAspect, Megapixels: 1, Multiple: 32},
	)
	require.Equal(t, "channel prompt", result.Prompt)
	require.Equal(t, 1, calls)
}

func TestApplyPromptEnhancementDefaultsOnAndCachesFinalPrompt(t *testing.T) {
	originalSettings := model_setting.GetComfyUIH3PromptEnhancerSettings()
	model_setting.ReplaceComfyUIH3PromptEnhancerSettings(model_setting.ComfyUIH3PromptEnhancerSettings{Enabled: true, BaseURL: "https://enhancer.example", Model: "vision", SystemPrompt: "system", TimeoutSeconds: 1})
	t.Cleanup(func() { model_setting.ReplaceComfyUIH3PromptEnhancerSettings(originalSettings) })

	originalEnhancer := enhanceH3Prompt
	calls := 0
	enhanceH3Prompt = func(_ context.Context, _ relaycommon.TaskSubmitReq, _ h3Selector, _ []string, _ model_setting.ComfyUIH3PromptEnhancerSettings, _ *http.Client) (string, error) {
		calls++
		return "enhanced prompt", nil
	}
	t.Cleanup(func() { enhanceH3Prompt = originalEnhancer })

	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", nil)
	adaptor := &TaskAdaptor{baseURL: "http://comfy.example"}
	req := relaycommon.TaskSubmitReq{Prompt: "original", Seconds: "5"}
	selector := h3Selector{AspectRatio: defaultAspect, Megapixels: 1, Multiple: 32}

	first := adaptor.applyPromptEnhancement(ctx, req, selector)
	second := adaptor.applyPromptEnhancement(ctx, req, selector)
	require.Equal(t, "enhanced prompt", first.Prompt)
	require.Equal(t, "enhanced prompt", second.Prompt)
	require.Equal(t, 1, calls)
}

func TestApplyPromptEnhancementHonorsExplicitFalseAndFallsBack(t *testing.T) {
	originalSettings := model_setting.GetComfyUIH3PromptEnhancerSettings()
	model_setting.ReplaceComfyUIH3PromptEnhancerSettings(model_setting.ComfyUIH3PromptEnhancerSettings{Enabled: true, BaseURL: "https://enhancer.example", Model: "vision", SystemPrompt: "system", TimeoutSeconds: 1})
	t.Cleanup(func() { model_setting.ReplaceComfyUIH3PromptEnhancerSettings(originalSettings) })

	originalEnhancer := enhanceH3Prompt
	calls := 0
	enhanceH3Prompt = func(_ context.Context, _ relaycommon.TaskSubmitReq, _ h3Selector, _ []string, _ model_setting.ComfyUIH3PromptEnhancerSettings, _ *http.Client) (string, error) {
		calls++
		return "", context.DeadlineExceeded
	}
	t.Cleanup(func() { enhanceH3Prompt = originalEnhancer })

	gin.SetMode(gin.TestMode)
	adaptor := &TaskAdaptor{baseURL: "http://comfy.example"}
	selector := h3Selector{AspectRatio: defaultAspect, Megapixels: 1, Multiple: 32}
	disabled := false
	disabledCtx, _ := gin.CreateTestContext(httptest.NewRecorder())
	disabledCtx.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", nil)
	disabledReq := adaptor.applyPromptEnhancement(disabledCtx, relaycommon.TaskSubmitReq{Prompt: "original", Seconds: "5", PromptEnhance: &disabled}, selector)
	require.Equal(t, "original", disabledReq.Prompt)
	require.Equal(t, 0, calls)

	fallbackCtx, _ := gin.CreateTestContext(httptest.NewRecorder())
	fallbackCtx.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", nil)
	fallbackReq := adaptor.applyPromptEnhancement(fallbackCtx, relaycommon.TaskSubmitReq{Prompt: "original", Seconds: "5"}, selector)
	require.Equal(t, "original", fallbackReq.Prompt)
	require.Equal(t, 1, calls)
}

func TestApplyPromptEnhancementLeavesMultiImagePromptUnchangedWhenExplicitlyDisabled(t *testing.T) {
	originalSettings := model_setting.GetComfyUIH3PromptEnhancerSettings()
	model_setting.ReplaceComfyUIH3PromptEnhancerSettings(model_setting.ComfyUIH3PromptEnhancerSettings{
		Enabled: true, BaseURL: "https://enhancer.example", Model: "vision", SystemPrompt: "system", TimeoutSeconds: 1,
	})
	t.Cleanup(func() { model_setting.ReplaceComfyUIH3PromptEnhancerSettings(originalSettings) })

	originalEnhancer := enhanceH3Prompt
	calls := 0
	enhanceH3Prompt = func(_ context.Context, _ relaycommon.TaskSubmitReq, _ h3Selector, _ []string, _ model_setting.ComfyUIH3PromptEnhancerSettings, _ *http.Client) (string, error) {
		calls++
		return "", nil
	}
	t.Cleanup(func() { enhanceH3Prompt = originalEnhancer })

	disabled := false
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", nil)
	prompt := "combine picture one and picture two into one video"
	result := (&TaskAdaptor{baseURL: "http://comfy.example"}).applyPromptEnhancement(
		ctx,
		relaycommon.TaskSubmitReq{
			Prompt:        prompt,
			Seconds:       "5",
			PromptEnhance: &disabled,
			Images: []string{
				"https://example.invalid/reference-1.png",
				"https://example.invalid/reference-2.png",
			},
		},
		h3Selector{AspectRatio: defaultAspect, Megapixels: 1, Multiple: 32},
	)

	require.Equal(t, 0, calls)
	require.Equal(t, prompt, result.Prompt)
}

func TestApplyPromptEnhancementFallbackLeavesMultiImagePromptUnchanged(t *testing.T) {
	originalSettings := model_setting.GetComfyUIH3PromptEnhancerSettings()
	model_setting.ReplaceComfyUIH3PromptEnhancerSettings(model_setting.ComfyUIH3PromptEnhancerSettings{
		Enabled: true, BaseURL: "https://enhancer.example", Model: "vision", SystemPrompt: "system", TimeoutSeconds: 1,
	})
	t.Cleanup(func() { model_setting.ReplaceComfyUIH3PromptEnhancerSettings(originalSettings) })

	originalEnhancer := enhanceH3Prompt
	enhanceH3Prompt = func(_ context.Context, _ relaycommon.TaskSubmitReq, _ h3Selector, _ []string, _ model_setting.ComfyUIH3PromptEnhancerSettings, _ *http.Client) (string, error) {
		return "", context.DeadlineExceeded
	}
	t.Cleanup(func() { enhanceH3Prompt = originalEnhancer })

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", nil)
	prompt := "combine picture one and picture two into one video"
	result := (&TaskAdaptor{baseURL: "http://comfy.example"}).applyPromptEnhancement(
		ctx,
		relaycommon.TaskSubmitReq{
			Prompt:  prompt,
			Seconds: "5",
			Images: []string{
				"data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=",
				"data:image/PNG;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=",
			},
		},
		h3Selector{AspectRatio: defaultAspect, Megapixels: 1, Multiple: 32},
	)

	require.Equal(t, prompt, result.Prompt)
}

func TestRequestEnhancedH3PromptHonorsTimeout(t *testing.T) {
	client := &http.Client{Transport: promptEnhancerRoundTripper(func(request *http.Request) (*http.Response, error) {
		<-request.Context().Done()
		return nil, request.Context().Err()
	})}

	started := time.Now()
	_, err := requestEnhancedH3Prompt(
		context.Background(),
		relaycommon.TaskSubmitReq{Prompt: "test", Seconds: "5"},
		h3Selector{AspectRatio: defaultAspect, Megapixels: 1, Multiple: 32},
		nil,
		model_setting.ComfyUIH3PromptEnhancerSettings{BaseURL: "https://enhancer.example", Model: "vision", SystemPrompt: "system", TimeoutSeconds: 1},
		client,
	)
	require.Error(t, err)
	require.Contains(t, strings.ToLower(err.Error()), "deadline")
	require.Less(t, time.Since(started), 3*time.Second)
}

func TestPromptEnhancerRequestTimeoutCapsOptionalEnhancementWait(t *testing.T) {
	setPromptEnhancerMaxWait(t, promptEnhancerDefaultMaxWait)
	require.Equal(t, promptEnhancerDefaultMaxWait, promptEnhancerRequestTimeout(model_setting.ComfyUIH3PromptEnhancerSettings{}))
	require.Equal(t, 3*time.Second, promptEnhancerRequestTimeout(model_setting.ComfyUIH3PromptEnhancerSettings{TimeoutSeconds: 3}))
	require.Equal(t, promptEnhancerDefaultMaxWait, promptEnhancerRequestTimeout(model_setting.ComfyUIH3PromptEnhancerSettings{TimeoutSeconds: 20}))
}

func TestApplyPromptEnhancementUsesSingleDeadline(t *testing.T) {
	setPromptEnhancerMaxWait(t, 40*time.Millisecond)
	originalSettings := model_setting.GetComfyUIH3PromptEnhancerSettings()
	settings := model_setting.ComfyUIH3PromptEnhancerSettings{
		Enabled:      true,
		ProviderMode: model_setting.ComfyUIH3PromptEnhancerProviderChannel,
		ChannelID:    27,
		Model:        "vision-model",
		SystemPrompt: "system",
	}
	model_setting.ReplaceComfyUIH3PromptEnhancerSettings(settings)
	t.Cleanup(func() { model_setting.ReplaceComfyUIH3PromptEnhancerSettings(originalSettings) })

	originalEnhancer := enhanceH3Prompt
	enhanceH3Prompt = func(ctx context.Context, _ relaycommon.TaskSubmitReq, _ h3Selector, _ []string, _ model_setting.ComfyUIH3PromptEnhancerSettings, _ *http.Client) (string, error) {
		<-ctx.Done()
		return "", ctx.Err()
	}
	t.Cleanup(func() { enhanceH3Prompt = originalEnhancer })

	adaptor := &TaskAdaptor{baseURL: "http://comfy.example"}
	selector := h3Selector{AspectRatio: defaultAspect, Megapixels: 1, Multiple: 32}
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", nil)
	started := time.Now()
	result := adaptor.applyPromptEnhancement(ctx, relaycommon.TaskSubmitReq{Prompt: "original", Seconds: "5"}, selector)
	require.Equal(t, "original", result.Prompt)
	require.Less(t, time.Since(started), 500*time.Millisecond)
}

func TestPromptEnhancerImagesHonorContextDeadline(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err := (&TaskAdaptor{baseURL: server.URL}).promptEnhancerImages(ctx, nil, relaycommon.TaskSubmitReq{Images: []string{"reference.png"}})
	require.Error(t, err)
	require.Less(t, time.Since(started), 500*time.Millisecond)
}

func TestPromptEnhancerEndpointRejectsQueryAndBuildsPathSafely(t *testing.T) {
	endpoint, err := promptEnhancerEndpoint("https://example.com/proxy/v1")
	require.NoError(t, err)
	require.Equal(t, "https://example.com/proxy/v1/chat/completions", endpoint)
	_, err = promptEnhancerEndpoint("https://example.com/proxy?tenant=one")
	require.Error(t, err)
}

func TestPromptEnhancerImageDataURLUsesDetectedMIMEType(t *testing.T) {
	pngBytes, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	require.NoError(t, err)

	dataURL, err := promptEnhancerImageDataURL(referenceInput{Name: "reference.jpg", Data: pngBytes})
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(dataURL, "data:image/png;base64,"))
}

func TestPromptEnhancerImageDataURLRejectsNonImageBytes(t *testing.T) {
	_, err := promptEnhancerImageDataURL(referenceInput{Name: "reference.jpg", Data: []byte("not an image")})
	require.ErrorContains(t, err, "not a valid image")
}

func TestPromptEnhancerImageBudgetRejectsNinthFourMegabyteImage(t *testing.T) {
	var totalBytes int64
	for index := 0; index < 8; index++ {
		require.NoError(t, reservePromptEnhancerImageBytes(&totalBytes, 4*1024*1024))
	}
	require.ErrorContains(t, reservePromptEnhancerImageBytes(&totalBytes, 4*1024*1024), "aggregate size")
	require.Equal(t, int64(32*1024*1024), totalBytes)
}

func TestPromptEnhancerImageBudgetRejectsOversizedSingleImage(t *testing.T) {
	var totalBytes int64
	require.ErrorContains(t, reservePromptEnhancerImageBytes(&totalBytes, maxPromptEnhancerImageBytes+1), "image exceeds")
	require.Zero(t, totalBytes)
}
