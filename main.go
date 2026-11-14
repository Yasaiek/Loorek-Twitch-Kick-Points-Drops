package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type instanceFile struct {
	Port int    `json:"port"`
	Key  string `json:"key"`
}

func main() {
	trayOnly, updated := false, false
	for _, a := range os.Args[1:] {
		switch a {
		case "--tray":
			trayOnly = true
		case "--updated":
			updated = true
		}
	}
	loadDicts()
	// everything the program keeps lives in the "data" folder next to the executable
	dir := os.Getenv("DROPOMAT_DATA")
	if dir == "" {
		exe, err := os.Executable()
		if err != nil {
			exe = os.Args[0]
		}
		dir = filepath.Join(filepath.Dir(exe), "data")
	}
	os.MkdirAll(dir, 0o700)
	if n, _ := strconv.Atoi(os.Getenv("DROPOMAT_FAST")); n > 1 {
		speed = n
	}
	instPath := filepath.Join(dir, "instance.json")

	// already running? ask that copy to show its panel and leave
	var inst instanceFile
	if readJSON(instPath, &inst) == nil && inst.Port > 0 {
		base := "http://127.0.0.1:" + strconv.Itoa(inst.Port)
		hc := &http.Client{Timeout: 2 * time.Second}
		call := func(method, path, body string) (string, bool) {
			req, _ := http.NewRequest(method, base+path, strings.NewReader(body))
			req.Header.Set("X-Dropomat-Key", inst.Key)
			res, err := hc.Do(req)
			if err != nil {
				return "", false
			}
			defer res.Body.Close()
			raw, _ := io.ReadAll(io.LimitReader(res.Body, 4<<20))
			return string(raw), res.StatusCode == 200
		}
		if state, ok := call("GET", "/api/state", ""); ok {
			var v struct {
				Version string `json:"version"`
			}
			json.Unmarshal([]byte(state), &v)
			if v.Version == version {
				call("GET", "/api/show", "")
				return
			}
			// a different version is running (this file is an update): ask it to close and take over
			call("POST", "/api/op", `{"op":"quit"}`)
			for i := 0; i < 30; i++ {
				time.Sleep(200 * time.Millisecond)
				if _, alive := call("GET", "/api/state", ""); !alive {
					break
				}
			}
		}
	}

	ln, err := net.Listen("tcp", "127.0.0.1:47616")
	for i := 0; err != nil && updated && i < 25; i++ { // the previous version may still be letting go of the port
		time.Sleep(200 * time.Millisecond)
		ln, err = net.Listen("tcp", "127.0.0.1:47616")
	}
	if p := os.Getenv("DROPOMAT_PORT"); p != "" {
		if ln != nil {
			ln.Close()
		}
		ln, err = net.Listen("tcp", "127.0.0.1:"+p)
		for i := 0; err != nil && updated && i < 25; i++ {
			time.Sleep(200 * time.Millisecond)
			ln, err = net.Listen("tcp", "127.0.0.1:"+p)
		}
	}
	if err != nil {
		ln, err = net.Listen("tcp", "127.0.0.1:0")
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot open a local port:", err)
		os.Exit(1)
	}
	e := newEngine(dir)
	srv := &server{e: e, key: randHex(32), port: ln.Addr().(*net.TCPAddr).Port}
	if k := os.Getenv("DROPOMAT_KEY"); k != "" {
		srv.key = k
	}
	writeJSON(instPath, instanceFile{Port: srv.port, Key: srv.key})
	e.apiKey = srv.key
	e.winGen, _ = strconv.Atoi(os.Getenv("DROPOMAT_GEN"))
	e.applyLang()
	cleanupUpdate()
	if updated {
		e.log.OK("L.updDone", version)
	}
	e.onQuit = func() {
		os.Remove(instPath)
		time.AfterFunc(300*time.Millisecond, func() { os.Exit(0) })
	}
	go http.Serve(ln, srv.routes())
	go e.run()

	e.mu.Lock()
	open := !trayOnly && (e.set.OpenOnStart || e.auth.Token == "")
	e.mu.Unlock()
	if open && os.Getenv("DROPOMAT_NO_WINDOW") == "" {
		srv.show()
	}
	runTray(e, srv.show)
	e.shutdown()
	time.Sleep(400 * time.Millisecond)
}
