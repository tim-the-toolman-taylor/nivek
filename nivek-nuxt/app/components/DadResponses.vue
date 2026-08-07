<script setup lang="ts">
import { API_ROUTES } from '~/utils/constants'

interface DadResponse {
    id: number
    channelname: string | null
    response: string
    is_global: boolean
    use_count: number
    created_at: string
    updated_at: string
}

const dadResponses = ref<DadResponse[]>([])
const newResponse = ref('')

// Track which response is awaiting delete confirmation.
const confirmingDelete = reactive<Record<number, boolean>>({})

const globalResponses = computed(() => dadResponses.value.filter((r) => r.is_global))
const channelResponses = computed(() => dadResponses.value.filter((r) => !r.is_global))

async function getResponses() {
    try {
        dadResponses.value = await api<DadResponse[]>(API_ROUTES.Secure.GetDadResponses)
        dadResponses.value.forEach((r) => (confirmingDelete[r.id] = false))
    } catch (err: unknown) {
        console.error('error fetching dad responses: ', err)
    }
}

async function addResponse() {
    if (!newResponse.value.trim()) return
    try {
        await api(API_ROUTES.Secure.PostCreateDadResponse, {
            method: 'POST',
            body: { response: newResponse.value.trim() },
        })
        await getResponses()
        newResponse.value = ''
    } catch (err: unknown) {
        console.error('error creating dad response: ', err)
    }
}

async function removeResponse(id: number) {
    try {
        await api(API_ROUTES.Secure.DeleteDadResponse(id), {
            method: 'DELETE',
        })
        await getResponses()
    } catch (err: unknown) {
        console.error('error deleting dad response: ', err)
    }
}

onMounted(() => {
    getResponses()
})
</script>

<template>
    <h4 class="title">!dad Responses</h4>
    <div>
        <p class="mb-2">
            When someone types <code>!dad</code> in your chat, the bot replies with a random line from your
            responses below (plus the shared defaults). You can also manage these from chat with
            <code>!dad add &lt;text&gt;</code> and <code>!dad remove &lt;id&gt;</code> (broadcaster/mods).
        </p>

        <form @submit.prevent="addResponse()" class="mb-3 py-3">
            <div class="form-group">
                <label for="dadResponse">New Response</label>
                <input
                    type="text"
                    class="form-control"
                    id="dadResponse"
                    v-model="newResponse"
                    placeholder="Enter a !dad response"
                    required
                />
            </div>
            <button type="submit" class="btn btn-primary mt-2">Add Response</button>
        </form>

        <h5 class="section-label">Your Responses</h5>
        <ul class="dad-list list-group">
            <li v-if="channelResponses.length === 0" class="list-group-item empty">
                No custom responses yet — add one above.
            </li>
            <li
                v-for="resp in channelResponses"
                :key="resp.id"
                class="list-group-item d-flex justify-content-between align-items-start"
            >
                <div>
                    <span>{{ resp.response }}</span>
                    <div class="meta">#{{ resp.id }} · Used {{ resp.use_count }}×</div>
                </div>
                <div class="text-end">
                    <div v-if="!confirmingDelete[resp.id]">
                        <button @click="confirmingDelete[resp.id] = true" class="btn btn-sm btn-danger">
                            Remove
                        </button>
                    </div>
                    <div v-else>
                        <button @click="removeResponse(resp.id)" class="btn btn-sm btn-success">YES</button>
                        <button @click="confirmingDelete[resp.id] = false" class="btn btn-sm btn-secondary">
                            NO
                        </button>
                    </div>
                </div>
            </li>
        </ul>

        <h5 class="section-label mt-4">Defaults (all channels)</h5>
        <ul class="dad-list list-group">
            <li
                v-for="resp in globalResponses"
                :key="resp.id"
                class="list-group-item d-flex justify-content-between align-items-start global"
            >
                <div>
                    <span>{{ resp.response }}</span>
                    <div class="meta">Used {{ resp.use_count }}×</div>
                </div>
                <span class="badge">default</span>
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
.dad-list {
    max-height: 400px;
    overflow-y: auto;
}
.dad-list li {
    background: inherit;
    border-top: 0;
    border-left: 0;
    border-right: 0;
    border-color: var(--color-text);
    color: inherit;
}
.dad-list li.global {
    opacity: 0.7;
}
.dad-list li.empty {
    opacity: 0.6;
    font-style: italic;
}
.meta {
    font-size: 0.8rem;
    opacity: 0.6;
}
.badge {
    align-self: center;
    border: 1px solid gray;
    border-radius: 6px;
    padding: 0.15rem 0.5rem;
    font-size: 0.7rem;
    opacity: 0.7;
}
.hidden {
    display: none !important;
}
.form-control {
    background-color: unset;
    border: 0;
    color: unset;
}
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
</style>
