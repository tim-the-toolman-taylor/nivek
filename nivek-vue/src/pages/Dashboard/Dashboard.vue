<script setup lang="ts">
import { useAuthStore } from '@/stores/auth'
import Weather from "@/components/Weather.vue";
import { getGreeting } from "@/utils/toolbox";

const auth = useAuthStore()

</script>

<template>
  <div class="text-center">
    <h1 class="green">{{ getGreeting() }} {{ auth.user.username }}</h1>
  </div>

  <Weather />
</template>

<style scoped>

</style>