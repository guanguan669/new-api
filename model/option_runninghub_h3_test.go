package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadOptionValuesLoadsGroupRatioBeforeRunningHubH3Prices(t *testing.T) {
	common.OptionMapRWMutex.Lock()
	originalOptionMap := common.OptionMap
	common.OptionMap = make(map[string]string)
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = originalOptionMap
		common.OptionMapRWMutex.Unlock()
	})

	originalGroupRatio := ratio_setting.GroupRatio2JSONString()
	originalH3Prices := ratio_setting.RunningHubH3GroupPrice2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatio))
		require.NoError(t, ratio_setting.UpdateRunningHubH3GroupPriceByJSONString(originalH3Prices))
	})

	loadOptionValues([]*Option{
		{Key: ratio_setting.RunningHubH3GroupPriceOptionKey, Value: `{"new_billable":{"price_768p":0.1,"price_2k":0.2}}`},
		{Key: "GroupRatio", Value: `{"new_billable":1}`},
	})

	price, ok := ratio_setting.GetRunningHubH3GroupPrice("new_billable")
	require.True(t, ok)
	assert.Equal(t, ratio_setting.RunningHubH3GroupPrice{Price768P: 0.1, Price2K: 0.2}, price)
}

func TestLoadOptionValuesLoadsNamespacedGroupRatioBeforeRunningHubH3Prices(t *testing.T) {
	common.OptionMapRWMutex.Lock()
	originalOptionMap := common.OptionMap
	common.OptionMap = make(map[string]string)
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = originalOptionMap
		common.OptionMapRWMutex.Unlock()
	})

	originalGroupRatio := ratio_setting.GroupRatio2JSONString()
	originalH3Prices := ratio_setting.RunningHubH3GroupPrice2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatio))
		require.NoError(t, ratio_setting.UpdateRunningHubH3GroupPriceByJSONString(originalH3Prices))
	})

	loadOptionValues([]*Option{
		{Key: "group_ratio_setting.runninghub_h3_group_price", Value: `{"namespaced_billable":{"price_768p":0.3,"price_2k":0.4}}`},
		{Key: "group_ratio_setting.group_ratio", Value: `{"namespaced_billable":1}`},
	})

	price, ok := ratio_setting.GetRunningHubH3GroupPrice("namespaced_billable")
	require.True(t, ok)
	assert.Equal(t, ratio_setting.RunningHubH3GroupPrice{Price768P: 0.3, Price2K: 0.4}, price)
}

func TestValidateOptionValueValidatesNamespacedRunningHubH3Prices(t *testing.T) {
	assert.Error(t, validateOptionValue("group_ratio_setting.runninghub_h3_group_price", `{"missing":{"price_768p":0,"price_2k":0}}`))
}
