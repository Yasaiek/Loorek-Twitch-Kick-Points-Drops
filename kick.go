package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/andybalholm/brotli"
	http "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
)

// Kick sits behind a bot filter that turns away programs which do not look like a browser,
// so every request is made with Chrome's network fingerprint and Chrome's headers.
const (
	kickUA          = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36"
	kickClientToken = "e1393935a959b4020a4491574f6490129f678acdaa92760471263db43487f823"
)

var (
	errKickAuth    = errors.New("kick: session not accepted")
	errKickBlocked = errors.New("kick: request refused (403)")
)

type kickChan struct {
	Login, Name          string
	Img                  string
	ID, UserID, StreamID int64
	Online, Missing      bool
	Watching             bool
	Points               int64
	PtsKnown             bool
	Earned               int64
	WatchedSec           int64
	Category             string
	Err                  string

	conn       *wsConn
	dead       *atomic.Bool
	retryAt    time.Time
	backoff    time.Duration
	lastShake  time.Time
	lastEvent  time.Time
	lastPoints time.Time
}

// watchedNames lists the channels points are being collected on right now.
func (k *Kick) watchedNames() []string {
	k.mu.Lock()
	defer k.mu.Unlock()
	var out []string
	for _, ch := range k.chans {
		if ch.Watching {
			out = append(out, firstNonEmpty(ch.Name, ch.Login))
		}
	}
	sort.Strings(out)
	return out
}

type Kick struct {
	e  *Engine
	hc tls_client.HttpClient

	mu       sync.Mutex
	state    string // out | off | ok | bad | blocked
	user     string
	checked  string // token that was last verified
	chans    map[string]*kickChan
	warmed   bool
	nextInfo time.Time
	nextAll  time.Time
	blockLog time.Time
	auto     bool // a sign-in window is trying keys: failures are expected and stay quiet
}

func newKick(e *Engine) *Kick {
	k := &Kick{e: e, state: "out", chans: map[string]*kickChan{}}
	opts := []tls_client.HttpClientOption{
		tls_client.WithClientProfile(profiles.Chrome_133),
		tls_client.WithTimeoutSeconds(20),
		tls_client.WithCookieJar(tls_client.NewCookieJar()),
		tls_client.WithRandomTLSExtensionOrder(),
	}
	hc, err := tls_client.NewHttpClient(tls_client.NewNoopLogger(), opts...)
	if err != nil {
		e.log.Err("L.kickPrep", err)
		return k
	}
	k.hc = hc
	return k
}

func (k *Kick) get(u string, extra map[string]string, token string) (int, any, error) {
	if k.hc == nil {
		return 0, nil, errors.New("kick: no client")
	}
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return 0, nil, err
	}
	base := strings.TrimRight(k.e.c.ep.Kick, "/")
	h := map[string]string{
		"accept":             "application/json, text/plain, */*",
		"accept-language":    "en-US,en;q=0.9",
		"accept-encoding":    "gzip, deflate, br",
		"origin":             "https://kick.com",
		"referer":            "https://kick.com/",
		"sec-ch-ua":          `"Not(A:Brand";v="99", "Google Chrome";v="133", "Chromium";v="133"`,
		"sec-ch-ua-mobile":   "?0",
		"sec-ch-ua-platform": `"Windows"`,
		"sec-fetch-dest":     "empty",
		"sec-fetch-mode":     "cors",
		"sec-fetch-site":     "same-site",
		"user-agent":         kickUA,
		"x-client-token":     kickClientToken,
		"x-requested-with":   "XMLHttpRequest",
	}
	if strings.HasPrefix(u, base) {
		h["sec-fetch-site"] = "same-origin"
	}
	if token != "" {
		h["authorization"] = "Bearer " + token
	}
	for k2, v := range extra {
		h[strings.ToLower(k2)] = v
	}
	order := []string{"sec-ch-ua-platform", "authorization", "x-client-token", "sec-ch-ua", "sec-ch-ua-mobile", "x-requested-with", "user-agent", "accept", "x-chatroom", "x-user-id", "origin", "sec-fetch-site", "sec-fetch-mode", "sec-fetch-dest", "referer", "accept-encoding", "accept-language"}
	req.Header = http.Header{http.HeaderOrderKey: order}
	for name, v := range h {
		req.Header.Set(name, v)
	}
	res, err := k.hc.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	// the body normally arrives already unpacked; unpack here if it did not
	if len(raw) > 2 && raw[0] == 0x1f && raw[1] == 0x8b {
		if zr, err := gzip.NewReader(bytes.NewReader(raw)); err == nil {
			raw, _ = io.ReadAll(io.LimitReader(zr, 8<<20))
		}
	} else if res.Header.Get("Content-Encoding") == "br" && !json.Valid(raw) {
		if out, err := io.ReadAll(io.LimitReader(brotli.NewReader(bytes.NewReader(raw)), 8<<20)); err == nil && len(out) > 0 {
			raw = out
		}
	}
	var v any
	json.Unmarshal(raw, &v)
	switch res.StatusCode {
	case 401:
		return 401, v, errKickAuth
	case 403:
		return 403, v, errKickBlocked
	}
	return res.StatusCode, v, nil
}

