<script setup lang="ts">
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
const user = auth.user

</script>

<template>
  <div class="text-center">
    <h1 class="green">Dashboard</h1>
    <h3 class="text-secondary">Welcome {{ user.username }}</h3>
  </div>
</template>

<style scoped>

</style>