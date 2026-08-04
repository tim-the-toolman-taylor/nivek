// Shared selection state for the dashboard command panels. The toggle buttons
// live in the sidebar (layouts/default.vue) while the panels themselves render
// in the page (pages/index.vue), so the selection is lifted into Nuxt useState
// so both sides share one source of truth.
//
// Single-selection (accordion): at most one panel is visible at a time.
// Clicking a panel's button selects it (hiding any other); clicking the
// already-active one deselects it (nothing visible).
export type DashPanel = 'autoshout' | 'fishing' | null

export const useDashPanels = () => {
  const activePanel = useState<DashPanel>('dash-active-panel', () => null)

  const togglePanel = (panel: Exclude<DashPanel, null>) => {
    activePanel.value = activePanel.value === panel ? null : panel
  }

  return { activePanel, togglePanel }
}
