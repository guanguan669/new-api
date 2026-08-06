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

import {
  CHANNEL_TYPE_RUNNINGHUB,
  CHANNEL_TYPE_OPTIONS,
  MODEL_FETCHABLE_TYPES,
} from '../../constants'
import type { Channel } from '../../types'
import {
  CHANNEL_FORM_DEFAULT_VALUES,
  RUNNINGHUB_DEFAULT_WORKFLOW_ID,
  channelFormSchema,
  transformChannelToFormDefaults,
  transformFormDataToCreatePayload,
} from '../channel-form'
import { getChannelTypeConfig } from '../channel-type-config'
import { getChannelTypeIcon, getKeyPromptForType } from '../channel-utils'

function runningHubForm(workflowId = RUNNINGHUB_DEFAULT_WORKFLOW_ID) {
  return {
    ...CHANNEL_FORM_DEFAULT_VALUES,
    name: 'RunningHub H3',
    type: CHANNEL_TYPE_RUNNINGHUB,
    key: 'runninghub-api-key',
    models: 'minimax_h3',
    runninghub_workflow_id: workflowId,
  }
}

describe('RunningHub channel', () => {
  test('registers label, ordering, model discovery, and metadata', () => {
    const kuocaiIndex = CHANNEL_TYPE_OPTIONS.findIndex(
      (item) => item.value === 61
    )
    const runningHubIndex = CHANNEL_TYPE_OPTIONS.findIndex(
      (item) => item.value === CHANNEL_TYPE_RUNNINGHUB
    )

    assert.deepEqual(CHANNEL_TYPE_OPTIONS[runningHubIndex], {
      value: CHANNEL_TYPE_RUNNINGHUB,
      label: 'RunningHub',
    })
    assert.equal(runningHubIndex, kuocaiIndex + 1)
    assert.equal(MODEL_FETCHABLE_TYPES.has(CHANNEL_TYPE_RUNNINGHUB), true)
    assert.equal(getChannelTypeIcon(CHANNEL_TYPE_RUNNINGHUB), 'OpenAI')
    assert.equal(
      getKeyPromptForType(CHANNEL_TYPE_RUNNINGHUB),
      'Enter RunningHub API key for this channel'
    )
    assert.equal(
      getChannelTypeConfig(CHANNEL_TYPE_RUNNINGHUB).name,
      'RunningHub'
    )
  })

  test('requires a non-blank workflow ID', () => {
    const blankResult = channelFormSchema.safeParse(runningHubForm('  '))

    assert.equal(blankResult.success, false)
    if (!blankResult.success) {
      assert.equal(
        blankResult.error.issues.some(
          (issue) =>
            issue.path[0] === 'runninghub_workflow_id' &&
            issue.message === 'RunningHub Workflow ID is required'
        ),
        true
      )
    }

    assert.equal(channelFormSchema.safeParse(runningHubForm()).success, true)
  })

  test('builds and cleans RunningHub workflow settings', () => {
    const runningHubPayload = transformFormDataToCreatePayload({
      ...runningHubForm('  123456  '),
      settings: '{"preserved":true,"runninghub_workflow_id":"old"}',
    })
    assert.deepEqual(JSON.parse(runningHubPayload.channel.settings || '{}'), {
      preserved: true,
      runninghub_workflow_id: '123456',
      disable_task_polling_sleep: false,
      upstream_model_update_check_enabled: false,
      upstream_model_update_auto_sync_enabled: false,
      upstream_model_update_ignored_models: [],
      upstream_model_update_last_detected_models: [],
      upstream_model_update_last_check_time: 0,
    })

    const otherPayload = transformFormDataToCreatePayload({
      ...runningHubForm('123456'),
      type: 1,
      settings: '{"preserved":true,"runninghub_workflow_id":"old"}',
    })
    assert.deepEqual(JSON.parse(otherPayload.channel.settings || '{}'), {
      preserved: true,
      allow_service_tier: false,
      disable_store: false,
      allow_safety_identifier: false,
      allow_include_obfuscation: false,
      allow_inference_geo: false,
      disable_task_polling_sleep: false,
      upstream_model_update_check_enabled: false,
      upstream_model_update_auto_sync_enabled: false,
      upstream_model_update_ignored_models: [],
      upstream_model_update_last_detected_models: [],
      upstream_model_update_last_check_time: 0,
    })
  })

  test('parses workflow ID from existing channel settings', () => {
    const formDefaults = transformChannelToFormDefaults({
      id: 62,
      name: 'RunningHub H3',
      type: CHANNEL_TYPE_RUNNINGHUB,
      key: '',
      status: 1,
      models: 'minimax_h3',
      group: 'default',
      settings: '{"runninghub_workflow_id":"987654"}',
      channel_info: {
        is_multi_key: false,
        multi_key_size: 0,
        multi_key_polling_index: 0,
        multi_key_mode: 'random',
      },
    } as Channel)

    assert.equal(formDefaults.runninghub_workflow_id, '987654')
  })
})
