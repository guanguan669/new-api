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
import { useInView } from '../hooks/use-in-view'
import { useCountUp } from '../hooks/use-count-up'

const METRICS = [
  {
    value: '100+',
    labelKey: 'models',
    descKey: 'modelsDesc',
    gradient: 'from-cyan-600 to-blue-600',
  },
  {
    value: '99.9%',
    labelKey: 'uptime',
    descKey: 'uptimeDesc',
    gradient: 'from-emerald-600 to-green-600',
  },
  {
    value: '100K+',
    labelKey: 'qps',
    descKey: 'qpsDesc',
    gradient: 'from-purple-600 to-pink-600',
  },
  {
    value: '40%',
    labelKey: 'cost',
    descKey: 'costDesc',
    gradient: 'from-orange-600 to-red-600',
  },
]

function MetricCard({
  metric,
  inView,
}: {
  metric: (typeof METRICS)[number]
  inView: boolean
}) {
  const { t } = useTranslation()
  const displayValue = useCountUp(metric.value, inView)

  return (
    <div className='group relative overflow-hidden rounded-2xl border border-slate-200 bg-white p-6 text-center shadow-sm transition-all duration-300 hover:border-slate-300 hover:shadow-md'>
      <div
        className={`pointer-events-none absolute -top-10 left-1/2 h-24 w-24 -translate-x-1/2 rounded-full bg-gradient-to-br ${metric.gradient} opacity-15 blur-2xl transition-opacity duration-500 group-hover:opacity-25`}
      />
      <div
        className={`mb-2 bg-gradient-to-br ${metric.gradient} bg-clip-text text-4xl font-bold tracking-tighter text-transparent sm:text-5xl`}
      >
        {displayValue}
      </div>
      <div className='mb-1 text-sm font-medium text-slate-700'>
        {t(`home.metrics.${metric.labelKey}`)}
      </div>
      <div className='text-xs text-slate-500'>
        {t(`home.metrics.${metric.descKey}`)}
      </div>
    </div>
  )
}

export function MetricsBar() {
  const { ref, inView } = useInView<HTMLDivElement>()

  return (
    <section className='border-t border-slate-200 px-5 py-16 sm:px-8 sm:py-20 lg:px-12 lg:py-24'>
      <div className='mx-auto max-w-6xl'>
        <div
          ref={ref}
          className='grid grid-cols-2 gap-4 md:grid-cols-4'
        >
          {METRICS.map((metric) => (
            <MetricCard key={metric.labelKey} metric={metric} inView={inView} />
          ))}
        </div>
      </div>
    </section>
  )
}
