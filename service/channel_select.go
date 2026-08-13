package service

import (
	"errors"
	"fmt"
	"math/rand"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

var h3LoadReservationState = struct {
	sync.Mutex
	counts map[int]int64
}{counts: make(map[int]int64)}

type RetryParam struct {
	Ctx          *gin.Context
	TokenGroup   string
	ModelName    string
	RequestPath  string
	Retry        *int
	resetNextTry bool
}

func (p *RetryParam) GetRetry() int {
	if p.Retry == nil {
		return 0
	}
	return *p.Retry
}

func (p *RetryParam) SetRetry(retry int) {
	p.Retry = &retry
}

func (p *RetryParam) IncreaseRetry() {
	if p.resetNextTry {
		p.resetNextTry = false
		return
	}
	if p.Retry == nil {
		p.Retry = new(int)
	}
	*p.Retry++
}

func (p *RetryParam) ResetRetryNextTry() {
	p.resetNextTry = true
}

// CacheGetRandomSatisfiedChannel tries to get a random channel that satisfies the requirements.
// 尝试获取一个满足要求的随机渠道。
//
// For "auto" tokenGroup with cross-group Retry enabled:
// 对于启用了跨分组重试的 "auto" tokenGroup：
//
//   - Each group will exhaust all its priorities before moving to the next group.
//     每个分组会用完所有优先级后才会切换到下一个分组。
//
//   - Uses ContextKeyAutoGroupIndex to track current group index.
//     使用 ContextKeyAutoGroupIndex 跟踪当前分组索引。
//
//   - Uses ContextKeyAutoGroupRetryIndex to track the global Retry count when current group started.
//     使用 ContextKeyAutoGroupRetryIndex 跟踪当前分组开始时的全局重试次数。
//
//   - priorityRetry = Retry - startRetryIndex, represents the priority level within current group.
//     priorityRetry = Retry - startRetryIndex，表示当前分组内的优先级级别。
//
//   - When GetRandomSatisfiedChannel returns nil (priorities exhausted), moves to next group.
//     当 GetRandomSatisfiedChannel 返回 nil（优先级用完）时，切换到下一个分组。
//
// Example flow (2 groups, each with 2 priorities, RetryTimes=3):
// 示例流程（2个分组，每个有2个优先级，RetryTimes=3）：
//
//	Retry=0: GroupA, priority0 (startRetryIndex=0, priorityRetry=0)
//	         分组A, 优先级0
//
//	Retry=1: GroupA, priority1 (startRetryIndex=0, priorityRetry=1)
//	         分组A, 优先级1
//
//	Retry=2: GroupA exhausted → GroupB, priority0 (startRetryIndex=2, priorityRetry=0)
//	         分组A用完 → 分组B, 优先级0
//
//	Retry=3: GroupB, priority1 (startRetryIndex=2, priorityRetry=1)
//	         分组B, 优先级1
func CacheGetRandomSatisfiedChannel(param *RetryParam) (*model.Channel, string, error) {
	ReleaseH3LoadReservation(param.Ctx)

	var channel *model.Channel
	var err error
	selectGroup := param.TokenGroup
	userGroup := common.GetContextKeyString(param.Ctx, constant.ContextKeyUserGroup)

	if param.TokenGroup == "auto" {
		autoGroups := GetRequestAutoGroups(param.Ctx, userGroup)
		if len(autoGroups) == 0 {
			return nil, selectGroup, errors.New("auto groups is not enabled")
		}

		// startGroupIndex: the group index to start searching from
		// startGroupIndex: 开始搜索的分组索引
		startGroupIndex := 0
		crossGroupRetry := common.GetContextKeyBool(param.Ctx, constant.ContextKeyTokenCrossGroupRetry)

		if lastGroupIndex, exists := common.GetContextKey(param.Ctx, constant.ContextKeyAutoGroupIndex); exists {
			if idx, ok := lastGroupIndex.(int); ok {
				startGroupIndex = idx
			}
		}

		for i := startGroupIndex; i < len(autoGroups); i++ {
			autoGroup := autoGroups[i]
			// Calculate priorityRetry for current group
			// 计算当前分组的 priorityRetry
			priorityRetry := param.GetRetry()
			// If moved to a new group, reset priorityRetry and update startRetryIndex
			// 如果切换到新分组，重置 priorityRetry 并更新 startRetryIndex
			if i > startGroupIndex {
				priorityRetry = 0
			}
			logger.LogDebug(param.Ctx, "Auto selecting group: %s, priorityRetry: %d", autoGroup, priorityRetry)

			channel, _ = model.GetRandomSatisfiedChannel(autoGroup, param.ModelName, priorityRetry, param.RequestPath)
			if channel != nil {
				channel = selectLeastLoadedH3Channel(param.Ctx, autoGroup, param.ModelName, priorityRetry, param.RequestPath, channel)
			}
			if channel == nil {
				// Current group has no available channel for this model, try next group
				// 当前分组没有该模型的可用渠道，尝试下一个分组
				logger.LogDebug(param.Ctx, "No available channel in group %s for model %s at priorityRetry %d, trying next group", autoGroup, param.ModelName, priorityRetry)
				// 重置状态以尝试下一个分组
				common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroupIndex, i+1)
				common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroupRetryIndex, 0)
				// Reset retry counter so outer loop can continue for next group
				// 重置重试计数器，以便外层循环可以为下一个分组继续
				param.SetRetry(0)
				continue
			}
			common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroup, autoGroup)
			selectGroup = autoGroup
			logger.LogDebug(param.Ctx, "Auto selected group: %s", autoGroup)

			// Prepare state for next retry
			// 为下一次重试准备状态
			if crossGroupRetry && priorityRetry >= common.RetryTimes {
				// Current group has exhausted all retries, prepare to switch to next group
				// This request still uses current group, but next retry will use next group
				// 当前分组已用完所有重试次数，准备切换到下一个分组
				// 本次请求仍使用当前分组，但下次重试将使用下一个分组
				logger.LogDebug(param.Ctx, "Current group %s retries exhausted (priorityRetry=%d >= RetryTimes=%d), preparing switch to next group for next retry", autoGroup, priorityRetry, common.RetryTimes)
				common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroupIndex, i+1)
				// Reset retry counter so outer loop can continue for next group
				// 重置重试计数器，以便外层循环可以为下一个分组继续
				param.SetRetry(0)
				param.ResetRetryNextTry()
			} else {
				// Stay in current group, save current state
				// 保持在当前分组，保存当前状态
				common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroupIndex, i)
			}
			break
		}
	} else {
		channel, err = model.GetRandomSatisfiedChannel(param.TokenGroup, param.ModelName, param.GetRetry(), param.RequestPath)
		if err != nil {
			return nil, param.TokenGroup, err
		}
		if channel != nil {
			channel = selectLeastLoadedH3Channel(param.Ctx, param.TokenGroup, param.ModelName, param.GetRetry(), param.RequestPath, channel)
		}
	}
	return channel, selectGroup, nil
}

