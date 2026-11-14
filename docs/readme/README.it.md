<p align="center"><img src="../img/icon.png" width="96" alt=""></p>
<h1 align="center">Loorek – Twitch &amp; Kick Points and Drops Collector</h1>
<p align="center">Una piccola app per Windows nell'area di notifica che raccoglie i <b>punti canale di Twitch e Kick</b> e ottiene i <b>Twitch Drops</b> in background.<br>Nessuna scheda del browser, nessun video riprodotto.</p>
<p align="center"><a href="../../README.md" title="English"><img src="../flags/en.svg" width="24" alt="English"></a> <a href="README.pl.md" title="Polski"><img src="../flags/pl.svg" width="24" alt="Polski"></a> <a href="README.de.md" title="Deutsch"><img src="../flags/de.svg" width="24" alt="Deutsch"></a> <a href="README.fr.md" title="Français"><img src="../flags/fr.svg" width="24" alt="Français"></a> <a href="README.es.md" title="Español"><img src="../flags/es.svg" width="24" alt="Español"></a> <a href="README.pt.md" title="Português (BR)"><img src="../flags/pt.svg" width="24" alt="Português (BR)"></a> <a href="README.it.md" title="Italiano"><img src="../flags/it.svg" width="24" alt="Italiano"></a> <a href="README.ru.md" title="Русский"><img src="../flags/ru.svg" width="24" alt="Русский"></a> <a href="README.uk.md" title="Українська"><img src="../flags/uk.svg" width="24" alt="Українська"></a> <a href="README.tr.md" title="Türkçe"><img src="../flags/tr.svg" width="24" alt="Türkçe"></a> <a href="README.ja.md" title="日本語"><img src="../flags/ja.svg" width="24" alt="日本語"></a> <a href="README.zh.md" title="简体中文"><img src="../flags/zh.svg" width="24" alt="简体中文"></a></p>
<p align="center">
  <a href="https://github.com/Yasaiek/Loorek-Twitch-Kick-Points-Drops/releases/latest/download/Loorek.exe"><b>⬇ Scarica Loorek.exe</b></a> &nbsp;·&nbsp;
  <a href="https://yasaiek.github.io/Loorek-Twitch-Kick-Points-Drops/"><b>🌐 Sito web</b></a> &nbsp;·&nbsp;
  <a href="../../CHANGELOG.md">Novità</a> &nbsp;·&nbsp;
  <a href="https://github.com/Yasaiek/Loorek-Twitch-Kick-Points-Drops/issues/new">Segnala un bug</a>
</p>
<p align="center"><img src="../img/demo.gif" alt="Loorek" width="860"></p>

> **Beta.** Loorek è giovane ed è stato provato su pochi computer. Qualcosa può non funzionare: in quel caso apri una issue.

## Cosa fa

- **Punti canale di Twitch** – guarda fino a due canali alla volta (di più Twitch non conta) e riscatta ogni bonus da solo.
- **Presenza in chat** – resta nelle chat di tutti i canali della tua lista.
- **Twitch Drops** – metti i giochi in coda, vedi ogni campagna e ricompensa; Loorek sceglie la live, la cambia quando finisce o si blocca e riscatta il drop.
- **Giochi seguiti** – cerca qualsiasi gioco e seguilo; parte da solo quando compare una campagna.
- **Punti canale di Kick** – raccoglie punti sui canali Kick che scegli.
- **Facile da configurare** – suggerimenti dei canali con avatar mentre scrivi.
- **Gira nell'area di notifica** – chiudi il pannello e tutto continua. Non decodifica video e non tocca altri processi.
- **Si aggiorna da solo** dalle release di questo repository (SHA-256 verificato).
- **12 lingue**, modificabili in ogni momento.

| Campagne drop | Coda |
| --- | --- |
| ![Campagne drop](../img/camps.png) | ![Coda](../img/queue.png) |
| **Punti Twitch** | **Punti Kick** |
| ![Punti Twitch](../img/points.png) | ![Punti Kick](../img/kick.png) |

## Per iniziare

1. Scarica **Loorek.exe** e avvialo. Non c'è installer; l'icona appare accanto all'orologio e si apre il pannello.
2. Clicca **Accedi**. Si apre una finestra separata di Twitch – accedi come sempre e si chiuderà da sola. Con Kick è uguale.
3. Aggiungi i canali in **Punti** e scegli i giochi in **Drop**. L'ordine è la priorità.

Windows può avvisare di un editore sconosciuto perché il file non è firmato. Ogni release è compilata da GitHub Actions a partire da questo codice.

## Dove sono i tuoi dati

Tutto ciò che Loorek salva è nella cartella `data` accanto a `Loorek.exe`:

| File | Cos'è |
| --- | --- |
| `settings.json` | impostazioni, elenchi dei canali, coda |
| `auth.json` | le tue sessioni Twitch e Kick – non condividere mai questo file |
| `login-profile/` | il profilo della finestra di accesso – non condividere nemmeno questo |
| `log.txt` | il diario |

Le sessioni vengono inviate solo a Twitch e Kick. La finestra di accesso è l'Edge/Chrome già installato, avviato con un profilo separato; Loorek non legge il tuo profilo abituale del browser.

Loorek comunica con: Twitch, Kick e GitHub (controllo aggiornamenti). Nient'altro.

## Compilare dai sorgenti

Go 1.24 o successivo:

```
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-H=windowsgui -s -w" -o Loorek.exe .
```

Il pannello è semplice HTML in `ui/`, le traduzioni sono file JSON in `ui/lang/`.

## Ringraziamenti

La conoscenza dei protocolli viene da questi progetti – grazie:

- [TwitchDropsMiner](https://github.com/DevilXD/TwitchDropsMiner) – DevilXD (MIT)
- [Kick_Channel_Points_Miner](https://github.com/Baillora/Kick_Channel_Points_Miner) – Baillora (MIT)
- [fyne.io/systray](https://github.com/fyne-io/systray), [bogdanfinn/tls-client](https://github.com/bogdanfinn/tls-client)

Preferisci un'estensione del browser? Vedi [Dropomat](https://github.com/Yasaiek/Dropomat-Auto-Twitch-Drops).

## Avvertenza

La visione automatizzata viola i termini di servizio di Twitch e Kick. Loorek non è affiliato a nessuno dei due. Lo usi a tuo rischio.

<sub>MIT</sub>
