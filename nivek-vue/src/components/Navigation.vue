<script setup lang="ts">
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
</script>

<template>
  <nav class="nav">
    <div><RouterLink to="/">Home</RouterLink></div>
    <div v-if="!auth.user"><RouterLink to="/login">Log In</RouterLink></div>
    <div v-if="!auth.user"><RouterLink to="/signup">Sign Up</RouterLink></div>
  </nav>
</template>

<style scoped>
.nav {
  display: flex;
  justify-content: center;
  list-style: none;
  margin: 0 auto 2rem;
  padding: 0;
}

.nav div:not(:last-child)::after {
  color: #f2f2f2;
  content: '|';
  padding-inline: 5px;
}

</style>