<script setup lang="ts">
import { reactive } from 'vue'
import { createHttpClient } from '@/services/HttpClient'
import { AxiosAdapter } from '@/services/AxiosAdapter'
import { User, API_ROUTES } from '@/constants'
import { useAuthStore } from "@/stores/auth.js";
import { useRouter } from 'vue-router'

interface LoginFormData {
  email:    string
  password: string
}

const http = createHttpClient(AxiosAdapter)
const router = useRouter()
const auth = useAuthStore()

const formData = reactive(<LoginFormData>{
  email: '',
  password: '',
})

async function doLogin() {
  try {
    const user = await http.post<User[]>(API_ROUTES.LOGIN, formData)
    auth.login(user)
    await router.push("/dashboard")
  } catch (err: unknown) {
    console.error('error logging in: ', err)
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