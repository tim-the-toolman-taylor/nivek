package overlay

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/tim-the-toolman-taylor/nivek/cmd/core-api/coreconfig"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/overlayrelay"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/twitcheventsub"
)

const (
	// overlayBackgroundTTL bounds a fire-and-forget subscribe/unsubscribe kicked
	// off from a device mint/revoke, so it can never outlive the request by much.
	overlayBackgroundTTL = 20 * time.Second
	// reconcileTimeout bounds the whole boot-time reconcile pass.
	reconcileTimeout = 2 * time.Minute
)

// overlaySubTypes are the EventSub subscription types the overlay relay ingests.
// They must equal the types the ingest handler parses (overlayrelay.SubType*).
var overlaySubTypes = []string{overlayrelay.SubTypeCheer, overlayrelay.SubTypeRedemption}

// overlayConfigured reports whether the overlay relay's dedicated EventSub secret
// and callback are both set. Empty means the relay is disabled; we never build a
// client then, partly because twitcheventsub.NewClient would default a blank
// callback to the BOT's /eventsub.
func overlayConfigured(cfg coreconfig.CoreAPIConfig) bool {
	return cfg.OverlayEventSubSecret != "" && cfg.OverlayEventSubCallbackURL != ""
}

// newOverlayEventSubClient builds a twitcheventsub client whose transport points
// at the OVERLAY callback + secret (not the bot's). Callers must have checked
// overlayConfigured first.
func newOverlayEventSubClient(cfg coreconfig.CoreAPIConfig) (twitcheventsub.TwitchEventSubClient, error) {
	if !overlayConfigured(cfg) {
		return nil, errors.New("overlay eventsub secret/callback not configured")
	}
	return twitcheventsub.NewClient(twitcheventsub.Config{
		ClientID:       cfg.TwitchClientID,
		ClientSecret:   cfg.TwitchClientSecret,
		EventSubSecret: cfg.OverlayEventSubSecret,
		CallbackURL:    cfg.OverlayEventSubCallbackURL,
	})
}

// runOverlayBackground runs fn detached from the request with a bounded context
// and panic recovery. Mirrors the auth package's runBackground; kept local so
// this feature does not reach into that package.
func runOverlayBackground(logger *logrus.Logger, name string, fn func(context.Context)) {
	ctx, cancel := context.WithTimeout(context.Background(), overlayBackgroundTTL)
	defer cancel()
	defer func() {
		if recovered := recover(); recovered != nil {
			logger.Errorf("%s panic: %v", name, recovered)
		}
	}()
	fn(ctx)
}

// createOverlaySub creates one overlay subscription and logs the outcome. A 403
// means the broadcaster authorized before the cheer/redemption scopes were added
// -- it self-heals when they next sign in -- so it is logged and skipped, not
// treated as a failure. A 409 means the subscription already exists.
func createOverlaySub(ctx context.Context, client twitcheventsub.TwitchEventSubClient, subType, twitchID string, logger *logrus.Logger) {
	var (
		res twitcheventsub.SubscribeResult
		err error
	)
	switch subType {
	case overlayrelay.SubTypeCheer:
		res, err = client.SubscribeChannelCheer(ctx, twitchID)
	case overlayrelay.SubTypeRedemption:
		res, err = client.SubscribeChannelPointsRedemptionAdd(ctx, twitchID)
	default:
		return
	}

	switch {
	case err != nil:
		logger.Errorf("overlay eventsub FAIL %s twitch_id=%s err=%v", subType, twitchID, err)
	case res.StatusCode == http.StatusForbidden:
		logger.Warnf("overlay eventsub %s twitch_id=%s not authorized: broadcaster must re-auth to grant bits:read/channel:read:redemptions", subType, twitchID)
	case res.AlreadyExists():
		logger.Infof("overlay eventsub %s already-subscribed twitch_id=%s", subType, twitchID)
	case res.OK():
		logger.Infof("overlay eventsub subscribed %s twitch_id=%s status=%d", subType, twitchID, res.StatusCode)
	default:
		logger.Errorf("overlay eventsub FAIL %s twitch_id=%s status=%d body=%s", subType, twitchID, res.StatusCode, string(res.Body))
	}
}

// subscribeOverlayWebhooks ensures the cheer + redemption subscriptions exist for
// one broadcaster on the overlay callback. Idempotent (409 = already exists).
func subscribeOverlayWebhooks(ctx context.Context, cfg coreconfig.CoreAPIConfig, twitchID string, logger *logrus.Logger) {
	if !overlayConfigured(cfg) {
		return
	}
	client, err := newOverlayEventSubClient(cfg)
	if err != nil {
		logger.Errorf("overlay eventsub: create client: %s", err.Error())
		return
	}
	for _, subType := range overlaySubTypes {
		createOverlaySub(ctx, client, subType, twitchID, logger)
	}
}

