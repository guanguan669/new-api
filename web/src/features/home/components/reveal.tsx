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
import type { ReactNode } from 'react'
import { cn } from '@/lib/utils'
import { useInView } from '../hooks/use-in-view'

type Variant = 'fade-up' | 'fade-in' | 'scale-in' | 'fade-left' | 'fade-right'

const VARIANT_CLASS: Record<Variant, string> = {
  'fade-up': 'landing-animate-fade-up',
  'fade-in': 'landing-animate-fade-in',
  'scale-in': 'landing-animate-scale-in',
  'fade-left': 'landing-animate-fade-left',
  'fade-right': 'landing-animate-fade-right',
}

export function Reveal({
  children,
  variant = 'fade-up',
  delay = 0,
  className,
}: {
  children: ReactNode
  variant?: Variant
  delay?: number
  className?: string
}) {
  const { ref, inView } = useInView<HTMLDivElement>()

  return (
    <div
      ref={ref}
      data-reveal
      className={cn(className, inView ? VARIANT_CLASS[variant] : 'opacity-0')}
      style={inView && delay ? { animationDelay: `${delay}ms` } : undefined}
    >
      {children}
    </div>
  )
}
