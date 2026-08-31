package twitchext

import (
	"encoding/base64"
	"testing"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v5"
)

// rawSecret is the key bytes; secretB64 is what the developer console shows.
var rawSecret = []byte("this-is-a-thirty-two-byte-secret")

func secretB64() string { return base64.StdEncoding.EncodeToString(rawSecret) }

func signIdentity(t *testing.T, secret []byte, channelID string, exp time.Time) string {
	t.Helper()
	claims := IdentityClaims{
		ChannelID:    channelID,
		UserID:       "150996923",
		OpaqueUserID: "U-150996923",
		Role:         "broadcaster",
		RegisteredClaims: jwtlib.RegisteredClaims{
			ExpiresAt: jwtlib.NewNumericDate(exp),
		},
	}
	tok, err := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, claims).SignedString(secret)
	if err != nil {
		t.Fatalf("sign identity: %v", err)
	}
	return tok
}

func signReceipt(t *testing.T, secret []byte, topic, domainID, sku string, bits int, txID string, exp time.Time) string {
	t.Helper()
	claims := ReceiptClaims{
		Topic: topic,
		RegisteredClaims: jwtlib.RegisteredClaims{
			ExpiresAt: jwtlib.NewNumericDate(exp),
		},
	}
	claims.Data.TransactionID = txID
	claims.Data.UserID = "150996923"
	claims.Data.DomainID = domainID
	claims.Data.Product.SKU = sku
	claims.Data.Product.DisplayName = "Fake Beans"
	claims.Data.Product.Cost.Amount = bits
	claims.Data.Product.Cost.Type = "bits"
	tok, err := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, claims).SignedString(secret)
	if err != nil {
		t.Fatalf("sign receipt: %v", err)
	}
	return tok
}

func TestDecodeSecret(t *testing.T) {
	got, err := DecodeSecret(secretB64())
	if err != nil {
		t.Fatalf("DecodeSecret: %v", err)
	}
	if string(got) != string(rawSecret) {
		t.Fatalf("decoded secret mismatch")
	}
	if _, err := DecodeSecret("not base64 !!!"); err == nil {
		t.Fatal("accepted non-base64 secret")
	}
}

func TestVerifyIdentity(t *testing.T) {
	future := time.Now().Add(time.Hour)
	tok := signIdentity(t, rawSecret, "150996923", future)

	claims, err := VerifyIdentity(tok, rawSecret)
	if err != nil {
		t.Fatalf("VerifyIdentity: %v", err)
	}
	if claims.ChannelID != "150996923" {
		t.Fatalf("channel = %q", claims.ChannelID)
	}

	// Wrong secret is rejected.
	if _, err := VerifyIdentity(tok, []byte("a-completely-different-secret-32b")); err == nil {
		t.Fatal("accepted a token signed with a different secret")
	}
	// Expired is rejected.
	expired := signIdentity(t, rawSecret, "150996923", time.Now().Add(-time.Hour))
	if _, err := VerifyIdentity(expired, rawSecret); err == nil {
		t.Fatal("accepted an expired identity token")
	}
	// Missing channel is rejected.
	noChannel := signIdentity(t, rawSecret, "", time.Now().Add(time.Hour))
	if _, err := VerifyIdentity(noChannel, rawSecret); err == nil {
		t.Fatal("accepted an identity token with no channel_id")
	}
}

func TestVerifyReceipt(t *testing.T) {
	future := time.Now().Add(time.Hour)
	clientID := "abcdef123456"
	domain := "twitch.ext." + clientID
	tok := signReceipt(t, rawSecret, "bits_transaction_receipt", domain, "test_beans", 30, "tx-1", future)

	claims, err := VerifyReceipt(tok, rawSecret, clientID)
	if err != nil {
		t.Fatalf("VerifyReceipt: %v", err)
	}
	if claims.Data.Product.SKU != "test_beans" || claims.Data.Product.Cost.Amount != 30 || claims.Data.TransactionID != "tx-1" {
		t.Fatalf("unexpected claims: %+v", claims.Data)
	}

	// Wrong secret.
	if _, err := VerifyReceipt(tok, []byte("a-completely-different-secret-32b"), clientID); err == nil {
		t.Fatal("accepted a receipt signed with a different secret")
	}
	// Wrong topic.
	badTopic := signReceipt(t, rawSecret, "not_a_receipt", domain, "test_beans", 30, "tx-2", future)
	if _, err := VerifyReceipt(badTopic, rawSecret, clientID); err == nil {
		t.Fatal("accepted a receipt with the wrong topic")
	}
	// Receipt for a different extension.
	otherDomain := signReceipt(t, rawSecret, "bits_transaction_receipt", "twitch.ext.someoneelse", "test_beans", 30, "tx-3", future)
	if _, err := VerifyReceipt(otherDomain, rawSecret, clientID); err == nil {
		t.Fatal("accepted a receipt for a different extension")
	}
	// Empty sku.
	noSKU := signReceipt(t, rawSecret, "bits_transaction_receipt", domain, "", 30, "tx-4", future)
	if _, err := VerifyReceipt(noSKU, rawSecret, clientID); err == nil {
		t.Fatal("accepted a receipt with no sku")
	}
	// Expired.
	expired := signReceipt(t, rawSecret, "bits_transaction_receipt", domain, "test_beans", 30, "tx-5", time.Now().Add(-time.Hour))
	if _, err := VerifyReceipt(expired, rawSecret, clientID); err == nil {
		t.Fatal("accepted an expired receipt")
	}
}
