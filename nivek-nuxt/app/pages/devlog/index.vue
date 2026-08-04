<script setup lang="ts">
import { marked } from 'marked'
import updatesMd from '~/content/bot-updates.md?raw'

const updatesHtml = computed(() => marked.parse(updatesMd) as string)

onMounted(() => {
    // console.log('hi mom')
})

</script>

<template>
    <section class="updates-section">
        <h2>Project updates</h2>
        <div class="updates-body" v-html="updatesHtml" />
    </section>
</template>

<style scoped>
.updates-section {
    max-width: 70ch;
    margin: 0 auto;
    padding: 1rem;
}
.updates-section h2 {
    color: #6fb;
}
.updates-body :deep(h3) {
    color: #6fb;
    margin-top: 2rem;
}
.updates-body :deep(a) {
    color: #6fb;
}
</style>

