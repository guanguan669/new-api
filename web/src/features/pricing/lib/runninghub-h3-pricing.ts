/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { formatBillingCurrencyFromUSD } from '@/lib/currency'

import type { PricingModel } from '../types'

export const RUNNING_HUB_H3_MODEL_NAME = 'minimax_h3'
export const RUNNING_HUB_H3_GROUP = 'minimaxh3'

export type RunningHubH3ClarityPreset = {
  megapixels: number
  outputSize16By9: string
}

export const RUNNING_HUB_H3_CLARITY_PRESETS: RunningHubH3ClarityPreset[] = [
  { megapixels: 0.2, outputSize16By9: '608 x 352' },
  { megapixels: 0.3, outputSize16By9: '736 x 416' },
  { megapixels: 0.4, outputSize16By9: '864 x 480' },
  { megapixels: 0.5, outputSize16By9: '960 x 544' },
  { megapixels: 0.6, outputSize16By9: '1056 x 608' },
  { megapixels: 0.7, outputSize16By9: '1152 x 640' },
  { megapixels: 0.8, outputSize16By9: '1216 x 672' },
  { megapixels: 0.9, outputSize16By9: '1280 x 736' },
  { megapixels: 0.98, outputSize16By9: '1344 x 768' },
  { megapixels: 1, outputSize16By9: '1376 x 768' },
  { megapixels: 1.2, outputSize16By9: '1504 x 832' },
  { megapixels: 1.5, outputSize16By9: '1664 x 928' },
  { megapixels: 1.8, outputSize16By9: '1824 x 1024' },
  { megapixels: 2, outputSize16By9: '1920 x 1088' },
]

export function isRunningHubH3Model(model: Pick<PricingModel, 'model_name'>) {
  return model.model_name === RUNNING_HUB_H3_MODEL_NAME
}

export function runningHubH3ClarityMultiplier(megapixels: number): number {
  if (Math.abs(megapixels - 1) < 0.000001) return 1
  return megapixels > 1 ? megapixels * 1.5 : megapixels * 1.1
}

export function runningHubH3EffectiveGroup(
  model: PricingModel,
  selectedGroup?: string
): string {
  const enabledGroups = Array.isArray(model.enable_groups)
    ? model.enable_groups
    : []

  if (selectedGroup && enabledGroups.includes(selectedGroup)) {
    return selectedGroup
  }

  if (enabledGroups.includes(RUNNING_HUB_H3_GROUP)) {
    return RUNNING_HUB_H3_GROUP
  }

  const configuredGroup = enabledGroups.find((group) =>
    Number.isFinite(Number(model.group_ratio?.[group]))
  )
  if (configuredGroup) return configuredGroup

  if (enabledGroups.length > 0) return enabledGroups[0]

  return RUNNING_HUB_H3_GROUP
}

export function runningHubH3PriceInUSD(
  model: PricingModel,
  seconds: number,
  megapixels: number,
  group?: string
): number {
  const groupRatio = Number(
    model.group_ratio?.[group ?? RUNNING_HUB_H3_GROUP] ?? 1
  )
  if (!Number.isFinite(groupRatio)) return 0
  return runningHubH3BasePriceInUSD(model, seconds, megapixels) * groupRatio
}

function runningHubH3BasePriceInUSD(
  model: PricingModel,
  seconds: number,
  megapixels: number
): number {
  const modelPrice = Number(model.model_price ?? 0)

  if (!Number.isFinite(modelPrice)) return 0

  return (
    modelPrice *
    Math.max(seconds, 0) *
    runningHubH3ClarityMultiplier(megapixels)
  )
}

export function formatRunningHubH3Price(
  model: PricingModel,
  seconds: number,
  megapixels: number,
  options: {
    showRechargePrice?: boolean
    priceRate?: number
    usdExchangeRate?: number
    selectedGroup?: string
  } = {}
): string {
  const group = runningHubH3EffectiveGroup(model, options.selectedGroup)
  const priceRate = options.priceRate ?? 1
  const usdExchangeRate = options.usdExchangeRate ?? 1
  let priceInUSD = runningHubH3PriceInUSD(model, seconds, megapixels, group)

  if (options.showRechargePrice) {
    priceInUSD = (priceInUSD * priceRate) / usdExchangeRate
  }

  return formatBillingCurrencyFromUSD(priceInUSD, {
    digitsLarge: 4,
    digitsSmall: 4,
    abbreviate: false,
  })
}
