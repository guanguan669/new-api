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
import { Reveal } from './reveal'

// Row 1: color SVGs (22)
const COLOR_LOGOS = [
  'aws-color.svg',
  'claude-color.svg',
  'cohere-color.svg',
  'copilot-color.svg',
  'deepseek-color.svg',
  'doubao-color.svg',
  'gemini-color.svg',
  'huawei-color.svg',
  'hunyuan-color.svg',
  'lg-color.svg',
  'meta-color.svg',
  'microsoft-color.svg',
  'minimax-color.svg',
  'mistral-color.svg',
  'nvidia-color.svg',
  'qwen-color.svg',
  'snowflake-color.svg',
  'spark-color.svg',
  'stability-color.svg',
  'tii-color.svg',
  'wenxin-color.svg',
  'zhipu-color.svg',
]

// Row 2: mono SVGs (24)
const MONO_LOGOS = [
  'openai.svg',
  'anthropic.svg',
  'google.svg',
  'moonshot.svg',
  'kimi.svg',
  'xai.svg',
  'perplexity.svg',
  'ollama.svg',
  'huggingface.svg',
  'deepmind.svg',
  'azure.svg',
  'bedrock.svg',
  'volcengine.svg',
  'tencent.svg',
  'alibaba.svg',
  'baidu.svg',
  'bytedance.svg',
  'iflytekcloud.svg',
  'internlm.svg',
  'sensenova.svg',
  'stepfun.svg',
  'zeroone.svg',
  'baichuan.svg',
  'ai21.svg',
]

function LogoItem({ src, alt, mono }: { src: string; alt: string; mono?: boolean }) {
  return (
    <div className='flex shrink-0 items-center justify-center px-4'>
      <img
        src={src}
        alt={alt}
        className={`h-8 w-auto object-contain opacity-60 transition-all duration-300 hover:opacity-100 hover:scale-110 ${
          mono ? 'brightness-0' : ''
        }`}
      />
    </div>
  )
}

function MarqueeRow({ logos, mono, reverse }: { logos: string[]; mono?: boolean; reverse?: boolean }) {
  const items = [...logos, ...logos]
  return (
    <div className='relative overflow-hidden'>
      <div
        className='flex w-max animate-marquee items-center py-4'
        style={{
          animationDirection: reverse ? 'reverse' : 'normal',
        }}
      >
        {items.map((logo, i) => (
          <LogoItem
            // Marquee duplicates the logo list; the position makes each key unique.
            key={`${logo}-${i >= logos.length ? 'dup' : 'orig'}`}
            src={`/icons/channels/${mono ? 'mono' : 'color'}/${logo}`}
            alt={logo.replace(/\.(svg|png)$/, '').replace(/-color$/, '')}
            mono={mono}
          />
        ))}
      </div>
      {/* Fade edges */}
      <div className='pointer-events-none absolute inset-y-0 left-0 w-24 bg-gradient-to-r from-[#FAFBFC] to-transparent' />
      <div className='pointer-events-none absolute inset-y-0 right-0 w-24 bg-gradient-to-l from-[#FAFBFC] to-transparent' />
    </div>
  )
}

export function TrustLogos() {
  const { t } = useTranslation()

  return (
    <section className='border-t border-slate-200 px-5 py-16 sm:px-8 sm:py-20 lg:px-12 lg:py-24'>
      <div className='mx-auto max-w-6xl'>
        <Reveal className='mb-10 text-center sm:mb-12'>
          <p className='mb-2 text-xs font-semibold uppercase tracking-widest text-slate-500'>
            {t('home.trust.label')}
          </p>
          <h2 className='text-2xl font-semibold tracking-tight text-slate-900 sm:text-3xl lg:text-4xl'>
            {t('home.trust.title')}
          </h2>
          <p className='mt-3 text-sm text-slate-500 sm:text-base'>
            {t('home.trust.subtitle')}
          </p>
        </Reveal>

        <Reveal variant='fade-in'>
          <div className='space-y-4'>
            <MarqueeRow logos={COLOR_LOGOS} />
            <MarqueeRow logos={MONO_LOGOS} mono reverse />
          </div>
        </Reveal>
      </div>
    </section>
  )
}
