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
import { Link } from '@tanstack/react-router'
import { ArrowLeft, Zap, Shield, Network } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { AuroraBackground } from '@/components/magicui/aurora-background'
import { GlowingEffect } from '@/components/magicui/glowing-effect'
import { Skeleton } from '@/components/ui/skeleton'
import { useSystemConfig } from '@/hooks/use-system-config'

type AuthLayoutProps = {
  children: React.ReactNode
}

const PANEL_ICONS = [
  '/icons/channels/color/claude-color.svg',
  '/icons/channels/color/gemini-color.svg',
  '/icons/channels/color/deepseek-color.svg',
  '/icons/channels/color/qwen-color.svg',
  '/icons/channels/color/zhipu-color.svg',
  '/icons/channels/color/doubao-color.svg',
  '/icons/channels/color/hunyuan-color.svg',
  '/icons/channels/color/minimax-color.svg',
  '/icons/channels/color/wenxin-color.svg',
  '/icons/channels/color/spark-color.svg',
]

const FEATURES = [
  { icon: <Network className='size-4' strokeWidth={2} />, key: 'routing' },
  { icon: <Zap className='size-4' strokeWidth={2} />, key: 'latency' },
  { icon: <Shield className='size-4' strokeWidth={2} />, key: 'security' },
] as const

function LogoItem({ src, mono }: { src: string; mono?: boolean }) {
  return (
    <div className='flex shrink-0 items-center justify-center px-5'>
      <img
        src={src}
        alt=''
        aria-hidden
        className={`h-7 w-auto object-contain ${mono ? 'opacity-70' : 'opacity-90'}`}
      />
    </div>
  )
}

function MarqueeRow({
  logos,
  mono,
  reverse,
}: {
  logos: string[]
  mono?: boolean
  reverse?: boolean
}) {
  const items = [...logos, ...logos]
  return (
    <div className='relative overflow-hidden'>
      <div
        className='flex w-max animate-marquee items-center py-3'
        style={{ animationDirection: reverse ? 'reverse' : 'normal' }}
      >
        {items.map((logo, i) => (
          <LogoItem
            key={`${logo}-${i >= logos.length ? 'dup' : 'orig'}`}
            src={logo}
            mono={mono}
          />
        ))}
      </div>
      <div className='pointer-events-none absolute inset-y-0 left-0 w-16 bg-gradient-to-r from-[#F8FAFF] to-transparent' />
      <div className='pointer-events-none absolute inset-y-0 right-0 w-16 bg-gradient-to-l from-[#F8FAFF] to-transparent' />
    </div>
  )
}

/** Feature card wrapped in the Aceternity glowing border effect */
function FeatureCard({
  icon,
  label,
  delay,
}: {
  icon: React.ReactNode
  label: string
  delay: number
}) {
  return (
    <li
      className='landing-animate-fade-up relative rounded-2xl border border-slate-200/70 bg-white/70 p-3.5 backdrop-blur-xl'
      style={{ animationDelay: `${delay}ms` }}
    >
      <GlowingEffect
        blur={6}
        spread={40}
        glow
        disabled={false}
        borderWidth={2}
        proximity={72}
        inactiveZone={0.2}
        movementDuration={1.6}
      />
      <div className='relative flex items-start gap-3'>
        <span className='flex size-9 shrink-0 items-center justify-center rounded-xl bg-gradient-to-br from-sky-500 to-indigo-500 text-white shadow-[0_6px_18px_-6px_rgba(56,120,248,0.65)]'>
          {icon}
        </span>
        <span className='pt-1 text-sm font-medium text-slate-700'>
          {label}
        </span>
      </div>
    </li>
  )
}

/**
 * Light split-screen auth shell: an ambient animated brand panel on the
 * left (lg+) and the form card pinned to the right.
 */
