package twitchbot

import (
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/commands"
)

func TestOverlayArgs(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		text    string
		trigger string
		want    []string
	}{
		{"no args", "!nuke", "!nuke", nil},
		{"one arg", "!b 20", "!b", []string{"20"}},
		{"trigger mid-message", "hey chat !b 20 go", "!b", []string{"20", "go"}},
		{"stops at the next command", "!b 20 !nuke", "!b", []string{"20"}},
		{"case-insensitive match, original casing kept", "!B KevIN", "!b", []string{"KevIN"}},
		{"trigger absent", "nothing here", "!b", nil},
		{"empty trigger", "!b 20", "", nil},
		{"trailing trigger", "go !b", "!b", nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := overlayArgs(tc.text, tc.trigger); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("overlayArgs(%q, %q) = %#v, want %#v", tc.text, tc.trigger, got, tc.want)
			}
		})
	}
}

func TestOverlayArgsBounds(t *testing.T) {
	t.Parallel()

	long := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" // 48 > maxOverlayArgLen
	got := overlayArgs("!b "+long, "!b")
	if len(got) != 1 || len(got[0]) != maxOverlayArgLen {
		t.Fatalf("long arg should be truncated to %d, got %#v", maxOverlayArgLen, got)
	}

	got = overlayArgs("!b 1 2 3 4 5 6 7 8", "!b")
	if len(got) != maxOverlayArgs {
		t.Fatalf("args should cap at %d, got %d", maxOverlayArgs, len(got))
	}
}

func newCapBot() *Bot {
	return &Bot{
		capabilities:    make(map[string]map[string]bool),
		commandRequires: map[string]string{"!b": commands.CapabilityOverlay},
	}
}

func TestCommandAllowedIn(t *testing.T) {
	t.Parallel()
	b := newCapBot()

	// An ungated trigger dispatches even in a channel we know nothing about.
	if !b.commandAllowedIn("somechannel", "!fish") {
		t.Fatal("ungated command should dispatch anywhere")
	}

	// A gated trigger must not dispatch in a channel that was never loaded --
	// this is the case that would otherwise fire !b in every channel the bot
	// sits in.
	if b.commandAllowedIn("somechannel", "!b") {
		t.Fatal("gated command must not dispatch in an unloaded channel")
	}

	// Nor in a loaded channel that holds a different capability.
	b.setCapabilities("somechannel", []string{"something_else"})
	if b.commandAllowedIn("somechannel", "!b") {
		t.Fatal("gated command must not dispatch without its own capability")
	}

	b.setCapabilities("SomeChannel", []string{commands.CapabilityOverlay})
	if !b.commandAllowedIn("somechannel", "!b") {
		t.Fatal("gated command should dispatch once the capability is held (case-insensitively)")
	}

	// Revoking the device between streams takes the commands away again.
	b.dropCapabilities("somechannel")
	if b.commandAllowedIn("somechannel", "!b") {
		t.Fatal("gated command must stop dispatching after the capability is dropped")
	}
}

func TestCreatorChannelAlwaysHoldsOverlay(t *testing.T) {
	t.Parallel()
	b := newCapBot()

	// The creator's channel holds the overlay capability with nothing ever
	// loaded (offline), so its overlay commands answer off-stream just like the
	// ungated builtins do -- the debug channel must not depend on being live.
	if !b.commandAllowedIn(botCreatorChannel, "!b") {
		t.Fatal("creator channel should hold the overlay capability while offline")
	}
	// Case-insensitively, since a broadcaster login may arrive uppercased.
	if !b.channelHas(strings.ToUpper(botCreatorChannel), commands.CapabilityOverlay) {
		t.Fatal("creator-channel overlay grant should be case-insensitive")
	}
	// A stream.offline drop must not take it away: the grant is provisioning,
	// not the loaded set.
	b.dropCapabilities(botCreatorChannel)
	if !b.commandAllowedIn(botCreatorChannel, "!b") {
		t.Fatal("creator channel should keep the overlay capability after a drop")
	}
	// The grant is scoped to overlay: it does not fabricate other capabilities.
	if b.channelHas(botCreatorChannel, "something_else") {
		t.Fatal("creator-channel grant must be limited to the overlay capability")
	}
	// And it does not leak to other channels.
	if b.commandAllowedIn("somechannel", "!b") {
		t.Fatal("only the creator channel gets the always-on overlay grant")
	}
}

