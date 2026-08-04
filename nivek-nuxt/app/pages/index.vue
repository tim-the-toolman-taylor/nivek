<script setup lang="ts">
// Single entry point at /. Renders Welcome content for unauthed visitors
// (and on SSR — server can't read localStorage so it always assumes
// unauthed), the dashboard for signed-in users once the client-side
// auth plugin hydrates.
const auth = useAuthStore()

function getGreeting(date: Date = new Date()): string {
    const hour = date.getHours()
    if (hour >= 12 && hour < 18) {
        return 'Good Afternoon'
    } else if (hour >= 18 || hour < 5) {
        return 'Good Evening'
    } else {
        return 'Good Morning'
    }
}

// The AutoShout / Fishing toggle buttons live in the sidebar (layout); the
// active selection is shared via useState so the buttons there control the
// panels here. At most one panel is visible at a time.
const { activePanel } = useDashPanels()
</script>

<template>
    <div v-if="!auth.user" class="greetings">
        <h1 class="green">Welcome</h1>
        <p>Welcome to my Programming Playground. <br />Feel free to have a look around</p>
    </div>

    <template v-else>
        <section class="panel panel-head">
            <h1 class="green">{{ getGreeting() }} {{ auth.user?.username }}</h1>
            <!-- <Weather /> -->
            <button class="btn btn-primary">BUTTON THAT DOES NOTHING</button>
        </section>

        <section class="panel">
            <p :class="{ hidden: activePanel !== null }">Select a command on the left to start</p>
            <div :class="{ hidden: activePanel !== 'autoshout' }"><AutoShout /></div>
            <div :class="{ hidden: activePanel !== 'fishing' }"><FishScore /></div>
        </section>
    </template>
</template>

<style scoped>
/* Welcome (unauthed) */
.greetings h1 {
    font-weight: 500;
    font-size: 2.6rem;
    position: relative;
    top: -10px;
    text-align: center;
}

.greetings h3 {
    font-size: 1.2rem;
    text-align: center;
}

@media (min-width: 1024px) {
    .greetings h1,
    .greetings h3 {
        text-align: left;
    }
}

/* Dashboard (authed) */
.panel {
    background: var(--color-background-soft);
    border: 1px solid var(--color-border);
    border-radius: 10px;
    padding: 1.25rem;
    margin-bottom: 1.5rem;
}
.panel-head {
    text-align: center;
}
.panel-head .btn {
    margin-top: 0.5rem;
}
.hidden {
    display: none !important;
}
</style>
