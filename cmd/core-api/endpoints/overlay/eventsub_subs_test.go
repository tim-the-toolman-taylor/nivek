package overlay

import (
	"context"
	"io"
	"net/http"
	"sort"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/tim-the-toolman-taylor/nivek/cmd/core-api/coreconfig"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/overlayrelay"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/twitcheventsub"
)

const overlayCB = "https://peanutbudderbot.com/api/overlay/eventsub"

func quietLogger() *logrus.Logger {
	l := logrus.New()
	l.SetOutput(io.Discard)
	return l
}

// fakeEventSubClient embeds the interface (so unimplemented methods panic if hit)
// and records the create/delete calls the reconcile makes.
type fakeEventSubClient struct {
	twitcheventsub.TwitchEventSubClient
	existing  []twitcheventsub.EventSubSubscription
	created   []string // "type|broadcaster"
	deleted   []string // subscription ids
	subResult twitcheventsub.SubscribeResult
}

func (f *fakeEventSubClient) ListEventSubSubscriptions(context.Context) ([]twitcheventsub.EventSubSubscription, error) {
	return f.existing, nil
}

func (f *fakeEventSubClient) DeleteEventSubSubscription(_ context.Context, id string) error {
	f.deleted = append(f.deleted, id)
	return nil
}

func (f *fakeEventSubClient) SubscribeChannelCheer(_ context.Context, id string) (twitcheventsub.SubscribeResult, error) {
	f.created = append(f.created, overlayrelay.SubTypeCheer+"|"+id)
	return f.subResult, nil
}

func (f *fakeEventSubClient) SubscribeChannelPointsRedemptionAdd(_ context.Context, id string) (twitcheventsub.SubscribeResult, error) {
	f.created = append(f.created, overlayrelay.SubTypeRedemption+"|"+id)
	return f.subResult, nil
}

func sub(id, subType, broadcaster, callback, status string) twitcheventsub.EventSubSubscription {
	var s twitcheventsub.EventSubSubscription
	s.ID = id
	s.Type = subType
	s.Status = status
	s.Condition.BroadcasterUserID = broadcaster
	s.Transport.Method = "webhook"
	s.Transport.Callback = callback
	return s
}

func TestIsOverlaySub(t *testing.T) {
	t.Parallel()
	cheerOnOverlay := sub("1", overlayrelay.SubTypeCheer, "111", overlayCB, twitcheventsub.StatusEnabled)
	if !isOverlaySub(cheerOnOverlay, overlayCB) {
		t.Fatal("overlay cheer on overlay callback should match")
	}
	// Overlay type but bot callback -> not ours (guards against deleting bot subs).
	if isOverlaySub(sub("2", overlayrelay.SubTypeCheer, "111", "https://peanutbudderbot.com/eventsub", twitcheventsub.StatusEnabled), overlayCB) {
		t.Fatal("overlay type on a different callback must not match")
	}
	// Bot type on the overlay callback -> not ours.
	if isOverlaySub(sub("3", "stream.online", "111", overlayCB, twitcheventsub.StatusEnabled), overlayCB) {
		t.Fatal("non-overlay type must not match")
	}
	// Non-webhook transport -> not ours.
	ws := cheerOnOverlay
	ws.Transport.Method = "websocket"
	if isOverlaySub(ws, overlayCB) {
		t.Fatal("non-webhook transport must not match")
	}
}

func TestOverlayConfigured(t *testing.T) {
	t.Parallel()
	both := coreconfig.CoreAPIConfig{OverlayEventSubSecret: "s", OverlayEventSubCallbackURL: overlayCB}
	if !overlayConfigured(both) {
		t.Fatal("both set should be configured")
	}
	for _, c := range []coreconfig.CoreAPIConfig{
		{OverlayEventSubSecret: "s"},
		{OverlayEventSubCallbackURL: overlayCB},
		{},
	} {
		if overlayConfigured(c) {
			t.Fatalf("partial/empty config should not be configured: %+v", c)
		}
	}
}

