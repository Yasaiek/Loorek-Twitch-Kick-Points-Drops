package main

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Signing in happens in a browser window of its own: the browser already installed on the computer, started
// with a separate profile kept in the program's data folder. The user signs in there like on any website and
// the program only reads the result (the address the service redirects to, or its session cookie) over the
// browser's local debugging port. Pages are not scripted and the user's everyday browser profile is never touched.

type cdpConn struct {
	ws    *wsConn
	mu    sync.Mutex
	next  int
	wait  map[int]chan map[string]any
	event func(method string, params any)
	dead  chan struct{}
}

func cdpDial(wsURL string, event func(string, any)) (*cdpConn, error) {
	ws, err := wsDial(wsURL, nil)
	if err != nil {
		return nil, err
	}
	c := &cdpConn{ws: ws, wait: map[int]chan map[string]any{}, event: event, dead: make(chan struct{})}
	go func() {
		defer close(c.dead)
		for {
			raw, err := ws.Read()
			if err != nil {
				return
			}
			var m map[string]any
			if json.Unmarshal([]byte(raw), &m) != nil {
				continue
			}
			if id := int(num(m["id"])); id > 0 {
				c.mu.Lock()
				ch := c.wait[id]
				delete(c.wait, id)
				c.mu.Unlock()
				if ch != nil {
					ch <- m
				}
				continue
			}
			if method := str(m["method"]); method != "" && c.event != nil {
				c.event(method, m["params"])
			}
		}
	}()
	return c, nil
}

func (c *cdpConn) call(method string, params map[string]any, session string) (any, error) {
	c.mu.Lock()
	c.next++
	id := c.next
	ch := make(chan map[string]any, 1)
	c.wait[id] = ch
	c.mu.Unlock()
	msg := map[string]any{"id": id, "method": method}
	if params != nil {
		msg["params"] = params
	}
	if session != "" {
		msg["sessionId"] = session
	}
	raw, _ := json.Marshal(msg)
	if err := c.ws.WriteText(string(raw)); err != nil {
		return nil, err
	}
	select {
	case m := <-ch:
		if e := m["error"]; e != nil {
			return nil, errors.New("cdp: " + str(dig(e, "message")))
		}
		return m["result"], nil
	case <-c.dead:
		return nil, io.EOF
	case <-time.After(10 * time.Second):
		c.mu.Lock()
		delete(c.wait, id)
		c.mu.Unlock()
		return nil, errors.New("cdp: timeout")
	}
}

type loginWin struct {
	cmd    *exec.Cmd
	c      *cdpConn
	mu     sync.Mutex
	pages  map[string]string // open tabs: id -> address
	had    bool
	urls   chan string
	hdrs   chan string // Authorization values seen while sniffing (Kick fallback)
	hdrFor string      // host whose requests are of interest
	closed chan struct{}
	once   sync.Once
}

func (lw *loginWin) close() {
	lw.once.Do(func() {
		close(lw.closed)
		go func() {
			lw.mu.Lock()
			c := lw.c
			lw.mu.Unlock()
			if c != nil {
				c.call("Browser.close", nil, "")
			}
			time.Sleep(1500 * time.Millisecond)
			if c != nil {
				c.ws.Close()
			}
			if lw.cmd != nil && lw.cmd.Process != nil {
				lw.cmd.Process.Kill()
			}
		}()
	})
}

func (lw *loginWin) isClosed() bool {
	select {
	case <-lw.closed:
		return true
	default:
		return false
	}
}

func (lw *loginWin) onEvent(method string, params any) {
	switch method {
	case "Target.targetCreated", "Target.targetInfoChanged":
		info := dig(params, "targetInfo")
		if str(dig(info, "type")) != "page" {
			return
		}
		id, u := str(dig(info, "targetId")), str(dig(info, "url"))
		lw.mu.Lock()
		lw.pages[id], lw.had = u, true
		lw.mu.Unlock()
		select {
		case lw.urls <- u:
		default:
		}
	case "Target.targetDestroyed":
		id := str(dig(params, "targetId"))
		lw.mu.Lock()
		_, was := lw.pages[id]
		delete(lw.pages, id)
		empty := was && len(lw.pages) == 0
		lw.mu.Unlock()
		if empty { // the user closed the window
			lw.close()
		}
	case "Network.requestWillBeSent", "Network.requestWillBeSentExtraInfo":
		lw.mu.Lock()
		host := lw.hdrFor
		lw.mu.Unlock()
		if host == "" {
			return
		}
		if u := str(dig(params, "request", "url")); u != "" && !hostMatches(u, host) {
			return
		}
		hs, _ := dig(params, "request", "headers").(map[string]any)
		if hs == nil {
			hs, _ = dig(params, "headers").(map[string]any)
			if a := str(hs[":authority"]); a != "" && !sameSite(a, host) {
				return
			}
		}
		for k, v := range hs {
			if strings.EqualFold(k, "authorization") {
				if val := str(v); len(val) > 12 && strings.HasPrefix(strings.ToLower(val), "bearer ") {
					select {
					case lw.hdrs <- strings.TrimSpace(val[7:]):
					default:
					}
				}
			}
		}
	}
}

