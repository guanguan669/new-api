package ratio_setting

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"unicode/utf8"
)

const RunningHubH3GroupPriceOptionKey = "RunningHubH3GroupPrice"

const h3PricePlanNoneSentinel = "__none__"

type RunningHubH3GroupPrice struct {
	Price768P  float64 `json:"price_768p"`
	Price2K    float64 `json:"price_2k"`
	BoundGroup string  `json:"group,omitempty"`
}

type runningHubH3GroupPriceInput struct {
	Price768P  *float64 `json:"price_768p"`
	Price2K    *float64 `json:"price_2k"`
	BoundGroup *string  `json:"group"`
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
	runningHubH3GroupPriceMap.Replace(prices)
	return nil
}

func ValidateRunningHubH3GroupPriceJSON(jsonStr string) error {
	_, err := parseRunningHubH3GroupPriceJSON(jsonStr)
	return err
}

func ParseRunningHubH3GroupPriceJSON(jsonStr string) (map[string]RunningHubH3GroupPrice, error) {
	return parseRunningHubH3GroupPriceJSON(jsonStr)
}

func parseRunningHubH3GroupPriceJSON(jsonStr string) (map[string]RunningHubH3GroupPrice, error) {
	inputs := make(map[string]runningHubH3GroupPriceInput)
	if err := json.Unmarshal([]byte(jsonStr), &inputs); err != nil {
		return nil, err
	}

	prices := make(map[string]RunningHubH3GroupPrice, len(inputs))
	for plan, price := range inputs {
		trimmedPlan := strings.TrimSpace(plan)
		if trimmedPlan == "" {
			return nil, errors.New("h3 price plan name cannot be empty")
		}
		if trimmedPlan != plan {
			return nil, fmt.Errorf("h3 price plan name cannot have surrounding whitespace: %q", plan)
		}
		if utf8.RuneCountInString(plan) > 64 {
			return nil, fmt.Errorf("h3 price plan name is too long: %s", plan)
		}
		if plan == "auto" || plan == h3PricePlanNoneSentinel {
			return nil, fmt.Errorf("h3 price plan cannot use reserved name: %s", plan)
		}
		if price.Price768P == nil {
			return nil, fmt.Errorf("h3 price plan missing price_768p: %s", plan)
		}
		if price.Price2K == nil {
			return nil, fmt.Errorf("h3 price plan missing price_2k: %s", plan)
		}
		if err := validateRunningHubH3GroupPriceValue("price_768p", plan, *price.Price768P); err != nil {
			return nil, err
		}
		if err := validateRunningHubH3GroupPriceValue("price_2k", plan, *price.Price2K); err != nil {
			return nil, err
		}
		boundGroup := ""
		if price.BoundGroup != nil {
			boundGroup = strings.TrimSpace(*price.BoundGroup)
			if boundGroup == "" {
				return nil, fmt.Errorf("h3 price plan group cannot be empty: %s", plan)
			}
			if !ContainsGroupRatio(boundGroup) {
				return nil, fmt.Errorf("h3 price plan group is not configured in GroupRatio: %s", boundGroup)
			}
		} else if ContainsGroupRatio(plan) {
			boundGroup = plan
		}
		prices[plan] = RunningHubH3GroupPrice{
			Price768P:  *price.Price768P,
			Price2K:    *price.Price2K,
			BoundGroup: boundGroup,
		}
	}
	return prices, nil
}

func validateRunningHubH3GroupPriceValue(field, plan string, value float64) error {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return fmt.Errorf("h3 price plan %s must be finite: %s", field, plan)
	}
	if value < 0 {
		return fmt.Errorf("h3 price plan %s must be not less than 0: %s", field, plan)
	}
	return nil
}
