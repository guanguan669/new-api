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
import { Shield, Lock, Globe, Server, Zap, HelpCircle, Check } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Reveal } from './reveal'

const GLASS_CARD = 'group relative overflow-hidden rounded-[2.5rem] border border-slate-200 bg-white p-10 shadow-sm transition-all duration-500 hover:border-slate-300 hover:shadow-[0_20px_60px_-15px_rgba(6,182,212,0.15)]'

const SECTION_LABEL = 'inline-flex items-center gap-2.5 rounded-full border border-slate-200 bg-white px-5 py-2.5 mb-8 shadow-sm'

const GLOW_ORB = 'absolute pointer-events-none transition-all duration-700 rounded-full blur-[120px] opacity-20 group-hover:opacity-30 group-hover:scale-110'

const ICON_WELL = 'relative flex h-14 w-14 items-center justify-center rounded-2xl bg-gradient-to-br from-cyan-500/10 to-blue-500/10 border border-slate-200'

const FAQ_ITEMS = [
  {
    q: 'How do you handle high-concurrency requests?',
    a: 'AetherVision employs a distributed gateway architecture with load balancing, supporting 100K+ QPS with millisecond latency. Elastic scaling automatically handles traffic spikes to ensure service stability.',
  },
  {
    q: 'Which AI models are supported?',
    a: 'Unified access to 100+ mainstream AI models including Claude, GPT, Gemini, Kimi, GLM, Qwen, DeepSeek, and more, with continuous integration of latest releases. All models are accessible through a unified OpenAI-compatible API.',
  },
  {
    q: 'How fast is failover?',
    a: 'Millisecond-level automatic failover. Multi-active architecture monitors channel health in real-time and automatically switches to backup channels upon anomalies, ensuring zero downtime.',
  },
  {
    q: 'How is data security ensured?',
    a: 'End-to-end TLS 1.3 encryption in transit and AES-256 at rest. Zero data persistence policy means no user request content is stored. SOC 2 Type II and GDPR compliant.',
  },
  {
    q: 'Is private deployment supported?',
    a: 'Enterprise-grade private deployment and dedicated resource reservation are supported. Custom deployment solutions tailored to business needs with dedicated technical support and SLA guarantees.',
  },
  {
    q: 'How to integrate with existing systems?',
    a: 'Fully compatible with OpenAI API protocol—switch with one line of code. SDKs available for Python, Node.js, Go, Java, and more, with detailed documentation and code examples for rapid integration.',
  },
]

