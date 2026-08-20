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

import { useCountUp } from '../hooks/use-count-up'
import { useInView } from '../hooks/use-in-view'
import { Reveal } from './reveal'

const METRICS = [
  { value: '100+', label: 'home.redesign.proof.models' },
  { value: '99.99%', label: 'home.redesign.proof.uptime' },
  { value: '<120ms', label: 'home.redesign.proof.latency' },
  { value: '40+', label: 'home.redesign.proof.providers' },
]

const LOGOS = [
  'openai.svg',
  'anthropic.svg',
  'google.svg',
  'moonshot.svg',
  'deepseek-color.svg',
  'qwen-color.svg',
  'mistral-color.svg',
  'hunyuan-color.svg',
]

function Metric({ props }: { props: { value: string; label: string; inView: boolean } }) {
  const { t } = useTranslation()
  const display = useCountUp(props.value.replace(/^</, ''), props.inView)
  const value = props.value.startsWith('<') ? `<${display}` : display

  return (
    <div className='border-border/70 border-b px-4 py-6 text-center last:border-b-0 sm:border-b-0 sm:border-r sm:px-6 sm:last:border-r-0'>
      <p className='text-3xl font-semibold tracking-[-0.04em] text-foreground sm:text-4xl'>{value}</p>
      <p className='mt-2 text-xs font-medium text-muted-foreground'>{t(props.label)}</p>
    </div>
  )
}

export function ProofBand() {
  const { t } = useTranslation()
  const { ref, inView } = useInView<HTMLDivElement>()

  return (
    <section className='border-b border-border/70 bg-background px-5 py-12 sm:px-8 sm:py-14 lg:px-12'>
      <div className='mx-auto max-w-7xl'>
        <Reveal>
          <div ref={ref} className='grid overflow-hidden rounded-xl border border-border bg-card sm:grid-cols-4'>
            {METRICS.map((metric) => <Metric key={metric.label} props={{ ...metric, inView }} />)}
          </div>
        </Reveal>
        <Reveal delay={100} className='mt-10'>
          <p className='mb-5 text-center text-xs font-medium text-muted-foreground'>
            {t('home.redesign.proof.ecosystem')}
          </p>
          <div className='flex flex-wrap items-center justify-center gap-x-8 gap-y-5'>
            {LOGOS.map((logo) => {
              const colorLogo = logo.endsWith('-color.svg')
              const source = colorLogo ? `/icons/channels/color/${logo}` : `/icons/channels/mono/${logo}`
              return <img key={logo} src={source} alt={logo.replace(/(-color)?\.svg$/, '')} className={colorLogo ? 'h-5 w-auto object-contain opacity-65 grayscale transition hover:grayscale-0 hover:opacity-100' : 'h-5 w-auto object-contain opacity-55 brightness-0 dark:brightness-0 dark:invert'} />
            })}
          </div>
        </Reveal>
      </div>
    </section>
  )
}
