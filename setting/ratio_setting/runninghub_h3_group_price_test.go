package ratio_setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunningHubH3GroupPriceValidationAndGetter(t *testing.T) {
	original := RunningHubH3GroupPrice2JSONString()
	t.Cleanup(func() {
		require.NoError(t, UpdateRunningHubH3GroupPriceByJSONString(original))
	})

	require.NoError(t, UpdateRunningHubH3GroupPriceByJSONString(`{"default":{"price_768p":0,"price_2k":0.25},"vip":{"price_768p":0.1,"price_2k":0.5}}`))

	price, ok := GetRunningHubH3GroupPrice("default")
	require.True(t, ok)
	assert.Equal(t, RunningHubH3GroupPrice{Price768P: 0, Price2K: 0.25}, price)

	_, ok = GetRunningHubH3GroupPrice("svip")
	assert.False(t, ok)
}

func TestRunningHubH3GroupPriceRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{name: "auto group", value: `{"auto":{"price_768p":0,"price_2k":0}}`},
		{name: "unknown group", value: `{"missing":{"price_768p":0,"price_2k":0}}`},
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
