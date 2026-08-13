package controller

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/console_setting"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/gin-gonic/gin"
)

var completionRatioMetaOptionKeys = []string{
	"ModelPrice",
	"ModelRatio",
	"CompletionRatio",
	"CacheRatio",
	"CreateCacheRatio",
	"ImageRatio",
	"AudioRatio",
	"AudioCompletionRatio",
}

func isPaymentComplianceOptionKey(key string) bool {
	return strings.HasPrefix(key, "payment_setting.compliance_")
}

func isPositiveOptionValue(value string) bool {
	intValue, err := strconv.Atoi(strings.TrimSpace(value))
	if err == nil {
		return intValue > 0
	}
	floatValue, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	return err == nil && floatValue > 0
}

func collectModelNamesFromOptionValue(raw string, modelNames map[string]struct{}) {
	if strings.TrimSpace(raw) == "" {
		return
	}

	var parsed map[string]any
	if err := common.UnmarshalJsonStr(raw, &parsed); err != nil {
		return
	}

	for modelName := range parsed {
		modelNames[modelName] = struct{}{}
	}
}

func buildCompletionRatioMetaValue(optionValues map[string]string) string {
	modelNames := make(map[string]struct{})
	for _, key := range completionRatioMetaOptionKeys {
		collectModelNamesFromOptionValue(optionValues[key], modelNames)
	}

	meta := make(map[string]ratio_setting.CompletionRatioInfo, len(modelNames))
	for modelName := range modelNames {
		meta[modelName] = ratio_setting.GetCompletionRatioInfo(modelName)
	}

	jsonBytes, err := common.Marshal(meta)
	if err != nil {
		return "{}"
	}
	return string(jsonBytes)
}

func GetOptions(c *gin.Context) {
	var options []*model.Option
	optionValues := make(map[string]string)
	common.OptionMapRWMutex.Lock()
	for k, v := range common.OptionMap {
		if k == "theme.frontend" {
			continue
		}
		value := common.Interface2String(v)
		isSensitiveKey := strings.HasSuffix(k, "Token") ||
			strings.HasSuffix(k, "Secret") ||
			strings.HasSuffix(k, "Key") ||
			strings.HasSuffix(k, "secret") ||
			strings.HasSuffix(k, "api_key")
		if isSensitiveKey {
			continue
		}
		options = append(options, &model.Option{
			Key:   k,
			Value: value,
		})
		for _, optionKey := range completionRatioMetaOptionKeys {
			if optionKey == k {
				optionValues[k] = value
				break
			}
		}
	}
	common.OptionMapRWMutex.Unlock()
	options = append(options, &model.Option{
		Key:   "CompletionRatioMeta",
		Value: buildCompletionRatioMetaValue(optionValues),
	})
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    options,
	})
}

type OptionUpdateRequest struct {
	Key   string `json:"key"`
	Value any    `json:"value"`
}

type ComfyUIH3PromptEnhancerUpdateRequest struct {
	Enabled        bool   `json:"enabled"`
	ProviderMode   string `json:"provider_mode"`
	ChannelID      int    `json:"channel_id"`
	BaseURL        string `json:"base_url"`
	APIKey         string `json:"api_key"`
	ClearAPIKey    bool   `json:"clear_api_key"`
	Model          string `json:"model"`
	TimeoutSeconds int    `json:"timeout_seconds"`
	SystemPrompt   string `json:"system_prompt"`
}

