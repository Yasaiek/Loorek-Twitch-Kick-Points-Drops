package main

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Discord Rich Presence. Loorek tells the Discord client running on the same computer what it is doing
// (the drop being farmed, the queue, the tracked games, the channels points are collected on), and Discord
// shows it on the user's profile. It talks to the client over its local pipe - nothing goes to Discord's
// servers from here, and nothing is shown unless the user switches it on.

// discordAppID is Loorek's application in the Discord Developer Portal. It names the activity ("Loorek")
// and holds its pictures. For trying another application, "discord": {"appId": "..."} in settings.json wins.
const discordAppID = "1572656033141096670"

// The pictures uploaded to that application under Rich Presence > Art Assets.
const (
	rpcLogo   = "logo"
	rpcDrops  = "drops"
	rpcPoints = "points"
	rpcIdle   = "idle"
)

const (
	siteURL = "https://yasaiek.github.io/Loorek-Twitch-Kick-Points-Drops/"
	repoURL = "https://github.com/Yasaiek/Loorek-Twitch-Kick-Points-Drops"
)

// RPCCfg: what the user lets Discord show.
type RPCCfg struct {
	On      bool   `json:"on"`
	Drop    bool   `json:"drop"`            // the drop being farmed: game, reward, progress
	Queue   bool   `json:"queue"`           // the drops queue
	Tracked bool   `json:"tracked"`         // the tracked games
	Points  bool   `json:"points"`          // the channels points are collected on
	Button  bool   `json:"button"`          // the link buttons: "Get Loorek" (the site) and "GitHub"
	AppID   string `json:"appId,omitempty"` // not in the panel: only for trying another application
}

type rpcActivity struct {
	Type       int         `json:"type"` // 3 = "Watching"
	Details    string      `json:"details,omitempty"`
	State      string      `json:"state,omitempty"`
	Timestamps *rpcTimes   `json:"timestamps,omitempty"`
	Assets     *rpcAssets  `json:"assets,omitempty"`
	Buttons    []rpcButton `json:"buttons,omitempty"`
	Instance   bool        `json:"instance"`
}

type rpcTimes struct {
	Start int64 `json:"start"`
}

type rpcAssets struct {
	LargeImage string `json:"large_image,omitempty"`
	LargeText  string `json:"large_text,omitempty"`
	SmallImage string `json:"small_image,omitempty"`
	SmallText  string `json:"small_text,omitempty"`
}

type rpcButton struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

// rpcText fits a text into what Discord accepts for one field: 2 to 128 characters.
func rpcText(s string) string {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) > 128 {
		r := []rune(s)
		s = strings.TrimSpace(string(r[:127])) + "…"
	}
	if utf8.RuneCountInString(s) < 2 {
		return ""
	}
	return s
}

// rpcNames joins names into a list that fits in about max characters: "A, B, C +2".
func rpcNames(names []string, max int) string {
	out, n := "", 0
	for i, name := range names {
		add := name
		if i > 0 {
			add = ", " + name
		}
		if i > 0 && utf8.RuneCountInString(out)+utf8.RuneCountInString(add) > max {
			break
		}
		out += add
		n++
	}
	if rest := len(names) - n; rest > 0 {
		out += " +" + strconv.Itoa(rest)
	}
	return out
}

