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
import { Network, Shield, Code2, Activity } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Reveal } from './reveal'

const PILLARS = [
  {
    icon: <Network className='size-6' strokeWidth={1.5} />,
    title: 'Intelligent Routing',
    titleZh: '智能调度',
    points: [
      'Auto-select optimal channel',
      'Load balancing',
      'Cost-optimized routing',
    ],
    pointsZh: ['自动选择最优渠道', '负载均衡', '成本优化路由'],
  },
  {
    icon: <Shield className='size-6' strokeWidth={1.5} />,
    title: 'Failover',
    titleZh: '故障切换',
    points: [
      'Millisecond failover',
      'Multi-active architecture',
      'Zero-downtime migration',
    ],
    pointsZh: ['毫秒级 failover', '多活架构', '零停机迁移'],
  },
  {
    icon: <Code2 className='size-6' strokeWidth={1.5} />,
    title: 'Unified API',
    titleZh: '统一接口',
    points: [
      'OpenAI compatible',
      'One integration for all models',
      'SDK support',
    ],
    pointsZh: ['OpenAI 兼容', '一次接入全模型', 'SDK 支持'],
  },
  {
    icon: <Activity className='size-6' strokeWidth={1.5} />,
    title: 'Real-Time Monitoring',
    titleZh: '实时监控',
    points: [
      'Request tracing',
      'Performance dashboard',
      'Anomaly alerts',
    ],
    pointsZh: ['调用链路追踪', '性能仪表盘', '异常告警'],
  },
]

export function ProductPillars() {
  const { t } = useTranslation()

  return (
    <section className='border-t border-slate-200 px-5 py-16 sm:px-8 sm:py-20 lg:px-12 lg:py-24'>
      <div className='mx-auto max-w-6xl'>
        <Reveal className='mb-10 text-center sm:mb-12 lg:mb-14'>
          <p className='mb-2 text-xs font-semibold uppercase tracking-widest text-slate-500'>
            {t('home.pillars.label')}
          </p>
          <h2 className='text-2xl font-semibold tracking-tight text-slate-900 sm:text-3xl lg:text-4xl'>
            {t('home.pillars.title')}
          </h2>
        </Reveal>

        <div className='grid grid-cols-1 gap-6 sm:grid-cols-2 lg:grid-cols-4'>
          {PILLARS.map((pillar, i) => (
            <Reveal key={pillar.title} delay={(i % 4) * 90}>
              <div className='group flex h-full flex-col rounded-2xl border border-slate-200 bg-white p-6 shadow-sm transition-all duration-300 hover:border-slate-300 hover:shadow-md'>
                <div className='mb-4 flex size-12 items-center justify-center rounded-xl bg-slate-100 text-slate-700 transition-colors duration-300 group-hover:bg-cyan-500/10 group-hover:text-cyan-600'>
                  {pillar.icon}
                </div>
                <h3 className='mb-1 text-lg font-semibold text-slate-900'>
                  {t(pillar.title)}
                </h3>
                <p className='mb-4 text-sm text-slate-500'>{t(pillar.titleZh)}</p>
                <ul className='mt-auto space-y-2'>
                  {pillar.points.map((point) => (
                    <li
                      key={point}
                      className='flex items-start gap-2 text-sm text-slate-700'
                    >
                      <span className='mt-1.5 size-1 shrink-0 rounded-full bg-cyan-500' />
                      <span>{t(point)}</span>
                    </li>
                  ))}
                </ul>
              </div>
            </Reveal>
          ))}
        </div>
      </div>
    </section>
  )
}
