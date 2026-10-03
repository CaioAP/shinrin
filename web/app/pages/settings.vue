<script setup lang="ts">
definePageMeta({ middleware: 'auth' })

const { t, locale } = useI18n()
const auth = useAuthStore()
const password = ref('')
const pending = ref(false)
const error = ref('')

async function deleteAccount() {
  pending.value = true
  error.value = ''
  try {
    await auth.deleteAccount(password.value)
    await navigateTo('/')
  }
  catch (err) {
    error.value = (err as { statusCode?: number }).statusCode === 401 ? t('auth.errors.unauthorized') : errorMessage(err)
  }
  finally {
    pending.value = false
  }
}

useHead({ title: () => `${t('settings.title')} · Shinrin` })
</script>

<template>
  <div v-if="auth.user" class="max-w-2xl mx-auto space-y-6">
    <h1 class="text-2xl font-bold">{{ t('settings.title') }}</h1>

    <UCard>
      <template #header>
        <h2 class="font-medium">{{ t('settings.account') }}</h2>
      </template>
      <p>{{ auth.user.email }}</p>
      <p class="text-sm text-muted">{{ t('settings.memberSince', { date: formatDate(auth.user.createdAt, locale) }) }}</p>
      <p class="text-xs text-dimmed mt-3">{{ t('settings.privacy') }}</p>
    </UCard>

    <UCard>
      <template #header>
        <h2 class="font-medium">{{ t('settings.profile') }}</h2>
      </template>
      <p v-if="auth.user.profile">
        {{ t('settings.profileSet', { profile: t(`profile.${auth.user.profile}`), date: formatDate(auth.user.profileUpdatedAt, locale) }) }}
      </p>
      <p v-else class="text-muted">{{ t('settings.profileNone') }}</p>
      <UButton to="/onboarding" variant="outline" class="mt-3">{{ auth.user.profile ? t('settings.retake') : t('settings.take') }}</UButton>
    </UCard>

    <UCard :ui="{ root: 'ring-error/50' }">
      <template #header>
        <h2 class="font-medium text-error">{{ t('settings.deleteTitle') }}</h2>
      </template>
      <p class="text-sm text-muted mb-3">{{ t('settings.deleteHint') }}</p>
      <form class="flex flex-wrap gap-2" @submit.prevent="deleteAccount">
        <UInput v-model="password" type="password" autocomplete="current-password" :placeholder="t('settings.deleteConfirm')" required class="flex-1 min-w-56" />
        <UButton type="submit" color="error" :loading="pending" :disabled="!password">{{ t('settings.deleteButton') }}</UButton>
      </form>
      <UAlert v-if="error" color="error" variant="subtle" :description="error" class="mt-3" />
    </UCard>
  </div>
</template>
