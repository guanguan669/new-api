package model

import (
	"runtime"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestUpdateComfyUIH3PromptEnhancerSettingsDatabaseFailurePreservesRuntimeState(t *testing.T) {
	previousDB := DB
	previousSettings := model_setting.GetComfyUIH3PromptEnhancerSettings()
	common.OptionMapRWMutex.Lock()
	previousOptionMap := common.OptionMap
	common.OptionMap = map[string]string{
		"comfyui_h3_prompt_enhancer.enabled":         "true",
		"comfyui_h3_prompt_enhancer.base_url":        "https://old.example/v1",
		"comfyui_h3_prompt_enhancer.api_key":         "old-key",
		"comfyui_h3_prompt_enhancer.model":           "old-model",
		"comfyui_h3_prompt_enhancer.timeout_seconds": "8",
		"comfyui_h3_prompt_enhancer.system_prompt":   "old prompt",
	}
	originalMap := cloneStringMap(common.OptionMap)
	common.OptionMapRWMutex.Unlock()

	t.Cleanup(func() {
		DB = previousDB
		model_setting.ReplaceComfyUIH3PromptEnhancerSettings(previousSettings)
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previousOptionMap
		common.OptionMapRWMutex.Unlock()
	})

	model_setting.ReplaceComfyUIH3PromptEnhancerSettings(model_setting.ComfyUIH3PromptEnhancerSettings{
		Enabled:        true,
		BaseURL:        "https://old.example/v1",
		APIKey:         "old-key",
		Model:          "old-model",
		TimeoutSeconds: 8,
		SystemPrompt:   "old prompt",
	})

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Option{}))
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	DB = db

	err = UpdateComfyUIH3PromptEnhancerSettings(model_setting.ComfyUIH3PromptEnhancerSettings{
		Enabled:        false,
		BaseURL:        "https://new.example/v1",
		APIKey:         "new-key",
		Model:          "new-model",
		TimeoutSeconds: 30,
		SystemPrompt:   "new prompt",
	})
	require.Error(t, err)
	require.Equal(t, model_setting.ComfyUIH3PromptEnhancerSettings{
		Enabled:        true,
		BaseURL:        "https://old.example/v1",
		APIKey:         "old-key",
		Model:          "old-model",
		TimeoutSeconds: 8,
		SystemPrompt:   "old prompt",
	}, model_setting.GetComfyUIH3PromptEnhancerSettings())

	common.OptionMapRWMutex.RLock()
	require.Equal(t, originalMap, common.OptionMap)
	common.OptionMapRWMutex.RUnlock()
}

func TestUpdateComfyUIH3PromptEnhancerSettingsCommitsCompleteSnapshot(t *testing.T) {
	previousDB := DB
	previousSettings := model_setting.GetComfyUIH3PromptEnhancerSettings()
	common.OptionMapRWMutex.Lock()
	previousOptionMap := common.OptionMap
	common.OptionMap = make(map[string]string)
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		DB = previousDB
		model_setting.ReplaceComfyUIH3PromptEnhancerSettings(previousSettings)
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previousOptionMap
		common.OptionMapRWMutex.Unlock()
	})

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Option{}))
	DB = db

	want := model_setting.ComfyUIH3PromptEnhancerSettings{
		Enabled:        true,
		BaseURL:        "https://enhancer.example/v1",
		APIKey:         "new-key",
		Model:          "enhancer-model",
		TimeoutSeconds: 20,
		SystemPrompt:   "system prompt",
	}
	wantOptions := model_setting.ComfyUIH3PromptEnhancerSettingsToOptions(want)
	require.NoError(t, UpdateComfyUIH3PromptEnhancerSettings(want))
	require.Equal(t, want, model_setting.GetComfyUIH3PromptEnhancerSettings())

	common.OptionMapRWMutex.RLock()
	for key, value := range wantOptions {
		require.Equal(t, value, common.OptionMap[key])
	}
	common.OptionMapRWMutex.RUnlock()

	for key, value := range wantOptions {
		var option Option
		require.NoError(t, db.First(&option, "key = ?", key).Error)
		require.Equal(t, value, option.Value)
	}
}

func TestLoadOptionValuesPublishesOneMergedComfyUIH3PromptEnhancerSnapshot(t *testing.T) {
	previousSettings := model_setting.GetComfyUIH3PromptEnhancerSettings()
	common.OptionMapRWMutex.Lock()
	previousOptionMap := common.OptionMap
	common.OptionMap = make(map[string]string)
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		model_setting.ReplaceComfyUIH3PromptEnhancerSettings(previousSettings)
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previousOptionMap
		common.OptionMapRWMutex.Unlock()
	})

	initial := model_setting.ComfyUIH3PromptEnhancerSettings{
		Enabled:        true,
		BaseURL:        "https://default.example/v1",
		APIKey:         "default-key",
		Model:          "default-model",
		TimeoutSeconds: 8,
		SystemPrompt:   "default prompt",
	}
	model_setting.ReplaceComfyUIH3PromptEnhancerSettings(initial)

	loadOptionValues([]*Option{
		{Key: "comfyui_h3_prompt_enhancer.base_url", Value: "https://loaded.example/v1"},
		{Key: "comfyui_h3_prompt_enhancer.api_key", Value: "loaded-key"},
		{Key: "comfyui_h3_prompt_enhancer.timeout_seconds", Value: "20"},
	})

	require.Equal(t, model_setting.ComfyUIH3PromptEnhancerSettings{
		Enabled:        initial.Enabled,
		BaseURL:        "https://loaded.example/v1",
		APIKey:         "loaded-key",
		Model:          initial.Model,
		TimeoutSeconds: 20,
		SystemPrompt:   initial.SystemPrompt,
	}, model_setting.GetComfyUIH3PromptEnhancerSettings())

	common.OptionMapRWMutex.RLock()
	require.Equal(t, "https://loaded.example/v1", common.OptionMap["comfyui_h3_prompt_enhancer.base_url"])
	require.Equal(t, "loaded-key", common.OptionMap["comfyui_h3_prompt_enhancer.api_key"])
	require.Equal(t, "20", common.OptionMap["comfyui_h3_prompt_enhancer.timeout_seconds"])
	common.OptionMapRWMutex.RUnlock()
}