export function DetailedSections() {
  const { t } = useTranslation()

  return (
    <div className='w-full bg-[#FAFBFC] relative overflow-hidden'>
      <div className='absolute inset-0 overflow-hidden pointer-events-none'>
        <div className='absolute top-[20%] left-[10%] w-[800px] h-[800px] bg-cyan-500/[0.05] rounded-full blur-[150px]' />
        <div className='absolute bottom-[10%] right-[15%] w-[600px] h-[600px] bg-blue-500/[0.04] rounded-full blur-[140px]' />
      </div>

      {/* Section: Security & Compliance */}
      <section className='relative py-32 px-6'>
        <div className='mx-auto max-w-7xl'>
          <Reveal className='mb-20 text-center'>
            <div className={SECTION_LABEL}>
              <Shield className='h-4 w-4 text-cyan-600' strokeWidth={2} />
              <span className='text-sm font-medium tracking-tight text-slate-700'>{t('home.security.label')}</span>
            </div>
            <h2 className='text-5xl md:text-6xl font-bold tracking-tighter leading-none bg-gradient-to-br from-slate-900 via-slate-800 to-slate-600 bg-clip-text text-transparent mb-5'>
              {t('home.security.titleNew')}
            </h2>
            <p className='text-lg text-slate-500 max-w-[65ch] mx-auto leading-relaxed'>
              {t('home.security.subtitleNew')}
            </p>
          </Reveal>

          <div className='grid grid-cols-1 md:grid-cols-3 gap-8'>
            <Reveal delay={0} className='h-full'>
              <div className={`${GLASS_CARD} h-full`}>
                <div className={`${GLOW_ORB} -top-12 -right-12 w-56 h-56 bg-cyan-500/15`} />
                <div className='relative'>
                  <div className={ICON_WELL}>
                    <Lock className='h-6 w-6 text-cyan-600' strokeWidth={2} />
                  </div>
                  <h3 className='text-xl font-bold tracking-tight text-slate-900 mt-6 mb-3'>{t('home.security.encryption.titleNew')}</h3>
                  <p className='text-slate-500 text-sm leading-relaxed'>{t('home.security.encryption.descNew')}</p>
                </div>
              </div>
            </Reveal>

            <Reveal delay={120} className='h-full'>
              <div className={`${GLASS_CARD} h-full`}>
                <div className={`${GLOW_ORB} -top-12 -right-12 w-56 h-56 bg-emerald-500/15`} />
                <div className='relative'>
                  <div className={`${ICON_WELL} bg-gradient-to-br from-emerald-500/10 to-green-500/10`}>
                    <Shield className='h-6 w-6 text-emerald-600' strokeWidth={2} />
                  </div>
                  <h3 className='text-xl font-bold tracking-tight text-slate-900 mt-6 mb-3'>{t('home.security.privacy.titleNew')}</h3>
                  <p className='text-slate-500 text-sm leading-relaxed'>{t('home.security.privacy.descNew')}</p>
                </div>
              </div>
            </Reveal>

            <Reveal delay={240} className='h-full'>
              <div className={`${GLASS_CARD} h-full`}>
                <div className={`${GLOW_ORB} -top-12 -right-12 w-56 h-56 bg-blue-500/15`} />
                <div className='relative'>
                  <div className={`${ICON_WELL} bg-gradient-to-br from-blue-500/10 to-indigo-500/10`}>
                    <Globe className='h-6 w-6 text-blue-600' strokeWidth={2} />
                  </div>
                  <h3 className='text-xl font-bold tracking-tight text-slate-900 mt-6 mb-3'>{t('home.security.compliance.titleNew')}</h3>
                  <p className='text-slate-500 text-sm leading-relaxed'>{t('home.security.compliance.descNew')}</p>
                </div>
              </div>
            </Reveal>
          </div>
        </div>
      </section>

      {/* Section: Technical Architecture */}
      <section className='relative py-32 px-6'>
        <div className='mx-auto max-w-7xl'>
          <Reveal className='mb-20 text-center'>
            <div className={SECTION_LABEL}>
              <Server className='h-4 w-4 text-cyan-600' strokeWidth={2} />
              <span className='text-sm font-medium tracking-tight text-slate-700'>{t('home.architecture.label')}</span>
            </div>
            <h2 className='text-5xl md:text-6xl font-bold tracking-tighter leading-none bg-gradient-to-br from-slate-900 via-slate-800 to-slate-600 bg-clip-text text-transparent mb-5'>
              {t('home.architecture.titleNew')}
            </h2>
            <p className='text-lg text-slate-500 max-w-[65ch] mx-auto leading-relaxed'>
              {t('home.architecture.subtitleNew')}
            </p>
          </Reveal>

          <div className='grid grid-cols-1 md:grid-cols-2 gap-8'>
            <Reveal variant='fade-left' className='h-full'>
              <div className={`${GLASS_CARD} h-full`}>
                <div className={`${GLOW_ORB} top-0 left-0 w-64 h-64 bg-cyan-500/10`} />
                <div className='relative'>
                  <div className={ICON_WELL}>
                    <Server className='h-6 w-6 text-cyan-600' strokeWidth={2} />
                  </div>
                  <h3 className='text-2xl font-bold tracking-tight text-slate-900 mt-6 mb-4'>{t('home.architecture.distributed.titleNew')}</h3>
                  <p className='text-slate-500 leading-relaxed mb-4'>{t('home.architecture.distributed.descNew')}</p>
                  <ul className='space-y-2 text-sm text-slate-600'>
                    <li className='flex items-start gap-2'>
                      <Check className='h-4 w-4 text-cyan-600 flex-shrink-0 mt-0.5' strokeWidth={2} />
                      <span>{t('home.architecture.distributed.point1New')}</span>
                    </li>
                    <li className='flex items-start gap-2'>
                      <Check className='h-4 w-4 text-cyan-600 flex-shrink-0 mt-0.5' strokeWidth={2} />
                      <span>{t('home.architecture.distributed.point2New')}</span>
                    </li>
                    <li className='flex items-start gap-2'>
                      <Check className='h-4 w-4 text-cyan-600 flex-shrink-0 mt-0.5' strokeWidth={2} />
                      <span>{t('home.architecture.distributed.point3New')}</span>
                    </li>
                  </ul>
                </div>
              </div>
            </Reveal>

            <Reveal variant='fade-right' className='h-full'>
              <div className={`${GLASS_CARD} h-full`}>
                <div className={`${GLOW_ORB} top-0 right-0 w-64 h-64 bg-blue-500/10`} />
                <div className='relative'>
                  <div className={`${ICON_WELL} bg-gradient-to-br from-blue-500/10 to-indigo-500/10`}>
                    <Zap className='h-6 w-6 text-blue-600' strokeWidth={2} />
                  </div>
                  <h3 className='text-2xl font-bold tracking-tight text-slate-900 mt-6 mb-4'>{t('home.architecture.intelligent.titleNew')}</h3>
                  <p className='text-slate-500 leading-relaxed mb-4'>{t('home.architecture.intelligent.descNew')}</p>
                  <ul className='space-y-2 text-sm text-slate-600'>
                    <li className='flex items-start gap-2'>
                      <Check className='h-4 w-4 text-blue-600 flex-shrink-0 mt-0.5' strokeWidth={2} />
                      <span>{t('home.architecture.intelligent.point1New')}</span>
                    </li>
                    <li className='flex items-start gap-2'>
                      <Check className='h-4 w-4 text-blue-600 flex-shrink-0 mt-0.5' strokeWidth={2} />
                      <span>{t('home.architecture.intelligent.point2New')}</span>
                    </li>
                    <li className='flex items-start gap-2'>
                      <Check className='h-4 w-4 text-blue-600 flex-shrink-0 mt-0.5' strokeWidth={2} />
                      <span>{t('home.architecture.intelligent.point3New')}</span>
                    </li>
                  </ul>
                </div>
              </div>
            </Reveal>
          </div>
        </div>
      </section>

      {/* Section: FAQ */}
      <section className='relative py-32 px-6'>
        <div className='mx-auto max-w-7xl'>
          <Reveal className='mb-20 text-center'>
            <div className={SECTION_LABEL}>
              <HelpCircle className='h-4 w-4 text-cyan-600' strokeWidth={2} />
              <span className='text-sm font-medium tracking-tight text-slate-700'>{t('home.faq.label')}</span>
            </div>
            <h2 className='text-5xl md:text-6xl font-bold tracking-tighter leading-none bg-gradient-to-br from-slate-900 via-slate-800 to-slate-600 bg-clip-text text-transparent mb-5'>
              {t('home.faq.titleNew')}
            </h2>
            <p className='text-lg text-slate-500 max-w-[65ch] mx-auto leading-relaxed'>
              {t('home.faq.subtitleNew')}
            </p>
          </Reveal>

          <div className='max-w-4xl mx-auto space-y-6'>
            {FAQ_ITEMS.map((item, index) => (
              <Reveal key={item.q} delay={(index % 3) * 100}>
                <div className={GLASS_CARD}>
                  <div className='relative'>
                    <div className='flex items-start gap-5'>
                      <div className='flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-xl bg-gradient-to-br from-cyan-500/15 to-blue-500/15 border border-cyan-500/25'>
                        <span className='text-sm font-bold text-cyan-600'>Q{index + 1}</span>
                      </div>
                      <div className='flex-1 pt-1'>
                        <h3 className='text-lg font-bold tracking-tight text-slate-900 mb-3 group-hover:text-cyan-600 transition-colors duration-300'>
                          {t(`home.faq.q${index + 1}.questionNew`)}
                        </h3>
                        <p className='text-slate-500 text-sm leading-relaxed'>
                          {t(`home.faq.q${index + 1}.answerNew`)}
                        </p>
                      </div>
                    </div>
                  </div>
                </div>
              </Reveal>
            ))}
          </div>
        </div>
      </section>
    </div>
  )
}
