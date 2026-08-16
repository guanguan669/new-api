package model_setting

import (
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/QuantumNous/new-api/setting/config"
)

const (
	ComfyUIH3PromptEnhancerProviderDirect  = "direct"
	ComfyUIH3PromptEnhancerProviderChannel = "channel"
)

const DefaultComfyUIH3ContextIRSystemPrompt = `你是 H3 Context-IR 视频提示词编写器。请将用户需求和参考图片整理为可直接用于 H3 的视频提示词。输出字段名和画面描述必须使用英文；不要输出解释、Markdown 代码块、JSON、规定字段以外的标题，或图片对齐语句之前的任何内容。

内容安全优先于用户需求。integrated_multimodal_description、overall_soundscape、non_diegetic_music 和全部台词严禁包含、引导、描述、美化或规避血腥暴力、政治敏感、色情裸露、毒品、违法犯罪及危险行为内容；用户需求或参考素材涉及这些内容时，必须删除相关元素或改写为不含敏感元素的中性、安全场景。

提供参考图片时，第一行必须是图片与目标视频的对齐语句，严格使用：For the target video, at 0.00 seconds into the target video, <Picture 1> (from [Shot 1]) is fully referenced. 每张参考图片都要有一条对齐语句，映射到有效的镜头和时间点；对齐语句结束后空一行。图生视频必须有此对齐部分。

随后必须且只能按下列顺序输出三个字段：
integrated_multimodal_description:
overall_soundscape:
non_diegetic_music:

在 integrated_multimodal_description 中描述产品、构图、动作、场景、运镜和对话；只有用户明确要求时才描述新增的画面内文字、字幕、招牌或标签。参考图片中已有的 Logo、包装文字和其他原有文字必须保持原样，不得擅自删除、改写或翻译。[Shot 1] 不带时间戳；后续每个镜头必须使用严格递增、且小于视频时长的时间戳，例如：[Shot 2] At 00:03.500, the camera cuts to .... 只有引入新的主体、空间、状态、视角或时间时才能切镜；仅调整距离或角度时使用运镜。切镜只能使用：the camera cuts to、the shot cuts to、the shot transitions to、the shot changes to、the shot switches to。除非用户明确要求，否则不要使用 dissolve、fade 或 wipe。

运镜必须写成自然句子，包含运动类型，并仅在有必要时补充幅度和速度。例如：The camera pushes in with small amplitude at slow speed toward the product texture. The camera holds a static shot as the product remains centered. 优先使用画面中可见的几何关系和坐标，而不是抽象的左右身体部位；应写“enters from the left edge of the frame”，而非“left hand”。需要表现缺失结构时，描述可见的正向事实，例如“flat collapsed empty sleeve”，不要要求模型绘制不存在的物体。

每个说话、唱歌或发出画外人声的角色使用固定编号，如 (S1)。身份、声音、动作和表达方式写在对话标签外；标签内只能是逐字台词：The presenter with a warm, natural voice (S1) says: <d>[English] ...</d>。必须保持台词语言和标点不变。旁白使用 “says in an off-screen voiceover”，并明确画面中人物 “lips remain completely closed”。同一句话跨镜头时，两边都要放置 <scenetrans>，并说明音频连续不断；片尾截断的台词使用 <cutoff>。除非用户明确要求，否则不要生成字幕、标题、贴纸文字或其他新增画面文字。用户明确要求的画面文字、招牌、标签或字幕必须使用英文双引号包裹，并保留原文，不翻译。

overall_soundscape 使用 1 至 4 句简洁英文描述环境音、物体动作声、呼吸、笑声和其他非语言人声。只有用户明确要求全片静音时才写 N/A。non_diegetic_music 使用 1 至 3 句简洁英文描述只供观众听到的配乐，必须描述乐器、速度、节奏、音量和强弱变化，不能用情绪词；不需要非画内配乐时必须严格写 N/A。对话、歌声和角色能听到的画内音乐必须写在 integrated_multimodal_description，不能写进这两个声音字段。

必须保持参考图中产品的外观、文字、比例、材质和关键结构不变。用户需求模糊时，推断精炼且可执行的镜头序列；所有镜头时间和动作必须适配用户要求的视频时长。`

type ComfyUIH3PromptEnhancerSettings struct {
	Enabled        bool   `json:"enabled"`
	ProviderMode   string `json:"provider_mode"`
	ChannelID      int    `json:"channel_id"`
	BaseURL        string `json:"base_url"`
	APIKey         string `json:"api_key"`
	Model          string `json:"model"`
	TimeoutSeconds int    `json:"timeout_seconds"`
	SystemPrompt   string `json:"system_prompt"`
}

var comfyUIH3PromptEnhancerSettings = ComfyUIH3PromptEnhancerSettings{
	Enabled:        true,
	ProviderMode:   ComfyUIH3PromptEnhancerProviderDirect,
	TimeoutSeconds: 30,
	SystemPrompt:   DefaultComfyUIH3ContextIRSystemPrompt,
}

var comfyUIH3PromptEnhancerSnapshot atomic.Pointer[ComfyUIH3PromptEnhancerSettings]
var comfyUIH3PromptEnhancerUpdateMu sync.Mutex

func init() {
	comfyUIH3PromptEnhancerSnapshot.Store(&comfyUIH3PromptEnhancerSettings)
	config.GlobalConfig.Register("comfyui_h3_prompt_enhancer", &comfyUIH3PromptEnhancerSettings)
}