func TestLoadOptionValuesClearsComfyUIH3PromptEnhancerAPIKeyWhenURLChangesAndKeyIsMissing(t *testing.T) {
	previousSettings := model_setting.GetComfyUIH3PromptEnhancerSettings()
	t.Cleanup(func() {
		model_setting.ReplaceComfyUIH3PromptEnhancerSettings(previousSettings)
	})

	initial := model_setting.ComfyUIH3PromptEnhancerSettings{
		BaseURL:        "https://old.example/v1",
		APIKey:         "stored-key",
		TimeoutSeconds: 8,
	}
	model_setting.ReplaceComfyUIH3PromptEnhancerSettings(initial)

	loadOptionValues([]*Option{
		{Key: "comfyui_h3_prompt_enhancer.base_url", Value: "https://new.example/v1"},
	})

	loaded := model_setting.GetComfyUIH3PromptEnhancerSettings()
	require.Equal(t, "https://new.example/v1", loaded.BaseURL)
	require.Empty(t, loaded.APIKey)
}

func TestLoadOptionValuesPreservesComfyUIH3PromptEnhancerAPIKeyWhenURLIsUnchanged(t *testing.T) {
	previousSettings := model_setting.GetComfyUIH3PromptEnhancerSettings()
	t.Cleanup(func() {
		model_setting.ReplaceComfyUIH3PromptEnhancerSettings(previousSettings)
	})

	initial := model_setting.ComfyUIH3PromptEnhancerSettings{
		BaseURL:        "https://same.example/v1/",
		APIKey:         "stored-key",
		TimeoutSeconds: 8,
	}
	model_setting.ReplaceComfyUIH3PromptEnhancerSettings(initial)

	loadOptionValues([]*Option{
		{Key: "comfyui_h3_prompt_enhancer.base_url", Value: "https://same.example/v1"},
	})

	loaded := model_setting.GetComfyUIH3PromptEnhancerSettings()
	require.Equal(t, "https://same.example/v1", loaded.BaseURL)
	require.Equal(t, "stored-key", loaded.APIKey)
}

func TestLoadOptionValuesKeepsComfyUIH3PromptEnhancerURLAndKeyCoherent(t *testing.T) {
	previousSettings := model_setting.GetComfyUIH3PromptEnhancerSettings()
	common.OptionMapRWMutex.Lock()
	previousOptionMap := common.OptionMap
	common.OptionMap = make(map[string]string)
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		model_setting.ReplaceComfyUIH3PromptEnhancerSettings(previousSettings)
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previousOptionMap
		common.OptionMapRWMutex.Unlock()
	})

	first := model_setting.ComfyUIH3PromptEnhancerSettings{
		BaseURL:        "https://first.example/v1",
		APIKey:         "first-key",
		TimeoutSeconds: 8,
	}
	model_setting.ReplaceComfyUIH3PromptEnhancerSettings(first)

	var waitGroup sync.WaitGroup
	start := make(chan struct{})
	failure := make(chan model_setting.ComfyUIH3PromptEnhancerSettings, 1)
	waitGroup.Add(2)
	go func() {
		defer waitGroup.Done()
		<-start
		for iteration := 0; iteration < 2_000; iteration++ {
			if iteration%2 == 0 {
				loadOptionValues([]*Option{
					{Key: "comfyui_h3_prompt_enhancer.base_url", Value: "https://second.example/v1"},
					{Key: "comfyui_h3_prompt_enhancer.api_key", Value: "second-key"},
				})
			} else {
				loadOptionValues([]*Option{
					{Key: "comfyui_h3_prompt_enhancer.base_url", Value: first.BaseURL},
					{Key: "comfyui_h3_prompt_enhancer.api_key", Value: first.APIKey},
				})
			}
			runtime.Gosched()
		}
	}()
	go func() {
		defer waitGroup.Done()
		<-start
		for iteration := 0; iteration < 20_000; iteration++ {
			settings := model_setting.GetComfyUIH3PromptEnhancerSettings()
			coherentFirst := settings.BaseURL == first.BaseURL && settings.APIKey == first.APIKey
			coherentSecond := settings.BaseURL == "https://second.example/v1" && settings.APIKey == "second-key"
			if !coherentFirst && !coherentSecond {
				select {
				case failure <- settings:
				default:
				}
				return
			}
			runtime.Gosched()
		}
	}()
	close(start)
	waitGroup.Wait()
	close(failure)
	for settings := range failure {
		t.Fatalf("observed partial enhancer snapshot: base_url=%q api_key=%q", settings.BaseURL, settings.APIKey)
	}
}

func cloneStringMap(source map[string]string) map[string]string {
	cloned := make(map[string]string, len(source))
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}
