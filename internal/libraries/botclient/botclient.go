// Package botclient is core-api's signed HTTP client to the twitch-bot's own
// listener. It is the reverse of internal/libraries/api.CoreAPIClient: instead
// of the bot calling core-api for state, core-api calls the bot to push
// realtime commands (e.g. "join this channel now"). Requests are signed with
// BOT_API_HMAC_KEY using the exact canonical string the bot's
// nivekmiddleware.NewHMACMiddleware verifies:
//
//	<METHOD>\n<PATH>\n<RAW_QUERY>\n<TIMESTAMP>\n<BODY>
package botclient

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/api"
)

// Client posts HMAC-signed commands to the bot's HTTP listener.
type Client struct {
	baseURL    string
	hmacKey    []byte
	httpClient *http.Client
}

// NewClient builds a Client. baseURL is the bot listener root (e.g.
// http://172.19.0.1:8090); hmacKeyHex is BOT_API_HMAC_KEY, hex-encoded.
func NewClient(baseURL, hmacKeyHex string) (*Client, error) {
	if baseURL == "" {
		return nil, fmt.Errorf("bot base URL is empty")
	}
	key, err := hex.DecodeString(hmacKeyHex)
	if err != nil {
		return nil, fmt.Errorf("BOT_API_HMAC_KEY is not valid hex: %w", err)
	}
	if len(key) < 16 {
		// Mirrors the sanity floor in api.CoreAPIClient — catches obvious typos.
		return nil, fmt.Errorf("BOT_API_HMAC_KEY too short (%d bytes)", len(key))
	}
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		hmacKey:    key,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}, nil
}

// do signs and sends a request. path is the bot route exactly as registered
// (e.g. "/internal/join"); the bot's Echo mounts routes at root, so — unlike
// api.CoreAPIClient — there is no "/api" prefix in the signed path.
func (c *Client) do(method, path string, body []byte) error {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	canonical := fmt.Sprintf("%s\n%s\n%s\n%s\n%s", method, path, "", ts, body)
	mac := hmac.New(sha256.New, c.hmacKey)
	mac.Write([]byte(canonical))
	sig := hex.EncodeToString(mac.Sum(nil))

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, c.baseURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("X-Nivek-Timestamp", ts)
	req.Header.Set("X-Nivek-HMAC", sig)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("bot %s %s: status %d: %s", method, path, resp.StatusCode, respBody)
	}
	return nil
}

// JoinChannel tells the bot to join a broadcaster's chat immediately.
func (c *Client) JoinChannel(broadcasterUserLogin string) error {
	body, err := json.Marshal(map[string]string{"twitch_login": broadcasterUserLogin})
	if err != nil {
		return fmt.Errorf("marshal join request: %w", err)
	}
	return c.do(http.MethodPost, api.PostBotJoinChannel, body)
}

// StopBother tells the bot to end a legacy user's hourly "please authenticate"
// nag loop — called when that user authenticates. No-op on the bot if no such
// loop is running.
func (c *Client) StopBother(broadcasterUserLogin string) error {
	body, err := json.Marshal(map[string]string{"twitch_login": broadcasterUserLogin})
	if err != nil {
		return fmt.Errorf("marshal stop-bother request: %w", err)
	}
	return c.do(http.MethodPost, api.PostBotStopBother, body)
}
