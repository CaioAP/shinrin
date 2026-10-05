<script setup lang="ts">
// Settings card for the user's own LLM account. The key field is write-only:
// the saved key is shown only by its last four characters, and leaving the
// field empty keeps it.
const { t, locale } = useI18n()
const toast = useToast()
const { data: settings, error: loadError, save, test, remove } = await useLLMSettings()

const providers = computed(() => [
  { label: 'Anthropic (Claude)', value: 'anthropic' },
  { label: t('ai.settings.openaiCompatible'), value: 'openai' },
])

const form = reactive<LLMSettingsInput>({ provider: 'anthropic', model: '', baseUrl: '', apiKey: '', monthlyCap: 30 })
function reset() {
  const s = settings.value
  Object.assign(form, {
    provider: s?.provider ?? 'anthropic',
    model: s?.model ?? '',
    baseUrl: s?.baseUrl ?? '',
    apiKey: '',
    monthlyCap: s?.monthlyCap ?? s?.usage.cap ?? 30,
  })
}
watch(settings, reset, { immediate: true })

const busy = ref<'' | 'save' | 'test' | 'remove'>('')
const error = ref('')

async function run(kind: 'save' | 'test' | 'remove', fn: () => Promise<void>, done: string) {
  busy.value = kind
  error.value = ''
  try {
    await fn()
    toast.add({ title: done, color: 'success' })
  }
  catch (err) {
    error.value = errorMessage(err)
  }
  finally {
    busy.value = ''
  }
}

const onSave = () => run('save', () => save({ ...form, monthlyCap: Number(form.monthlyCap) }), t('ai.settings.saved'))
const onTest = () => run('test', test, t('ai.settings.works'))
const onRemove = () => run('remove', remove, t('ai.settings.removed'))
</script>

<template>
  <UCard>
    <template #header>
      <h2 class="font-medium">{{ t('ai.settings.title') }}</h2>
      <p class="text-xs text-muted">{{ t('ai.settings.hint') }}</p>
    </template>

    <UAlert v-if="loadError" color="error" variant="subtle" :description="t('common.loadError', { message: loadError.statusMessage ?? loadError.message })" />
    <UAlert v-else-if="settings && !settings.available" color="neutral" variant="subtle" icon="i-lucide-lock" :description="t('ai.unavailable')" />

    <form v-else-if="settings" class="space-y-4" @submit.prevent="onSave">
      <p v-if="settings.configured" class="text-sm">
        {{ t('ai.settings.current', { key: settings.keyHint, date: formatDate(settings.updatedAt, locale) }) }}
      </p>

      <UFormField :label="t('ai.settings.provider')" name="provider">
        <USelect v-model="form.provider" :items="providers" class="w-full" />
      </UFormField>

      <UFormField
        :label="t('ai.settings.apiKey')"
        name="apiKey"
        :help="settings.configured ? t('ai.settings.apiKeyKeep') : t('ai.settings.apiKeyHelp')"
        :required="!settings.configured"
      >
        <UInput
          v-model="form.apiKey"
          type="password"
          autocomplete="off"
          spellcheck="false"
          :placeholder="settings.configured ? settings.keyHint : 'sk-…'"
          :required="!settings.configured"
          class="w-full"
        />
      </UFormField>

      <div class="grid sm:grid-cols-2 gap-4">
        <UFormField
          :label="t('ai.settings.model')"
          name="model"
          :help="form.provider === 'openai' ? t('ai.settings.modelRequired') : t('ai.settings.modelDefault')"
          :required="form.provider === 'openai'"
        >
          <UInput v-model="form.model" :placeholder="form.provider === 'anthropic' ? 'claude-opus-5-5' : 'gpt-…'" :required="form.provider === 'openai'" class="w-full" />
        </UFormField>
        <UFormField :label="t('ai.settings.cap')" name="monthlyCap" :help="t('ai.settings.capHelp')">
          <UInput v-model="form.monthlyCap" type="number" :min="1" :max="500" class="w-full" />
        </UFormField>
      </div>

      <UFormField v-if="form.provider === 'openai'" :label="t('ai.settings.baseUrl')" name="baseUrl" :help="t('ai.settings.baseUrlHelp')">
        <UInput v-model="form.baseUrl" type="url" placeholder="https://openrouter.ai/api/v1" class="w-full" />
      </UFormField>

      <p class="text-sm text-muted">{{ t('ai.usage', { used: settings.usage.used, cap: settings.usage.cap }) }}</p>
      <UAlert v-if="error" color="error" variant="subtle" :description="error" />

      <div class="flex flex-wrap gap-2">
        <UButton type="submit" :loading="busy === 'save'" :disabled="!!busy">{{ t('ai.settings.save') }}</UButton>
        <UButton v-if="settings.configured" variant="outline" :loading="busy === 'test'" :disabled="!!busy" @click="onTest">{{ t('ai.settings.test') }}</UButton>
        <UButton v-if="settings.configured" variant="ghost" color="error" :loading="busy === 'remove'" :disabled="!!busy" @click="onRemove">{{ t('ai.settings.remove') }}</UButton>
      </div>
      <p class="text-xs text-dimmed">{{ t('ai.settings.security') }}</p>
    </form>
  </UCard>
</template>
