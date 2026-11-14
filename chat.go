package main

import (
	"bufio"
	"crypto/tls"
	"net"
	"strings"
	"sync"
	"time"
)

// chatClient keeps the account present in the chats of the listed channels (plain IRC, nothing is sent to chat).
type chatClient struct {
	e      *Engine
	mu     sync.Mutex
	state  string // off | connecting | on | denied
	joined map[string]bool
	wake   chan struct{}
	denied string // token that was refused, so it is not retried in a loop
}

func newChat(e *Engine) *chatClient {
	return &chatClient{e: e, state: "off", joined: map[string]bool{}, wake: make(chan struct{}, 1)}
}

func (c *chatClient) sync() {
	select {
	case c.wake <- struct{}{}:
	default:
	}
}

func (c *chatClient) status() (string, int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.state, len(c.joined)
}

func (c *chatClient) setState(s string) {
	c.mu.Lock()
	c.state = s
	if s != "on" {
		c.joined = map[string]bool{}
	}
	c.mu.Unlock()
}

// want returns the channels to sit in, or nil when chat presence should be off.
func (c *chatClient) want() (token, login string, chans []string) {
	e := c.e
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.authState != "ok" || !e.set.ChatOn {
		return "", "", nil
	}
	for _, ch := range e.set.Channels {
		if ch.On {
			chans = append(chans, ch.Login)
		}
	}
	return e.auth.Token, e.auth.Login, chans
}

func (c *chatClient) run() {
	backoff := 5 * time.Second
	for {
		token, login, chans := c.want()
		if token == "" || len(chans) == 0 || token == c.denied {
			if token != c.denied {
				c.setState("off")
			}
			select {
			case <-c.wake:
			case <-time.After(15 * time.Second):
			case <-c.e.quit:
				return
			}
			continue
		}
		c.setState("connecting")
		ok := c.session(token, login)
		if ok {
			backoff = 5 * time.Second
		} else if backoff < 2*time.Minute {
			backoff *= 2
		}
		select {
		case <-time.After(backoff):
		case <-c.e.quit:
			return
		}
	}
}

func (c *chatClient) dial() (net.Conn, error) {
	addr := c.e.c.ep.IRC
	d := &net.Dialer{Timeout: 15 * time.Second}
	if strings.HasPrefix(addr, "plain:") {
		return d.Dial("tcp", strings.TrimPrefix(addr, "plain:"))
	}
	return tls.DialWithDialer(d, "tcp", addr, nil)
}

// session runs one connection; returns true if it was established (so the next retry can be quick).
func (c *chatClient) session(token, login string) bool {
	conn, err := c.dial()
	if err != nil {
		c.e.log.Debug("chat: %v", err)
		c.setState("off")
		return false
	}
	defer conn.Close()
	send := func(line string) error {
		conn.SetWriteDeadline(time.Now().Add(15 * time.Second))
		_, err := conn.Write([]byte(line + "\r\n"))
		return err
	}
	send("PASS oauth:" + token)
	send("NICK " + login)

	lines := make(chan string, 64)
	go func() {
		defer close(lines)
		r := bufio.NewReaderSize(conn, 64<<10)
		for {
			conn.SetReadDeadline(time.Now().Add(6 * time.Minute))
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			// chat messages themselves are of no interest - only server lines are passed on
			if strings.HasPrefix(line, "PING") || strings.HasPrefix(line, ":tmi.twitch.tv") || strings.Contains(line, " RECONNECT") {
				select {
				case lines <- strings.TrimRight(line, "\r\n"):
				default:
				}
			}
		}
	}()

	ready := false
	ping := time.NewTicker(4 * time.Minute)
	defer ping.Stop()
	resync := time.NewTicker(30 * time.Second)
	defer resync.Stop()
	joinDiff := func() bool {
		tok, _, want := c.want()
		if tok != token || len(want) == 0 {
			return false // signed out, switched off or nothing to join: close
		}
		wantSet := map[string]bool{}
		for _, w := range want {
			wantSet[w] = true
		}
		c.mu.Lock()
		var join, part []string
		for w := range wantSet {
			if !c.joined[w] {
				join = append(join, w)
			}
		}
		for j := range c.joined {
			if !wantSet[j] {
				part = append(part, j)
			}
		}
		c.mu.Unlock()
		if len(join) > 15 { // Twitch limits joins per 10 s; the rest goes with the next round
			join = join[:15]
		}
		for _, p := range part {
			send("PART #" + p)
		}
		for _, j := range join {
			if send("JOIN #"+j) != nil {
				return false
			}
		}
		c.mu.Lock()
		for _, p := range part {
			delete(c.joined, p)
		}
		for _, j := range join {
			c.joined[j] = true
		}
		c.mu.Unlock()
		return true
	}
	for {
		select {
		case <-c.e.quit:
			return ready
		case line, ok := <-lines:
			if !ok {
				c.setState("off")
				return ready
			}
			switch {
			case strings.HasPrefix(line, "PING"):
				send("PONG" + strings.TrimPrefix(line, "PING"))
			case strings.Contains(line, " 001 "):
				ready = true
				c.setState("on")
				c.e.log.Info("L.chatOn")
				if !joinDiff() {
					c.setState("off")
					return true
				}
			case strings.Contains(line, "NOTICE") && (strings.Contains(line, "Login authentication failed") || strings.Contains(line, "Improperly formatted auth")):
				c.denied = token
				c.setState("denied")
				c.e.log.Warn("L.chatDenied")
				return false
			case strings.Contains(line, " RECONNECT"):
				c.setState("off")
				return true
			}
		case <-ping.C:
			if send("PING :tmi.twitch.tv") != nil {
				c.setState("off")
				return ready
			}
		case <-resync.C:
			if ready && !joinDiff() {
				c.setState("off")
				return true
			}
		case <-c.wake:
			if ready && !joinDiff() {
				c.setState("off")
				return true
			}
		}
	}
}
