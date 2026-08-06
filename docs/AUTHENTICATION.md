# Authentication architecture

## Request flow

1. The browser opens `GET /api/auth/twitch/start`.
2. Core API creates a 256-bit random OAuth `state` value and stores it in a short-lived, HttpOnly, path-scoped cookie.
3. Twitch redirects to the exact registered callback: `/api/auth/twitch/callback`.
4. Core API compares `state` in constant time, burns the state cookie, exchanges the one-time authorization code, and resolves the Twitch user through Helix.
5. The local user is found or created by immutable Twitch user ID.
6. Core API creates a short-lived signed local session and sends it only in the `nivek_session` HttpOnly cookie. The JWT is never placed in a URL, localStorage, or JavaScript state.
7. A separate readable `nivek_csrf` cookie supplies the double-submit token required in `X-CSRF-Token` for unsafe cookie-authenticated API methods.
8. The callback redirects to `/auth/landing` without credentials in the URL. The page confirms the session with `GET /api/profile`.

## Cookie model

| Cookie | HttpOnly | Path | Purpose |
|---|---:|---|---|
| `twitch_oauth_state` | yes | `/api/auth/twitch/callback` | One-time OAuth CSRF correlation |
| `nivek_session` | yes | `/api` | Signed local session |
| `nivek_csrf` | no | `/` | Double-submit CSRF value |

Production must use HTTPS and `SESSION_COOKIE_SECURE=true`. Plain HTTP is accepted only for localhost or loopback development.

## Twitch application settings

The registered redirect URI must exactly match:

```text
https://peanutbudderbot.com/api/auth/twitch/callback
```

The `/api` segment is required because Traefik routes the Core API under that prefix.

## Session properties

Local JWTs use only HS256 and include a fixed issuer, audience, subject, unique token ID, issued-at time, not-before time, and expiration. The default lifetime is eight hours and may be shortened with `SESSION_TTL_MINUTES`.

Changing `JWT_SECRET` invalidates all active sessions. Use at least 32 random bytes and keep the value outside source control.

## Local development

Use matching hostnames for frontend and callback, even when ports differ:

```dotenv
TWITCH_REDIRECT_URI=http://localhost:8080/api/auth/twitch/callback
FRONTEND_BASE_URL=http://localhost:3000
SESSION_COOKIE_SECURE=false
```

Register the exact local callback in the Twitch developer console. Do not use `SESSION_COOKIE_SECURE=false` on a non-loopback host.