func UpdateComfyUIH3PromptEnhancer(c *gin.Context) {
	var request ComfyUIH3PromptEnhancerUpdateRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorMsg(c, "无效的参数")
		return
	}

	request.BaseURL = normalizeComfyUIH3PromptEnhancerBaseURL(request.BaseURL)
	providerMode, err := normalizeComfyUIH3PromptEnhancerProviderMode(request.ProviderMode)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	request.ProviderMode = providerMode
	request.APIKey = strings.TrimSpace(request.APIKey)
	request.Model = strings.TrimSpace(request.Model)
	request.SystemPrompt = strings.TrimSpace(request.SystemPrompt)
	current := model_setting.GetComfyUIH3PromptEnhancerSettings()
	if request.ProviderMode == model_setting.ComfyUIH3PromptEnhancerProviderChannel {
		// Direct credentials are dormant in channel mode. Keep the existing
		// values so switching modes does not silently destroy that configuration,
		// and ignore hidden/stale direct fields submitted by the form.
		request.BaseURL = current.BaseURL
		request.APIKey = ""
		request.ClearAPIKey = false
	}
	if request.ProviderMode == model_setting.ComfyUIH3PromptEnhancerProviderDirect && request.ClearAPIKey && request.APIKey != "" {
		common.ApiErrorMsg(c, "清除 API Key 时不能同时提供新的 API Key")
		return
	}
	validations := []struct {
		key   string
		value string
	}{
		{"comfyui_h3_prompt_enhancer.timeout_seconds", strconv.Itoa(request.TimeoutSeconds)},
	}
	if request.ProviderMode == model_setting.ComfyUIH3PromptEnhancerProviderDirect {
		validations = append(validations, struct {
			key   string
			value string
		}{"comfyui_h3_prompt_enhancer.base_url", request.BaseURL})
	}
	for _, validation := range validations {
		key, value := validation.key, validation.value
		if err := validateComfyUIH3PromptEnhancerOption(key, value); err != nil {
			common.ApiErrorMsg(c, err.Error())
			return
		}
	}
	if request.Enabled {
		if request.Model == "" {
			common.ApiErrorMsg(c, "请先填写提示词增强模型")
			return
		}
		if request.SystemPrompt == "" {
			common.ApiErrorMsg(c, "请先填写 H3 Context-IR 系统提示词")
			return
		}
		switch request.ProviderMode {
		case model_setting.ComfyUIH3PromptEnhancerProviderChannel:
			if err := validateComfyUIH3PromptEnhancerChannel(request.ChannelID, request.Model); err != nil {
				common.ApiErrorMsg(c, err.Error())
				return
			}
		case model_setting.ComfyUIH3PromptEnhancerProviderDirect:
			if request.BaseURL == "" {
				common.ApiErrorMsg(c, "请先填写提示词增强接口地址")
				return
			}
		}
	}

	apiKey := current.APIKey
	if request.ProviderMode == model_setting.ComfyUIH3PromptEnhancerProviderDirect {
		apiKey = resolveComfyUIH3PromptEnhancerAPIKey(request.BaseURL, request.APIKey, request.ClearAPIKey, current)
	}
	settings := model_setting.ComfyUIH3PromptEnhancerSettings{
		Enabled:        request.Enabled,
		ProviderMode:   request.ProviderMode,
		ChannelID:      request.ChannelID,
		BaseURL:        request.BaseURL,
		APIKey:         apiKey,
		Model:          request.Model,
		TimeoutSeconds: request.TimeoutSeconds,
		SystemPrompt:   request.SystemPrompt,
	}
	if err := model.UpdateComfyUIH3PromptEnhancerSettings(settings); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "option.update", map[string]interface{}{
		"key": "comfyui_h3_prompt_enhancer",
	})
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

func normalizeComfyUIH3PromptEnhancerProviderMode(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", model_setting.ComfyUIH3PromptEnhancerProviderDirect, "external":
		return model_setting.ComfyUIH3PromptEnhancerProviderDirect, nil
	case model_setting.ComfyUIH3PromptEnhancerProviderChannel:
		return model_setting.ComfyUIH3PromptEnhancerProviderChannel, nil
	default:
		return "", fmt.Errorf("提示词增强来源必须是渠道或直连")
	}
}

func normalizeComfyUIH3PromptEnhancerBaseURL(value string) string {
	return strings.TrimRight(strings.TrimSpace(value), "/")
}

func resolveComfyUIH3PromptEnhancerAPIKey(baseURL, suppliedAPIKey string, clearAPIKey bool, current model_setting.ComfyUIH3PromptEnhancerSettings) string {
	apiKey := strings.TrimSpace(suppliedAPIKey)
	if apiKey != "" {
		return apiKey
	}
	if clearAPIKey {
		return ""
	}
	if normalizeComfyUIH3PromptEnhancerBaseURL(baseURL) == normalizeComfyUIH3PromptEnhancerBaseURL(current.BaseURL) {
		return current.APIKey
	}
	return ""
}

