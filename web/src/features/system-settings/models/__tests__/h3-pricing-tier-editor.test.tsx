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

const { act, useState } = await import('react')
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
    assert.deepEqual(Object.keys(JSON.parse(latestValue)), ['tier_1'])

    await act(async () => addButton.click())
    assert.deepEqual(Object.keys(JSON.parse(latestValue)), ['tier_1', 'tier_2'])

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
})