func cleanKickToken(raw string) string {
	s := strings.TrimSpace(raw)
	s = strings.Trim(s, `"'`)
	for _, p := range []string{"authorization:", "Authorization:", "session_token=", "Bearer ", "bearer "} {
		s = strings.TrimSpace(strings.TrimPrefix(s, p))
	}
	if strings.Contains(s, "%7C") || strings.Contains(s, "%7c") {
		if d, err := url.QueryUnescape(s); err == nil {
			s = d
		}
	}
	if len(s) < 20 || len(s) > 400 || strings.ContainsAny(s, " \t\r\n") {
		return ""
	}
	return s
}

// snapshot of what the user wants. Reads Engine state under its lock.
func (k *Kick) wants() (token string, on bool, logins []string, max int) {
	e := k.e
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, c := range e.set.KickChannels {
		if c.On {
			logins = append(logins, c.Login)
		}
	}
	max = e.set.KickMax
	if max < 1 {
		max = 2
	}
	return e.auth.KickToken, e.set.KickOn, logins, max
}

func (k *Kick) rt(login string) *kickChan {
	ch := k.chans[login]
	if ch == nil {
		ch = &kickChan{Login: login, Name: login, backoff: 30 * time.Second}
		k.chans[login] = ch
	}
	return ch
}

func (k *Kick) closeAll() {
	k.mu.Lock()
	defer k.mu.Unlock()
	for _, ch := range k.chans {
		k.drop(ch)
	}
}

// drop closes a channel's connection. Call with k.mu held.
func (k *Kick) drop(ch *kickChan) {
	if ch.conn != nil {
		ch.conn.Close()
		ch.conn = nil
	}
	ch.Watching = false
}

func (k *Kick) setState(s string) {
	k.mu.Lock()
	k.state = s
	k.mu.Unlock()
}

func (k *Kick) run() {
	t := time.NewTicker(iv(time.Second))
	defer t.Stop()
	for {
		select {
		case <-k.e.quit:
			k.closeAll()
			return
		case <-t.C:
		}
		func() {
			defer func() {
				if r := recover(); r != nil {
					k.e.log.Err("L.kickInternal", r)
				}
			}()
			k.step(time.Now())
		}()
	}
}

func (k *Kick) blocked(now time.Time, where string) {
	k.mu.Lock()
	k.state, k.warmed = "blocked", false
	k.nextInfo = now.Add(iv(3 * time.Minute))
	logIt := now.Sub(k.blockLog) > iv(30*time.Minute)
	if logIt {
		k.blockLog = now
	}
	k.mu.Unlock()
	if logIt {
		k.e.log.Warn("L.kickBlocked", where)
	}
}

func (k *Kick) authFailed() {
	k.e.mu.Lock()
	k.e.auth.KickToken = ""
	k.e.saveAuth()
	k.e.mu.Unlock()
	k.closeAll()
	k.mu.Lock()
	k.state, k.user, k.checked = "bad", "", ""
	auto := k.auto
	k.mu.Unlock()
	if !auto {
		k.e.log.Warn("L.kickBadKey")
	}
}

