<script setup lang="ts">
const { t } = useI18n()
const auth = useAuthStore()
const route = useRoute()
const authError = useAuthError()

const email = ref('')
const password = ref('')
const pending = ref(false)
const error = ref('')

// Only same-site paths, so a crafted link cannot bounce users elsewhere.
const next = computed(() => {
  const n = String(route.query.next ?? '/')
  return n.startsWith('/') && !n.startsWith('//') ? n : '/'
})

async function submit() {
  pending.value = true
  error.value = ''
  try {
    await auth.signIn(email.value, password.value)
    await navigateTo(next.value)
  }
  catch (err) {
    error.value = authError(err)
  }
  finally {
    pending.value = false
  }
}

useHead({ title: () => `${t('auth.signInTitle')} · Shinrin` })
</script>

<template>
  <UCard class="max-w-md mx-auto">
    <template #header>
      <h1 class="text-xl font-semibold">{{ t('auth.signInTitle') }}</h1>
    </template>
    <form class="space-y-4" @submit.prevent="submit">
      <UFormField :label="t('auth.email')" name="email" required>
        <UInput v-model="email" type="email" autocomplete="email" required class="w-full" />
      </UFormField>
      <UFormField :label="t('auth.password')" name="password" required>
        <UInput v-model="password" type="password" autocomplete="current-password" required class="w-full" />
      </UFormField>
      <UAlert v-if="error" color="error" variant="subtle" :description="error" />
      <UButton type="submit" block :loading="pending">{{ t('auth.submitSignIn') }}</UButton>
    </form>
    <template #footer>
      <p class="text-sm text-muted">
        {{ t('auth.noAccount') }}
        <NuxtLink :to="{ path: '/signup', query: route.query }" class="text-primary hover:underline">{{ t('nav.signUp') }}</NuxtLink>
      </p>
    </template>
  </UCard>
</template>
