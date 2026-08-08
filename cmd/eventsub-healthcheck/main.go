// eventsub-healthcheck is a one-shot CLI that audits the EventSub webhook
// subscriptions for every opted-in user (bot_opt_in=true) and repairs any that
// are unhealthy — the tool to reach for when a channel is live but is_live never
// flipped, i.e. Twitch stopped delivering go-live notifications.
//
// For each user it requires exactly one healthy stream.online AND one healthy
// stream.offline subscription. "Healthy" means Helix status == "enabled".
// Anything else — missing, duplicated, or in a failed/revoked/pending status —
// is repaired by deleting the offending subscription(s) and creating a fresh
// one. Healthy users are left untouched, so the tool is safe to re-run.
//
// Callback drift (a subscription pointing at an unexpected callback URL) is only
// REPORTED by default, not repaired, because the expected callback differs by
// environment; pass -fix-callback to also recreate those.
//
// Intended to be run manually on prod with the same env as core-api (ssh +
// binary). Read-only unless it finds something to fix; use -dry-run to audit
// without touching Twitch.
//
// Usage:
//
//	eventsub-healthcheck [-dry-run] [-fix-callback] [-delay 200ms]
//
// Required env (same names as core-api / eventsub-backfill):
//
//	TWITCH_CLIENT_ID
//	TWITCH_CLIENT_SECRET
//	TWITCH_EVENTSUB_SECRET
//	CORE_API_URL
//	BOT_API_HMAC_KEY
//
// Optional:
//
//	TWITCH_EVENTSUB_CALLBACK (default https://peanutbudderbot.com/eventsub)
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"time"

	"github.com/joho/godotenv"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/api"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/twitcheventsub"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/user"
)

// requiredTypes are the subscription types every opted-in channel must have.
var requiredTypes = []string{"stream.online", "stream.offline"}

func main() {
	dryRun := flag.Bool("dry-run", false, "audit only; do not create or delete subscriptions")
	fixCallback := flag.Bool("fix-callback", false, "also recreate subscriptions whose callback URL differs from the expected one")
	delay := flag.Duration("delay", 200*time.Millisecond, "pause between users to stay under Helix rate limits")
	flag.Parse()

	_ = godotenv.Load()

	clientID := mustEnv("TWITCH_CLIENT_ID")
	clientSecret := mustEnv("TWITCH_CLIENT_SECRET")
	eventSubSecret := mustEnv("TWITCH_EVENTSUB_SECRET")
	callback := envOr("TWITCH_EVENTSUB_CALLBACK", fmt.Sprintf("https://peanutbudderbot.com%s", api.TwitchWebhookSubscriptionRequest))

	ctx := context.Background()

	client, err := twitcheventsub.NewClient(twitcheventsub.Config{
		ClientID:       clientID,
		ClientSecret:   clientSecret,
		EventSubSecret: eventSubSecret,
		CallbackURL:    callback,
	})
	if err != nil {
		log.Fatalf("eventsub client: %v", err)
	}

	users, err := loadOptedInUsers()
	if err != nil {
		log.Fatalf("load users: %v", err)
	}
	log.Printf("auditing %d opted-in users with a twitch_id (expected callback: %s)", len(users), callback)

	// One list call, then index every subscription by broadcaster_user_id so the
	// per-user audit is a local map lookup rather than an API call each.
	allSubs, err := client.ListEventSubSubscriptions(ctx)
	if err != nil {
		log.Fatalf("list subscriptions: %v", err)
	}
	byBroadcaster := make(map[string][]twitcheventsub.EventSubSubscription)
	for _, s := range allSubs {
		byBroadcaster[s.Condition.BroadcasterUserID] = append(byBroadcaster[s.Condition.BroadcasterUserID], s)
	}
	log.Printf("found %d total EventSub subscriptions across the app", len(allSubs))

	var healthy, repaired, failed, wouldRepair int
	for i, u := range users {
		twitchID := *u.TwitchID
		subs := byBroadcaster[twitchID]

		for _, typ := range requiredTypes {
			good, bad := classify(subs, typ, callback, *fixCallback)

			// Healthy: exactly one good subscription and nothing to clean up.
			if len(good) == 1 && len(bad) == 0 {
				healthy++
				continue
			}

			reason := describe(subs, typ)
			if *dryRun {
				wouldRepair++
				log.Printf("[%d/%d] WOULD REPAIR %s user=%s twitch_id=%s (%s)",
					i+1, len(users), typ, u.Username, twitchID, reason)
				continue
			}

			log.Printf("[%d/%d] repairing %s user=%s twitch_id=%s (%s)",
				i+1, len(users), typ, u.Username, twitchID, reason)
			if err := repair(ctx, client, typ, twitchID, good, bad); err != nil {
				failed++
				log.Printf("        FAIL %s user=%s twitch_id=%s err=%v", typ, u.Username, twitchID, err)
				continue
			}
			repaired++
			log.Printf("        OK %s user=%s twitch_id=%s now has one enabled subscription", typ, u.Username, twitchID)
		}

		if i < len(users)-1 && *delay > 0 {
			time.Sleep(*delay)
		}
	}

	if *dryRun {
		log.Printf("dry-run complete: healthy=%d would_repair=%d users=%d", healthy, wouldRepair, len(users))
		return
	}
	log.Printf("done: healthy=%d repaired=%d failed=%d users=%d", healthy, repaired, failed, len(users))
	if failed > 0 {
		os.Exit(1)
	}
}

