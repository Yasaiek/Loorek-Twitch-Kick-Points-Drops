package main

import (
	"os"
	"path/filepath"
)

// hasData: Loorek's files are already in this folder.
func hasData(dir string) bool {
	for _, n := range []string{settingsFile, "settings.json", "auth.json"} {
		if _, err := os.Stat(filepath.Join(dir, n)); err == nil {
			return true
		}
	}
	return false
}

// legacyData brings the files over from where versions up to 0.9.3 kept them: "data" next to the
// program file. A "data" folder that holds only Orbek's leftovers is not Loorek's and stays.
func legacyData(dir string) {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	old := filepath.Join(filepath.Dir(exe), "data")
	if hasData(old) {
		moveTree(old, dir)
	}
}

func dirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}
