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
import { Activity, Code2, Cpu, Gauge, Network, Shield } from 'lucide-react'
import { useTranslation } from 'react-i18next'

/**
 * Relay-specific sections below the hero: providers grid, how it works,
 * core features. Keeps the dark cinematic aesthetic with glassmorphism.
 */

const PROVIDERS = [
  'OpenAI',
  'Anthropic',
  'Google',
  'DeepSeek',
  'Qwen',
  'Llama',
  'Mistral',
  'Cohere',
  'Azure',
  'AWS',
  'Together',
  'Groq',
  'xAI',
  'Minimax',
  'Doubao',
  'Zhipu',
]

export function RelaySections() {
  const { t } = useTranslation()

  const steps = [
    {
      num: '01',
      title: t('Get Your Key'),
      titleZh: '获取密钥',
      desc: t('Sign up and generate your unified API key in seconds'),
      descZh: '注册并在几秒内生成统一 API 密钥',
      icon: <Shield className='size-5' strokeWidth={1.5} />,
    },
    {
      num: '02',
      title: t('Point Your Endpoint'),
      titleZh: '配置端点',
      desc: t('Replace your provider URL with AetherVision relay endpoint'),
      descZh: '将提供商 URL 替换为 AetherVision 中转端点',
      icon: <Code2 className='size-5' strokeWidth={1.5} />,
    },
    {
      num: '03',
      title: t('Start Calling'),
      titleZh: '开始调用',
      desc: t('Same SDKs, same schemas — zero migration overhead'),
      descZh: '相同 SDK、相同模式——零迁移成本',
      icon: <Activity className='size-5' strokeWidth={1.5} />,
    },
  ]

  const features = [
    {
      icon: <Network className='size-6' strokeWidth={1.5} />,
      title: t('Unified API'),
      titleZh: '统一接口',
      desc: t('One endpoint for 40+ providers'),
      descZh: '一个端点接入 40+ 提供商',
    },
    {
      icon: <Gauge className='size-6' strokeWidth={1.5} />,
      title: t('Auto Failover'),
      titleZh: '自动故障切换',
      desc: t('Seamless fallback across channels'),
      descZh: '跨渠道无缝降级',
    },
    {
      icon: <Cpu className='size-6' strokeWidth={1.5} />,
      title: t('Intelligent Routing'),
      titleZh: '智能调度',
      desc: t('Optimal model selection based on load and latency'),
      descZh: '根据负载和延迟自动选择最优模型',
    },
  ]

  return (
    <>
      <div className='relative z-20 bg-gradient-to-b from-black/60 via-black/80 to-black'>
        {/* Providers Grid */}
        <section className='border-t border-white/10 px-5 py-16 sm:px-8 sm:py-20 lg:px-12 lg:py-24'>
          <div className='mx-auto max-w-6xl'>
            <div className='mb-10 text-center sm:mb-12 lg:mb-14'>
              <p className='mb-2 text-xs font-semibold uppercase tracking-widest text-white/50'>
                {t('Supported Providers')}
              </p>
              <h2 className='text-2xl font-semibold tracking-tight text-white sm:text-3xl lg:text-4xl'>
                40+ 渠道 · 一把密钥
              </h2>
            </div>
            <div className='grid grid-cols-2 gap-3 sm:grid-cols-3 sm:gap-4 lg:grid-cols-4'>
              {PROVIDERS.map((name) => (
                <div
                  key={name}
                  className='flex items-center justify-center rounded-xl border border-white/10 bg-white/5 px-4 py-5 backdrop-blur-lg transition-all duration-300 hover:border-white/20 hover:bg-white/10'
                >
                  <span className='text-sm font-medium text-white/80'>{name}</span>
                </div>
              ))}
            </div>
          </div>
        </section>

        {/* How It Works */}
        <section className='border-t border-white/10 px-5 py-16 sm:px-8 sm:py-20 lg:px-12 lg:py-24'>
          <div className='mx-auto max-w-6xl'>
            <div className='mb-10 text-center sm:mb-12 lg:mb-14'>
              <p className='mb-2 text-xs font-semibold uppercase tracking-widest text-white/50'>
                {t('How It Works')}
              </p>
              <h2 className='text-2xl font-semibold tracking-tight text-white sm:text-3xl lg:text-4xl'>
                三步开始使用
              </h2>
            </div>
            <div className='grid gap-6 sm:gap-8 md:grid-cols-3'>
              {steps.map((step) => (
                <div
                  key={step.num}
                  className='flex flex-col rounded-2xl border border-white/10 bg-white/5 p-6 backdrop-blur-lg sm:p-7'
                >
                  <div className='mb-4 flex items-center gap-3'>
                    <span className='flex size-10 items-center justify-center rounded-lg bg-white/10 text-white/90'>
                      {step.icon}
                    </span>
                    <span className='text-4xl font-light text-white/30' style={{ fontFamily: "'Silkscreen', cursive" }}>
                      {step.num}
                    </span>
                  </div>
                  <h3 className='mb-1 text-lg font-semibold text-white'>{step.title}</h3>
                  <p className='text-sm text-white/60'>{step.titleZh}</p>
                  <p className='mt-3 text-sm leading-relaxed text-white/70'>{step.desc}</p>
                  <p className='mt-1 text-xs text-white/50'>{step.descZh}</p>
                </div>
              ))}
            </div>
          </div>
        </section>

        {/* Core Features */}
        <section className='border-t border-white/10 px-5 py-16 sm:px-8 sm:py-20 lg:px-12 lg:py-24'>
          <div className='mx-auto max-w-6xl'>
            <div className='mb-10 text-center sm:mb-12 lg:mb-14'>
              <p className='mb-2 text-xs font-semibold uppercase tracking-widest text-white/50'>
                {t('Core Features')}
              </p>
              <h2 className='text-2xl font-semibold tracking-tight text-white sm:text-3xl lg:text-4xl'>
                企业级中转能力
              </h2>
            </div>
            <div className='grid gap-6 sm:gap-8 md:grid-cols-3'>
              {features.map((feat) => (
                <div
                  key={feat.title}
                  className='flex flex-col items-center rounded-2xl border border-white/10 bg-white/5 p-6 text-center backdrop-blur-lg sm:p-7'
                >
                  <div className='mb-4 flex size-14 items-center justify-center rounded-full bg-white/10 text-white/90'>
                    {feat.icon}
                  </div>
                  <h3 className='mb-1 text-lg font-semibold text-white'>{feat.title}</h3>
                  <p className='mb-3 text-sm text-white/60'>{feat.titleZh}</p>
                  <p className='text-sm leading-relaxed text-white/70'>{feat.desc}</p>
                  <p className='mt-1 text-xs text-white/50'>{feat.descZh}</p>
                </div>
              ))}
            </div>
          </div>
        </section>
      </div>
    </>
  )
}
