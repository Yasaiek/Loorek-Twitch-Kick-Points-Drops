package main

import (
	"errors"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

var version = "0.8.8" // a variable so that test builds can override it
const appName = "Loorek"

// speed > 1 shortens every waiting time (tests only: DROPOMAT_FAST).
var speed = 1

func iv(d time.Duration) time.Duration { return d / time.Duration(speed) }

type GameRef struct {
	ID   string `json:"id"`
	Slug string `json:"slug"`
	Name string `json:"name"`
	Art  string `json:"art"`
}

type ChanCfg struct {
	Login string `json:"login"`
	On    bool   `json:"on"`
	Name  string `json:"name,omitempty"` // display name and avatar as last seen: shown before the first answer arrives
	Img   string `json:"img,omitempty"`
}

type Settings struct {
	Lang         string    `json:"lang"`
	PointsOn     bool      `json:"pointsOn"`
	ChatOn       bool      `json:"chatOn"`
	DropsOn      bool      `json:"dropsOn"`
	AutoClaim    bool      `json:"autoClaim"`
	Channels     []ChanCfg `json:"channels"`
	Queue        []GameRef `json:"queue"`
	Tracked      []GameRef `json:"tracked"`
	RotateMin    int       `json:"rotateMin"`
	StallMin     int       `json:"stallMin"`
	KickOn       bool      `json:"kickOn"`
	KickChannels []ChanCfg `json:"kickChannels"`
	KickMax      int       `json:"kickMax"`
	Autostart    bool      `json:"autostart"`
	OpenOnStart  bool      `json:"openOnStart"`
	AutoUpdate   bool      `json:"autoUpdate"`
	Discord      RPCCfg    `json:"discord"`
}

func defaultSettings() Settings {
	return Settings{Lang: "auto", PointsOn: true, ChatOn: true, DropsOn: true, AutoClaim: true, StallMin: 15, OpenOnStart: true, KickOn: true, KickMax: 2, AutoUpdate: true,
		Discord: RPCCfg{Drop: true, Queue: true, Tracked: true, Points: true, Button: true}} // off until the user switches it on
}

type AuthFile struct {
	Token     string `json:"token"`
	UserID    string `json:"userId"`
	Login     string `json:"login"`
	DeviceID  string `json:"deviceId"`
	KickToken string `json:"kickToken,omitempty"`
}

// chanRT is the live state of one channel (a points channel from the list, or the channel drops are farmed on).
type chanRT struct {
	Login, ID, Name string
	Img             string
	Missing         bool
	Online          bool
	StreamID        string
	Game, GameID    string
	Viewers         int64
	Balance         int64
	BalKnown        bool
	Earned          int64
	Claims          int
	Watching        bool
	WatchedSec      int64
	Err             string

	playlist   string
	playlistAt time.Time
	spade      string
	spadeTried time.Time
	lastTouch  time.Time
	lastOK     time.Time
	lastSpade  time.Time
	lastCtx    time.Time
	gainAt     time.Time // when the balance last grew, or when watching started
	noGain     bool      // already said that watching brings no points
	fails      int
	gone       int
}

type Engine struct {
	mu  sync.Mutex
	dir string
	log *Logger
	c   *Client

	set       Settings
	auth      AuthFile
	authState string // out | checking | ok | login
	authNote  string
	authRetry time.Time
	loginMode string // auto (own sign-in window) | manual (everyday browser + pasted address)

	loginMu       sync.Mutex
	lw            *loginWin
	kickLoginOn   bool
	kickLoginNote string

	upd    updState
	apiKey string
	port   int

	chans map[string]*chanRT
	slots []*chanRT

	rpcState string // Discord Rich Presence: off | noid | nodiscord | badid | refused | ok

	nextOnline   time.Time
	nextBalances time.Time
	rotateStart  time.Time

	drops dropsState
	chat  *chatClient
	kick  *Kick

	wake   chan struct{}
	quit   chan struct{}
	onQuit func()

	winGen  int
	started time.Time
}

func newEngine(dir string) *Engine {
	log := newLogger(dir)
	e := &Engine{
		dir: dir, log: log, c: newClient(log), set: defaultSettings(), authState: "checking",
		chans: map[string]*chanRT{}, wake: make(chan struct{}, 1), quit: make(chan struct{}), started: time.Now(),
	}
	e.drops.init()
	readJSON(filepath.Join(dir, "settings.json"), &e.set)
	if e.set.StallMin < 5 {
		e.set.StallMin = 15
	}
	if e.set.Lang == "" {
		e.set.Lang = "pl"
	}
	readJSON(filepath.Join(dir, "auth.json"), &e.auth)
	var hashes map[string]string
	if readJSON(filepath.Join(dir, "hashes.json"), &hashes) == nil {
		for k, v := range hashes {
			gqlHashes[k] = v
		}
	}
	e.chat = newChat(e)
	if e.set.KickMax < 1 || e.set.KickMax > 5 {
		e.set.KickMax = 2
	}
	e.kick = newKick(e)
	return e
}

func (e *Engine) saveSettings() { writeJSON(filepath.Join(e.dir, "settings.json"), &e.set) }
func (e *Engine) saveAuth()     { writeJSON(filepath.Join(e.dir, "auth.json"), &e.auth) }

func (e *Engine) poke() {
	select {
	case e.wake <- struct{}{}:
	default:
	}
}

func (e *Engine) run() {
	e.log.Info("L.started", appName, version)
	if e.set.Autostart { // the file name changes with every version: keep the autostart entry pointing at this one
		setAutostart(true)
	}
	go e.chat.run()
	go e.kick.run()
	go e.updateLoop()
	go usageLoop(e.quit)
	go e.discordLoop()
	t := time.NewTicker(iv(time.Second))
	defer t.Stop()
	for {
		select {
		case <-e.quit:
			return
		case <-t.C:
		case <-e.wake:
		}
		func() {
			defer func() {
				if r := recover(); r != nil {
					e.log.Err("L.internal", r)
				}
			}()
			e.step(time.Now())
		}()
	}
}

func (e *Engine) step(now time.Time) {
	if !e.ensureAuth(now) {
		return
	}
	// the panel moves these two timers too (refresh, a new channel), so they are read and set under the lock
	e.mu.Lock()
	online := !now.Before(e.nextOnline)
	if online {
		e.nextOnline = now.Add(iv(2 * time.Minute))
	}
	e.mu.Unlock()
	if online {
		e.checkOnline()
	}
	e.dropsStep(now)
	e.decideSlots(now)
	e.mu.Lock()
	slots := append([]*chanRT(nil), e.slots...)
	pointsOn := e.set.PointsOn
	e.mu.Unlock()
	for _, ch := range slots {
		if now.Sub(ch.lastTouch) >= iv(20*time.Second) {
			e.watchOnce(ch, now)
		}
	}
	if pointsOn {
		for _, ch := range slots {
			if e.isListed(ch) && now.Sub(ch.lastCtx) >= iv(time.Minute) {
				e.pointsContext(ch, now)
			}
		}
		e.mu.Lock()
		balances := !now.Before(e.nextBalances)
		if balances {
			e.nextBalances = now.Add(iv(10 * time.Minute))
		}
		e.mu.Unlock()
		if balances {
			e.refreshBalances(now)
		}
	}
}

// ---- sign-in state ----

func (e *Engine) ensureAuth(now time.Time) bool {
	e.mu.Lock()
	state, token := e.authState, e.auth.Token
	retry := e.authRetry
	e.mu.Unlock()
	if state == "ok" {
		return true
	}
	if state != "checking" || now.Before(retry) {
		return false
	}
	if token == "" {
		e.mu.Lock()
		e.authState = "out"
		e.mu.Unlock()
		return false
	}
	uid, login, ok, err := e.c.validate(token)
	e.mu.Lock()
	defer e.mu.Unlock()
	if errors.Is(err, errWrongClient) {
		e.log.Warn("L.wrongClient")
		e.auth.Token = ""
		e.saveAuth()
		e.authState, e.authNote = "out", "wrongclient"
		return false
	}
	if err != nil {
		e.authNote = "net"
		e.authRetry = now.Add(iv(30 * time.Second))
		e.log.Debug("validate: %v", err)
		return false
	}
	if !ok {
		e.log.Warn("L.expired")
		e.auth.Token = ""
		e.saveAuth()
		e.authState, e.authNote = "out", "expired"
		return false
	}
	e.auth.UserID, e.auth.Login = uid, login
	e.c.setAuth(token, uid, login, e.auth.DeviceID)
	if e.auth.DeviceID == "" {
		e.mu.Unlock()
		dev := e.c.ensureDevice()
		e.mu.Lock()
		e.auth.DeviceID = dev
	}
	e.saveAuth()
	e.authState, e.authNote = "ok", ""
	e.nextOnline, e.nextBalances = time.Time{}, now.Add(iv(20*time.Second))
	e.drops.force = true
	e.log.OK("L.signedIn", login)
	e.chat.sync()
	return true
}

// authLost is called when Twitch answers 401 in the middle of work.
func (e *Engine) authLost() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.authState != "ok" {
		return
	}
	e.authState, e.authRetry = "checking", time.Time{}
	e.slots = nil
	for _, ch := range e.chans {
		ch.Watching = false
	}
	e.drops.cur = nil
}

