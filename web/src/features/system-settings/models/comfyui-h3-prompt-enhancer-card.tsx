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
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useMemo, useRef } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import * as z from 'zod'

import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'

import {
  getComfyUIH3PromptEnhancerChannels,
  updateComfyUIH3PromptEnhancer,
} from '../api'
import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { safeNumberFieldProps } from '../utils/numeric-field'

const schema = z
  .object({
    enabled: z.boolean(),
    provider_mode: z.enum(['channel', 'direct']),
    channel_id: z.coerce.number().int().min(0),
    base_url: z.string(),
    api_key: z.string(),
    clear_api_key: z.boolean(),
    model: z.string(),
    timeout_seconds: z.coerce.number().int().min(1).max(300),
    system_prompt: z.string(),
  })
  .superRefine((values, ctx) => {
    if (!values.enabled) return
    if (values.provider_mode === 'channel') {
      if (values.channel_id <= 0) {
        ctx.addIssue({
          code: 'custom',
          path: ['channel_id'],
          message: 'Required',
        })
      }
    } else if (!values.base_url.trim()) {
      ctx.addIssue({ code: 'custom', path: ['base_url'], message: 'Required' })
    } else {
      try {
        const url = new URL(values.base_url)
        if (url.protocol !== 'http:' && url.protocol !== 'https:') throw null
      } catch {
        ctx.addIssue({
          code: 'custom',
          path: ['base_url'],
          message: 'Enter a valid HTTP or HTTPS URL',
        })
      }
    }
    if (!values.model.trim()) {
      ctx.addIssue({ code: 'custom', path: ['model'], message: 'Required' })
    }
    if (!values.system_prompt.trim()) {
      ctx.addIssue({
        code: 'custom',
        path: ['system_prompt'],
        message: 'Required',
      })
    }
  })

type FormInput = z.input<typeof schema>
type FormValues = z.output<typeof schema>

export type ComfyUIH3PromptEnhancerDefaults = {
  'comfyui_h3_prompt_enhancer.enabled': boolean
  'comfyui_h3_prompt_enhancer.provider_mode'?: 'channel' | 'direct'
  'comfyui_h3_prompt_enhancer.channel_id'?: number
  'comfyui_h3_prompt_enhancer.base_url': string
  'comfyui_h3_prompt_enhancer.api_key': string
  'comfyui_h3_prompt_enhancer.model': string
  'comfyui_h3_prompt_enhancer.timeout_seconds': number
  'comfyui_h3_prompt_enhancer.system_prompt': string
}

const buildFormDefaults = (
  defaults: ComfyUIH3PromptEnhancerDefaults
): FormInput => ({
  enabled: defaults['comfyui_h3_prompt_enhancer.enabled'],
  provider_mode:
    defaults['comfyui_h3_prompt_enhancer.provider_mode'] ??
    (defaults['comfyui_h3_prompt_enhancer.base_url'] ? 'direct' : 'channel'),
  channel_id: defaults['comfyui_h3_prompt_enhancer.channel_id'] ?? 0,
  base_url: defaults['comfyui_h3_prompt_enhancer.base_url'] ?? '',
  api_key: '',
  clear_api_key: false,
  model: defaults['comfyui_h3_prompt_enhancer.model'] ?? '',
  timeout_seconds: defaults['comfyui_h3_prompt_enhancer.timeout_seconds'] ?? 8,
  system_prompt: defaults['comfyui_h3_prompt_enhancer.system_prompt'] ?? '',
})

const normalizeValues = (values: FormInput | FormValues) => ({
  'comfyui_h3_prompt_enhancer.enabled': values.enabled,
  'comfyui_h3_prompt_enhancer.provider_mode': values.provider_mode,
  'comfyui_h3_prompt_enhancer.channel_id': Number(values.channel_id),
  'comfyui_h3_prompt_enhancer.base_url': values.base_url.trim(),
  'comfyui_h3_prompt_enhancer.model': values.model.trim(),
  'comfyui_h3_prompt_enhancer.timeout_seconds': Number(values.timeout_seconds),
  'comfyui_h3_prompt_enhancer.system_prompt': values.system_prompt.trim(),
})

