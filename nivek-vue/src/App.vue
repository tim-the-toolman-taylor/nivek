<script setup lang="ts">
import Navigation from "@/components/Navigation.vue"
import Logout from '@/components/Logout.vue'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
</script>

<template>
  <header>
    <img alt="Vue logo" class="logo" src="./assets/pepelogo.jpg" width="125" height="125" />
    <Navigation/>
  </header>

  <div v-if="auth.user"><Logout/></div>

  <main>
    <div class="wrapper">
      <RouterView />
    </div>
  </main>
</template>

<style scoped>
header {
  line-height: 1.5;
}

.logo {
  display: block;
  margin: 0 auto 2rem;
}

@media (min-width: 1024px) {
  header {
    display: flex;
    place-items: center;
    padding-right: calc(var(--section-gap) / 2);
  }

  .logo {
    margin: 0 2rem 0 0;
  }

  header .wrapper {
    display: flex;
    place-items: flex-start;
    flex-wrap: wrap;
  }
}
</style>