// sameSite: a is host, or one is a subdomain of the other (kick.com / web.kick.com).
func sameSite(a, host string) bool {
	a, host = strings.TrimPrefix(strings.ToLower(a), "."), strings.ToLower(host)
	if h, _, err := net.SplitHostPort(a); err == nil {
		a = h
	}
	return a == host || strings.HasSuffix(a, "."+host) || strings.HasSuffix(host, "."+a)
}

func hostMatches(raw, host string) bool {
	u, err := url.Parse(raw)
	return err == nil && sameSite(u.Hostname(), host)
}

func hostOf(base string) string {
	u, err := url.Parse(base)
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Hostname())
}

func freePort() int {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

func debugWS(port int) string {
	hc := &http.Client{Timeout: 2 * time.Second}
	res, err := hc.Get("http://127.0.0.1:" + strconv.Itoa(port) + "/json/version")
	if err != nil {
		return ""
	}
	defer res.Body.Close()
	var v struct {
		WS string `json:"webSocketDebuggerUrl"`
	}
	json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&v)
	return v.WS
}

// openLogin starts the sign-in window at the given address. Only one such window exists at a time.
func (e *Engine) openLogin(start string) (*loginWin, error) {
	e.loginMu.Lock()
	defer e.loginMu.Unlock()
	if e.lw != nil {
		e.lw.close()
		e.lw = nil
		time.Sleep(1800 * time.Millisecond)
	}
	exe := findBrowser()
	if exe == "" {
		return nil, errors.New("no browser found")
	}
	portFile := filepath.Join(e.dir, "login-port")
	// a window left over from a previous run would swallow the new one: close it first
	if raw, err := os.ReadFile(portFile); err == nil {
		if p, _ := strconv.Atoi(strings.TrimSpace(string(raw))); p > 0 {
			if ws := debugWS(p); ws != "" {
				if c, err := cdpDial(ws, nil); err == nil {
					c.call("Browser.close", nil, "")
					c.ws.Close()
					time.Sleep(1500 * time.Millisecond)
				}
			}
		}
	}
	port := freePort()
	if port == 0 {
		return nil, errors.New("no free port")
	}
	os.WriteFile(portFile, []byte(strconv.Itoa(port)), 0o600)
	args := []string{
		"--remote-debugging-port=" + strconv.Itoa(port),
		"--user-data-dir=" + filepath.Join(e.dir, "login-profile"),
		"--no-first-run", "--no-default-browser-check", "--disable-sync", "--disable-background-mode",
		"--window-size=1000,780", "--new-window",
	}
	args = append(args, extraFlags()...)
	args = append(args, start)
	cmd := exec.Command(exe, args...)
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	trackProcess(cmd.Process.Pid)
	lw := &loginWin{cmd: cmd, pages: map[string]string{}, urls: make(chan string, 64), hdrs: make(chan string, 8), closed: make(chan struct{})}
	exited := make(chan struct{})
	go func() { cmd.Wait(); close(exited); lw.close() }()
	ws := ""
	for i := 0; i < 60 && ws == ""; i++ {
		select {
		case <-exited:
			return nil, errors.New("browser exited")
		case <-time.After(250 * time.Millisecond):
		}
		ws = debugWS(port)
	}
	if ws == "" {
		lw.close()
		return nil, errors.New("browser did not answer")
	}
	c, err := cdpDial(ws, lw.onEvent)
	if err != nil {
		lw.close()
		return nil, err
	}
	lw.mu.Lock()
	lw.c = c
	lw.mu.Unlock()
	go func() { <-c.dead; lw.close() }()
	if _, err := c.call("Target.setDiscoverTargets", map[string]any{"discover": true}, ""); err != nil {
		lw.close()
		return nil, err
	}
	e.lw = lw
	return lw, nil
}

