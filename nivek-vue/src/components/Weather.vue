<script setup lang="ts">
import { createHttpClient } from '@/services/HttpClient'
import { AxiosAdapter } from '@/services/AxiosAdapter'
import {onMounted} from "vue";
import {API_ROUTES} from "@/constants";

const http = createHttpClient(AxiosAdapter)

onMounted(_ => {
  try {
    const resp = await http.get(API_ROUTES.GET_WEATHER)
  } catch (err: unknown) {
    console.error("error fetching weather info: ", err)
  }

  console.log("success fetching weather")
  console.log(resp)
})
</script>

<template>

</template>

<style scoped>

</style>