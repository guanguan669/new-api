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
import { AnimatePresence, motion, useReducedMotion } from 'motion/react'
import {
  Activity,
  ArrowUpRight,
  Check,
  ChevronRight,
  CircleDot,
  Gauge,
  Layers3,
  Play,
  Route,
  Sparkles,
  Zap,
} from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Link } from '@tanstack/react-router'

import { cn } from '@/lib/utils'

import { Reveal } from './reveal'

interface RouteModel {
  id: string
  name: string
  vendor: string
  latency: number
  successRate: string
  context: string
  color: string
  softColor: string
  capabilities: string[]
  route: string[]
  tokens: number
}

const ROUTE_MODELS: RouteModel[] = [
  {
    id: 'atlas',
    name: 'GPT-5.4',
    vendor: 'OpenAI',
    latency: 142,
    successRate: '99.98%',
    context: '128K context',
    color: '#2563eb',
    softColor: 'bg-blue-500/10 text-blue-700 ring-blue-500/20',
    capabilities: ['Tool use', 'Vision', 'Code'],
    route: ['Gateway', 'Policy check', 'OpenAI / primary'],
    tokens: 842,
  },
  {
    id: 'reasoning',
    name: 'Claude Opus 5',
    vendor: 'Anthropic',
    latency: 168,
    successRate: '99.95%',
    context: '200K context',
    color: '#d97706',
    softColor: 'bg-amber-500/10 text-amber-700 ring-amber-500/20',
    capabilities: ['Reasoning', 'Vision', 'Long context'],
    route: ['Gateway', 'Budget guard', 'Anthropic / failover'],
    tokens: 916,
  },
  {
    id: 'fast',
    name: 'Gemini 3.0',
    vendor: 'Google',
    latency: 93,
    successRate: '99.99%',
    context: '1M context',
    color: '#059669',
    softColor: 'bg-emerald-500/10 text-emerald-700 ring-emerald-500/20',
    capabilities: ['Multimodal', 'Fast', 'Long context'],
    route: ['Gateway', 'Latency route', 'Google / edge'],
    tokens: 734,
  },
]

function Stat({
  icon: Icon,
  label,
  value,
}: {
  icon: typeof Activity
  label: string
  value: string
}) {
  return (
    <div className='flex items-center gap-2.5'>
      <Icon className='size-4 text-slate-400' strokeWidth={1.8} />
      <div className='min-w-0'>
        <p className='text-[10px] font-medium uppercase tracking-[0.12em] text-slate-400'>
          {label}
        </p>
        <p className='mt-0.5 truncate text-sm font-semibold text-slate-800'>
          {value}
        </p>
      </div>
    </div>
  )
}

