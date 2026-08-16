package controller

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveRunningHubH3GroupTimeDiscountsOnlyReturnsUsableGroups(t *testing.T) {
	original := ratio_setting.RunningHubH3GroupTimeDiscount2JSONString()
	originalGroups := ratio_setting.GroupRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroups))
		require.NoError(t, ratio_setting.UpdateRunningHubH3GroupTimeDiscountByJSONString(original))
	})
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"hidden":1,"vip":1}`))
	require.NoError(t, ratio_setting.UpdateRunningHubH3GroupTimeDiscountByJSONString(`{
		"default":[{"start":"00:00","end":"08:00","discount":0.8}],
		"hidden":[{"start":"00:00","end":"08:00","discount":0.4}]
	}`))

	now := time.Date(2026, 8, 15, 1, 0, 0, 0, time.FixedZone("UTC+8", 8*60*60))
	states := resolveRunningHubH3GroupTimeDiscounts(map[string]string{"default": "Default", "vip": "VIP"}, now)

	assert.Equal(t, 0.8, states["default"].Multiplier)
	assert.NotZero(t, states["default"].NextChangeAt)
	assert.Equal(t, ratio_setting.RunningHubH3GroupTimeDiscountState{Multiplier: 1}, states["vip"])
	assert.NotContains(t, states, "hidden")
}
