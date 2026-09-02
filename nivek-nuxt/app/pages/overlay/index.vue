<script setup lang="ts">
// Owner-only pairing page for the Ridiculous Stream overlay. Mints the device
// token the overlay sends in its first websocket frame; once paired, cheers,
// power-ups, extension Bits interactions and relayed chat commands are delivered
// to that machine.
//
// Two server-side behaviours drive the design here:
//   1. Minting REPLACES — CreateDevice revokes the prior active token and drops
//      any overlay still connected on it. So the button is destructive.
//   2. The plaintext token exists only in the create response; the DB stores
//      sha256(token). It cannot be shown again, so the copy step is mandatory.
definePageMeta({
  ssr: false,
  middleware: 'overlay-access',
})

interface Device {
  id: number
  user_id: number
  label: string
  created_at: string
  last_seen_at?: string | null
  revoked_at?: string | null
  connected: boolean
}

const devices = ref<Device[]>([])
const loadingDevices = ref(true)
const generating = ref(false)
const token = ref<string | null>(null)
const error = ref<string | null>(null)
const copied = ref(false)

const activeDevice = computed(() => devices.value.find((d) => !d.revoked_at) ?? null)

async function loadDevices(): Promise<void> {
  loadingDevices.value = true
  try {
    const res = await api<{ devices: Device[] }>('/overlay/device')
    devices.value = res.devices ?? []
  } catch (e) {
    error.value = 'Could not load the current overlay device.'
    console.error('failed to list overlay devices', e)
  } finally {
    loadingDevices.value = false
  }
}

async function generateToken(): Promise<void> {
  generating.value = true
  error.value = null
  copied.value = false
  token.value = null
  try {
    // Label is capped at 100 characters server-side; a date stamp is well under.
    const res = await api<{ device: Device; token: string }>('/overlay/device', {
      method: 'POST',
      body: { label: `Overlay ${new Date().toLocaleDateString()}` },
    })
    token.value = res.token
    await loadDevices()
  } catch (e) {
    error.value = 'Could not generate a device token. Please try again.'
    console.error('failed to create overlay device', e)
  } finally {
    generating.value = false
  }
}

async function copyToken(): Promise<void> {
  if (!token.value) return
  try {
    await navigator.clipboard.writeText(token.value)
    copied.value = true
  } catch {
    // Clipboard can be blocked by permissions; the token stays selectable.
    copied.value = false
  }
}

onMounted(loadDevices)

useHead({ title: 'Overlay — peanutbudderbot' })
</script>

<template>
  <div class="overlay-page">
    <section class="panel panel-head">
      <h1 class="green">Overlay</h1>
      <p>
        Pair the Ridiculous Stream overlay with your channel. The overlay sends its device token
        when it connects, and everything routed to your channel — bits, power-ups, extension
        purchases and overlay chat commands — is delivered to whichever machine holds it.
      </p>
    </section>

    <section class="panel">
      <h2>Download the overlay</h2>
      <p>
        Grab the Ridiculous Stream overlay build, then pair it below. Unzip it and run
        <code>RidiculousStream.exe</code> from inside the extracted folder — keep the
        <code>.dll</code> next to it.
      </p>
      <a class="btn download" href="/api/overlay/download" download>Download for Windows</a>
      <p class="hint">Windows 64-bit · ~52&nbsp;MB</p>
    </section>

    <section class="panel">
      <h2>Current device</h2>
      <p v-if="loadingDevices" class="state">Checking…</p>
      <p v-else-if="!activeDevice" class="state">
        No overlay device registered yet. Generate a token below to pair one.
      </p>
      <p v-else class="state device">
        <span class="dot" :class="{ live: activeDevice.connected }" aria-hidden="true"></span>
        <strong>{{ activeDevice.label || 'Overlay' }}</strong>
        <span class="muted">{{ activeDevice.connected ? 'connected now' : 'not connected' }}</span>
      </p>
    </section>

    <section class="panel">
      <h2>Device token</h2>
      <p class="warn">
        Generating a token <strong>replaces</strong> the current one. Any overlay still running on
        the old token is disconnected immediately and will need the new token pasted in.
      </p>

      <button class="btn" type="button" :disabled="generating" @click="generateToken">
        {{ generating ? 'Generating…' : 'Generate device token' }}
      </button>

      <p v-if="error" class="state error">{{ error }}</p>

      <div v-if="token" class="token-box">
        <p class="warn once">
          Copy this now — it is shown once and cannot be retrieved again. Only a hash is stored.
        </p>
        <code class="token">{{ token }}</code>
        <button class="btn btn-secondary" type="button" @click="copyToken">
          {{ copied ? 'Copied' : 'Copy token' }}
        </button>
        <p class="hint">
          In the overlay: floating menu → <strong>Overlay Configuration</strong> → paste into the
          device token field → <strong>Save &amp; Reconnect</strong>.
        </p>
      </div>
    </section>
  </div>
</template>

<style scoped>
.overlay-page {
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
  opacity: 1;
  margin-top: 0.75rem;
}
.device {
  display: flex;
  align-items: center;
  gap: 0.6rem;
  opacity: 1;
}
.muted {
  opacity: 0.7;
}
.dot {
  width: 0.6rem;
  height: 0.6rem;
  border-radius: 50%;
  background: var(--color-border);
  flex: 0 0 auto;
}
.dot.live {
  background: hsla(160, 100%, 37%, 1);
}
.warn {
  color: var(--color-text);
  opacity: 0.85;
  margin: 0 0 1rem 0;
  padding: 0.7rem 0.9rem;
  border-radius: 8px;
  border: 1px solid hsla(35, 90%, 55%, 0.35);
  background: hsla(35, 90%, 55%, 0.08);
}
.warn.once {
  border-color: hsla(0, 80%, 60%, 0.4);
  background: hsla(0, 80%, 60%, 0.08);
}
.btn {
  font: inherit;
  cursor: pointer;
  padding: 0.55rem 1rem;
  border-radius: 8px;
  border: 1px solid hsla(160, 100%, 37%, 0.5);
  background: hsla(160, 100%, 37%, 0.12);
  color: hsla(160, 100%, 37%, 1);
  transition: background-color 0.2s;
}
.btn:hover:not(:disabled) {
  background: hsla(160, 100%, 37%, 0.22);
}
.btn:disabled {
  cursor: default;
  opacity: 0.6;
}
.btn-secondary {
  border-color: var(--color-border);
  background: var(--color-background-mute);
  color: var(--color-text);
}
/* The download control is an anchor (native file download) styled as a button. */
.btn.download {
  display: inline-block;
  text-decoration: none;
}
.token-box {
  margin-top: 1.25rem;
  padding-top: 1.25rem;
  border-top: 1px solid var(--color-border);
}
.token {
  display: block;
  word-break: break-all;
  user-select: all;
  padding: 0.8rem 0.9rem;
  margin-bottom: 0.75rem;
  border-radius: 8px;
  border: 1px solid var(--color-border);
  background: var(--color-background-mute);
  font-size: 0.95rem;
}
.hint {
  margin: 0.75rem 0 0 0;
  opacity: 0.75;
  font-size: 0.9rem;
}
</style>