func (e *Engine) gqlErr(what string, err error) {
	if err == nil {
		return
	}
	if errors.Is(err, errAuth) {
		e.authLost()
		return
	}
	e.log.Debug("%s: %v", what, err)
}

func (e *Engine) setToken(token string) {
	e.mu.Lock()
	e.auth.Token = token
	e.authState, e.authRetry, e.authNote = "checking", time.Time{}, ""
	e.saveAuth()
	e.mu.Unlock()
	e.poke()
}

func (e *Engine) logout() {
	e.mu.Lock()
	e.auth = AuthFile{DeviceID: e.auth.DeviceID, KickToken: e.auth.KickToken}
	e.saveAuth()
	e.c.setAuth("", "", "", "")
	e.authState, e.authNote = "out", ""
	defer e.forgetBrowserSessions()
	e.slots = nil
	for _, ch := range e.chans {
		ch.Watching, ch.Online, ch.BalKnown = false, false, false
	}
	e.drops.cur = nil
	e.drops.camps = map[string]*Campaign{}
	e.mu.Unlock()
	e.log.Info("L.signedOut")
}

// ---- channel list ----

func normLogin(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	if i := strings.Index(s, "twitch.tv/"); i >= 0 {
		s = s[i+len("twitch.tv/"):]
	}
	s = strings.TrimPrefix(s, "@")
	if i := strings.IndexAny(s, "/?# "); i >= 0 {
		s = s[:i]
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_') {
			return ""
		}
	}
	if len(s) < 2 || len(s) > 30 {
		return ""
	}
	return s
}

