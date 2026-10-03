<script setup lang="ts">
const { t } = useI18n()
const auth = useAuthStore()
const authError = useAuthError()

const email = ref('')
const password = ref('')
const acknowledged = ref(false)
const pending = ref(false)
const error = ref('')

async function submit() {
  if (!acknowledged.value) return
  pending.value = true
  error.value = ''
  try {
    await auth.signUp(email.value, password.value)
    await navigateTo('/onboarding')
  }
  catch (err) {
    error.value = authError(err)
  }
  finally {
    pending.value = false
  }
}

useHead({ title: () => `${t('auth.signUpTitle')} · Shinrin` })
</script>

<template>
  <UCard class="max-w-md mx-auto">
    <template #header>
      <h1 class="text-xl font-semibold">{{ t('auth.signUpTitle') }}</h1>
      <p class="text-sm text-muted mt-1">{{ t('auth.signUpIntro') }}</p>
    </template>
    <form class="space-y-4" @submit.prevent="submit">
      <UFormField :label="t('auth.email')" name="email" required>
        <UInput v-model="email" type="email" autocomplete="email" required class="w-full" />
      </UFormField>
      <UFormField :label="t('auth.password')" name="password" :hint="t('auth.passwordHint')" required>
        <UInput v-model="password" type="password" autocomplete="new-password" minlength="10" maxlength="128" required class="w-full" />
      </UFormField>
      <!-- Design section 11: the disclaimer is a required acknowledgement at sign-up. -->
      <UCheckbox v-model="acknowledged" :label="t('auth.acknowledge')" required />
      <UAlert v-if="error" color="error" variant="subtle" :description="error" />
      <UButton type="submit" block :loading="pending" :disabled="!acknowledged">{{ t('auth.submitSignUp') }}</UButton>
    </form>
    <template #footer>
      <p class="text-sm text-muted">
        {{ t('auth.haveAccount') }}
        <NuxtLink to="/signin" class="text-primary hover:underline">{{ t('nav.signIn') }}</NuxtLink>
      </p>
    </template>
  </UCard>
</template>