// presence builds what Discord should show right now; nil when there is nothing to show (switched off, no id).
func (e *Engine) presence(now time.Time) (cfg RPCCfg, id string, act *rpcActivity) {
	kick := e.kick.watchedNames()
	e.mu.Lock()
	defer e.mu.Unlock()
	cfg = e.set.Discord
	id = firstNonEmpty(strings.TrimSpace(cfg.AppID), discordAppID)
	if !cfg.On || id == "" {
		return cfg, id, nil
	}

	// What there is to tell.
	var queue, tracked []string
	for _, g := range e.set.Queue {
		queue = append(queue, g.Name)
	}
	for _, g := range e.set.Tracked {
		tracked = append(tracked, g.Name)
	}
	var twitch []string
	if e.set.PointsOn {
		for _, ch := range e.slots {
			if ch.Watching {
				twitch = append(twitch, firstNonEmpty(ch.Name, ch.Login))
			}
		}
	}
	points := ""
	if cfg.Points {
		var parts []string
		if len(twitch) > 0 {
			parts = append(parts, "Twitch: "+rpcNames(twitch, 44))
		}
		if len(kick) > 0 {
			parts = append(parts, "Kick: "+rpcNames(kick, 44))
		}
		points = strings.Join(parts, " · ")
	}
	var lists []string // the queue and the tracked games
	if cfg.Queue && len(queue) > 0 {
		lists = append(lists, fmt.Sprintf(tr("rpc.queue"), rpcNames(queue, 50)))
	}
	if cfg.Tracked && len(tracked) > 0 {
		lists = append(lists, fmt.Sprintf(tr("rpc.tracked"), rpcNames(tracked, 50)))
	}

	act = &rpcActivity{Type: 3, Assets: &rpcAssets{LargeImage: rpcLogo}}
	more := "" // goes under the small picture
	switch f := e.drops.cur; {
	case cfg.Drop && f != nil:
		act.Details = fmt.Sprintf(tr("rpc.drop"), f.game.Name)
		act.State = fmt.Sprintf(tr("rpc.on"), firstNonEmpty(f.ch.Name, f.ch.Login))
		if d := e.farmedDrop(f, now); d != nil && d.Req > 0 {
			act.State = fmt.Sprintf(tr("rpc.prog"), firstNonEmpty(d.Name, f.game.Name), min(d.Cur, d.Req), d.Req)
		}
		act.Timestamps = &rpcTimes{Start: f.started.Unix()}
		if art := rpcArt(f.game.Art); art != "" {
			act.Assets.LargeImage = art
		}
		act.Assets.SmallImage = rpcDrops
		if points != "" {
			more = tr("set.rpcPoints") + " – " + points
		}
	case points != "":
		act.Details = tr("rpc.pointsHead")
		act.State = points
		act.Timestamps = &rpcTimes{Start: e.started.Unix()}
		act.Assets.SmallImage = rpcPoints
	default:
		// Nothing being farmed or collected: Discord shows nothing (no "waiting for a drop" activity).
		return cfg, id, nil
	}
	act.Details, act.State = rpcText(act.Details), rpcText(act.State)
	act.Assets.LargeText = rpcText(firstNonEmpty(strings.Join(lists, " • "), appName+" "+version))
	act.Assets.SmallText = rpcText(firstNonEmpty(more, appName+" "+version))
	if cfg.Button {
		act.Buttons = []rpcButton{{Label: rpcText(tr("rpc.btn")), URL: siteURL}, {Label: "GitHub", URL: repoURL}}
	}
	return cfg, id, act
}

// farmedDrop is the reward the current farm is working on: the one Twitch reports, otherwise (reward
// campaigns are not reported) the open one that is furthest along. Call with e.mu held.
func (e *Engine) farmedDrop(f *farm, now time.Time) *Drop {
	ids := make([]string, 0, len(f.campIDs))
	for id := range f.campIDs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var best *Drop
	for _, id := range ids {
		c := e.drops.camps[id]
		if c == nil {
			continue
		}
		if d := c.drop(f.dropID); d != nil && f.dropID != "" {
			return d
		}
		for _, d := range c.earnable(now) {
			if best == nil || d.Cur > best.Cur {
				best = d
			}
		}
	}
	return best
}

// rpcArt turns Twitch's box art address (with a {width}x{height} placeholder) into a picture Discord can load.
func rpcArt(u string) string {
	if !strings.HasPrefix(u, "https://") {
		return ""
	}
	u = strings.NewReplacer("{width}", "288", "{height}", "384").Replace(u)
	if len(u) > 256 {
		return ""
	}
	return u
}

// --- the connection ---

type discordRPC struct {
	conn    net.Conn
	appID   string
	last    string // the activity Discord has, as JSON; "" = none
	sentAt  time.Time
	retryAt time.Time
	nonce   int
}

var errDiscordID = errors.New("discord does not know this application id")

func (d *discordRPC) close() {
	if d.conn != nil {
		d.conn.Close()
	}
	d.conn, d.last = nil, ""
}

func (d *discordRPC) write(op uint32, v any) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	buf := make([]byte, 8+len(raw))
	binary.LittleEndian.PutUint32(buf, op)
	binary.LittleEndian.PutUint32(buf[4:], uint32(len(raw)))
	copy(buf[8:], raw)
	_, err = d.conn.Write(buf)
	return err
}

func (d *discordRPC) read() (uint32, map[string]any, error) {
	var head [8]byte
	if _, err := io.ReadFull(d.conn, head[:]); err != nil {
		return 0, nil, err
	}
	n := binary.LittleEndian.Uint32(head[4:])
	if n > 1<<20 {
		return 0, nil, errors.New("discord: frame too long")
	}
	raw := make([]byte, n)
	if _, err := io.ReadFull(d.conn, raw); err != nil {
		return 0, nil, err
	}
	var m map[string]any
	json.Unmarshal(raw, &m)
	return binary.LittleEndian.Uint32(head[:]), m, nil
}