func TestSetCapabilitiesEmptyDropsChannel(t *testing.T) {
	t.Parallel()
	b := newCapBot()

	b.setCapabilities("chan", []string{commands.CapabilityOverlay})
	b.setCapabilities("chan", nil)

	if b.channelHas("chan", commands.CapabilityOverlay) {
		t.Fatal("an empty capability set should leave the channel holding nothing")
	}
	if _, present := b.capabilities["chan"]; present {
		t.Fatal("an empty set should drop the channel from the map, not store an empty one")
	}
}

func ptr(s string) *string { return &s }

func TestBuildCommandMaps(t *testing.T) {
	t.Parallel()

	cmds, requires, err := buildCommandMaps([]commands.Commands{
		{Trigger: "!fish", Kind: "builtin", HandlerKey: ptr("fish"), Enabled: true},
		{Trigger: "!b", Kind: "builtin", HandlerKey: ptr("overlay_beans"), Enabled: true, Requires: ptr(commands.CapabilityOverlay)},
		{Trigger: "!off", Kind: "builtin", HandlerKey: ptr("fish"), Enabled: false},
		{Trigger: "!custom", Kind: "custom", ResponseTmpl: ptr("hi"), Enabled: true},
	})
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if len(cmds) != 2 {
		t.Fatalf("expected 2 dispatchable commands, got %d", len(cmds))
	}
	if _, ok := cmds["!off"]; ok {
		t.Fatal("disabled rows must not reach the dispatch map")
	}
	if _, ok := cmds["!custom"]; ok {
		t.Fatal("custom rows have no compiled handler and must not reach the dispatch map")
	}
	if requires["!b"] != commands.CapabilityOverlay {
		t.Fatalf("!b should be gated on %q, got %q", commands.CapabilityOverlay, requires["!b"])
	}
	if _, gated := requires["!fish"]; gated {
		t.Fatal("a row with a null requires must be unconditional")
	}
}

func TestBuildCommandMapsRejectsBadRows(t *testing.T) {
	t.Parallel()

	// A trigger stored with upper case can never match: the message is
	// lowercased before lookup but the map is keyed on the column verbatim.
	// This is the trap that makes '!zeroG' silently dead, so it must fail loud.
	if _, _, err := buildCommandMaps([]commands.Commands{
		{Trigger: "!zeroG", Kind: "builtin", HandlerKey: ptr("overlay_zero_g"), Enabled: true},
	}); err == nil {
		t.Fatal("a mixed-case trigger should fail at boot")
	}

	if _, _, err := buildCommandMaps([]commands.Commands{
		{Trigger: "!nope", Kind: "builtin", HandlerKey: ptr("no_such_handler"), Enabled: true},
	}); err == nil {
		t.Fatal("an unknown handler_key should fail at boot")
	}

	if _, _, err := buildCommandMaps([]commands.Commands{
		{Trigger: "!nope", Kind: "builtin", Enabled: true},
	}); err == nil {
		t.Fatal("a null handler_key on a builtin should fail at boot")
	}
}

// TestOverlayHandlersMatchMigration reads the migration itself rather than a
// hardcoded list, so the two cannot drift. Every handler_key the SQL seeds must
// exist in builtinRegistry -- an unknown key fails the bot at boot the moment
// that migration is applied.
func TestOverlayHandlersMatchMigration(t *testing.T) {
	t.Parallel()

	const path = "../../../database/prod-apply-overlay-commands.sql"
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %s", path, err)
	}

	// Only the live INSERT matters; commented-out lines are not applied.
	var live []string
	for _, line := range strings.Split(string(raw), "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "--") {
			live = append(live, line)
		}
	}
	sql := strings.Join(live, "\n")

	rowRe := regexp.MustCompile(`\('(![a-z0-9_]+)',\s*'builtin',\s*'(overlay_[a-z_]+)'`)
	matches := rowRe.FindAllStringSubmatch(sql, -1)
	if len(matches) == 0 {
		t.Fatal("no overlay command rows found in the migration; did its shape change?")
	}

	for _, m := range matches {
		trigger, key := m[1], m[2]
		if _, ok := builtinRegistry[key]; !ok {
			t.Errorf("migration seeds handler_key %q for %s, but builtinRegistry has no such key", key, trigger)
		}
		if trigger != strings.ToLower(trigger) {
			t.Errorf("migration seeds mixed-case trigger %q, which can never match", trigger)
		}
	}

	t.Logf("checked %d seeded overlay rows", len(matches))
}
