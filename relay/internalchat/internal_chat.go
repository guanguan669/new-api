package internalchat

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/factory"
	openaichannel "github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/gin-gonic/gin"
)

var getChannelByID = model.GetChannelById
var shouldUseResponsesForInternalChat = service.ShouldChatCompletionsUseResponsesGlobal

// SupportsChannel reports whether a channel can serve an internal OpenAI chat
// completion. Task/video-only and Responses-only channels are excluded.
func SupportsChannel(channel *model.Channel) bool {
	if channel == nil {
		return false
	}
	if channel.Type == constant.ChannelTypeAdvancedCustom {
		config := channel.GetOtherSettings().AdvancedCustom
		if config == nil || !config.SupportsPath("/v1/chat/completions") {
			return false
		}
	}
	apiType, ok := internalChatAPIType(channel.Type)
	if !ok || apiType == constant.APITypeCodex {
		return false
	}
	if factory.GetAdaptor(apiType) == nil {
		return false
	}
	return slices.Contains(common.GetEndpointTypesByChannelType(channel.Type, ""), constant.EndpointTypeOpenAI)
}

func internalChatAPIType(channelType int) (int, bool) {
	apiType, ok := common.ChannelType2APIType(channelType)
	if ok {
		return apiType, true
	}
	switch channelType {
	case constant.ChannelTypeAzure,
		constant.ChannelTypeOpenAIMax,
		constant.ChannelTypeOhMyGPT,
		constant.ChannelTypeCustom,
		constant.ChannelTypeAILS,
		constant.ChannelTypeAIProxy,
		constant.ChannelTypeAPI2GPT,
		constant.ChannelTypeAIGC2D,
		constant.ChannelType360,
		constant.ChannelTypeFastGPT,
		constant.ChannelTypeLingYiWanWu:
		return constant.APITypeOpenAI, true
	default:
		return constant.APITypeOpenAI, false
	}
}

// ValidateChannel verifies that the exact channel is enabled, supports chat
// completions, and explicitly contains the requested model.
func ValidateChannel(channel *model.Channel, modelName string) error {
	if channel == nil {
		return errors.New("LLM channel does not exist")
	}
	if channel.Status != common.ChannelStatusEnabled {
		return fmt.Errorf("LLM channel %d is disabled", channel.Id)
	}
	if !SupportsChannel(channel) {
		return fmt.Errorf("channel %d does not support OpenAI chat completions", channel.Id)
	}
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		return errors.New("LLM model is required")
	}
	modelConfigured := false
	for _, configured := range channel.GetModels() {
		if strings.TrimSpace(configured) == modelName {
			modelConfigured = true
			break
		}
	}
	if !modelConfigured {
		return fmt.Errorf("model %q is not configured on channel %d", modelName, channel.Id)
	}
	if channel.Type == constant.ChannelTypeAdvancedCustom {
		config := channel.GetOtherSettings().AdvancedCustom
		if config == nil || !config.SupportsPathForModel("/v1/chat/completions", modelName) {
			return fmt.Errorf("advanced custom channel %d does not configure /v1/chat/completions for model %q", channel.Id, modelName)
		}
	}
	return nil
}

