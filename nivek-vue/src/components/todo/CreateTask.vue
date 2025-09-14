<script setup lang="ts">
import { ref, reactive } from 'vue'
import { createHttpClient } from '@/services/HttpClient'
import { AxiosAdapter } from '@/services/AxiosAdapter'
import { useAuthStore } from '@/stores/auth'
import { API_ROUTES } from '@/constants'

interface NewTaskRequest {
  title: string
  description: null | string
  priority: string
  expires_at: null | string
  is_important: boolean
  estimated_duration: null | string
}

const http = createHttpClient(AxiosAdapter)
const auth = useAuthStore()

const newTaskFormData = reactive(<NewTaskRequest>{
  title: "",
  description: null,
  priority: "",
  expires_at: null,
  is_important: false,
  estimated_duration: null,
})

const loading = ref(false)
const error = ref('')

let createTaskActive = ref(<boolean>false)

async function createTask() {
  loading.value = true
  error.value = ''

  try {
    const result = await http.post<int>(
        API_ROUTES.Secure.Tasks.Create(auth.user.id),
        newTaskFormData
    )
    if (result) {
      console.log('success!')
      console.log(result)
    } else {
      console.log('fail?')
      console.log(result)
    }
  } catch (err: unknown) {
    console.error('error creating new task: ', err)
  }
}

function toggleCreateTaskActive() {
  createTaskActive.value = !createTaskActive.value
}
</script>

<template>
  <div class="create-task">
    <h4 @click="toggleCreateTaskActive()">
      <span class="pe-1">Create Task</span>
      <svg v-if="createTaskActive" xmlns="http://www.w3.org/2000/svg" width="16" height="16" fill="currentColor" class="bi bi-chevron-down" viewBox="0 0 16 16">
        <path fill-rule="evenodd" d="M1.646 4.646a.5.5 0 0 1 .708 0L8 10.293l5.646-5.647a.5.5 0 0 1 .708.708l-6 6a.5.5 0 0 1-.708 0l-6-6a.5.5 0 0 1 0-.708"/>
      </svg>
      <svg v-else xmlns="http://www.w3.org/2000/svg" width="16" height="16" fill="currentColor" class="bi bi-chevron-up" viewBox="0 0 16 16">
        <path fill-rule="evenodd" d="M7.646 4.646a.5.5 0 0 1 .708 0l6 6a.5.5 0 0 1-.708.708L8 5.707l-5.646 5.647a.5.5 0 0 1-.708-.708z"/>
      </svg>
    </h4>

    <form v-if="createTaskActive" @submit.prevent="createTask()">
      <div class="form-group mb-1">
        <label for="newTaskTitle">Title</label>
        <input type="text"
               id="newTaskTitle"
               class="form-control"
               aria-describedby="newTaskTitleHelp"
               placeholder="Enter title"
               v-model="newTaskFormData.title"
               required
        >
        <small id="newTaskTitleHelp" class="form-text text-secondary">Enter task title.</small>
      </div>
      <div class="form-group mb-1">
        <label for="newTaskDescription">Description</label>
        <input type="text"
               id="newTaskDescription"
               class="form-control"
               aria-describedby="newTaskDescriptionHelp"
               placeholder="Enter description"
               v-model="newTaskFormData.description"
               required
        >
        <small id="newTaskDescriptionHelp" class="form-text text-secondary">Enter task description.</small>
      </div>
      <div class="form-group mb-1">
        <label for="newTaskPriority">Priority</label>
        <input type="text"
               id="newTaskPriority"
               class="form-control"
               aria-describedby="newTaskPriorityHelp"
               placeholder="Enter priority"
               v-model="newTaskFormData.priority"
               required
        >
        <small id="newTaskPriorityHelp" class="form-text text-secondary">Enter task priority.</small>
      </div>
      <div class="form-group mb-1">
        <label for="newTaskExpiresAt">Expiration Date / Time</label>
        <input type="text"
               id="newTaskExpiresAt"
               class="form-control"
               aria-describedby="newTaskExpiresAtHelp"
               placeholder="Enter Expiration Date"
               v-model="newTaskFormData.expires_at"
        >
        <small id="newTaskExpiresAtHelp" class="form-text text-secondary">Enter task expiration date and/or time.</small>
      </div>
      <div class="form-check mb-1">
        <input class="form-check-input"
               type="checkbox"
               id="newTaskIsImportant"
               aria-describedby="newTaskIsImportantHelp"
               v-model="newTaskFormData.is_important"
        >
        <label for="newTaskIsImportant" class="form-check-label">Is Important?</label>
      </div>
      <div class="form-group mb-1">
        <label for="newTaskEstimatedDuration">Estimated Duration</label>
        <input type="text"
               id="newTaskEstimatedDuration"
               class="form-control"
               aria-describedby="newTaskEstimatedDurationHelp"
               placeholder="Enter Estimated Duration"
               v-model="newTaskFormData.estimated_duration"
        >
        <small id="newTaskEstimatedDurationHelp" class="form-text text-secondary">Enter estimated task duration.</small>
      </div>
      <button type="submit" class="btn btn-primary">Submit</button>
    </form>
  </div>
</template>

<style scoped>

</style>