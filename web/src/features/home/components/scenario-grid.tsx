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
import { Code2, Bot, BookOpen, Palette, MessageSquare, Search } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Reveal } from './reveal'

const SCENARIOS = [
  {
    icon: <Code2 className='size-6' strokeWidth={1.5} />,
    title: 'Coding',
    titleZh: 'Coding',
    desc: 'Code understanding, generation, and auto-completion with IDE plugin and CI integration support',
    descZh: '代码理解、生成与自动补全，支持 IDE 插件与 CI 集成',
  },
  {
    icon: <Bot className='size-6' strokeWidth={1.5} />,
    title: 'Agent',
    titleZh: 'Agent',
    desc: 'Multi-step reasoning, tool calling, and autonomous task execution',
    descZh: '多步推理、工具调用、自主任务执行',
  },
  {
    icon: <BookOpen className='size-6' strokeWidth={1.5} />,
    title: 'RAG',
    titleZh: 'RAG',
    desc: 'Knowledge base retrieval Q&A with precise long-context recall',
    descZh: '知识库检索问答，长上下文精准召回',
  },
  {
    icon: <Palette className='size-6' strokeWidth={1.5} />,
    title: 'Content Generation',
    titleZh: '内容生成',
    desc: 'Multimodal content creation across text, image, and video',
    descZh: '文本、图像、视频多模态内容创作',
  },
  {
    icon: <MessageSquare className='size-6' strokeWidth={1.5} />,
    title: 'AI Assistants',
    titleZh: 'AI 助手',
    desc: 'Workflow automation, intelligent customer service, and document review',
    descZh: '工作流自动化、智能客服、文档审查',
  },
  {
    icon: <Search className='size-6' strokeWidth={1.5} />,
    title: 'Search',
    titleZh: '搜索',
    desc: 'Query understanding, real-time answers, and long-context summarization',
    descZh: '查询理解、实时回答、长上下文总结',
  },
]

export function ScenarioGrid() {
  const { t } = useTranslation()

  return (
    <section className='border-t border-slate-200 px-5 py-16 sm:px-8 sm:py-20 lg:px-12 lg:py-24'>
      <div className='mx-auto max-w-6xl'>
        <Reveal className='mb-10 text-center sm:mb-12 lg:mb-14'>
          <p className='mb-2 text-xs font-semibold uppercase tracking-widest text-slate-500'>
            {t('home.scenarios.label')}
          </p>
          <h2 className='text-2xl font-semibold tracking-tight text-slate-900 sm:text-3xl lg:text-4xl'>
            {t('home.scenarios.title')}
          </h2>
          <p className='mt-3 text-sm text-slate-500 sm:text-base'>
            {t('home.scenarios.subtitle')}
          </p>
        </Reveal>
        <div className='grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3'>
          {SCENARIOS.map((scenario, i) => (
            <Reveal key={scenario.title} delay={(i % 3) * 90}>
              <div className='group flex h-full flex-col rounded-2xl border border-slate-200 bg-white p-6 shadow-sm transition-all duration-300 hover:border-slate-300 hover:shadow-md hover:shadow-cyan-500/5'>
                <div className='mb-4 flex size-12 items-center justify-center rounded-xl bg-slate-100 text-slate-700 transition-colors duration-300 group-hover:bg-cyan-500/10 group-hover:text-cyan-600'>
                  {scenario.icon}
                </div>
                <h3 className='mb-1 text-lg font-semibold text-slate-900'>
                  {t(scenario.title)}
                </h3>
                <p className='text-sm leading-relaxed text-slate-600'>
                  {t(scenario.desc)}
                </p>
              </div>
            </Reveal>
          ))}
        </div>
      </div>
    </section>
  )
}
