<p align="center"><img src="../img/icon.png" width="96" alt=""></p>
<h1 align="center">Loorek – Twitch &amp; Kick Points and Drops Collector</h1>
<p align="center">Eine kleine Windows-App im Infobereich, die <b>Kanalpunkte auf Twitch und Kick</b> sammelt und <b>Twitch Drops</b> im Hintergrund holt.<br>Kein Browser-Tab, keine Videowiedergabe.</p>
<p align="center"><a href="../../README.md" title="English"><img src="../flags/en.svg" width="24" alt="English"></a> <a href="README.pl.md" title="Polski"><img src="../flags/pl.svg" width="24" alt="Polski"></a> <a href="README.de.md" title="Deutsch"><img src="../flags/de.svg" width="24" alt="Deutsch"></a> <a href="README.fr.md" title="Français"><img src="../flags/fr.svg" width="24" alt="Français"></a> <a href="README.es.md" title="Español"><img src="../flags/es.svg" width="24" alt="Español"></a> <a href="README.pt.md" title="Português (BR)"><img src="../flags/pt.svg" width="24" alt="Português (BR)"></a> <a href="README.it.md" title="Italiano"><img src="../flags/it.svg" width="24" alt="Italiano"></a> <a href="README.ru.md" title="Русский"><img src="../flags/ru.svg" width="24" alt="Русский"></a> <a href="README.uk.md" title="Українська"><img src="../flags/uk.svg" width="24" alt="Українська"></a> <a href="README.tr.md" title="Türkçe"><img src="../flags/tr.svg" width="24" alt="Türkçe"></a> <a href="README.ja.md" title="日本語"><img src="../flags/ja.svg" width="24" alt="日本語"></a> <a href="README.zh.md" title="简体中文"><img src="../flags/zh.svg" width="24" alt="简体中文"></a></p>
<p align="center">
  <a href="https://github.com/Yasaiek/Loorek-Twitch-Kick-Points-Drops/releases/latest/download/Loorek.exe"><b>⬇ Loorek.exe herunterladen</b></a> &nbsp;·&nbsp;
  <a href="https://yasaiek.github.io/Loorek-Twitch-Kick-Points-Drops/"><b>🌐 Website</b></a> &nbsp;·&nbsp;
  <a href="../../CHANGELOG.md">Änderungen</a> &nbsp;·&nbsp;
  <a href="https://github.com/Yasaiek/Loorek-Twitch-Kick-Points-Drops/issues/new">Fehler melden</a>
</p>
<p align="center"><img src="../img/demo.gif" alt="Loorek" width="860"></p>

> **Beta.** Loorek ist jung und wurde nur auf wenigen Rechnern getestet. Rechne mit Ecken und Kanten und eröffne bitte ein Issue, wenn etwas nicht funktioniert.

## Was es tut

- **Twitch-Kanalpunkte** – schaut bis zu zwei Kanäle gleichzeitig (mehr zählt Twitch nicht) und holt jeden Bonus selbst ab.
- **Im Chat anwesend** – bleibt in den Chats aller Kanäle auf deiner Liste.
- **Twitch Drops** – Spiele einreihen, jede Kampagne und Belohnung sehen; Loorek wählt den Stream, wechselt, wenn er endet oder hängt, und holt den Drop ab.
- **Beobachtete Spiele** – beliebiges Spiel suchen und beobachten; sobald eine Kampagne erscheint, geht es von selbst los.
- **Kick-Kanalpunkte** – sammelt Punkte auf den Kick-Kanälen deiner Wahl.
- **Schnell eingerichtet** – Kanalvorschläge mit Avataren schon beim Tippen.
- **Läuft im Infobereich** – Panel schließen, alles läuft weiter. Dekodiert kein Video und fasst keine anderen Prozesse an.
- **Aktualisiert sich selbst** aus den Releases dieses Repositorys (SHA-256 geprüft).
- **12 Sprachen**, jederzeit umschaltbar.

| Drop-Kampagnen | Warteschlange |
| --- | --- |
| ![Drop-Kampagnen](../img/camps.png) | ![Warteschlange](../img/queue.png) |
| **Twitch-Punkte** | **Kick-Punkte** |
| ![Twitch-Punkte](../img/points.png) | ![Kick-Punkte](../img/kick.png) |

## Erste Schritte

1. **Loorek.exe** herunterladen und starten. Kein Installer; das Symbol erscheint im Infobereich neben der Uhr und das Panel öffnet sich.
2. Auf **Anmelden** klicken. Ein eigenes Anmeldefenster für Twitch öffnet sich – wie gewohnt anmelden, das Fenster schließt sich von selbst. Bei Kick genauso.
3. Kanäle unter **Punkte** hinzufügen und Spiele unter **Drops** wählen. Die Reihenfolge ist die Priorität.

Windows warnt eventuell vor einem unbekannten Herausgeber, weil die Datei nicht signiert ist. Jedes Release wird von GitHub Actions aus diesem Quellcode gebaut.

## Wo deine Daten liegen

Alles, was Loorek speichert, liegt im Ordner `data` neben `Loorek.exe`:

| Datei | Was es ist |
| --- | --- |
| `settings.json` | Einstellungen, Kanallisten, Warteschlange |
| `auth.json` | deine Twitch- und Kick-Sitzungen – gib diese Datei nie weiter |
| `login-profile/` | das Profil des Anmeldefensters – ebenfalls nie weitergeben |
| `log.txt` | das Journal |

Sitzungen gehen nur an Twitch und Kick selbst. Das Anmeldefenster ist das bereits installierte Edge/Chrome mit einem eigenen Profil; Loorek liest dein normales Browserprofil nicht.

Loorek spricht mit: Twitch, Kick und GitHub (Update-Prüfung). Sonst mit nichts.

## Aus dem Quellcode bauen

Go 1.24 oder neuer:

```
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-H=windowsgui -s -w" -o Loorek.exe .
```

Das Panel ist einfaches HTML in `ui/`, die Übersetzungen sind JSON-Dateien in `ui/lang/`.

## Danksagung

Das Protokollwissen stammt aus diesen Projekten – danke:

- [TwitchDropsMiner](https://github.com/DevilXD/TwitchDropsMiner) – DevilXD (MIT)
- [Kick_Channel_Points_Miner](https://github.com/Baillora/Kick_Channel_Points_Miner) – Baillora (MIT)
- [fyne.io/systray](https://github.com/fyne-io/systray), [bogdanfinn/tls-client](https://github.com/bogdanfinn/tls-client)

Lieber eine Browser-Erweiterung? Siehe [Dropomat](https://github.com/Yasaiek/Dropomat-Auto-Twitch-Drops).

## Hinweis

Automatisches Zuschauen verstößt gegen die Nutzungsbedingungen von Twitch und Kick. Loorek steht mit keinem von beiden in Verbindung. Nutzung auf eigenes Risiko.

<sub>MIT</sub>
