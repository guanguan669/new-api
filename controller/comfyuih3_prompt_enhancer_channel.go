package controller

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/internalchat"
	"github.com/gin-gonic/gin"
)

type comfyUIH3PromptEnhancerChannel struct {
	ID     int      `json:"id"`
	Name   string   `json:"name"`
	Type   int      `json:"type"`
	Status int      `json:"status"`
	Models []string `json:"models"`
}

var getComfyUIH3PromptEnhancerChannelByID = model.GetChannelById

var listComfyUIH3PromptEnhancerChannels = func() ([]*model.Channel, error) {
	var channels []*model.Channel
	err := model.DB.
		Select("id", "name", "type", "status", "models", "settings").
		Find(&channels).Error
	return channels, err
}

func validateComfyUIH3PromptEnhancerChannel(channelID int, modelName string) error {
	if channelID <= 0 {
		return fmt.Errorf("请选择提示词增强渠道")
	}
	channel, err := getComfyUIH3PromptEnhancerChannelByID(channelID, true)
	if err != nil {
		return fmt.Errorf("读取提示词增强渠道失败: %w", err)
	}
	if err := internalchat.ValidateChannel(channel, modelName); err != nil {
		return fmt.Errorf("提示词增强渠道不可用: %w", err)
	}
	return nil
}

// GetComfyUIH3PromptEnhancerChannels returns only the metadata needed by the
// root settings form. Channel credentials and upstream configuration never
// enter the response DTO.
func GetComfyUIH3PromptEnhancerChannels(c *gin.Context) {
	channels, err := listComfyUIH3PromptEnhancerChannels()
	if err != nil {
		common.ApiError(c, err)
		return
	}

	result := make([]comfyUIH3PromptEnhancerChannel, 0, len(channels))
	for _, channel := range channels {
		if channel == nil || channel.Status != common.ChannelStatusEnabled || !internalchat.SupportsChannel(channel) {
			continue
		}
		models := make([]string, 0)
		seen := make(map[string]struct{})
		for _, modelName := range channel.GetModels() {
			modelName = strings.TrimSpace(modelName)
			if modelName == "" {
				continue
			}
			if err := internalchat.ValidateChannel(channel, modelName); err != nil {
				continue
			}
			if _, exists := seen[modelName]; exists {
				continue
			}
			seen[modelName] = struct{}{}
			models = append(models, modelName)
		}
		sort.Strings(models)
		if len(models) == 0 {
			continue
		}
		result = append(result, comfyUIH3PromptEnhancerChannel{
			ID: channel.Id, Name: channel.Name, Type: channel.Type,
			Status: channel.Status, Models: models,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Name == result[j].Name {
			return result[i].ID < result[j].ID
		}
		return result[i].Name < result[j].Name
	})

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": result})
}
