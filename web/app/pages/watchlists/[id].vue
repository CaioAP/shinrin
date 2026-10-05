<script setup lang="ts">
definePageMeta({ middleware: 'auth' })

const { t } = useI18n()
const toast = useToast()
const route = useRoute()
const prefs = usePreferencesStore()
const { profile } = storeToRefs(prefs)

const id = computed(() => Number(route.params.id))
const { data, error } = await useWatchlist(id, profile)
const { rename, removeAsset } = await useWatchlists()

if (error.value) {
  throw createError({ statusCode: error.value.statusCode ?? 500, statusMessage: error.value.statusMessage, fatal: true })
}

const editing = ref(false)
const newName = ref('')

function startRename() {
  newName.value = data.value?.name ?? ''
  editing.value = true
}

async function saveName() {
  try {
    await rename(id.value, newName.value)
    editing.value = false
  }
  catch (err) {
    toast.add({ color: 'error', title: errorMessage(err) })
  }
}

async function onRemove(asset: AssetKey) {
  try {
    await removeAsset(id.value, asset)
    toast.add({ title: t('watchlists.removed', { name: data.value?.name ?? '' }) })
  }
  catch (err) {
    toast.add({ color: 'error', title: errorMessage(err) })
  }
}

useHead({ title: () => `${data.value?.name ?? t('watchlists.title')} · Shinrin` })
</script>

<template>
  <div v-if="data" class="space-y-6">
    <UButton to="/watchlists" variant="link" icon="i-lucide-arrow-left" class="px-0">{{ t('watchlists.title') }}</UButton>
    <header class="flex flex-wrap items-center gap-2">
      <form v-if="editing" class="flex gap-2" @submit.prevent="saveName">
        <UInput v-model="newName" maxlength="60" :aria-label="t('watchlists.rename')" autofocus />
        <UButton type="submit">{{ t('watchlists.save') }}</UButton>
        <UButton color="neutral" variant="ghost" @click="editing = false">{{ t('watchlists.cancel') }}</UButton>
      </form>
      <template v-else>
        <h1 class="text-2xl font-bold">{{ data.name }}</h1>
        <UButton icon="i-lucide-pencil" color="neutral" variant="ghost" size="sm" :aria-label="t('watchlists.rename')" @click="startRename" />
      </template>
    </header>
    <AnalysisDisclaimer />
    <WatchlistTable :items="data.items" @remove="onRemove" />
  </div>
</template>
