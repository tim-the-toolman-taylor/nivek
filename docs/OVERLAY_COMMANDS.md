# Overlay commands

Chat commands whose effect happens on the streamer's PC, not in chat. `!b`
spawns beans in the overlay's physics scene; `!nuke` clears it. The bot is not
the executor — it is a courier, and the channel has to have provisioned an
overlay before the command means anything.

This document covers how those differ from ordinary builtins and what has to
change to carry them. It assumes [OVERLAY_RELAY.md](OVERLAY_RELAY.md) for the
transport.

## Why they are not just more builtins

`!bread` and `!fish` are self-contained: the handler calls core-api and answers
in chat. Every channel the bot sits in can run them, and the answer is the same
everywhere. They are cheap and unconditional.

`!b` is neither. It needs a paired overlay device on the far end, and in a
channel without one there is nothing to send it to.

The current dispatch cannot express that. `b.commands` is a single map
consulted for every channel (`internal/libraries/twitchbot/bot_read_message.go`,
the word loop around line 127):

```go
for msgword := range strings.SplitSeq(msg, " ") {
    if handler, ok := b.commands[msgword]; ok {
        handler(b, &messageEvent)
        continue
    }
    if cmd, ok := b.customCommandFor(channel, msgword); ok { ... }
}
```

Per-channel availability exists only for **custom** commands, via
`customCommandFor`. A global builtin has no notion of "this channel, but not
that one". Registering `!b` globally as things stand fires it in every channel
the bot is in.

## The axis is capability, not scope

The distinction is not `scope` (who defined the command) and not `kind` (how the
reply is produced). It is **what the channel must have provisioned**.

Add one nullable column to `nivek.command`:

```sql
ALTER TABLE nivek.command
    ADD COLUMN IF NOT EXISTS requires TEXT
        CHECK (requires IS NULL OR requires IN ('overlay'));
```

`NULL` means unconditional — every existing row keeps its current behaviour with
no backfill. `'overlay'` means the channel needs a live-capable overlay pairing.

`!b` is then `scope='global'`, `kind='builtin'`, `requires='overlay'`. Shared
definition, conditional availability.

The column and the command rows are in
`database/prod-apply-overlay-commands.sql`. The `ALTER` is safe to run now; the
`INSERT` is not, for the reasons in that file's header.

### Why not channel-scoped custom rows

Because that is the thing redistribution cannot afford. Seeding `!b`, `!nuke`,
`!laser` as `scope='channel'` rows means re-seeding them for every streamer who
adopts the overlay, forever, by hand. With `requires`, a streamer installs the
overlay, pairs a device, and inherits the whole overlay command set with no DB
write specific to them.

### Why not reuse `enabled` or `channel_command_settings`

`nivek.channel_command_settings` already carries per-channel enable/disable and
role overrides. That is *preference* — "I have an overlay and I still don't want
`!nuke`". `requires` is *capability* — "this cannot work here at all". Keeping
them separate means a streamer who disables `!nuke`, then later revokes their
device, then re-pairs, still has `!nuke` off. Collapsing them loses that.

## Resolving the capability

The provisioning signal is a non-revoked row in `nivek.overlay_device`. That
table is keyed by `users.id`, while commands are keyed by `channel_twitch_id`,
so the join goes through `nivek.users.twitch_id` (UNIQUE):

```sql
SELECT EXISTS (
    SELECT 1
      FROM nivek.overlay_device d
      JOIN nivek.users u ON u.id = d.user_id
     WHERE u.twitch_id = $1
       AND d.revoked_at IS NULL
);
```

**This is a provisioning gate, not a liveness gate.** It deliberately does not
consult `Registry.IsConnected`. Two reasons: the bot process cannot see the
registry (core-api holds it), and an overlay that is merely closed right now
should still own its triggers — the relay's durable log is what covers the gap.
Liveness is the replay question below, not the dispatch question.

## Dispatch changes

### Loading

The bot already fetches per-channel state on `stream.online`
(`loadCustomCommands`, `internal/libraries/twitchbot/custom_commands.go`) and
drops it on `stream.offline`. Capabilities ride the same lifecycle: extend the
`GET /bot/commands/:bid` response with the channel's capability set, store it
next to `b.customCommands` under the same mutex, and evict it in
`dropCustomCommands`.

No new endpoint, no new network chatter, and the freshness story is the one the
bot already has — a streamer who pairs a device mid-stream picks it up on their
next go-live, exactly like a command they edited.

### The fall-through, which is the part that is easy to get wrong

Globals currently shadow customs: the word loop `continue`s on a builtin hit, so
a channel cannot override a builtin trigger. When an overlay command's
requirement is *not* met, dispatch must fall through **into the custom lookup**,
not `continue` the word loop:

