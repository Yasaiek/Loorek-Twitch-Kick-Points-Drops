package main

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	utls "github.com/bogdanfinn/utls"
)

// wsConn is a small WebSocket client (text frames only). For wss:// it presents the same TLS handshake
// as Chrome does, because the service behind it turns away connections that do not look like a browser.
type wsConn struct {
	c    net.Conn
	r    *bufio.Reader
	wmu  sync.Mutex
	dead bool
}

func wsDial(raw string, headers map[string]string) (*wsConn, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	host := u.Host
	secure := u.Scheme == "wss" || u.Scheme == "https"
	if u.Port() == "" {
		if secure {
			host += ":443"
		} else {
			host += ":80"
		}
	}
	d := &net.Dialer{Timeout: 15 * time.Second}
	conn, err := d.Dial("tcp", host)
	if err != nil {
		return nil, err
	}
	conn.SetDeadline(time.Now().Add(20 * time.Second))
	if secure {
		uc := utls.UClient(conn, &utls.Config{ServerName: u.Hostname()}, utls.HelloChrome_133, true, true, true)
		if err := uc.Handshake(); err != nil {
			conn.Close()
			return nil, fmt.Errorf("tls: %w", err)
		}
		conn = uc
	}
	keyRaw := make([]byte, 16)
	rand.Read(keyRaw)
	path := u.RequestURI()
	req := "GET " + path + " HTTP/1.1\r\nHost: " + u.Host + "\r\nConnection: Upgrade\r\nPragma: no-cache\r\nCache-Control: no-cache\r\n"
	if ua := headers["User-Agent"]; ua != "" {
		req += "User-Agent: " + ua + "\r\n"
	}
	req += "Upgrade: websocket\r\n"
	if o := headers["Origin"]; o != "" {
		req += "Origin: " + o + "\r\n"
	}
	req += "Sec-WebSocket-Version: 13\r\nAccept-Encoding: gzip, deflate, br\r\nAccept-Language: en-US,en;q=0.9\r\nSec-WebSocket-Key: " + base64.StdEncoding.EncodeToString(keyRaw) + "\r\n\r\n"
	if _, err := conn.Write([]byte(req)); err != nil {
		conn.Close()
		return nil, err
	}
	br := bufio.NewReaderSize(conn, 8<<10)
	res, err := http.ReadResponse(br, nil)
	if err != nil {
		conn.Close()
		return nil, err
	}
	if res.StatusCode != 101 {
		conn.Close()
		return nil, fmt.Errorf("websocket: HTTP %d", res.StatusCode)
	}
	conn.SetDeadline(time.Time{})
	return &wsConn{c: conn, r: br}, nil
}

func (w *wsConn) writeFrame(op byte, payload []byte) error {
	w.wmu.Lock()
	defer w.wmu.Unlock()
	n := len(payload)
	head := []byte{0x80 | op}
	switch {
	case n < 126:
		head = append(head, 0x80|byte(n))
	case n < 65536:
		head = append(head, 0x80|126, byte(n>>8), byte(n))
	default:
		b := make([]byte, 8)
		binary.BigEndian.PutUint64(b, uint64(n))
		head = append(append(head, 0x80|127), b...)
	}
	mask := make([]byte, 4)
	rand.Read(mask)
	head = append(head, mask...)
	out := make([]byte, n)
	for i := range payload {
		out[i] = payload[i] ^ mask[i%4]
	}
	w.c.SetWriteDeadline(time.Now().Add(15 * time.Second))
	_, err := w.c.Write(append(head, out...))
	return err
}

func (w *wsConn) WriteText(s string) error { return w.writeFrame(1, []byte(s)) }

// Read returns the next text message. Pings from the server are answered on the way.
func (w *wsConn) Read() (string, error) {
	var msg []byte
	for {
		w.c.SetReadDeadline(time.Now().Add(3 * time.Minute))
		h := make([]byte, 2)
		if _, err := io.ReadFull(w.r, h); err != nil {
			return "", err
		}
		fin, op := h[0]&0x80 != 0, h[0]&0x0f
		masked := h[1]&0x80 != 0
		n := uint64(h[1] & 0x7f)
		switch n {
		case 126:
			b := make([]byte, 2)
			if _, err := io.ReadFull(w.r, b); err != nil {
				return "", err
			}
			n = uint64(binary.BigEndian.Uint16(b))
		case 127:
			b := make([]byte, 8)
			if _, err := io.ReadFull(w.r, b); err != nil {
				return "", err
			}
			n = binary.BigEndian.Uint64(b)
		}
		if n > 4<<20 {
			return "", errors.New("websocket: message too large")
		}
		var mask []byte
		if masked {
			mask = make([]byte, 4)
			if _, err := io.ReadFull(w.r, mask); err != nil {
				return "", err
			}
		}
		payload := make([]byte, n)
		if _, err := io.ReadFull(w.r, payload); err != nil {
			return "", err
		}
		for i := range payload {
			if masked {
				payload[i] ^= mask[i%4]
			}
		}
		switch op {
		case 8:
			return "", io.EOF
		case 9:
			w.writeFrame(10, payload)
			continue
		case 10:
			continue
		}
		msg = append(msg, payload...)
		if fin {
			return string(msg), nil
		}
	}
}

func (w *wsConn) Close() {
	w.writeFrame(8, nil)
	w.c.Close()
}
