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
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"time"

	"github.com/joho/godotenv"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/api"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/twitcheventsub"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/user"
)

// requiredTypes are the subscription types every opted-in channel must have.
var requiredTypes = []string{"stream.online", "stream.offline", "channel.chat.message"}

// errChatAuthMissing marks a channel.chat.message subscription Twitch refused
// with 403 because the broadcaster hasn't granted chat-read (no moderator
// status, no channel:bot). It's an opt-in gap, not a health failure, so the
// audit reports it separately and never exits non-zero for it.
var errChatAuthMissing = errors.New("chat-read not authorized (needs /mod or channel:bot)")

func main() {
	dryRun := flag.Bool("dry-run", false, "audit only; do not create or delete subscriptions")
	fixCallback := flag.Bool("fix-callback", false, "also recreate subscriptions whose callback URL differs from the expected one")
	force := flag.Bool("force", false, "recreate ALL subscriptions even if they look healthy — use after rotating TWITCH_EVENTSUB_SECRET, since a stale-secret sub still reports enabled")
	listOnly := flag.Bool("list", false, "print every opted-in user's subscriptions (type/status/callback) and exit; diagnostic only, changes nothing")
	pruneOrphans := flag.Bool("prune-orphans", false, "delete subscriptions whose broadcaster is NOT in the opted-in set (banished/opted-out leftovers) and exit; honors -dry-run")
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

	if *listOnly {
		listSubscriptions(users, byBroadcaster, allSubs)
		return
	}

	if *pruneOrphans {
		pruneOrphanSubscriptions(ctx, client, users, allSubs, *dryRun, *delay)
		return
	}

	var healthy, repaired, failed, wouldRepair, unauthorized int
	for i, u := range users {
		if u.TwitchID == nil || u.TwitchLogin == nil {
			log.Printf("user %d missing TwitchId or TwitchLogin. Skipping...", u.Id)
			continue
		}

		twitchID := *u.TwitchID
		subs := byBroadcaster[twitchID]

		for _, typ := range requiredTypes {
			good, bad := classify(subs, typ, callback, *fixCallback, *force)

			// Healthy: exactly one good subscription and nothing to clean up.
			// -force skips this so every sub is torn down and recreated.
			if !*force && len(good) == 1 && len(bad) == 0 {
				healthy++
				continue
			}

			reason := describe(subs, typ)
			if *force {
				reason = "force-recreate; " + reason
			}
			if *dryRun {
				wouldRepair++
				log.Printf("[%d/%d] WOULD REPAIR %s user=%s twitch_id=%s (%s)",
					i+1, len(users), typ, *u.TwitchLogin, twitchID, reason)
				continue
			}

			log.Printf("[%d/%d] repairing %s user=%s twitch_id=%s (%s)",
				i+1, len(users), typ, *u.TwitchLogin, twitchID, reason)
			if err := repair(ctx, client, typ, twitchID, good, bad); err != nil {
				if errors.Is(err, errChatAuthMissing) {
					unauthorized++
					log.Printf("[%d/%d] SKIP %s user=%s twitch_id=%s (not authorized — needs /mod or channel:bot)",
						i+1, len(users), typ, *u.TwitchLogin, twitchID)
					continue
				}
				failed++
				log.Printf("FAIL %s user=%s twitch_id=%s err=%v", typ, *u.TwitchLogin, twitchID, err)
				continue
			}
			repaired++
			log.Printf("OK %s user=%s twitch_id=%s now has one enabled subscription", typ, *u.TwitchLogin, twitchID)
		}

		if i < len(users)-1 && *delay > 0 {
			time.Sleep(*delay)
		}
	}

	if *dryRun {
		log.Printf("dry-run complete: healthy=%d would_repair=%d users=%d", healthy, wouldRepair, len(users))
		return
	}
	log.Printf("done: healthy=%d repaired=%d unauthorized=%d failed=%d users=%d",
		healthy, repaired, unauthorized, failed, len(users))
	if failed > 0 {
		os.Exit(1)
	}
}

