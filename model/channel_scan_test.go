package model

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/require"
)

func TestChannelInfoScanSupportsSQLiteTextJSON(t *testing.T) {
	value := `{"is_multi_key":true,"multi_key_size":2,"multi_key_status_list":{"0":1,"1":2},"multi_key_disabled_reason":{"1":"rate limited"},"multi_key_disabled_time":{"1":12345},"multi_key_polling_index":1,"multi_key_mode":"polling"}`

	var info ChannelInfo
	require.NoError(t, info.Scan(value))

	require.True(t, info.IsMultiKey)
	require.Equal(t, 2, info.MultiKeySize)
	require.Equal(t, map[int]int{0: 1, 1: 2}, info.MultiKeyStatusList)
	require.Equal(t, map[int]string{1: "rate limited"}, info.MultiKeyDisabledReason)
	require.Equal(t, map[int]int64{1: 12345}, info.MultiKeyDisabledTime)
	require.Equal(t, 1, info.MultiKeyPollingIndex)
	require.Equal(t, constant.MultiKeyModePolling, info.MultiKeyMode)
}

func TestChannelInfoScanSupportsBlobJSON(t *testing.T) {
	value := []byte(`{"is_multi_key":true,"multi_key_size":3,"multi_key_status_list":{"0":1,"2":3},"multi_key_polling_index":2,"multi_key_mode":"random"}`)

	var info ChannelInfo
	require.NoError(t, info.Scan(value))

	require.True(t, info.IsMultiKey)
	require.Equal(t, 3, info.MultiKeySize)
	require.Equal(t, map[int]int{0: 1, 2: 3}, info.MultiKeyStatusList)
	require.Equal(t, 2, info.MultiKeyPollingIndex)
	require.Equal(t, constant.MultiKeyModeRandom, info.MultiKeyMode)
}

func TestChannelInfoScanIgnoresNil(t *testing.T) {
	info := ChannelInfo{IsMultiKey: true}
	require.NoError(t, info.Scan(nil))
	require.True(t, info.IsMultiKey)
}
