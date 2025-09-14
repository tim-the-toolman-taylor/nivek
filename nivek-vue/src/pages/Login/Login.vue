<script setup lang="ts">
import { reactive } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { ref } from 'vue'

interface LoginCredentials {
  email:    string
  password: string
}

const router = useRouter()
const auth = new useAuthStore()

const formData = reactive(<LoginCredentials>{
  email: '',
  password: '',
})

const loading = ref(false)
const error = ref('')

async function doLogin() {
  loading.value = true
  error.value = ''

  try {
    const result = await auth.login(formData)

    if (result.success) {
      await router.push('/dashboard')
    } else {
      error.value = result.error
    }
  } catch (err: unknown) {
      console.error('error logging in: ', err)
    error.value = 'An unexpected error occurred'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="form-login m-auto">
    <h1 class="green mb-4">Log In</h1>
    <form @submit.prevent="doLogin">
      <div class="form-group mb-1">
        <label for="exampleInputEmail1">Email</label>
        <input type="email"
               id="exampleInputEmail1"
               class="form-control"
               aria-describedby="emailHelp"
               placeholder="Enter email"
               v-model="formData.email"
               required
        >
        <small id="emailHelp" class="form-text text-secondary">We'll never share your email with anyone else.</small>
      </div>
      <div class="form-group mb-4">
        <label for="exampleInputPassword1">Password</label>
        <input type="password"
               class="form-control"
               id="exampleInputPassword1"
               placeholder="Password"
               v-model="formData.password"
               required
        >
      </div>
      <button type="submit" class="btn btn-primary">Submit</button>
    </form>
  </div>
</template>

<style scoped>
.form-login {
  max-width: 420px;
}
</style>