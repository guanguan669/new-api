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
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import {
  applyRunningHubH3TimeDiscounts,
  getH3DiscountRefreshDelay,
} from '../h3-time-discount'
import { getRunningHubH3DisplayPrices } from '../runninghub-h3-pricing'

describe('H3 active time discounts', () => {
  test('uses the pricing tier bound service group multiplier', () => {
    const prices = applyRunningHubH3TimeDiscounts(
      {
        vip_tier: {
          price_768p: 0.1,
          price_2k: 0.3,
          group: 'vip',
        },
      },
      { vip: { multiplier: 0.8, next_change_at: 2_000 } }
    )
    assert.equal(prices.vip_tier.group, 'vip')
    assert.ok(Math.abs(prices.vip_tier.price_768p - 0.08) < 0.000001)
    assert.ok(Math.abs(prices.vip_tier.price_2k - 0.24) < 0.000001)
  })

  test('falls back to the pricing map key and to multiplier one', () => {
    assert.deepEqual(
      applyRunningHubH3TimeDiscounts(
        { minimaxh3: { price_768p: 0.1, price_2k: 0.3 } },
        {}
      ),
      { minimaxh3: { price_768p: 0.1, price_2k: 0.3 } }
    )
  })

  test('supports free periods and generic H3 fallback prices', () => {
    const prices = applyRunningHubH3TimeDiscounts(
      {},
      { minimaxh3: { multiplier: 0, next_change_at: 2_000 } }
    )
    assert.deepEqual(prices, {
      minimaxh3: {
        group: 'minimaxh3',
        price_768p: 0,
        price_2k: 0,
      },
    })
    const displayPrices = getRunningHubH3DisplayPrices(
      {
        id: 1,
        model_name: 'minimax_h3',
        quota_type: 1,
        model_ratio: 1,
        completion_ratio: 1,
        enable_groups: ['minimaxh3'],
      },
      prices
    )
    assert.equal(displayPrices[0].price.startsWith('0.00'), true)
    assert.equal(displayPrices[1].price.startsWith('0.00'), true)
  })

  test('refreshes just after the nearest future boundary', () => {
    assert.equal(
      getH3DiscountRefreshDelay(
        {
          vip: { multiplier: 0.8, next_change_at: 1_010 },
          default: { multiplier: 1, next_change_at: 1_020 },
        },
        1_000_000
      ),
      10_250
    )
  })
})
