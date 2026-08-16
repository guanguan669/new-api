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

export type H3TimeDiscountRule = {
  start: string
  end: string
  discount: number
}

export type H3TimeDiscountMap = Record<string, H3TimeDiscountRule[]>

const TIME_PATTERN = /^(?:[01]\d|2[0-3]):[0-5]\d$/
const MINUTES_PER_DAY = 24 * 60

export function timeToMinutes(value: string): number | null {
  if (!TIME_PATTERN.test(value)) return null
  const [hours, minutes] = value.split(':').map(Number)
  return hours * 60 + minutes
}

function ruleSegments(rule: H3TimeDiscountRule): Array<[number, number]> {
  const start = timeToMinutes(rule.start)
  const end = timeToMinutes(rule.end)
  if (start === null || end === null || start === end) return []
  if (start < end) return [[start, end]]
  return [
    [start, MINUTES_PER_DAY],
    [0, end],
  ]
}

export function rulesOverlap(
  left: H3TimeDiscountRule,
  right: H3TimeDiscountRule
): boolean {
  return ruleSegments(left).some(([leftStart, leftEnd]) =>
    ruleSegments(right).some(
      ([rightStart, rightEnd]) => leftStart < rightEnd && rightStart < leftEnd
    )
  )
}

export function getOverlappingRuleIndexes(
  rules: H3TimeDiscountRule[]
): Set<number> {
  const indexes = new Set<number>()
  for (let left = 0; left < rules.length; left += 1) {
    for (let right = left + 1; right < rules.length; right += 1) {
      if (rulesOverlap(rules[left], rules[right])) {
        indexes.add(left)
        indexes.add(right)
      }
    }
  }
  return indexes
}

export function isValidH3TimeDiscountMap(value: unknown): boolean {
  if (value === null || typeof value !== 'object' || Array.isArray(value)) {
    return false
  }

  return Object.entries(value as Record<string, unknown>).every(
    ([group, rawRules]) => {
      if (!group.trim() || !Array.isArray(rawRules)) return false
      const rules = rawRules as H3TimeDiscountRule[]
      if (
        rules.some((rule) => {
          const start = timeToMinutes(rule?.start)
          const end = timeToMinutes(rule?.end)
          return (
            start === null ||
            end === null ||
            start === end ||
            !Number.isFinite(Number(rule?.discount)) ||
            Number(rule.discount) < 0 ||
            Number(rule.discount) > 1
          )
        })
      ) {
        return false
      }
      return getOverlappingRuleIndexes(rules).size === 0
    }
  )
}
