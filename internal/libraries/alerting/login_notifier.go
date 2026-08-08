package alerting

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

// LoginNotifier posts human-friendly sign-in notifications to the same Discord
// webhook the error hook uses (CORE_API_ALERT_WEBHOOK). It is intentionally
// separate from DiscordErrorHook: logins are informational, so they carry no
// dedupe/burst limiting and read as a friendly line rather than a 🚨 alert.
// Inert when the webhook env var is unset.
type LoginNotifier struct {
	webhookURL string
	client     *http.Client
}

// NewLoginNotifier reads the webhook URL from the environment at construction,
// matching DiscordErrorHook's behavior.
func NewLoginNotifier() *LoginNotifier {
	return &LoginNotifier{
		webhookURL: strings.TrimSpace(os.Getenv(AlertWebhookEnv)),
		client:     &http.Client{Timeout: 10 * time.Second},
	}
}

// Enabled reports whether a webhook URL is configured.
func (n *LoginNotifier) Enabled() bool { return n.webhookURL != "" }

// NotifyLogin posts a sign-in notification. No-op when the webhook is unset.
// Network-bound — call it in a goroutine so it never blocks the OAuth response.
func (n *LoginNotifier) NotifyLogin(login string, isNew bool) {
	if n.webhookURL == "" {
		return
	}

	emoji, kind := "👋", "returning"
	if isNew {
		emoji, kind = "🎉", "NEW"
	}
	content := fmt.Sprintf("%s **%s** signed in (%s user)", emoji, login, kind)
	postWebhook(n.client, n.webhookURL, content, "login "+login)
}
