<script setup lang="ts">
// defineProps({
//   currentPage: {
//     type: Object,
//     required: true,
//   },
// })
</script>

<template>
  <nav class="nav">
    <div><RouterLink to="/">Home</RouterLink></div>
    <div><RouterLink to="/login">Log In</RouterLink></div>
    <div><RouterLink to="/signup">Sign Up</RouterLink></div>
  </nav>
</template>

<style scoped>
.nav {
  display: flex;
  justify-content: center;
  list-style: none;
  margin: 0 auto 2rem;
  padding: 0;
}

nav div::after {
  color: #f2f2f2;
  content: '|';
  padding-inline: 5px;
}

nav div:last-child::after {
  content: '';
}

</style>