<script setup lang="ts">
// "Add to watchlist" on the asset page: a menu of the user's lists with a
// check on the ones that hold the asset, plus "new watchlist".
const props = defineProps<{ asset: AssetKey }>()
const { t } = useI18n()
const toast = useToast()
const auth = useAuthStore()
const route = useRoute()

const lists = auth.signedIn ? await useWatchlists() : undefined

function holds(w: Watchlist) {
  return w.assets.some(a => a.market === props.asset.market && a.symbol === props.asset.symbol)
}

async function toggle(w: Watchlist) {
  if (!lists) return
  try {
    if (holds(w)) {
      await lists.removeAsset(w.id, props.asset)
      toast.add({ title: t('watchlists.removed', { name: w.name }) })
    }
    else {
      await lists.addAsset(w.id, props.asset)
      toast.add({ title: t('watchlists.added', { name: w.name }), color: 'success' })
    }
  }
  catch (err) {
    toast.add({ color: 'error', title: errorMessage(err) })
  }
}

const items = computed(() => [
  (lists?.data.value.items ?? []).map(w => ({
    label: w.name,
    icon: holds(w) ? 'i-lucide-check' : 'i-lucide-plus',
    onSelect: () => toggle(w),
  })),
  [{ label: t('watchlists.new'), icon: 'i-lucide-list-plus', to: '/watchlists' }],
])
</script>

<template>
  <UDropdownMenu v-if="auth.signedIn" :items="items">
    <UButton icon="i-lucide-bookmark-plus" variant="outline">{{ t('watchlists.add') }}</UButton>
  </UDropdownMenu>
  <UButton
    v-else
    :to="{ path: '/signin', query: { next: route.fullPath } }"
    icon="i-lucide-bookmark-plus"
    variant="outline"
    color="neutral"
  >
    {{ t('watchlists.signInToSave') }}
  </UButton>
</template>
