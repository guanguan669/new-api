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
import { ArrowRight, BookOpen, Check } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Link } from '@tanstack/react-router'

import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from '@/components/ui/accordion'
import { useStatus } from '@/hooks/use-status'
import { useAuthStore } from '@/stores/auth-store'

import { Reveal } from './reveal'

const FAQ_KEYS = [1, 2, 3, 4, 5, 6]

export function FaqAndCta() {
  const { t } = useTranslation()
  const { status } = useStatus()
  const { auth } = useAuthStore()
  const isAuthenticated = !!auth.user
  const docsUrl = (status?.docs_link as string | undefined) || 'https://docs.newapi.pro'
  const primaryLabel = isAuthenticated ? t('Go to Dashboard') : t('home.redesign.cta.primary')
  const primaryHref = isAuthenticated ? '/dashboard' : '/sign-up'

  return (
    <section className='bg-background px-5 py-16 sm:px-8 sm:py-20 lg:px-12 lg:py-24'>
      <div className='mx-auto max-w-7xl'>
        <div className='grid gap-12 lg:grid-cols-[0.8fr_1.2fr] lg:gap-20'>
          <Reveal>
            <p className='mb-3 text-xs font-semibold uppercase tracking-[0.16em] text-primary'>{t('home.redesign.faq.eyebrow')}</p>
            <h2 className='max-w-sm text-3xl font-semibold tracking-[-0.03em] text-foreground sm:text-4xl'>{t('home.redesign.faq.title')}</h2>
            <p className='mt-4 max-w-sm text-sm leading-6 text-muted-foreground'>{t('home.redesign.faq.subtitle')}</p>
            <div className='mt-7 space-y-3 text-xs text-muted-foreground'>
              {[t('home.redesign.faq.proofOne'), t('home.redesign.faq.proofTwo')].map((proof) => <span key={proof} className='flex items-center gap-2'><Check className='size-4 text-emerald-500' />{proof}</span>)}
            </div>
          </Reveal>
          <Reveal delay={100} variant='fade-left'>
            <Accordion className='rounded-xl border border-border bg-card px-5 sm:px-6'>
              {FAQ_KEYS.map((index) => (
                <AccordionItem key={index} value={`faq-${index}`} className='border-border/70'>
                  <AccordionTrigger className='py-5 text-sm text-foreground hover:no-underline'>
                    {t(`home.faq.q${index}.questionNew`)}
                  </AccordionTrigger>
                  <AccordionContent className='pb-5 text-sm leading-6 text-muted-foreground'>
                    {t(`home.faq.q${index}.answerNew`)}
                  </AccordionContent>
                </AccordionItem>
              ))}
            </Accordion>
          </Reveal>
        </div>

        <Reveal className='mt-16 sm:mt-20'>
          <div className='relative overflow-hidden rounded-xl border border-border bg-foreground px-6 py-10 text-background sm:px-10 sm:py-12'>
            <div className='pointer-events-none absolute -right-20 -top-24 size-80 rounded-full bg-cyan-400/20 blur-3xl' />
            <div className='relative flex flex-col gap-7 lg:flex-row lg:items-end lg:justify-between'>
              <div className='max-w-2xl'>
                <p className='mb-3 text-xs font-semibold uppercase tracking-[0.16em] text-background/60'>{t('home.redesign.cta.eyebrow')}</p>
                <h2 className='text-3xl font-semibold tracking-[-0.03em] text-background sm:text-4xl'>{t('home.redesign.cta.title')}</h2>
                <p className='mt-3 text-sm leading-6 text-background/70 sm:text-base'>{t('home.redesign.cta.subtitle')}</p>
              </div>
              <div className='flex flex-wrap gap-3'>
                <Link to={primaryHref} className='group inline-flex h-11 items-center gap-2 rounded-lg bg-background px-5 text-sm font-semibold text-foreground transition-transform hover:-translate-y-0.5 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-background focus-visible:ring-offset-2 focus-visible:ring-offset-foreground'>
                  {primaryLabel}<ArrowRight className='size-4 transition-transform group-hover:translate-x-0.5' />
                </Link>
                <a href={docsUrl} target='_blank' rel='noopener noreferrer' className='inline-flex h-11 items-center gap-2 rounded-lg border border-background/20 px-5 text-sm font-semibold text-background transition-colors hover:bg-background/10 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-background focus-visible:ring-offset-2 focus-visible:ring-offset-foreground'>
                  <BookOpen className='size-4' />{t('Read the docs')}
                </a>
              </div>
            </div>
          </div>
        </Reveal>
      </div>
    </section>
  )
}
