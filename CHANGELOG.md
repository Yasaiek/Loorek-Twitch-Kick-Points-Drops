# Changelog

## 0.6.3
- Updates download their own copy of the program file, so the download count on the site shows real downloads, not every automatic update.

## 0.6.2
- The panel window asks the browser for a dark title bar in the app's own colour (0.6.1 tried the page's theme colour, which browsers ignore for this kind of window).

## 0.6.1
- Fixed Kick not working at all (search stuck on "Searching…", no channel data) when Loorek was started by a program that passes on the SSLKEYLOGFILE setting. Loorek now ignores that setting.
- The window's title bar takes the colour of the panel instead of the browser's grey.
- The status label in queue and tracked rows is centred; a search that gets no answer says so instead of waiting for ever.

## 0.6.0
- Simpler navigation: two sections, **Drops** (Drop campaigns, Queue, Tracked games) and **Points** (Twitch, Kick).
- Tracked games: search for any game and track it, also one that has no campaign right now.
- Channel avatars on the Twitch and Kick lists.
- Suggestions while typing a channel name (Twitch and Kick), so you pick the right one.
- Games in the queue and on the tracked list start folded; click a row to see its campaigns and rewards.

## 0.5.2
- Fixed: after an update the open panel could stay on "Downloading…" with "No connection" although the app had already updated and restarted. The new version now takes over the address of the old one, so the panel reloads by itself.
- The panel says clearly that the update is done ("Updated to version X") and, while downloading, that the app will restart by itself.
- When the panel loses the app for longer, it tells you to reopen it from the tray icon.
- Starting a newer Loorek.exe by hand while an older one runs keeps the open panel working too.
- Small fixes: a timing race between the panel and the watcher, the panel page is served to local addresses only.

## 0.5.1
- In-app language switch with flags; change language any time, including on the sign-in screen.
- Track a drop game with a visible star button on each campaign card; clearer campaign icons.
- Kick sign-in window opens the login form itself (Kick has no direct login URL).
- Real game covers and the overview now show Kick channels and the drop being farmed.

## 0.5.0
- New name and icon: **Loorek**.
- Sign-in window for Twitch and Kick: sign in like on any website, the app picks up the session and closes the window.
- Automatic updates from GitHub releases (download, SHA-256 check, swap, restart in the tray).
- Interface, journal and tray in 12 languages; follows the system language by default.
- Real game covers in the campaign list.

## 0.3.0
- Kick channel points.

## 0.2.0
- Bigger panel, full campaign and reward details.

## 0.1.0
- First version: Twitch channel points, bonus claiming, chat presence, Twitch Drops queue.
