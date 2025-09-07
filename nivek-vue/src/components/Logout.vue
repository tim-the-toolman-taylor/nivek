<script setup lang="ts">
import { useAuthStore } from '@/stores/auth'
import { useRouter } from 'vue-router'

const auth = useAuthStore()
const router = useRouter()

async function doLogout() {
  auth.logout()
  await router.push('/')
}
</script>

<template>
  <div class="logout-button">
    <button @click="doLogout" class="btn btn-secondary" type="button">Log out</button>
  </div>
</template>

<style scoped>
.logout-button {
  position: absolute;
  top: 2rem;
  right: 2rem;
}
</style>