package relay

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyH3TimeDiscountRecomputesCustomPriceData(t *testing.T) {
	info := &relaycommon.RelayInfo{PriceData: types.PriceData{ModelPrice: 2, UsePrice: true}}
	info.PriceData.AddOtherRatio("seconds", 5)
	applyH3TimeDiscount(info, 0.5, true)
	expected, _ := common.QuotaFromFloatChecked(2 * common.QuotaPerUnit * 5 * 0.5)
	assert.Equal(t, expected, info.PriceData.Quota)
	assert.Equal(t, 0.5, info.PriceData.OtherRatios()[h3TimeDiscountContextKey])
}

func TestApplyH3TimeDiscountGenericFallbackUsesNormalQuotaPass(t *testing.T) {
	info := &relaycommon.RelayInfo{PriceData: types.PriceData{Quota: 100}}
	applyH3TimeDiscount(info, 0.8, false)
	quota, _ := common.QuotaFromFloatChecked(info.PriceData.ApplyOtherRatiosToFloat(float64(info.PriceData.Quota)))
	assert.Equal(t, 80, quota)
}

func TestApplyH3TimeDiscountZeroIsFrozenAndFree(t *testing.T) {
	info := &relaycommon.RelayInfo{PriceData: types.PriceData{ModelPrice: 2, UsePrice: true, Quota: 100}}
	applyH3TimeDiscount(info, 0, true)
	assert.True(t, info.PriceData.FreeModel)
	assert.Zero(t, info.PriceData.Quota)
	assert.Equal(t, map[string]float64{h3TimeDiscountContextKey: 0}, info.PriceData.OtherRatios())
}

func TestAdjustedRatiosPreserveFrozenH3TimeDiscount(t *testing.T) {
	info := &relaycommon.RelayInfo{PriceData: types.PriceData{Quota: 100}}
	info.PriceData.AddOtherRatio("seconds", 2)
	info.PriceData.AddOtherRatioAllowZero(h3TimeDiscountContextKey, 0.5)
	adjusted := map[string]float64{"seconds": 3}
	preserveH3TimeDiscount(adjusted, "minimax_h3", 0.5)
	quota, ok := recalcQuotaFromRatios(info, adjusted)
	require.True(t, ok)
	assert.Equal(t, 150, quota)
	assert.Equal(t, 0.5, adjusted[h3TimeDiscountContextKey])
}
