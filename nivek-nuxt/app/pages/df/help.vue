<script setup lang="ts">

// Single source of truth for what the bot accepts. Mirrors the verbs
// listed in internal/libraries/overseer/parse.go's ParseCommand switch —
// if a verb is added there, add it here too.
const commands = [
    {
        verb: 'make',
        syntax: '!DF make [qty] <material> <item>',
        example: '!DF make 5 iron pick',
        desc: 'Queue a manufacturing workorder. Quantity defaults to 1 if omitted.',
    },
    {
        verb: 'brew',
        syntax: '!DF brew [qty] <fruit|plant>',
        example: '!DF brew 3 plump helmet',
        desc: 'Queue a brewing workorder.',
    },
    {
        verb: 'place',
        syntax: '!DF place <item> <x> <y> <z>',
        example: '!DF place door 42 18 130',
        desc: 'Queue a building placement (uses DFHack buildingplan). Z is in-game elevation.',
    },
    {
        verb: 'mine',
        syntax: '!DF mine <x,y,z> <x,y>',
        example: '!DF mine 40,20,128 45,25',
        desc: 'Designate a rectangular region for mining. Corners are inclusive.',
    },
    {
        verb: 'channel',
        syntax: '!DF channel <x,y,z> <x,y>',
        example: '!DF channel 40,20,128 45,25',
        desc: 'Designate a region for channeling (digs down a Z-level).',
    },
    {
        verb: 'digramp',
        syntax: '!DF digramp <x,y,z> <x,y>',
        example: '!DF digramp 40,20,128 45,25',
        desc: 'Designate a region for ramp-digging.',
    },
    {
        verb: 'cuttree',
        syntax: '!DF cuttree <x,y,z> <x,y>',
        example: '!DF cuttree 40,20,128 45,25',
        desc: 'Designate a region for tree-chopping.',
    },
    {
        verb: 'stockpile',
        syntax: '!DF stockpile <category> <x,y,z> <x,y>',
        example: '!DF stockpile food 40,20,128 45,25',
        desc: 'Build a top-level-category stockpile in the given area.',
    },
    {
        verb: 'zone',
        syntax: '!DF zone <type> <x,y,z> <x,y>',
        example: '!DF zone bedroom 40,20,128 45,25',
        desc: 'Designate a civzone (office, bedroom, dormitory).',
    },
    {
        verb: 'taskat',
        syntax: '!DF taskat #<workshop_id> [qty] <material> <item>',
        example: '!DF taskat #17 5 iron pick',
        desc: 'Queue a job at a specific workshop by ID instead of the manager queue.',
    },
    {
        verb: 'camera',
        syntax: '!DF camera <x> <y> <z>',
        example: '!DF camera 42 18 130',
        desc: 'Recenter the in-game camera. Z is in-game elevation.',
    },
    {
        verb: 'appoint',
        syntax: '!DF appoint <position> <id>',
        example: '!DF appoint manager 17566',
        desc: 'Assign a citizen to a fort position. IDs are in the /df/citizens table.',
    },
    {
        verb: 'pause',
        syntax: '!DF pause',
        example: '!DF pause',
        desc: 'Pause DF.',
    },
    {
        verb: 'unpause',
        syntax: '!DF unpause',
        example: '!DF unpause',
        desc: 'Unpause DF.',
    },
    {
        verb: 'help',
        syntax: '!DF help',
        example: '!DF help',
        desc: 'Print the verb list as a chat reply (this page is the long form).',
    },
]
</script>