// Execute sends one non-streaming chat completion through an exact configured
// channel. It bypasses channel distribution, public-token authorization,
// billing, consume logs, and task persistence.
func Execute(ctx context.Context, channelID int, request *dto.GeneralOpenAIRequest) (*dto.OpenAITextResponse, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if request == nil {
		return nil, errors.New("LLM request is nil")
	}
	channel, err := getChannelByID(channelID, true)
	if err != nil {
		return nil, fmt.Errorf("load LLM channel %d: %w", channelID, err)
	}
	if err := ValidateChannel(channel, request.Model); err != nil {
		return nil, err
	}

	reqCopy, err := common.DeepCopy(request)
	if err != nil {
		return nil, fmt.Errorf("copy LLM request: %w", err)
	}
	stream := false
	reqCopy.Stream = &stream
	reqCopy.StreamOptions = nil

	body, err := common.Marshal(reqCopy)
	if err != nil {
		return nil, fmt.Errorf("marshal LLM request: %w", err)
	}
	httpReq := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body)).WithContext(ctx)
	httpReq.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httpReq
	common.SetContextKey(c, common.RequestIdKey, common.NewRequestId())

	if setupErr := middleware.SetupContextForSelectedChannel(c, channel, reqCopy.Model); setupErr != nil {
		return nil, fmt.Errorf("initialize LLM channel %d: %w", channelID, setupErr)
	}

	info, err := relaycommon.GenRelayInfo(c, types.RelayFormatOpenAI, reqCopy, nil)
	if err != nil {
		return nil, fmt.Errorf("initialize LLM relay: %w", err)
	}
	info.RelayMode = relayconstant.RelayModeChatCompletions
	info.RequestURLPath = "/v1/chat/completions"
	info.IsStream = false
	info.StartTime = time.Now()
	info.InitChannelMeta(c)
	apiType, ok := internalChatAPIType(channel.Type)
	if !ok {
		return nil, fmt.Errorf("unsupported LLM channel type %d", channel.Type)
	}
	info.ApiType = apiType

	if err := helper.ModelMappedHelper(c, info, reqCopy); err != nil {
		return nil, fmt.Errorf("map LLM model: %w", err)
	}
	adaptor := factory.GetAdaptor(info.ApiType)
	if adaptor == nil {
		return nil, fmt.Errorf("unsupported LLM API type %d", info.ApiType)
	}
	adaptor.Init(info)
	if shouldUseResponses(c, info) {
		return executeViaResponses(c, info, adaptor, reqCopy, recorder)
	}

	converted, err := adaptor.ConvertOpenAIRequest(c, info, reqCopy)
	if err != nil {
		return nil, fmt.Errorf("convert LLM request: %w", err)
	}
	relaycommon.AppendRequestConversionFromRequest(info, converted)
	jsonData, err := common.Marshal(converted)
	if err != nil {
		return nil, fmt.Errorf("marshal converted LLM request: %w", err)
	}
	jsonData, err = relaycommon.RemoveDisabledFields(jsonData, info.ChannelOtherSettings, false)
	if err != nil {
		return nil, fmt.Errorf("filter LLM request: %w", err)
	}
	if len(info.ParamOverride) > 0 {
		jsonData, err = relaycommon.ApplyParamOverrideWithRelayInfo(jsonData, info)
		if err != nil {
			return nil, fmt.Errorf("apply LLM channel parameter override: %w", err)
		}
	}
	outbound, size, closer, err := relaycommon.NewOutboundJSONBody(jsonData)
	if err != nil {
		return nil, fmt.Errorf("build LLM request body: %w", err)
	}
	defer closer.Close()
	info.UpstreamRequestBodySize = size

	response, err := adaptor.DoRequest(c, info, outbound)
	if err != nil {
		return nil, fmt.Errorf("send LLM request: %w", err)
	}
	httpResponse, ok := response.(*http.Response)
	if !ok || httpResponse == nil {
		return nil, fmt.Errorf("LLM channel %d returned unsupported response type %T", channelID, response)
	}
	if httpResponse.StatusCode != http.StatusOK {
		return nil, service.RelayErrorHandler(ctx, httpResponse, false)
	}
	if _, relayErr := adaptor.DoResponse(c, httpResponse, info); relayErr != nil {
		return nil, relayErr
	}
	return internalChatResult(recorder)
}

func shouldUseResponses(c *gin.Context, info *relaycommon.RelayInfo) bool {
	if c == nil || info == nil {
		return false
	}
	return !model_setting.GetGlobalSettings().PassThroughRequestEnabled &&
		!info.ChannelSetting.PassThroughBodyEnabled &&
		shouldUseResponsesForInternalChat(
			info.ChannelId,
			info.ChannelType,
			info.OriginModelName,
		)
}

