package model

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/require"
)

func TestInitTaskStoresComfyUIH3GatewayKey(t *testing.T) {
	task := InitTask(constant.TaskPlatform("comfyuih3"), &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType: constant.ChannelTypeComfyUIH3,
			ApiKey:      "gateway-secret",
			ChannelOtherSettings: dto.ChannelOtherSettings{
				ComfyUIH3GatewayURL: "http://gateway.internal:8090",
			},
		},
	})
	require.Equal(t, "gateway-secret", task.PrivateData.Key)
	require.True(t, task.PrivateData.ComfyUIH3Gateway)

	directTask := InitTask(constant.TaskPlatform("comfyuih3"), &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeComfyUIH3, ApiKey: "direct-secret"},
	})
	require.False(t, directTask.PrivateData.ComfyUIH3Gateway)
}