// classify splits a broadcaster's subscriptions of one type into good (healthy,
// keep) and bad (delete). A subscription is good when its status is enabled and,
// when fixCallback is set, its callback matches the expected one.
func classify(subs []twitcheventsub.EventSubSubscription, typ, callback string, fixCallback bool) (good, bad []twitcheventsub.EventSubSubscription) {
	for _, s := range subs {
		if s.Type != typ {
			continue
		}
		ok := s.Status == twitcheventsub.StatusEnabled
		if ok && fixCallback && s.Transport.Callback != callback {
			ok = false
		}
		if ok {
			good = append(good, s)
		} else {
			bad = append(bad, s)
		}
	}
	return good, bad
}

// repair converges a broadcaster's subscriptions of one type to exactly one
// enabled subscription: delete every bad one, drop any surplus good ones, and
// create a fresh subscription if none healthy remain.
func repair(ctx context.Context, client *twitcheventsub.Client, typ, twitchID string, good, bad []twitcheventsub.EventSubSubscription) error {
	for _, s := range bad {
		if err := client.DeleteEventSubSubscription(ctx, s.ID); err != nil {
			return fmt.Errorf("delete %s (%s): %w", s.ID, s.Status, err)
		}
	}
	// Keep at most one healthy subscription; delete duplicates.
	for _, s := range good[min(len(good), 1):] {
		if err := client.DeleteEventSubSubscription(ctx, s.ID); err != nil {
			return fmt.Errorf("delete surplus %s: %w", s.ID, err)
		}
	}
	if len(good) >= 1 {
		return nil // one healthy subscription already remains
	}

	result, err := subscribe(ctx, client, typ, twitchID)
	if err != nil {
		return fmt.Errorf("create %s: %w", typ, err)
	}
	if !result.OK() && !result.AlreadyExists() {
		return fmt.Errorf("create %s returned %d: %s", typ, result.StatusCode, string(result.Body))
	}
	return nil
}

func subscribe(ctx context.Context, client *twitcheventsub.Client, typ, twitchID string) (twitcheventsub.SubscribeResult, error) {
	switch typ {
	case "stream.online":
		return client.SubscribeStreamOnline(ctx, twitchID)
	case "stream.offline":
		return client.SubscribeStreamOffline(ctx, twitchID)
	default:
		return twitcheventsub.SubscribeResult{}, fmt.Errorf("unknown subscription type %q", typ)
	}
}

// describe summarizes the statuses present for a type, for readable log lines.
func describe(subs []twitcheventsub.EventSubSubscription, typ string) string {
	var statuses []string
	for _, s := range subs {
		if s.Type == typ {
			statuses = append(statuses, s.Status)
		}
	}
	if len(statuses) == 0 {
		return "missing"
	}
	sort.Strings(statuses)
	return fmt.Sprintf("found: %v", statuses)
}

func loadOptedInUsers() ([]user.User, error) {
	coreAPIURL := envOr("CORE_API_URL", "")
	botHmacKey := envOr("BOT_API_HMAC_KEY", "")
	if coreAPIURL == "" || botHmacKey == "" {
		log.Fatal("Missing required environment variables: CORE_API_URL, BOT_API_HMAC_KEY")
	}

	coreAPI, err := api.NewCoreAPIClient(coreAPIURL, botHmacKey)
	if err != nil {
		log.Fatalf("Failed to create core-api client: %v", err)
	}

	// GetActiveChannels returns users with bot_opt_in=true.
	users, err := coreAPI.GetActiveChannels()
	if err != nil {
		log.Fatalf("Failed to fetch active channels from core-api: %v", err)
	}

	out := make([]user.User, 0, len(users))
	for _, u := range users {
		if u.TwitchID != nil && *u.TwitchID != "" {
			out = append(out, u)
		}
	}
	if len(out) == 0 {
		log.Fatal("No opted-in users with a twitch_id returned by core-api")
	}
	return out, nil
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("missing required env %s", key)
	}
	return v
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
