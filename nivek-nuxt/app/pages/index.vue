<script setup lang="ts">
const auth = useAuthStore()
const { activePanel } = useDashPanels()

const displayName = computed(() =>
  auth.user?.twitch_display_name || auth.user?.twitch_login || auth.user?.username || 'Streamer',
)

function getGreeting(date: Date = new Date()): string {
  const hour = date.getHours()
  if (hour >= 12 && hour < 18) return 'Good Afternoon'
  if (hour >= 18 || hour < 5) return 'Good Evening'
  return 'Good Morning'
}
</script>

<template>
  <div v-if="!auth.initialized" class="auth-loading" aria-live="polite">
    Checking your session…
  </div>

  <div v-else-if="!auth.user" class="greetings">
    <h1 class="green">Welcome</h1>
    <p>Welcome to my Programming Playground.<br />Feel free to have a look around.</p>
  </div>

  <template v-else>
    <section class="panel panel-head">
      <h1 class="green">{{ getGreeting() }}, {{ displayName }}</h1>
    </section>

    <section class="panel">
      <p :class="{ hidden: activePanel !== null }">Select a command on the left to start.</p>
      <div :class="{ hidden: activePanel !== 'autoshout' }"><AutoShout /></div>
      <div :class="{ hidden: activePanel !== 'fishing' }"><FishScore /></div>
    </section>
  </template>
</template>

<style scoped>
.auth-loading {
  min-height: 8rem;
  display: grid;
  place-items: center;
  color: var(--color-text-soft);
}
.greetings h1 {
  font-weight: 500;
  font-size: 2.6rem;
  position: relative;
  top: -10px;
  text-align: center;
}
.greetings p {
  text-align: center;
}
@media (min-width: 1024px) {
  .greetings h1,
  .greetings p {
    text-align: left;
  }
}
.panel {
  background: var(--color-background-soft);
  border: 1px solid var(--color-border);
  border-radius: 10px;
  padding: 1.25rem;
  margin-bottom: 1.5rem;
}
.panel-head {
  text-align: center;
}
.hidden {
  display: none !important;
}
</style>