func (e *Engine) closeLogin() {
	e.loginMu.Lock()
	if e.lw != nil {
		e.lw.close()
		e.lw = nil
	}
	e.loginMu.Unlock()
}

// forgetBrowserSessions removes the sign-in window's profile, so the next sign-in starts from a clean slate.
func (e *Engine) forgetBrowserSessions() {
	e.closeLogin()
	go func() {
		time.Sleep(2500 * time.Millisecond)
		e.loginMu.Lock()
		defer e.loginMu.Unlock()
		if e.lw == nil {
			os.RemoveAll(filepath.Join(e.dir, "login-profile"))
		}
	}()
}

const loginWait = 15 * time.Minute

// ---- Twitch ----

// startLogin opens the sign-in window on Twitch's own login page, asked to hand a session for the Twitch app
// back to twitch.tv. The address Twitch redirects to carries that session; the window is closed as soon as it shows up.
func (e *Engine) startLogin(manual bool) {
	e.mu.Lock()
	if e.authState == "ok" {
		e.mu.Unlock()
		return
	}
	e.authState, e.authNote = "login", ""
	e.loginMode = "auto"
	if manual {
		e.loginMode = "manual"
	}
	e.mu.Unlock()
	target := loginURL(e.c.ep.WWW)
	if manual {
		e.closeLogin()
		e.log.Info("L.loginManual")
		if err := openURL(target); err != nil {
			e.log.Warn("L.browserFail", err)
		}
		return
	}
	go func() {
		e.log.Info("L.loginWin")
		lw, err := e.openLogin(target)
		if err != nil {
			e.log.Warn("L.loginWinFail", err)
			e.mu.Lock()
			still := e.authState == "login"
			e.loginMode = "manual"
			e.mu.Unlock()
			if still {
				openURL(target)
			}
			return
		}
		host := hostOf(e.c.ep.WWW)
		beat := time.NewTicker(20 * time.Second)
		defer beat.Stop()
		deadline := time.After(loginWait)
		for {
			select {
			case u := <-lw.urls:
				if !hostMatches(u, host) {
					continue
				}
				if m := reToken.FindStringSubmatch(u); m != nil {
					e.log.Info("L.loginGot")
					e.setToken(m[1])
					lw.close()
					return
				}
			case <-beat.C:
				lw.c.call("Browser.getVersion", nil, "")
			case <-deadline:
				lw.close()
			case <-lw.closed:
				e.mu.Lock()
				if e.authState == "login" && e.loginMode == "auto" {
					e.authState, e.authNote = "out", "closed"
				}
				e.mu.Unlock()
				return
			}
		}
	}()
}

func (e *Engine) cancelLogin() {
	e.mu.Lock()
	if e.authState == "login" {
		e.authState, e.authNote = "out", ""
	}
	e.mu.Unlock()
	e.closeLogin()
}

// ---- Kick ----

// kickVerdict waits for the Kick service to check a freshly stored key.
func (e *Engine) kickVerdict(token string) string {
	for i := 0; i < 40; i++ {
		time.Sleep(iv(time.Second))
		e.mu.Lock()
		cur := e.auth.KickToken
		e.mu.Unlock()
		e.kick.mu.Lock()
		checked, state := e.kick.checked, e.kick.state
		e.kick.mu.Unlock()
		switch {
		case cur != token:
			return "bad"
		case checked == token:
			return "ok"
		case state == "blocked":
			return "blocked"
		}
	}
	return "unknown"
}