<template>
    <div class="df-page">
        <header>
            <div class="header-row">
                <h1>DF Twitch-plays — help</h1>
                <div class="nav-group">
                    <NuxtLink to="/df" class="nav-btn">Map</NuxtLink>
                    <NuxtLink to="/df/citizens" class="nav-btn">Citizens</NuxtLink>
                </div>
            </div>
            <p class="lede">
                Type any of the commands below in
                <a href="https://twitch.tv/timallenfanclubofficial" target="_blank" rel="noopener">
                    twitch.tv/timallenfanclubofficial
                </a>
                chat. The bot only listens for <code>!DF</code> commands there
                — other channels it has joined are ignored for DF purposes.
            </p>
        </header>

        <section class="commands-section">
            <h2>Commands</h2>
            <p class="tolerance-note">
                Commands are case-insensitive and tolerate extra whitespace and
                filler words (<code>a</code>, <code>an</code>, <code>the</code>,
                <code>some</code>, <code>me</code>, <code>us</code>,
                <code>please</code>, <code>drink</code>, <code>from</code>).
                Coordinates are <code>x,y,z</code> with z in raw embark-local
                form on dig/region commands and in-game elevation on
                <code>place</code> / <code>camera</code>.
            </p>
            <table class="cmd-table">
                <thead>
                    <tr>
                        <th>Verb</th>
                        <th>Syntax</th>
                        <th>Example</th>
                        <th>What it does</th>
                    </tr>
                </thead>
                <tbody>
                    <tr v-for="c in commands" :key="c.verb">
                        <td class="verb">{{ c.verb }}</td>
                        <td><code>{{ c.syntax }}</code></td>
                        <td><code class="example">{{ c.example }}</code></td>
                        <td>{{ c.desc }}</td>
                    </tr>
                </tbody>
            </table>
        </section>
    </div>
</template>

<style scoped>
.df-page {
    padding: 1rem 2rem;
    color: #ddd;
    background: #1a1a1a;
    min-height: 100vh;
    font-family: system-ui, sans-serif;
}

header h1 {
    margin: 0 0 0.25rem 0;
    color: #6fb;
}

.header-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    flex-wrap: wrap;
    gap: 0.5rem;
}

.nav-group {
    display: flex;
    gap: 0.5rem;
}

.nav-btn {
    display: inline-block;
    background: #222;
    color: #6fb;
    border: 1px solid #3a6;
    border-radius: 4px;
    padding: 0.35rem 0.9rem;
    font-family: monospace;
    font-size: 0.9rem;
    text-decoration: none;
    white-space: nowrap;
}
.nav-btn:hover {
    background: #2a3a30;
    border-color: #6fb;
}

.lede {
    color: #aaa;
    font-size: 0.92rem;
    margin: 0 0 1.5rem 0;
    max-width: 70ch;
}
.lede code,
.tolerance-note code {
    background: #2a3a30;
    color: #6fb;
    padding: 0.1rem 0.4rem;
    border-radius: 3px;
    font-size: 0.85em;
}
.lede a {
    color: #6fb;
}

section {
    margin-top: 2rem;
}
section h2 {
    color: #6fb;
    font-size: 1.2rem;
    text-transform: uppercase;
    letter-spacing: 1px;
    border-bottom: 1px solid #333;
    padding-bottom: 0.4rem;
    margin: 0 0 1rem 0;
}

.tolerance-note {
    color: #888;
    font-size: 0.85rem;
    margin: 0 0 1rem 0;
    max-width: 80ch;
}

.cmd-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.88rem;
}
.cmd-table th {
    text-align: left;
    color: #6fb;
    font-size: 0.78rem;
    text-transform: uppercase;
    letter-spacing: 1px;
    border-bottom: 1px solid #333;
    padding: 0.4rem 0.75rem;
}
.cmd-table td {
    padding: 0.5rem 0.75rem;
    border-bottom: 1px solid #242424;
    vertical-align: top;
}
.cmd-table .verb {
    color: #6fb;
    font-family: monospace;
    font-weight: bold;
    white-space: nowrap;
}
.cmd-table code {
    background: #2a3a30;
    color: #6fb;
    padding: 0.1rem 0.4rem;
    border-radius: 3px;
    font-family: monospace;
    font-size: 0.85em;
    white-space: nowrap;
}
.cmd-table code.example {
    background: #1f2a25;
    color: #aef;
}

.updates-body {
    color: #ccc;
    font-size: 0.95rem;
    max-width: 80ch;
    line-height: 1.5;
}
.updates-body :deep(h3) {
    color: #6fb;
    font-size: 1rem;
    margin: 1.5rem 0 0.5rem 0;
}
.updates-body :deep(a) {
    color: #6fb;
}
.updates-body :deep(code) {
    background: #2a3a30;
    color: #6fb;
    padding: 0.1rem 0.4rem;
    border-radius: 3px;
    font-size: 0.9em;
}
.updates-body :deep(em) {
    color: #888;
    font-style: italic;
}
</style>
