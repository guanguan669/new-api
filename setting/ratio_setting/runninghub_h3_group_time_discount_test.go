package ratio_setting

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunningHubH3GroupTimeDiscountCrossMidnightAndNextChange(t *testing.T) {
	original := RunningHubH3GroupTimeDiscount2JSONString()
	originalGroups := GroupRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, UpdateGroupRatioByJSONString(originalGroups))
		require.NoError(t, UpdateRunningHubH3GroupTimeDiscountByJSONString(original))
	})
	require.NoError(t, UpdateGroupRatioByJSONString(`{"standard":1}`))
	require.NoError(t, UpdateRunningHubH3GroupTimeDiscountByJSONString(`{
		"standard":[{"start":"22:00","end":"02:00","discount":0.5}]
	}`))

	shanghai := time.FixedZone("test-shanghai", 8*60*60)
	state := ResolveRunningHubH3GroupTimeDiscount("standard", time.Date(2026, 8, 15, 23, 30, 0, 0, shanghai))
	assert.Equal(t, 0.5, state.Multiplier)
	assert.Equal(t, time.Date(2026, 8, 16, 2, 0, 0, 0, shanghai).Unix(), state.NextChangeAt)

	state = ResolveRunningHubH3GroupTimeDiscount("standard", time.Date(2026, 8, 16, 3, 0, 0, 0, shanghai))
	assert.Equal(t, 1.0, state.Multiplier)
	assert.Equal(t, time.Date(2026, 8, 16, 22, 0, 0, 0, shanghai).Unix(), state.NextChangeAt)
}

func TestRunningHubH3GroupTimeDiscountAllowsWindowEndingAtMidnight(t *testing.T) {
	original := RunningHubH3GroupTimeDiscount2JSONString()
	originalGroups := GroupRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, UpdateGroupRatioByJSONString(originalGroups))
		require.NoError(t, UpdateRunningHubH3GroupTimeDiscountByJSONString(original))
	})
	require.NoError(t, UpdateGroupRatioByJSONString(`{"minimaxh3":1}`))
	require.NoError(t, UpdateRunningHubH3GroupTimeDiscountByJSONString(`{
		"minimaxh3":[
			{"start":"00:00","end":"09:00","discount":0.3},
			{"start":"18:00","end":"22:00","discount":0.8},
			{"start":"22:00","end":"00:00","discount":0.5}
		]
	}`))

	shanghai := time.FixedZone("test-shanghai", 8*60*60)
	state := ResolveRunningHubH3GroupTimeDiscount("minimaxh3", time.Date(2026, 8, 15, 1, 0, 0, 0, shanghai))
	assert.Equal(t, 0.3, state.Multiplier)
	assert.Equal(t, time.Date(2026, 8, 15, 9, 0, 0, 0, shanghai).Unix(), state.NextChangeAt)

	state = ResolveRunningHubH3GroupTimeDiscount("minimaxh3", time.Date(2026, 8, 15, 20, 0, 0, 0, shanghai))
	assert.Equal(t, 0.8, state.Multiplier)
	assert.Equal(t, time.Date(2026, 8, 15, 22, 0, 0, 0, shanghai).Unix(), state.NextChangeAt)

	state = ResolveRunningHubH3GroupTimeDiscount("minimaxh3", time.Date(2026, 8, 15, 22, 30, 0, 0, shanghai))
	assert.Equal(t, 0.5, state.Multiplier)
	assert.Equal(t, time.Date(2026, 8, 16, 0, 0, 0, 0, shanghai).Unix(), state.NextChangeAt)
}

func TestRunningHubH3GroupTimeDiscountValidation(t *testing.T) {
	originalGroups := GroupRatio2JSONString()
	t.Cleanup(func() { require.NoError(t, UpdateGroupRatioByJSONString(originalGroups)) })
	require.NoError(t, UpdateGroupRatioByJSONString(`{"standard":1}`))

	tests := []string{
		`{"missing":[{"start":"08:00","end":"09:00","discount":0.8}]}`,
		`{"standard":[{"start":"8:00","end":"09:00","discount":0.8}]}`,
		`{"standard":[{"start":"08:00","end":"08:00","discount":0.8}]}`,
		`{"standard":[{"start":"08:00","end":"09:00","discount":-0.1}]}`,
		`{"standard":[{"start":"08:00","end":"09:00","discount":1.1}]}`,
		`{"standard":[{"start":"08:00","end":"10:00","discount":0.8},{"start":"09:00","end":"11:00","discount":0.7}]}`,
		`{"standard":[{"start":"22:00","end":"02:00","discount":0.8},{"start":"01:00","end":"03:00","discount":0.7}]}`,
	}
	for _, value := range tests {
		assert.Error(t, ValidateRunningHubH3GroupTimeDiscountJSON(value), value)
	}
	assert.NoError(t, ValidateRunningHubH3GroupTimeDiscountJSON(`{"standard":[{"start":"08:00","end":"10:00","discount":0},{"start":"10:00","end":"12:00","discount":1}]}`))
}
