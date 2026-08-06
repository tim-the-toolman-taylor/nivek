package api

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/user"
)

// CoreAPIClient is the bot's only path to persistent state. Every call signs
// the request body + method + path + timestamp with HMAC-SHA256 and includes:
//
//	X-Nivek-Timestamp: unix seconds
//	X-Nivek-HMAC:      hex(SHA256(key, <METHOD>\n<PATH>\n<QUERY>\n<TS>\n<BODY>))
//
// Matches the canonical-string format enforced by
// nivekmiddleware.NewHMACMiddleware on core-api.
type CoreAPIClient struct {
	baseURL    string
	hmacKey    []byte
	httpClient *http.Client
}

func NewCoreAPIClient(baseURL, hmacKeyHex string) (*CoreAPIClient, error) {
	if baseURL == "" {
		return nil, fmt.Errorf("CORE_API_URL is empty")
	}
	key, err := hex.DecodeString(hmacKeyHex)
	if err != nil {
		return nil, fmt.Errorf("BOT_API_HMAC_KEY is not valid hex: %w", err)
	}
	if len(key) < 16 {
		// Server enforces 32-byte minimum via deploy convention; 16 here is a
		// sanity floor that catches obvious typos like a 2-char key.
		return nil, fmt.Errorf("BOT_API_HMAC_KEY too short (%d bytes)", len(key))
	}
	return &CoreAPIClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		hmacKey:    key,
		httpClient: &http.Client{Timeout: 10 * time.Second, Transport: ipv4OnlyTransport()},
	}, nil
}

// ipv4OnlyTransport returns an http.Transport whose dialer forces "tcp4"
// (A-record-only resolution), bypassing Go's pure-Go resolver's behavior of
// firing AAAA + A in parallel and waiting for both. On the Pi-hosted bot,
// AAAA lookups for peanutbudderbot.com via the home router (192.168.1.1) hang ~10s
// before falling back to A, which fired the http.Client.Timeout *before* the
// request was ever sent — the surfaced error ("Client.Timeout exceeded while
// awaiting headers") masked the real DNS-level cause and consumed an entire
// debug session in 2026-06.
//
// We don't actually want IPv6 anywhere in the bot's outbound path: peanutbudderbot.com
// only has an A record (no AAAA), the Pi's residential network is v4-only,
// and Twitch IRC is reached via the chat client, not this transport. Forcing
// tcp4 has no behavioral cost for the bot and makes the binary robust against
// the resolver pathology regardless of build flags (CGO on/off) or env state
// (no GODEBUG=netdns=cgo dependency).
func ipv4OnlyTransport() *http.Transport {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		if network == "tcp" || network == "tcp6" {
			network = "tcp4"
		}
		return dialer.DialContext(ctx, network, addr)
	}
	return transport
}

// do executes a signed request and decodes the JSON response into `out`.
// Path is the route under /api (e.g. "/bot/channels"); query is the raw
// query string without leading "?". Body may be nil for GETs.
func (c *CoreAPIClient) do(method, path, rawQuery string, body []byte, out any) error {
	full := c.baseURL + "/api" + path
	if rawQuery != "" {
		full += "?" + rawQuery
	}

	ts := strconv.FormatInt(time.Now().Unix(), 10)
	canonical := fmt.Sprintf("%s\n/api%s\n%s\n%s\n%s", method, path, rawQuery, ts, body)
	mac := hmac.New(sha256.New, c.hmacKey)
	mac.Write([]byte(canonical))
	sig := hex.EncodeToString(mac.Sum(nil))

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, full, bodyReader)
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
		return fmt.Errorf("core-api %s %s: status %d: %s", method, path, resp.StatusCode, respBody)
	}
	if out != nil {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}

func (c *CoreAPIClient) CreateNewUser(newUser *user.User) error {
	body, err := json.Marshal(newUser)
	if err != nil {
		return fmt.Errorf("failed to marshal CreateNewUser request body for broadcaster %s - %s", *newUser.TwitchLogin, err.Error())
	}

	if err := c.do(http.MethodPut, PutCreateNewUser, "", body, nil); err != nil {
		return fmt.Errorf("CreateNewUser request failed for broadcaster %d - %s", newUser.TwitchLogin, err.Error())
	}

	return nil
}

func (c *CoreAPIClient) HealLegacyUser(user *user.User) error {
	body, err := json.Marshal(user)
	if err != nil {
		return fmt.Errorf("failed to marshal HealLegacyUser user object: %+v - %s", user, err.Error())
	}

	if err := c.do(http.MethodPost, PostHealLegacyUser, "", body, nil); err != nil {
		return fmt.Errorf("heal legacy user request failed for user: %+v - %s", user, err.Error())
	}

	return nil
}

func (c *CoreAPIClient) PushState(broadcasterUserLogin *string, isLive bool) error {
	var request struct {
		BroadcasterUserLogin *string `json:"twitch_login"`
		IsLive               bool    `json:"is_live"`
	}

	request.BroadcasterUserLogin = broadcasterUserLogin
	request.IsLive = isLive

	body, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal PushState request body for broadcaster %s state %v - %s",
			*broadcasterUserLogin,
			isLive,
			err.Error(),
		)
	}

	if err := c.do(http.MethodPut, PutBroadcasterState, "", body, nil); err != nil {
		return err
	}

	return nil
}

func (c *CoreAPIClient) GetActiveChannels() ([]user.User, error) {
	var resp struct {
		Channels []user.User `json:"channels"`
	}
	if err := c.do(http.MethodGet, GetActiveChannels, "", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Channels, nil
}

func (c *CoreAPIClient) IncrementBread(channel, chatter string) (int, error) {
	body, _ := json.Marshal(map[string]string{"channel": channel, "chatter": chatter})
	var resp struct {
		Count int `json:"count"`
	}
	if err := c.do(http.MethodPost, "/bot/bread/increment", "", body, &resp); err != nil {
		return 0, err
	}
	return resp.Count, nil
}

func (c *CoreAPIClient) GetBreadTotal(channel string) (int, error) {
	q := url.Values{}
	q.Set("channel", channel)
	var resp struct {
		Total int `json:"total"`
	}
	if err := c.do(http.MethodGet, "/bot/bread/total", q.Encode(), nil, &resp); err != nil {
		return 0, err
	}
	return resp.Total, nil
}

func (c *CoreAPIClient) LurkOnMessage(channel, chatter string) int {
	body, _ := json.Marshal(map[string]string{"channel": channel, "chatter": chatter})
	var resp struct {
		Count int `json:"count"`
	}
	if err := c.do(http.MethodPost, "/bot/lurk/message", "", body, &resp); err != nil {
		// Mirror lurk.OnMessage's swallow-and-return-0 behavior so the
		// caller's `count > 0` gate keeps working untouched.
		return 0
	}
	return resp.Count
}

func (c *CoreAPIClient) GoFishing(channel, chatter string) (string, error) {
	body, _ := json.Marshal(map[string]string{"channel": channel, "chatter": chatter})
	var resp struct {
		Message string `json:"message"`
	}
	if err := c.do(http.MethodPost, "/bot/fish/go", "", body, &resp); err != nil {
		return "", err
	}
	return resp.Message, nil
}
