package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Twitch identifies its own Android app by this public client ID; requests made as that app
// are answered without the browser "integrity" challenge the website client gets.
const (
	clientID  = "kd1unb4b3q4t58fwlpcbzcbnm76a8fp"
	userAgent = "Dalvik/2.1.0 (Linux; U; Android 16; SM-S911B Build/TP1A.220624.014) tv.twitch.android.app/31.8.0/3108006"
)

// Endpoints can be redirected to a local mock in tests (DROPOMAT_ENDPOINTS).
type Endpoints struct {
	GQL    string `json:"gql"`
	ID     string `json:"id"`
	WWW    string `json:"www"`
	Usher  string `json:"usher"`
	IRC    string `json:"irc"` // host:port, TLS unless it starts with "plain:"
	Kick   string `json:"kick"`
	KickWS string `json:"kickws"`
}

func defaultEndpoints() Endpoints {
	ep := Endpoints{
		GQL:    "https://gql.twitch.tv/gql",
		ID:     "https://id.twitch.tv",
		WWW:    "https://www.twitch.tv",
		Usher:  "https://usher.ttvnw.net",
		IRC:    "irc.chat.twitch.tv:6697",
		Kick:   "https://kick.com",
		KickWS: "https://websockets.kick.com",
	}
	if raw := os.Getenv("DROPOMAT_ENDPOINTS"); raw != "" {
		json.Unmarshal([]byte(raw), &ep)
	}
	return ep
}

// Persisted GraphQL operations: name -> sha256 of the query Twitch has stored under that name.
// Twitch changes these from time to time; data/hashes.json (same shape) overrides them without a new build.
var gqlHashes = map[string]string{
	"VideoPlayerStreamInfoOverlayChannel":  "198492e0857f6aedead9665c81c5a06d67b25b58034649687124083ff288597d",
	"ClaimCommunityPoints":                 "46aaeebe02c99afdf4fc97c7c0cba964124bf6b0af229395f1f6d1feed05b3d0",
	"DropsPage_ClaimDropRewards":           "a455deea71bdc9015b78eb49f4acfbce8baa7ccbedd28e549bb025bd0f751930",
	"ChannelPointsContext":                 "374314de591e69925fce3ddc2bcf085796f56ebb8cad67a0daa3165c03adc345",
	"Inventory":                            "8337eb8541b314040b0edde0c09c5c7a2783ba1960aa9edfbf3bac16d0fec404",
	"DropCurrentSessionContext":            "4d06b702d25d652afb9ef835d2a550031f1cf762b193523a92166f40ea3d142b",
	"ViewerDropsDashboard":                 "c16bb890cc8ce7647a96ee69cd313d423a378a3dedadf630a1017cde18975feb",
	"DropCampaignDetails":                  "039277bf98f3130929262cc7c6efd9c141ca3749cb6dca442fc8ead9a53f77c1",
	"DropsHighlightService_AvailableDrops": "782dad0f032942260171d2d80a654f88bdd0c5a9dddc392e9bc92218a0f42d20",
	"PlaybackAccessToken":                  "ed230aa1e33e07eebb8928504583da78a5173989fadfb1ac94be06a04f3cdbe9",
	"DirectoryPage_Game":                   "86bcceb4e8b1a51256ff8eed8bd8aae4acacf80d737efe904f84f3aeadf8cafd",
	"DirectoryGameRedirect":                "1f0300090caceec51f33c5e20647aceff9017f740f223c3c532ba6fa59f6b6cc",
}

type gqlOp struct {
	Name string
	Vars map[string]any
}

var errAuth = errors.New("twitch: not authorized")
var errGone = errors.New("stream is not live")
var errWrongClient = errors.New("token belongs to another client")

type Client struct {
	http *http.Client
	ep   Endpoints
	log  *Logger

	mu        sync.Mutex
	token     string
	userID    string
	login     string
	deviceID  string
	sessionID string
}

func newClient(log *Logger) *Client {
	return &Client{
		http:      &http.Client{Timeout: 20 * time.Second},
		ep:        defaultEndpoints(),
		log:       log,
		sessionID: randHex(16),
	}
}

func (c *Client) setAuth(token, userID, login, deviceID string) {
	c.mu.Lock()
	c.token, c.userID, c.login = token, userID, login
	if deviceID != "" {
		c.deviceID = deviceID
	}
	c.mu.Unlock()
}

func (c *Client) auth() (token, userID, login, deviceID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.token, c.userID, c.login, c.deviceID
}

