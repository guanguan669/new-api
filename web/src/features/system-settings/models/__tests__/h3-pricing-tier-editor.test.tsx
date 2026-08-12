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
import assert from 'node:assert/strict'
import { after, describe, test } from 'node:test'

import { Window } from 'happy-dom'

const domWindow = new Window()
const domGlobals = [
  'window',
  'document',
  'navigator',
  'HTMLElement',
  'HTMLInputElement',
  'SVGElement',
  'Node',
  'Element',
  'Event',
  'CustomEvent',
  'MutationObserver',
  'requestAnimationFrame',
  'cancelAnimationFrame',
  'getComputedStyle',
] as const

for (const key of domGlobals) {
  Object.defineProperty(globalThis, key, {
    configurable: true,
    value: domWindow[key],
  })
}

const React = await import('react')
const { act, useState } = React
Object.defineProperty(globalThis, 'React', {
  configurable: true,
  value: React,
})
const { createRoot } = await import('react-dom/client')
const { QueryClient, QueryClientProvider } =
  await import('@tanstack/react-query')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { RunningHubH3GroupPriceEditor } =
  await import('../group-ratio-visual-editor')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        'Add pricing tier': 'Add pricing tier',
        'Pricing tier name (optional; blank creates tier_N)':
          'Pricing tier name (optional; blank creates tier_N)',
      },
    },
  },
})

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

function changeInputValue(input: HTMLInputElement, value: string) {
  const valueSetter = Object.getOwnPropertyDescriptor(
    domWindow.HTMLInputElement.prototype,
    'value'
  )?.set
  assert.ok(valueSetter)
  valueSetter.call(input, value)
  input.dispatchEvent(
    new domWindow.Event('input', { bubbles: true }) as unknown as Event
  )
}

describe('H3 pricing tier editor', () => {
  after(() => {
    domWindow.close()
  })

  test('adds unlimited tiers without requiring a custom name', async () => {
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })
    queryClient.setQueryData(['h3-price-groups'], { success: true, data: [] })

    let latestValue = '{}'
    function Harness() {
      const [value, setValue] = useState('{}')
      return (
        <RunningHubH3GroupPriceEditor
          value={value}
          groupOptions={['default', 'vip']}
          onChange={(_, nextValue) => {
            latestValue = nextValue
            setValue(nextValue)
          }}
        />
      )
    }

    await act(async () => {
      root.render(
        <QueryClientProvider client={queryClient}>
          <I18nextProvider i18n={i18n}>
            <Harness />
          </I18nextProvider>
        </QueryClientProvider>
      )
    })

    const addButton = [...container.querySelectorAll('button')].find(
      (button) => button.textContent === 'Add pricing tier'
    )
    assert.ok(addButton)
    assert.equal(addButton.disabled, false)

    await act(async () => addButton.click())
    assert.deepEqual(JSON.parse(latestValue), {
      tier_1: { price_768p: 0.1, price_2k: 0.3, group: 'default' },
    })

    await act(async () => addButton.click())
    assert.deepEqual(JSON.parse(latestValue), {
      tier_1: { price_768p: 0.1, price_2k: 0.3, group: 'default' },
      tier_2: { price_768p: 0.1, price_2k: 0.3, group: 'default' },
    })

    const nameInput = container.querySelector<HTMLInputElement>(
      'input[aria-label="Pricing tier name (optional; blank creates tier_N)"]'
    )
    assert.ok(nameInput)

    await act(async () => changeInputValue(nameInput, 'tier_1'))
    assert.equal(addButton.disabled, true)

    await act(async () => changeInputValue(nameInput, 'auto'))
    assert.equal(addButton.disabled, true)

    await act(async () => changeInputValue(nameInput, 'custom-tier'))
    assert.equal(addButton.disabled, false)

    await act(async () => root.unmount())
    container.remove()
    queryClient.clear()
  })

  test('shows the normal routing group bound to each pricing tier', async () => {
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })
    queryClient.setQueryData(['h3-price-groups'], {
      success: true,
      data: [
        {
          group: 'dance-tier',
          bound_group: 'h3-route',
          price_768p: 0.04,
          price_2k: 0.3,
          user_count: 0,
        },
      ],
    })

    await act(async () => {
      root.render(
        <QueryClientProvider client={queryClient}>
          <I18nextProvider i18n={i18n}>
            <RunningHubH3GroupPriceEditor
              value={JSON.stringify({
                'dance-tier': {
                  group: 'h3-route',
                  price_768p: 0.04,
                  price_2k: 0.3,
                },
              })}
              groupOptions={['default', 'h3-route']}
              onChange={() => undefined}
            />
          </I18nextProvider>
        </QueryClientProvider>
      )
    })

    const groupTrigger = container.querySelector<HTMLElement>(
      '[aria-label="Bound normal group: dance-tier"]'
    )
    assert.ok(groupTrigger)
    assert.match(groupTrigger.textContent ?? '', /h3-route/)

    await act(async () => root.unmount())
    container.remove()
    queryClient.clear()
  })
})
