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
import { ArrowRight } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Reveal } from './reveal'

const CTA_GRADIENT = 'linear-gradient(to bottom, #334155, #0F172A)'

export function BottomCTA() {
  const { t } = useTranslation()

  return (
    <section className='border-t border-slate-200 px-5 py-16 sm:px-8 sm:py-20 lg:px-12 lg:py-24'>
      <div className='mx-auto max-w-4xl text-center'>
        <Reveal variant='scale-in'>
          <h2 className='mb-4 text-2xl font-semibold tracking-tight text-slate-900 sm:text-3xl lg:text-4xl'>
            {t('home.cta.title')}
          </h2>
          <p className='mb-8 text-sm text-slate-500 sm:text-base'>
            {t('home.cta.subtitle')}
          </p>
          <div className='flex flex-col items-center justify-center gap-4 sm:flex-row'>
            <Link
              to='/sign-up'
              style={{ background: CTA_GRADIENT }}
              className='inline-flex items-center gap-2 rounded-full px-8 py-3.5 text-sm font-semibold text-white transition-all duration-300 hover:opacity-90 hover:shadow-[0_0_30px_rgba(6,182,212,0.3)]'
            >
              {t('home.cta.getStarted')}
              <ArrowRight className='size-4' strokeWidth={2} />
            </Link>
            <Link
              to='/contact'
              className='inline-flex items-center gap-2 rounded-full border border-slate-300 bg-white px-8 py-3.5 text-sm font-semibold text-slate-700 transition-all duration-300 hover:border-slate-400 hover:bg-slate-50'
            >
              {t('home.cta.contactSales')}
            </Link>
          </div>
        </Reveal>
      </div>
    </section>
  )
}
