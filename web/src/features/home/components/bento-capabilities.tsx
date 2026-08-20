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
import { motion, useReducedMotion } from 'motion/react'
import {
  Activity,
  ArrowUpRight,
  Coins,
  Gauge,
  KeyRound,
  Network,
  ShieldCheck,
  SlidersHorizontal,
  Waypoints,
} from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Link } from '@tanstack/react-router'

import { cn } from '@/lib/utils'

import { Reveal } from './reveal'

const ROUTES = [
  { name: 'GPT-5.4', provider: 'OpenAI', latency: '142 ms', health: '99.98%', tone: 'bg-blue-500' },
  { name: 'Claude Opus 5', provider: 'Anthropic', latency: '168 ms', health: '99.95%', tone: 'bg-amber-500' },
  { name: 'Gemini 3.0', provider: 'Google', latency: '93 ms', health: '99.99%', tone: 'bg-emerald-500' },
]

const PILLARS = [
  { icon: SlidersHorizontal, title: 'home.redesign.bento.policyTitle', desc: 'home.redesign.bento.policyDesc', tone: 'text-cyan-600 bg-cyan-500/10' },
  { icon: ShieldCheck, title: 'home.redesign.bento.securityTitle', desc: 'home.redesign.bento.securityDesc', tone: 'text-emerald-600 bg-emerald-500/10' },
  { icon: Coins, title: 'home.redesign.bento.costTitle', desc: 'home.redesign.bento.costDesc', tone: 'text-amber-600 bg-amber-500/10' },
  { icon: Waypoints, title: 'home.redesign.bento.failoverTitle', desc: 'home.redesign.bento.failoverDesc', tone: 'text-violet-600 bg-violet-500/10' },
]

function SectionIntro() {
  const { t } = useTranslation()

  return (
    <Reveal className='mb-10 max-w-2xl sm:mb-12'>
      <p className='mb-3 text-xs font-semibold uppercase tracking-[0.16em] text-primary'>
        {t('home.redesign.bento.eyebrow')}
      </p>
      <h2 className='text-3xl font-semibold tracking-[-0.03em] text-foreground sm:text-4xl'>
        {t('home.redesign.bento.title')}
      </h2>
      <p className='mt-3 text-sm leading-6 text-muted-foreground sm:text-base'>
        {t('home.redesign.bento.subtitle')}
      </p>
    </Reveal>
  )
}

