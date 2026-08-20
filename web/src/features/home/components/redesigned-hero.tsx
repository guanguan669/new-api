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
import { ArrowRight, Check, Circle, Code2, ExternalLink, Route, Sparkles, Zap } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Link } from '@tanstack/react-router'

import { useStatus } from '@/hooks/use-status'
import { useAuthStore } from '@/stores/auth-store'

import { HeroTerminalDemo } from './hero-terminal-demo'
import { Reveal } from './reveal'

const ROUTE_STEPS = [
  { label: 'Client', detail: 'OpenAI SDK', icon: Code2 },
  { label: 'Gateway', detail: 'Policy engine', icon: Route },
  { label: 'Provider', detail: 'Best available', icon: Zap },
]

export function RedesignedHero() {
  const { t } = useTranslation()
  const { status } = useStatus()
  const { auth } = useAuthStore()
  const prefersReducedMotion = useReducedMotion()
  const isAuthenticated = !!auth.user
  const docsUrl = (status?.docs_link as string | undefined) || 'https://docs.newapi.pro'
  const primaryLabel = isAuthenticated ? t('Go to Dashboard') : t('Create API key')
  const primaryHref = isAuthenticated ? '/dashboard' : '/sign-up'

  return (
    <section className='relative overflow-hidden border-b border-border/70 bg-background px-5 pb-16 pt-28 sm:px-8 sm:pb-20 lg:px-12 lg:pb-24 lg:pt-36'>
      <div className='pointer-events-none absolute inset-0 bg-[linear-gradient(to_right,color-mix(in_oklch,var(--border)_45%,transparent)_1px,transparent_1px),linear-gradient(to_bottom,color-mix(in_oklch,var(--border)_45%,transparent)_1px,transparent_1px)] [mask-image:radial-gradient(ellipse_75%_70%_at_50%_0%,black_20%,transparent_100%)] bg-[size:4rem_4rem] opacity-40' />
      <div className='pointer-events-none absolute -top-40 left-1/2 h-[32rem] w-[48rem] -translate-x-1/2 rounded-full bg-cyan-500/10 blur-3xl dark:bg-cyan-400/5' />

      <div className='relative mx-auto grid max-w-7xl items-center gap-12 lg:grid-cols-[0.92fr_1.08fr] lg:gap-16'>
        <Reveal className='max-w-2xl'>
          <div className='mb-6 inline-flex items-center gap-2 rounded-full border border-primary/20 bg-primary/5 px-3 py-1.5 text-xs font-medium text-primary'>
            <Sparkles className='size-3.5' />
            <span>{t('home.redesign.eyebrow')}</span>
            <span className='text-muted-foreground/70'>·</span>
            <span className='text-muted-foreground'>{t('home.redesign.eyebrowMeta')}</span>
          </div>
          <h1 className='max-w-3xl text-4xl font-semibold leading-[1.04] tracking-[-0.04em] text-foreground sm:text-5xl lg:text-7xl'>
            {t('home.redesign.heroTitle')}
            <span className='block bg-gradient-to-r from-cyan-500 via-blue-600 to-violet-600 bg-clip-text text-transparent'>
              {t('home.redesign.heroAccent')}
            </span>
          </h1>
          <p className='mt-6 max-w-xl text-base leading-7 text-muted-foreground sm:text-lg'>
            {t('home.redesign.heroSubtitle')}
          </p>
          <div className='mt-8 flex flex-wrap items-center gap-3'>
            <Link
              to={primaryHref}
              className='group inline-flex h-11 items-center gap-2 rounded-lg bg-foreground px-5 text-sm font-semibold text-background transition-transform hover:-translate-y-0.5 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2'
            >
              {primaryLabel}
              <ArrowRight className='size-4 transition-transform group-hover:translate-x-0.5' />
            </Link>
            <a
              href={docsUrl}
              target='_blank'
              rel='noopener noreferrer'
              className='inline-flex h-11 items-center gap-2 rounded-lg border border-border bg-background px-5 text-sm font-semibold text-foreground transition-colors hover:bg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2'
            >
              {t('Read the docs')}
              <ExternalLink className='size-3.5 text-muted-foreground' />
            </a>
          </div>
          <div className='mt-9 flex flex-wrap gap-x-5 gap-y-2 text-xs text-muted-foreground'>
            {[t('home.redesign.heroProofOne'), t('home.redesign.heroProofTwo'), t('home.redesign.heroProofThree')].map((proof) => (
              <span key={proof} className='inline-flex items-center gap-1.5'>
                <Check className='size-3.5 text-emerald-500' />
                {proof}
              </span>
            ))}
          </div>
        </Reveal>

        <Reveal variant='fade-left' delay={120}>
          <div className='relative rounded-2xl border border-border bg-card p-2 shadow-[0_24px_80px_-32px_rgba(15,23,42,0.35)]'>
            <div className='rounded-xl border border-border/70 bg-muted/30 p-4 sm:p-5'>
              <div className='mb-4 flex items-center justify-between gap-4'>
                <div className='flex items-center gap-2'>
                  <span className='flex size-7 items-center justify-center rounded-md bg-foreground text-background'>
                    <Circle className='size-2.5 fill-emerald-400 text-emerald-400' />
                  </span>
                  <div>
                    <p className='text-xs font-semibold text-foreground'>{t('home.redesign.requestTitle')}</p>
                    <p className='text-[10px] text-muted-foreground'>{t('home.redesign.requestMeta')}</p>
                  </div>
                </div>
                <span className='rounded-full bg-emerald-500/10 px-2 py-1 text-[10px] font-semibold text-emerald-600 dark:text-emerald-400'>200 OK</span>
              </div>
              <HeroTerminalDemo />
              <div className='mt-4 grid gap-2 sm:grid-cols-3'>
                {ROUTE_STEPS.map((step, index) => {
                  const Icon = step.icon
                  return (
                    <div key={step.label} className='relative flex items-center gap-2 rounded-lg border border-border bg-background px-3 py-2.5'>
                      <span className='flex size-7 shrink-0 items-center justify-center rounded-md bg-primary/10 text-primary'>
                        <Icon className='size-3.5' />
                      </span>
                      <div className='min-w-0'>
                        <p className='truncate text-[11px] font-semibold text-foreground'>{step.label}</p>
                        <p className='truncate text-[10px] text-muted-foreground'>{step.detail}</p>
                      </div>
                      {index < ROUTE_STEPS.length - 1 && (
                        <motion.span
                          aria-hidden
                          animate={prefersReducedMotion ? undefined : { opacity: [0.25, 1, 0.25] }}
                          transition={{ duration: 1.8, repeat: Infinity, delay: index * 0.25 }}
                          className='pointer-events-none absolute -right-2.5 top-1/2 hidden size-5 -translate-y-1/2 items-center justify-center rounded-full border border-border bg-background text-[10px] text-muted-foreground sm:flex'
                        >
                          →
                        </motion.span>
                      )}
                    </div>
                  )
                })}
              </div>
            </div>
          </div>
        </Reveal>
      </div>
    </section>
  )
}
