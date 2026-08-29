package overlayrelay

import "testing"

func TestMintTokenIsUniqueAndSelfConsistent(t *testing.T) {
	seen := make(map[string]struct{}, 64)
	for i := 0; i < 64; i++ {
		token, hash, err := MintToken()
		if err != nil {
			t.Fatalf("MintToken: %v", err)
		}
		if !LooksLikeToken(token) {
			t.Fatalf("minted token failed its own shape check: %q", token)
		}
		if got := HashToken(token); got != hash {
			t.Fatalf("hash mismatch: MintToken said %q, HashToken said %q", hash, got)
		}
		if _, dup := seen[token]; dup {
			t.Fatalf("MintToken repeated a token after %d draws", i)
		}
		seen[token] = struct{}{}
	}
}

func TestHashTokenDoesNotLeakPlaintext(t *testing.T) {
	token, hash, err := MintToken()
	if err != nil {
		t.Fatalf("MintToken: %v", err)
	}
	if len(hash) != 64 {
		t.Fatalf("expected 64 hex chars, got %d", len(hash))
	}
	if hash == token {
		t.Fatal("hash equals plaintext")
	}
}

func TestLooksLikeTokenRejectsJunk(t *testing.T) {
	for _, bad := range []string{
		"",
		"hunter2",
		"rsov_",
		"rsov_short",
		"rsov_!!!!not-base64!!!!",
		"wrongprefix_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
	} {
		if LooksLikeToken(bad) {
			t.Errorf("LooksLikeToken(%q) = true, want false", bad)
		}
	}
}