// listSubscriptions prints, for every opted-in user, their subscriptions of
// each required type with status + callback (or MISSING), then any orphan subs
// whose broadcaster is not in the opted-in set. Pure diagnostic; mutates nothing.
func listSubscriptions(users []user.User, byBroadcaster map[string][]twitcheventsub.EventSubSubscription, all []twitcheventsub.EventSubSubscription) {
	optedIn := make(map[string]string, len(users)) // twitch_id -> username
	for _, u := range users {
		if u.TwitchID == nil || u.TwitchLogin == nil {
			log.Printf("user %d missing TwitchId or TwitchLogin. Skipping...", u.Id)
			continue
		}
		optedIn[*u.TwitchID] = *u.TwitchLogin
	}

	for _, u := range users {
		if u.TwitchID == nil || u.TwitchLogin == nil {
			log.Printf("user %d missing TwitchId or TwitchLogin. Skipping...", u.Id)
			continue
		}
		tid := *u.TwitchID
		fmt.Printf("\n%s (twitch_id=%s)\n", *u.TwitchLogin, tid)
		subs := byBroadcaster[tid]
		for _, typ := range requiredTypes {
			found := false
			for _, s := range subs {
				if s.Type == typ {
					fmt.Printf("  %-15s %-42s %s\n", typ, s.Status, s.Transport.Callback)
					found = true
				}
			}
			if !found {
				fmt.Printf("  %-15s %s\n", typ, "MISSING")
			}
		}
	}

	var orphans []twitcheventsub.EventSubSubscription
	for _, s := range all {
		if _, ok := optedIn[s.Condition.BroadcasterUserID]; !ok {
			orphans = append(orphans, s)
		}
	}
	if len(orphans) > 0 {
		fmt.Printf("\norphan subscriptions (broadcaster not in the opted-in set):\n")
		for _, s := range orphans {
			fmt.Printf("  %-15s %-42s bid=%s cb=%s\n", s.Type, s.Status, s.Condition.BroadcasterUserID, s.Transport.Callback)
		}
	}
}

// pruneOrphanSubscriptions deletes every subscription whose broadcaster is not
// in the opted-in set — leftovers from channels that opted out via !banish (we
// don't unsubscribe on banish, so their subs linger). Honors dryRun.
func pruneOrphanSubscriptions(ctx context.Context, client *twitcheventsub.Client, users []user.User, all []twitcheventsub.EventSubSubscription, dryRun bool, delay time.Duration) {
	optedIn := make(map[string]struct{}, len(users))
	for _, u := range users {
		optedIn[*u.TwitchID] = struct{}{}
	}

	var orphans []twitcheventsub.EventSubSubscription
	for _, s := range all {
		if _, ok := optedIn[s.Condition.BroadcasterUserID]; !ok {
			orphans = append(orphans, s)
		}
	}
	if len(orphans) == 0 {
		log.Printf("no orphan subscriptions to prune")
		return
	}
	log.Printf("found %d orphan subscription(s) (broadcaster not in the opted-in set)", len(orphans))

	var deleted, failed int
	for i, s := range orphans {
		if dryRun {
			log.Printf("WOULD DELETE %s %s bid=%s id=%s", s.Type, s.Status, s.Condition.BroadcasterUserID, s.ID)
			continue
		}
		if err := client.DeleteEventSubSubscription(ctx, s.ID); err != nil {
			failed++
			log.Printf("FAIL delete %s bid=%s id=%s: %v", s.Type, s.Condition.BroadcasterUserID, s.ID, err)
		} else {
			deleted++
			log.Printf("deleted %s bid=%s id=%s", s.Type, s.Condition.BroadcasterUserID, s.ID)
		}
		if i < len(orphans)-1 && delay > 0 {
			time.Sleep(delay)
		}
	}

	if dryRun {
		log.Printf("dry-run complete: would prune %d orphan(s)", len(orphans))
		return
	}
	log.Printf("prune complete: deleted=%d failed=%d", deleted, failed)
	if failed > 0 {
		os.Exit(1)
	}
}

// classify splits a broadcaster's subscriptions of one type into good (healthy,
// keep) and bad (delete). A subscription is good when its status is enabled and,
// when fixCallback is set, its callback matches the expected one.
func classify(subs []twitcheventsub.EventSubSubscription, typ, callback string, fixCallback, force bool) (good, bad []twitcheventsub.EventSubSubscription) {
	for _, s := range subs {
		if s.Type != typ {
			continue
		}
		// force treats every existing subscription as replaceable so it gets
		// deleted and recreated with the current secret (post-rotation repair).
		ok := !force && s.Status == twitcheventsub.StatusEnabled
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
	if typ == "channel.chat.message" && result.StatusCode == http.StatusForbidden {
		return errChatAuthMissing // broadcaster hasn't modded the bot / granted channel:bot yet
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
	case "channel.chat.message":
		return client.SubscribeChannelChatMessages(ctx, twitchID)
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
