package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

//go:embed ui/*
var uiFS embed.FS

type server struct {
	e    *Engine
	key  string
	port int
}

func ms(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixMilli()
}

// view builds everything the panel shows.
func (e *Engine) view() map[string]any {
	now := time.Now()
	chatState, chatJoined := e.chat.status()
	kickView := e.kick.view()
	e.mu.Lock()
	defer e.mu.Unlock()
	kickView["login"], kickView["loginNote"] = e.kickLoginOn, e.kickLoginNote

	chans := []map[string]any{}
	var earned int64
	claims, watching := 0, 0
	for _, c := range e.set.Channels {
		ch := e.rt(c.Login)
		earned += ch.Earned
		claims += ch.Claims
		if ch.Watching {
			watching++
		}
		name := ch.Name
		if name == ch.Login && c.Name != "" {
			name = c.Name
		}
		chans = append(chans, map[string]any{
			"login": ch.Login, "name": name, "img": firstNonEmpty(ch.Img, c.Img), "on": c.On, "online": ch.Online, "watching": ch.Watching,
			"game": ch.Game, "balance": ch.Balance, "balKnown": ch.BalKnown, "earned": ch.Earned, "claims": ch.Claims,
			"missing": ch.Missing, "viewers": ch.Viewers, "watchedMin": ch.WatchedSec / 60,
		})
	}

	dropView := func(d *Drop) map[string]any {
		return map[string]any{"id": d.ID, "name": d.Name, "img": d.Img, "cur": d.Cur, "req": d.Req, "claimed": d.Claimed}
	}
	_ = dropView
	gameDrops := func(g GameRef) (drops []map[string]any, link string, end int64) {
		for _, c := range e.gameCamps(g, now) {
			if !c.Linked && link == "" {
				link = c.LinkURL
			}
			if end == 0 || ms(c.End) < end {
				end = ms(c.End)
			}
			for _, d := range c.Drops {
				if d.Req > 0 {
					drops = append(drops, dropView(d))
				}
			}
		}
		if drops == nil {
			drops = []map[string]any{}
		}
		return
	}
	gameDetail := func(g GameRef) []map[string]any { return e.campViews(g, now) }
	queuePos := func(g GameRef) int {
		for i, q := range e.set.Queue {
			if sameGame(q, g) {
				return i + 1
			}
		}
		return 0
	}
	isTracked := func(g GameRef) bool {
		for _, t := range e.set.Tracked {
			if sameGame(t, g) {
				return true
			}
		}
		return false
	}
	entry := func(g GameRef) map[string]any {
		drops, link, end := gameDrops(g)
		if g.Art == "" { // older entries may lack the cover: take it from a campaign of that game
			for _, c := range e.drops.camps {
				if sameGame(c.Game, g) && c.Game.Art != "" {
					g.Art = c.Game.Art
					break
				}
			}
		}
		st := e.gameStatus(g, now)
		note := ""
		farming := e.drops.cur != nil && sameGame(e.drops.cur.game, g)
		channel := ""
		if farming {
			channel = e.drops.cur.ch.Name
		} else if st == "ready" {
			note = e.drops.gameNote[g.ID]
		}
		return map[string]any{"id": g.ID, "slug": g.Slug, "name": g.Name, "art": g.Art, "status": st, "note": note, "next": ms(e.nextRound(g, now)),
			"farming": farming, "channel": channel, "drops": drops, "camps": gameDetail(g), "link": link, "end": end, "tracked": isTracked(g), "queued": queuePos(g)}
	}

	// campaign list grouped by game
	type agg struct {
		g     GameRef
		n     int
		end   time.Time
		name  string
		start time.Time
		names []string
		quest []*Campaign // Twitch's own reward campaigns among them: these get a banner of their own
	}
	games := map[string]*agg{}
	for _, c := range e.drops.camps {
		if !now.Before(c.End) || c.Game.ID == "" {
			continue
		}
		timed := false
		for _, d := range c.Drops {
			if d.Req > 0 {
				timed = true
			}
		}
		if !timed {
			continue
		}
		a := games[c.Game.ID]
		if a == nil {
			a = &agg{g: c.Game, end: c.End, name: c.Name, start: c.Start}
			games[c.Game.ID] = a
		}
		a.names = append(a.names, c.Name)
		if c.Quest {
			a.quest = append(a.quest, c)
		}
		if len(c.Cats) > 0 && a.name == c.Game.Name { // listed under its own name: say where it counts instead of repeating it
			a.name = fmt.Sprintf("%s +%d", c.Cats[0].Name, len(c.Cats)-1)
		}
		a.n++
		if c.End.Before(a.end) {
			a.end = c.End
		}
		if c.Start.Before(a.start) {
			a.start = c.Start
		}
		if a.g.Slug == "" {
			a.g.Slug = c.Game.Slug
		}
	}
	list := []map[string]any{}
	for _, a := range games {
		row := map[string]any{"id": a.g.ID, "slug": a.g.Slug, "name": a.g.Name, "art": a.g.Art, "camps": a.n, "camp": a.name, "names": strings.Join(a.names, " | "),
			"end": ms(a.end), "upcoming": now.Before(a.start), "start": ms(a.start), "status": e.gameStatus(a.g, now), "queued": queuePos(a.g), "tracked": isTracked(a.g), "next": ms(e.nextRound(a.g, now))}
		if len(a.quest) > 0 {
			sort.Slice(a.quest, func(i, j int) bool {
				if !a.quest[i].End.Equal(a.quest[j].End) {
					return a.quest[i].End.Before(a.quest[j].End)
				}
				return a.quest[i].ID < a.quest[j].ID
			})
			first := a.quest[0]
			where := first.Game.Name // the categories it counts in
			if len(first.Cats) > 1 {
				where = fmt.Sprintf("%s +%d", first.Cats[0].Name, len(first.Cats)-1)
			}
			rewards := []map[string]any{}
			for _, c := range a.quest {
				for _, d := range c.Drops {
					if d.Req > 0 {
						rewards = append(rewards, map[string]any{"id": d.ID, "name": d.Name, "img": d.Img, "cur": d.Cur, "req": d.Req, "claimed": d.Claimed,
							"locked": d.Later && !d.finished(), "day": d.Day, "next": ms(d.Next), "days": d.Days, "done": d.Done, "until": ms(d.Until)})
					}
				}
			}
			row["quest"], row["title"], row["where"], row["rewards"] = true, first.Name, where, rewards
		}
		list = append(list, row)
	}
	sort.Slice(list, func(i, j int) bool {
		return strings.ToLower(list[i]["name"].(string)) < strings.ToLower(list[j]["name"].(string))
	})

	queue := []map[string]any{}
	for _, g := range e.set.Queue {
		queue = append(queue, entry(g))
	}
	tracked := []map[string]any{}
	for _, g := range e.set.Tracked {
		tracked = append(tracked, entry(g))
	}
	var current any
	if f := e.drops.cur; f != nil {
		art := f.game.Art
		for _, c := range e.drops.camps {
			if art == "" && sameGame(c.Game, f.game) {
				art = c.Game.Art
			}
		}
		cur := map[string]any{"game": f.game.Name, "art": art, "channel": f.ch.Name, "login": f.ch.Login, "since": ms(f.started), "drop": "", "cur": 0, "req": 0, "gameId": f.game.ID, "dropId": f.dropID}
		for _, c := range e.drops.camps {
			if d := c.drop(f.dropID); d != nil && f.dropID != "" {
				cur["drop"], cur["cur"], cur["req"], cur["img"] = d.Name, d.Cur, d.Req, d.Img
			}
		}
		// the whole campaign of the game being watched: every drop with its picture and state, so the
		// overview shows it too (whether the game came from the queue or from the tracked games)
		var camps []map[string]any
		for _, c := range e.campViews(f.game, now) {
			if c["active"] == true {
				camps = append(camps, c)
			}
		}
		cur["camps"] = camps
		current = cur
	}

	return map[string]any{
		"version": version, "usage": currentUsage(), "statsOn": statsURL != "",
		"app": appName, "closing": e.closing, "dataDir": e.dir, "dataDefault": dataDirIn(defaultRoot()), "dataCustom": readLocation().Root != "",
		"lang":   resolveLang(e.set.Lang),
		"update": map[string]any{"state": e.upd.State, "latest": e.upd.Latest, "checked": ms(e.upd.Checked), "doneAt": ms(e.upd.Done), "from": e.upd.From, "got": updGot.Load(), "size": updSize.Load()},
		"auth":   map[string]any{"state": e.authState, "login": e.auth.Login, "note": e.authNote, "mode": e.loginMode},
		"settings": map[string]any{"pointsOn": e.set.PointsOn, "chatOn": e.set.ChatOn, "dropsOn": e.set.DropsOn, "autoClaim": e.set.AutoClaim,
			"rotateMin": e.set.RotateMin, "stallMin": e.set.StallMin, "autostart": e.set.Autostart, "openOnStart": e.set.OpenOnStart,
			"kickOn": e.set.KickOn, "kickMax": e.set.KickMax, "autoUpdate": e.set.AutoUpdate, "stats": e.set.Stats, "lang": e.set.Lang,
			"rpcOn": e.set.Discord.On, "rpcDrop": e.set.Discord.Drop, "rpcQueue": e.set.Discord.Queue, "rpcTracked": e.set.Discord.Tracked,
			"rpcPoints": e.set.Discord.Points, "rpcBtn": e.set.Discord.Button},
		"rpc":    map[string]any{"state": e.rpcState, "ready": firstNonEmpty(e.set.Discord.AppID, discordAppID) != "", "err": e.rpcErr, "test": max(0, time.Until(e.rpcTestUntil).Milliseconds())},
		"points": map[string]any{"channels": chans, "earned": earned, "claims": claims, "watching": watching},
		"chat":   map[string]any{"state": chatState, "joined": chatJoined},
		"kick":   kickView,
		"drops":  map[string]any{"fetchedAt": ms(e.drops.fetchedAt), "err": e.drops.fetchErr, "games": list, "queue": queue, "tracked": tracked, "current": current},
		"log":    e.log.tail(),
		"winGen": e.winGen,
		"upSec":  int64(now.Sub(e.started) / time.Second),
	}
}