func UpdateOption(c *gin.Context) {
	var option OptionUpdateRequest
	err := common.DecodeJson(c.Request.Body, &option)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的参数",
		})
		return
	}
	switch option.Value.(type) {
	case bool:
		option.Value = common.Interface2String(option.Value.(bool))
	case float64:
		option.Value = common.Interface2String(option.Value.(float64))
	case int:
		option.Value = common.Interface2String(option.Value.(int))
	default:
		option.Value = fmt.Sprintf("%v", option.Value)
	}
	switch option.Key {
	case "QuotaForInviter", "QuotaForInvitee":
		if isPositiveOptionValue(option.Value.(string)) && !operation_setting.IsPaymentComplianceConfirmed() {
			common.ApiErrorI18n(c, i18n.MsgPaymentComplianceRequired)
			return
		}
	default:
		if isPaymentComplianceOptionKey(option.Key) {
			common.ApiErrorMsg(c, "合规确认字段不允许通过通用设置接口修改")
			return
		}
	}
	if strings.HasPrefix(option.Key, "comfyui_h3_prompt_enhancer.") {
		common.ApiErrorMsg(c, "自部署 H3 提示词增强配置必须通过专用设置接口整体保存")
		return
	}
	optionValue, ok := option.Value.(string)
	if !ok {
		optionValue = fmt.Sprintf("%v", option.Value)
	}
	if err := validateComfyUIH3PromptEnhancerOption(option.Key, optionValue); err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	switch option.Key {
	case "GitHubOAuthEnabled":
		if option.Value == "true" && common.GitHubClientId == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用 GitHub OAuth，请先填入 GitHub Client Id 以及 GitHub Client Secret！",
			})
			return
		}
	case "discord.enabled":
		if option.Value == "true" && system_setting.GetDiscordSettings().ClientId == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用 Discord OAuth，请先填入 Discord Client Id 以及 Discord Client Secret！",
			})
			return
		}
	case "oidc.enabled":
		if option.Value == "true" && system_setting.GetOIDCSettings().ClientId == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用 OIDC 登录，请先填入 OIDC Client Id 以及 OIDC Client Secret！",
			})
			return
		}
	case "LinuxDOOAuthEnabled":
		if option.Value == "true" && common.LinuxDOClientId == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用 LinuxDO OAuth，请先填入 LinuxDO Client Id 以及 LinuxDO Client Secret！",
			})
			return
		}
	case "EmailDomainRestrictionEnabled":
		if option.Value == "true" && len(common.EmailDomainWhitelist) == 0 {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用邮箱域名限制，请先填入限制的邮箱域名！",
			})
			return
		}
	case "WeChatAuthEnabled":
		if option.Value == "true" && common.WeChatServerAddress == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用微信登录，请先填入微信登录相关配置信息！",
			})
			return
		}
	case "TurnstileCheckEnabled":
		if option.Value == "true" && common.TurnstileSiteKey == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用 Turnstile 校验，请先填入 Turnstile 校验相关配置信息！",
			})

			return
		}
	case "TelegramOAuthEnabled":
		if option.Value == "true" && common.TelegramBotToken == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用 Telegram OAuth，请先填入 Telegram Bot Token！",
			})
			return
		}
	case "theme.frontend":
		if option.Value != "default" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "Classic 前端已移除，主题只能设置为 default",
			})
			return
		}
	case "GroupRatio":
		err = ratio_setting.CheckGroupRatio(option.Value.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	case ratio_setting.RunningHubH3GroupPriceOptionKey:
		err = ratio_setting.ValidateRunningHubH3GroupPriceJSON(option.Value.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	case "gemini.safety_settings":
		err = model_setting.ValidateGeminiSafetySettings(option.Value.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	case "claude.default_max_tokens":
		err = model_setting.ValidateClaudeDefaultMaxTokens(option.Value.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	case operation_setting.ToolPriceOptionKey:
		err = operation_setting.ValidateToolPricesJSON(option.Value.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	case "ImageRatio":
		err = ratio_setting.UpdateImageRatioByJSONString(option.Value.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "图片倍率设置失败: " + err.Error(),
			})
			return
		}
	case "AudioRatio":
		err = ratio_setting.UpdateAudioRatioByJSONString(option.Value.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "音频倍率设置失败: " + err.Error(),
			})
			return
		}
	case "AudioCompletionRatio":
		err = ratio_setting.UpdateAudioCompletionRatioByJSONString(option.Value.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "音频补全倍率设置失败: " + err.Error(),
			})
			return
		}
	case "CreateCacheRatio":
		err = ratio_setting.UpdateCreateCacheRatioByJSONString(option.Value.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "缓存创建倍率设置失败: " + err.Error(),
			})
			return
		}
	case "ModelRequestRateLimitGroup":
		err = setting.CheckModelRequestRateLimitGroup(option.Value.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	case "AutomaticDisableStatusCodes":
		_, err = operation_setting.ParseHTTPStatusCodeRanges(option.Value.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	case "AutomaticRetryStatusCodes":
		_, err = operation_setting.ParseHTTPStatusCodeRanges(option.Value.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	case "console_setting.api_info":
		err = console_setting.ValidateConsoleSettings(option.Value.(string), "ApiInfo")
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	case "console_setting.announcements":
		err = console_setting.ValidateConsoleSettings(option.Value.(string), "Announcements")
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	case "console_setting.faq":
		err = console_setting.ValidateConsoleSettings(option.Value.(string), "FAQ")
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	case "console_setting.uptime_kuma_groups":
		err = console_setting.ValidateConsoleSettings(option.Value.(string), "UptimeKumaGroups")
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	}
	err = model.UpdateOption(option.Key, option.Value.(string))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	// 出于安全考虑只记录被修改的配置项名称，不记录配置值（可能含密钥等敏感信息）。
	recordManageAudit(c, "option.update", map[string]interface{}{
		"key": option.Key,
	})
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

func validateComfyUIH3PromptEnhancerOption(key, value string) error {
	switch key {
	case "comfyui_h3_prompt_enhancer.base_url":
		if strings.TrimSpace(value) == "" {
			return nil
		}
		parsed, err := url.Parse(strings.TrimSpace(value))
		if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.RawQuery != "" || parsed.Fragment != "" {
			return fmt.Errorf("提示词增强接口地址必须是有效的 HTTP 或 HTTPS URL")
		}
	case "comfyui_h3_prompt_enhancer.timeout_seconds":
		timeout, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil || timeout < 1 || timeout > 300 {
			return fmt.Errorf("提示词增强超时时间必须在 1 到 300 秒之间")
		}
	case "comfyui_h3_prompt_enhancer.enabled":
		enabled, err := strconv.ParseBool(strings.TrimSpace(value))
		if err != nil {
			return fmt.Errorf("提示词增强启用状态必须是布尔值")
		}
		if !enabled {
			return nil
		}
		settings := model_setting.GetComfyUIH3PromptEnhancerSettings()
		if strings.TrimSpace(settings.Model) == "" {
			return fmt.Errorf("请先填写提示词增强模型")
		}
		if strings.TrimSpace(settings.SystemPrompt) == "" {
			return fmt.Errorf("请先填写 H3 Context-IR 系统提示词")
		}
		switch model_setting.NormalizeComfyUIH3PromptEnhancerProviderMode(settings.ProviderMode) {
		case model_setting.ComfyUIH3PromptEnhancerProviderChannel:
			return validateComfyUIH3PromptEnhancerChannel(settings.ChannelID, settings.Model)
		case model_setting.ComfyUIH3PromptEnhancerProviderDirect:
			if strings.TrimSpace(settings.BaseURL) == "" {
				return fmt.Errorf("请先填写提示词增强接口地址")
			}
		}
	}
	return nil
}
