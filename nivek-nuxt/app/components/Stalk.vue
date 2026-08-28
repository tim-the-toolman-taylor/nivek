<script setup lang="ts">
import { API_ROUTES } from '~/utils/constants'

interface StalkConfig {
  found: boolean
  target: string
  last_message: string
  set_by: string
}

const loading = ref(true)
const saving = ref(false)
const error = ref<string | null>(null)
const config = ref<StalkConfig>({ found: false, target: '', last_message: '', set_by: '' })
const targetInput = ref('')
const confirmingClear = ref(false)

async function getStalk() {
  loading.value = true
  error.value = null
  try {
    config.value = await api<StalkConfig>(API_ROUTES.Secure.GetStalk)
  } catch (err: unknown) {
    console.error('error fetching stalk target: ', err)
    error.value = 'Could not load stalk settings.'
  } finally {
    loading.value = false
  }
}

async function saveTarget() {
  const target = targetInput.value.trim()
  if (!target) return
  saving.value = true
  error.value = null
  try {
    await api(API_ROUTES.Secure.PostStalk, {
      method: 'POST',
      body: { target },
    })
    targetInput.value = ''
    confirmingClear.value = false
    await getStalk()
  } catch (err: unknown) {
    console.error('error setting stalk target: ', err)
    error.value = 'Could not save that username.'
  } finally {
    saving.value = false
  }
}

async function clearTarget() {
  saving.value = true
  error.value = null
  try {
    await api(API_ROUTES.Secure.DeleteStalk, { method: 'DELETE' })
    confirmingClear.value = false
    await getStalk()
  } catch (err: unknown) {
    console.error('error clearing stalk target: ', err)
    error.value = 'Could not clear the stalk target.'
  } finally {
    saving.value = false
  }
}

onMounted(() => {
  getStalk()
})
</script>

<template>
  <h4 class="title">Stalk</h4>
  <div>
    <p class="mb-2">
      Pick one chatter. Anyone in your chat can then run <code>!stalk</code> and the bot quotes
      that person's last message, verbatim.
    </p>
    <p class="mb-2 chat-shortcuts">
      Broadcaster/mods can also set this from chat: <code>!stalk set Stan</code> (or
      <code>!stalk Stan</code>), and <code>!stalk clear</code> to stop.
    </p>

    <form @submit.prevent="saveTarget()" class="mb-3 py-3">
      <div class="form-group">
        <label for="stalkTarget">Chatter to stalk</label>
        <input
          id="stalkTarget"
          type="text"
          class="form-control"
          v-model="targetInput"
          placeholder="username"
          required
        />
      </div>
      <button type="submit" class="btn btn-primary mt-2" :disabled="saving">
        {{ config.found ? 'Update Target' : 'Set Target' }}
      </button>
    </form>

    <p v-if="loading" class="empty">Loading…</p>
    <p v-else-if="error" class="empty error">{{ error }}</p>

    <div v-else-if="config.found" class="current">
      <div class="current-head">
        Currently stalking <code>{{ config.target }}</code>
        <span v-if="config.set_by" class="meta">set by {{ config.set_by }}</span>
      </div>
      <div v-if="config.last_message" class="last-message">
        Last message: “{{ config.last_message }}”
      </div>
      <div v-else class="meta">No message stored yet — they'll be quoted after they talk.</div>
      <div class="actions">
        <template v-if="!confirmingClear">
          <button class="btn btn-sm btn-danger" :disabled="saving" @click="confirmingClear = true">
            Clear
          </button>
        </template>
        <template v-else>
          <button class="btn btn-sm btn-success" :disabled="saving" @click="clearTarget">YES</button>
          <button class="btn btn-sm btn-secondary" @click="confirmingClear = false">NO</button>
        </template>
      </div>
    </div>
    <p v-else class="empty">Nobody is being stalked yet — set a username above.</p>
  </div>
</template>

<style scoped>
.title {
  border-bottom: 2px solid grey;
}
.chat-shortcuts {
  font-size: 0.9rem;
  opacity: 0.85;
}
.form-control {
  background-color: unset;
  border: 1px solid gray;
  border-radius: 6px;
  color: unset;
  width: 100%;
}
.form-control::placeholder {
  color: unset;
  font-style: italic;
  opacity: 0.6;
}
.btn.btn-primary {
  background-color: transparent;
  border: 1px solid gray;
  color: inherit;
}
form {
  border-top: 2px solid gray;
  border-bottom: 2px solid gray;
}
.form-group {
  margin-bottom: 0.5rem;
}
.empty {
  opacity: 0.6;
  font-style: italic;
}
.empty.error {
  opacity: 1;
  font-style: normal;
  color: #e66;
}
.current {
  display: flex;
  flex-direction: column;
  gap: 0.4rem;
}
.current-head {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 0.6rem;
}
.last-message {
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
.meta {
  font-size: 0.8rem;
  opacity: 0.6;
}
.actions {
  display: flex;
  gap: 0.35rem;
  margin-top: 0.35rem;
}
</style>
