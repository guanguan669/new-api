package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadOptionValuesLoadsRunningHubH3GroupTimeDiscount(t *testing.T) {
	original := ratio_setting.RunningHubH3GroupTimeDiscount2JSONString()
	originalGroups := ratio_setting.GroupRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroups))
		require.NoError(t, ratio_setting.UpdateRunningHubH3GroupTimeDiscountByJSONString(original))
	})
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"service_group":1}`))

	common.OptionMapRWMutex.Lock()
	originalOptionMap := common.OptionMap
	common.OptionMap = make(map[string]string)
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = originalOptionMap
		common.OptionMapRWMutex.Unlock()
	})

	value := `{"service_group":[{"start":"00:00","end":"08:00","discount":0.8}]}`
	loadOptionValues([]*Option{{Key: ratio_setting.RunningHubH3GroupTimeDiscountOptionKey, Value: value}})

	state := ratio_setting.ResolveRunningHubH3GroupTimeDiscount("service_group", time.Date(2026, 8, 15, 1, 0, 0, 0, time.FixedZone("UTC+8", 8*60*60)))
	assert.Equal(t, 0.8, state.Multiplier)
	common.OptionMapRWMutex.Lock()
	assert.Equal(t, value, common.OptionMap[ratio_setting.RunningHubH3GroupTimeDiscountOptionKey])
	common.OptionMapRWMutex.Unlock()
}

func TestValidateOptionValueRejectsInvalidRunningHubH3GroupTimeDiscount(t *testing.T) {
	originalGroups := ratio_setting.GroupRatio2JSONString()
	t.Cleanup(func() { require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroups)) })
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"service_group":1}`))
	assert.NoError(t, validateOptionValue(ratio_setting.RunningHubH3GroupTimeDiscountOptionKey, `{"service_group":[{"start":"00:00","end":"08:00","discount":0}]}`))
	assert.Error(t, validateOptionValue(ratio_setting.RunningHubH3GroupTimeDiscountOptionKey, `{"service_group":[{"start":"00:00","end":"08:00","discount":2}]}`))
}

func TestUpdateOptionHotUpdatesRunningHubH3GroupTimeDiscount(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.AutoMigrate(&Option{}))
	common.OptionMapRWMutex.Lock()
	originalOptionMap := common.OptionMap
	common.OptionMap = make(map[string]string)
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = originalOptionMap
		common.OptionMapRWMutex.Unlock()
	})
	original := ratio_setting.RunningHubH3GroupTimeDiscount2JSONString()
	originalGroups := ratio_setting.GroupRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroups))
		require.NoError(t, ratio_setting.UpdateRunningHubH3GroupTimeDiscountByJSONString(original))
	})
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"service_group":1}`))

	value := `{"service_group":[{"start":"00:00","end":"08:00","discount":0.6}]}`
	require.NoError(t, UpdateOption(ratio_setting.RunningHubH3GroupTimeDiscountOptionKey, value))

	var option Option
	require.NoError(t, DB.First(&option, "key = ?", ratio_setting.RunningHubH3GroupTimeDiscountOptionKey).Error)
	assert.Equal(t, value, option.Value)
	state := ratio_setting.ResolveRunningHubH3GroupTimeDiscount("service_group", time.Date(2026, 8, 15, 1, 0, 0, 0, time.FixedZone("UTC+8", 8*60*60)))
	assert.Equal(t, 0.6, state.Multiplier)
}

func TestTaskBillingContextPreservesZeroH3TimeDiscount(t *testing.T) {
	original := TaskPrivateData{BillingContext: &TaskBillingContext{
		OtherRatios: map[string]float64{"h3_time_discount": 0},
	}}
	value, err := original.Value()
	require.NoError(t, err)
	var decoded TaskPrivateData
	require.NoError(t, decoded.Scan(value))
	require.NotNil(t, decoded.BillingContext)
	assert.Equal(t, map[string]float64{"h3_time_discount": 0}, decoded.BillingContext.OtherRatios)
}