func TestReconcileCreatesMissing(t *testing.T) {
	t.Parallel()
	f := &fakeEventSubClient{subResult: twitcheventsub.SubscribeResult{StatusCode: http.StatusAccepted}}
	reconcileSubscriptions(context.Background(), f, overlayCB, []string{"111"}, quietLogger())

	sort.Strings(f.created)
	want := []string{overlayrelay.SubTypeCheer + "|111", overlayrelay.SubTypeRedemption + "|111"}
	sort.Strings(want)
	if len(f.created) != 2 || f.created[0] != want[0] || f.created[1] != want[1] {
		t.Fatalf("created = %v, want both types for 111", f.created)
	}
	if len(f.deleted) != 0 {
		t.Fatalf("nothing should be deleted, got %v", f.deleted)
	}
}

func TestReconcileSkipsAlreadyEnabled(t *testing.T) {
	t.Parallel()
	f := &fakeEventSubClient{
		subResult: twitcheventsub.SubscribeResult{StatusCode: http.StatusAccepted},
		existing: []twitcheventsub.EventSubSubscription{
			sub("a", overlayrelay.SubTypeCheer, "111", overlayCB, twitcheventsub.StatusEnabled),
			sub("b", overlayrelay.SubTypeRedemption, "111", overlayCB, twitcheventsub.StatusEnabled),
		},
	}
	reconcileSubscriptions(context.Background(), f, overlayCB, []string{"111"}, quietLogger())
	if len(f.created) != 0 {
		t.Fatalf("nothing should be created when both enabled subs exist, got %v", f.created)
	}
	if len(f.deleted) != 0 {
		t.Fatalf("nothing should be deleted, got %v", f.deleted)
	}
}

func TestReconcileRecreatesDisabled(t *testing.T) {
	t.Parallel()
	// A disabled overlay sub for an active broadcaster is treated as missing.
	f := &fakeEventSubClient{
		subResult: twitcheventsub.SubscribeResult{StatusCode: http.StatusAccepted},
		existing: []twitcheventsub.EventSubSubscription{
			sub("a", overlayrelay.SubTypeCheer, "111", overlayCB, "authorization_revoked"),
			sub("b", overlayrelay.SubTypeRedemption, "111", overlayCB, twitcheventsub.StatusEnabled),
		},
	}
	reconcileSubscriptions(context.Background(), f, overlayCB, []string{"111"}, quietLogger())
	if len(f.created) != 1 || f.created[0] != overlayrelay.SubTypeCheer+"|111" {
		t.Fatalf("expected only the disabled cheer to be recreated, got %v", f.created)
	}
	// The disabled sub is for an active broadcaster, so it is NOT pruned.
	if len(f.deleted) != 0 {
		t.Fatalf("active broadcaster's sub must not be pruned, got %v", f.deleted)
	}
}

func TestReconcilePrunesOrphans(t *testing.T) {
	t.Parallel()
	f := &fakeEventSubClient{
		subResult: twitcheventsub.SubscribeResult{StatusCode: http.StatusAccepted},
		existing: []twitcheventsub.EventSubSubscription{
			// Orphan: broadcaster 999 no longer has a device.
			sub("orphan", overlayrelay.SubTypeCheer, "999", overlayCB, twitcheventsub.StatusEnabled),
			// Enabled sub for an active broadcaster stays.
			sub("keep", overlayrelay.SubTypeCheer, "111", overlayCB, twitcheventsub.StatusEnabled),
		},
	}
	reconcileSubscriptions(context.Background(), f, overlayCB, []string{"111"}, quietLogger())
	if len(f.deleted) != 1 || f.deleted[0] != "orphan" {
		t.Fatalf("expected only the orphan pruned, got %v", f.deleted)
	}
}

func TestReconcileNeverTouchesBotSubs(t *testing.T) {
	t.Parallel()
	// A bot sub (different callback) for a broadcaster with NO active device must
	// never be pruned, and an overlay-type sub on the bot callback likewise.
	botCB := "https://peanutbudderbot.com/eventsub"
	f := &fakeEventSubClient{
		subResult: twitcheventsub.SubscribeResult{StatusCode: http.StatusAccepted},
		existing: []twitcheventsub.EventSubSubscription{
			sub("botstream", "stream.online", "999", botCB, twitcheventsub.StatusEnabled),
			sub("botcheer", overlayrelay.SubTypeCheer, "999", botCB, twitcheventsub.StatusEnabled),
		},
	}
	reconcileSubscriptions(context.Background(), f, overlayCB, []string{"111"}, quietLogger())
	if len(f.deleted) != 0 {
		t.Fatalf("bot subscriptions must never be deleted, got %v", f.deleted)
	}
}
