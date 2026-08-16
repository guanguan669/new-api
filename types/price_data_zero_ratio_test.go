package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPriceDataPreservesZeroOtherRatio(t *testing.T) {
	priceData := PriceData{Quota: 100}
	priceData.AddOtherRatioAllowZero("h3_time_discount", 0)

	assert.Equal(t, map[string]float64{"h3_time_discount": 0}, priceData.OtherRatios())
	assert.Zero(t, priceData.OtherRatioMultiplier())
	assert.Zero(t, priceData.ApplyOtherRatiosToFloat(100))
	assert.Equal(t, 100.0, priceData.RemoveOtherRatiosFromFloat(100))
}
