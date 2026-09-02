// Whether the signed-in user may see the /overlay pairing page. Driven by the
// backend (`overlay_access` on the profile, computed from the overlay allowlist)
// so the allowlist lives in one place instead of being duplicated in the client.
// Kept in one composable so the nav link, the route guard and the page can never
// disagree about who is allowed in.
export function useHasOverlayAccess() {
  const auth = useAuthStore()
  return computed(() => auth.user?.overlay_access === true)
}
