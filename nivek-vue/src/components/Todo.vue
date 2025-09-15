<script setup lang="ts">
import { ref, watch } from 'vue'
import { createHttpClient } from '@/services/HttpClient'
import { AxiosAdapter } from '@/services/AxiosAdapter'
import { useAuthStore } from '@/stores/auth'
import { API_ROUTES, Task } from '@/constants'

import CreateTask from '@/components/todo/CreateTask.vue'
import { formatDate } from '@/utils/Time'

const http = createHttpClient(AxiosAdapter)
const auth = useAuthStore()

const loading = ref(false)
const error = ref('')

let tasks = ref(<Task[]>[])

async function getTasks() {
  loading.value = true
  error.value = ''

  try {
    const result = await http.get<Task[]>(
        API_ROUTES.Secure.Tasks.GetAll(auth.user.id)
    )

    if (result) {
      tasks.value = result.data
    }
  } catch (err: unknown) {
    console.error('error fetching tasks: ', err)
  }
}

watch(() => auth.user?.id, (newUserId) => {
  if (newUserId) {
    getTasks()
  }
})
</script>

<template>
<div class="todo border m-auto">
  <div class="p-4 pt-3">
    <h3>Tasks</h3>
    <ul v-if="tasks.length" class="list-group">
      <li v-for="task in tasks" class="list-group-item">
        <p class="d-flex justify-content-between">
          <span>
            <span>{{ task.title }}</span>
            <span v-if="task.is_important" class="text-danger pl-1"> !!</span>
            <span v-if="task.priority == 'high'" class="text-danger ps-1">1</span>
            <span v-if="task.priority == 'med'" class="text-warning ps-1">2</span>
            <span v-if="task.priority == 'low'" class="ps-1">3</span>
          </span>
          <span>
            <span>Created: {{ formatDate(task.created_at) }}</span>
            <span v-if="task.status == 'in_progress'">In Progress</span>
            <span v-if="task.status == 'completed'">Completed</span>
          </span>
        </p>
        <hr>
        <p>{{ task.description }}</p>
        <p class="text-secondary" v-if="task.estimated_duration">
          {{ task.estimated_duration }}
        </p>
      </li>
    </ul>
    <p v-else>No tasks found</p>
    <hr>
    <CreateTask />
  </div>
</div>
</template>

<style scoped>
.todo {
  max-width: 400px;
}
.todo .list-group-item {
  background-color: unset;
  color: unset;
}
</style>