<p align="center"><img src="docs/img/icon.png" width="96" alt=""></p>
<h1 align="center">Loorek – Twitch &amp; Kick Points and Drops Collector</h1>
<p align="center">A small Windows tray app that collects <b>Twitch and Kick channel points</b> and farms <b>Twitch Drops</b> in the background.<br>No browser tab, no video playback.</p>
<p align="center"><a href="README.md" title="English"><img src="docs/flags/en.svg" width="24" alt="English"></a> <a href="docs/readme/README.pl.md" title="Polski"><img src="docs/flags/pl.svg" width="24" alt="Polski"></a> <a href="docs/readme/README.de.md" title="Deutsch"><img src="docs/flags/de.svg" width="24" alt="Deutsch"></a> <a href="docs/readme/README.fr.md" title="Français"><img src="docs/flags/fr.svg" width="24" alt="Français"></a> <a href="docs/readme/README.es.md" title="Español"><img src="docs/flags/es.svg" width="24" alt="Español"></a> <a href="docs/readme/README.pt.md" title="Português (BR)"><img src="docs/flags/pt.svg" width="24" alt="Português (BR)"></a> <a href="docs/readme/README.it.md" title="Italiano"><img src="docs/flags/it.svg" width="24" alt="Italiano"></a> <a href="docs/readme/README.ru.md" title="Русский"><img src="docs/flags/ru.svg" width="24" alt="Русский"></a> <a href="docs/readme/README.uk.md" title="Українська"><img src="docs/flags/uk.svg" width="24" alt="Українська"></a> <a href="docs/readme/README.tr.md" title="Türkçe"><img src="docs/flags/tr.svg" width="24" alt="Türkçe"></a> <a href="docs/readme/README.ja.md" title="日本語"><img src="docs/flags/ja.svg" width="24" alt="日本語"></a> <a href="docs/readme/README.zh.md" title="简体中文"><img src="docs/flags/zh.svg" width="24" alt="简体中文"></a></p>
<p align="center">
  <a href="https://github.com/Yasaiek/Loorek-Twitch-Kick-Points-Drops/releases/latest/download/Loorek.exe"><b>⬇ Download Loorek.exe</b></a> &nbsp;·&nbsp;
  <a href="https://yasaiek.github.io/Loorek-Twitch-Kick-Points-Drops/"><b>🌐 Website</b></a> &nbsp;·&nbsp;
  <a href="CHANGELOG.md">Changelog</a> &nbsp;·&nbsp;
  <a href="https://github.com/Yasaiek/Loorek-Twitch-Kick-Points-Drops/issues/new">Report a bug</a>
</p>
<p align="center"><img src="docs/img/demo.gif" alt="Loorek" width="860"></p>

> **Beta.** Loorek is young and has been tested on a small number of setups. Expect rough edges and please open an issue when something does not work.

## What it does

- **Twitch channel points** – watches up to two channels at a time (that is all Twitch credits) and claims every bonus on its own.
- **Chat presence** – stays in the chats of all the channels on your list.
- **Twitch Drops** – queue games, see every campaign and reward, and let Loorek pick a stream, switch when it ends or stalls, and claim the drop.
- **Tracked games** – search for any game and track it; farming starts by itself when a campaign appears.
- **Kick channel points** – collects points on the Kick channels you choose.
- **Easy to set up** – channel suggestions with avatars while you type.
- **Runs in the tray** – close the panel and everything keeps going. It does not decode video and never touches other processes.
- **Updates itself** from the releases of this repository (SHA-256 checked).
- **12 languages**, switchable at any time.

| Drop campaigns | Queue |
| --- | --- |
| ![Drop campaigns](docs/img/camps.png) | ![Queue](docs/img/queue.png) |
| **Twitch points** | **Kick points** |
| ![Twitch points](docs/img/points.png) | ![Kick points](docs/img/kick.png) |

## Getting started

1. Download **Loorek.exe** and run it. There is no installer; the icon appears in the tray next to the clock and the panel opens.
2. Click **Sign in**. A separate sign-in window opens with Twitch – sign in as usual and the window closes by itself. Kick works the same way.
3. Add channels under **Points** and pick games under **Drops**. Order is priority.

Windows may warn about an unknown publisher, because the file is not code-signed. Every release is built from this source by GitHub Actions.

## Where your data lives

Everything Loorek stores is in `%LOCALAPPDATA%\Yasaiek\Loorek` (another place can be picked in Settings → Data folder):

| File | What it is |
| --- | --- |
| `loorek-settings.json` | settings, channel lists, queue |
| `auth.json` | your Twitch and Kick sessions – never share this file |
| `login-profile/` | the profile of the sign-in window – never share it either |
| `log.txt` | the journal |

Sessions are sent only to Twitch and Kick themselves. The sign-in window is the Edge/Chrome already installed on your computer, started with its own separate profile; Loorek does not read your everyday browser profile.

Loorek talks to: Twitch, Kick, and GitHub (update check). Nothing else.

## Build from source

Go 1.24 or newer:

```
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-H=windowsgui -s -w" -o Loorek.exe .
```

The panel is plain HTML in `ui/`, translations are JSON files in `ui/lang/`.

## Credits

Protocol knowledge comes from these projects – thank you:

- [TwitchDropsMiner](https://github.com/DevilXD/TwitchDropsMiner) – DevilXD (MIT)
- [Kick_Channel_Points_Miner](https://github.com/Baillora/Kick_Channel_Points_Miner) – Baillora (MIT)
- [fyne.io/systray](https://github.com/fyne-io/systray), [bogdanfinn/tls-client](https://github.com/bogdanfinn/tls-client)

Looking for a browser extension instead? See [Dropomat](https://github.com/Yasaiek/Dropomat-Auto-Twitch-Drops).

## Disclaimer

Automated watching is against the terms of service of Twitch and Kick. Loorek is not affiliated with either of them. You use it at your own risk.

<sub>MIT</sub>
