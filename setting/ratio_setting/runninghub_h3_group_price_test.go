package ratio_setting

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunningHubH3GroupPriceValidationAndGetter(t *testing.T) {
	original := RunningHubH3GroupPrice2JSONString()
	t.Cleanup(func() {
		require.NoError(t, UpdateRunningHubH3GroupPriceByJSONString(original))
	})

	require.NoError(t, UpdateRunningHubH3GroupPriceByJSONString(`{"standard":{"price_768p":0,"price_2k":0.25},"user-105":{"price_768p":0.1,"price_2k":0.5}}`))

	price, ok := GetRunningHubH3GroupPrice("standard")
	require.True(t, ok)
	assert.Equal(t, RunningHubH3GroupPrice{Price768P: 0, Price2K: 0.25}, price)

	_, ok = GetRunningHubH3GroupPrice("missing")
	assert.False(t, ok)
}

func TestRunningHubH3GroupPriceAllowsPlansOutsideGroupRatio(t *testing.T) {
	require.NoError(t, ValidateRunningHubH3GroupPriceJSON(`{"private-plan":{"price_768p":0.04,"price_2k":0.3}}`))
}

func TestRunningHubH3GroupPriceExplicitAndLegacyBindings(t *testing.T) {
	originalPrices := RunningHubH3GroupPrice2JSONString()
	originalGroups := GroupRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, UpdateGroupRatioByJSONString(originalGroups))
		require.NoError(t, UpdateRunningHubH3GroupPriceByJSONString(originalPrices))
	})
	require.NoError(t, UpdateGroupRatioByJSONString(`{"default":1,"vip":1}`))
	require.NoError(t, UpdateRunningHubH3GroupPriceByJSONString(`{
		"legacy-same-name":{"price_768p":0.1,"price_2k":0.3},
		"vip":{"price_768p":0.2,"price_2k":0.6},
		"custom-tier":{"group":"default","price_768p":0.3,"price_2k":0.9},
		"legacy-standalone":{"price_768p":0.4,"price_2k":1.2}
	}`))

	vip, ok := GetRunningHubH3GroupPrice("vip")
	require.True(t, ok)
	assert.Equal(t, "vip", vip.BoundGroup)
	custom, ok := GetRunningHubH3GroupPrice("custom-tier")
	require.True(t, ok)
	assert.Equal(t, "default", custom.BoundGroup)
	standalone, ok := GetRunningHubH3GroupPrice("legacy-standalone")
	require.True(t, ok)
	assert.Empty(t, standalone.BoundGroup)
	assert.Contains(t, RunningHubH3GroupPrice2JSONString(), `"group":"default"`)
}

func TestRunningHubH3GroupPriceRejectsUnknownExplicitBinding(t *testing.T) {
	assert.ErrorContains(t, ValidateRunningHubH3GroupPriceJSON(`{"custom":{"group":"missing","price_768p":0.1,"price_2k":0.3}}`), "GroupRatio")
	assert.ErrorContains(t, ValidateRunningHubH3GroupPriceJSON(`{"custom":{"group":" ","price_768p":0.1,"price_2k":0.3}}`), "cannot be empty")
}

func TestRunningHubH3GroupPriceAllowsManyPricingTiers(t *testing.T) {
	tiers := make(map[string]RunningHubH3GroupPrice, 512)
	for index := 1; index <= 512; index++ {
		tiers[fmt.Sprintf("tier_%d", index)] = RunningHubH3GroupPrice{
			Price768P: 0.04,
			Price2K:   0.3,
		}
	}
	value, err := json.Marshal(tiers)
	require.NoError(t, err)
	require.NoError(t, ValidateRunningHubH3GroupPriceJSON(string(value)))
}

func TestRunningHubH3GroupPriceUpdateNeverExposesEmptyMap(t *testing.T) {
	original := RunningHubH3GroupPrice2JSONString()
	t.Cleanup(func() {
		require.NoError(t, UpdateRunningHubH3GroupPriceByJSONString(original))
	})
	require.NoError(t, UpdateRunningHubH3GroupPriceByJSONString(`{"old":{"price_768p":0.1,"price_2k":0.3}}`))

	var observedEmpty atomic.Bool
	var readers sync.WaitGroup
	start := make(chan struct{})
	for range 4 {
		readers.Add(1)
		go func() {
			defer readers.Done()
			<-start
			for range 250 {
				if len(GetRunningHubH3GroupPriceCopy()) == 0 {
					observedEmpty.Store(true)
					return
				}
			}
		}()
	}

	close(start)
	for range 100 {
		require.NoError(t, UpdateRunningHubH3GroupPriceByJSONString(`{"new":{"price_768p":0.2,"price_2k":0.6}}`))
		require.NoError(t, UpdateRunningHubH3GroupPriceByJSONString(`{"old":{"price_768p":0.1,"price_2k":0.3}}`))
	}
	readers.Wait()
	require.False(t, observedEmpty.Load())
}

func TestRunningHubH3GroupPriceRejectsInvalidValues(t *testing.T) {
	tooLongPlan := strings.Repeat("a", 65)
	tests := []struct {
		name  string
		value string
	}{
		{name: "reserved auto plan", value: `{"auto":{"price_768p":0,"price_2k":0}}`},
		{name: "reserved none sentinel", value: `{"__none__":{"price_768p":0,"price_2k":0}}`},
		{name: "empty plan", value: `{"":{"price_768p":0,"price_2k":0}}`},
		{name: "whitespace plan", value: `{" private ":{"price_768p":0,"price_2k":0}}`},
		{name: "plan name too long", value: `{"` + tooLongPlan + `":{"price_768p":0,"price_2k":0}}`},
		{name: "missing 768p", value: `{"default":{"price_2k":0}}`},
		{name: "missing 2k", value: `{"default":{"price_768p":0}}`},
		{name: "negative 768p", value: `{"default":{"price_768p":-0.01,"price_2k":0}}`},
		{name: "negative 2k", value: `{"default":{"price_768p":0,"price_2k":-0.01}}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Error(t, ValidateRunningHubH3GroupPriceJSON(test.value))
		})
	}
}
