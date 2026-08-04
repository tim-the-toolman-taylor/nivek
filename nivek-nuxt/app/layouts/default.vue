<script setup lang="ts">
// Dashboard shell: a fixed left sidebar (logo + dashboard command nav) and a
// top bar (title + site nav + logout) over the scrollable content area — the
// StreamElements dashboard structure, styled with the project's existing color
// tokens (see app/assets/css/base.css). Global chrome that used to live in
// app.vue now lives here.
const auth = useAuthStore()
const route = useRoute()
const { activePanel, togglePanel } = useDashPanels()

const pageTitle = computed(() => {
  const p = route.path
  if (p.startsWith('/df')) return 'DF Dashboard'
  if (p.startsWith('/devlog')) return 'Devlog'
  if (p.startsWith('/auth')) return 'Signing in…'
  return 'Dashboard'
})

// The command toggles only make sense on the authed dashboard at /.
const showCommandNav = computed(() => !!auth.user && route.path === '/')
</script>

<template>
  <div class="dash">
    <aside class="dash-sidebar">
      <div class="dash-logo">
        <!-- munk image preserved: authed-only, as before -->
        <img v-if="auth.user" class="logo" src="/munk.gif" width="96" height="96" alt="peanutbudderbot" />
      </div>

      <!-- AutoShout / Fishing command toggles (moved here from the page) -->
      <nav v-if="showCommandNav" class="dash-cmd-nav">
        <button
          type="button"
          class="dash-cmd"
          :class="{ active: activePanel === 'autoshout' }"
          @click="togglePanel('autoshout')"
        >
          AutoShout
        </button>
        <button
          type="button"
          class="dash-cmd"
          :class="{ active: activePanel === 'fishing' }"
          @click="togglePanel('fishing')"
        >
          Fishing
        </button>
      </nav>

      <footer class="build-tag">
        peanutbudderbot · build <code>{{ BUILD_VERSION }}</code>
      </footer>
    </aside>

    <div class="dash-main">
      <header class="dash-topbar">
        <div class="dash-title">{{ pageTitle }}</div>
        <!-- header-nav preserved: same links/logic, now in the top bar -->
        <Navigation class="dash-nav" />
        <!-- logout preserved: relocated into the top bar -->
        <div v-if="auth.user" class="dash-actions"><Logout /></div>
      </header>

      <main class="dash-content">
        <slot />
      </main>
    </div>
  </div>
</template>

<style scoped>
.dash {
  display: flex;
  min-height: 100vh;
}

/* Sidebar */
.dash-sidebar {
  flex: 0 0 240px;
  width: 240px;
  display: flex;
  flex-direction: column;
  gap: 1rem;
  padding: 1.25rem 1rem;
  background: var(--color-background-mute);
  border-right: 1px solid var(--color-border);
  position: sticky;
  top: 0;
  height: 100vh;
}
.dash-logo {
  text-align: center;
}
.dash-logo .logo {
  display: block;
  margin: 0 auto;
  border-radius: 10px;
}
.dash-cmd-nav {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}
.dash-cmd {
  text-align: left;
  background: transparent;
  border: 1px solid transparent;
  border-radius: 6px;
  padding: 0.5rem 0.75rem;
  color: var(--color-text);
  font: inherit;
  cursor: pointer;
  transition:
    background-color 0.2s,
    color 0.2s;
}
.dash-cmd:hover {
  background-color: hsla(160, 100%, 37%, 0.12);
}
.dash-cmd.active {
  color: hsla(160, 100%, 37%, 1);
  background-color: hsla(160, 100%, 37%, 0.12);
}
.build-tag {
  margin-top: auto;
  font-size: 0.75rem;
  color: var(--color-text);
  opacity: 0.6;
  text-align: center;
  padding-top: 1rem;
}
.build-tag code {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
}

/* Main column */
.dash-main {
  flex: 1 1 auto;
  min-width: 0;
  display: flex;
  flex-direction: column;
}
.dash-topbar {
  display: flex;
  align-items: center;
  gap: 1.5rem;
  padding: 0.9rem 1.5rem;
  background: var(--color-background-soft);
  border-bottom: 1px solid var(--color-border);
  position: sticky;
  top: 0;
  z-index: 10;
}
.dash-title {
  font-size: 1.1rem;
  font-weight: 600;
  color: var(--color-heading);
  white-space: nowrap;
}
.dash-actions {
  margin-left: auto;
}
.dash-content {
  flex: 1 1 auto;
  padding: 1.5rem;
}

/* Narrow screens: sidebar becomes a top strip */
@media (max-width: 768px) {
  .dash {
    flex-direction: column;
  }
  .dash-sidebar {
    flex-basis: auto;
    width: 100%;
    height: auto;
    position: static;
    flex-direction: row;
    align-items: center;
    flex-wrap: wrap;
    gap: 1rem;
    padding: 0.75rem 1rem;
  }
  .dash-logo .logo {
    width: 48px;
    height: 48px;
  }
  .dash-cmd-nav {
    flex-direction: row;
  }
  .build-tag {
    margin-top: 0;
    margin-left: auto;
    padding-top: 0;
  }
  .dash-topbar {
    flex-wrap: wrap;
  }
}
</style>
