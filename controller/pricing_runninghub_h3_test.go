package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilterRunningHubH3GroupPricesByUsableGroups(t *testing.T) {
	original := ratio_setting.RunningHubH3GroupPrice2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateRunningHubH3GroupPriceByJSONString(original))
	})
	require.NoError(t, ratio_setting.UpdateRunningHubH3GroupPriceByJSONString(`{"default":{"price_768p":0,"price_2k":0.2},"vip":{"price_768p":0.1,"price_2k":0.4},"svip":{"price_768p":0.3,"price_2k":0.8}}`))

	prices := filterRunningHubH3GroupPricesByUsableGroups(map[string]string{
		"default": "Default",
		"svip":    "SVIP",
	})

	assert.Equal(t, map[string]ratio_setting.RunningHubH3GroupPrice{
		"default": {Price768P: 0, Price2K: 0.2},
		"svip":    {Price768P: 0.3, Price2K: 0.8},
	}, prices)
	assert.NotContains(t, prices, "vip")
}

func TestFilterRunningHubH3GroupPricesNoUsableGroups(t *testing.T) {
	prices := filterRunningHubH3GroupPricesByUsableGroups(map[string]string{})
	assert.Empty(t, prices)
}

func TestFilterRunningHubH3GroupPricesForUserUsesAssignedGroupOutsideUsableGroups(t *testing.T) {
	original := ratio_setting.RunningHubH3GroupPrice2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateRunningHubH3GroupPriceByJSONString(original))
	})
	require.NoError(t, ratio_setting.UpdateRunningHubH3GroupPriceByJSONString(`{"default":{"price_768p":0.04,"price_2k":0.12}}`))

	prices := filterRunningHubH3GroupPricesForUser(map[string]string{}, "default")

	assert.Equal(t, map[string]ratio_setting.RunningHubH3GroupPrice{
		"default": {Price768P: 0.04, Price2K: 0.12},
	}, prices)
}

func TestFilterRunningHubH3GroupPricesForUserFallsBackWhenAssignmentRemoved(t *testing.T) {
	original := ratio_setting.RunningHubH3GroupPrice2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateRunningHubH3GroupPriceByJSONString(original))
	})
	require.NoError(t, ratio_setting.UpdateRunningHubH3GroupPriceByJSONString(`{"default":{"price_768p":0.1,"price_2k":0.3}}`))

	prices := filterRunningHubH3GroupPricesForUser(map[string]string{
		"default": "Default",
	}, "removed_group")

	assert.Equal(t, map[string]ratio_setting.RunningHubH3GroupPrice{
		"default": {Price768P: 0.1, Price2K: 0.3},
	}, prices)
}
