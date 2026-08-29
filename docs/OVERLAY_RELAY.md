# Overlay relay

Delivers Twitch monetisation events — cheers and channel point redemptions — to
a streamer's overlay application running on their own machine.

## Why a relay at all

The overlay can already subscribe to `channel.cheer` itself over EventSub
websockets. The relay exists for two things that in-process subscription cannot
do:

- **Distribution.** Without it, every streamer must register their own Twitch
  application and configure a client ID and secret into their overlay. With it
  they sign in once at `peanutbudderbot.com`, mint a device token, and paste it
  in.
- **Durability.** A websocket subscription only receives events while the
  overlay is running. The relay writes every event to a durable log, so a cheer
  that lands during a restart is replayed on reconnect rather than lost.

## Shape

The overlay sits behind residential NAT and cannot be reached from here, so it
dials out over `wss://` and holds the connection open. This is the same shape
every tool in this class uses (Tangia, Crowd Control, Streamer.bot) because it
is the only one that works without port forwarding or a VPN.

```
Twitch  --webhook-->  POST /api/overlay/eventsub
                            |
                            v
                      overlay_event  (durable log, per-user cursor)
                            |
                            v
                      Registry.Push  --wss-->  overlay on the streamer's PC
```

**The log is the truth; the socket is an optimisation.** Ingest commits to
Postgres before it pushes. If no overlay is attached, or the client is wedged,
the push is dropped and the event is picked up from the cursor on the next
connect. Nothing on the ingest path ever blocks on a slow client — Twitch
expects an acknowledgement within seconds.

## Routes

| Route | Auth | Purpose |
|---|---|---|
| `POST /api/overlay/eventsub` | Twitch message signature | Ingest |
| `GET /api/overlay/connect` | Device token in the handshake | Event stream |
| `GET /api/overlay/device` | Session | List devices + live connection state |
| `POST /api/overlay/device` | Session + CSRF | Mint a device token |
| `DELETE /api/overlay/device/:id` | Session + CSRF | Revoke a device token |

`/api/overlay/eventsub` must stay outside the JWT middleware and the
credentialed CORS policy: Twitch authenticates by signing the message, not by
carrying a session.

## Wire protocol

Overlay opens the socket and sends its cursor:

```json
{"type":"hello","token":"rsov_…","since":411}
```

The relay replays everything after `since`, then sends `{"type":"ready"}`, then
streams live:

```json
{"type":"event","event":{"seq":412,"kind":"cheer","id":"a1b2…","ts":"…",
                         "data":{"user_login":"x","bits":500,"message":"Cheer500 nuke"}}}
```

The overlay persists `seq` as it processes each event and sends that value as
`since` on the next connect. It should also keep the set of executed `id`s:
**bits are not refundable, so acting on one twice is the failure that cannot be
undone.** Deduplication happens at ingest too, but the overlay is the last line.

Client may send `{"type":"ping"}` (answered with `pong`) and
`{"type":"ack","seq":N}` (advisory — the cursor on reconnect is what actually
determines delivery). The relay pings every 30s to keep NAT state alive.

## Design notes

**Per-user `seq`, not the `BIGSERIAL` id.** Sequence values are allocated before
commit, so under concurrent inserts a lower id can become visible *after* a
higher one, and a cursor reader can step past an event that had not committed
yet. `seq` is assigned as `MAX(seq)+1` inside the insert, and
`UNIQUE (user_id, seq)` turns a lost race into a retry rather than a silently
dropped event.

**Opaque device tokens, not JWTs.** Device tokens are long-lived, and a JWT
cannot be revoked without a blocklist that reduces it to a database lookup
anyway. Only `sha256(token)` is stored. The short-lived browser session keeps
using the `jwt` package.

**Server timeouts are cleared per-connection.** `core-api` sets
`ReadTimeout`/`WriteTimeout` on its `http.Server`. Those deadlines are already
on the connection when the handler runs and `websocket.Accept` hijacks without
clearing them, so a socket would die ~30s in. `connect.go` clears them via
`http.NewResponseController` rather than weakening the timeouts for every other
route.

**The registry is process-local.** With one `core-api` instance the map *is* the
truth, which is how `GET /api/overlay/device` answers "is your overlay running?"
with no heartbeat table and no TTL to guess at. Running more than one instance
would need this behind Postgres `LISTEN`/`NOTIFY` or Redis.

## Deploy

1. Apply `database/overlay-relay.sql` (safe to run before or after the deploy —
   nothing reads the tables until the new `core-api` is live).
2. Set `OVERLAY_EVENTSUB_SECRET` and `OVERLAY_EVENTSUB_CALLBACK_URL`. Blank
   secret leaves the webhook answering 503 and changes nothing else.
3. Deploy `core-api`. Confirm the callback is reachable over TLS before creating
   any subscriptions — Twitch verifies it synchronously at creation time.

## Not yet built

- **Subscription creation.** Nothing creates the `channel.cheer` or
  `channel.channel_points_custom_reward_redemption.add` subscriptions yet; this
  PR only receives them. That work needs a `twitcheventsub` client configured
  with the overlay callback and secret, and it needs this endpoint deployed and
  reachable first.
- **The Godot client.** The overlay-side addon that dials in, tracks the cursor,
  and keeps the executed-transaction ledger.
- Broadcasters authorised before `bits:read` and `channel:read:redemptions` were
  added to the OAuth scope string hold tokens without them and must sign in
  again. `cmd/eventsub-healthcheck` is the natural place to detect that.
