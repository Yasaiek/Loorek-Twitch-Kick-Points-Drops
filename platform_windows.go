//go:build windows

package main

import (
	"embed"
	"golang.org/x/sys/windows"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"

	"fyne.io/systray"
	"golang.org/x/sys/windows/registry"
)

//go:embed assets/icon.ico
var iconICO []byte

// runTray puts the icon next to the clock and blocks until the user quits.
func runTray(e *Engine, a trayActions) {
	darkMenus()
	systray.Run(func() {
		systray.SetIcon(iconICO)
		systray.SetTitle(appName)
		systray.SetTooltip(appName)
		systray.SetOnTapped(a.Open)

		head := systray.AddMenuItem(appName+" "+version, "")
		head.SetIcon(iconICO)
		mState := systray.AddMenuItem("", "")
		mUse := systray.AddMenuItem(usageLine(), "")
		mUse.SetIcon(menuIcon("usage"))
		systray.AddSeparator()
		mOpen := systray.AddMenuItem(tr("tray.open"), "")
		mOpen.SetIcon(menuIcon("open"))
		mPoints := systray.AddMenuItemCheckbox(tr("tray.points"), "", false)
		mDrops := systray.AddMenuItemCheckbox(tr("tray.drops"), "", false)
		mKick := systray.AddMenuItemCheckbox(tr("tray.kick"), "", false)
		systray.AddSeparator()
		mUpd := systray.AddMenuItem(tr("tray.update"), "")
		mUpd.SetIcon(menuIcon("update"))
		mSite := systray.AddMenuItem(tr("tray.site"), "")
		mSite.SetIcon(menuIcon("site"))
		mDir := systray.AddMenuItem(tr("tray.folder"), "")
		mDir.SetIcon(menuIcon("folder"))
		systray.AddSeparator()
		mQuit := systray.AddMenuItem(tr("quit"), "")
		mQuit.SetIcon(menuIcon("quit"))

		lastDot := ""
		check := func(m *systray.MenuItem, on bool) {
			if on != m.Checked() {
				if on {
					m.Check()
				} else {
					m.Uncheck()
				}
			}
		}
		refresh := func() {
			st := e.trayState()
			systray.SetTooltip(e.statusLine() + "\n" + usageLine())
			mState.SetTitle(st.Line)
			if dot := map[bool]string{true: "on", false: "off"}[st.Active]; dot != lastDot {
				mState.SetIcon(menuIcon(dot))
				lastDot = dot
			}
			mUse.SetTitle(usageLine())
			check(mPoints, st.Points)
			check(mDrops, st.Drops)
			check(mKick, st.Kick)
			mOpen.SetTitle(tr("tray.open"))
			mPoints.SetTitle(tr("tray.points"))
			mDrops.SetTitle(tr("tray.drops"))
			mKick.SetTitle(tr("tray.kick"))
			mUpd.SetTitle(tr("tray.update"))
			mSite.SetTitle(tr("tray.site"))
			mDir.SetTitle(tr("tray.folder"))
			mQuit.SetTitle(tr("quit"))
		}
		refresh()
		go func() {
			t := time.NewTicker(2 * time.Second)
			defer t.Stop()
			for {
				select {
				case <-mOpen.ClickedCh:
					a.Open()
				case <-head.ClickedCh:
					a.Open()
				case <-mState.ClickedCh:
					a.Open()
				case <-mUse.ClickedCh:
					a.Open()
				case <-mPoints.ClickedCh:
					a.Set("pointsOn", !mPoints.Checked())
					refresh()
				case <-mDrops.ClickedCh:
					a.Set("dropsOn", !mDrops.Checked())
					refresh()
				case <-mKick.ClickedCh:
					a.Set("kickOn", !mKick.Checked())
					refresh()
				case <-mUpd.ClickedCh:
					a.Update()
				case <-mSite.ClickedCh:
					openURL(siteURL)
				case <-mDir.ClickedCh:
					if exe, err := os.Executable(); err == nil {
						exec.Command("explorer", filepath.Dir(exe)).Start()
					}
				case <-mQuit.ClickedCh:
					systray.Quit()
					return
				case <-e.quit:
					systray.Quit()
					return
				case <-t.C:
					refresh()
				}
			}
		}()
	}, func() {})
}

//go:embed assets/menu
var menuIcons embed.FS

func menuIcon(name string) []byte {
	b, _ := menuIcons.ReadFile("assets/menu/" + name + ".ico")
	return b
}

// systemLocale is the user's Windows display language, like "pl-PL".
func systemLocale() string {
	buf := make([]uint16, 85)
	r, _, _ := syscall.NewLazyDLL("kernel32.dll").NewProc("GetUserDefaultLocaleName").Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if r == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf)
}

const runKey = `Software\Microsoft\Windows\CurrentVersion\Run`

// setAutostart adds or removes this program from the user's "start with Windows" list.
func setAutostart(on bool) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	k.DeleteValue("Dropomat") // the name this program had before 0.4
	if !on {
		err := k.DeleteValue(appName)
		if err == registry.ErrNotExist {
			return nil
		}
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return k.SetStringValue(appName, `"`+exe+`" --tray`)
}

// darkMenus makes the program's menus (the tray menu) dark, whatever the Windows theme. It uses the same
// switch Windows' own apps use (uxtheme ordinals 135 SetPreferredAppMode and 136 FlushMenuThemes, Windows 10
// 1903 and newer); on older Windows the call does nothing harmful.
func darkMenus() {
	h, err := windows.LoadLibrary("uxtheme.dll")
	if err != nil {
		return
	}
	if p, err := windows.GetProcAddressByOrdinal(h, 135); err == nil {
		syscall.SyscallN(p, 2) // ForceDark
	}
	if p, err := windows.GetProcAddressByOrdinal(h, 136); err == nil {
		syscall.SyscallN(p)
	}
}
