package main

import "strings"

// trayActions are what the tray menu can do.
type trayActions struct {
	Open, Update, Quit func()
	Set                func(key string, on bool) // a switch from the settings
}

type trayInfo struct {
	Line                string
	Active              bool // signed in and watching something
	Points, Drops, Kick bool
}

// trayState is the line under the program's name in the tray menu, and the switches.
func (e *Engine) trayState() trayInfo {
	e.mu.Lock()
	defer e.mu.Unlock()
	st := trayInfo{Points: e.set.PointsOn, Drops: e.set.DropsOn, Kick: e.set.KickOn}
	if e.authState != "ok" {
		st.Line = tr("notSigned")
		return st
	}
	var names []string
	for _, ch := range e.slots {
		names = append(names, ch.Name)
	}
	if len(names) == 0 {
		st.Line = tr("status.wait")
		return st
	}
	st.Active = true
	st.Line = tr("status.watching") + " " + strings.Join(names, ", ")
	if r := []rune(st.Line); len(r) > 70 {
		st.Line = string(r[:69]) + "…"
	}
	return st
}