```go
if handler, ok := b.commands[msgword]; ok {
    if req, gated := b.commandRequires[msgword]; !gated || b.channelHas(channel, req) {
        handler(b, &messageEvent)
        continue
    }
    // requirement unmet: fall through to the channel's own commands
}
if cmd, ok := b.customCommandFor(channel, msgword); ok { ... }
```

Skipping the word entirely would mean a channel with no overlay loses `!b` to
nothing at all — the global row claims the trigger and then declines to answer
it. That is worse than not registering the command.

### What a gated command does when unmet

Nothing. No reply, no hint. `!b` in a channel that never installed the overlay
is a command the chatter does not have, and an error line in someone else's chat
is spam.

## Trigger namespace

Three of the shipped triggers are single letters: `!b`, `!n`, `!g`. Promoting
those to the global namespace claims them across every channel the bot serves,
including channels already using them as custom commands. The fall-through above
softens this — an unmet requirement yields the trigger back — but a channel that
*does* have an overlay can no longer define its own `!b`.

Options, in increasing order of effort:

1. Accept it, and pick less collision-prone triggers for the overlay set.
2. Let capability-gated globals lose to channel customs rather than win, so a
   channel that has defined its own `!b` keeps it.
3. Namespace overlay commands behind a prefix (`!o b`), which is uglier to type
   and probably not worth it.

Worth deciding before the first overlay command ships, because changing it later
breaks muscle memory in every adopting channel.

## Replay: why `KindCommand` must be its own kind

The relay replays everything after a client's cursor with no per-kind expiry
(`EventsAfter`, and the paging loop in `cmd/core-api/endpoints/overlay/connect.go`).
That is deliberate and correct for money — a cheer that lands during an overlay
restart must not be lost.

It is wrong for a command. `!b` typed while the overlay is closed would fire on
reconnect, possibly hours later, dumping beans on screen with no chatter in
sight to explain them. Bits want durability; commands want fire-now-or-never.

Two ways to do that were on the table: a TTL stamped at ingest that
`EventsAfter` filters on, or simply not replaying `KindCommand` at all.

**The second is what shipped.** `EventsAfter` excludes `KindCommand` in its
`WHERE` clause, so a command is delivered live or not at all. An overlay's
cursor stays at the last non-command event it saw, which is correct — there is
nothing to catch up on.

The rows are still written, and the durable log still earns its place: it keeps
the per-user `seq` monotonic and applies the same unique-key dedupe every other
kind gets, so a command is never delivered twice across a reconnect. The
dedupe key is `<chat message id>:<action>` — per action, not per message, so
`!b 20 !nuke` in one line does not collapse into a single event.

## What this buys

The overlay command set becomes a **published capability** rather than a
per-streamer arrangement. Adoption is: install the overlay, sign in, mint a
device token, paste it. No DB row is written for that streamer that is not
written by the pairing flow itself. That is the property the whole relay exists
for, extended from events to commands.

## What shipped

| Piece | Where |
|---|---|
| `requires` column + the 13 command rows | `database/prod-apply-overlay-commands.sql` |
| `KindCommand`, `CommandPayload` | `internal/libraries/overlayrelay/const.go` |
| Replay exclusion | `EventsAfter`, `internal/libraries/overlayrelay/service.go` |
| Capability lookup | `HasActiveDevice`, same file |
| Dispatch route | `POST /api/bot/overlay/dispatch`, `cmd/core-api/endpoints/overlay/dispatch.go` |
| Capability set on the channel load | `cmd/core-api/endpoints/bot/commands.go` |
| `overlay_*` handlers | `internal/libraries/twitchbot/cmd_overlay.go` |
| The gate and its fall-through | `internal/libraries/twitchbot/bot_read_message.go` |

`buildCommandMaps` also rejects a mixed-case trigger at boot, for the reason in
the migration: the dispatch map is keyed on the column verbatim while the
message is lowercased, so `!zeroG` would be silently dead. Failing loud beats a
command nobody can explain.

## Not covered here

- **Which overlay commands become global** — settled in
  `database/prod-apply-overlay-commands.sql`. Eight actions on the physics and
  visual scene (`!b`, `!n`, `!shake`, `!laser`, `!nuke`, `!zerog`, `!g`,
  `!snow`), thirteen rows once the overlay's existing aliases are expanded,
  since the schema has no alias concept. Everything else the overlay registers
  either plays a specific audio file from the streamer's own sfx folder or
  drives their particular setup, so none of it means anything in a fresh
  install.
- **Cooldowns.** `cooldown_secs` exists on the column but nothing reads it. An
  overlay command is a much better argument for enforcing it than a chat reply
  is.