export function BentoCapabilities() {
  const { t } = useTranslation()
  const prefersReducedMotion = useReducedMotion()
  const [activeRoute, setActiveRoute] = useState(0)
  const route = ROUTES[activeRoute]

  return (
    <section className='border-b border-border/70 bg-background px-5 py-16 sm:px-8 sm:py-20 lg:px-12 lg:py-24'>
      <div className='mx-auto max-w-7xl'>
        <SectionIntro />
        <div className='grid auto-rows-[minmax(13rem,auto)] grid-cols-1 gap-3 md:grid-cols-2 lg:grid-cols-12'>
          <Reveal className='lg:col-span-7 lg:row-span-2'>
            <div className='group relative flex h-full min-h-[27rem] flex-col overflow-hidden rounded-xl border border-border bg-card p-5 shadow-sm sm:p-7'>
              <div className='pointer-events-none absolute -right-20 -top-20 size-64 rounded-full bg-cyan-500/10 blur-3xl transition-opacity group-hover:opacity-80' />
              <div className='relative flex items-start justify-between gap-4'>
                <div>
                  <p className='text-sm font-semibold text-foreground'>{t('home.redesign.bento.routingTitle')}</p>
                  <p className='mt-1 max-w-md text-xs leading-5 text-muted-foreground'>
                    {t('home.redesign.bento.routingDesc')}
                  </p>
                </div>
                <span className='flex size-9 items-center justify-center rounded-lg bg-primary/10 text-primary'>
                  <Network className='size-4' />
                </span>
              </div>

              <div className='relative mt-8 flex flex-1 flex-col justify-center'>
                <div className='absolute left-7 top-7 bottom-7 w-px bg-border' />
                <div className='space-y-4'>
                  {ROUTES.map((item, index) => {
                    const isActive = index === activeRoute
                    return (
                      <button
                        key={item.name}
                        type='button'
                        onClick={() => setActiveRoute(index)}
                        aria-pressed={isActive}
                        className={cn(
                          'relative z-10 flex w-full items-center gap-3 rounded-xl border px-3 py-3 text-left transition-all',
                          isActive
                            ? 'border-primary/30 bg-primary/5 shadow-sm'
                            : 'border-transparent hover:border-border hover:bg-muted/60'
                        )}
                      >
                        <span className={cn('flex size-8 shrink-0 items-center justify-center rounded-full ring-4 ring-card', item.tone)}>
                          <span className='size-2 rounded-full bg-background/90' />
                        </span>
                        <span className='min-w-0 flex-1'>
                          <span className='flex items-center gap-2'>
                            <span className='truncate text-sm font-semibold text-foreground'>{item.name}</span>
                            {isActive && <span className='rounded-full bg-emerald-500/10 px-1.5 py-0.5 text-[9px] font-semibold uppercase text-emerald-600 dark:text-emerald-400'>{t('home.redesign.bento.active')}</span>}
                          </span>
                          <span className='mt-0.5 block text-xs text-muted-foreground'>{item.provider}</span>
                        </span>
                        <span className='shrink-0 text-right'>
                          <span className='block text-xs font-semibold text-foreground'>{item.latency}</span>
                          <span className='block text-[10px] text-muted-foreground'>{item.health}</span>
                        </span>
                      </button>
                    )
                  })}
                </div>
              </div>

              <div className='relative mt-5 flex items-center justify-between border-t border-border pt-4 text-xs'>
                <span className='inline-flex items-center gap-1.5 text-muted-foreground'><Activity className='size-3.5 text-emerald-500' />{t('home.redesign.bento.healthLabel')}</span>
                <span className='font-semibold text-foreground'>{route.name} · {route.latency}</span>
              </div>
            </div>
          </Reveal>

          <Reveal delay={80} className='lg:col-span-5'>
            <div className='relative flex h-full min-h-[13rem] flex-col overflow-hidden rounded-xl border border-border bg-card p-5 shadow-sm sm:p-6'>
              <div className='flex items-start justify-between gap-4'>
                <div>
                  <p className='text-sm font-semibold text-foreground'>{t('home.redesign.bento.observabilityTitle')}</p>
                  <p className='mt-1 text-xs text-muted-foreground'>{t('home.redesign.bento.observabilityDesc')}</p>
                </div>
                <Gauge className='size-5 text-primary' />
              </div>
              <div className='mt-auto pt-8'>
                <div className='flex items-end justify-between'>
                  <div><span className='text-3xl font-semibold tracking-tight text-foreground'>99.99</span><span className='ml-1 text-sm font-medium text-muted-foreground'>%</span></div>
                  <span className='text-xs font-medium text-emerald-600 dark:text-emerald-400'>+0.18%</span>
                </div>
                <div className='mt-3 flex h-10 items-end gap-1'>
                  {[34, 45, 38, 62, 52, 72, 68, 82, 75, 92, 88, 96].map((height, index) => (
                    <motion.span
                      key={index}
                      initial={prefersReducedMotion ? false : { height: 0 }}
                      whileInView={{ height: `${height}%` }}
                      viewport={{ once: true, amount: 0.7 }}
                      transition={{ duration: 0.45, delay: index * 0.03 }}
                      className='min-w-0 flex-1 rounded-t-sm bg-primary/20'
                    />
                  ))}
                </div>
              </div>
            </div>
          </Reveal>

          <Reveal delay={140} className='lg:col-span-5'>
            <div className='flex h-full min-h-[13rem] flex-col rounded-xl border border-border bg-card p-5 shadow-sm sm:p-6'>
              <div className='flex items-start justify-between gap-4'>
                <div>
                  <p className='text-sm font-semibold text-foreground'>{t('home.redesign.bento.keysTitle')}</p>
                  <p className='mt-1 text-xs text-muted-foreground'>{t('home.redesign.bento.keysDesc')}</p>
                </div>
                <KeyRound className='size-5 text-primary' />
              </div>
              <div className='mt-auto grid grid-cols-3 gap-2 pt-7'>
                {['prod_live', 'staging', 'research'].map((keyName, index) => (
                  <div key={keyName} className='rounded-lg border border-border bg-muted/40 p-2.5'>
                    <div className='mb-3 flex items-center justify-between'><span className='size-1.5 rounded-full bg-emerald-500' /><span className='text-[9px] text-muted-foreground'>0{index + 1}</span></div>
                    <p className='truncate font-mono text-[10px] text-foreground'>{keyName}</p>
                    <p className='mt-1 text-[9px] text-muted-foreground'>{t('home.redesign.bento.scoped')}</p>
                  </div>
                ))}
              </div>
            </div>
          </Reveal>

          {PILLARS.map((pillar, index) => {
            const Icon = pillar.icon
            return (
              <Reveal key={pillar.title} delay={index * 70} className='lg:col-span-3'>
                <div className='flex h-full min-h-[11rem] flex-col rounded-xl border border-border bg-card p-5 shadow-sm transition-colors hover:bg-muted/30'>
                  <span className={cn('flex size-9 items-center justify-center rounded-lg', pillar.tone)}><Icon className='size-4' /></span>
                  <p className='mt-5 text-sm font-semibold text-foreground'>{t(pillar.title)}</p>
                  <p className='mt-1 text-xs leading-5 text-muted-foreground'>{t(pillar.desc)}</p>
                </div>
              </Reveal>
            )
          })}
        </div>
        <Reveal className='mt-6 flex justify-end'>
          <Link to='/dashboard' className='group inline-flex items-center gap-1.5 text-sm font-semibold text-primary hover:underline underline-offset-4'>
            {t('home.redesign.bento.exploreDashboard')}
            <ArrowUpRight className='size-4 transition-transform group-hover:-translate-y-0.5 group-hover:translate-x-0.5' />
          </Link>
        </Reveal>
      </div>
    </section>
  )
}
