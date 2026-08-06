<script setup lang="ts">
definePageMeta({
  ssr: false,
  hideForAuth: true,
})

const auth = useAuthStore()
const route = useRoute()
const error = ref('')

const readableErrors: Record<string, string> = {
  access_denied: 'Twitch sign-in was cancelled.',
  missing_code_or_state: 'The Twitch callback was incomplete. Please start again.',
  missing_state_cookie: 'The sign-in state cookie was unavailable. Check cookie settings and retry.',
  state_mismatch: 'The sign-in request expired or failed its security check.',
  token_exchange_failed: 'Twitch did not accept the authorization code.',
  profile_fetch_failed: 'Twitch accepted sign-in, but the profile could not be loaded.',
  user_upsert_failed: 'The Twitch account could not be linked to a local account.',
  session_failed: 'A secure local session could not be created.',
  provider_error: 'Twitch returned a sign-in error.',
}

onMounted(async () => {
  const backendError = typeof route.query.error === 'string' ? route.query.error : ''
  if (backendError) {
    error.value = readableErrors[backendError] || 'Sign-in failed. Please retry.'
    return
  }

  try {
    await auth.fetchUserProfile()
    await navigateTo('/', { replace: true })
  } catch {
    error.value = 'The session cookie was not accepted. Verify the callback URL, HTTPS, and cookie configuration.'
  }
})
</script>

<template>
  <main class="auth-landing" aria-live="polite">
    <p v-if="!error">Finishing secure sign-in…</p>
    <div v-else>
      <h2 class="red">Sign-in failed</h2>
      <p>{{ error }}</p>
      <p><a href="/api/auth/twitch/start">Try again</a></p>
    </div>
  </main>
</template>

<style scoped>
.auth-landing {
  max-width: 560px;
  margin: 4rem auto;
  padding: 1rem;
  text-align: center;
}
.red {
  color: #e06c75;
}
</style>
