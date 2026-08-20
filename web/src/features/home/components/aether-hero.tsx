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
import { Link, useNavigate } from '@tanstack/react-router'
import { ChevronDown, Menu, X } from 'lucide-react'
import { useCallback, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { useStatus } from '@/hooks/use-status'
import { cn } from '@/lib/utils'

/**
 * AetherVision single-screen cinematic hero: one full-bleed background
 * video with glassmorphism navigation and two glass cards pinned over it.
 * Light theme: dark ink on a softly tinted video overlay.
 */

const BACKGROUND_VIDEO_URL =
  'https://d8j0ntlcm91z4.cloudfront.net/user_38xzZboKViGWJOttwIXH07lWA1P/hf_20260803_192301_9231ed6b-c55c-4a48-909c-4ebe11cf2e11.mp4'

// Shared dark vertical gradient for every "Get started" surface.
const CTA_GRADIENT = 'linear-gradient(to bottom, #334155, #0F172A)'

export interface AetherHeroProps {
  className?: string
  isAuthenticated?: boolean
}

type HeroNavLink = {
  title: string
  href: string
  external?: boolean
  requiresAuth?: boolean
  chevron?: boolean
}

// AetherVision mark — 24×24, four petals. Ink #010101 → white on lg+.
function LogoMark({ className }: { className?: string }) {
  return (
    <svg
      className={className}
      viewBox='0 0 256 256'
      xmlns='http://www.w3.org/2000/svg'
    >
      <path
        d='M 128 128 C 128 198.692 70.692 256 0 256 C 0 185.308 57.308 128 128 128 Z M 128 128 C 198.692 128 256 185.308 256 256 C 185.308 256 128 198.692 128 128 Z M 0 0 C 70.692 0 128 57.308 128 128 C 57.308 128 0 70.692 0 0 Z M 256 0 C 256 70.692 198.692 128 128 128 C 128 57.308 185.308 0 256 0 Z'
        fill='currentColor'
      />
    </svg>
  )
}

export function AetherHero(props: AetherHeroProps) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { status } = useStatus()
  const docsUrl =
    (status?.docs_link as string | undefined) || 'https://docs.newapi.pro'
  const isAuthenticated = !!props.isAuthenticated
  const [menuOpen, setMenuOpen] = useState(false)

  // Lock body scroll while the mobile drawer is open.
  useEffect(() => {
    document.body.style.overflow = menuOpen ? 'hidden' : ''
    return () => {
      document.body.style.overflow = ''
    }
  }, [menuOpen])

  const links: HeroNavLink[] = [
    { title: t('Models'), href: '/models', requiresAuth: true },
    { title: t('Docs'), href: docsUrl, external: true, chevron: true },
    { title: t('About'), href: '/about' },
  ]

  // Guests clicking Models detour through sign-in with a redirect back.
  const guardModelsClick = useCallback(
    (event: React.MouseEvent<HTMLAnchorElement>) => {
      setMenuOpen(false)
      if (!isAuthenticated) {
        event.preventDefault()
        navigate({ to: '/sign-in', search: { redirect: '/models' } })
      }
    },
    [isAuthenticated, navigate]
  )

  const [email, setEmail] = useState('')

  const goSignUp = useCallback(
    (event: React.FormEvent) => {
      event.preventDefault()
      const trimmed = email.trim()
      navigate({
        to: '/sign-up',
        search: trimmed ? { email: trimmed } : {},
      })
    },
    [email, navigate]
  )

  const ctaTo = isAuthenticated ? '/dashboard' : '/sign-up'
  const ctaLabel = isAuthenticated ? t('Go to Dashboard') : t('Get Started')

  const renderGradientPill = (
    className: string,
    options?: { onClick?: () => void }
  ) => (
    <Link
      to={ctaTo}
      onClick={options?.onClick}
      style={{ background: CTA_GRADIENT }}
      className={cn(
        'rounded-full text-sm font-medium text-white transition-opacity duration-300 hover:opacity-90',
        className
      )}
    >
      {ctaLabel}
    </Link>
  )

  const renderDesktopLinks = () => (
    <div className='flex items-center gap-1'>
      {links.map((link) => {
        const linkClassName =
          'flex items-center gap-1.5 rounded-full px-4 py-1.5 text-sm font-medium text-slate-700 transition-colors duration-200 hover:bg-slate-900/5 hover:text-slate-900'
        if (link.external) {
          return (
            <a
              key={`desktop-${link.href}`}
              href={link.href}
              target='_blank'
              rel='noopener noreferrer'
              className={linkClassName}
            >
              <span>{link.title}</span>
              {link.chevron && <ChevronDown className='size-3.5 shrink-0' />}
            </a>
          )
        }
        return (
          <Link
            key={`desktop-${link.href}`}
            to={link.href}
            onClick={link.requiresAuth ? guardModelsClick : undefined}
            className={linkClassName}
          >
            {link.title}
          </Link>
        )
      })}
    </div>
  )

  const renderDrawerLinks = () => (
    <nav className='flex flex-col gap-2 px-6 pt-24'>
      {links.map((link, i) => {
        const linkClassName = cn(
          'flex items-center justify-between rounded-xl px-4 py-3.5 text-base font-medium text-slate-700 transition-all duration-500 hover:bg-slate-900/5 hover:text-slate-900',
          menuOpen ? 'translate-x-0 opacity-100' : 'translate-x-6 opacity-0'
        )
        const transitionStyle = {
          transitionDelay: menuOpen ? `${(i + 1) * 60}ms` : '0ms',
        }
        if (link.external) {
          return (
            <a
              key={`drawer-${link.href}`}
              href={link.href}
              target='_blank'
              rel='noopener noreferrer'
              onClick={() => setMenuOpen(false)}
              className={linkClassName}
              style={transitionStyle}
            >
              <span>{link.title}</span>
              {link.chevron && <ChevronDown className='size-3.5 shrink-0' />}
            </a>
          )
        }
        return (
          <Link
            key={`drawer-${link.href}`}
            to={link.href}
            onClick={
              link.requiresAuth ? guardModelsClick : () => setMenuOpen(false)
            }
            className={linkClassName}
            style={transitionStyle}
          >
            {link.title}
          </Link>
        )
      })}
    </nav>
  )

  return (
    <section
      className={cn(
        'relative min-h-screen w-full antialiased',
        props.className
      )}
      style={{
        fontFamily: `'Geist', -apple-system, BlinkMacSystemFont, 'PingFang SC', 'Hiragino Sans GB', 'Microsoft YaHei', 'Noto Sans SC', sans-serif`,
      }}
    >
      {/* Full-bleed background video (continuous muted loop, fixed to first screen) */}
      <video
        src={BACKGROUND_VIDEO_URL}
        className='pointer-events-none fixed inset-0 h-screen w-full object-cover'
        autoPlay
        loop
        muted
        playsInline
        disablePictureInPicture
        disableRemotePlayback
        controlsList='nodownload noplaybackrate nofullscreen'
      />

      {/* Content layer over the video */}
      <div className='relative z-10 flex min-h-screen flex-col'>
        {/* Top navigation bar */}
        <nav className='landing-animate-fade-in flex items-center justify-between px-5 py-5 sm:px-8 sm:py-6 lg:px-12'>
          {/* Brand: mark + AetherVision / 以太视界 */}
          <Link to='/' className='flex shrink-0 items-center gap-2'>
            <LogoMark className='size-6 shrink-0 text-white drop-shadow-[0_1px_2px_rgba(0,0,0,0.5)]' />
            <span className='flex flex-col items-start leading-none'>
              <span className='text-lg font-semibold tracking-tight text-white drop-shadow-[0_1px_2px_rgba(0,0,0,0.5)]'>
                AetherVision
              </span>
              <span className='mt-0.5 text-sm font-semibold tracking-widest text-white/90 drop-shadow-[0_1px_2px_rgba(0,0,0,0.5)]'>
                以太视界
              </span>
            </span>
          </Link>

          <div className='flex items-center'>
            {/* Desktop glass pill cluster + CTA (md+) */}
            <div className='hidden items-stretch gap-3 md:flex'>
              <div className='rounded-full bg-white/85 px-1.5 py-1.5 backdrop-blur-lg shadow-md ring-1 ring-slate-900/10'>
                {renderDesktopLinks()}
              </div>
              {renderGradientPill(
                'flex items-center self-stretch px-5 text-sm font-medium'
              )}
            </div>

            {/* Mobile circular hamburger */}
            <button
              type='button'
              onClick={() => setMenuOpen((v) => !v)}
              aria-label={t('Toggle navigation menu')}
              className='relative z-50 flex h-10 w-10 items-center justify-center rounded-full bg-white/85 backdrop-blur-lg shadow-md ring-1 ring-slate-900/10 transition-all duration-300 md:hidden'
            >
              <span className='relative size-5'>
                <Menu
                  className={cn(
                    'absolute inset-0 size-5 text-[#0F172A] transition-all duration-300',
                    menuOpen && 'rotate-90 scale-0 opacity-0'
                  )}
                />
                <X
                  className={cn(
                    'absolute inset-0 size-5 text-[#0F172A] transition-all duration-300',
                    !menuOpen && '-rotate-90 scale-0 opacity-0'
                  )}
                />
              </span>
            </button>
          </div>
        </nav>

        {/* Main block pinned to the bottom */}
        <main className='mt-auto flex flex-col gap-6 px-5 pb-8 sm:gap-8 sm:px-8 sm:pb-12 lg:flex-row lg:items-end lg:justify-between lg:px-12 lg:pb-16'>
          {/* Left: headline + email CTA */}
          <div
            className='landing-animate-fade-up flex max-w-xl flex-col items-start'
            style={{ animationDelay: '150ms' }}
          >
            <h1 className='text-3xl leading-[1.1] font-semibold tracking-tight text-white drop-shadow-[0_2px_8px_rgba(0,0,0,0.6)] sm:text-4xl lg:text-[3.5rem]'>
              {t('home.hero.title')}
            </h1>
            <p className='mt-2 text-base tracking-tight text-white/95 drop-shadow-[0_1px_4px_rgba(0,0,0,0.6)] sm:text-lg'>
              {t('home.hero.subtitle')}
            </p>

            {isAuthenticated ? (
              renderGradientPill('mt-6 inline-flex px-6 py-3 sm:mt-8 sm:py-2.5')
            ) : (
              <div className='mt-6 flex flex-col gap-3 sm:mt-8 sm:flex-row sm:items-center'>
                <form
                  onSubmit={goSignUp}
                  className='flex w-full flex-col gap-3 sm:inline-flex sm:w-auto sm:flex-row sm:items-center sm:rounded-full sm:bg-white/95 sm:p-1.5 sm:shadow-lg sm:ring-1 sm:ring-slate-900/10 sm:backdrop-blur-lg'
                >
                  <input
                    type='email'
                    required
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                    placeholder={t('Type your email')}
                    className='rounded-full bg-white/95 px-5 py-3 text-sm text-gray-900 outline-none placeholder:text-gray-400 ring-1 ring-slate-200 backdrop-blur-lg sm:w-64 sm:rounded-none sm:bg-transparent sm:px-4 sm:py-2 sm:ring-0'
                  />
                  <button
                    type='submit'
                    style={{ background: CTA_GRADIENT }}
                    className='rounded-full px-6 py-3 text-sm font-medium text-white transition-opacity duration-300 hover:opacity-90 sm:py-2.5'
                  >
                    {t('Get Started for Free')}
                  </button>
                </form>
                <Link
                  to='/contact'
                  className='inline-flex items-center justify-center rounded-full border border-slate-300/80 bg-white/85 px-6 py-3 text-sm font-medium text-slate-700 backdrop-blur-lg shadow-md transition-all duration-300 hover:border-slate-400 hover:bg-white sm:py-2.5'
                >
                  {t('Contact Sales')}
                </Link>
              </div>
            )}
          </div>

          {/* Right: the two glass cards */}
          <div
            className='landing-animate-fade-left flex w-full flex-col gap-4 sm:flex-row lg:w-auto lg:gap-5'
            style={{ animationDelay: '300ms' }}
          >
            {/* Stats card */}
            <div className='flex w-full flex-col justify-between rounded-2xl bg-white/85 p-5 backdrop-blur-lg shadow-md ring-1 ring-slate-900/10 sm:w-64 sm:p-6'>
              <span
                className='text-3xl font-normal tracking-tight text-[#0F172A] sm:text-4xl'
                style={{ fontFamily: `'Silkscreen', cursive` }}
              >
                100+
              </span>
              <p className='mt-3 text-sm leading-relaxed text-slate-600 sm:mt-4'>
                {t('home.hero.statsDesc')}
              </p>
              <div className='mt-4 grid grid-cols-3 gap-2 border-t border-slate-200 pt-4'>
                <div className='flex flex-col'>
                  <span className='text-sm font-semibold text-[#0F172A]'>99.9%</span>
                  <span className='text-[10px] text-slate-500'>{t('home.hero.miniUptime')}</span>
                </div>
                <div className='flex flex-col'>
                  <span className='text-sm font-semibold text-[#0F172A]'>100K+</span>
                  <span className='text-[10px] text-slate-500'>{t('home.hero.miniQps')}</span>
                </div>
                <div className='flex flex-col'>
                  <span className='text-sm font-semibold text-[#0F172A]'>&lt;50ms</span>
                  <span className='text-[10px] text-slate-500'>{t('home.hero.miniLatency')}</span>
                </div>
              </div>
              <p className='mt-3 text-xs leading-relaxed text-slate-500'>
                {t('home.hero.stats')}
              </p>
            </div>

            {/* Testimonial card */}
            <div className='flex w-full flex-col rounded-2xl bg-white/85 p-5 backdrop-blur-lg shadow-md ring-1 ring-slate-900/10 sm:w-64 sm:p-6'>
              <div className='mb-3 flex shrink-0 items-center gap-2 sm:mb-4'>
                <span className='flex size-6 shrink-0 items-center justify-center rounded-md bg-[#0F172A]'>
                  <LogoMark className='size-3.5 text-white' />
                </span>
                <span className='flex flex-col items-start leading-[1.1]'>
                  <span className='text-sm font-semibold text-[#0F172A]'>
                    AetherVision
                  </span>
                  <span className='text-[11px] font-normal text-slate-500'>
                    以太视界
                  </span>
                </span>
              </div>
              <blockquote className='text-sm leading-relaxed text-slate-700'>
                {t('home.hero.testimonial')}
              </blockquote>
              <p className='mt-1 text-xs leading-relaxed text-slate-500'>
                {t('home.hero.testimonialZh')}
              </p>
              <div className='mt-4 flex items-center gap-3 sm:mt-5'>
                <div className='flex size-9 shrink-0 items-center justify-center rounded-full bg-gradient-to-br from-cyan-500 to-blue-600 text-sm font-semibold text-white'>
                  SK
                </div>
                <div className='flex flex-col'>
                  <span className='text-sm font-semibold text-[#0F172A]'>
                    Sara Klein
                  </span>
                  <span className='text-xs text-slate-500'>
                    {t('home.hero.role')}
                  </span>
                </div>
              </div>
            </div>
          </div>
        </main>
      </div>

      {/* Mobile menu: glass backdrop */}
      <div
        aria-hidden
        onClick={() => setMenuOpen(false)}
        className={cn(
          'fixed inset-0 z-40 bg-slate-900/20 backdrop-blur-md transition-opacity duration-300 md:hidden',
          menuOpen ? 'opacity-100' : 'pointer-events-none opacity-0'
        )}
      />

      {/* Mobile menu: right-anchored glass drawer */}
      <div
        className={cn(
          'fixed top-0 right-0 z-40 flex h-full w-72 flex-col bg-white/95 backdrop-blur-xl shadow-xl transition-transform duration-500 ease-[cubic-bezier(0.16,1,0.3,1)] md:hidden',
          menuOpen ? 'translate-x-0' : 'translate-x-full'
        )}
      >
        {renderDrawerLinks()}

        {/* Bottom CTA: delayed fade + 16px slide-up */}
        <div
          className={cn(
            'mt-auto px-6 pb-10 transition-all duration-300',
            menuOpen ? 'translate-y-0 opacity-100' : 'translate-y-4 opacity-0'
          )}
          style={{ transitionDelay: menuOpen ? '300ms' : '0ms' }}
        >
          {renderGradientPill('block w-full px-6 py-3 text-center', {
            onClick: () => setMenuOpen(false),
          })}
        </div>
      </div>
    </section>
  )
}
