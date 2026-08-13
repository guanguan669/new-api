/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { buildModelPreviewPayload } from '../channel-mutate-drawer'

const formValues = {
  type: 1,
  key: 'new-key',
  base_url: ' https://upstream.example/v1/// ',
  advanced_custom: '{"routes":[]}',
  header_override: '{"X-Test":"current"}',
  proxy: 'http://proxy.example:8080',
}

describe('channel model preview payload', () => {
  test('uses the current create form configuration without a channel id', () => {
    assert.deepEqual(buildModelPreviewPayload(formValues), {
      type: 1,
      key: 'new-key',
      channel_id: undefined,
      base_url: 'https://upstream.example/v1',
      advanced_custom: '{"routes":[]}',
      header_override: '{"X-Test":"current"}',
      proxy: 'http://proxy.example:8080',
    })
  })

  test('reuses the saved key when the edit form key is blank', () => {
    const payload = buildModelPreviewPayload({ ...formValues, key: '   ' }, 42)

    assert.equal(Object.hasOwn(payload, 'key'), false)
    assert.deepEqual(payload, {
      type: 1,
      channel_id: 42,
      base_url: 'https://upstream.example/v1',
      advanced_custom: '{"routes":[]}',
      header_override: '{"X-Test":"current"}',
      proxy: 'http://proxy.example:8080',
    })
  })

  test('sends a newly entered key while editing', () => {
    assert.equal(buildModelPreviewPayload(formValues, 42).key, 'new-key')
  })
})