// connect finds the running Discord client (it listens on one of ten pipes) and introduces the application.
func (d *discordRPC) connect(appID string) error {
	var err error
	for i := 0; i < 10; i++ {
		var c net.Conn
		if c, err = dialDiscord(i); err != nil {
			continue
		}
		d.conn, d.appID, d.last = c, appID, ""
		c.SetDeadline(time.Now().Add(5 * time.Second))
		if err = d.write(0, map[string]any{"v": 1, "client_id": appID}); err == nil {
			var op uint32
			var m map[string]any
			if op, m, err = d.read(); err == nil {
				if op == 1 && str(m["evt"]) == "READY" {
					return nil
				}
				err = fmt.Errorf("discord: handshake refused (%d %s)", num(m["code"]), str(m["message"]))
				if num(m["code"]) == 4000 {
					err = errDiscordID
				}
			}
		}
		d.close()
		if err == errDiscordID {
			return err
		}
	}
	if err == nil {
		err = errors.New("discord is not running")
	}
	return err
}

// set shows the activity (nil clears it). A refusal of the activity itself comes back as rpcRefused.
type rpcRefused string

func (r rpcRefused) Error() string { return "discord refused the activity: " + string(r) }

func (d *discordRPC) set(act *rpcActivity) error {
	d.nonce++
	nonce := strconv.Itoa(d.nonce)
	args := map[string]any{"pid": os.Getpid()}
	if act != nil {
		args["activity"] = act
	}
	d.conn.SetDeadline(time.Now().Add(5 * time.Second))
	if err := d.write(1, map[string]any{"cmd": "SET_ACTIVITY", "args": args, "nonce": nonce}); err != nil {
		return err
	}
	for {
		op, m, err := d.read()
		if err != nil {
			return err
		}
		switch {
		case op == 3: // ping
			if err := d.write(4, m); err != nil {
				return err
			}
		case op == 2:
			return fmt.Errorf("discord closed the connection (%d %s)", num(m["code"]), str(m["message"]))
		case op == 1 && str(m["nonce"]) == nonce:
			if str(m["evt"]) == "ERROR" {
				return rpcRefused(str(dig(m, "data", "message")))
			}
			return nil
		}
	}
}

// discordLoop keeps Discord in step with what Loorek is doing.
func (e *Engine) discordLoop() {
	var d discordRPC
	t := time.NewTicker(3 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-e.quit:
			if d.conn != nil {
				d.set(nil)
				d.close()
			}
			return
		case <-t.C:
		}
		state := e.discordStep(&d, time.Now())
		e.mu.Lock()
		e.rpcState = state
		e.mu.Unlock()
	}
}

// discordStep returns the state shown in the settings: off | noid | nodiscord | badid | refused | ok.
func (e *Engine) discordStep(d *discordRPC, now time.Time) string {
	cfg, id, act := e.presence(now)
	if act == nil {
		if d.conn != nil {
			d.set(nil)
			d.close()
		}
		d.retryAt = time.Time{}
		switch {
		case !cfg.On:
			return "off"
		case id == "":
			return "noid"
		}
		return "idle"
	}
	if d.conn != nil && d.appID != id {
		d.set(nil)
		d.close()
		d.retryAt = time.Time{}
	}
	if d.conn == nil {
		if now.Before(d.retryAt) {
			return e.rpcStateNow()
		}
		if err := d.connect(id); err != nil {
			d.retryAt = now.Add(20 * time.Second)
			if err == errDiscordID {
				e.log.Debug("discord: %v", err)
				return "badid"
			}
			return "nodiscord"
		}
		e.log.Debug("discord: connected")
		d.sentAt = time.Time{}
	}
	raw, _ := json.Marshal(act)
	if string(raw) == d.last || now.Sub(d.sentAt) < 4*time.Second { // Discord takes five changes in twenty seconds
		if d.last == "" {
			return e.rpcStateNow()
		}
		return "ok"
	}
	err := d.set(act)
	d.sentAt = now
	var refused rpcRefused
	switch {
	case errors.As(err, &refused):
		e.log.Debug("discord: %v", err)
		d.last = string(raw) // sending the same thing again would be refused again
		return "refused"
	case err != nil:
		e.log.Debug("discord: %v", err)
		d.close()
		d.retryAt = now.Add(10 * time.Second)
		return "nodiscord"
	}
	d.last = string(raw)
	return "ok"
}

func (e *Engine) rpcStateNow() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.rpcState
}
