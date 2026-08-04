<script setup lang="ts">
const auth = useAuthStore()
const route = useRoute()
</script>

<template>
  <nav class="nav">
    <NuxtLink v-if="route.path !== '/'" class="nav-link" to="/">Home</NuxtLink>
    <NuxtLink v-if="route.path !== '/df'" class="nav-link" to="/df">DF Dashboard</NuxtLink>
    <!--
      Plain <a>, not <NuxtLink>: /api/auth/twitch/start is a backend route
      that issues a 302 to Twitch. NuxtLink would intercept and try to match
      against app routes.
    -->
    <a v-if="!auth.user" class="nav-link" href="/api/auth/twitch/start">Sign in with Twitch</a>
    <a class="nav-link" href="/devlog">Devlog</a>
  </nav>
</template>

<style scoped>
.nav {
  display: flex;
  align-items: center;
  gap: 0.25rem;
  flex-wrap: wrap;
}
.nav-link {
  padding: 0.4rem 0.7rem;
  border-radius: 6px;
  color: var(--color-text);
  text-decoration: none;
  white-space: nowrap;
  transition:
    background-color 0.2s,
    color 0.2s;
}
.nav-link:hover {
  background-color: hsla(160, 100%, 37%, 0.15);
  color: hsla(160, 100%, 37%, 1);
}
.router-link-active.nav-link {
  color: hsla(160, 100%, 37%, 1);
  background-color: hsla(160, 100%, 37%, 0.1);
}
</style>
