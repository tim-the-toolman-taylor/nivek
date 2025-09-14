<script setup lang="ts">
import { ref, reactive } from 'vue'
import CreateTask from "@/components/todo/CreateTask.vue";

let tasks = ref({})
</script>

<template>
<div class="todo border m-auto">
  <div class="p-4 pt-3">
    <h3>Tasks</h3>
    <ul v-if="tasks.length">
      <li v-for="task in tasks">{{ task.name }} - {{ task.description }}</li>
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
</style>