<script setup lang="ts">
const { t, locale, locales, setLocale } = useI18n()
const { data: meta } = await useMeta()
const colorMode = useColorMode()

const auth = useAuthStore()
const nav = computed(() => [
  { label: t('nav.home'), to: '/' },
  { label: t('nav.explore'), to: '/explore' },
  ...(auth.signedIn ? [{ label: t('nav.watchlists'), to: '/watchlists' }] : []),
])

const localeItems = computed(() => locales.value.map(l => ({
  label: l.name ?? l.code,
  onSelect: () => setLocale(l.code),
})))

function toggleTheme() {
  colorMode.preference = colorMode.value === 'dark' ? 'light' : 'dark'
}
</script>

<template>
  <div class="min-h-screen flex flex-col">
    <header class="border-b border-default">
      <UContainer class="flex items-center gap-4 h-14">
        <NuxtLink to="/" class="font-semibold text-lg">Shinrin</NuxtLink>
        <small v-if="meta" class="text-muted">v{{ meta.version }}</small>
        <UNavigationMenu :items="nav" class="hidden sm:flex" />
        <div class="ms-auto flex items-center gap-1">
          <ProfileSelect />
          <UDropdownMenu :items="localeItems">
            <UButton color="neutral" variant="ghost" icon="i-lucide-languages" :aria-label="t('nav.language')">
              <span class="hidden md:inline">{{ locale }}</span>
            </UButton>
          </UDropdownMenu>
          <ClientOnly>
            <UButton
              color="neutral"
              variant="ghost"
              :icon="colorMode.value === 'dark' ? 'i-lucide-sun' : 'i-lucide-moon'"
              :aria-label="t('nav.theme')"
              @click="toggleTheme"
            />
          </ClientOnly>
          <UserMenu />
        </div>
      </UContainer>
      <UContainer class="sm:hidden pb-2">
        <UNavigationMenu :items="nav" />
      </UContainer>
    </header>
    <UContainer as="main" class="flex-1 w-full py-8">
      <slot />
    </UContainer>
    <DisclaimerFooter />
  </div>
</template>
