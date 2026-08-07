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
import { useTranslation } from 'react-i18next'

import { StaticDataTable } from '@/components/data-table'

import {
  formatRunningHubH3Price,
  RUNNING_HUB_H3_CLARITY_PRESETS,
  RUNNING_HUB_H3_GROUP,
  runningHubH3EffectiveGroup,
  runningHubH3PriceInUSD,
} from '../lib/runninghub-h3-pricing'
import type { PricingModel } from '../types'

type PriceDisplay = {
  platform: string
  cny: string
}

type PriceRow = {
  megapixels: number
  outputSize16By9: string
  perSecond: PriceDisplay
  seconds5: PriceDisplay
  seconds10: PriceDisplay
  seconds15: PriceDisplay
}

type ReferenceImageRow = {
  count: string
  surcharge: PriceDisplay | null
}

type RunningHubH3PricingProps = {
  model: PricingModel
  priceRate: number
  usdExchangeRate: number
  showRechargePrice?: boolean
}

function formatCNY(amount: number): string {
  if (!Number.isFinite(amount)) return '-'
  const digits = Math.abs(amount) < 1 ? 4 : 3
  return `¥${amount.toFixed(digits)}`
}

function PriceCell(props: { value: PriceDisplay }) {
  return (
    <div className='min-w-[4.5rem] text-right whitespace-nowrap'>
      <div className='font-mono text-sm font-semibold tabular-nums'>
        {props.value.platform}
      </div>
      <div className='text-muted-foreground text-[10px] tabular-nums'>
        {props.value.cny}
      </div>
    </div>
  )
}

