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
import { Check, CircleCheck, Play, TerminalSquare } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { cn } from '@/lib/utils'

import { Reveal } from './reveal'

type CodeLanguage = 'curl' | 'typescript' | 'python'

const CODE_EXAMPLES: Record<CodeLanguage, string> = {
  curl: `curl https://api.example.com/v1/chat/completions \\
  -H "Authorization: Bearer $API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "auto",
    "messages": [{ "role": "user", "content": "Ship it." }]
  }'`,
  typescript: `import OpenAI from 'openai'

const client = new OpenAI({
  apiKey: process.env.API_KEY,
  baseURL: 'https://api.example.com/v1',
})

const result = await client.chat.completions.create({
  model: 'auto',
  messages: [{ role: 'user', content: 'Ship it.' }],
})`,
  python: `from openai import OpenAI

client = OpenAI(
    api_key=os.environ['API_KEY'],
    base_url='https://api.example.com/v1',
)

result = client.chat.completions.create(
    model='auto',
    messages=[{'role': 'user', 'content': 'Ship it.'}],
)`,
}

const TIMELINE_STEPS = [
  'home.redesign.integration.stepAuthenticated',
  'home.redesign.integration.stepPolicy',
  'home.redesign.integration.stepRouted',
  'home.redesign.integration.stepStream',
  'home.redesign.integration.stepTrace',
]

function CodePanel({ props }: { props: { language: CodeLanguage } }) {
  return (
    <AnimatePresence mode='wait'>
      <motion.pre
        key={props.language}
        initial={{ opacity: 0, y: 5 }}
        animate={{ opacity: 1, y: 0 }}
        exit={{ opacity: 0, y: -5 }}
        transition={{ duration: 0.16 }}
        className='min-w-max font-mono text-xs leading-6 text-slate-200'
      >
        <code>{CODE_EXAMPLES[props.language]}</code>
      </motion.pre>
    </AnimatePresence>
  )
}

