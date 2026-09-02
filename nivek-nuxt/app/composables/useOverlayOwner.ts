// The overlay pairing page is restricted while the overlay is pre-release.
// Kept in one place so the nav link, the route guard and the page itself can
// never disagree about who is allowed to see it.
export const OVERLAY_OWNER_LOGIN = 'timallenfanclubofficial'

export function isOverlayOwnerLogin(login: string | null | undefined): boolean {
  return (login ?? '').toLowerCase() === OVERLAY_OWNER_LOGIN
}

export function useIsOverlayOwner() {
  const auth = useAuthStore()
  return computed(() => isOverlayOwnerLogin(auth.user?.twitch_login))
}
