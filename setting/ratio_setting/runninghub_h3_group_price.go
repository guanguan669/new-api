package ratio_setting

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
)

const RunningHubH3GroupPriceOptionKey = "RunningHubH3GroupPrice"

type RunningHubH3GroupPrice struct {
	Price768P float64 `json:"price_768p"`
	Price2K   float64 `json:"price_2k"`
}

type runningHubH3GroupPriceInput struct {
	Price768P *float64 `json:"price_768p"`
	Price2K   *float64 `json:"price_2k"`
}

func RunningHubH3GroupPrice2JSONString() string {
	return runningHubH3GroupPriceMap.MarshalJSONString()
}

func GetRunningHubH3GroupPrice(group string) (RunningHubH3GroupPrice, bool) {
	return runningHubH3GroupPriceMap.Get(group)
}

func GetRunningHubH3GroupPriceCopy() map[string]RunningHubH3GroupPrice {
	return runningHubH3GroupPriceMap.ReadAll()
}

func UpdateRunningHubH3GroupPriceByJSONString(jsonStr string) error {
	prices, err := parseRunningHubH3GroupPriceJSON(jsonStr)
	if err != nil {
		return err
	}
	runningHubH3GroupPriceMap.Clear()
	runningHubH3GroupPriceMap.AddAll(prices)
	return nil
}

func ValidateRunningHubH3GroupPriceJSON(jsonStr string) error {
	_, err := parseRunningHubH3GroupPriceJSON(jsonStr)
	return err
}

func parseRunningHubH3GroupPriceJSON(jsonStr string) (map[string]RunningHubH3GroupPrice, error) {
	inputs := make(map[string]runningHubH3GroupPriceInput)
	if err := json.Unmarshal([]byte(jsonStr), &inputs); err != nil {
		return nil, err
	}

	prices := make(map[string]RunningHubH3GroupPrice, len(inputs))
	for group, price := range inputs {
		if group == "auto" {
			return nil, errors.New("runninghub h3 group price cannot be configured for auto group")
		}
		if !ContainsGroupRatio(group) {
			return nil, fmt.Errorf("runninghub h3 group price group must exist in GroupRatio: %s", group)
		}
		if price.Price768P == nil {
			return nil, fmt.Errorf("runninghub h3 group price missing price_768p: %s", group)
		}
		if price.Price2K == nil {
			return nil, fmt.Errorf("runninghub h3 group price missing price_2k: %s", group)
		}
		if err := validateRunningHubH3GroupPriceValue("price_768p", group, *price.Price768P); err != nil {
			return nil, err
		}
		if err := validateRunningHubH3GroupPriceValue("price_2k", group, *price.Price2K); err != nil {
			return nil, err
		}
		prices[group] = RunningHubH3GroupPrice{
			Price768P: *price.Price768P,
			Price2K:   *price.Price2K,
		}
	}
	return prices, nil
}

func validateRunningHubH3GroupPriceValue(field, group string, value float64) error {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return fmt.Errorf("runninghub h3 group price %s must be finite: %s", field, group)
	}
	if value < 0 {
		return fmt.Errorf("runninghub h3 group price %s must be not less than 0: %s", field, group)
	}
	return nil
}
