// Command twitch-bot-auth is a one-time, run-it-locally helper that mints a
// Twitch user access token + refresh token for the bot's chat account, bound to
// YOUR OWN registered application (TWITCH_CLIENT_ID / TWITCH_CLIENT_SECRET).
//
// Why this exists: the bot auto-refreshes its IRC token (internal/libraries/
// twitchauth), but that only works when the refresh token was issued by the same
// app as the client ID/secret the bot uses. Tokens from third-party generators
// are bound to THEIR app and get rejected with "Invalid refresh token". Run this
// once to produce a compatible pair, then paste the values into the bot's .env.
//
// Prerequisites (one-time):
//  1. In dev.twitch.tv/console, on the app that owns TWITCH_CLIENT_ID, add
//     an OAuth Redirect URL of exactly:  http://localhost:3000
//  2. Log into twitch.tv in your browser AS THE BOT ACCOUNT (peanutbudderbot).
//
// Usage:
//
//	TWITCH_CLIENT_ID=xxx TWITCH_CLIENT_SECRET=yyy go run ./cmd/twitch-bot-auth
//
// It prints an authorize URL, waits for the browser redirect on :3000, exchanges
// the code, and prints the .env lines to set.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	authorizeURL = "https://id.twitch.tv/oauth2/authorize"
	tokenURL     = "https://id.twitch.tv/oauth2/token"
	// chat:read + chat:edit are all the IRC bot needs. Keep in sync with the
	// scopes the running bot expects.
	scopes = "chat:read chat:edit"
)

func main() {
	port := flag.String("port", "3000", "local port for the OAuth redirect (must match the app's registered http://localhost:<port>)")
	clientID := flag.String("client-id", os.Getenv("TWITCH_CLIENT_ID"), "Twitch app client ID (defaults to $TWITCH_CLIENT_ID)")
	clientSecret := flag.String("client-secret", os.Getenv("TWITCH_CLIENT_SECRET"), "Twitch app client secret (defaults to $TWITCH_CLIENT_SECRET)")
	flag.Parse()

	if *clientID == "" || *clientSecret == "" {
		log.Fatal("set TWITCH_CLIENT_ID and TWITCH_CLIENT_SECRET (env or flags)")
	}

	redirectURI := fmt.Sprintf("http://localhost:%s", *port)

	authURL := authorizeURL + "?" + url.Values{
		"client_id":     {*clientID},
		"redirect_uri":  {redirectURI},
		"response_type": {"code"},
		"scope":         {scopes},
		"force_verify":  {"true"}, // always show consent so you can confirm the account
	}.Encode()

	fmt.Println("── Twitch bot token minter ──")
	fmt.Println()
	fmt.Println("1. Make sure your browser is logged in AS THE BOT ACCOUNT (peanutbudderbot).")
	fmt.Printf("2. Confirm the app has %s registered as an OAuth Redirect URL.\n", redirectURI)
	fmt.Println("3. Open this URL and click Authorize:")
	fmt.Println()
	fmt.Println("   " + authURL)
	fmt.Println()
	fmt.Printf("Waiting for the redirect on %s ...\n", redirectURI)

	codeCh := make(chan string, 1)
	errCh := make(chan string, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if e := q.Get("error"); e != "" {
			msg := e + ": " + q.Get("error_description")
			http.Error(w, msg, http.StatusBadRequest)
			errCh <- msg
			return
		}
		code := q.Get("code")
		if code == "" {
			return // ignore favicon etc.
		}
		io.WriteString(w, "Got it — you can close this tab and return to the terminal.")
		codeCh <- code
	})

	srv := &http.Server{Addr: ":" + *port, Handler: mux}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("local server: %v", err)
		}
	}()

	var code string
	select {
	case code = <-codeCh:
	case msg := <-errCh:
		log.Fatalf("authorization denied/failed: %s", msg)
	case <-time.After(5 * time.Minute):
		log.Fatal("timed out waiting for authorization")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)

	form := url.Values{
		"client_id":     {*clientID},
		"client_secret": {*clientSecret},
		"code":          {code},
		"grant_type":    {"authorization_code"},
		"redirect_uri":  {redirectURI},
	}
	req, err := http.NewRequest(http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		log.Fatalf("building token request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		log.Fatalf("token request: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		log.Fatalf("token endpoint returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var parsed struct {
		AccessToken  string   `json:"access_token"`
		RefreshToken string   `json:"refresh_token"`
		ExpiresIn    int      `json:"expires_in"`
		Scope        []string `json:"scope"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		log.Fatalf("decoding token response: %v", err)
	}

	fmt.Println()
	fmt.Println("✅ Success. Put these in the bot's MASTER .env (/home/deploy/actions-runner/.env):")
	fmt.Println()
	fmt.Printf("TWITCH_BOT_OAUTH=oauth:%s\n", parsed.AccessToken)
	fmt.Printf("TWITCH_BOT_REFRESH=%s\n", parsed.RefreshToken)
	fmt.Println()
	fmt.Printf("(scopes: %s | access token expires in ~%dm — the bot will auto-refresh from here)\n",
		strings.Join(parsed.Scope, " "), parsed.ExpiresIn/60)
}