func selectLeastLoadedH3Channel(ctx *gin.Context, group, modelName string, retry int, requestPath string, selected *model.Channel) *model.Channel {
	if selected == nil || selected.Type != constant.ChannelTypeComfyUIH3 || ctx == nil {
		return selected
	}
	candidates, err := model.GetSatisfiedChannelsAtPriority(group, modelName, retry, requestPath)
	if err != nil || len(candidates) < 2 {
		return selected
	}
	for _, candidate := range candidates {
		if candidate == nil || candidate.Type != constant.ChannelTypeComfyUIH3 {
			return selected
		}
	}

	channelIDs := make([]int, 0, len(candidates))
	for _, candidate := range candidates {
		channelIDs = append(channelIDs, candidate.Id)
	}
	activeCounts, err := model.CountActiveTasksByChannel(channelIDs)
	if err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("failed to read ComfyUI H3 channel load; using normal weighted selection: %v", err))
		return selected
	}

	h3LoadReservationState.Lock()
	defer h3LoadReservationState.Unlock()
	minLoad := int64(-1)
	leastLoaded := make([]*model.Channel, 0, len(candidates))
	for _, candidate := range candidates {
		load := activeCounts[candidate.Id] + h3LoadReservationState.counts[candidate.Id]
		if minLoad == -1 || load < minLoad {
			minLoad = load
			leastLoaded = leastLoaded[:0]
			leastLoaded = append(leastLoaded, candidate)
			continue
		}
		if load == minLoad {
			leastLoaded = append(leastLoaded, candidate)
		}
	}
	chosen := weightedH3Candidate(leastLoaded)
	if chosen == nil {
		return selected
	}
	h3LoadReservationState.counts[chosen.Id]++
	common.SetContextKey(ctx, constant.ContextKeyH3LoadReservation, chosen.Id)
	return chosen
}

func weightedH3Candidate(channels []*model.Channel) *model.Channel {
	if len(channels) == 0 {
		return nil
	}
	if len(channels) == 1 {
		return channels[0]
	}
	totalWeight := 0
	for _, channel := range channels {
		weight := channel.GetWeight()
		if weight <= 0 {
			weight = 100
		}
		totalWeight += weight
	}
	choice := rand.Intn(totalWeight)
	for _, channel := range channels {
		weight := channel.GetWeight()
		if weight <= 0 {
			weight = 100
		}
		choice -= weight
		if choice < 0 {
			return channel
		}
	}
	return channels[len(channels)-1]
}

// ReleaseH3LoadReservation releases the temporary selection reservation after
// the upstream submission attempt has either produced a persisted task or
// failed. Persisted active tasks are counted by the next selection query.
func ReleaseH3LoadReservation(ctx *gin.Context) {
	if ctx == nil {
		return
	}
	channelID, ok := common.GetContextKeyType[int](ctx, constant.ContextKeyH3LoadReservation)
	if !ok || channelID <= 0 {
		return
	}
	h3LoadReservationState.Lock()
	if h3LoadReservationState.counts[channelID] <= 1 {
		delete(h3LoadReservationState.counts, channelID)
	} else {
		h3LoadReservationState.counts[channelID]--
	}
	h3LoadReservationState.Unlock()
	common.SetContextKey(ctx, constant.ContextKeyH3LoadReservation, 0)
}