// campViews describes a game's campaigns in full (running first, then upcoming). Call with e.mu held.
func (e *Engine) campViews(g GameRef, now time.Time) []map[string]any {
	var camps []*Campaign
	for _, c := range e.drops.camps {
		if sameGame(c.Game, g) && now.Before(c.End) {
			camps = append(camps, c)
		}
	}
	sort.Slice(camps, func(i, j int) bool {
		ai, aj := camps[i].active(now), camps[j].active(now)
		if ai != aj {
			return ai
		}
		return camps[i].End.Before(camps[j].End)
	})
	out := []map[string]any{}
	for _, c := range camps {
		names := []string{}
		for i, a := range c.Channels {
			if i >= 14 {
				break
			}
			names = append(names, firstNonEmpty(a.Name, a.Login))
		}
		drops := []map[string]any{}
		var left int64
		for _, d := range c.Drops {
			if d.Req <= 0 {
				continue
			}
			pre := []string{}
			blocked := false
			for _, p := range d.Pre {
				if pd := c.drop(p); pd != nil {
					pre = append(pre, pd.Name)
					if !pd.Claimed {
						blocked = true
					}
				}
			}
			if !d.Claimed && d.Cur < d.Req {
				left += d.Req - d.Cur
			}
			drops = append(drops, map[string]any{"id": d.ID, "name": d.Name, "img": d.Img, "cur": d.Cur, "req": d.Req, "claimed": d.Claimed,
				"pre": pre, "blocked": (blocked || d.Later) && !d.finished(), "end": ms(d.End), "day": d.Day, "next": ms(d.Next), "days": d.Days, "done": d.Done, "until": ms(d.Until)})
		}
		out = append(out, map[string]any{"id": c.ID, "name": c.Name, "start": ms(c.Start), "end": ms(c.End), "active": c.active(now),
			"linked": c.Linked, "link": c.LinkURL, "channels": names, "channelCount": len(c.Channels), "drops": drops, "left": left, "quest": c.Quest})
	}
	return out
}