// executeViaResponses mirrors the normal Chat Completions -> Responses bridge
// while keeping the internal caller's return type as an OpenAI chat response.
// This matters for a NewAPI channel that has the global Responses routing rule
// enabled: the channel remains selectable and uses its configured endpoint.
func executeViaResponses(
	c *gin.Context,
	info *relaycommon.RelayInfo,
	adaptor channel.Adaptor,
	request *dto.GeneralOpenAIRequest,
	recorder *httptest.ResponseRecorder,
) (*dto.OpenAITextResponse, error) {
	chatJSON, err := common.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("marshal LLM chat request: %w", err)
	}
	chatJSON, err = relaycommon.RemoveDisabledFields(chatJSON, info.ChannelOtherSettings, info.ChannelSetting.PassThroughBodyEnabled)
	if err != nil {
		return nil, fmt.Errorf("filter LLM chat request: %w", err)
	}
	if len(info.ParamOverride) > 0 {
		chatJSON, err = relaycommon.ApplyParamOverrideWithRelayInfo(chatJSON, info)
		if err != nil {
			return nil, fmt.Errorf("apply LLM channel parameter override: %w", err)
		}
	}

	var overriddenChatRequest dto.GeneralOpenAIRequest
	if err := common.Unmarshal(chatJSON, &overriddenChatRequest); err != nil {
		return nil, fmt.Errorf("decode overridden LLM chat request: %w", err)
	}
	conversion, err := service.ConvertRequestVia(c, info, &overriddenChatRequest, types.RelayFormatOpenAI, types.RelayFormatOpenAIResponses)
	if err != nil {
		return nil, fmt.Errorf("convert LLM chat request to Responses: %w", err)
	}
	responsesRequest, ok := conversion.Value.(*dto.OpenAIResponsesRequest)
	if !ok {
		return nil, fmt.Errorf("expected OpenAI Responses request, got %T", conversion.Value)
	}

	info.RelayMode = relayconstant.RelayModeResponses
	info.RequestURLPath = "/v1/responses"
	converted, err := adaptor.ConvertOpenAIResponsesRequest(c, info, *responsesRequest)
	if err != nil {
		return nil, fmt.Errorf("convert LLM Responses request: %w", err)
	}
	relaycommon.AppendRequestConversionFromRequest(info, converted)
	jsonData, err := common.Marshal(converted)
	if err != nil {
		return nil, fmt.Errorf("marshal converted LLM Responses request: %w", err)
	}
	jsonData, err = relaycommon.RemoveDisabledFields(jsonData, info.ChannelOtherSettings, info.ChannelSetting.PassThroughBodyEnabled)
	if err != nil {
		return nil, fmt.Errorf("filter LLM Responses request: %w", err)
	}
	outbound, size, closer, err := relaycommon.NewOutboundJSONBody(jsonData)
	if err != nil {
		return nil, fmt.Errorf("build LLM Responses request body: %w", err)
	}
	defer closer.Close()
	info.UpstreamRequestBodySize = size

	response, err := adaptor.DoRequest(c, info, outbound)
	if err != nil {
		return nil, fmt.Errorf("send LLM Responses request: %w", err)
	}
	httpResponse, ok := response.(*http.Response)
	if !ok || httpResponse == nil {
		return nil, fmt.Errorf("LLM channel %d returned unsupported response type %T", info.ChannelId, response)
	}
	if httpResponse.StatusCode != http.StatusOK {
		return nil, service.RelayErrorHandler(c.Request.Context(), httpResponse, false)
	}

	var relayErr *types.NewAPIError
	if strings.Contains(strings.ToLower(httpResponse.Header.Get("Content-Type")), "text/event-stream") {
		info.IsStream = false
		_, relayErr = openaichannel.OaiResponsesToChatBufferedStreamHandler(c, info, httpResponse)
	} else {
		_, relayErr = openaichannel.OaiResponsesToChatHandler(c, info, httpResponse)
	}
	if relayErr != nil {
		return nil, relayErr
	}
	return internalChatResult(recorder)
}

func internalChatResult(recorder *httptest.ResponseRecorder) (*dto.OpenAITextResponse, error) {
	if recorder == nil {
		return nil, errors.New("LLM response recorder is nil")
	}
	if recorder.Code != http.StatusOK {
		return nil, fmt.Errorf("convert LLM response: status %d", recorder.Code)
	}
	var result dto.OpenAITextResponse
	if err := common.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		return nil, fmt.Errorf("decode LLM response: %w", err)
	}
	if len(result.Choices) == 0 {
		return nil, errors.New("LLM response contains no choices")
	}
	return &result, nil
}
