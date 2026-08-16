package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel/task/comfyuih3"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/require"
)

func TestComfyUIH3ChannelRegistrationAndCapabilityCheck(t *testing.T) {
	apiType, ok := common.ChannelType2APIType(constant.ChannelTypeComfyUIH3)
	require.True(t, ok)
	require.Equal(t, constant.APITypeOpenAI, apiType)
	require.Equal(t, []constant.EndpointType{constant.EndpointTypeOpenAIVideo}, common.GetEndpointTypesByChannelType(constant.ChannelTypeComfyUIH3, "minimax_h3"))
	require.Equal(t, "ComfyUI H3", constant.GetChannelTypeName(constant.ChannelTypeComfyUIH3))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/system_stats":
			_, _ = w.Write([]byte(`{"system": {"os": "linux"}}`))
		case strings.HasPrefix(r.URL.Path, "/object_info/"):
			nodeType := strings.TrimPrefix(r.URL.Path, "/object_info/")
			_, _ = w.Write(comfyUIObjectInfoJSON(t, nodeType, "", ""))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	baseURL := server.URL
	channel := &model.Channel{Type: constant.ChannelTypeComfyUIH3, BaseURL: &baseURL}

	testResult := testComfyUIH3Channel(context.Background(), channel)
	require.NoError(t, testResult.localErr)
	models, err := fetchComfyUIH3UpstreamModelIDs(channel, server.URL)
	require.NoError(t, err)
	require.Equal(t, []string{"minimax_h3"}, models)
}

func TestComfyUIH3GatewayChannelCheckUsesBearerAndAcceptsGatewayTaskNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/gateway/tasks/__newapi_channel_probe__", r.URL.Path)
		require.Equal(t, "Bearer gateway-secret", r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"detail":"dispatch task not found"}`))
	}))
	defer server.Close()

	baseURL := "http://direct-worker.invalid:5900"
	channel := &model.Channel{Type: constant.ChannelTypeComfyUIH3, BaseURL: &baseURL, Key: "gateway-secret"}
	channel.SetOtherSettings(dto.ChannelOtherSettings{ComfyUIH3GatewayURL: server.URL})
	require.NoError(t, validateComfyUIH3ChannelWorkers(context.Background(), channel))
}

func TestComfyUIH3GatewayChannelCheckRejectsGenericNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`not the H3 gateway`))
	}))
	defer server.Close()

	baseURL := "http://direct-worker.invalid:5900"
	channel := &model.Channel{Type: constant.ChannelTypeComfyUIH3, BaseURL: &baseURL, Key: "gateway-secret"}
	channel.SetOtherSettings(dto.ChannelOtherSettings{ComfyUIH3GatewayURL: server.URL})
	err := validateComfyUIH3ChannelWorkers(context.Background(), channel)
	require.ErrorContains(t, err, "404")
}

func TestComfyUIH3GatewayChannelCheckRejectsUnauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"detail":"invalid token"}`))
	}))
	defer server.Close()

	baseURL := "http://direct-worker.invalid:5900"
	channel := &model.Channel{Type: constant.ChannelTypeComfyUIH3, BaseURL: &baseURL, Key: "bad-secret"}
	channel.SetOtherSettings(dto.ChannelOtherSettings{ComfyUIH3GatewayURL: server.URL})
	err := validateComfyUIH3ChannelWorkers(context.Background(), channel)
	require.ErrorContains(t, err, "authentication failed")
	require.ErrorContains(t, err, "401")
}

func TestComfyUIH3ChannelCapabilityCheckRejectsMissingAssetsAndNodeDefinition(t *testing.T) {
	for _, testCase := range []struct {
		name             string
		missingNodeType  string
		missingInputName string
		missingValue     string
		body             string
	}{
		{name: "missing H3 model asset", missingNodeType: "UNETLoader", missingInputName: "unet_name", missingValue: "minimax_h3_fl2va_pruned_int8_convrot.safetensors"},
		{name: "unexpected object info node", missingNodeType: "UNETLoader", body: `{"different_node": {"input": {"required": {}}}}`},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.URL.Path == "/system_stats":
					_, _ = w.Write([]byte(`{"system": {"os": "linux"}}`))
				case strings.HasPrefix(r.URL.Path, "/object_info/"):
					nodeType := strings.TrimPrefix(r.URL.Path, "/object_info/")
					if nodeType == testCase.missingNodeType && testCase.body != "" {
						_, _ = w.Write([]byte(testCase.body))
						return
					}
					_, _ = w.Write(comfyUIObjectInfoJSON(t, nodeType, testCase.missingInputName, testCase.missingValue))
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()

			baseURL := server.URL
			channel := &model.Channel{Type: constant.ChannelTypeComfyUIH3, BaseURL: &baseURL}
			result := testComfyUIH3Channel(context.Background(), channel)
			require.Error(t, result.localErr)
		})
	}
}

func comfyUIObjectInfoJSON(t *testing.T, nodeType string, omitInputName string, omitValue string) []byte {
	t.Helper()
	choices, err := comfyuih3.RequiredNodeChoices()
	require.NoError(t, err)
	required := make(map[string]any)
	for _, choice := range choices {
		if choice.NodeType != nodeType || (choice.InputName == omitInputName && choice.Value == omitValue) {
			continue
		}
		current, _ := required[choice.InputName].([]any)
		if len(current) == 0 {
			current = []any{[]any{}}
		}
		options := current[0].([]any)
		current[0] = append(options, choice.Value)
		required[choice.InputName] = current
	}
	payload := map[string]any{
		nodeType: map[string]any{
			"input": map[string]any{"required": required},
		},
	}
	body, err := common.Marshal(payload)
	require.NoError(t, err)
	return body
}