export function RoutingLab() {
  const { t } = useTranslation()
  const prefersReducedMotion = useReducedMotion()
  const [activeId, setActiveId] = useState(ROUTE_MODELS[0].id)
  const [isRunning, setIsRunning] = useState(false)
  const [pointer, setPointer] = useState({ x: 50, y: 50 })
  const activeModel =
    ROUTE_MODELS.find((model) => model.id === activeId) ?? ROUTE_MODELS[0]

  const runRequest = () => {
    setIsRunning(true)
    window.setTimeout(() => setIsRunning(false), 1600)
  }

  return (
    <section className='relative overflow-hidden border-y border-slate-200 bg-[#F5F7FA] px-5 py-16 sm:px-8 sm:py-20 lg:px-12 lg:py-24'>
      <div className='pointer-events-none absolute inset-0 bg-[linear-gradient(to_right,rgba(148,163,184,0.11)_1px,transparent_1px),linear-gradient(to_bottom,rgba(148,163,184,0.11)_1px,transparent_1px)] [mask-image:linear-gradient(to_bottom,black,transparent_88%)] bg-[size:3.5rem_3.5rem]' />
      <div className='relative mx-auto max-w-6xl'>
        <Reveal className='mb-10 flex flex-col gap-4 sm:mb-12 sm:flex-row sm:items-end sm:justify-between'>
          <div className='max-w-2xl'>
            <div className='mb-3 inline-flex items-center gap-2 rounded-full border border-cyan-500/20 bg-white px-3 py-1.5 text-[10px] font-semibold uppercase tracking-[0.16em] text-cyan-700 shadow-sm'>
              <Sparkles className='size-3.5' />
              <span>{t('Live routing lab')}</span>
            </div>
            <h2 className='text-2xl font-semibold tracking-tight text-slate-900 sm:text-3xl lg:text-4xl'>
              {t('Route every request with intent.')}
            </h2>
            <p className='mt-3 max-w-xl text-sm leading-relaxed text-slate-600 sm:text-base'>
              {t(
                'See how policy, latency, and model capability work together before a request reaches the provider.'
              )}
            </p>
          </div>
          <Link
            to='/dashboard'
            className='group inline-flex shrink-0 items-center gap-2 text-sm font-semibold text-slate-700 transition-colors hover:text-cyan-700'
          >
            {t('Explore routing controls')}
            <ArrowUpRight className='size-4 transition-transform group-hover:-translate-y-0.5 group-hover:translate-x-0.5' />
          </Link>
        </Reveal>

        <div className='grid gap-5 lg:grid-cols-[0.9fr_1.1fr]'>
          <Reveal variant='fade-right' className='h-full'>
            <div className='h-full rounded-2xl border border-slate-200 bg-white p-5 shadow-[0_16px_40px_-28px_rgba(15,23,42,0.35)] sm:p-6'>
              <div className='mb-5 flex items-start justify-between gap-4'>
                <div>
                  <p className='text-sm font-semibold text-slate-900'>
                    {t('Choose a route')}
                  </p>
                  <p className='mt-1 text-xs text-slate-500'>
                    {t('Switch the target without changing your API.')}
                  </p>
                </div>
                <div className='flex size-9 items-center justify-center rounded-xl bg-slate-100 text-slate-600'>
                  <Route className='size-4' />
                </div>
              </div>

              <div className='space-y-2'>
                {ROUTE_MODELS.map((model) => {
                  const isActive = model.id === activeId
                  return (
                    <button
                      key={model.id}
                      type='button'
                      onClick={() => setActiveId(model.id)}
                      aria-pressed={isActive}
                      className={cn(
                        'group relative flex w-full items-center gap-3 overflow-hidden rounded-xl border p-3 text-left transition-all duration-200',
                        isActive
                          ? 'border-slate-300 bg-slate-50 shadow-sm'
                          : 'border-transparent hover:border-slate-200 hover:bg-slate-50/70'
                      )}
                    >
                      {isActive && (
                        <motion.span
                          layoutId='route-active-indicator'
                          className='absolute inset-y-2 left-0 w-0.5 rounded-full bg-cyan-500'
                          transition={{ duration: prefersReducedMotion ? 0 : 0.2 }}
                        />
                      )}
                      <span
                        className='flex size-9 shrink-0 items-center justify-center rounded-lg text-white'
                        style={{ backgroundColor: model.color }}
                      >
                        <CircleDot className='size-4' />
                      </span>
                      <span className='min-w-0 flex-1'>
                        <span className='flex items-center gap-2'>
                          <span className='truncate text-sm font-semibold text-slate-900'>
                            {model.name}
                          </span>
                          {isActive && (
                            <span className='rounded-full bg-emerald-500/10 px-1.5 py-0.5 text-[9px] font-semibold uppercase tracking-wide text-emerald-700'>
                              {t('Active')}
                            </span>
                          )}
                        </span>
                        <span className='mt-0.5 block text-xs text-slate-500'>
                          {model.vendor} · {model.context}
                        </span>
                      </span>
                      <ChevronRight
                        className={cn(
                          'size-4 shrink-0 text-slate-300 transition-transform',
                          isActive && 'translate-x-0.5 text-slate-500'
                        )}
                      />
                    </button>
                  )
                })}
              </div>

              <div className='mt-5 grid grid-cols-2 gap-3 border-t border-slate-100 pt-5'>
                <Stat
                  icon={Gauge}
                  label={t('Median latency')}
                  value={`${activeModel.latency} ms`}
                />
                <Stat
                  icon={Activity}
                  label={t('Success rate')}
                  value={activeModel.successRate}
                />
              </div>
            </div>
          </Reveal>

          <Reveal variant='fade-left' className='h-full'>
            <div
              className='group relative h-full min-h-[360px] overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-[0_16px_40px_-28px_rgba(15,23,42,0.35)]'
              onPointerMove={(event) => {
                const rect = event.currentTarget.getBoundingClientRect()
                setPointer({
                  x: ((event.clientX - rect.left) / rect.width) * 100,
                  y: ((event.clientY - rect.top) / rect.height) * 100,
                })
              }}
              style={{
                backgroundImage: `radial-gradient(circle at ${pointer.x}% ${pointer.y}%, rgba(34, 211, 238, 0.14), transparent 32%), linear-gradient(135deg, #ffffff 0%, #f8fafc 100%)`,
              }}
            >
              <div className='relative flex h-full flex-col p-5 sm:p-6'>
                <div className='flex items-center justify-between gap-4'>
                  <div className='flex items-center gap-2'>
                    <span className='relative flex size-2'>
                      <span className='absolute inline-flex h-full w-full animate-ping rounded-full bg-emerald-400 opacity-60' />
                      <span className='relative inline-flex size-2 rounded-full bg-emerald-500' />
                    </span>
                    <span className='text-xs font-semibold uppercase tracking-[0.14em] text-slate-500'>
                      {t('Request simulator')}
                    </span>
                  </div>
                  <span className='rounded-full border border-emerald-500/20 bg-emerald-500/10 px-2 py-1 text-[10px] font-semibold text-emerald-700'>
                    {isRunning ? t('Routing') : t('200 OK')}
                  </span>
                </div>

                <div className='mt-8 flex items-center gap-3'>
                  <motion.div
                    animate={prefersReducedMotion ? undefined : { rotate: 360 }}
                    transition={{ duration: 8, repeat: Infinity, ease: 'linear' }}
                    className='flex size-12 items-center justify-center rounded-2xl bg-slate-900 text-white shadow-lg shadow-slate-900/10'
                  >
                    <Zap className='size-5 fill-cyan-300 text-cyan-300' />
                  </motion.div>
                  <div>
                    <p className='text-sm font-semibold text-slate-900'>
                      POST /v1/chat/completions
                    </p>
                    <p className='mt-1 text-xs text-slate-500'>
                      {t('Policy matched · target selected automatically')}
                    </p>
                  </div>
                </div>

                <div className='mt-7 space-y-3'>
                  <div className='flex items-center gap-3 text-xs text-slate-500'>
                    <span className='w-20 shrink-0 font-medium'>{t('Route')}</span>
                    <div className='flex min-w-0 flex-1 items-center gap-2'>
                      {activeModel.route.map((step, index) => (
                        <span
                          key={step}
                          className='flex min-w-0 items-center gap-2'
                        >
                          <span className='truncate rounded-md border border-slate-200 bg-white px-2 py-1 font-medium text-slate-700 shadow-sm'>
                            {step}
                          </span>
                          {index < activeModel.route.length - 1 && (
                            <ChevronRight className='size-3 shrink-0 text-slate-300' />
                          )}
                        </span>
                      ))}
                    </div>
                  </div>
                  <div className='flex items-center gap-3 text-xs text-slate-500'>
                    <span className='w-20 shrink-0 font-medium'>{t('Signals')}</span>
                    <div className='flex flex-wrap gap-1.5'>
                      {activeModel.capabilities.map((capability) => (
                        <span
                          key={capability}
                          className={cn(
                            'rounded-full px-2 py-1 text-[10px] font-semibold ring-1',
                            activeModel.softColor
                          )}
                        >
                          {capability}
                        </span>
                      ))}
                    </div>
                  </div>
                </div>

                <div className='mt-auto border-t border-slate-200/80 pt-5'>
                  <div className='mb-4 grid grid-cols-3 gap-3'>
                    <div>
                      <p className='text-[10px] uppercase tracking-[0.12em] text-slate-400'>
                        {t('Target')}
                      </p>
                      <AnimatePresence mode='wait'>
                        <motion.p
                          key={activeModel.id}
                          initial={
                            prefersReducedMotion ? undefined : { opacity: 0, y: 5 }
                          }
                          animate={{ opacity: 1, y: 0 }}
                          exit={
                            prefersReducedMotion ? undefined : { opacity: 0, y: -5 }
                          }
                          className='mt-1 truncate text-sm font-semibold text-slate-800'
                        >
                          {activeModel.name}
                        </motion.p>
                      </AnimatePresence>
                    </div>
                    <div>
                      <p className='text-[10px] uppercase tracking-[0.12em] text-slate-400'>
                        {t('Latency')}
                      </p>
                      <p className='mt-1 text-sm font-semibold text-slate-800'>
                        {activeModel.latency} ms
                      </p>
                    </div>
                    <div>
                      <p className='text-[10px] uppercase tracking-[0.12em] text-slate-400'>
                        {t('Tokens')}
                      </p>
                      <AnimatePresence mode='wait'>
                        <motion.p
                          key={`${activeModel.id}-${isRunning}`}
                          initial={
                            prefersReducedMotion ? undefined : { opacity: 0, y: 5 }
                          }
                          animate={{ opacity: 1, y: 0 }}
                          exit={
                            prefersReducedMotion ? undefined : { opacity: 0, y: -5 }
                          }
                          className='mt-1 text-sm font-semibold text-slate-800'
                        >
                          {isRunning ? activeModel.tokens + 18 : activeModel.tokens}
                        </motion.p>
                      </AnimatePresence>
                    </div>
                  </div>

                  <button
                    type='button'
                    onClick={runRequest}
                    className='group relative inline-flex h-11 w-full items-center justify-center gap-2 overflow-hidden rounded-xl bg-slate-900 px-5 text-sm font-semibold text-white shadow-lg shadow-slate-900/10 transition-transform hover:-translate-y-0.5 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cyan-500 focus-visible:ring-offset-2'
                  >
                    {!prefersReducedMotion && (
                      <motion.span
                        aria-hidden
                        animate={{ x: ['-120%', '120%'] }}
                        transition={{
                          duration: 2.2,
                          repeat: Infinity,
                          ease: 'linear',
                        }}
                        className='pointer-events-none absolute inset-y-0 w-1/3 -skew-x-12 bg-gradient-to-r from-transparent via-white/25 to-transparent'
                      />
                    )}
                    <Play className='relative size-4 fill-current' />
                    <span className='relative'>
                      {isRunning ? t('Request in flight') : t('Run live request')}
                    </span>
                  </button>
                </div>
              </div>
            </div>
          </Reveal>
        </div>

        <Reveal className='mt-5 flex flex-wrap items-center justify-center gap-x-5 gap-y-2 text-xs text-slate-500'>
          <span className='inline-flex items-center gap-1.5'>
            <Check className='size-3.5 text-emerald-600' />
            {t('OpenAI-compatible')}
          </span>
          <span className='inline-flex items-center gap-1.5'>
            <Layers3 className='size-3.5 text-cyan-600' />
            {t('Policy-aware routing')}
          </span>
          <span className='inline-flex items-center gap-1.5'>
            <Activity className='size-3.5 text-blue-600' />
            {t('Live health signals')}
          </span>
        </Reveal>
      </div>
    </section>
  )
}
