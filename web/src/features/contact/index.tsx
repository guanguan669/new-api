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
import { ArrowLeft, ArrowRight, CheckCircle2, Building2, Shield, Headset } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

const CTA_GRADIENT = 'linear-gradient(to bottom, #2B2B2B, #101010)'

const INPUT_CLASS =
  'w-full rounded-xl border border-white/10 bg-white/5 px-4 py-3 text-sm text-white placeholder:text-white/30 outline-none backdrop-blur-lg transition-colors duration-200 focus:border-cyan-400/50 focus:bg-white/[0.07]'

export function Contact() {
  const { t } = useTranslation()
  const [submitted, setSubmitted] = useState(false)
  const [form, setForm] = useState({
    name: '',
    company: '',
    email: '',
    message: '',
  })

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    setSubmitted(true)
  }

  const set = (key: keyof typeof form) => (
    e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>
  ) => setForm((f) => ({ ...f, [key]: e.target.value }))

  const PERKS = [
    { icon: <Building2 className='size-5' strokeWidth={1.5} />, key: 'enterprise' },
    { icon: <Shield className='size-5' strokeWidth={1.5} />, key: 'sla' },
    { icon: <Headset className='size-5' strokeWidth={1.5} />, key: 'support' },
  ] as const

  return (
    <div className='relative min-h-svh w-full overflow-hidden bg-[#05070C] text-white antialiased'>
      {/* Ambient glow orbs */}
      <div className='pointer-events-none absolute inset-0 overflow-hidden'>
        <div className='absolute -top-40 left-[15%] h-[480px] w-[480px] rounded-full bg-cyan-500/[0.07] blur-[140px]' />
        <div className='absolute -bottom-40 right-[10%] h-[420px] w-[420px] rounded-full bg-blue-500/[0.06] blur-[130px]' />
      </div>

      <Link
        to='/'
        className='group absolute top-5 left-5 z-10 flex items-center gap-2 text-sm text-white/60 transition-colors hover:text-white sm:top-8 sm:left-8'
      >
        <ArrowLeft
          className='size-4 transition-transform duration-300 group-hover:-translate-x-0.5'
          strokeWidth={2}
        />
        {t('Back to Home')}
      </Link>

      <div className='relative z-10 mx-auto flex min-h-svh max-w-6xl flex-col items-center justify-center gap-12 px-5 py-20 lg:flex-row lg:items-center lg:gap-16'>
        {/* Left: pitch */}
        <div className='landing-animate-fade-up w-full max-w-xl'>
          <p className='mb-3 text-xs font-semibold uppercase tracking-widest text-cyan-400/80'>
            {t('contact.label')}
          </p>
          <h1 className='bg-gradient-to-br from-white via-white to-white/60 bg-clip-text text-4xl font-bold tracking-tighter text-transparent sm:text-5xl'>
            {t('contact.title')}
          </h1>
          <p className='mt-4 max-w-[50ch] text-base leading-relaxed text-white/50'>
            {t('contact.subtitle')}
          </p>

          <ul className='mt-10 space-y-5'>
            {PERKS.map((perk) => (
              <li key={perk.key} className='flex items-start gap-4'>
                <span className='flex size-11 shrink-0 items-center justify-center rounded-xl border border-white/[0.08] bg-white/5 text-cyan-400'>
                  {perk.icon}
                </span>
                <span>
                  <span className='block text-sm font-semibold text-white'>
                    {t(`contact.perks.${perk.key}.title`)}
                  </span>
                  <span className='mt-1 block text-sm leading-relaxed text-white/50'>
                    {t(`contact.perks.${perk.key}.desc`)}
                  </span>
                </span>
              </li>
            ))}
          </ul>
        </div>

        {/* Right: glass form */}
        <div
          className='landing-animate-fade-left w-full max-w-md'
          style={{ animationDelay: '150ms' }}
        >
          <div className='rounded-[2rem] border border-white/[0.08] bg-white/[0.04] p-8 shadow-[0_20px_60px_-15px_rgba(0,0,0,0.5)] backdrop-blur-2xl sm:p-9'>
            {submitted ? (
              <div className='flex flex-col items-center py-10 text-center'>
                <CheckCircle2 className='mb-4 size-12 text-emerald-400' strokeWidth={1.5} />
                <h2 className='text-xl font-semibold text-white'>
                  {t('contact.success.title')}
                </h2>
                <p className='mt-2 text-sm leading-relaxed text-white/50'>
                  {t('contact.success.desc')}
                </p>
                <Link
                  to='/'
                  className='mt-8 inline-flex items-center gap-2 rounded-full border border-white/20 bg-white/5 px-6 py-2.5 text-sm font-medium text-white backdrop-blur-lg transition-all hover:border-white/30 hover:bg-white/10'
                >
                  {t('Back to Home')}
                </Link>
              </div>
            ) : (
              <form onSubmit={handleSubmit} className='space-y-4'>
                <div>
                  <label className='mb-1.5 block text-xs font-medium text-white/60'>
                    {t('contact.form.name')}
                  </label>
                  <input
                    required
                    value={form.name}
                    onChange={set('name')}
                    placeholder={t('contact.form.namePlaceholder')}
                    className={INPUT_CLASS}
                  />
                </div>
                <div>
                  <label className='mb-1.5 block text-xs font-medium text-white/60'>
                    {t('contact.form.company')}
                  </label>
                  <input
                    required
                    value={form.company}
                    onChange={set('company')}
                    placeholder={t('contact.form.companyPlaceholder')}
                    className={INPUT_CLASS}
                  />
                </div>
                <div>
                  <label className='mb-1.5 block text-xs font-medium text-white/60'>
                    {t('contact.form.email')}
                  </label>
                  <input
                    required
                    type='email'
                    value={form.email}
                    onChange={set('email')}
                    placeholder={t('contact.form.emailPlaceholder')}
                    className={INPUT_CLASS}
                  />
                </div>
                <div>
                  <label className='mb-1.5 block text-xs font-medium text-white/60'>
                    {t('contact.form.message')}
                  </label>
                  <textarea
                    rows={4}
                    value={form.message}
                    onChange={set('message')}
                    placeholder={t('contact.form.messagePlaceholder')}
                    className={`${INPUT_CLASS} resize-none`}
                  />
                </div>
                <button
                  type='submit'
                  style={{ background: CTA_GRADIENT }}
                  className='inline-flex w-full items-center justify-center gap-2 rounded-full px-6 py-3 text-sm font-semibold text-white transition-all duration-300 hover:opacity-90 hover:shadow-[0_0_30px_rgba(56,189,248,0.25)]'
                >
                  {t('contact.form.submit')}
                  <ArrowRight className='size-4' strokeWidth={2} />
                </button>
                <p className='text-center text-xs leading-relaxed text-white/35'>
                  {t('contact.form.privacy')}
                </p>
              </form>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
