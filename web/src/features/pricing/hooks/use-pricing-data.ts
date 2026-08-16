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
import { useQuery } from '@tanstack/react-query'
import { useEffect, useMemo } from 'react'

import { useStatus } from '@/hooks/use-status'

import { getPricing } from '../api'
import {
  applyRunningHubH3TimeDiscounts,
  getH3DiscountRefreshDelay,
} from '../lib/h3-time-discount'

export function usePricingData() {
  const { status } = useStatus()

  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ['pricing'],
    queryFn: getPricing,
    staleTime: 5 * 60 * 1000,
  })

  // Ensure rates never reach zero to prevent division errors
  const priceRate = useMemo(
    () => Math.max((status?.price as number) ?? 1, 0.001),
    [status?.price]
  )
  const usdExchangeRate = useMemo(
    () => Math.max((status?.usd_exchange_rate as number) ?? priceRate, 0.001),
    [status?.usd_exchange_rate, priceRate]
  )

  const models = useMemo(() => {
    if (!data?.data || !data?.vendors) return []

    const vendorMap = new Map(data.vendors.map((v) => [v.id, v]))

    return data.data.map((model) => {
      const vendor = model.vendor_id
        ? vendorMap.get(model.vendor_id)
        : undefined
      return {
        ...model,
        key: model.model_name,
        vendor_name: vendor?.name,
        vendor_icon: vendor?.icon,
        vendor_description: vendor?.description,
        group_ratio: data.group_ratio,
      }
    })
  }, [data])

  const runningHubH3GroupTimeDiscounts = useMemo(
    () => data?.runninghub_h3_group_time_discounts ?? {},
    [data?.runninghub_h3_group_time_discounts]
  )
  const runningHubH3GroupPrices = useMemo(
    () =>
      applyRunningHubH3TimeDiscounts(
        data?.runninghub_h3_group_prices ?? {},
        runningHubH3GroupTimeDiscounts
      ),
    [data?.runninghub_h3_group_prices, runningHubH3GroupTimeDiscounts]
  )

  useEffect(() => {
    const delay = getH3DiscountRefreshDelay(runningHubH3GroupTimeDiscounts)
    if (delay === null) return undefined
    const timeout = window.setTimeout(() => void refetch(), delay)
    return () => window.clearTimeout(timeout)
  }, [refetch, runningHubH3GroupTimeDiscounts])

  return {
    models,
    vendors: data?.vendors ?? [],
    groupRatio: data?.group_ratio ?? {},
    usableGroup: data?.usable_group ?? {},
    endpointMap: data?.supported_endpoint ?? {},
    autoGroups: data?.auto_groups ?? [],
    runningHubH3GroupPrices,
    h3PricingPlan: data?.h3_pricing_plan?.trim() ?? '',
    isLoading,
    error,
    refetch,
    priceRate,
    usdExchangeRate,
  }
}
