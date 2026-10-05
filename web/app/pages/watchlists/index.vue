<script setup lang="ts">
definePageMeta({ middleware: 'auth' })

const { t } = useI18n()
const toast = useToast()
const { data, create, remove } = await useWatchlists()

const name = ref('')
const pending = ref(false)

async function submit() {
  if (!name.value.trim()) return
  pending.value = true
  try {
    const w = await create(name.value)
    name.value = ''
    await navigateTo(`/watchlists/${w.id}`)
  }
  catch (err) {
    toast.add({ color: 'error', title: errorMessage(err) })
  }
  finally {
    pending.value = false
  }
}

async function confirmRemove(w: Watchlist) {
  if (!confirm(t('watchlists.confirmDelete', { name: w.name }))) return
  try {
    await remove(w.id)
  }
  catch (err) {
    toast.add({ color: 'error', title: errorMessage(err) })
  }
}

useHead({ title: () => `${t('watchlists.title')} · Shinrin` })
</script>

<template>
  <div class="space-y-6">
    <header class="space-y-2">
      <h1 class="text-2xl font-bold">{{ t('watchlists.title') }}</h1>
      <p class="text-muted">{{ t('watchlists.intro') }}</p>
    </header>

    <form class="flex gap-2 max-w-md" @submit.prevent="submit">
      <UInput v-model="name" :placeholder="t('watchlists.namePlaceholder')" :aria-label="t('watchlists.new')" maxlength="60" class="flex-1" />
      <UButton type="submit" icon="i-lucide-plus" :loading="pending">{{ t('watchlists.create') }}</UButton>
    </form>

    <p v-if="!data.items.length" class="text-muted">{{ t('watchlists.empty') }}</p>
    <div v-else class="grid sm:grid-cols-2 lg:grid-cols-3 gap-4">
      <UCard v-for="w in data.items" :key="w.id">
        <div class="flex items-start justify-between gap-2">
          <div>
            <NuxtLink :to="`/watchlists/${w.id}`" class="font-semibold hover:underline">{{ w.name }}</NuxtLink>
            <p class="text-sm text-muted">{{ t('watchlists.assets', { n: w.assets.length }) }}</p>
          </div>
          <UButton icon="i-lucide-trash-2" color="neutral" variant="ghost" size="sm" :aria-label="t('watchlists.delete')" @click="confirmRemove(w)" />
        </div>
        <p class="text-xs text-dimmed mt-2 truncate">{{ w.assets.map(a => a.symbol).join(', ') }}</p>
      </UCard>
    </div>
  </div>
</template>
