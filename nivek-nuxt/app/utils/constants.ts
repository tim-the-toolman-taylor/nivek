export const API_ROUTES = {
  TwitchStart: '/api/auth/twitch/start',
  AuthLanding: '/auth/landing',
  Secure: {
    Profile: '/profile',
    Weather: '/weather',
    GetFishScore: '/fishing',
    GetAutoShoutChatters: '/auto-shout',
    PostCreateAutoShoutChatter: '/auto-shout',
    PostUpdateAutoShoutChatter: (id: number) => `/auto-shout/${id}`,
    DeleteAutoShoutChatter: (id: number) => `/auto-shout/${id}`,
    GetDadResponses: '/dad',
    PostCreateDadResponse: '/dad',
    DeleteDadResponse: (id: number) => `/dad/${id}`,
    GetPromos: '/promo',
    PostCreatePromo: '/promo',
    PostUpdatePromo: (id: number) => `/promo/${id}`,
    DeletePromo: (id: number) => `/promo/${id}`,
    GetStalk: '/stalk',
    PostStalk: '/stalk',
    DeleteStalk: '/stalk',
    Tasks: {
      Create: (id: number) => `/user/${id}/task`,
      GetAll: (id: number) => `/user/${id}/task`,
    },
  },
} as const

export interface User {
  id: number
  username?: string
  created_at: string
  twitch_id: string | null
  twitch_login: string | null
  twitch_display_name: string | null
  bot_opt_in: boolean
  is_live: boolean
  // Whether this user may see the /overlay pairing page. Computed server-side
  // from the overlay allowlist (OVERLAY_DOWNLOAD_ALLOWLIST).
  overlay_access?: boolean
}

export interface Task {
  id: number
  uuid: string
  user_id: number
  title: string
  description: string
  priority: string
  status: string
  expires_at: string
  completed_at: string
  created_at: string
  updated_at: string
  is_important: boolean
  position: number
  estimated_duration: string
  actual_duration: string
}