func (k *Kick) step(now time.Time) {
	token, on, logins, max := k.wants()
	if token == "" {
		k.mu.Lock()
		if k.state != "bad" {
			k.state = "out"
		}
		k.mu.Unlock()
		k.closeAll()
		return
	}
	if !on {
		k.setState("off")
		k.closeAll()
		return
	}
	base := strings.TrimRight(k.e.c.ep.Kick, "/")
	k.mu.Lock()
	warmed, checked, nextInfo := k.warmed, k.checked, k.nextInfo
	k.mu.Unlock()
	if now.Before(nextInfo) && !warmed {
		return // waiting after a refusal
	}
	if !warmed {
		st, _, err := k.get(base, map[string]string{"accept": "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8", "sec-fetch-dest": "document", "sec-fetch-mode": "navigate", "sec-fetch-site": "none"}, "")
		if err != nil && !errors.Is(err, errKickBlocked) {
			k.e.log.Debug("kick warm: %v", err)
			k.mu.Lock()
			k.nextInfo = now.Add(iv(time.Minute))
			k.mu.Unlock()
			return
		}
		if st == 403 {
			k.blocked(now, "home page")
			return
		}
		k.mu.Lock()
		k.warmed = true
		k.nextInfo = time.Time{}
		k.mu.Unlock()
	}
	if checked != token { // who is this session
		st, v, err := k.get(base+"/api/v1/user", nil, token)
		switch {
		case errors.Is(err, errKickAuth):
			k.authFailed()
			return
		case errors.Is(err, errKickBlocked):
			k.blocked(now, "account")
			return
		case err != nil:
			k.e.log.Debug("kick user: %v", err)
			return
		}
		name := str(dig(v, "username"))
		if name == "" {
			name = str(dig(v, "data", "username"))
		}
		k.mu.Lock()
		k.checked, k.user, k.state = token, name, "ok"
		k.nextInfo, k.nextAll = time.Time{}, now.Add(iv(30*time.Second))
		k.mu.Unlock()
		if st == 200 && name != "" {
			k.e.log.OK("L.kickSignedIn", name)
		} else {
			k.e.log.Info("L.kickKeyOk", st)
		}
	}

	// who is live
	k.mu.Lock()
	due := !now.Before(k.nextInfo)
	if due {
		k.nextInfo = now.Add(iv(2 * time.Minute))
	}
	want := map[string]bool{}
	var chs []*kickChan
	for _, l := range logins {
		want[l] = true
		chs = append(chs, k.rt(l))
	}
	for l, ch := range k.chans {
		if !want[l] {
			k.drop(ch)
			delete(k.chans, l)
		}
	}
	k.mu.Unlock()
	if due {
		for i, ch := range chs {
			if i > 0 {
				time.Sleep(iv(700 * time.Millisecond)) // be gentle: one channel at a time
			}
			stop := k.info(ch, token, now)
			k.mu.Lock()
			login, name, img := ch.Login, ch.Name, ch.Img
			k.mu.Unlock()
			k.e.mu.Lock()
			k.e.rememberFace(k.e.set.KickChannels, login, name, img)
			k.e.mu.Unlock()
			if stop {
				return
			}
		}
	}

	// watch the first live ones, up to the limit
	k.mu.Lock()
	n := 0
	var open, live []*kickChan
	for _, ch := range chs {
		watch := ch.Online && !ch.Missing && n < max
		if watch {
			n++
			if ch.conn != nil && ch.dead != nil && ch.dead.Load() {
				ch.conn.Close()
				ch.conn = nil
				ch.retryAt = now.Add(iv(ch.backoff))
				if ch.backoff < 10*time.Minute {
					ch.backoff *= 2
				}
				k.e.log.Debug("kick %s: connection lost", ch.Login)
			}
			if ch.conn == nil {
				ch.Watching = false
				if !now.Before(ch.retryAt) {
					open = append(open, ch)
				}
			} else {
				live = append(live, ch)
			}
		} else if ch.conn != nil || ch.Watching {
			k.drop(ch)
		}
	}
	k.mu.Unlock()

	for _, ch := range open {
		k.connect(ch, token, now)
		time.Sleep(iv(500 * time.Millisecond))
	}
	for _, ch := range live {
		k.keep(ch, now)
		if now.Sub(ch.lastPoints) >= iv(5*time.Minute) {
			if stop := k.points(ch, token, now); stop {
				return
			}
		}
	}
	// balances of the channels that are not being watched, now and then
	k.mu.Lock()
	all := !now.Before(k.nextAll)
	if all {
		k.nextAll = now.Add(iv(20 * time.Minute))
	}
	k.mu.Unlock()
	if all {
		for _, ch := range chs {
			if ch.conn == nil && !ch.Missing && ch.ID != 0 {
				time.Sleep(iv(700 * time.Millisecond))
				if stop := k.points(ch, token, now); stop {
					return
				}
			}
		}
	}
}

