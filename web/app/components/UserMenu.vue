<script setup lang="ts">
const { t } = useI18n()
const auth = useAuthStore()

async function signOut() {
  await auth.signOut()
  await navigateTo('/')
}

const items = computed(() => [
  [{ label: auth.user?.email ?? '', type: 'label' as const }],
  [
    { label: t('nav.watchlists'), icon: 'i-lucide-list', to: '/watchlists' },
    { label: t('nav.settings'), icon: 'i-lucide-settings', to: '/settings' },
  ],
  [{ label: t('nav.signOut'), icon: 'i-lucide-log-out', onSelect: signOut }],
])
</script>

<template>
  <UDropdownMenu v-if="auth.signedIn" :items="items">
    <UButton color="neutral" variant="ghost" icon="i-lucide-circle-user" :aria-label="auth.user?.email" />
  </UDropdownMenu>
  <UButton v-else to="/signin" color="neutral" variant="outline" size="sm">{{ t('nav.signIn') }}</UButton>
</template>
