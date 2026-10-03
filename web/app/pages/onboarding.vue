<script setup lang="ts">
definePageMeta({ middleware: 'auth' })

const { t } = useI18n()
const auth = useAuthStore()
const { data: questionnaire } = await useQuestionnaire()

// Start from previous answers when retaking.
const answers = ref<Record<string, string>>({ ...auth.user?.profileAnswers })
const pending = ref(false)
const error = ref('')
const saved = ref(false)

const complete = computed(() => (questionnaire.value?.items ?? []).every(q => answers.value[q.id]))

async function submit() {
  if (!complete.value) {
    error.value = t('onboarding.unanswered')
    return
  }
  pending.value = true
  error.value = ''
  try {
    await auth.saveRiskProfile(answers.value)
    saved.value = true
  }
  catch (err) {
    error.value = errorMessage(err)
  }
  finally {
    pending.value = false
  }
}

useHead({ title: () => `${t('onboarding.title')} · Shinrin` })
</script>

<template>
  <div class="max-w-2xl mx-auto space-y-6">
    <header class="space-y-2">
      <h1 class="text-2xl font-bold">{{ t('onboarding.title') }}</h1>
      <p class="text-muted">{{ t('onboarding.intro') }}</p>
    </header>

    <UCard v-if="saved && auth.user?.profile">
      <p class="text-lg font-semibold">{{ t('onboarding.result', { profile: t(`profile.${auth.user.profile}`) }) }}</p>
      <p class="text-sm text-muted mt-1">{{ t('onboarding.resultHint') }}</p>
      <UButton to="/" class="mt-4" trailing-icon="i-lucide-arrow-right">{{ t('onboarding.continue') }}</UButton>
    </UCard>

    <form v-else class="space-y-4" @submit.prevent="submit">
      <UCard v-for="(q, i) in questionnaire?.items ?? []" :key="q.id">
        <fieldset>
          <legend class="font-medium mb-3">{{ i + 1 }}. {{ t(`questions.${q.id}.q`) }}</legend>
          <URadioGroup
            v-model="answers[q.id]"
            :items="q.answers.map(a => ({ label: t(`questions.${q.id}.${a}`), value: a }))"
          />
        </fieldset>
      </UCard>
      <UAlert v-if="error" color="error" variant="subtle" :description="error" />
      <div class="flex gap-3">
        <UButton type="submit" :loading="pending">{{ t('onboarding.submit') }}</UButton>
        <UButton to="/" variant="ghost" color="neutral">{{ t('onboarding.skip') }}</UButton>
      </div>
    </form>
  </div>
</template>
