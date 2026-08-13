package controller

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestValidateComfyUIH3PromptEnhancerBaseURL(t *testing.T) {
	require.NoError(t, validateComfyUIH3PromptEnhancerOption(
		"comfyui_h3_prompt_enhancer.base_url",
		"https://enhancer.example/proxy/v1",
	))
	require.Error(t, validateComfyUIH3PromptEnhancerOption(
		"comfyui_h3_prompt_enhancer.base_url",
		"https://enhancer.example/proxy?tenant=one",
	))
	require.Error(t, validateComfyUIH3PromptEnhancerOption(
		"comfyui_h3_prompt_enhancer.base_url",
		"https://enhancer.example/proxy#fragment",
	))
}

func TestValidateComfyUIH3PromptEnhancerTimeout(t *testing.T) {
	require.NoError(t, validateComfyUIH3PromptEnhancerOption(
		"comfyui_h3_prompt_enhancer.timeout_seconds",
		"8",
	))
	require.Error(t, validateComfyUIH3PromptEnhancerOption(
		"comfyui_h3_prompt_enhancer.timeout_seconds",
		"0",
	))
	require.Error(t, validateComfyUIH3PromptEnhancerOption(
		"comfyui_h3_prompt_enhancer.timeout_seconds",
		"301",
	))
}

func TestValidateComfyUIH3PromptEnhancerEnabled(t *testing.T) {
	require.NoError(t, validateComfyUIH3PromptEnhancerOption(
		"comfyui_h3_prompt_enhancer.enabled",
		"false",
	))
	require.Error(t, validateComfyUIH3PromptEnhancerOption(
		"comfyui_h3_prompt_enhancer.enabled",
		"enabled",
	))
}

func TestNormalizeComfyUIH3PromptEnhancerProviderMode(t *testing.T) {
	for _, input := range []string{"", "direct", "external", " DIRECT "} {
		mode, err := normalizeComfyUIH3PromptEnhancerProviderMode(input)
		require.NoError(t, err)
		require.Equal(t, model_setting.ComfyUIH3PromptEnhancerProviderDirect, mode)
	}
	mode, err := normalizeComfyUIH3PromptEnhancerProviderMode(" channel ")
	require.NoError(t, err)
	require.Equal(t, model_setting.ComfyUIH3PromptEnhancerProviderChannel, mode)
	_, err = normalizeComfyUIH3PromptEnhancerProviderMode("automatic")
	require.ErrorContains(t, err, "必须是渠道或直连")
}

func TestValidateComfyUIH3PromptEnhancerChannel(t *testing.T) {
	originalGetter := getComfyUIH3PromptEnhancerChannelByID
	t.Cleanup(func() { getComfyUIH3PromptEnhancerChannelByID = originalGetter })

	getComfyUIH3PromptEnhancerChannelByID = func(id int, selectAll bool) (*model.Channel, error) {
		require.Equal(t, 12, id)
		require.True(t, selectAll)
		return &model.Channel{
			Id: 12, Type: constant.ChannelTypeOpenAI,
			Status: common.ChannelStatusEnabled, Models: "gpt-4o,vision-model",
		}, nil
	}
	require.NoError(t, validateComfyUIH3PromptEnhancerChannel(12, "vision-model"))
	require.ErrorContains(t, validateComfyUIH3PromptEnhancerChannel(12, "missing-model"), "not configured")
	require.ErrorContains(t, validateComfyUIH3PromptEnhancerChannel(0, "vision-model"), "请选择")

	getComfyUIH3PromptEnhancerChannelByID = func(int, bool) (*model.Channel, error) {
		return nil, errors.New("database unavailable")
	}
	require.ErrorContains(t, validateComfyUIH3PromptEnhancerChannel(12, "vision-model"), "读取提示词增强渠道失败")
}