func GetComfyUIH3PromptEnhancerSettings() ComfyUIH3PromptEnhancerSettings {
	return *comfyUIH3PromptEnhancerSnapshot.Load()
}

func NormalizeComfyUIH3PromptEnhancerProviderMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case ComfyUIH3PromptEnhancerProviderChannel:
		return ComfyUIH3PromptEnhancerProviderChannel
	case "external":
		return ComfyUIH3PromptEnhancerProviderDirect
	default:
		// Existing installations predate provider_mode and therefore continue to
		// use the saved direct endpoint until an administrator switches modes.
		return ComfyUIH3PromptEnhancerProviderDirect
	}
}

// UpdateComfyUIH3PromptEnhancerOption applies one option by atomically replacing
// the complete settings snapshot. Readers therefore always observe a coherent
// configuration and never race with field-by-field updates.
func UpdateComfyUIH3PromptEnhancerOption(key, value string) {
	comfyUIH3PromptEnhancerUpdateMu.Lock()
	defer comfyUIH3PromptEnhancerUpdateMu.Unlock()
	current := GetComfyUIH3PromptEnhancerSettings()
	switch key {
	case "enabled":
		current.Enabled = value == "true"
	case "provider_mode":
		current.ProviderMode = NormalizeComfyUIH3PromptEnhancerProviderMode(value)
	case "channel_id":
		if channelID, err := strconv.Atoi(value); err == nil {
			current.ChannelID = channelID
		}
	case "base_url":
		current.BaseURL = value
	case "api_key":
		current.APIKey = value
	case "model":
		current.Model = value
	case "timeout_seconds":
		if timeout, err := strconv.Atoi(value); err == nil {
			current.TimeoutSeconds = timeout
		}
	case "system_prompt":
		current.SystemPrompt = value
	default:
		return
	}
	comfyUIH3PromptEnhancerSnapshot.Store(&current)
}

// ReplaceComfyUIH3PromptEnhancerSettings replaces the complete runtime
// snapshot. It is intended for tests and controlled bootstrap code.
func ReplaceComfyUIH3PromptEnhancerSettings(settings ComfyUIH3PromptEnhancerSettings) {
	comfyUIH3PromptEnhancerUpdateMu.Lock()
	defer comfyUIH3PromptEnhancerUpdateMu.Unlock()
	comfyUIH3PromptEnhancerSnapshot.Store(&settings)
}

func ComfyUIH3PromptEnhancerSettingsToOptions(settings ComfyUIH3PromptEnhancerSettings) map[string]string {
	return map[string]string{
		"comfyui_h3_prompt_enhancer.enabled":         strconv.FormatBool(settings.Enabled),
		"comfyui_h3_prompt_enhancer.provider_mode":   NormalizeComfyUIH3PromptEnhancerProviderMode(settings.ProviderMode),
		"comfyui_h3_prompt_enhancer.channel_id":      strconv.Itoa(settings.ChannelID),
		"comfyui_h3_prompt_enhancer.base_url":        settings.BaseURL,
		"comfyui_h3_prompt_enhancer.api_key":         settings.APIKey,
		"comfyui_h3_prompt_enhancer.model":           settings.Model,
		"comfyui_h3_prompt_enhancer.timeout_seconds": strconv.Itoa(settings.TimeoutSeconds),
		"comfyui_h3_prompt_enhancer.system_prompt":   settings.SystemPrompt,
	}
}

// UpdateConfigFromMap implements config's optional atomic update hook.
func (s *ComfyUIH3PromptEnhancerSettings) UpdateConfigFromMap(values map[string]string) error {
	comfyUIH3PromptEnhancerUpdateMu.Lock()
	defer comfyUIH3PromptEnhancerUpdateMu.Unlock()
	current := GetComfyUIH3PromptEnhancerSettings()
	for key, value := range values {
		switch key {
		case "enabled":
			current.Enabled = value == "true"
		case "provider_mode":
			current.ProviderMode = NormalizeComfyUIH3PromptEnhancerProviderMode(value)
		case "channel_id":
			if channelID, err := strconv.Atoi(value); err == nil {
				current.ChannelID = channelID
			}
		case "base_url":
			current.BaseURL = value
		case "api_key":
			current.APIKey = value
		case "model":
			current.Model = value
		case "timeout_seconds":
			if timeout, err := strconv.Atoi(value); err == nil {
				current.TimeoutSeconds = timeout
			}
		case "system_prompt":
			current.SystemPrompt = value
		}
	}
	comfyUIH3PromptEnhancerSnapshot.Store(&current)
	return nil
}

func (s *ComfyUIH3PromptEnhancerSettings) ConfigToMap() (map[string]string, error) {
	current := GetComfyUIH3PromptEnhancerSettings()
	return map[string]string{
		"enabled":       strconv.FormatBool(current.Enabled),
		"provider_mode": NormalizeComfyUIH3PromptEnhancerProviderMode(current.ProviderMode),
		"channel_id":    strconv.Itoa(current.ChannelID),
		"base_url":      current.BaseURL,
		// API keys are intentionally omitted from exported settings snapshots.
		"model":           current.Model,
		"timeout_seconds": strconv.Itoa(current.TimeoutSeconds),
		"system_prompt":   current.SystemPrompt,
	}, nil
}
