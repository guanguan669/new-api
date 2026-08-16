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
  getOverlappingRuleIndexes,
  isValidH3TimeDiscountMap,
} from '../h3-time-discount'

describe('H3 time discount validation', () => {
  test('accepts multiple non-overlapping rules and overnight ranges', () => {
    assert.equal(
      isValidH3TimeDiscountMap({
        minimaxh3: [
          { start: '22:00', end: '02:00', discount: 0.8 },
          { start: '08:00', end: '12:00', discount: 0.9 },
        ],
      }),
      true
    )
  })

  test('reports both rules when an overnight range overlaps after midnight', () => {
    const overlaps = getOverlappingRuleIndexes([
      { start: '22:00', end: '02:00', discount: 0.8 },
      { start: '01:00', end: '03:00', discount: 0.9 },
    ])
    assert.deepEqual([...overlaps], [0, 1])
  })

  test('accepts zero for free pricing', () => {
    assert.equal(
      isValidH3TimeDiscountMap({
        minimaxh3: [{ start: '08:00', end: '10:00', discount: 0 }],
      }),
      true
    )
  })

  test('rejects equal times, out-of-range multipliers, and overlaps', () => {
    assert.equal(
      isValidH3TimeDiscountMap({
        minimaxh3: [{ start: '08:00', end: '08:00', discount: 0.8 }],
      }),
      false
    )
    assert.equal(
      isValidH3TimeDiscountMap({
        minimaxh3: [{ start: '08:00', end: '10:00', discount: -0.1 }],
      }),
      false
    )
    assert.equal(
      isValidH3TimeDiscountMap({
        minimaxh3: [{ start: '08:00', end: '10:00', discount: 1.1 }],
      }),
      false
    )
    assert.equal(
      isValidH3TimeDiscountMap({
        minimaxh3: [
          { start: '08:00', end: '10:00', discount: 0.8 },
          { start: '09:00', end: '11:00', discount: 0.9 },
        ],
      }),
      false
    )
  })
})