func (c *Client) baseHeaders(req *http.Request) {
	_, _, _, dev := c.auth()
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-US")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Client-Id", clientID)
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Client-Session-Id", c.sessionID)
	if dev != "" {
		req.Header.Set("X-Device-Id", dev)
	}
}

// ensureDevice obtains the device id Twitch hands out in the "unique_id" cookie (random one as a fallback).
func (c *Client) ensureDevice() string {
	if _, _, _, dev := c.auth(); dev != "" {
		return dev
	}
	dev := ""
	if req, err := http.NewRequest("GET", c.ep.WWW, nil); err == nil {
		c.baseHeaders(req)
		if res, err := c.http.Do(req); err == nil {
			for _, ck := range res.Cookies() {
				if ck.Name == "unique_id" && ck.Value != "" {
					dev = ck.Value
				}
			}
			io.Copy(io.Discard, io.LimitReader(res.Body, 1<<20))
			res.Body.Close()
		}
	}
	if dev == "" {
		dev = randHex(32)
	}
	c.mu.Lock()
	c.deviceID = dev
	c.mu.Unlock()
	return dev
}

// validate asks Twitch who the token belongs to. ok=false with nil error means the token is no longer valid.
func (c *Client) validate(token string) (userID, login string, ok bool, err error) {
	req, _ := http.NewRequest("GET", c.ep.ID+"/oauth2/validate", nil)
	req.Header.Set("Authorization", "OAuth "+token)
	res, err := c.http.Do(req)
	if err != nil {
		return "", "", false, err
	}
	defer res.Body.Close()
	if res.StatusCode == 401 {
		return "", "", false, nil
	}
	if res.StatusCode != 200 {
		return "", "", false, fmt.Errorf("validate: HTTP %d", res.StatusCode)
	}
	var v struct {
		ClientID string `json:"client_id"`
		Login    string `json:"login"`
		UserID   string `json:"user_id"`
	}
	if err := json.NewDecoder(res.Body).Decode(&v); err != nil {
		return "", "", false, err
	}
	if v.UserID == "" {
		return "", "", false, nil
	}
	if v.ClientID != "" && v.ClientID != clientID {
		return "", "", false, errWrongClient // a session of a different Twitch client cannot be used here
	}
	return v.UserID, v.Login, true, nil
}

// gql sends one or more persisted operations in a single request and returns each operation's "data".
// Entries are nil when that operation failed; err reports transport/auth failures or the first operation error.
func (c *Client) gql(ops ...gqlOp) ([]map[string]any, error) {
	token, _, _, _ := c.auth()
	if token == "" {
		return nil, errAuth
	}
	body := make([]map[string]any, len(ops))
	for i, op := range ops {
		hash := gqlHashes[op.Name]
		if hash == "" {
			return nil, fmt.Errorf("gql: unknown operation %s", op.Name)
		}
		vars := op.Vars
		if vars == nil {
			vars = map[string]any{}
		}
		body[i] = map[string]any{
			"operationName": op.Name,
			"variables":     vars,
			"extensions":    map[string]any{"persistedQuery": map[string]any{"version": 1, "sha256Hash": hash}},
		}
	}
	payload, _ := json.Marshal(body)
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt*3) * time.Second)
		}
		req, _ := http.NewRequest("POST", c.ep.GQL, bytes.NewReader(payload))
		c.baseHeaders(req)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", "https://www.twitch.tv")
		req.Header.Set("Referer", "https://www.twitch.tv")
		req.Header.Set("Authorization", "OAuth "+token)
		res, err := c.http.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		raw, _ := io.ReadAll(io.LimitReader(res.Body, 32<<20))
		res.Body.Close()
		if res.StatusCode == 401 {
			return nil, errAuth
		}
		if res.StatusCode >= 500 {
			lastErr = fmt.Errorf("gql: HTTP %d", res.StatusCode)
			continue
		}
		var arr []map[string]any
		if err := json.Unmarshal(raw, &arr); err != nil {
			// a single object means a request-level error, e.g. {"error":"Unauthorized","status":401,"message":"..."}
			var one map[string]any
			if json.Unmarshal(raw, &one) == nil {
				if num(one["status"]) == 401 {
					return nil, errAuth
				}
				lastErr = fmt.Errorf("gql: %s %s", str(one["error"]), str(one["message"]))
			} else {
				lastErr = fmt.Errorf("gql: HTTP %d, unreadable answer", res.StatusCode)
			}
			continue
		}
		out := make([]map[string]any, len(ops))
		var opErr error
		retry := false
		for i := range ops {
			if i >= len(arr) {
				break
			}
			r := arr[i]
			if data, ok := r["data"].(map[string]any); ok {
				out[i] = data
			}
			for _, e := range list(r["errors"]) {
				msg := str(dig(e, "message"))
				switch msg {
				case "service error", "PersistedQueryNotFound", "service timeout", "request cancelled", "service unavailable", "context deadline exceeded":
					retry = true
				case "server error":
					// partial data; keep what came back
				default:
					if opErr == nil {
						opErr = fmt.Errorf("gql %s: %s", ops[i].Name, msg)
					}
				}
				if opErr == nil && retry {
					lastErr = fmt.Errorf("gql %s: %s", ops[i].Name, msg)
				}
			}
		}
		if retry && attempt < 2 {
			continue
		}
		if opErr == nil && retry {
			opErr = lastErr
		}
		return out, opErr
	}
	return nil, lastErr
}