export function AuthLayout({ children }: AuthLayoutProps) {
  const { t } = useTranslation()
  const { systemName, loading } = useSystemConfig()

  return (
    <div className='relative grid min-h-svh w-full bg-[#F8FAFF] text-slate-900 antialiased lg:grid-cols-2'>
      {/* ── Left ambient panel (lg+) ── */}
      <aside className='relative hidden overflow-hidden lg:flex lg:flex-col'>
        <AuroraBackground className='flex-1'>
          {/* Faint dot grid, above aurora, below content */}
          <div
            className='pointer-events-none absolute inset-0 z-0 opacity-[0.55] [mask-image:radial-gradient(ellipse_at_center,black,transparent_75%)]'
            style={{
              backgroundImage:
                'radial-gradient(circle, rgba(100,116,139,0.16) 1px, transparent 1px)',
              backgroundSize: '26px 26px',
            }}
          />

          <div className='relative z-10 flex min-h-svh flex-1 flex-col p-12 xl:p-16'>
            {/* Brand — wordmark only, no site logo */}
            <div className='landing-animate-fade-in flex items-center'>
              {loading ? (
                <Skeleton className='h-6 w-24 bg-slate-200/60' />
              ) : (
                <span className='text-lg font-semibold tracking-tight text-slate-900'>
                  {systemName}
                </span>
              )}
            </div>

            {/* Center stage — nudged up so the stack reads vertically centered */}
            <div className='flex flex-1 flex-col items-center justify-center pb-10 text-center xl:pb-14'>
              <h1
                className='landing-animate-fade-up max-w-[18ch] bg-gradient-to-br from-slate-900 via-slate-700 to-indigo-400 bg-clip-text text-4xl font-bold tracking-tighter text-transparent xl:text-[3.4rem] xl:leading-[1.06]'
                style={{ animationDelay: '100ms' }}
              >
                {t('auth.panel.title')}
              </h1>
              <p
                className='landing-animate-fade-up mt-5 max-w-[46ch] text-base leading-relaxed text-slate-500'
                style={{ animationDelay: '180ms' }}
              >
                {t('auth.panel.subtitle')}
              </p>

              {/* Glowing feature cards */}
              <ul className='mt-10 grid w-full max-w-md grid-cols-1 gap-3 text-left'>
                {FEATURES.map((f, i) => (
                  <FeatureCard
                    key={f.key}
                    icon={f.icon}
                    label={t(`auth.panel.features.${f.key}`)}
                    delay={260 + i * 90}
                  />
                ))}
              </ul>
            </div>

            {/* Logo marquee */}
            <div
              className='landing-animate-fade-in'
              style={{ animationDelay: '480ms' }}
            >
              <p className='mb-1 text-center text-xs font-medium tracking-widest text-slate-400 uppercase'>
                {t('auth.panel.trusted')}
              </p>
              <MarqueeRow logos={PANEL_ICONS.slice(0, 5)} />
              <MarqueeRow logos={PANEL_ICONS.slice(5)} reverse />
            </div>
          </div>
        </AuroraBackground>
      </aside>

      {/* ── Right form column (force light tokens regardless of global theme) ── */}
      <main className='auth-force-light relative flex min-h-svh flex-col items-center justify-center bg-white px-4 py-10 sm:px-8 lg:min-h-0 lg:border-l lg:border-slate-100'>
        {/* Subtle top sheen */}
        <div className='pointer-events-none absolute inset-x-0 top-0 h-40 bg-gradient-to-b from-sky-50/60 to-transparent' />

        <Link
          to='/'
          className='group absolute top-5 left-5 z-10 flex items-center gap-2 text-sm text-slate-400 transition-colors hover:text-slate-700 sm:top-8 sm:left-8'
        >
          <ArrowLeft
            className='size-4 transition-transform duration-300 group-hover:-translate-x-0.5'
            strokeWidth={2}
          />
          {t('Back to Home')}
        </Link>

        {/* Mobile brand (panel hidden below lg) — wordmark only */}
        <div className='mb-8 flex items-center lg:hidden'>
          {loading ? (
            <Skeleton className='h-6 w-24 bg-slate-200/60' />
          ) : (
            <span className='text-lg font-semibold tracking-tight text-slate-900'>
              {systemName}
            </span>
          )}
        </div>

        <div className='landing-animate-scale-in relative z-10 w-full max-w-md'>
          {children}
        </div>
      </main>
    </div>
  )
}
