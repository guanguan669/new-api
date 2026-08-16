package ratio_setting

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync/atomic"
	"time"
)

const RunningHubH3GroupTimeDiscountOptionKey = "RunningHubH3GroupTimeDiscount"

var shanghaiLocation = time.FixedZone("Asia/Shanghai", 8*60*60)

type RunningHubH3GroupTimeDiscount struct {
	Start    string  `json:"start"`
	End      string  `json:"end"`
	Discount float64 `json:"discount"`
}

type RunningHubH3GroupTimeDiscountState struct {
	Multiplier   float64 `json:"multiplier"`
	NextChangeAt int64   `json:"next_change_at"`
}

type h3TimeDiscountWindow struct {
	startMinute int
	endMinute   int
	discount    float64
}

type h3TimeDiscountSnapshot struct {
	raw     map[string][]RunningHubH3GroupTimeDiscount
	windows map[string][]h3TimeDiscountWindow
}

var runningHubH3GroupTimeDiscountSnapshot atomic.Pointer[h3TimeDiscountSnapshot]

func init() {
	runningHubH3GroupTimeDiscountSnapshot.Store(&h3TimeDiscountSnapshot{
		raw:     map[string][]RunningHubH3GroupTimeDiscount{},
		windows: map[string][]h3TimeDiscountWindow{},
	})
}

func RunningHubH3GroupTimeDiscount2JSONString() string {
	value, _ := json.Marshal(GetRunningHubH3GroupTimeDiscountCopy())
	return string(value)
}

func GetRunningHubH3GroupTimeDiscountCopy() map[string][]RunningHubH3GroupTimeDiscount {
	snapshot := runningHubH3GroupTimeDiscountSnapshot.Load()
	result := make(map[string][]RunningHubH3GroupTimeDiscount, len(snapshot.raw))
	for group, discounts := range snapshot.raw {
		result[group] = append([]RunningHubH3GroupTimeDiscount(nil), discounts...)
	}
	return result
}

func ValidateRunningHubH3GroupTimeDiscountJSON(jsonStr string) error {
	_, err := parseRunningHubH3GroupTimeDiscountJSON(jsonStr)
	return err
}

func UpdateRunningHubH3GroupTimeDiscountByJSONString(jsonStr string) error {
	snapshot, err := parseRunningHubH3GroupTimeDiscountJSON(jsonStr)
	if err != nil {
		return err
	}
	runningHubH3GroupTimeDiscountSnapshot.Store(snapshot)
	return nil
}

func ResolveRunningHubH3GroupTimeDiscount(group string, now time.Time) RunningHubH3GroupTimeDiscountState {
	group = strings.TrimSpace(group)
	state := RunningHubH3GroupTimeDiscountState{Multiplier: 1}
	if group == "" {
		return state
	}
	windows := runningHubH3GroupTimeDiscountSnapshot.Load().windows[group]
	if len(windows) == 0 {
		return state
	}

	localNow := now.In(shanghaiLocation)
	minute := localNow.Hour()*60 + localNow.Minute()
	state.Multiplier = discountAtMinute(windows, minute)
	dayStart := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, shanghaiLocation)
	boundaries := make([]int, 0, len(windows)*2)
	for _, window := range windows {
		boundaries = append(boundaries, window.startMinute, window.endMinute)
	}
	sort.Ints(boundaries)
	for dayOffset := 0; dayOffset <= 1; dayOffset++ {
		for _, boundary := range boundaries {
			if dayOffset == 0 && boundary <= minute {
				continue
			}
			candidateMinute := boundary + dayOffset*24*60
			before := discountAtMinute(windows, (candidateMinute-1+24*60)%(24*60))
			after := discountAtMinute(windows, candidateMinute%(24*60))
			if before == after {
				continue
			}
			state.NextChangeAt = dayStart.Add(time.Duration(candidateMinute) * time.Minute).Unix()
			return state
		}
	}
	return state
}

func parseRunningHubH3GroupTimeDiscountJSON(jsonStr string) (*h3TimeDiscountSnapshot, error) {
	var input map[string][]RunningHubH3GroupTimeDiscount
	if err := json.Unmarshal([]byte(jsonStr), &input); err != nil {
		return nil, err
	}
	if input == nil {
		return nil, errors.New("h3 group time discount must be a JSON object")
	}
	snapshot := &h3TimeDiscountSnapshot{
		raw:     make(map[string][]RunningHubH3GroupTimeDiscount, len(input)),
		windows: make(map[string][]h3TimeDiscountWindow, len(input)),
	}
	for group, discounts := range input {
		trimmedGroup := strings.TrimSpace(group)
		if trimmedGroup == "" {
			return nil, errors.New("h3 group time discount group cannot be empty")
		}
		if trimmedGroup != group {
			return nil, fmt.Errorf("h3 group time discount group cannot have surrounding whitespace: %q", group)
		}
		if !ContainsGroupRatio(group) {
			return nil, fmt.Errorf("h3 group time discount group is not configured in GroupRatio: %s", group)
		}
		windows := make([]h3TimeDiscountWindow, 0, len(discounts))
		segments := make([][2]int, 0, len(discounts)*2)
		for index, discount := range discounts {
			start, err := parseDailyMinute(discount.Start)
			if err != nil {
				return nil, fmt.Errorf("h3 group time discount %s[%d] start: %w", group, index, err)
			}
			end, err := parseDailyMinute(discount.End)
			if err != nil {
				return nil, fmt.Errorf("h3 group time discount %s[%d] end: %w", group, index, err)
			}
			if start == end {
				return nil, fmt.Errorf("h3 group time discount %s[%d] start and end cannot be equal", group, index)
			}
			if math.IsNaN(discount.Discount) || math.IsInf(discount.Discount, 0) || discount.Discount < 0 || discount.Discount > 1 {
				return nil, fmt.Errorf("h3 group time discount %s[%d] discount must be between 0 and 1", group, index)
			}
			windows = append(windows, h3TimeDiscountWindow{startMinute: start, endMinute: end, discount: discount.Discount})
			if start < end {
				segments = append(segments, [2]int{start, end})
			} else {
				segments = append(segments, [2]int{start, 24 * 60})
				if end > 0 {
					segments = append(segments, [2]int{0, end})
				}
			}
		}
		sort.Slice(segments, func(i, j int) bool { return segments[i][0] < segments[j][0] })
		for index := 1; index < len(segments); index++ {
			if segments[index][0] < segments[index-1][1] {
				return nil, fmt.Errorf("h3 group time discount windows overlap: %s", group)
			}
		}
		snapshot.raw[group] = append([]RunningHubH3GroupTimeDiscount(nil), discounts...)
		snapshot.windows[group] = windows
	}
	return snapshot, nil
}

func parseDailyMinute(value string) (int, error) {
	if len(value) != 5 || value[2] != ':' {
		return 0, fmt.Errorf("must use HH:MM")
	}
	parsed, err := time.Parse("15:04", value)
	if err != nil {
		return 0, fmt.Errorf("must use HH:MM")
	}
	return parsed.Hour()*60 + parsed.Minute(), nil
}

func discountAtMinute(windows []h3TimeDiscountWindow, minute int) float64 {
	for _, window := range windows {
		if window.startMinute < window.endMinute {
			if minute >= window.startMinute && minute < window.endMinute {
				return window.discount
			}
			continue
		}
		if minute >= window.startMinute || minute < window.endMinute {
			return window.discount
		}
	}
	return 1
}