// gqlQuery sends one plain (not persisted) query. Used for the searches behind the suggestions,
// which have no persisted operation in this client.
func (c *Client) gqlQuery(query string, vars map[string]any) (map[string]any, error) {
	token, _, _, _ := c.auth()
	if token == "" {
		return nil, errAuth
	}
	data, err := c.gqlQueryAs(query, vars, token)
	if err != nil && !errors.Is(err, errAuth) {
		data, err = c.gqlQueryAs(query, vars, "") // these searches are public: try once more without the session
	}
	return data, err
}

func (c *Client) gqlQueryAs(query string, vars map[string]any, token string) (map[string]any, error) {
	payload, _ := json.Marshal(map[string]any{"query": query, "variables": vars})
	req, _ := http.NewRequest("POST", c.ep.GQL, bytes.NewReader(payload))
	c.baseHeaders(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://www.twitch.tv")
	req.Header.Set("Referer", "https://www.twitch.tv")
	if token != "" {
		req.Header.Set("Authorization", "OAuth "+token)
	}
	hc := &http.Client{Timeout: 8 * time.Second}
	res, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if res.StatusCode == 401 {
		return nil, errAuth
	}
	var one map[string]any
	if json.Unmarshal(raw, &one) != nil {
		var arr []map[string]any // some gateways answer with a one-element list
		if json.Unmarshal(raw, &arr) != nil || len(arr) == 0 {
			return nil, fmt.Errorf("gql query: HTTP %d, unreadable answer", res.StatusCode)
		}
		one = arr[0]
	}
	data, _ := one["data"].(map[string]any)
	if data == nil {
		msg := str(one["message"])
		if errs := list(one["errors"]); len(errs) > 0 {
			msg = str(dig(errs[0], "message"))
		}
		return nil, fmt.Errorf("gql query: HTTP %d %s", res.StatusCode, msg)
	}
	return data, nil
}

// gqlOne is gql for a single operation.
func (c *Client) gqlOne(name string, vars map[string]any) (map[string]any, error) {
	out, err := c.gql(gqlOp{name, vars})
	if len(out) == 1 && out[0] != nil {
		return out[0], err
	}
	if err == nil {
		err = fmt.Errorf("gql %s: empty answer", name)
	}
	return nil, err
}

// gqlBatch runs many operations, 20 per request, and never fails as a whole: failed entries are nil.
func (c *Client) gqlBatch(ops []gqlOp) ([]map[string]any, error) {
	out := make([]map[string]any, 0, len(ops))
	var firstErr error
	for i := 0; i < len(ops); i += 20 {
		part := ops[i:min(i+20, len(ops))]
		res, err := c.gql(part...)
		if errors.Is(err, errAuth) {
			return nil, err
		}
		if err != nil && firstErr == nil {
			firstErr = err
		}
		if res == nil {
			res = make([]map[string]any, len(part))
		}
		out = append(out, res...)
	}
	return out, firstErr
}

// ---- watching ----

// playlistURL returns the address of the lowest-quality variant playlist of a live channel.
func (c *Client) playlistURL(login string) (string, error) {
	data, err := c.gqlOne("PlaybackAccessToken", map[string]any{
		"isLive": true, "isVod": false, "login": login, "platform": "web", "playerType": "site", "vodID": "",
	})
	if err != nil {
		return "", err
	}
	tok := dig(data, "streamPlaybackAccessToken")
	value, sig := str(dig(tok, "value")), str(dig(tok, "signature"))
	if value == "" || sig == "" {
		return "", errors.New("no playback token")
	}
	q := url.Values{"sig": {sig}, "token": {value}}
	req, _ := http.NewRequest("GET", c.ep.Usher+"/api/channel/hls/"+url.PathEscape(login)+".m3u8?"+q.Encode(), nil)
	c.baseHeaders(req)
	res, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	res.Body.Close()
	if res.StatusCode >= 400 && res.StatusCode < 500 {
		return "", errGone // Twitch answers like this when the channel is not live
	}
	if res.StatusCode >= 500 {
		return "", fmt.Errorf("playlist: HTTP %d", res.StatusCode)
	}
	last := ""
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "http") {
			last = line // qualities go from best to worst
		}
	}
	if last == "" {
		return "", errors.New("playlist: no stream variants")
	}
	return last, nil
}