// gameView is the detail page of one game.
func (e *Engine) gameView(id string) map[string]any {
	now := time.Now()
	e.mu.Lock()
	defer e.mu.Unlock()
	var g GameRef
	for _, c := range e.drops.camps {
		if c.Game.ID == id {
			g = c.Game
			break
		}
	}
	if g.ID == "" {
		return map[string]any{"id": id, "camps": []any{}}
	}
	queued, tracked := 0, false
	for i, q := range e.set.Queue {
		if sameGame(q, g) {
			queued = i + 1
		}
	}
	for _, t := range e.set.Tracked {
		if sameGame(t, g) {
			tracked = true
		}
	}
	return map[string]any{"id": g.ID, "slug": g.Slug, "name": g.Name, "art": g.Art, "status": e.gameStatus(g, now),
		"queued": queued, "tracked": tracked, "camps": e.campViews(g, now)}
}

// statusLine is the tray tooltip.
func (e *Engine) statusLine() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.authState != "ok" {
		return appName + " – " + tr("notSigned")
	}
	var names []string
	for _, ch := range e.slots {
		names = append(names, ch.Name)
	}
	if len(names) == 0 {
		return appName + " – " + tr("status.wait")
	}
	s := appName + " – " + tr("status.watching") + " " + strings.Join(names, ", ")
	if len(s) > 120 {
		s = s[:120]
	}
	return s
}

