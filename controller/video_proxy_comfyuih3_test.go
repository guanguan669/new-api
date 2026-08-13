package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/require"
)

func TestTrustedComfyUIH3ResultURLRequiresConfiguredOutputOrigin(t *testing.T) {
	baseURL := "http://127.0.0.1:5900"
	channel := &model.Channel{Type: constant.ChannelTypeComfyUIH3, BaseURL: &baseURL}
	require.True(t, isTrustedComfyUIH3ResultURL(channel, "", "http://127.0.0.1:5900/view?filename=result.mp4&type=output"))
	require.True(t, isTrustedComfyUIH3ResultURL(channel, "", "http://127.0.0.1:5900/view?filename=result.mp4&subfolder=videos%2Fh3&type=output"))
	require.False(t, isTrustedComfyUIH3ResultURL(channel, "", "http://127.0.0.1:5900/view?filename=../secret.mp4&type=output"))
	require.False(t, isTrustedComfyUIH3ResultURL(channel, "", "http://127.0.0.1:5900/view?filename=result.mp4&type=input"))
	require.False(t, isTrustedComfyUIH3ResultURL(channel, "", "http://127.0.0.1:5901/view?filename=result.mp4&type=output"))
	require.False(t, isTrustedComfyUIH3ResultURL(channel, "", "http://127.0.0.1:5900/api/view?filename=result.mp4&type=output"))
	require.False(t, isTrustedComfyUIH3ResultURL(channel, "", "http://127.0.0.1:5900/view?filename=result.mp4&type=output&url=http://example.com"))
	baseURLWithPath := "http://127.0.0.1:5900/comfy"
	channel.BaseURL = &baseURLWithPath
	require.True(t, isTrustedComfyUIH3ResultURL(channel, "", "http://127.0.0.1:5900/comfy/view?filename=result.mp4&type=output"))
}

func TestTrustedComfyUIH3ResultURLAcceptsConfiguredWorkerOnly(t *testing.T) {
	baseURL := "http://127.0.0.1:5900"
	channel := &model.Channel{Type: constant.ChannelTypeComfyUIH3, BaseURL: &baseURL}
	channel.SetOtherSettings(dto.ChannelOtherSettings{ComfyUIH3BackendURLs: []string{"http://127.0.0.1:5901"}})
	require.True(t, isTrustedComfyUIH3ResultURL(channel, "http://127.0.0.1:5901", "http://127.0.0.1:5901/view?filename=result.mp4&type=output"))
	require.False(t, isTrustedComfyUIH3ResultURL(channel, "http://127.0.0.1:5999", "http://127.0.0.1:5999/view?filename=result.mp4&type=output"))
}
