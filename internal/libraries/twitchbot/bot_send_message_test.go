package twitchbot

import (
	"context"
	"testing"
)

func TestHelixUserTokenStripsOAuthPrefix(t *testing.T) {
	t.Parallel()
	b := &Bot{config: Config{BotOAuth: "oauth:abc"}}
	tok, err := b.helixUserToken(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok != "abc" {
		t.Fatalf("got %q, want abc", tok)
	}
}

func TestHelixUserTokenUsesProvider(t *testing.T) {
	t.Parallel()
	b := &Bot{
		config: Config{BotOAuth: "oauth:static"},
		tokenProvider: func(context.Context) (string, error) {
			return "fresh", nil
		},
	}
	tok, err := b.helixUserToken(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok != "fresh" {
		t.Fatalf("got %q, want fresh", tok)
	}
}