type opReq struct {
	Op    string          `json:"op"`
	Login string          `json:"login"`
	Name  string          `json:"name"`
	Img   string          `json:"img"`
	On    bool            `json:"on"`
	Order []string        `json:"order"`
	Game  GameRef         `json:"game"`
	Patch json.RawMessage `json:"patch"`
}

func (s *server) handleOp(req opReq) map[string]any {
	e := s.e
	out := map[string]any{"ok": true}
	switch req.Op {
	case "login":
		e.startLogin(false)
	case "loginManual":
		e.startLogin(true)
	case "kickLogin":
		e.kickLogin()
	case "kickLoginCancel":
		e.kickLoginCancel()
	case "cancelLogin":
		e.cancelLogin()
	case "token":
		out["result"] = e.submitToken(req.Login)
	case "logout":
		e.logout()
	case "addChannel":
		out["result"] = e.addChannel(req.Login, req.Name, req.Img)
	case "removeChannel":
		e.removeChannel(req.Login)
	case "toggleChannel":
		e.toggleChannel(req.Login, req.On)
	case "reorderChannels":
		e.reorderChannels(req.Order)
	case "kickToken":
		out["result"] = e.kickSetToken(req.Login)
	case "kickLogout":
		e.kickLogout()
	case "kickAdd":
		out["result"] = e.kickAdd(req.Login, req.Name, req.Img)
	case "kickRemove":
		e.kickRemove(req.Login)
	case "kickToggle":
		e.kickToggle(req.Login, req.On)
	case "kickReorder":
		e.kickReorder(req.Order)
	case "queueToggle":
		e.queueToggle(req.Game)
	case "queueReorder":
		e.queueReorder(req.Order)
	case "trackToggle":
		e.trackToggle(req.Game)
	case "refresh":
		e.mu.Lock()
		e.drops.force = true
		e.nextOnline, e.nextBalances = time.Time{}, time.Time{}
		e.mu.Unlock()
		e.poke()
	case "settings":
		var p map[string]any
		json.Unmarshal(req.Patch, &p)
		e.mu.Lock()
		for k, v := range p {
			switch k {
			case "pointsOn":
				e.set.PointsOn = boolean(v)
			case "chatOn":
				e.set.ChatOn = boolean(v)
			case "dropsOn":
				e.set.DropsOn = boolean(v)
				e.drops.nextPick = time.Time{}
			case "autoClaim":
				e.set.AutoClaim = boolean(v)
				e.drops.claimFail = map[string]int{}
				if e.set.AutoClaim {
					e.drops.force = true
				}
			case "kickOn":
				e.set.KickOn = boolean(v)
			case "kickMax":
				e.set.KickMax = int(min(max(num(v), 1), 5))
			case "openOnStart":
				e.set.OpenOnStart = boolean(v)
			case "rpcOn":
				e.set.Discord.On = boolean(v)
			case "rpcDrop":
				e.set.Discord.Drop = boolean(v)
			case "rpcQueue":
				e.set.Discord.Queue = boolean(v)
			case "rpcTracked":
				e.set.Discord.Tracked = boolean(v)
			case "rpcPoints":
				e.set.Discord.Points = boolean(v)
			case "rpcBtn":
				e.set.Discord.Button = boolean(v)
			case "autoUpdate":
				e.set.AutoUpdate = boolean(v)
			case "stats":
				e.set.Stats = boolean(v)
			case "lang":
				e.set.Lang = "auto"
				for _, c := range langCodes {
					if c == str(v) {
						e.set.Lang = c
					}
				}
				curLang.Store(resolveLang(e.set.Lang))
			case "rotateMin":
				e.set.RotateMin = int(min(max(num(v), 0), 720))
				e.rotateStart = time.Time{}
			case "stallMin":
				e.set.StallMin = int(min(max(num(v), 5), 120))
			case "autostart":
				on := boolean(v)
				if err := setAutostart(on); err != nil {
					e.log.Warn("L.autostart", err)
				} else {
					e.set.Autostart = on
				}
			}
		}
		e.saveSettings()
		e.mu.Unlock()
		e.chat.sync()
		e.poke()
	case "updateCheck":
		go e.updateCheck(false)
	case "updateInstall":
		go e.updateCheck(true)
	case "clearLog":
		e.log.clear()
	case "quit":
		if req.On { // a newer version takes over: the windows stay and carry on with it
			go func() {
				time.Sleep(200 * time.Millisecond)
				e.shutdown()
			}()
		} else {
			go e.quitApp()
		}
	case "rpcTest": // show a test status in Discord for 30 seconds
		e.mu.Lock()
		e.rpcTestUntil = time.Now().Add(30 * time.Second)
		e.mu.Unlock()
		e.log.Info("L.rpcTest")
	case "dataOpen":
		openFolder(e.dir)
	case "dataPick": // the system folder picker; the program's own folder is made inside the one picked
		p := pickFolder(tr("set.dataDir"), filepath.Dir(e.dir))
		if p == "" {
			return map[string]any{"ok": false}
		}
		return map[string]any{"ok": true, "path": p, "target": dataDirIn(p)}
	case "dataMove":
		if os.Getenv("DROPOMAT_DATA") != "" {
			return map[string]any{"ok": false, "err": "bad"}
		}
		dir, err := chooseRoot(req.Name, e.dir)
		if err != nil {
			return map[string]any{"ok": false, "err": err.Error()}
		}
		e.log.Info("L.dataMoving", dir)
		go func() {
			time.Sleep(400 * time.Millisecond)
			if err := e.restartSelf("--moved"); err != nil {
				e.log.Err("L.dataMoveFail", err)
			}
		}()
		return map[string]any{"ok": true, "target": dir}
	default:
		out["ok"] = false
	}
	return out
}

