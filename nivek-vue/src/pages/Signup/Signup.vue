<script setup lang="ts">
import { reactive } from 'vue'
import { createHttpClient } from '@/services/HttpClient'
import { AxiosAdapter } from '@/services/AxiosAdapter'
import { User, API_ROUTES } from '@/constants'

interface SignupFormData {
  username: string
  email:    string
  password: string
}

const http = createHttpClient(AxiosAdapter)

const formData = reactive(<SignupFormData>{
  username: '',
  email: '',
  password: '',
})

async function doSignup() {
  try {
    const success = await http.post<User[]>(API_ROUTES.SIGNUP, formData)
    if (success) {
      await router.push("/login")
    } else {
      console.warn("signup failed! try a different username")
    }
  } catch (err: unknown) {
    console.error('error signing up: ', err)
  }
}
</script>

<template>
  <div class="form-signup m-auto">
    <h1 class="green mb-4">Sign Up</h1>
    <form @submit.prevent="doSignup">
      <div class="form-group mb-1">
        <label for="usernameInput">Username</label>
        <input type="text"
               id="usernameInput"
               class="form-control"
               aria-describedby="usernameHelp"
               placeholder="Choose a Username"
               v-model="formData.username"
               required
        >
        <small id="usernameHelp" class="form-text text-secondary">We'll share your username with everyone</small>
      </div>
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
.form-signup {
  max-width: 420px;
}
</style>