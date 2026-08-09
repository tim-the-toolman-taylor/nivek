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
    <h1 class="green">{{ getGreeting() }}</h1>
    <p>This is the website for the <a href="https://twitch.tv/peanutbudderbot/">peanutbudderbot</a> Twitch Chat Bot.</p>
    <p>You can sign up for the bot by <a href="/api/auth/twitch/start">signing in with Twitch</a>, or by dropping a !joinme in <a href="https://twitch.tv/timallenfanclubofficial/">my chat</a> or <a href="https://twitch.tv/peanutbudderbot/">the bot's chat</a>. You can always banish the bot from your chat by having the broadcaster or a moderator run the !banish command.</p>
    <p>This is largely a hobby project, but has been quite fun to develop. Take a peek at the <NuxtLink to="/devlog">dev log</NuxtLink> for insight into my thoughts and struggles. Additionally, the project is entirely open source. Take a peek at the code <a href="https://github.com/debugging-in-prod/nivek/">here</a>. Feel free to contribute or fork as well!</p>
  </div>

  <template v-else>
    <section class="panel panel-head">
      <h1 class="green">{{ getGreeting() }}, {{ displayName }}</h1>
    </section>

    <section class="panel">
      <p :class="{ hidden: activePanel !== null }">Select a command on the left to start.</p>
      <div :class="{ hidden: activePanel !== 'autoshout' }"><AutoShout /></div>
      <div :class="{ hidden: activePanel !== 'fishing' }"><FishScore /></div>
      <div :class="{ hidden: activePanel !== 'dad' }"><DadResponses /></div>
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