func (e *Engine) shutdown() {
	select {
	case <-e.quit:
	default:
		close(e.quit)
	}
	if e.loginMu.TryLock() {
		if lw := e.lw; lw != nil { // do not leave the sign-in window behind
			lw.mu.Lock()
			c := lw.c
			lw.mu.Unlock()
			if c != nil {
				c.call("Browser.close", nil, "")
			}
		}
		e.loginMu.Unlock()
	}
	if e.onQuit != nil {
		e.onQuit()
	}
}

func (s *server) guard(w http.ResponseWriter, r *http.Request) bool {
	host := r.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	if host != "127.0.0.1" && host != "localhost" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return false
	}
	if r.Header.Get("X-Dropomat-Key") != s.key {
		http.Error(w, "forbidden", http.StatusForbidden)
		return false
	}
	return true
}

func (s *server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/state", func(w http.ResponseWriter, r *http.Request) {
		if !s.guard(w, r) {
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		json.NewEncoder(w).Encode(s.e.view())
	})
	mux.HandleFunc("/api/op", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || !s.guard(w, r) {
			if r.Method != "POST" {
				http.Error(w, "method", http.StatusMethodNotAllowed)
			}
			return
		}
		raw, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		var req opReq
		if json.Unmarshal(raw, &req) != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(s.handleOp(req))
	})
	mux.HandleFunc("/api/game", func(w http.ResponseWriter, r *http.Request) {
		if !s.guard(w, r) {
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		json.NewEncoder(w).Encode(s.e.gameView(r.URL.Query().Get("id")))
	})
	// suggestions while typing: Twitch channels, Kick channels, games
	mux.HandleFunc("/api/search", func(w http.ResponseWriter, r *http.Request) {
		if !s.guard(w, r) {
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		items, err := s.e.search(r.URL.Query().Get("kind"), r.URL.Query().Get("q"))
		json.NewEncoder(w).Encode(map[string]any{"items": items, "failed": err != nil})
	})
	// a second start of the program asks the running one to show its panel
	mux.HandleFunc("/api/show", func(w http.ResponseWriter, r *http.Request) {
		if !s.guard(w, r) {
			return
		}
		s.show()
		fmt.Fprint(w, "ok")
	})
	mux.HandleFunc("/changelog.md", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Write(changelog)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
		if host != "127.0.0.1" && host != "localhost" { // the page carries the key: local addresses only
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/")
		if name == "" || name == "panel" {
			name = "index.html"
		}
		raw, err := uiFS.ReadFile("ui/" + name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		switch {
		case strings.HasSuffix(name, ".html"):
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store")
			raw = []byte(strings.ReplaceAll(string(raw), "__KEY__", s.key))
		case strings.HasSuffix(name, ".json"):
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store")
		case strings.HasSuffix(name, ".png"):
			w.Header().Set("Content-Type", "image/png")
		case strings.HasSuffix(name, ".ico"):
			w.Header().Set("Content-Type", "image/x-icon")
		}
		w.Write(raw)
	})
	return mux
}

// show opens the panel window; a panel that is already open closes itself when it sees the new generation.
func (s *server) show() {
	s.e.mu.Lock()
	s.e.winGen++
	gen := s.e.winGen
	s.e.mu.Unlock()
	if err := openPanel(fmt.Sprintf("http://127.0.0.1:%d/panel?g=%d", s.port, gen), s.e.dir); err != nil {
		s.e.log.Err("L.panelFail", err)
	}
}