export function IntegrationShowcase() {
  const { t } = useTranslation()
  const prefersReducedMotion = useReducedMotion()
  const [language, setLanguage] = useState<CodeLanguage>('curl')
  const [isRunning, setIsRunning] = useState(false)

  const runDemo = () => {
    setIsRunning(true)
    window.setTimeout(() => setIsRunning(false), 1450)
  }

  return (
    <section className='border-b border-border/70 bg-muted/30 px-5 py-16 sm:px-8 sm:py-20 lg:px-12 lg:py-24'>
      <div className='mx-auto max-w-7xl'>
        <Reveal className='mb-10 flex flex-col gap-4 sm:mb-12 sm:flex-row sm:items-end sm:justify-between'>
          <div className='max-w-2xl'>
            <p className='mb-3 text-xs font-semibold uppercase tracking-[0.16em] text-primary'>
              {t('home.redesign.integration.eyebrow')}
            </p>
            <h2 className='text-3xl font-semibold tracking-[-0.03em] text-foreground sm:text-4xl'>
              {t('home.redesign.integration.title')}
            </h2>
            <p className='mt-3 text-sm leading-6 text-muted-foreground sm:text-base'>
              {t('home.redesign.integration.subtitle')}
            </p>
          </div>
          <span className='inline-flex items-center gap-2 text-xs font-medium text-muted-foreground'>
            <Check className='size-4 text-emerald-500' />
            {t('home.redesign.integration.compatible')}
          </span>
        </Reveal>

        <div className='grid overflow-hidden rounded-xl border border-border bg-card shadow-sm lg:grid-cols-[1.08fr_0.92fr]'>
          <Reveal className='min-w-0 border-b border-border lg:border-b-0 lg:border-r'>
            <div className='flex items-center justify-between gap-3 border-b border-white/10 bg-slate-950 px-4 py-3 text-slate-100 sm:px-5'>
              <div className='flex items-center gap-2'>
                <TerminalSquare className='size-4 text-cyan-300' />
                <span className='font-mono text-xs'>{t('home.redesign.integration.requestFile')}</span>
              </div>
              <div className='flex rounded-md border border-white/10 bg-white/5 p-0.5' role='tablist' aria-label={t('home.redesign.integration.languageSelector')}>
                {(Object.keys(CODE_EXAMPLES) as CodeLanguage[]).map((item) => (
                  <button
                    key={item}
                    type='button'
                    role='tab'
                    aria-selected={language === item}
                    onClick={() => setLanguage(item)}
                    className={cn(
                      'rounded px-2 py-1 text-[10px] font-medium capitalize transition-colors',
                      language === item ? 'bg-white/15 text-white' : 'text-slate-400 hover:text-slate-200'
                    )}
                  >
                    {item === 'typescript' ? 'TS' : item}
                  </button>
                ))}
              </div>
            </div>
            <div className='overflow-x-auto bg-slate-950 px-4 py-5 sm:px-5'>
              <CodePanel props={{ language }} />
            </div>
          </Reveal>

          <Reveal delay={110} variant='fade-left'>
            <div className='flex h-full min-h-[23rem] flex-col p-5 sm:p-6'>
              <div className='flex items-start justify-between gap-4'>
                <div>
                  <p className='text-sm font-semibold text-foreground'>{t('home.redesign.integration.traceTitle')}</p>
                  <p className='mt-1 text-xs text-muted-foreground'>{t('home.redesign.integration.traceDesc')}</p>
                </div>
                <span className='rounded-full bg-emerald-500/10 px-2 py-1 text-[10px] font-semibold text-emerald-600 dark:text-emerald-400'>
                  {isRunning ? t('home.redesign.integration.running') : '200 OK'}
                </span>
              </div>

              <div className='relative mt-7 flex-1 space-y-0'>
                <div className='absolute bottom-3 left-2.5 top-3 w-px bg-border' />
                {TIMELINE_STEPS.map((step, index) => (
                  <motion.div
                    key={step}
                    initial={prefersReducedMotion ? false : { opacity: 0, x: 8 }}
                    whileInView={{ opacity: 1, x: 0 }}
                    viewport={{ once: true, amount: 0.5 }}
                    transition={{ duration: 0.25, delay: index * 0.08 }}
                    className='relative flex items-center gap-3 py-2.5'
                  >
                    <span className={cn('relative z-10 flex size-5 shrink-0 items-center justify-center rounded-full border border-card bg-background', isRunning ? 'text-primary' : 'text-emerald-500')}>
                      {index === TIMELINE_STEPS.length - 1 ? <CircleCheck className='size-3.5' /> : <span className='size-1.5 rounded-full bg-current' />}
                    </span>
                    <span className='text-xs font-medium text-foreground'>{t(step)}</span>
                    <span className='ml-auto text-[10px] tabular-nums text-muted-foreground'>+{[0, 4, 18, 24, 36][index]} ms</span>
                  </motion.div>
                ))}
              </div>

              <button
                type='button'
                onClick={runDemo}
                className='group relative mt-6 inline-flex h-10 items-center justify-center gap-2 overflow-hidden rounded-lg bg-foreground px-4 text-sm font-semibold text-background transition-transform hover:-translate-y-0.5 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2'
              >
                {!prefersReducedMotion && <motion.span aria-hidden animate={{ x: ['-140%', '140%'] }} transition={{ duration: 2.4, repeat: Infinity, ease: 'linear' }} className='pointer-events-none absolute inset-y-0 w-1/3 -skew-x-12 bg-gradient-to-r from-transparent via-background/35 to-transparent' />}
                <Play className='relative size-3.5 fill-current' />
                <span className='relative'>{isRunning ? t('home.redesign.integration.running') : t('home.redesign.integration.runDemo')}</span>
              </button>
            </div>
          </Reveal>
        </div>
      </div>
    </section>
  )
}
