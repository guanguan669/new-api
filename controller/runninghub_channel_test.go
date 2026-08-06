package controller

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunningHubChannelRegistrationAndValidation(t *testing.T) {
	apiType, ok := common.ChannelType2APIType(constant.ChannelTypeRunningHub)
	require.True(t, ok)
	assert.Equal(t, constant.APITypeOpenAI, apiType)
	assert.Equal(t, []constant.EndpointType{constant.EndpointTypeOpenAIVideo}, common.GetEndpointTypesByChannelType(constant.ChannelTypeRunningHub, "minimax_h3"))
	assert.Equal(t, "RunningHub", constant.GetChannelTypeName(constant.ChannelTypeRunningHub))
	require.Greater(t, len(constant.ChannelBaseURLs), constant.ChannelTypeRunningHub)
	assert.Equal(t, "https://www.runninghub.cn", constant.ChannelBaseURLs[constant.ChannelTypeRunningHub])

	channel := &model.Channel{Type: constant.ChannelTypeRunningHub}
	require.ErrorContains(t, validateChannel(channel, false), "RunningHub image-to-video workflow ID cannot be empty")

	channel.SetOtherSettings(dto.ChannelOtherSettings{RunningHubWorkflowID: "wf-image", RunningHubTextWorkflowID: "wf-text"})
	require.NoError(t, validateChannel(channel, false))
}

func TestRunningHubChannelTestUsesAPIFormatEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/openapi/getJsonApiFormat", r.URL.Path)
		assert.Equal(t, "Bearer rh-test-key", r.Header.Get("Authorization"))
		var payload map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
		assert.Equal(t, "rh-test-key", payload["apiKey"])
		w.Header().Set("Content-Type", "application/json")
		switch payload["workflowId"] {
		case "wf-image":
			_, _ = w.Write(runningHubH3APIFormatResponse(t))
		case "wf-text":
			_, _ = w.Write(runningHubH3TextAPIFormatResponse(t))
		default:
			t.Fatalf("unexpected workflow id %q", payload["workflowId"])
		}
	}))
	defer server.Close()

	channel := &model.Channel{
		Type:    constant.ChannelTypeRunningHub,
		BaseURL: common.GetPointer(server.URL),
		Key:     "rh-test-key",
	}
	channel.SetOtherSettings(dto.ChannelOtherSettings{RunningHubWorkflowID: "wf-image", RunningHubTextWorkflowID: "wf-text"})

	result := testChannel(context.Background(), channel, 0, "minimax_h3", "", false)
	require.NoError(t, result.localErr)
	assert.Equal(t, string(constant.EndpointTypeOpenAIVideo), normalizeChannelTestEndpoint(channel, "minimax_h3", ""))
}

func TestRunningHubChannelTestRedactsKeyOnError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad rh-secret-key", http.StatusUnauthorized)
	}))
	defer server.Close()

	channel := &model.Channel{
		Type:    constant.ChannelTypeRunningHub,
		BaseURL: common.GetPointer(server.URL),
		Key:     "rh-secret-key",
	}
	channel.SetOtherSettings(dto.ChannelOtherSettings{RunningHubWorkflowID: "wf-image", RunningHubTextWorkflowID: "wf-text"})

	result := testChannel(context.Background(), channel, 0, "minimax_h3", "", false)
	require.Error(t, result.localErr)
	assert.NotContains(t, result.localErr.Error(), "rh-secret-key")
	assert.Contains(t, result.localErr.Error(), "[REDACTED]")
}

func TestFetchRunningHubModelsValidatesAPIFormatAndReturnsFixedH3(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/openapi/getJsonApiFormat", r.URL.Path)
		assert.Equal(t, "Bearer rh-fetch-key", r.Header.Get("Authorization"))
		var payload map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
		assert.Equal(t, "rh-fetch-key", payload["apiKey"])
		w.Header().Set("Content-Type", "application/json")
		switch payload["workflowId"] {
		case "wf-image":
			_, _ = w.Write(runningHubH3APIFormatResponse(t))
		case "wf-text":
			_, _ = w.Write(runningHubH3TextAPIFormatResponse(t))
		default:
			t.Fatalf("unexpected workflow id %q", payload["workflowId"])
		}
	}))
	defer server.Close()

	channel := &model.Channel{
		Type:    constant.ChannelTypeRunningHub,
		BaseURL: common.GetPointer(server.URL),
		Key:     "rh-fetch-key",
	}
	channel.SetOtherSettings(dto.ChannelOtherSettings{RunningHubWorkflowID: "wf-image", RunningHubTextWorkflowID: "wf-text"})

	models, err := fetchChannelUpstreamModelIDs(channel)
	require.NoError(t, err)
	assert.Equal(t, []string{"minimax_h3"}, models)
}