// unsubscribeOverlayWebhooks deletes the overlay cheer/redemption subscriptions
// for one broadcaster. The callback + type filter guarantees it never touches the
// bot's stream/chat subscriptions, which share the app but use a different
// callback. DeleteEventSubSubscription is idempotent.
func unsubscribeOverlayWebhooks(ctx context.Context, cfg coreconfig.CoreAPIConfig, twitchID string, logger *logrus.Logger) {
	if !overlayConfigured(cfg) {
		return
	}
	client, err := newOverlayEventSubClient(cfg)
	if err != nil {
		logger.Errorf("overlay eventsub: create client: %s", err.Error())
		return
	}
	subs, err := client.ListEventSubSubscriptions(ctx)
	if err != nil {
		logger.Errorf("overlay eventsub: list for unsubscribe twitch_id=%s: %v", twitchID, err)
		return
	}
	for _, sub := range subs {
		if !isOverlaySub(sub, cfg.OverlayEventSubCallbackURL) || sub.Condition.BroadcasterUserID != twitchID {
			continue
		}
		if err := client.DeleteEventSubSubscription(ctx, sub.ID); err != nil {
			logger.Errorf("overlay eventsub: delete %s (%s) twitch_id=%s: %v", sub.ID, sub.Type, twitchID, err)
		} else {
			logger.Infof("overlay eventsub: deleted %s (%s) twitch_id=%s", sub.ID, sub.Type, twitchID)
		}
	}
}

// isOverlaySub reports whether a subscription is one of ours: an overlay type on
// the overlay callback over webhook transport.
func isOverlaySub(sub twitcheventsub.EventSubSubscription, overlayCallback string) bool {
	if sub.Transport.Method != "webhook" || sub.Transport.Callback != overlayCallback {
		return false
	}
	return sub.Type == overlayrelay.SubTypeCheer || sub.Type == overlayrelay.SubTypeRedemption
}

// ReconcileOverlaySubscriptions converges Twitch's overlay subscriptions with the
// set of broadcasters who currently run an overlay (hold an active device): it
// creates any missing cheer/redemption subscription and prunes overlay
// subscriptions for broadcasters who no longer have a device (orphans from a
// failed revoke-delete). It is a no-op when the relay is unconfigured, and is
// safe to run on every boot (409 = already exists, delete is idempotent).
func ReconcileOverlaySubscriptions(ctx context.Context, cfg coreconfig.CoreAPIConfig, relay overlayrelay.Service, logger *logrus.Logger) {
	if !overlayConfigured(cfg) {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, reconcileTimeout)
	defer cancel()
	defer func() {
		if recovered := recover(); recovered != nil {
			logger.Errorf("overlay reconcile panic: %v", recovered)
		}
	}()

	ids, err := relay.BroadcastersWithActiveDevices()
	if err != nil {
		logger.Errorf("overlay reconcile: list broadcasters: %v", err)
		return
	}

	client, err := newOverlayEventSubClient(cfg)
	if err != nil {
		logger.Errorf("overlay reconcile: create client: %v", err)
		return
	}

	reconcileSubscriptions(ctx, client, cfg.OverlayEventSubCallbackURL, ids, logger)
}

// reconcileSubscriptions is the pure convergence step over a given client: create
// the missing cheer/redemption subs for each active broadcaster and prune overlay
// subs for broadcasters no longer in the set. Split out so it can be tested with a
// fake client.
func reconcileSubscriptions(ctx context.Context, client twitcheventsub.TwitchEventSubClient, overlayCallback string, activeIDs []string, logger *logrus.Logger) {
	existing, err := client.ListEventSubSubscriptions(ctx)
	if err != nil {
		logger.Errorf("overlay reconcile: list subscriptions: %v", err)
		return
	}

	// Index the enabled overlay subs already present, keyed by broadcaster+type,
	// so we only POST creates for genuinely missing ones.
	enabled := make(map[string]bool)
	for _, sub := range existing {
		if isOverlaySub(sub, overlayCallback) && sub.Status == twitcheventsub.StatusEnabled {
			enabled[sub.Condition.BroadcasterUserID+"|"+sub.Type] = true
		}
	}

	idSet := make(map[string]bool, len(activeIDs))
	for _, id := range activeIDs {
		idSet[id] = true
		for _, subType := range overlaySubTypes {
			if !enabled[id+"|"+subType] {
				createOverlaySub(ctx, client, subType, id, logger)
			}
		}
	}

	// Prune overlay subs for broadcasters who no longer run an overlay.
	for _, sub := range existing {
		if !isOverlaySub(sub, overlayCallback) || idSet[sub.Condition.BroadcasterUserID] {
			continue
		}
		if err := client.DeleteEventSubSubscription(ctx, sub.ID); err != nil {
			logger.Errorf("overlay reconcile: prune %s (%s) broadcaster=%s: %v", sub.ID, sub.Type, sub.Condition.BroadcasterUserID, err)
		} else {
			logger.Infof("overlay reconcile: pruned %s (%s) broadcaster=%s", sub.ID, sub.Type, sub.Condition.BroadcasterUserID)
		}
	}

	logger.Infof("overlay reconcile: complete for %d broadcaster(s) with active devices", len(activeIDs))
}
