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

interface ModelCard {
  name: string
  vendor: string
  context: string
  capabilities: string[]
  icon: string
}

const MODELS: ModelCard[] = [
  {
    name: 'Claude Opus 5',
    vendor: 'Anthropic',
    context: '200K',
    capabilities: ['Vision', 'Tool Use', 'Reasoning'],
    icon: '/icons/channels/color/claude-color.svg',
  },
  {
    name: 'GPT-5.4',
    vendor: 'OpenAI',
    context: '128K',
    capabilities: ['Vision', 'Tool Use', 'Code'],
    icon: '/icons/channels/mono/openai.svg',
  },
  {
    name: 'Kimi-K3',
    vendor: 'Moonshot AI',
    context: '1049K',
    capabilities: ['Long Context', 'Reasoning'],
    icon: '/icons/channels/mono/kimi.svg',
  },
  {
    name: 'GLM-5.2',
    vendor: 'Z.ai',
    context: '1049K',
    capabilities: ['Long Context', 'Multilingual'],
    icon: '/icons/channels/color/zhipu-color.svg',
  },
  {
    name: 'Qwen3.6-122B',
    vendor: 'Qwen',
    context: '262K',
    capabilities: ['Code', 'Tool Use'],
    icon: '/icons/channels/color/qwen-color.svg',
  },
  {
    name: 'DeepSeek-V4',
    vendor: 'DeepSeek',
    context: '128K',
    capabilities: ['Reasoning', 'Code'],
    icon: '/icons/channels/color/deepseek-color.svg',
  },
  {
    name: 'Gemini 3.0',
    vendor: 'Google',
    context: '1000K',
    capabilities: ['Vision', 'Multilingual'],
    icon: '/icons/channels/color/gemini-color.svg',
  },
  {
    name: 'Doubao 2.0',
    vendor: 'ByteDance',
    context: '256K',
    capabilities: ['Multilingual', 'Tool Use'],
    icon: '/icons/channels/color/doubao-color.svg',
  },
  {
    name: 'Hunyuan Pro',
    vendor: 'Tencent',
    context: '128K',
    capabilities: ['Multilingual', 'Code'],
    icon: '/icons/channels/color/hunyuan-color.svg',
  },
  {
    name: 'Wenxin 5.0',
    vendor: 'Baidu',
    context: '128K',
    capabilities: ['Multilingual', 'Reasoning'],
    icon: '/icons/channels/color/wenxin-color.svg',
  },
  {
    name: 'Spark 4.0',
    vendor: 'iFlytek',
    context: '128K',
    capabilities: ['Multilingual', 'Audio'],
    icon: '/icons/channels/color/spark-color.svg',
  },
  {
    name: 'MiniMax-M3',
    vendor: 'MiniMax',
    context: '1049K',
    capabilities: ['Long Context', 'Reasoning'],
    icon: '/icons/channels/color/minimax-color.svg',
  },
]

const CAPABILITY_COLORS: Record<string, string> = {
  Vision: 'bg-purple-500/10 text-purple-700 border-purple-500/20',
  'Tool Use': 'bg-blue-500/10 text-blue-700 border-blue-500/20',
  Reasoning: 'bg-emerald-500/10 text-emerald-700 border-emerald-500/20',
  'Long Context': 'bg-cyan-500/10 text-cyan-700 border-cyan-500/20',
  Code: 'bg-orange-500/10 text-orange-700 border-orange-500/20',
  Multilingual: 'bg-pink-500/10 text-pink-700 border-pink-500/20',
  Audio: 'bg-yellow-500/10 text-yellow-700 border-yellow-500/20',
}

function isMonoIcon(path: string) {
  return path.includes('/mono/')
}

export function ModelWall() {
  const { t } = useTranslation()

  return (
    <section className='border-t border-slate-200 px-5 py-16 sm:px-8 sm:py-20 lg:px-12 lg:py-24'>
      <div className='mx-auto max-w-6xl'>
        <Reveal className='mb-10 text-center sm:mb-12 lg:mb-14'>
          <p className='mb-2 text-xs font-semibold uppercase tracking-widest text-slate-500'>
            {t('home.models.label')}
          </p>
          <h2 className='text-2xl font-semibold tracking-tight text-slate-900 sm:text-3xl lg:text-4xl'>
            {t('home.models.title')}
          </h2>
          <p className='mt-3 text-sm text-slate-500 sm:text-base'>
            {t('home.models.subtitle')}
          </p>
        </Reveal>

        <div className='grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4'>
          {MODELS.map((model, i) => (
            <Reveal key={model.name} delay={(i % 4) * 70}>
              <div className='group flex h-full flex-col rounded-2xl border border-slate-200 bg-white p-5 shadow-sm transition-all duration-300 hover:border-slate-300 hover:shadow-md'>
                <div className='mb-4 flex items-center gap-3'>
                  <div className='flex size-10 items-center justify-center rounded-lg bg-slate-100 p-1.5'>
                    <img
                      src={model.icon}
                      alt={model.vendor}
                      className={
                        isMonoIcon(model.icon)
                          ? 'size-full object-contain brightness-0'
                          : 'size-full object-contain'
                      }
                    />
                  </div>
                  <div className='flex flex-col'>
                    <span className='text-sm font-semibold text-slate-900'>
                      {model.name}
                    </span>
                    <span className='text-xs text-slate-500'>{model.vendor}</span>
                  </div>
                </div>

                <div className='mb-3 flex items-center gap-2'>
                  <span className='rounded-md bg-slate-100 px-2 py-0.5 text-xs font-medium text-slate-600'>
                    {model.context}
                  </span>
                </div>

                <div className='flex flex-wrap gap-1.5'>
                  {model.capabilities.map((cap) => (
                    <span
                      key={cap}
                      className={`rounded-full border px-2 py-0.5 text-[11px] font-medium ${CAPABILITY_COLORS[cap] || 'bg-slate-100 text-slate-600 border-slate-200'}`}
                    >
                      {t(cap)}
                    </span>
                  ))}
                </div>
              </div>
            </Reveal>
          ))}
        </div>

        <Reveal className='mt-8 text-center'>
          <a
            href='/models'
            className='inline-flex items-center gap-2 text-sm font-medium text-cyan-600 transition-colors hover:text-cyan-700'
          >
            {t('home.models.viewAll')}
            <span aria-hidden>→</span>
          </a>
        </Reveal>
      </div>
    </section>
  )
}