// info reads a channel: its ids and whether it is live. Returns true when the whole round should stop.
func (k *Kick) info(ch *kickChan, token string, now time.Time) bool {
	base := strings.TrimRight(k.e.c.ep.Kick, "/")
	st, v, err := k.get(base+"/api/v2/channels/"+url.PathEscape(ch.Login), map[string]string{"referer": "https://kick.com/" + ch.Login}, token)
	switch {
	case errors.Is(err, errKickAuth):
		k.authFailed()
		return true
	case errors.Is(err, errKickBlocked):
		k.blocked(now, "channel info")
		return true
	case err != nil:
		k.e.log.Debug("kick info %s: %v", ch.Login, err)
		return false
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.state == "blocked" {
		k.state = "ok"
	}
	if st == 404 {
		ch.Missing, ch.Online = true, false
		return false
	}
	if st != 200 {
		k.e.log.Debug("kick info %s: HTTP %d", ch.Login, st)
		return false
	}
	if d := dig(v, "data"); d != nil && dig(v, "id") == nil {
		v = d
	}
	id := num(dig(v, "id"))
	if id == 0 {
		return false
	}
	ch.Missing, ch.ID = false, id
	ch.UserID = num(dig(v, "user_id"))
	if ch.UserID == 0 {
		ch.UserID = num(dig(v, "user", "id"))
	}
	if n := str(dig(v, "user", "username")); n != "" {
		ch.Name = n
	}
	if img := firstNonEmpty(str(dig(v, "user", "profile_pic")), str(dig(v, "user", "profilePic"))); img != "" {
		ch.Img = img
	}
	ls := dig(v, "livestream")
	if d := dig(ls, "data"); d != nil && dig(ls, "id") == nil {
		ls = d
	}
	was := ch.Online
	sid := num(dig(ls, "id"))
	live := ls != nil && sid != 0 && (dig(ls, "is_live") == nil || boolean(dig(ls, "is_live")))
	ch.Online = live
	if live {
		if ch.StreamID != sid && ch.conn != nil { // a new broadcast: report under the new id
			ch.conn.Close()
			ch.conn = nil
		}
		ch.StreamID = sid
		ch.Category = ""
		if cats := list(dig(ls, "categories")); len(cats) > 0 {
			ch.Category = str(dig(cats[0], "name"))
		}
	} else {
		ch.StreamID = 0
	}
	if was != ch.Online {
		if ch.Online {
			k.e.log.Info("L.kickLive", ch.Name)
		} else {
			k.e.log.Info("L.kickEnded", ch.Name)
		}
	}
	return false
}

// connect opens the viewer connection for a channel - the same one the Kick player keeps open.
func (k *Kick) connect(ch *kickChan, token string, now time.Time) {
	fail := func(err error) {
		k.mu.Lock()
		ch.Err = err.Error()
		ch.retryAt = now.Add(iv(ch.backoff))
		if ch.backoff < 10*time.Minute {
			ch.backoff *= 2
		}
		k.mu.Unlock()
		k.e.log.Debug("kick connect %s: %v", ch.Login, err)
	}
	wsBase := strings.TrimRight(k.e.c.ep.KickWS, "/")
	uid := ch.UserID
	if uid == 0 {
		uid = ch.ID
	}
	st, v, err := k.get(wsBase+"/viewer/v1/token", map[string]string{
		"referer": "https://kick.com/" + ch.Login, "content-type": "application/json",
		"x-chatroom": fmt.Sprint(ch.ID), "x-user-id": fmt.Sprint(uid),
	}, token)
	if errors.Is(err, errKickAuth) {
		k.authFailed()
		return
	}
	if errors.Is(err, errKickBlocked) {
		k.blocked(now, "viewer connection")
		fail(err)
		return
	}
	if err != nil {
		fail(err)
		return
	}
	wsToken := firstNonEmpty(str(dig(v, "data", "token")), str(dig(v, "data", "websocket_token")), str(dig(v, "token")), str(dig(v, "websocket_token")))
	if st != 200 || wsToken == "" {
		fail(fmt.Errorf("viewer token: HTTP %d", st))
		return
	}
	wsURL := strings.Replace(wsBase, "http", "ws", 1) + "/viewer/v1/connect?token=" + url.QueryEscape(wsToken)
	conn, err := wsDial(wsURL, map[string]string{"User-Agent": kickUA, "Origin": "https://kick.com"})
	if err != nil {
		fail(err)
		return
	}
	dead := &atomic.Bool{}
	go func() { // answer the server's pings; notice when the connection drops
		for {
			msg, err := conn.Read()
			if err != nil {
				dead.Store(true)
				return
			}
			m := strings.TrimSpace(msg)
			if m == "ping" || strings.Contains(m, `"type":"ping"`) || strings.Contains(m, `"type": "ping"`) {
				conn.WriteText(`{"type":"pong"}`)
			}
		}
	}()
	k.mu.Lock()
	ch.conn, ch.dead, ch.Watching, ch.Err = conn, dead, true, ""
	ch.backoff = 30 * time.Second
	ch.lastShake, ch.lastEvent = time.Time{}, now
	k.mu.Unlock()
	k.e.log.OK("L.kickWatch", ch.Name)
	k.keep(ch, now)
}

// keep sends what the player sends while a stream is open: a handshake every ~30 s and a
// "watching" event every ~11 s.
func (k *Kick) keep(ch *kickChan, now time.Time) {
	conn := ch.conn
	if conn == nil {
		return
	}
	if now.Sub(ch.lastShake) >= iv(30*time.Second) {
		ch.lastShake = now
		conn.WriteText(fmt.Sprintf(`{"type":"channel_handshake","data":{"message":{"channelId":%d}}}`, ch.ID))
		conn.WriteText(`{"type":"ping"}`)
	}
	if now.Sub(ch.lastEvent) >= iv(11*time.Second) {
		k.mu.Lock()
		if d := now.Sub(ch.lastEvent); d < iv(40*time.Second) {
			ch.WatchedSec += int64(d * time.Duration(speed) / time.Second)
		}
		k.mu.Unlock()
		ch.lastEvent = now
		if err := conn.WriteText(fmt.Sprintf(`{"type":"user_event","data":{"message":{"name":"tracking.user.watch.livestream","channel_id":%d,"livestream_id":%d}}}`, ch.ID, ch.StreamID)); err != nil && ch.dead != nil {
			ch.dead.Store(true)
		}
	}
}

// points reads the balance on a channel. Returns true when the whole round should stop.
func (k *Kick) points(ch *kickChan, token string, now time.Time) bool {
	ch.lastPoints = now
	base := strings.TrimRight(k.e.c.ep.Kick, "/")
	st, v, err := k.get(base+"/api/v2/channels/"+url.PathEscape(ch.Login)+"/points", map[string]string{"referer": "https://kick.com/" + ch.Login}, token)
	switch {
	case errors.Is(err, errKickAuth):
		k.authFailed()
		return true
	case errors.Is(err, errKickBlocked):
		k.blocked(now, "points")
		return true
	case err != nil || st != 200:
		k.e.log.Debug("kick points %s: %v HTTP %d", ch.Login, err, st)
		return false
	}
	p := dig(v, "data", "points")
	if p == nil {
		p = dig(v, "points")
	}
	if p == nil {
		return false
	}
	k.mu.Lock()
	pts := num(p)
	if ch.PtsKnown && pts > ch.Points {
		ch.Earned += pts - ch.Points
	}
	ch.Points, ch.PtsKnown = pts, true
	k.mu.Unlock()
	return false
}

// ---- panel ----

func (k *Kick) view() map[string]any {
	e := k.e
	e.mu.Lock()
	cfg := append([]ChanCfg(nil), e.set.KickChannels...)
	on, max, has := e.set.KickOn, e.set.KickMax, e.auth.KickToken != ""
	e.mu.Unlock()
	if max < 1 {
		max = 2
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	chans := []map[string]any{}
	var earned int64
	watching := 0
	for _, c := range cfg {
		ch := k.rt(c.Login)
		earned += ch.Earned
		if ch.Watching {
			watching++
		}
		name := ch.Name
		if name == ch.Login && c.Name != "" {
			name = c.Name
		}
		chans = append(chans, map[string]any{"login": ch.Login, "name": name, "img": firstNonEmpty(ch.Img, c.Img), "on": c.On, "online": ch.Online, "watching": ch.Watching,
			"game": ch.Category, "balance": ch.Points, "balKnown": ch.PtsKnown, "earned": ch.Earned, "missing": ch.Missing, "watchedMin": ch.WatchedSec / 60})
	}
	state := k.state
	if !has && state != "bad" {
		state = "out"
	}
	return map[string]any{"state": state, "user": k.user, "on": on, "max": max, "channels": chans, "earned": earned, "watching": watching}
}

func (e *Engine) kickSetToken(raw string) string {
	t := cleanKickToken(raw)
	if t == "" {
		return "bad"
	}
	e.mu.Lock()
	e.auth.KickToken = t
	e.saveAuth()
	e.mu.Unlock()
	e.kick.mu.Lock()
	e.kick.state, e.kick.checked, e.kick.warmed = "ok", "", false
	e.kick.nextInfo = time.Time{}
	e.kick.mu.Unlock()
	return "ok"
}

func (e *Engine) kickLogout() {
	e.mu.Lock()
	e.auth.KickToken = ""
	e.saveAuth()
	e.mu.Unlock()
	e.kick.closeAll()
	e.kick.mu.Lock()
	e.kick.state, e.kick.user, e.kick.checked = "out", "", ""
	e.kick.mu.Unlock()
	e.forgetBrowserSessions()
	e.log.Info("L.kickOut")
}

func (e *Engine) kickAdd(raw, name, img string) string {
	name, img = face(name, img)
	s := strings.TrimSpace(strings.ToLower(raw))
	if i := strings.Index(s, "kick.com/"); i >= 0 {
		s = s[i+len("kick.com/"):]
	}
	if i := strings.IndexAny(s, "/?# "); i >= 0 {
		s = s[:i]
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_' || r == '-') {
			return "bad"
		}
	}
	if len(s) < 2 || len(s) > 40 {
		return "bad"
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, c := range e.set.KickChannels {
		if c.Login == s {
			return "dup"
		}
	}
	e.set.KickChannels = append(e.set.KickChannels, ChanCfg{Login: s, On: true, Name: name, Img: img})
	e.saveSettings()
	e.kick.mu.Lock()
	e.kick.nextInfo = time.Time{}
	e.kick.mu.Unlock()
	return "ok"
}

func (e *Engine) kickRemove(login string) {
	e.mu.Lock()
	out := e.set.KickChannels[:0]
	for _, c := range e.set.KickChannels {
		if c.Login != login {
			out = append(out, c)
		}
	}
	e.set.KickChannels = out
	e.saveSettings()
	e.mu.Unlock()
}

func (e *Engine) kickToggle(login string, on bool) {
	e.mu.Lock()
	for i := range e.set.KickChannels {
		if e.set.KickChannels[i].Login == login {
			e.set.KickChannels[i].On = on
		}
	}
	e.saveSettings()
	e.mu.Unlock()
	e.kick.mu.Lock()
	e.kick.nextInfo = time.Time{}
	e.kick.mu.Unlock()
}

func (e *Engine) kickReorder(order []string) {
	e.mu.Lock()
	pos := map[string]int{}
	for i, l := range order {
		pos[l] = i + 1
	}
	sort.SliceStable(e.set.KickChannels, func(a, b int) bool {
		pa, pb := pos[e.set.KickChannels[a].Login], pos[e.set.KickChannels[b].Login]
		if pa == 0 {
			pa = 1 << 30
		}
		if pb == 0 {
			pb = 1 << 30
		}
		return pa < pb
	})
	e.saveSettings()
	e.mu.Unlock()
}