export function RunningHubH3Pricing(props: RunningHubH3PricingProps) {
  const { t } = useTranslation()
  const selectedGroup = runningHubH3EffectiveGroup(props.model)
  const groupRatio = Number(props.model.group_ratio?.[selectedGroup] ?? 1)
  const cnyRate = props.showRechargePrice
    ? props.priceRate
    : props.usdExchangeRate

  const priceFor = (seconds: number, megapixels: number): PriceDisplay => {
    const priceInUSD = runningHubH3PriceInUSD(
      props.model,
      seconds,
      megapixels,
      selectedGroup
    )
    return {
      platform: formatRunningHubH3Price(props.model, seconds, megapixels, {
        showRechargePrice: props.showRechargePrice,
        priceRate: props.priceRate,
        usdExchangeRate: props.usdExchangeRate,
        selectedGroup,
      }),
      cny: formatCNY(priceInUSD * cnyRate),
    }
  }

  const priceRows: PriceRow[] = RUNNING_HUB_H3_CLARITY_PRESETS.map(
    (preset) => ({
      ...preset,
      perSecond: priceFor(1, preset.megapixels),
      seconds5: priceFor(5, preset.megapixels),
      seconds10: priceFor(10, preset.megapixels),
      seconds15: priceFor(15, preset.megapixels),
    })
  )

  const referenceImageRows: ReferenceImageRow[] = [
    { count: '0-5', surcharge: null },
    ...[6, 7, 8, 9].map((count) => ({
      count: String(count),
      surcharge: priceFor(count - 5, 1),
    })),
  ]

  return (
    <div className='space-y-5'>
      <div className='space-y-1.5'>
        <p className='text-foreground text-sm font-semibold'>
          {t('H3 video pricing')}
        </p>
        <p className='text-muted-foreground text-sm leading-relaxed'>
          {t(
            'Video requests are billed by requested duration and clarity. The price in the first line follows the current platform display; the second line is a CNY reference.'
          )}
        </p>
        <p className='text-muted-foreground text-xs leading-relaxed'>
          {t('Current group: {{group}} ({{ratio}}x)', {
            group: RUNNING_HUB_H3_GROUP,
            ratio: groupRatio,
          })}
        </p>
        <p className='text-muted-foreground text-xs leading-relaxed'>
          {t(
            'Clarity 1.0 is the base rate. Lower tiers use clarity × 1.1; higher tiers use clarity × 1.5.'
          )}
        </p>
      </div>

      <div className='overflow-x-auto border-y'>
        <StaticDataTable
          className='min-w-[760px] rounded-none border-0'
          tableClassName='text-sm'
          headerRowClassName='hover:bg-transparent'
          data={priceRows}
          getRowKey={(row) => String(row.megapixels)}
          columns={[
            {
              id: 'clarity',
              header: t('Clarity'),
              className:
                'text-muted-foreground py-2 text-[10px] font-medium tracking-wider uppercase',
              cellClassName: 'py-2.5 font-mono tabular-nums',
              cell: (row) =>
                row.megapixels.toFixed(row.megapixels % 1 === 0 ? 1 : 2),
            },
            {
              id: 'output',
              header: t('Approx. 16:9 output'),
              className:
                'text-muted-foreground py-2 text-[10px] font-medium tracking-wider uppercase',
              cellClassName: 'py-2.5 font-mono text-xs tabular-nums',
              cell: (row) => row.outputSize16By9,
            },
            {
              id: 'per-second',
              header: t('Price / second'),
              className:
                'text-muted-foreground py-2 text-right text-[10px] font-medium tracking-wider uppercase',
              cellClassName: 'py-2.5',
              cell: (row) => <PriceCell value={row.perSecond} />,
            },
            {
              id: '5-seconds',
              header: '5s',
              className:
                'text-muted-foreground py-2 text-right text-[10px] font-medium tracking-wider uppercase',
              cellClassName: 'py-2.5',
              cell: (row) => <PriceCell value={row.seconds5} />,
            },
            {
              id: '10-seconds',
              header: '10s',
              className:
                'text-muted-foreground py-2 text-right text-[10px] font-medium tracking-wider uppercase',
              cellClassName: 'py-2.5',
              cell: (row) => <PriceCell value={row.seconds10} />,
            },
            {
              id: '15-seconds',
              header: '15s',
              className:
                'text-muted-foreground py-2 text-right text-[10px] font-medium tracking-wider uppercase',
              cellClassName: 'py-2.5',
              cell: (row) => <PriceCell value={row.seconds15} />,
            },
          ]}
        />
      </div>

      <div className='space-y-2'>
        <p className='text-foreground text-sm font-semibold'>
          {t('Image-to-video reference images')}
        </p>
        <div className='overflow-x-auto border-y'>
          <StaticDataTable
            className='min-w-[360px] rounded-none border-0'
            tableClassName='text-sm'
            headerRowClassName='hover:bg-transparent'
            data={referenceImageRows}
            getRowKey={(row) => row.count}
            columns={[
              {
                id: 'count',
                header: t('Reference images'),
                className:
                  'text-muted-foreground py-2 text-[10px] font-medium tracking-wider uppercase',
                cellClassName: 'py-2.5 font-mono tabular-nums',
                cell: (row) => row.count,
              },
              {
                id: 'surcharge',
                header: t('Cumulative added fee'),
                className:
                  'text-muted-foreground py-2 text-right text-[10px] font-medium tracking-wider uppercase',
                cellClassName: 'py-2.5',
                cell: (row) =>
                  row.surcharge ? (
                    <PriceCell value={row.surcharge} />
                  ) : (
                    <span className='text-muted-foreground block text-right text-sm'>
                      {t('Included')}
                    </span>
                  ),
              },
            ]}
          />
        </div>
      </div>

      <ul className='text-muted-foreground space-y-1 text-xs leading-relaxed'>
        <li>
          {t(
            'The first five reference images are included; each image from the sixth through ninth adds the listed one-time fee.'
          )}
        </li>
        <li>
          {t(
            'The same clarity tier has the same price for 1:1, 2:3, 3:2, 3:4, 4:3, 9:16, 16:9, and 21:9 outputs.'
          )}
        </li>
        <li>
          {t(
            'Up to three reference audio files have no additional charge. Automatic OOM splitting and merging does not add a second charge.'
          )}
        </li>
      </ul>
    </div>
  )
}
