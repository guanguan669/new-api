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
/* oxlint-disable react/no-array-index-key -- persisted rules have no IDs */
import { Plus, Trash2 } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

import { safeJsonParse } from '../utils/json-parser'
import {
  getOverlappingRuleIndexes,
  timeToMinutes,
  type H3TimeDiscountMap,
  type H3TimeDiscountRule,
} from './h3-time-discount'

type H3TimeDiscountEditorProps = {
  value: string
  groupOptions: string[]
  onChange: (value: string) => void
}

const DEFAULT_RULE: H3TimeDiscountRule = {
  start: '00:00',
  end: '08:00',
  discount: 0.8,
}

export function H3TimeDiscountEditor(props: H3TimeDiscountEditorProps) {
  const { t } = useTranslation()
  const schedules = useMemo(
    () =>
      safeJsonParse<H3TimeDiscountMap>(props.value, {
        fallback: {},
        silent: true,
      }),
    [props.value]
  )
  const configuredGroups = Object.keys(schedules)
  const [selectedGroup, setSelectedGroup] = useState('')
  const activeGroup = props.groupOptions.includes(selectedGroup)
    ? selectedGroup
    : configuredGroups.find((group) => props.groupOptions.includes(group)) ||
      props.groupOptions[0] ||
      ''
  const rules = useMemo(
    () => schedules[activeGroup] ?? [],
    [activeGroup, schedules]
  )
  const overlapIndexes = useMemo(
    () => getOverlappingRuleIndexes(rules),
    [rules]
  )

  const emit = (next: H3TimeDiscountMap) =>
    props.onChange(JSON.stringify(next, null, 2))

  const updateRule = (index: number, patch: Partial<H3TimeDiscountRule>) => {
    const nextRules = rules.map((rule, ruleIndex) =>
      ruleIndex === index ? { ...rule, ...patch } : rule
    )
    emit({ ...schedules, [activeGroup]: nextRules })
  }

  const addRule = () => {
    if (!activeGroup) return
    emit({ ...schedules, [activeGroup]: [...rules, { ...DEFAULT_RULE }] })
  }

  const removeRule = (index: number) => {
    const nextRules = rules.filter((_, ruleIndex) => ruleIndex !== index)
    const nextSchedules = { ...schedules }
    if (nextRules.length === 0) delete nextSchedules[activeGroup]
    else nextSchedules[activeGroup] = nextRules
    emit(nextSchedules)
  }

  return (
    <Card>
      <CardHeader className='bg-muted/20 border-b'>
        <div className='flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between'>
          <div>
            <CardTitle>{t('H3 peak and valley discounts')}</CardTitle>
            <CardDescription>
              {t(
                'Set daily H3 price multipliers for an existing service group in Asia/Shanghai time. Overnight ranges are supported.'
              )}
            </CardDescription>
          </div>
          <div className='flex flex-wrap gap-2'>
            <Select
              value={activeGroup || undefined}
              onValueChange={(group) => {
                if (typeof group === 'string') setSelectedGroup(group)
              }}
            >
              <SelectTrigger
                className='w-48'
                aria-label={t('H3 discount service group')}
              >
                <SelectValue placeholder={t('Select a service group')} />
              </SelectTrigger>
              <SelectContent alignItemWithTrigger={false}>
                <SelectGroup>
                  {props.groupOptions.map((group) => (
                    <SelectItem key={group} value={group}>
                      {group}
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
            <Button
              type='button'
              size='sm'
              disabled={!activeGroup}
              onClick={addRule}
            >
              <Plus className='mr-2 h-4 w-4' />
              {t('Add time rule')}
            </Button>
          </div>
        </div>
      </CardHeader>
      <CardContent className='space-y-3'>
        {rules.map((rule, index) => {
          const invalidTime =
            timeToMinutes(rule.start) === null ||
            timeToMinutes(rule.end) === null ||
            rule.start === rule.end
          const invalidDiscount =
            !Number.isFinite(Number(rule.discount)) ||
            rule.discount < 0 ||
            rule.discount > 1
          const hasOverlap = overlapIndexes.has(index)
          return (
            <div
              key={`${activeGroup}-${index}`}
              className='space-y-2 rounded-md border p-3'
            >
              <div className='grid gap-3 sm:grid-cols-[1fr_1fr_1fr_auto] sm:items-end'>
                <label className='space-y-1 text-sm'>
                  <span className='font-medium'>{t('Start time')}</span>
                  <Input
                    type='time'
                    value={rule.start}
                    aria-invalid={invalidTime || hasOverlap}
                    onChange={(event) =>
                      updateRule(index, { start: event.target.value })
                    }
                  />
                </label>
                <label className='space-y-1 text-sm'>
                  <span className='font-medium'>{t('End time')}</span>
                  <Input
                    type='time'
                    value={rule.end}
                    aria-invalid={invalidTime || hasOverlap}
                    onChange={(event) =>
                      updateRule(index, { end: event.target.value })
                    }
                  />
                </label>
                <label className='space-y-1 text-sm'>
                  <span className='font-medium'>{t('Price multiplier')}</span>
                  <Input
                    type='number'
                    min={0}
                    max={1}
                    step={0.01}
                    value={rule.discount}
                    aria-invalid={invalidDiscount}
                    onChange={(event) =>
                      updateRule(index, {
                        discount: Number(event.target.value),
                      })
                    }
                  />
                </label>
                <Button
                  type='button'
                  variant='ghost'
                  size='icon'
                  aria-label={t('Remove H3 discount time rule')}
                  onClick={() => removeRule(index)}
                >
                  <Trash2 className='h-4 w-4' />
                </Button>
              </div>
              {invalidTime && (
                <p className='text-destructive text-sm'>
                  {t('Start and end must be different valid times.')}
                </p>
              )}
              {invalidDiscount && (
                <p className='text-destructive text-sm'>
                  {t(
                    'Multiplier must be between 0 and 1. For example, 0 is free and 0.8 means 20% off.'
                  )}
                </p>
              )}
              {hasOverlap && (
                <p className='text-destructive text-sm'>
                  {t(
                    'This time rule overlaps another rule for the same service group.'
                  )}
                </p>
              )}
            </div>
          )
        })}
        {activeGroup && rules.length === 0 && (
          <p className='text-muted-foreground text-sm'>
            {t('No time discounts configured for this service group.')}
          </p>
        )}
        {!activeGroup && (
          <p className='text-muted-foreground text-sm'>
            {t('Add a service group before configuring H3 time discounts.')}
          </p>
        )}
      </CardContent>
    </Card>
  )
}