// touchStream is what counts as watching: read the current list of video chunks and ask for the
// headers of each one (HEAD) - nothing is downloaded or decoded. gone=true when the stream ended.
func (c *Client) touchStream(playlist string) (gone bool, err error) {
	req, _ := http.NewRequest("GET", playlist, nil)
	req.Header.Set("User-Agent", userAgent)
	req.Close = true
	res, err := c.http.Do(req)
	if err != nil {
		return false, err
	}
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	res.Body.Close()
	if res.StatusCode >= 400 {
		return true, nil
	}
	n := 0
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "http") {
			continue
		}
		hreq, _ := http.NewRequest("HEAD", line, nil)
		hreq.Header.Set("User-Agent", userAgent)
		hres, err := c.http.Do(hreq)
		if err != nil {
			return false, err
		}
		hres.Body.Close()
		if hres.StatusCode != 200 {
			return false, fmt.Errorf("chunk: HTTP %d", hres.StatusCode)
		}
		n++
	}
	if n == 0 {
		return true, nil
	}
	return false, nil
}

var (
	reSpade    = regexp.MustCompile(`(?i)"spade_?url": ?"(https://[.\w\-/]+)"`)
	reSettings = regexp.MustCompile(`(?i)src="(https://[\w.]+/config/settings\.[0-9a-f]{32}\.js)"`)
)

func (c *Client) getText(u string) (string, error) {
	req, _ := http.NewRequest("GET", u, nil)
	c.baseHeaders(req)
	res, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	return string(raw), err
}

// spadeURL finds the address Twitch's player reports "minute watched" to (channel page -> settings script).
func (c *Client) spadeURL(login string) (string, error) {
	html, err := c.getText(c.ep.WWW + "/" + url.PathEscape(login))
	if err != nil {
		return "", err
	}
	if m := reSpade.FindStringSubmatch(html); m != nil {
		if t := os.Getenv("DROPOMAT_SPADE"); t != "" { // tests: the mock cannot serve https
			return t, nil
		}
		return m[1], nil
	}
	m := reSettings.FindStringSubmatch(html)
	if m == nil {
		return "", errors.New("spade: settings script not found")
	}
	js, err := c.getText(m[1])
	if err != nil {
		return "", err
	}
	if m := reSpade.FindStringSubmatch(js); m != nil {
		return m[1], nil
	}
	return "", errors.New("spade: address not found")
}

type watchInfo struct {
	ChannelID, Login, BroadcastID, Game, GameID string
}

// minuteWatched reports one watched minute, the way Twitch's own web player does.
// "location" and "player" say that the report comes from the player on a channel page. Twitch accepts a
// report without them (it still answers 204) but then credits no channel points for the watching.
func (c *Client) minuteWatched(spade string, w watchInfo) error {
	_, uid, _, _ := c.auth()
	payload := []map[string]any{{
		"event": "minute-watched",
		"properties": map[string]any{
			"broadcast_id": w.BroadcastID, "channel_id": w.ChannelID, "channel": w.Login,
			"client_time": time.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
			"game":        w.Game, "game_id": w.GameID, "hidden": false, "is_live": true, "live": true,
			"location": "channel", "player": "site",
			"logged_in": true, "minutes_logged": 1, "muted": false, "user_id": num(uid),
		},
	}}
	raw, _ := json.Marshal(payload)
	form := url.Values{"data": {base64.StdEncoding.EncodeToString(raw)}}
	req, _ := http.NewRequest("POST", spade, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", userAgent)
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	io.Copy(io.Discard, io.LimitReader(res.Body, 1<<16))
	res.Body.Close()
	if res.StatusCode != 204 && res.StatusCode != 200 {
		return fmt.Errorf("minute-watched: HTTP %d", res.StatusCode)
	}
	return nil
}
