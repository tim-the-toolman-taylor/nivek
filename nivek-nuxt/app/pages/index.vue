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

const hideAutoShout = ref(true)
const hideFishing = ref(true)
</script>

<template>
    <div v-if="!auth.user" class="greetings">
        <h1 class="green">Welcome</h1>
        <p>Welcome to my Programming Playground. <br />Feel free to have a look around</p>
    </div>

    <template v-else>
        <div>
            <h1 class="text-center green">{{ getGreeting() }} {{ auth.user?.username }}</h1>
            <!-- <Weather /> -->
            <button class="btn btn-primary">BUTTON THAT DOES NOTHING</button>
        </div>

        <div class="container">
            <div class="row">
                <div class="col-md-2">
                    <ul class="command-config-nav">
                        <li @click="hideAutoShout = !hideAutoShout">AutoShout</li>
                        <li @click="hideFishing = !hideFishing">Fishing</li>
                    </ul>
                </div>

                <div class="col-md-8 pt-1 pb-5">
                    <p :class="{ hidden: (!hideAutoShout || !hideFishing) }">Select a command on the left to start</p>
                    <div :class="{ hidden: hideAutoShout }"><AutoShout /></div>
                    <div :class="{ hidden: hideFishing }"><FishScore /></div>
                </div>
                <div class="col-md-2">
                    <Messager />
                </div>
            </div>
        </div>
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
.hidden {
    display: none !important;
}
.container {
    border: 2px solid grey;
    border-radius: 5px;
}
.container .row > *:not(:last-child) {
    border-right: 2px solid grey;
}
.container .row {
    min-height: 500px;
}

.command-config-nav {
    list-style: none;
    margin: 0;
    padding: 0;
}
.command-config-nav > *:hover {
    cursor: pointer;
}
.command-config-nav > *:not(:last-child) {
    border-bottom: 2px solid grey;
}
</style>
