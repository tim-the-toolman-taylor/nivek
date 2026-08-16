<script setup lang="ts">
import { API_ROUTES } from '~/utils/constants'

interface Promo {
    id: number
    channelname: string
    message: string
    interval_seconds: number
    enabled: boolean
    created_at: string
    updated_at: string
}

const MIN_MINUTES = 1
const MAX_MINUTES = 1440 // 24h, matches the server-side clamp

const promos = ref<Promo[]>([])
const loading = ref(true)

// New-promo form.
const newMessage = ref('')
const newMinutes = ref(30)

// Per-row edit + delete-confirm state, keyed by promo id.
const editing = reactive<Record<number, boolean>>({})
const editMessage = reactive<Record<number, string>>({})
const editMinutes = reactive<Record<number, number>>({})
const confirmingDelete = reactive<Record<number, boolean>>({})

function minutesOf(seconds: number): number {
    return Math.max(MIN_MINUTES, Math.round(seconds / 60))
}

// Human-friendly interval label, e.g. "30m", "1h", "1h 30m".
function intervalLabel(seconds: number): string {
    const mins = Math.round(seconds / 60)
    const h = Math.floor(mins / 60)
    const m = mins % 60
    if (h > 0 && m > 0) return `${h}h ${m}m`
    if (h > 0) return `${h}h`
    return `${m}m`
}

async function getPromos() {
    loading.value = true
    try {
        promos.value = await api<Promo[]>(API_ROUTES.Secure.GetPromos)
        promos.value.forEach((p) => {
            editing[p.id] = false
            confirmingDelete[p.id] = false
        })
    } catch (err: unknown) {
        console.error('error fetching promos: ', err)
    } finally {
        loading.value = false
    }
}

async function addPromo() {
    const message = newMessage.value.trim()
    if (!message) return
    const minutes = clampMinutes(newMinutes.value)
    try {
        await api(API_ROUTES.Secure.PostCreatePromo, {
            method: 'POST',
            body: { message, interval_seconds: minutes * 60 },
        })
        newMessage.value = ''
        newMinutes.value = 30
        await getPromos()
    } catch (err: unknown) {
        console.error('error creating promo: ', err)
    }
}

function startEdit(p: Promo) {
    editMessage[p.id] = p.message
    editMinutes[p.id] = minutesOf(p.interval_seconds)
    editing[p.id] = true
}

async function saveEdit(p: Promo) {
    const message = (editMessage[p.id] ?? '').trim()
    if (!message) return
    const minutes = clampMinutes(editMinutes[p.id] ?? minutesOf(p.interval_seconds))
    try {
        await api(API_ROUTES.Secure.PostUpdatePromo(p.id), {
            method: 'POST',
            body: { message, interval_seconds: minutes * 60, enabled: p.enabled },
        })
        editing[p.id] = false
        await getPromos()
    } catch (err: unknown) {
        console.error('error updating promo: ', err)
    }
}

// Flip enabled without entering edit mode; sends the row's current message/interval.
async function toggleEnabled(p: Promo) {
    try {
        await api(API_ROUTES.Secure.PostUpdatePromo(p.id), {
            method: 'POST',
            body: {
                message: p.message,
                interval_seconds: p.interval_seconds,
                enabled: !p.enabled,
            },
        })
        await getPromos()
    } catch (err: unknown) {
        console.error('error toggling promo: ', err)
    }
}

async function removePromo(id: number) {
    try {
        await api(API_ROUTES.Secure.DeletePromo(id), { method: 'DELETE' })
        await getPromos()
    } catch (err: unknown) {
        console.error('error deleting promo: ', err)
    }
}

function clampMinutes(n: number): number {
    if (!Number.isFinite(n)) return MIN_MINUTES
    return Math.min(MAX_MINUTES, Math.max(MIN_MINUTES, Math.round(n)))
}

onMounted(() => {
    getPromos()
})
</script>

