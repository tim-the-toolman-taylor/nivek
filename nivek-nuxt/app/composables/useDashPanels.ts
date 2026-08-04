// Shared show/hide state for the dashboard command panels. The toggle buttons
// live in the sidebar (layouts/default.vue) while the panels themselves render
// in the page (pages/index.vue), so the flags are lifted into Nuxt useState so
// both sides share one source of truth. Both default to hidden (true).
export const useDashPanels = () => {
  const hideAutoShout = useState('dash-hide-autoshout', () => true)
  const hideFishing = useState('dash-hide-fishing', () => true)
  return { hideAutoShout, hideFishing }
}
