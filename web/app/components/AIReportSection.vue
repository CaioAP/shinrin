<script setup lang="ts">
// AI reports on the asset page. Signed-out users and users without a saved
// key get a pointer to what they need; the rest can write a report with
// their own key and read their earlier ones.
const props = defineProps<{ asset: AssetKey, currency: string }>()
const { t, locale } = useI18n()
const auth = useAuthStore()
const route = useRoute()
const prefs = usePreferencesStore()

const settings = auth.signedIn ? await useLLMSettings() : undefined
const reports = auth.signedIn && settings?.data.value?.configured
  ? await useAssetReports(() => props.asset.market, () => props.asset.symbol)
  : undefined

const selected = ref(0)
const writing = ref(false)
const error = ref('')
const current = computed(() => reports?.data.value.items[selected.value])
const usage = computed(() => settings?.data.value?.usage)
const atCap = computed(() => !!usage.value && usage.value.used >= usage.value.cap)
const history = computed(() => (reports?.data.value.items ?? []).map((r, i) => ({
  label: t('ai.historyItem', { date: formatDate(r.createdAt, locale.value), profile: t(`profile.${r.profile}`) }),
  value: i,
})))

async function write() {
  if (!reports) return
  writing.value = true
  error.value = ''
  try {
    await reports.generate(prefs.profile, locale.value)
    selected.value = 0
  }
  catch (err) {
    error.value = errorMessage(err)
  }
  finally {
    writing.value = false
  }
}
</script>

<template>
  <UCard>
    <template #header>
      <div class="flex flex-wrap items-center justify-between gap-2">
        <div>
          <h2 class="font-medium flex items-center gap-1.5"><UIcon name="i-lucide-sparkles" />{{ t('ai.title') }}</h2>
          <p class="text-xs text-muted">{{ t('ai.hint') }}</p>
        </div>
        <div v-if="reports" class="flex flex-wrap items-center gap-2">
          <span v-if="usage" class="text-xs text-muted">{{ t('ai.usage', { used: usage.used, cap: usage.cap }) }}</span>
          <UButton icon="i-lucide-pen-line" :loading="writing" :disabled="writing || atCap" @click="write">
            {{ t('ai.write', { profile: t(`profile.${prefs.profile}`) }) }}
          </UButton>
        </div>
      </div>
    </template>

    <div v-if="!auth.signedIn" class="flex flex-wrap items-center justify-between gap-3">
      <p class="text-sm text-muted">{{ t('ai.signInHint') }}</p>
      <UButton :to="{ path: '/signin', query: { next: route.fullPath } }" variant="outline">{{ t('nav.signIn') }}</UButton>
    </div>
    <UAlert v-else-if="settings?.data.value && !settings.data.value.available" color="neutral" variant="subtle" icon="i-lucide-lock" :description="t('ai.unavailable')" />
    <div v-else-if="!reports" class="flex flex-wrap items-center justify-between gap-3">
      <p class="text-sm text-muted">{{ t('ai.noKey') }}</p>
      <UButton to="/settings" variant="outline" icon="i-lucide-key-round">{{ t('ai.addKey') }}</UButton>
    </div>

    <div v-else class="space-y-4">
      <UAlert v-if="writing" color="primary" variant="subtle" icon="i-lucide-loader-circle" :ui="{ icon: 'animate-spin' }" :description="t('ai.writing')" />
      <UAlert v-if="atCap && !writing" color="warning" variant="subtle" :description="t('ai.atCap', { cap: usage?.cap })" />
      <UAlert v-if="error" color="error" variant="subtle" :description="error" />

      <USelect v-if="history.length > 1" v-model="selected" :items="history" size="sm" class="w-full sm:w-72" :aria-label="t('ai.history')" />
      <AIReportView v-if="current" :report="current" :currency="currency" />
      <p v-else-if="!writing" class="text-sm text-muted">{{ t('ai.none') }}</p>
    </div>
  </UCard>
</template>