// kickLogin opens the sign-in window on kick.com and waits for the session to appear there.
func (e *Engine) kickLogin() {
	e.mu.Lock()
	if e.kickLoginOn {
		e.mu.Unlock()
		return
	}
	e.kickLoginOn, e.kickLoginNote = true, ""
	if !e.set.KickOn {
		e.set.KickOn = true
		e.saveSettings()
	}
	e.mu.Unlock()
	finish := func(note string) {
		e.mu.Lock()
		e.kickLoginOn, e.kickLoginNote = false, note
		e.mu.Unlock()
	}
	go func() {
		base := strings.TrimRight(e.c.ep.Kick, "/")
		host := hostOf(base)
		e.log.Info("L.kickWin")
		lw, err := e.openLogin(base + "/")
		if err != nil {
			e.log.Warn("L.kickWinFail", err)
			finish("nowindow")
			return
		}
		e.kick.mu.Lock()
		e.kick.auto = true
		e.kick.mu.Unlock()
		defer func() {
			e.kick.mu.Lock()
			e.kick.auto = false
			e.kick.mu.Unlock()
		}()
		go lw.openForm(host, `[data-testid="login"]`)
		try := func(tok, how string) string {
			if e.kickSetToken(tok) != "ok" {
				return "bad"
			}
			v := e.kickVerdict(cleanKickToken(tok))
			e.log.Debug("kick login: %s -> %s", how, v)
			return v
		}
		tried := map[string]bool{}
		tick := time.NewTicker(1500 * time.Millisecond)
		defer tick.Stop()
		deadline := time.After(loginWait)
		for {
			select {
			case <-lw.closed:
				finish("closed")
				return
			case <-deadline:
				lw.close()
			case <-tick.C:
			}
			res, err := lw.c.call("Storage.getCookies", nil, "")
			if err != nil {
				continue
			}
			cookie := ""
			for _, ck := range list(dig(res, "cookies")) {
				if str(dig(ck, "name")) == "session_token" && sameSite(str(dig(ck, "domain")), host) {
					cookie = str(dig(ck, "value"))
				}
			}
			if cookie == "" || tried[cookie] {
				continue
			}
			tried[cookie] = true
			v := try(cookie, "cookie")
			if v == "bad" { // the cookie is not the key the site's own requests use: read that key from one of them
				if tok := lw.sniffAuth(host); tok != "" && !tried[tok] {
					tried[tok] = true
					v = try(tok, "header")
				}
			}
			if v == "bad" {
				continue // not signed in yet, or a stale session: keep waiting
			}
			if v == "ok" {
				e.log.Info("L.kickGot")
			}
			lw.close()
			finish("")
			return
		}
	}()
}

// openForm presses the site's own "log in" button once the page has loaded, so the sign-in form is what the
// user sees first (Kick has no address that opens it). Nothing else on the page is touched.
func (lw *loginWin) openForm(host, selector string) {
	js := `(()=>{const b=[...document.querySelectorAll(` + strconv.Quote(selector) + `)].find(x=>x.offsetParent!==null);if(!b)return false;b.click();return true})()`
	for i := 0; i < 25 && !lw.isClosed(); i++ {
		time.Sleep(time.Second)
		lw.mu.Lock()
		target := ""
		for id, u := range lw.pages {
			if hostMatches(u, host) {
				target = id
			}
		}
		lw.mu.Unlock()
		if target == "" {
			continue
		}
		res, err := lw.c.call("Target.attachToTarget", map[string]any{"targetId": target, "flatten": true}, "")
		if err != nil {
			continue
		}
		sid := str(dig(res, "sessionId"))
		out, _ := lw.c.call("Runtime.evaluate", map[string]any{"expression": js, "returnByValue": true}, sid)
		lw.c.call("Target.detachFromTarget", map[string]any{"sessionId": sid}, "")
		if boolean(dig(out, "result", "value")) {
			return
		}
	}
}

// sniffAuth reloads the site's tab once and returns the Authorization key its own requests carry.
func (lw *loginWin) sniffAuth(host string) string {
	lw.mu.Lock()
	target := ""
	for id, u := range lw.pages {
		if hostMatches(u, host) {
			target = id
		}
	}
	lw.hdrFor = host
	lw.mu.Unlock()
	defer func() {
		lw.mu.Lock()
		lw.hdrFor = ""
		lw.mu.Unlock()
	}()
	if target == "" {
		return ""
	}
	res, err := lw.c.call("Target.attachToTarget", map[string]any{"targetId": target, "flatten": true}, "")
	if err != nil {
		return ""
	}
	sid := str(dig(res, "sessionId"))
	defer lw.c.call("Target.detachFromTarget", map[string]any{"sessionId": sid}, "")
	for len(lw.hdrs) > 0 {
		<-lw.hdrs
	}
	lw.c.call("Network.enable", nil, sid)
	lw.c.call("Page.reload", nil, sid)
	select {
	case tok := <-lw.hdrs:
		return tok
	case <-lw.closed:
	case <-time.After(15 * time.Second):
	}
	return ""
}

func (e *Engine) kickLoginCancel() {
	e.closeLogin()
}
