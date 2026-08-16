<script setup lang="ts">
// Public, no-login page. The set of commands is driven by the DB (fetched from
// the public /api/commands endpoint), so enabling/disabling or adding a command
// server-side is reflected here automatically. Friendly descriptions are kept
// frontend-side, keyed by trigger, mirroring app/pages/df/help.vue — a command
// with no description entry still lists, just without the blurb.

interface Command {
  id: number
  trigger: string
  kind: string
  handler_key: string | null
  scope: string
  min_role: string
  cooldown_secs: number
  enabled: boolean
}

const DESCRIPTIONS: Record<string, string> = {
  '!dad': 'Rolls a random dad joke. Rate-limited per stream so it stays fun, not spammy.',
  '!fish': 'Cast a line and reel in a random catch — builds up your fishing score over time.',
  '!bread': 'Bump the channel\'s communal bread counter. 🍞',
  '!lurk': 'Let the streamer know you\'re sticking around in the background.',
  '!joinme': 'Have the bot join YOUR channel. In channels other than the bot\'s home chats, mention the bot: @peanutbudderbot !joinme',
  '!banish': 'Remove the bot from your channel (kept your data — just opts out). Broadcaster or moderator only.',
  '!pbcommands': 'Drops a link to this very page in chat.',
}

const commands = ref<Command[]>([])
const loading = ref(true)
const error = ref<string | null>(null)

function roleLabel(role: string): string | null {
  switch (role) {
    case 'broadcaster': return 'Broadcaster only'
    case 'mod': return 'Mod only'
    case 'vip': return 'VIP+'
    case 'sub': return 'Subscribers'
    default: return null // 'everyone' — no badge
  }
}

onMounted(async () => {
  try {
    const res = await api<{ commands: Command[] }>('/commands')
    commands.value = (res.commands ?? [])
      .filter((c) => c.enabled && c.scope === 'global')
      .sort((a, b) => a.trigger.localeCompare(b.trigger))
  } catch (e) {
    error.value = 'Could not load the command list. Please try again later.'
    console.error('failed to fetch commands', e)
  } finally {
    loading.value = false
  }
})

useHead({ title: 'Commands — peanutbudderbot' })
</script>

<template>
  <div class="commands-page">
    <section class="panel panel-head">
      <h1 class="green">Commands &amp; Actions</h1>
      <p>
        Type any of these in a chat the
        <a href="https://twitch.tv/peanutbudderbot/">peanutbudderbot</a> bot has joined.
        Commands are case-insensitive.
      </p>
    </section>

    <section class="panel">
      <h2>Chat commands</h2>

      <p v-if="loading" class="state">Loading commands…</p>
      <p v-else-if="error" class="state error">{{ error }}</p>
      <p v-else-if="commands.length === 0" class="state">No commands are currently available.</p>

      <ul v-else class="cmd-list">
        <li v-for="c in commands" :key="c.id" class="cmd">
          <div class="cmd-head">
            <code class="trigger">{{ c.trigger }}</code>
            <span v-if="roleLabel(c.min_role)" class="badge">{{ roleLabel(c.min_role) }}</span>
            <span v-if="c.cooldown_secs > 0" class="badge cooldown">{{ c.cooldown_secs }}s cooldown</span>
          </div>
          <p class="desc">{{ DESCRIPTIONS[c.trigger] || 'A bot command.' }}</p>
        </li>
      </ul>
    </section>

    <section class="panel">
      <h2>Automated actions</h2>
      <p class="section-note">
        Not chat commands — these run on their own once a streamer sets them up from the
        <a href="/api/auth/twitch/start">dashboard</a>.
      </p>
      <ul class="cmd-list">
        <li class="cmd">
          <div class="cmd-head">
            <span class="trigger action">Auto Shout</span>
            <span class="badge">Broadcaster configured</span>
          </div>
          <p class="desc">
            Automatically shouts out chatters you've added to your auto-shout list the first
            time they talk in your stream — great for regulars and fellow streamers.
          </p>
        </li>
      </ul>
    </section>

    <section class="panel see-also">
      <p>
        Running a Dwarf Fortress stream? The <code>!DF</code> commands have their own
        <NuxtLink to="/df/help">help page</NuxtLink>.
      </p>
    </section>
  </div>
</template>

<style scoped>
.commands-page {
  max-width: 820px;
  margin: 0 auto;
}
.panel {
  background: var(--color-background-soft);
  border: 1px solid var(--color-border);
  border-radius: 10px;
  padding: 1.25rem 1.5rem;
  margin-bottom: 1.5rem;
}
.panel-head {
  text-align: center;
}
.panel-head h1 {
  font-weight: 500;
  font-size: 2.4rem;
  margin-bottom: 0.5rem;
}
.panel h2 {
  margin: 0 0 1rem 0;
  color: var(--color-heading);
  font-size: 1.4rem;
}
.state {
  color: var(--color-text);
  opacity: 0.8;
}
.state.error {
  color: #e66;
}
.cmd-list {
  list-style: none;
  padding: 0;
  margin: 0;
  display: flex;
  flex-direction: column;
  gap: 1rem;
}
.cmd {
  padding: 0.9rem 1rem;
  border: 1px solid var(--color-border);
  border-radius: 8px;
  background: var(--color-background-mute);
}
.cmd-head {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 0.6rem;
  margin-bottom: 0.35rem;
}
.trigger {
  font-size: 1.05rem;
  font-weight: 600;
  color: hsla(160, 100%, 37%, 1);
  background: hsla(160, 100%, 37%, 0.1);
  padding: 0.15rem 0.5rem;
  border-radius: 6px;
}
.trigger.action {
  background: hsla(200, 90%, 50%, 0.12);
  color: hsla(200, 90%, 55%, 1);
}
.badge {
  font-size: 0.75rem;
  text-transform: uppercase;
  letter-spacing: 0.03em;
  padding: 0.15rem 0.5rem;
  border-radius: 999px;
  border: 1px solid var(--color-border);
  color: var(--color-text);
  opacity: 0.85;
}
.badge.cooldown {
  opacity: 0.7;
}
.desc {
  margin: 0;
  color: var(--color-text);
  line-height: 1.5;
}
.section-note {
  margin-top: -0.4rem;
  margin-bottom: 1rem;
  color: var(--color-text);
  opacity: 0.8;
}
.see-also {
  text-align: center;
  font-size: 0.95rem;
}
.see-also p {
  margin: 0;
  opacity: 0.85;
}
</style>
