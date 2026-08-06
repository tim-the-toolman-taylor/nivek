<script setup lang="ts">
const auth = useAuthStore()
const busy = ref(false)
const error = ref('')

async function doLogout() {
  if (busy.value) return
  busy.value = true
  error.value = ''
  try {
    await auth.logout()
    await navigateTo('/')
  } catch {
    error.value = 'Logout failed. Reloading will retry session detection.'
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="logout-button">
    <button :disabled="busy" @click="doLogout" class="btn btn-secondary" type="button">
      {{ busy ? 'Logging out…' : 'Log out' }}
    </button>
    <small v-if="error" class="error" role="alert">{{ error }}</small>
  </div>
</template>

<style scoped>
.logout-button {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
}
.error {
  color: #e06c75;
}
</style>