export function ComfyUIH3PromptEnhancerCard({
  defaultValues,
}: {
  defaultValues: ComfyUIH3PromptEnhancerDefaults
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const updateSettings = useMutation({
    mutationFn: updateComfyUIH3PromptEnhancer,
    onSuccess: (response) => {
      if (!response.success) return
      queryClient.invalidateQueries({ queryKey: ['system-options'] })
      toast.success(t('Setting updated successfully'))
    },
    onError: (error: Error) => {
      toast.error(error.message || t('Failed to update setting'))
    },
  })
  const formDefaults = useMemo(
    () => buildFormDefaults(defaultValues),
    [defaultValues]
  )
  const form = useForm<FormInput, unknown, FormValues>({
    resolver: zodResolver(schema),
    defaultValues: formDefaults,
  })
  const baselineRef = useRef(normalizeValues(formDefaults))
  const baselineSerializedRef = useRef(JSON.stringify(defaultValues))

  useEffect(() => {
    const serialized = JSON.stringify(defaultValues)
    if (serialized === baselineSerializedRef.current) return
    baselineRef.current = normalizeValues(formDefaults)
    baselineSerializedRef.current = serialized
    form.reset(formDefaults)
  }, [defaultValues, form, formDefaults])

  const enabled = form.watch('enabled')
  const providerMode = form.watch('provider_mode')
  const channelId = form.watch('channel_id')
  const channelsQuery = useQuery({
    queryKey: ['comfyui-h3-prompt-enhancer-channels'],
    queryFn: getComfyUIH3PromptEnhancerChannels,
    enabled: enabled && providerMode === 'channel',
  })
  const channels = useMemo(
    () => channelsQuery.data?.data ?? [],
    [channelsQuery.data?.data]
  )
  const selectedChannel = channels.find((channel) => channel.id === channelId)
  const channelModels = selectedChannel?.models ?? []

  const onSubmit = async (values: FormValues) => {
    const normalized = normalizeValues(values)
    const hasConfigChanges = Object.entries(normalized).some(
      ([key, value]) =>
        value !== baselineRef.current[key as keyof typeof baselineRef.current]
    )
    const apiKey = values.api_key.trim()

    if (!hasConfigChanges && !apiKey && !values.clear_api_key) {
      toast.info(t('No changes to save'))
      return
    }

    try {
      const response = await updateSettings.mutateAsync({
        enabled: normalized['comfyui_h3_prompt_enhancer.enabled'],
        provider_mode: normalized['comfyui_h3_prompt_enhancer.provider_mode'],
        channel_id: normalized['comfyui_h3_prompt_enhancer.channel_id'],
        base_url: normalized['comfyui_h3_prompt_enhancer.base_url'],
        api_key: apiKey,
        clear_api_key: values.clear_api_key,
        model: normalized['comfyui_h3_prompt_enhancer.model'],
        timeout_seconds:
          normalized['comfyui_h3_prompt_enhancer.timeout_seconds'],
        system_prompt: normalized['comfyui_h3_prompt_enhancer.system_prompt'],
      })
      if (!response.success) {
        toast.error(response.message || t('Failed to update setting'))
        return
      }

      baselineRef.current = normalized
      form.reset({ ...values, api_key: '', clear_api_key: false })
    } catch {
      // Keep the dirty form and baseline so the operator can correct the
      // failed setting without silently losing the entered system prompt.
    }
  }

  return (
    <SettingsSection title={t('Self-hosted H3 Prompt Enhancement')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)} autoComplete='off'>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateSettings.isPending}
          />

          <FormField
            control={form.control}
            name='enabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Enable H3 prompt enhancement')}</FormLabel>
                  <FormDescription>
                    {t(
                      'Enhance self-hosted H3 video prompts before submitting the ComfyUI workflow.'
                    )}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />

          <FormField
            control={form.control}
            name='provider_mode'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Prompt enhancer source')}</FormLabel>
                <Select
                  items={[
                    {
                      value: 'channel',
                      label: t('New API channel (recommended)'),
                    },
                    { value: 'direct', label: t('Direct endpoint') },
                  ]}
                  value={field.value}
                  onValueChange={field.onChange}
                  disabled={!enabled}
                >
                  <FormControl>
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                  </FormControl>
                  <SelectContent alignItemWithTrigger={false}>
                    <SelectGroup>
                      <SelectItem value='channel'>
                        {t('New API channel (recommended)')}
                      </SelectItem>
                      <SelectItem value='direct'>
                        {t('Direct endpoint')}
                      </SelectItem>
                    </SelectGroup>
                  </SelectContent>
                </Select>
                <FormDescription>
                  {t(
                    'Use a configured New API channel or connect to an OpenAI-compatible endpoint directly.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          {providerMode === 'channel' ? (
            <>
              <FormField
                control={form.control}
                name='channel_id'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Enhancer channel')}</FormLabel>
                    <Select
                      items={channels.map((channel) => ({
                        value: String(channel.id),
                        label: channel.name,
                      }))}
                      value={Number(field.value) > 0 ? String(field.value) : ''}
                      onValueChange={(value) => {
                        field.onChange(Number(value))
                        form.setValue('model', '', {
                          shouldDirty: true,
                          shouldValidate: true,
                        })
                      }}
                      disabled={!enabled || channelsQuery.isPending}
                    >
                      <FormControl>
                        <SelectTrigger>
                          <SelectValue
                            placeholder={
                              channelsQuery.isPending
                                ? t('Loading channels...')
                                : t('Select a channel')
                            }
                          />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent alignItemWithTrigger={false}>
                        <SelectGroup>
                          {channels.map((channel) => (
                            <SelectItem
                              key={channel.id}
                              value={String(channel.id)}
                            >
                              {channel.name}
                            </SelectItem>
                          ))}
                        </SelectGroup>
                      </SelectContent>
                    </Select>
                    <FormDescription>
                      {channelsQuery.isError
                        ? t('Failed to load enhancer channels.')
                        : t('The request uses this channel and its saved key.')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='model'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Enhancer Model')}</FormLabel>
                    <Select
                      items={channelModels.map((model) => ({
                        value: model,
                        label: model,
                      }))}
                      value={field.value}
                      onValueChange={field.onChange}
                      disabled={!enabled || !selectedChannel}
                    >
                      <FormControl>
                        <SelectTrigger>
                          <SelectValue placeholder={t('Select a model')} />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent alignItemWithTrigger={false}>
                        <SelectGroup>
                          {channelModels.map((model) => (
                            <SelectItem key={model} value={model}>
                              {model}
                            </SelectItem>
                          ))}
                        </SelectGroup>
                      </SelectContent>
                    </Select>
                    <FormDescription>
                      {t('Use the model name configured on this channel.')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </>
          ) : (
            <>
              <FormField
                control={form.control}
                name='base_url'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Enhancer API Base URL')}</FormLabel>
                    <FormControl>
                      <Input
                        placeholder='https://example.com/v1'
                        disabled={!enabled}
                        {...field}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('OpenAI-compatible chat completions endpoint.')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='model'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Enhancer Model')}</FormLabel>
                    <FormControl>
                      <Input disabled={!enabled} {...field} />
                    </FormControl>
                    <FormDescription>
                      {t('Use a model that supports image inputs.')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='api_key'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Enhancer API Key')}</FormLabel>
                    <FormControl>
                      <Input
                        type='password'
                        autoComplete='new-password'
                        placeholder={t('Leave blank to keep the existing key')}
                        disabled={!enabled}
                        {...field}
                        onChange={(event) => {
                          field.onChange(event)
                          if (event.target.value.trim()) {
                            form.setValue('clear_api_key', false, {
                              shouldDirty: true,
                              shouldValidate: true,
                            })
                          }
                        }}
                      />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'The saved key is never returned by the settings API.'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='clear_api_key'
                render={({ field }) => (
                  <SettingsSwitchItem>
                    <SettingsSwitchContent>
                      <FormLabel>{t('Clear saved enhancer API key')}</FormLabel>
                      <FormDescription>
                        {t(
                          'Explicitly remove the saved key when these settings are saved.'
                        )}
                      </FormDescription>
                    </SettingsSwitchContent>
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={(checked) => {
                          field.onChange(checked)
                          if (checked) {
                            form.setValue('api_key', '', {
                              shouldDirty: true,
                              shouldValidate: true,
                            })
                          }
                        }}
                      />
                    </FormControl>
                  </SettingsSwitchItem>
                )}
              />
            </>
          )}

          <FormField
            control={form.control}
            name='timeout_seconds'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Enhancement Timeout (seconds)')}</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min={1}
                    max={300}
                    step={1}
                    disabled={!enabled}
                    {...safeNumberFieldProps(field)}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'When enhancement fails or times out, the original prompt is used.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='system_prompt'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('H3 Context-IR System Prompt')}</FormLabel>
                <FormControl>
                  <Textarea
                    rows={18}
                    className='min-h-80 font-mono text-xs'
                    disabled={!enabled}
                    {...field}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'The enhancer receives the original prompt, duration, aspect ratio, resolution preset, and reference images.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