// rt returns (creating if needed) the live record of a listed channel. Call with e.mu held.
func (e *Engine) rt(login string) *chanRT {
	ch := e.chans[login]
	if ch == nil {
		ch = &chanRT{Login: login, Name: login}
		e.chans[login] = ch
	}
	return ch
}

func (e *Engine) isListed(ch *chanRT) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, c := range e.set.Channels {
		if c.Login == ch.Login && c.On {
			return e.chans[c.Login] == ch
		}
	}
	return false
}

func (e *Engine) enabledChannels() []*chanRT {
	e.mu.Lock()
	defer e.mu.Unlock()
	var out []*chanRT
	for _, c := range e.set.Channels {
		if c.On {
			out = append(out, e.rt(c.Login))
		}
	}
	return out
}

func (e *Engine) applyStream(ch *chanRT, user any) {
	if user == nil {
		ch.Missing, ch.Online = true, false
		return
	}
	ch.Missing = false
	if id := str(dig(user, "id")); id != "" {
		ch.ID = id
	}
	if n := str(dig(user, "displayName")); n != "" {
		ch.Name = n
	}
	if img := str(dig(user, "profileImageURL")); img != "" {
		ch.Img = img
	}
	if e.chans[ch.Login] == ch {
		e.rememberFace(e.set.Channels, ch.Login, ch.Name, ch.Img)
	}
	stream := dig(user, "stream")
	was := ch.Online
	if stream == nil {
		ch.Online, ch.StreamID, ch.Viewers = false, "", 0
		ch.playlist = ""
	} else {
		ch.Online = true
		ch.StreamID = str(dig(stream, "id"))
		ch.Viewers = num(dig(stream, "viewersCount"))
		game := dig(user, "broadcastSettings", "game")
		ch.GameID = str(dig(game, "id"))
		ch.Game = str(dig(game, "displayName"))
		if ch.Game == "" {
			ch.Game = str(dig(game, "name"))
		}
		ch.gone = 0
	}
	if was != ch.Online && e.chans[ch.Login] == ch {
		if ch.Online {
			e.log.Info("L.live", ch.Name)
		} else {
			e.log.Info("L.ended", ch.Name)
		}
	}
}