func TestGetComfyUIH3PromptEnhancerChannelsFiltersAndDoesNotExposeSecrets(t *testing.T) {
	advancedChat := &model.Channel{Id: 6, Name: "advanced-chat", Type: constant.ChannelTypeAdvancedCustom, Status: common.ChannelStatusEnabled, Models: " vision-model,other-model ", Key: "advanced-secret"}
	advancedChat.SetOtherSettings(dto.ChannelOtherSettings{AdvancedCustom: &dto.AdvancedCustomConfig{
		Routes: []dto.AdvancedCustomRoute{{
			IncomingPath: "/v1/chat/completions",
			UpstreamPath: "/provider/chat",
			Models:       []string{"vision-model"},
		}},
	}})
	advancedResponses := &model.Channel{Id: 7, Name: "advanced-responses", Type: constant.ChannelTypeAdvancedCustom, Status: common.ChannelStatusEnabled, Models: "vision-model", Key: "responses-secret"}
	advancedResponses.SetOtherSettings(dto.ChannelOtherSettings{AdvancedCustom: &dto.AdvancedCustomConfig{
		Routes: []dto.AdvancedCustomRoute{{
			IncomingPath: "/v1/responses",
			UpstreamPath: "/provider/responses",
		}},
	}})
	originalList := listComfyUIH3PromptEnhancerChannels
	listComfyUIH3PromptEnhancerChannels = func() ([]*model.Channel, error) {
		return []*model.Channel{
			{Id: 3, Name: "usable", Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled, Models: " vision-model,vision-model, gpt-4o ", Key: "must-not-leak", Other: "secret-other"},
			{Id: 4, Name: "disabled", Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusManuallyDisabled, Models: "gpt-4o", Key: "disabled-secret"},
			{Id: 5, Name: "video-only", Type: constant.ChannelTypeComfyUIH3, Status: common.ChannelStatusEnabled, Models: "minimax_h3", Key: "video-secret"},
			advancedChat,
			advancedResponses,
		}, nil
	}
	t.Cleanup(func() { listComfyUIH3PromptEnhancerChannels = originalList })

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	GetComfyUIH3PromptEnhancerChannels(context)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{
		"success": true,
		"message": "",
		"data": [
			{"id":6,"name":"advanced-chat","type":58,"status":1,"models":["vision-model"]},
			{"id":3,"name":"usable","type":1,"status":1,"models":["gpt-4o","vision-model"]}
		]
	}`, recorder.Body.String())
	require.NotContains(t, recorder.Body.String(), "must-not-leak")
	require.NotContains(t, recorder.Body.String(), "secret-other")
	require.NotContains(t, recorder.Body.String(), "advanced-secret")
	require.NotContains(t, recorder.Body.String(), "responses-secret")
}

func TestResolveComfyUIH3PromptEnhancerAPIKey(t *testing.T) {
	current := model_setting.ComfyUIH3PromptEnhancerSettings{
		BaseURL: "https://enhancer.example/proxy/",
		APIKey:  "stored-key",
	}

	t.Run("same normalized URL and blank key retains stored key", func(t *testing.T) {
		require.Equal(t, "stored-key", resolveComfyUIH3PromptEnhancerAPIKey(
			" https://enhancer.example/proxy ",
			" ",
			false,
			current,
		))
	})

	t.Run("changed URL and blank key clears stored key", func(t *testing.T) {
		require.Empty(t, resolveComfyUIH3PromptEnhancerAPIKey(
			"https://other.example/proxy",
			"",
			false,
			current,
		))
	})

	t.Run("supplied key replaces stored key", func(t *testing.T) {
		require.Equal(t, "new-key", resolveComfyUIH3PromptEnhancerAPIKey(
			current.BaseURL,
			" new-key ",
			false,
			current,
		))
	})

	t.Run("explicit clear removes stored key for same URL", func(t *testing.T) {
		require.Empty(t, resolveComfyUIH3PromptEnhancerAPIKey(
			current.BaseURL,
			"",
			true,
			current,
		))
	})
}

func TestGenericOptionEndpointRejectsComfyUIH3PromptEnhancerFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(
		http.MethodPut,
		"/api/option/",
		bytes.NewBufferString(`{"key":"comfyui_h3_prompt_enhancer.base_url","value":"https://other.example/v1"}`),
	)
	context.Request.Header.Set("Content-Type", "application/json")

	UpdateOption(context)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), "必须通过专用设置接口整体保存")
}

func TestUpdateComfyUIH3PromptEnhancerRejectsNewAndClearedAPIKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(
		http.MethodPut,
		"/api/option/comfyui-h3-prompt-enhancer",
		bytes.NewBufferString(`{
			"enabled": false,
			"base_url": "https://enhancer.example/v1",
			"api_key": "new-key",
			"clear_api_key": true,
			"model": "enhancer-model",
			"timeout_seconds": 8,
			"system_prompt": "system prompt"
		}`),
	)
	context.Request.Header.Set("Content-Type", "application/json")

	UpdateComfyUIH3PromptEnhancer(context)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), "不能同时提供新的 API Key")
}

func TestUpdateComfyUIH3PromptEnhancerChannelModeRejectsInvalidChannelBeforeSaving(t *testing.T) {
	originalGetter := getComfyUIH3PromptEnhancerChannelByID
	getComfyUIH3PromptEnhancerChannelByID = func(id int, selectAll bool) (*model.Channel, error) {
		return &model.Channel{
			Id: id, Type: constant.ChannelTypeComfyUIH3,
			Status: common.ChannelStatusEnabled, Models: "vision-model",
		}, nil
	}
	t.Cleanup(func() { getComfyUIH3PromptEnhancerChannelByID = originalGetter })

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(
		http.MethodPut,
		"/api/option/comfyui-h3-prompt-enhancer",
		bytes.NewBufferString(`{
			"enabled": true,
			"provider_mode": "channel",
			"channel_id": 7,
			"base_url": "not-a-url",
			"api_key": "ignored-direct-key",
			"clear_api_key": true,
			"model": "vision-model",
			"timeout_seconds": 8,
			"system_prompt": "system prompt"
		}`),
	)
	context.Request.Header.Set("Content-Type", "application/json")

	UpdateComfyUIH3PromptEnhancer(context)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), "does not support OpenAI chat completions")
	require.NotContains(t, recorder.Body.String(), "不能同时提供新的 API Key")
	require.NotContains(t, recorder.Body.String(), "有效的 HTTP")
}
