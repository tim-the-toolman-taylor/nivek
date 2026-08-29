# Logging

The repo runs two kinds of process, and each gets one logger. Which one you use
is decided by whether the code holds a `nivek.NivekService`, not by taste.

## The rule

| Code | Logger |
| --- | --- |
| Anything holding a `nivek.NivekService` — `cmd/core-api/**` and the `internal/libraries/*` services built with `NewService(nivek)` | `svc.Logger()` (logrus) |
| The standalone binaries and the libraries only they use — `cmd/twitch-bot`, `cmd/df-executor`, `cmd/df-snapshot-pusher`, `cmd/eventsub-*`, `cmd/twitch-bot-auth`, `internal/libraries/twitchbot`, `internal/libraries/overseer`, `internal/libraries/twitcheventsub` | stdlib `log` |

If a function needs to log but has no service, take the logger as a parameter
(`logger *logrus.Logger`) rather than reaching for a package global — see
`utilities.GetUserFromContext`.

## Why package-level `logrus` is banned in core-api

`logrus.Errorf(...)` writes to `logrus.StandardLogger()`, which is a *different*
instance from the one `nivek.NewNivekService` builds and hands out through
`svc.Logger()`. `cmd/core-api/main.go` registers the Discord alerting hook on the
service logger only:

```go
if hook := alerting.NewDiscordErrorHook(); hook.Enabled() {
    svc.Logger().AddHook(hook)
}
```

So an error logged through the package-level logger — or through stdlib `log` —
is written to container stdout and nothing else. It never pages anyone. That is
the exact failure `internal/libraries/alerting` was written to close, so the
alerting package's promise ("every `Logger().Errorf` becomes a Discord ping")
only holds if core-api code actually logs through `svc.Logger()`.

## Deliberate exceptions

Three places use stdlib `log` inside otherwise service-owning code, on purpose:

- `internal/libraries/config/config.go` and `cmd/core-api/coreconfig/coreconfig.go`
  parse configuration *before* the service and its logger exist.
- `internal/libraries/alerting/discord_hook.go` logs its own webhook failures
  with stdlib `log`. Routing a hook's failure back through logrus would re-fire
  the hook and loop.

`internal/libraries/twitcheventsub` is imported by both core-api and the bot, so
it stays on stdlib `log` — the lowest common denominator — rather than forcing a
service dependency onto the standalone binaries.