<template>
    <h4 class="title">Recurring Messages</h4>
    <div>
        <p class="mb-2">
            Set messages the bot re-posts in your chat on a timer while you're live — a Discord link, social
            handles, whatever. You can also add one from chat (broadcaster/mods):
            <code>!newpromo 30m Join my discord! https://discord.gg/...</code>
        </p>

        <form @submit.prevent="addPromo()" class="mb-3 py-3">
            <div class="form-group">
                <label for="promoMessage">Message</label>
                <textarea
                    id="promoMessage"
                    class="form-control"
                    v-model="newMessage"
                    rows="2"
                    placeholder="Enter the message to repeat"
                    required
                ></textarea>
            </div>
            <div class="form-group interval-group">
                <label for="promoMinutes">Every</label>
                <input
                    id="promoMinutes"
                    type="number"
                    class="form-control minutes"
                    v-model.number="newMinutes"
                    :min="MIN_MINUTES"
                    :max="MAX_MINUTES"
                />
                <span>minutes</span>
            </div>
            <button type="submit" class="btn btn-primary mt-2">Add Message</button>
        </form>

        <h5 class="section-label">Your Messages</h5>
        <p v-if="loading" class="empty">Loading…</p>
        <ul v-else class="promo-list list-group">
            <li v-if="promos.length === 0" class="list-group-item empty">
                No recurring messages yet — add one above.
            </li>

            <li v-for="p in promos" :key="p.id" class="list-group-item" :class="{ disabled: !p.enabled }">
                <!-- View mode -->
                <template v-if="!editing[p.id]">
                    <div class="promo-body">
                        <div class="promo-message">{{ p.message }}</div>
                        <div class="meta">
                            every {{ intervalLabel(p.interval_seconds) }}
                            <span v-if="!p.enabled" class="badge paused">paused</span>
                        </div>
                    </div>
                    <div class="promo-actions">
                        <button class="btn btn-sm btn-secondary" @click="toggleEnabled(p)">
                            {{ p.enabled ? 'Pause' : 'Resume' }}
                        </button>
                        <button class="btn btn-sm btn-primary" @click="startEdit(p)">Edit</button>
                        <template v-if="!confirmingDelete[p.id]">
                            <button class="btn btn-sm btn-danger" @click="confirmingDelete[p.id] = true">
                                Delete
                            </button>
                        </template>
                        <template v-else>
                            <button class="btn btn-sm btn-success" @click="removePromo(p.id)">YES</button>
                            <button class="btn btn-sm btn-secondary" @click="confirmingDelete[p.id] = false">
                                NO
                            </button>
                        </template>
                    </div>
                </template>

                <!-- Edit mode -->
                <template v-else>
                    <div class="promo-body">
                        <textarea class="form-control" v-model="editMessage[p.id]" rows="2"></textarea>
                        <div class="form-group interval-group mt-2">
                            <label>Every</label>
                            <input
                                type="number"
                                class="form-control minutes"
                                v-model.number="editMinutes[p.id]"
                                :min="MIN_MINUTES"
                                :max="MAX_MINUTES"
                            />
                            <span>minutes</span>
                        </div>
                    </div>
                    <div class="promo-actions">
                        <button class="btn btn-sm btn-success" @click="saveEdit(p)">Save</button>
                        <button class="btn btn-sm btn-secondary" @click="editing[p.id] = false">Cancel</button>
                    </div>
                </template>
            </li>
        </ul>
    </div>
</template>

<style scoped>
.title {
    border-bottom: 2px solid grey;
}
.section-label {
    opacity: 0.8;
    font-size: 0.95rem;
}
.promo-list {
    max-height: 460px;
    overflow-y: auto;
}
.promo-list li {
    background: inherit;
    border-top: 0;
    border-left: 0;
    border-right: 0;
    border-color: var(--color-text);
    color: inherit;
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    gap: 1rem;
    flex-wrap: wrap;
}
.promo-list li.disabled {
    opacity: 0.55;
}
.promo-list li.empty {
    opacity: 0.6;
    font-style: italic;
    display: block;
}
.promo-body {
    flex: 1 1 60%;
    min-width: 0;
}
.promo-message {
    white-space: pre-wrap;
    overflow-wrap: anywhere;
}
.promo-actions {
    display: flex;
    gap: 0.35rem;
    flex-wrap: wrap;
    align-items: flex-start;
}
.meta {
    font-size: 0.8rem;
    opacity: 0.6;
    margin-top: 0.25rem;
}
.badge.paused {
    border: 1px solid gray;
    border-radius: 6px;
    padding: 0.1rem 0.4rem;
    font-size: 0.7rem;
    margin-left: 0.4rem;
}
.interval-group {
    display: flex;
    align-items: center;
    gap: 0.5rem;
}
.minutes {
    width: 5rem;
}
.form-control {
    background-color: unset;
    border: 1px solid gray;
    border-radius: 6px;
    color: unset;
    width: 100%;
}
.interval-group .form-control {
    width: 5rem;
}
textarea.form-control::placeholder,
input.form-control::placeholder {
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
</style>
