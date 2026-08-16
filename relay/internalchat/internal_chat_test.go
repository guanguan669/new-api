package internalchat

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateChannel(t *testing.T) {
	advancedChatChannel := testChannel(8, constant.ChannelTypeAdvancedCustom, common.ChannelStatusEnabled, "vision-model,other-model")
	advancedChatChannel.SetOtherSettings(dto.ChannelOtherSettings{AdvancedCustom: &dto.AdvancedCustomConfig{
		Routes: []dto.AdvancedCustomRoute{{
			IncomingPath: "/v1/chat/completions",
			UpstreamPath: "/provider/chat",
			Models:       []string{"vision-model"},
		}},
	}})
	advancedResponsesChannel := testChannel(9, constant.ChannelTypeAdvancedCustom, common.ChannelStatusEnabled, "vision-model")
	advancedResponsesChannel.SetOtherSettings(dto.ChannelOtherSettings{AdvancedCustom: &dto.AdvancedCustomConfig{
		Routes: []dto.AdvancedCustomRoute{{
			IncomingPath: "/v1/responses",
			UpstreamPath: "/provider/responses",
		}},
	}})
	tests := []struct {
		name    string
		channel *model.Channel
		model   string
		wantErr string
	}{
		{
			name:    "enabled chat channel and trimmed model",
			channel: testChannel(7, constant.ChannelTypeOpenAI, common.ChannelStatusEnabled, " gpt-4o , vision-model "),
			model:   "gpt-4o",
		},
		{
			name:    "legacy azure chat channel",
			channel: testChannel(7, constant.ChannelTypeAzure, common.ChannelStatusEnabled, "gpt-4o"),
			model:   "gpt-4o",
		},
		{
			name:    "disabled channel",
			channel: testChannel(7, constant.ChannelTypeOpenAI, common.ChannelStatusManuallyDisabled, "gpt-4o"),
			model:   "gpt-4o",
			wantErr: "disabled",
		},
		{
			name:    "wrong model",
			channel: testChannel(7, constant.ChannelTypeOpenAI, common.ChannelStatusEnabled, "gpt-4o"),
			model:   "gpt-4.1",
			wantErr: "not configured",
		},
		{
			name:    "task channel cannot recurse",
			channel: testChannel(7, constant.ChannelTypeComfyUIH3, common.ChannelStatusEnabled, "minimax_h3"),
			model:   "minimax_h3",
			wantErr: "does not support OpenAI chat completions",
		},
		{
			name:    "responses only channel",
			channel: testChannel(7, constant.ChannelTypeCodex, common.ChannelStatusEnabled, "gpt-5-codex"),
			model:   "gpt-5-codex",
			wantErr: "does not support OpenAI chat completions",
		},
		{
			name:    "advanced custom matching chat route",
			channel: advancedChatChannel,
			model:   "vision-model",
		},
		{
			name:    "advanced custom route excludes selected model",
			channel: advancedChatChannel,
			model:   "other-model",
			wantErr: "does not configure /v1/chat/completions",
		},
		{
			name:    "advanced custom responses route is not chat",
			channel: advancedResponsesChannel,
			model:   "vision-model",
			wantErr: "does not support OpenAI chat completions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateChannel(tt.channel, tt.model)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestExecuteUsesExactChannelMappingAndReturnsOpenAIResponse(t *testing.T) {
	var upstreamModel string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/chat/completions", r.URL.Path)
		assert.Equal(t, "Bearer header-override-secret", r.Header.Get("Authorization"))
		assert.Equal(t, "enhancer", r.Header.Get("X-Internal-Purpose"))
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var request dto.GeneralOpenAIRequest
		require.NoError(t, json.Unmarshal(body, &request))
		upstreamModel = request.Model
		require.NotNil(t, request.Temperature)
		assert.InDelta(t, 0.25, *request.Temperature, 0.0001)
		assert.NotNil(t, request.Stream)
		assert.False(t, *request.Stream)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"chatcmpl-test","object":"chat.completion","created":1,"model":"mapped-model","choices":[{"index":0,"message":{"role":"assistant","content":"enhanced prompt"},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":2,"total_tokens":5}}`)
	}))
	defer server.Close()

	baseURL := server.URL
	mapping := `{"visible-model":"mapped-model"}`
	paramOverride := `{"temperature":0.25}`
	headerOverride := `{"Authorization":"Bearer header-override-secret","X-Internal-Purpose":"enhancer"}`
	selectedID := 0
	originalGetter := getChannelByID
	getChannelByID = func(id int, selectAll bool) (*model.Channel, error) {
		selectedID = id
		assert.True(t, selectAll)
		channel := testChannel(id, constant.ChannelTypeOpenAI, common.ChannelStatusEnabled, "visible-model")
		channel.Key = "upstream-secret"
		channel.BaseURL = &baseURL
		channel.ModelMapping = &mapping
		channel.ParamOverride = &paramOverride
		channel.HeaderOverride = &headerOverride
		return channel, nil
	}
	t.Cleanup(func() { getChannelByID = originalGetter })

	stream := true
	response, err := Execute(context.Background(), 42, &dto.GeneralOpenAIRequest{
		Model:  "visible-model",
		Stream: &stream,
		Messages: []dto.Message{{
			Role:    "user",
			Content: "hello",
		}},
	})
	require.NoError(t, err)
	require.NotNil(t, response)
	assert.Equal(t, 42, selectedID)
	assert.Equal(t, "mapped-model", upstreamModel)
	require.Len(t, response.Choices, 1)
	assert.Equal(t, "enhanced prompt", response.Choices[0].Message.StringContent())
}

func TestExecutePreservingSystemRoleKeepsGPT5SystemMessage(t *testing.T) {
	service.InitTokenEncoders()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/chat/completions", r.URL.Path)
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var request dto.GeneralOpenAIRequest
		require.NoError(t, json.Unmarshal(body, &request))
		require.Len(t, request.Messages, 2)
		assert.Equal(t, "system", request.Messages[0].Role)
		assert.Equal(t, "user", request.Messages[1].Role)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"preserved"}}]}`)
	}))
	defer server.Close()

	baseURL := server.URL
	originalGetter := getChannelByID
	getChannelByID = func(id int, selectAll bool) (*model.Channel, error) {
		require.Equal(t, 61, id)
		require.True(t, selectAll)
		channel := testChannel(id, constant.ChannelTypeOpenAI, common.ChannelStatusEnabled, "gpt-5.6-terra")
		channel.Key = "upstream-secret"
		channel.BaseURL = &baseURL
		return channel, nil
	}
	t.Cleanup(func() { getChannelByID = originalGetter })

	originalShouldUseResponses := shouldUseResponsesForInternalChat
	shouldUseResponsesForInternalChat = func(int, int, string) bool { return false }
	t.Cleanup(func() { shouldUseResponsesForInternalChat = originalShouldUseResponses })

	response, err := ExecutePreservingSystemRole(context.Background(), 61, &dto.GeneralOpenAIRequest{
		Model: "gpt-5.6-terra",
		Messages: []dto.Message{
			{Role: "system", Content: "system instructions"},
			{Role: "user", Content: "user request"},
		},
	})
	require.NoError(t, err)
	require.Len(t, response.Choices, 1)
	assert.Equal(t, "preserved", response.Choices[0].Message.StringContent())
}

func TestExecuteHonorsContextDeadline(t *testing.T) {
	requestStarted := make(chan struct{})
	releaseHandler := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(requestStarted)
		select {
		case <-r.Context().Done():
		case <-releaseHandler:
		}
	}))
	defer server.Close()
	defer close(releaseHandler)

	baseURL := server.URL
	originalGetter := getChannelByID
	getChannelByID = func(id int, selectAll bool) (*model.Channel, error) {
		channel := testChannel(id, constant.ChannelTypeOpenAI, common.ChannelStatusEnabled, "visible-model")
		channel.Key = "upstream-secret"
		channel.BaseURL = &baseURL
		return channel, nil
	}
	t.Cleanup(func() { getChannelByID = originalGetter })

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	result := make(chan error, 1)
	go func() {
		_, err := Execute(ctx, 42, &dto.GeneralOpenAIRequest{
			Model:    "visible-model",
			Messages: []dto.Message{{Role: "user", Content: "hello"}},
		})
		result <- err
	}()
	select {
	case <-requestStarted:
	case <-time.After(time.Second):
		t.Fatal("upstream request did not start")
	}
	started := time.Now()
	err := <-result
	require.Error(t, err)
	require.Less(t, time.Since(started), time.Second)
}

func TestExecuteUsesResponsesRouteWhenChannelPolicyRequiresIt(t *testing.T) {
	var upstreamModel string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/responses", r.URL.Path)
		assert.Equal(t, "Bearer upstream-secret", r.Header.Get("Authorization"))
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var request dto.OpenAIResponsesRequest
		require.NoError(t, json.Unmarshal(body, &request))
		upstreamModel = request.Model
		require.NotNil(t, request.Stream)
		assert.False(t, *request.Stream)
		assert.NotEmpty(t, request.Input)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{
			"id":"resp-test","object":"response","created_at":1,"status":"completed","model":"mapped-vision",
			"output":[{"type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"enhanced via responses"}]}],
			"usage":{"input_tokens":3,"output_tokens":2,"total_tokens":5}
		}`)
	}))
	defer server.Close()

	baseURL := server.URL
	mapping := `{"visible-model":"mapped-vision"}`
	originalGetter := getChannelByID
	getChannelByID = func(id int, selectAll bool) (*model.Channel, error) {
		require.True(t, selectAll)
		channel := testChannel(id, constant.ChannelTypeNewAPI, common.ChannelStatusEnabled, "visible-model")
		channel.Key = "upstream-secret"
		channel.BaseURL = &baseURL
		channel.ModelMapping = &mapping
		return channel, nil
	}
	t.Cleanup(func() { getChannelByID = originalGetter })

	originalShouldUseResponses := shouldUseResponsesForInternalChat
	shouldUseResponsesForInternalChat = func(channelID int, channelType int, model string) bool {
		assert.Equal(t, 51, channelID)
		assert.Equal(t, constant.ChannelTypeNewAPI, channelType)
		assert.Equal(t, "visible-model", model)
		return true
	}
	t.Cleanup(func() { shouldUseResponsesForInternalChat = originalShouldUseResponses })

	response, err := Execute(context.Background(), 51, &dto.GeneralOpenAIRequest{
		Model:    "visible-model",
		Messages: []dto.Message{{Role: "user", Content: "write a video prompt"}},
	})
	require.NoError(t, err)
	require.NotNil(t, response)
	assert.Equal(t, "mapped-vision", upstreamModel)
	require.Len(t, response.Choices, 1)
	assert.Equal(t, "enhanced via responses", response.Choices[0].Message.StringContent())
}

func TestExecuteRejectsInvalidSelectedChannelBeforeRequest(t *testing.T) {
	originalGetter := getChannelByID
	getChannelByID = func(id int, selectAll bool) (*model.Channel, error) {
		return testChannel(id, constant.ChannelTypeOpenAI, common.ChannelStatusEnabled, "other-model"), nil
	}
	t.Cleanup(func() { getChannelByID = originalGetter })

	_, err := Execute(context.Background(), 99, &dto.GeneralOpenAIRequest{Model: "requested-model"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not configured")
}

func testChannel(id, channelType, status int, models string) *model.Channel {
	return &model.Channel{
		Id:     id,
		Type:   channelType,
		Status: status,
		Models: models,
		Name:   "test",
	}
}
