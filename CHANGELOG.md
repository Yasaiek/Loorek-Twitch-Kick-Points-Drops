# Changelog

## 0.8.3
- Daily rewards of Twitch's campaigns (the GTA VI badges: an hour a day on six days) show which day is next, how many days are collected (a dot per day), today's minutes with the time the day closes, or "done today" with the time the next day opens.
- A game whose daily reward is done for today is shown as "Done today" with the next round's time in the queue, not as finished.
- The journal says when a day is collected and when the next round opens.
- Fixed: for these rewards Twitch's watching session adds up the minutes of all days (e.g. 1454/60); Loorek took that as finished and kept stopping and restarting the stream every 40 seconds.

## 0.8.2
- Fixed: a daily reward of a Twitch campaign that Twitch grants by itself (GTA DVD Chest) was "claimed" again every ten seconds. Such a reward is now shown as done until its next day opens, and nothing is claimed unless Twitch says there is something to claim.
- Fixed: "drop finished" was written to the journal every minute, and the whole account was re-read every ten seconds, when Twitch counted the minutes on a reward that cannot be earned.
- A Twitch campaign that watching does not move at all is put aside for six hours instead of being watched for nothing.

## 0.8.1
- Discord status: a second button, "GitHub", next to "Get Loorek".

## 0.8.0
- Discord status (Rich Presence): Loorek can show on your Discord profile what it is doing – the drop being farmed with its progress, the queue, the tracked games and the channels points are collected on. It follows the language of the app.
- Settings > Discord status: switch it on or off as a whole, and choose which of those parts are shown. It is off until you switch it on.

## 0.7.2
- Twitch's own reward campaigns have a place of their own at the top of the campaign list: one wide bar per campaign with all its rewards and their progress. Game drops follow below as before.
- While such a campaign is being farmed its progress is re-read every two minutes (it used to move only every ten).

## 0.7.1
- The campaign search also matches campaign names, not only game names: "GTAVI Launch Celebration" sits under the game "Grand Theft Auto VI", so typing "gta" found nothing.
- Reward campaigns that count in many categories (the Pokémon ones count in 21 games) are listed under their own name instead of being hidden under the first game, and are farmed on whichever of those categories has a drops-enabled stream.
- In staged campaigns (evolutions) only the stages Twitch has opened are counted; the later ones are shown as locked.
- If Twitch times out on the very first campaign load, Loorek retries after 20 seconds instead of 2 minutes.

## 0.7.0
- Twitch's own reward campaigns are in the campaign list now: daily chests and badge collections such as "GTAVI Launch Celebration". They can be queued and tracked like any other campaign; a daily one is farmed again by itself when the next day opens.
- For these campaigns Loorek keeps Twitch's last answer in `data/quests-debug.json` (names and progress only), to make problems with this new kind of campaign easy to find.

## 0.6.4
- Fixed Twitch channel points not being earned: the "minute watched" report lacked two fields Twitch's web player sends, so Twitch accepted it but credited nothing.
- The journal now warns when a channel is watched for 25 minutes without a single point.

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
