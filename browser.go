package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// findBrowser looks for a Chromium-based browser already installed on the computer (Edge first: every Windows has it).
func findBrowser() string {
	if p := os.Getenv("DROPOMAT_BROWSER"); p != "" {
		return p
	}
	var cands []string
	if runtime.GOOS == "windows" {
		roots := []string{os.Getenv("ProgramFiles(x86)"), os.Getenv("ProgramFiles"), os.Getenv("LocalAppData")}
		for _, rel := range []string{`Microsoft\Edge\Application\msedge.exe`, `Google\Chrome\Application\chrome.exe`, `BraveSoftware\Brave-Browser\Application\brave.exe`} {
			for _, r := range roots {
				if r != "" {
					cands = append(cands, filepath.Join(r, rel))
				}
			}
		}
	} else {
		for _, n := range []string{"microsoft-edge", "google-chrome", "chromium", "chromium-browser"} {
			if p, err := exec.LookPath(n); err == nil {
				cands = append(cands, p)
			}
		}
	}
	for _, c := range cands {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c
		}
	}
	return ""
}

func extraFlags() []string {
	if f := os.Getenv("DROPOMAT_BROWSER_FLAGS"); f != "" {
		return strings.Fields(f)
	}
	return nil
}

// openPanel shows the control panel as a chromeless app window; the window is a separate browser profile
// that exists only while the panel is open.
func openPanel(url, dataDir string) error {
	exe := findBrowser()
	if exe == "" {
		if runtime.GOOS == "windows" {
			return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
		}
		return exec.Command("xdg-open", url).Start()
	}
	args := []string{
		"--app=" + url,
		"--user-data-dir=" + filepath.Join(dataDir, "panel-profile"),
		"--window-size=1100,760",
		"--no-first-run", "--no-default-browser-check", "--disable-sync",
		"--disable-background-networking", "--disable-extensions",
	}
	args = append(args, extraFlags()...)
	cmd := exec.Command(exe, args...)
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait()
	return nil
}

var reToken = regexp.MustCompile(`access_token=([A-Za-z0-9]+)`)
var reBareToken = regexp.MustCompile(`^[A-Za-z0-9]{20,64}$`)

// openURL opens an address in the user's default browser.
func openURL(u string) error {
	if runtime.GOOS == "windows" {
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", u).Start()
	}
	if os.Getenv("DROPOMAT_NO_WINDOW") != "" {
		return nil
	}
	return exec.Command("xdg-open", u).Start()
}

// loginURL is Twitch's own sign-in page asked to hand a session for the Twitch app back to twitch.tv.
// In a browser where the user is already signed in it redirects straight away, without asking for a password.
func loginURL(www string) string {
	return www + "/login?client_id=" + clientID + "&redirect_params=client_id%3D" + clientID +
		"%26redirect_uri%3Dhttps%253A%252F%252Fwww.twitch.tv%252F%26response_type%3Dtoken"
}

// submitToken takes what the user pasted: the whole address from the browser, or just the token.
func (e *Engine) submitToken(raw string) string {
	raw = strings.TrimSpace(raw)
	token := ""
	if m := reToken.FindStringSubmatch(raw); m != nil {
		token = m[1]
	} else if reBareToken.MatchString(raw) {
		token = raw
	}
	if token == "" {
		return "bad"
	}
	e.setToken(token)
	return "ok"
}
