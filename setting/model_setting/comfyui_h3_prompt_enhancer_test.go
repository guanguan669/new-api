package model_setting

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestComfyUIH3PromptEnhancerSnapshotsAreImmutable(t *testing.T) {
	original := GetComfyUIH3PromptEnhancerSettings()
	t.Cleanup(func() { ReplaceComfyUIH3PromptEnhancerSettings(original) })

	ReplaceComfyUIH3PromptEnhancerSettings(ComfyUIH3PromptEnhancerSettings{
		Enabled:        true,
		BaseURL:        "https://one.example/v1",
		APIKey:         "secret-one",
		Model:          "vision-one",
		TimeoutSeconds: 8,
		SystemPrompt:   "system-one",
	})
	snapshot := GetComfyUIH3PromptEnhancerSettings()
	UpdateComfyUIH3PromptEnhancerOption("base_url", "https://two.example/v1")

	require.Equal(t, "https://one.example/v1", snapshot.BaseURL)
	require.Equal(t, "https://two.example/v1", GetComfyUIH3PromptEnhancerSettings().BaseURL)
}

func TestComfyUIH3PromptEnhancerConcurrentReadsAndUpdates(t *testing.T) {
	original := GetComfyUIH3PromptEnhancerSettings()
	t.Cleanup(func() { ReplaceComfyUIH3PromptEnhancerSettings(original) })
	ReplaceComfyUIH3PromptEnhancerSettings(ComfyUIH3PromptEnhancerSettings{
		Enabled:        true,
		BaseURL:        "https://initial.example/v1",
		Model:          "vision",
		TimeoutSeconds: 8,
		SystemPrompt:   "system",
	})

	var workers sync.WaitGroup
	for i := 0; i < 8; i++ {
		workers.Add(1)
		go func(worker int) {
			defer workers.Done()
			for update := 0; update < 100; update++ {
				if worker%2 == 0 {
					UpdateComfyUIH3PromptEnhancerOption("base_url", fmt.Sprintf("https://host-%d-%d.example/v1", worker, update))
				} else {
					settings := GetComfyUIH3PromptEnhancerSettings()
					require.NotEmpty(t, settings.BaseURL)
				}
			}
		}(i)
	}
	workers.Wait()
}

func TestComfyUIH3PromptEnhancerExportOmitsAPIKey(t *testing.T) {
	original := GetComfyUIH3PromptEnhancerSettings()
	t.Cleanup(func() { ReplaceComfyUIH3PromptEnhancerSettings(original) })
	ReplaceComfyUIH3PromptEnhancerSettings(ComfyUIH3PromptEnhancerSettings{
		Enabled:        true,
		BaseURL:        "https://example.com/v1",
		APIKey:         "must-not-export",
		Model:          "vision",
		TimeoutSeconds: 8,
		SystemPrompt:   "system",
	})

	exported, err := comfyUIH3PromptEnhancerSettings.ConfigToMap()
	require.NoError(t, err)
	require.NotContains(t, exported, "api_key")
	require.Equal(t, "must-not-export", GetComfyUIH3PromptEnhancerSettings().APIKey)
}

func TestDefaultComfyUIH3ContextIRSystemPromptIntegrity(t *testing.T) {
	digest := sha256.Sum256([]byte(DefaultComfyUIH3ContextIRSystemPrompt))
	require.Equal(t, "46737a9cffcf560633e1bd7a8f892c5368843785a01c3418dd7344579ee5e4ec", hex.EncodeToString(digest[:]))
	require.Less(t, strings.Index(DefaultComfyUIH3ContextIRSystemPrompt, "integrated_multimodal_description:"), strings.Index(DefaultComfyUIH3ContextIRSystemPrompt, "overall_soundscape:"))
	require.Less(t, strings.Index(DefaultComfyUIH3ContextIRSystemPrompt, "overall_soundscape:"), strings.Index(DefaultComfyUIH3ContextIRSystemPrompt, "non_diegetic_music:"))
}
