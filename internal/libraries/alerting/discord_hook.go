// Package alerting forwards production error-level log events to a Discord
// webhook so failures page a human in real time instead of dying silently in
// container logs. It exists because a new-user signup INSERT failure went
// unnoticed for ~2 months: the error was logged, but nothing was watching the
// logs. This closes that gap for the whole codebase at once — every
// Logger().Errorf becomes a Discord ping.
package alerting

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	// AlertWebhookEnv names the Discord webhook the hook posts to. Kept separate
	// from the bot's go-live announcement webhook so operational alerts land in a
	// private admin channel, not the public "who's live" feed.
	AlertWebhookEnv = "CORE_API_ALERT_WEBHOOK"

	alertHTTPTimeout = 10 * time.Second
	// dedupeWindow suppresses repeats of an identical message — a crash loop
	// emitting the same error 400× pages you once, not 400 times.
	dedupeWindow = 5 * time.Minute
	// minInterval is a global floor between posts so a burst of *distinct*
	// errors can't flood the channel.
	minInterval = 2 * time.Second
	// discordContentLimit is Discord's hard cap on message content length.
	discordContentLimit = 2000
)

// DiscordErrorHook is a logrus hook that forwards Error/Fatal/Panic entries to a
// Discord webhook. It is inert when CORE_API_ALERT_WEBHOOK is unset.
//
// The hook deliberately uses the stdlib log package (never logrus) for its OWN
// failures: routing a hook error back through logrus would re-fire the hook and
// loop. Posts happen in a goroutine so a slow Discord never blocks the request
// that logged the error.
type DiscordErrorHook struct {
	webhookURL string
	client     *http.Client

	mu       sync.Mutex
	lastSent map[string]time.Time // message -> last post time (dedupe)
	lastPost time.Time            // last post of any message (burst floor)
}

// NewDiscordErrorHook reads the webhook URL from the environment. Call Enabled()
// to decide whether to register it.
func NewDiscordErrorHook() *DiscordErrorHook {
	return &DiscordErrorHook{
		webhookURL: strings.TrimSpace(os.Getenv(AlertWebhookEnv)),
		client:     &http.Client{Timeout: alertHTTPTimeout},
		lastSent:   make(map[string]time.Time),
	}
}

// Enabled reports whether a webhook URL is configured.
func (h *DiscordErrorHook) Enabled() bool { return h.webhookURL != "" }

// Levels restricts the hook to genuine failures — info/warn/debug never page.
func (h *DiscordErrorHook) Levels() []logrus.Level {
	return []logrus.Level{logrus.PanicLevel, logrus.FatalLevel, logrus.ErrorLevel}
}

// Fire is called by logrus for every Error/Fatal/Panic entry.
func (h *DiscordErrorHook) Fire(entry *logrus.Entry) error {
	if h.webhookURL == "" {
		return nil
	}
	if !h.allow(entry.Message) {
		return nil
	}
	go h.post(formatEntry(entry), entry.Message)
	return nil
}

// allow applies dedupe + burst limiting and records the decision. Returns true
// if this message should be posted now.
func (h *DiscordErrorHook) allow(msg string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	now := time.Now()
	if last, ok := h.lastSent[msg]; ok && now.Sub(last) < dedupeWindow {
		return false
	}
	if !h.lastPost.IsZero() && now.Sub(h.lastPost) < minInterval {
		return false
	}

	h.lastSent[msg] = now
	h.lastPost = now

	// Bound the dedupe map: drop entries older than the window whenever it grows.
	if len(h.lastSent) > 256 {
		for k, t := range h.lastSent {
			if now.Sub(t) > dedupeWindow {
				delete(h.lastSent, k)
			}
		}
	}
	return true
}

func (h *DiscordErrorHook) post(content, msg string) {
	body, err := json.Marshal(map[string]string{"content": content})
	if err != nil {
		log.Printf("[ALERT] marshal failed: %v", err)
		return
	}

	resp, err := h.client.Post(h.webhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("[ALERT] POST failed for %q: %v", msg, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("[ALERT] POST for %q returned status %d", msg, resp.StatusCode)
	}
}

// formatEntry renders a logrus entry as Discord message content, appending any
// structured fields and truncating to Discord's content limit.
func formatEntry(entry *logrus.Entry) string {
	var b strings.Builder
	fmt.Fprintf(&b, "🚨 **core-api %s**\n```\n%s", strings.ToUpper(entry.Level.String()), entry.Message)
	if len(entry.Data) > 0 {
		b.WriteString("\n")
		for k, v := range entry.Data {
			fmt.Fprintf(&b, "%s=%v ", k, v)
		}
	}
	b.WriteString("\n```")

	s := b.String()
	if len(s) > discordContentLimit {
		s = s[:discordContentLimit-1] + "…"
	}
	return s
}
