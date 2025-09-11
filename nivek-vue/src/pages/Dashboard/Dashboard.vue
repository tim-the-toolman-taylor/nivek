<script setup lang="ts">
import { useAuthStore } from '@/stores/auth'
import Weather from "@/components/Weather.vue";
import { getGreeting } from "@/utils/toolbox";

const auth = useAuthStore()
const user = auth.user

console.log('dashboard')
console.log('-')
console.log('user: ')
console.log(user)
console.warn()

</script>

<template>
  <div class="text-center">
    <h1 class="green">{{ getGreeting() }} {{ user.username }}</h1>
  </div>

  <Weather />
</template>

<style scoped>

</style>