// streamInfo reads live status for a set of channels in batches.
func (e *Engine) streamInfo(chs []*chanRT) bool {
	if len(chs) == 0 {
		return true
	}
	ops := make([]gqlOp, len(chs))
	for i, ch := range chs {
		ops[i] = gqlOp{"VideoPlayerStreamInfoOverlayChannel", map[string]any{"channel": ch.Login}}
	}
	res, err := e.c.gqlBatch(ops)
	e.gqlErr("stream info", err)
	if res == nil {
		return false
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	for i, ch := range chs {
		if res[i] == nil {
			continue
		}
		e.applyStream(ch, res[i]["user"])
	}
	return true
}

func (e *Engine) checkOnline() {
	e.streamInfo(e.enabledChannels())
}

// ---- which (at most two) channels are being watched ----

func (e *Engine) decideSlots(now time.Time) {
	e.mu.Lock()
	defer e.mu.Unlock()
	var want []*chanRT
	if e.drops.cur != nil && e.set.DropsOn {
		want = append(want, e.drops.cur.ch)
	}
	if e.set.PointsOn && !(e.drops.cur != nil && e.drops.cur.solo && e.set.DropsOn) {
		var online []*chanRT
		for _, c := range e.set.Channels {
			if ch := e.chans[c.Login]; c.On && ch != nil && ch.Online && !ch.Missing {
				online = append(online, ch)
			}
		}
		free := 2 - len(want)
		if e.set.RotateMin > 0 && len(online) > free && free > 0 {
			if e.rotateStart.IsZero() {
				e.rotateStart = now
			}
			shift := int(now.Sub(e.rotateStart)/iv(time.Duration(e.set.RotateMin)*time.Minute)) % len(online)
			online = append(online[shift:], online[:shift]...)
		}
		for _, ch := range online {
			if len(want) >= 2 {
				break
			}
			dup := false
			for _, w := range want {
				if w == ch || (w.ID != "" && w.ID == ch.ID) || w.Login == ch.Login {
					dup = true
				}
			}
			if !dup {
				want = append(want, ch)
			}
		}
	}
	for _, old := range e.slots {
		keep := false
		for _, w := range want {
			if w == old {
				keep = true
			}
		}
		if !keep {
			old.Watching = false
		}
	}
	for _, w := range want {
		if !w.Watching {
			w.Watching = true
			w.gainAt, w.noGain = now, false
			w.lastTouch, w.lastOK = time.Time{}, time.Time{}
			w.fails = 0
		}
	}
	e.slots = want
}

// watchOnce performs one "I am watching" round for a channel.
func (e *Engine) watchOnce(ch *chanRT, now time.Time) {
	ch.lastTouch = now
	if ch.playlist == "" || now.Sub(ch.playlistAt) > iv(25*time.Minute) {
		u, err := e.c.playlistURL(ch.Login)
		if err != nil {
			if errors.Is(err, errAuth) {
				e.authLost()
				return
			}
			if errors.Is(err, errGone) {
				e.streamGone(ch)
				return
			}
			e.watchFail(ch, err)
			return
		}
		ch.playlist, ch.playlistAt = u, now
	}
	gone, err := e.c.touchStream(ch.playlist)
	if gone {
		ch.playlist = ""
		e.streamGone(ch)
		return
	}
	if err != nil {
		e.watchFail(ch, err)
		return
	}
	e.mu.Lock()
	if !ch.lastOK.IsZero() {
		if d := now.Sub(ch.lastOK); d < iv(90*time.Second) {
			ch.WatchedSec += int64(d * time.Duration(speed) / time.Second)
		}
	}
	ch.lastOK, ch.fails, ch.gone, ch.Err = now, 0, 0, ""
	e.mu.Unlock()

	if now.Sub(ch.lastSpade) >= iv(time.Minute) {
		if ch.spade == "" && now.Sub(ch.spadeTried) > iv(10*time.Minute) {
			ch.spadeTried = now
			if u, err := e.c.spadeURL(ch.Login); err == nil {
				ch.spade = u
			} else {
				e.log.Debug("spade %s: %v", ch.Login, err)
			}
		}
		if ch.spade != "" && ch.ID != "" {
			ch.lastSpade = now
			if err := e.c.minuteWatched(ch.spade, watchInfo{ChannelID: ch.ID, Login: ch.Login, BroadcastID: ch.StreamID, Game: ch.Game, GameID: ch.GameID}); err != nil {
				e.log.Debug("minute-watched %s: %v", ch.Login, err)
				ch.spade = "" // look it up again later
			}
		}
	}
}

// streamGone: the stream's playlist is not there. Twice in a row means the stream has ended.
func (e *Engine) streamGone(ch *chanRT) {
	e.mu.Lock()
	defer e.mu.Unlock()
	ch.gone++
	if ch.gone >= 2 && ch.Online {
		ch.Online = false
		if e.chans[ch.Login] == ch {
			e.log.Info("L.ended", ch.Name)
		}
	}
}

func (e *Engine) watchFail(ch *chanRT, err error) {
	e.mu.Lock()
	ch.fails++
	ch.Err = err.Error()
	fails := ch.fails
	if fails <= 3 {
		e.log.Debug("watch %s: %v", ch.Login, err)
	}
	if fails == 3 {
		e.log.Warn("L.keepFail", ch.Name, err)
	}
	if fails >= 3 {
		ch.playlist = ""
	}
	e.mu.Unlock()
	if fails >= 3 && fails%3 == 0 {
		e.streamInfo([]*chanRT{ch}) // is it still live at all?
	}
}

// ---- channel points ----

func (e *Engine) applyPoints(ch *chanRT, data map[string]any) (claimID string) {
	community := dig(data, "community")
	if community == nil {
		return ""
	}
	if id := str(dig(community, "id")); id != "" && ch.ID == "" {
		ch.ID = id
	}
	cp := dig(community, "channel", "self", "communityPoints")
	if cp == nil {
		return ""
	}
	bal := num(dig(cp, "balance"))
	if ch.BalKnown && bal > ch.Balance {
		ch.Earned += bal - ch.Balance
		ch.gainAt, ch.noGain = time.Now(), false
	}
	ch.Balance, ch.BalKnown = bal, true
	return str(dig(cp, "availableClaim", "id"))
}

// noGainAfter: watching for this long without a single point is reported in the journal.
const noGainAfter = 25 * time.Minute

func (e *Engine) pointsContext(ch *chanRT, now time.Time) {
	ch.lastCtx = now
	data, err := e.c.gqlOne("ChannelPointsContext", map[string]any{"channelLogin": ch.Login})
	if err != nil {
		e.gqlErr("points "+ch.Login, err)
		return
	}
	e.mu.Lock()
	claim := e.applyPoints(ch, data)
	id := ch.ID
	// Watching should bring +10 every five minutes. When it brings nothing for a long time although every
	// request succeeds, say so: that is how a change on Twitch's side shows up, and it is silent otherwise.
	silent := ch.Watching && ch.BalKnown && !ch.noGain && !ch.gainAt.IsZero() && now.Sub(ch.gainAt) > iv(noGainAfter)
	if silent {
		ch.noGain = true
	}
	name := ch.Name
	e.mu.Unlock()
	if silent {
		e.log.Warn("L.noPoints", name, int(noGainAfter/time.Minute))
	}
	if claim == "" || id == "" {
		return
	}
	_, err = e.c.gqlOne("ClaimCommunityPoints", map[string]any{"input": map[string]any{"claimID": claim, "channelID": id}})
	if err != nil {
		e.gqlErr("claim bonus "+ch.Login, err)
		e.log.Warn("L.bonusFail", ch.Name)
		return
	}
	e.mu.Lock()
	ch.Claims++
	e.mu.Unlock()
	e.log.OK("L.bonus", ch.Name)
	ch.lastCtx = now.Add(-iv(45 * time.Second)) // read the new balance soon
}

// refreshBalances reads point balances for channels that are not being watched right now.
func (e *Engine) refreshBalances(now time.Time) {
	var chs []*chanRT
	for _, ch := range e.enabledChannels() {
		if !ch.Watching && !ch.Missing {
			chs = append(chs, ch)
		}
	}
	if len(chs) == 0 {
		return
	}
	ops := make([]gqlOp, len(chs))
	for i, ch := range chs {
		ops[i] = gqlOp{"ChannelPointsContext", map[string]any{"channelLogin": ch.Login}}
	}
	res, err := e.c.gqlBatch(ops)
	e.gqlErr("balances", err)
	if res == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	for i, ch := range chs {
		if res[i] != nil {
			e.applyPoints(ch, res[i])
			ch.lastCtx = now
		}
	}
}

// ---- operations from the panel ----

// rememberFace keeps a channel's display name and avatar in the settings, so that the list shows them
// right after a start and for channels that are switched off. Call with e.mu held.
func (e *Engine) rememberFace(list []ChanCfg, login, name, img string) {
	for i := range list {
		c := &list[i]
		if c.Login != login {
			continue
		}
		if (name != "" && name != login && c.Name != name) || (img != "" && c.Img != img) {
			if name != "" && name != login {
				c.Name = name
			}
			if img != "" {
				c.Img = img
			}
			e.saveSettings()
		}
		return
	}
}

// face cleans what the panel passes along with a channel picked from the suggestions.
func face(name, img string) (string, string) {
	name = strings.TrimSpace(name)
	if len(name) > 60 {
		name = ""
	}
	if !strings.HasPrefix(img, "https://") || len(img) > 400 {
		img = ""
	}
	return name, img
}

func (e *Engine) addChannel(raw, name, img string) string {
	login := normLogin(raw)
	if login == "" {
		return "bad"
	}
	name, img = face(name, img)
	e.mu.Lock()
	for _, c := range e.set.Channels {
		if c.Login == login {
			e.mu.Unlock()
			return "dup"
		}
	}
	e.set.Channels = append(e.set.Channels, ChanCfg{Login: login, On: true, Name: name, Img: img})
	e.rt(login)
	e.saveSettings()
	e.nextOnline, e.nextBalances = time.Time{}, time.Now().Add(iv(3*time.Second))
	e.mu.Unlock()
	e.chat.sync()
	e.poke()
	return "ok"
}

func (e *Engine) removeChannel(login string) {
	e.mu.Lock()
	out := e.set.Channels[:0]
	for _, c := range e.set.Channels {
		if c.Login != login {
			out = append(out, c)
		}
	}
	e.set.Channels = out
	if ch := e.chans[login]; ch != nil {
		ch.Watching = false
	}
	delete(e.chans, login)
	e.saveSettings()
	e.mu.Unlock()
	e.chat.sync()
	e.poke()
}

func (e *Engine) toggleChannel(login string, on bool) {
	e.mu.Lock()
	for i := range e.set.Channels {
		if e.set.Channels[i].Login == login {
			e.set.Channels[i].On = on
		}
	}
	e.saveSettings()
	e.nextOnline = time.Time{}
	e.mu.Unlock()
	e.chat.sync()
	e.poke()
}

func (e *Engine) reorderChannels(order []string) {
	e.mu.Lock()
	pos := map[string]int{}
	for i, l := range order {
		pos[l] = i + 1
	}
	sort.SliceStable(e.set.Channels, func(a, b int) bool {
		pa, pb := pos[e.set.Channels[a].Login], pos[e.set.Channels[b].Login]
		if pa == 0 {
			pa = 1 << 30
		}
		if pb == 0 {
			pb = 1 << 30
		}
		return pa < pb
	})
	e.rotateStart = time.Time{}
	e.saveSettings()
	e.mu.Unlock()
	e.poke()
}
