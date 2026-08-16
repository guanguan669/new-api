package controller

import (
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
)

func filterPricingByUsableGroups(pricing []model.Pricing, usableGroup map[string]string) []model.Pricing {
	if len(pricing) == 0 {
		return pricing
	}
	if len(usableGroup) == 0 {
		return []model.Pricing{}
	}

	filtered := make([]model.Pricing, 0, len(pricing))
	for _, item := range pricing {
		if common.StringsContains(item.EnableGroup, "all") {
			filtered = append(filtered, item)
			continue
		}
		for _, group := range item.EnableGroup {
			if _, ok := usableGroup[group]; ok {
				filtered = append(filtered, item)
				break
			}
		}
	}
	return filtered
}

func filterRunningHubH3GroupPricesByUsableGroups(usableGroup map[string]string) map[string]ratio_setting.RunningHubH3GroupPrice {
	if len(usableGroup) == 0 {
		return map[string]ratio_setting.RunningHubH3GroupPrice{}
	}
	configuredPrices := ratio_setting.GetRunningHubH3GroupPriceCopy()
	if len(configuredPrices) == 0 {
		return map[string]ratio_setting.RunningHubH3GroupPrice{}
	}

	prices := make(map[string]ratio_setting.RunningHubH3GroupPrice, len(configuredPrices))
	for group, price := range configuredPrices {
		boundGroup := strings.TrimSpace(price.BoundGroup)
		if boundGroup == "" {
			continue
		}
		if _, ok := usableGroup[boundGroup]; ok {
			prices[group] = price
		}
	}
	return prices
}

// filterRunningHubH3GroupPricesForUser returns a user's explicitly assigned
// H3 price group when present. That group is a billing preference, not an API
// key or account group, so it does not need to be exposed through usableGroup.
// A stale assignment falls back to the normal visible-group behavior.
func filterRunningHubH3GroupPricesForUser(usableGroup map[string]string, priceGroup string) map[string]ratio_setting.RunningHubH3GroupPrice {
	prices, _ := resolveRunningHubH3PricingForUser(usableGroup, priceGroup)
	return prices
}

func resolveRunningHubH3PricingForUser(usableGroup map[string]string, priceGroup string) (map[string]ratio_setting.RunningHubH3GroupPrice, string) {
	priceGroup = strings.TrimSpace(priceGroup)
	if priceGroup != "" {
		if price, ok := ratio_setting.GetRunningHubH3GroupPrice(priceGroup); ok && (strings.TrimSpace(price.BoundGroup) == "" || hasUsableBoundGroup(usableGroup, price.BoundGroup)) {
			return map[string]ratio_setting.RunningHubH3GroupPrice{
				priceGroup: price,
			}, priceGroup
		}
	}
	return filterRunningHubH3GroupPricesByUsableGroups(usableGroup), ""
}

func hasUsableBoundGroup(usableGroup map[string]string, boundGroup string) bool {
	_, ok := usableGroup[strings.TrimSpace(boundGroup)]
	return ok
}

func resolveRunningHubH3GroupTimeDiscounts(usableGroup map[string]string, now time.Time) map[string]ratio_setting.RunningHubH3GroupTimeDiscountState {
	states := make(map[string]ratio_setting.RunningHubH3GroupTimeDiscountState, len(usableGroup))
	for group := range usableGroup {
		states[group] = ratio_setting.ResolveRunningHubH3GroupTimeDiscount(group, now)
	}
	return states
}

func GetPricing(c *gin.Context) {
	pricing := model.GetPricing()
	userId, exists := c.Get("id")
	usableGroup := map[string]string{}
	groupRatio := map[string]float64{}
	runningHubH3PriceGroup := ""
	for s, f := range ratio_setting.GetGroupRatioCopy() {
		groupRatio[s] = f
	}
	var group string
	if exists {
		user, err := model.GetUserCache(userId.(int))
		if err == nil {
			group = user.Group
			runningHubH3PriceGroup = user.GetSetting().RunningHubH3PriceGroup
			for g := range groupRatio {
				ratio, ok := ratio_setting.GetGroupGroupRatio(group, g)
				if ok {
					groupRatio[g] = ratio
				}
			}
		}
	}

	usableGroup = service.GetUserUsableGroups(group)
	pricing = filterPricingByUsableGroups(pricing, usableGroup)
	runningHubH3Prices, h3PricingPlan := resolveRunningHubH3PricingForUser(usableGroup, runningHubH3PriceGroup)
	// check groupRatio contains usableGroup
	for group := range ratio_setting.GetGroupRatioCopy() {
		if _, ok := usableGroup[group]; !ok {
			delete(groupRatio, group)
		}
	}

	c.JSON(200, gin.H{
		"success":                            true,
		"data":                               pricing,
		"vendors":                            model.GetVendors(),
		"group_ratio":                        groupRatio,
		"usable_group":                       usableGroup,
		"supported_endpoint":                 model.GetSupportedEndpointMap(),
		"auto_groups":                        service.GetUserAutoGroup(group),
		"runninghub_h3_group_prices":         runningHubH3Prices,
		"runninghub_h3_group_time_discounts": resolveRunningHubH3GroupTimeDiscounts(usableGroup, time.Now()),
		"h3_pricing_plan":                    h3PricingPlan,
		"pricing_version":                    "a42d372ccf0b5dd13ecf71203521f9d2",
	})
}

func ResetModelRatio(c *gin.Context) {
	defaultStr := ratio_setting.DefaultModelRatio2JSONString()
	err := model.UpdateOption("ModelRatio", defaultStr)
	if err != nil {
		c.JSON(200, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	err = ratio_setting.UpdateModelRatioByJSONString(defaultStr)
	if err != nil {
		c.JSON(200, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(200, gin.H{
		"success": true,
		"message": "重置模型倍率成功",
	})
}
