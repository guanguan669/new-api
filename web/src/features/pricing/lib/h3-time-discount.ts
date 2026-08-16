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
import type {
  RunningHubH3GroupPrice,
  RunningHubH3TimeDiscountStatus,
} from '../types'

const MIN_REFRESH_DELAY_MS = 250
const MAX_TIMEOUT_MS = 2_147_000_000
const DEFAULT_H3_PRICE_768P = 0.1
const DEFAULT_H3_PRICE_2K = 0.3

export function applyRunningHubH3TimeDiscounts(
  prices: Record<string, RunningHubH3GroupPrice>,
  discounts: Record<string, RunningHubH3TimeDiscountStatus>
): Record<string, RunningHubH3GroupPrice> {
  const discountedPrices = Object.fromEntries(
    Object.entries(prices).map(([plan, price]) => {
      const serviceGroup = price.group?.trim() || plan
      const configuredMultiplier = Number(discounts[serviceGroup]?.multiplier)
      const multiplier =
        Number.isFinite(configuredMultiplier) &&
        configuredMultiplier >= 0 &&
        configuredMultiplier <= 1
          ? configuredMultiplier
          : 1
      return [
        plan,
        {
          ...price,
          price_768p: price.price_768p * multiplier,
          price_2k: price.price_2k * multiplier,
        },
      ]
    })
  )

  for (const [serviceGroup, status] of Object.entries(discounts)) {
    if (Object.hasOwn(discountedPrices, serviceGroup)) continue
    const multiplier = Number(status.multiplier)
    if (!Number.isFinite(multiplier) || multiplier < 0 || multiplier > 1) {
      continue
    }
    discountedPrices[serviceGroup] = {
      group: serviceGroup,
      price_768p: DEFAULT_H3_PRICE_768P * multiplier,
      price_2k: DEFAULT_H3_PRICE_2K * multiplier,
    }
  }

  return discountedPrices
}

export function getH3DiscountRefreshDelay(
  discounts: Record<string, RunningHubH3TimeDiscountStatus>,
  nowMs = Date.now()
): number | null {
  const futureBoundaries = Object.values(discounts)
    .map((status) => Number(status.next_change_at) * 1000)
    .filter((boundary) => Number.isFinite(boundary) && boundary > nowMs)

  if (futureBoundaries.length === 0) return null
  const delay = Math.min(...futureBoundaries) - nowMs + MIN_REFRESH_DELAY_MS
  return Math.min(Math.max(delay, MIN_REFRESH_DELAY_MS), MAX_TIMEOUT_MS)
}
