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

import { getRunningHubH3DisplayPrices } from '../lib/runninghub-h3-pricing'
import type { PricingModel, RunningHubH3GroupPrice } from '../types'

type RunningHubH3PricingProps = {
  model: PricingModel
  groupPrices?: Record<string, RunningHubH3GroupPrice>
  selectedGroup?: string
  pricingPlan?: string
}

export function RunningHubH3Pricing(props: RunningHubH3PricingProps) {
  const { t } = useTranslation()
  const prices = getRunningHubH3DisplayPrices(
    props.model,
    props.groupPrices,
    props.selectedGroup,
    props.pricingPlan
  )

  return (
    <div className='overflow-x-auto border-y'>
      <StaticDataTable
        className='min-w-[300px] rounded-none border-0'
        tableClassName='text-sm'
        headerRowClassName='hover:bg-transparent'
        data={prices}
        getRowKey={(row) => row.resolution}
        columns={[
          {
            id: 'resolution',
            header: t('Resolution'),
            className:
              'text-muted-foreground py-2 text-[10px] font-medium tracking-wider uppercase',
            cellClassName: 'py-2.5 font-medium',
            cell: (row) => row.resolution,
          },
          {
            id: 'price',
            header: t('Price'),
            className:
              'text-muted-foreground py-2 text-right text-[10px] font-medium tracking-wider uppercase',
            cellClassName:
              'py-2.5 text-right font-mono font-semibold tabular-nums',
            cell: (row) => row.price,
          },
        ]}
      />
    </div>
  )
}
