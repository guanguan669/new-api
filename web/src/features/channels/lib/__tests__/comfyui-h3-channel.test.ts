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
import { describe, test } from 'node:test'

import { CHANNEL_TYPE_COMFYUI_H3 } from '../../constants'
import type { Channel } from '../../types'
import {
  CHANNEL_FORM_DEFAULT_VALUES,
  channelFormSchema,
  transformChannelToFormDefaults,
  transformFormDataToCreatePayload,
} from '../channel-form'

function comfyUIH3Form(workerURLs = '') {
  return {
    ...CHANNEL_FORM_DEFAULT_VALUES,
    name: 'ComfyUI H3',
    type: CHANNEL_TYPE_COMFYUI_H3,
    base_url: 'http://comfy-fallback.internal:8188',
    key: 'comfyui-no-auth',
    models: 'minimax_h3',
    comfyui_h3_backend_urls: workerURLs,
  }
}

describe('ComfyUI H3 worker URLs', () => {
  test('parses an existing JSON array into one URL per line', () => {
    const defaults = transformChannelToFormDefaults({
      id: 63,
      name: 'ComfyUI H3',
      type: CHANNEL_TYPE_COMFYUI_H3,
      status: 1,
      models: 'minimax_h3',
      group: 'default',
      settings:
        '{"comfyui_h3_backend_urls":["http://worker-1:8188","https://worker-2.example"]}',
      channel_info: {
        is_multi_key: false,
        multi_key_size: 0,
        multi_key_polling_index: 0,
        multi_key_mode: 'random',
      },
    } as Channel)

    assert.equal(
      defaults.comfyui_h3_backend_urls,
      'http://worker-1:8188\nhttps://worker-2.example'
    )
  })

  test('validates every configured worker URL', () => {
    assert.equal(
      channelFormSchema.safeParse(
        comfyUIH3Form('http://worker-1:8188\nhttps://worker-2.example/')
      ).success,
      true
    )
    assert.equal(
      channelFormSchema.safeParse(
        comfyUIH3Form('http://worker-1:8188\nnot-a-url')
      ).success,
      false
    )
    assert.equal(
      channelFormSchema.safeParse({
        ...comfyUIH3Form('not-a-url'),
        type: 1,
      }).success,
      true
    )
  })

  test('serializes a normalized array and removes it for other types', () => {
    const payload = transformFormDataToCreatePayload({
      ...comfyUIH3Form(
        ' http://worker-1:8188/ \nhttps://worker-2.example///\nhttp://worker-1:8188'
      ),
      settings: '{"preserved":true}',
    })
    assert.deepEqual(JSON.parse(payload.channel.settings || '{}'), {
      preserved: true,
      comfyui_h3_backend_urls: [
        'http://worker-1:8188',
        'https://worker-2.example',
      ],
      disable_task_polling_sleep: false,
      upstream_model_update_check_enabled: false,
      upstream_model_update_auto_sync_enabled: false,
      upstream_model_update_ignored_models: [],
      upstream_model_update_last_detected_models: [],
      upstream_model_update_last_check_time: 0,
    })

    const otherPayload = transformFormDataToCreatePayload({
      ...comfyUIH3Form('http://worker-1:8188'),
      type: 1,
      settings:
        '{"preserved":true,"comfyui_h3_backend_urls":["http://old-worker:8188"]}',
    })
    const otherSettings = JSON.parse(otherPayload.channel.settings || '{}')
    assert.equal('comfyui_h3_backend_urls' in otherSettings, false)
  })
})