func TestValidateRunningHubAPIFormatAcceptsPromptJSONString(t *testing.T) {
	body := runningHubH3APIFormatResponse(t)
	var response map[string]any
	require.NoError(t, json.Unmarshal(body, &response))
	promptBytes, err := json.Marshal(response["data"].(map[string]any)["prompt"])
	require.NoError(t, err)
	response["data"].(map[string]any)["prompt"] = string(promptBytes)
	stringPromptBody, err := json.Marshal(response)
	require.NoError(t, err)

	require.NoError(t, validateRunningHubAPIFormatResponse(stringPromptBody))
}

func TestValidateRunningHubAPIFormatRequiresH3Inputs(t *testing.T) {
	body := runningHubH3APIFormatResponse(t)
	require.NoError(t, validateRunningHubAPIFormatResponse(body))

	var response map[string]any
	require.NoError(t, json.Unmarshal(body, &response))
	prompt := response["data"].(map[string]any)["prompt"].(map[string]any)
	delete(prompt, "138")
	broken, err := json.Marshal(response)
	require.NoError(t, err)
	require.ErrorContains(t, validateRunningHubAPIFormatResponse(broken), "node 138")
}

func TestValidateRunningHubAPIFormatRejectsCompetingImageOutput(t *testing.T) {
	body := runningHubH3APIFormatResponse(t)
	var response map[string]any
	require.NoError(t, json.Unmarshal(body, &response))
	prompt := response["data"].(map[string]any)["prompt"].(map[string]any)
	prompt["603"] = map[string]any{
		"class_type": "solarL_SaveImagesToZip",
		"inputs":     map[string]any{"zip": []any{"522", 0}},
	}
	broken, err := json.Marshal(response)
	require.NoError(t, err)
	require.ErrorContains(t, validateRunningHubAPIFormatResponse(broken), "competing non-video output node 603")
}

func TestValidateRunningHubTextAPIFormatDoesNotRequireImageNodes(t *testing.T) {
	require.NoError(t, validateRunningHubAPIFormatResponseForMode(runningHubH3TextAPIFormatResponse(t), common.RunningHubH3WorkflowTextToVideo))
}

func runningHubH3APIFormatResponse(t *testing.T) []byte {
	t.Helper()
	prompt := map[string]any{
		"138": map[string]any{"inputs": map[string]any{"value": "prompt"}},
		"132": map[string]any{"inputs": map[string]any{"value": "negative"}},
		"115": map[string]any{"inputs": map[string]any{"aspect_ratio": "16:9", "megapixels": "1", "multiple": 1}},
		"92":  map[string]any{"class_type": "SaveVideo", "inputs": map[string]any{"video": []any{"130", 0}}},
	}
	for _, nodeID := range []string{"137", "618", "617", "619", "627", "626", "625", "624", "623"} {
		prompt[nodeID] = map[string]any{"inputs": map[string]any{"image": ""}}
	}
	for _, nodeID := range []string{"628", "630", "629"} {
		prompt[nodeID] = map[string]any{"inputs": map[string]any{"audio": ""}}
	}
	body, err := json.Marshal(map[string]any{
		"code": 0,
		"data": map[string]any{"prompt": prompt},
	})
	require.NoError(t, err)
	return body
}

func runningHubH3TextAPIFormatResponse(t *testing.T) []byte {
	t.Helper()
	prompt := map[string]any{
		"138": map[string]any{"inputs": map[string]any{"value": "prompt"}},
		"132": map[string]any{"inputs": map[string]any{"value": 5}},
		"115": map[string]any{"inputs": map[string]any{"aspect_ratio": "16:9", "megapixels": 1, "multiple": 32}},
		"92":  map[string]any{"class_type": "SaveVideo", "inputs": map[string]any{"video": []any{"130", 0}}},
	}
	for _, nodeID := range []string{"628", "630", "629"} {
		prompt[nodeID] = map[string]any{"inputs": map[string]any{"audio": ""}}
	}
	body, err := json.Marshal(map[string]any{"code": 0, "data": map[string]any{"prompt": prompt}})
	require.NoError(t, err)
	return body
}
