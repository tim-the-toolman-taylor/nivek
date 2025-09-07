<script setup lang="ts">
import { createHttpClient } from '@/services/HttpClient'
import { AxiosAdapter } from '@/services/AxiosAdapter'
import { onMounted } from 'vue'
import { API_ROUTES } from '@/constants'
import { ref } from 'vue'

const http = createHttpClient(AxiosAdapter)

interface WeatherReport {
  city?: string
  temp?: string
}

let weatherReport = ref<WeatherReport>({})

async function getWeather() {
  try {
    const ip = await http.get<string>('https://api.ipify.org')
    if (!ip) {
      console.error('error fetching public IP')
      return;
    }

    weatherReport.value = await http.post<object>(API_ROUTES.GET_WEATHER, {
      ip,
    })
  } catch (err: unknown) {
    console.error("error fetching weather info: ", err)
  }
}

onMounted(() => {
  getWeather()
})
</script>

<template>
  <div class="text-center">
    <p>Weather Report for {{ weatherReport.city }}: <span class="green">{{ weatherReport.temp }}</span></p>
  </div>
</template>

<style scoped>

</style>