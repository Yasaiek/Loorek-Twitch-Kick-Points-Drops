<p align="center"><img src="../img/icon.png" width="96" alt=""></p>
<h1 align="center">Loorek – Twitch &amp; Kick Points and Drops Collector</h1>
<p align="center">Mała aplikacja na Windowsa siedząca w zasobniku, która zbiera <b>punkty kanału na Twitchu i Kicku</b> oraz wbija <b>dropy Twitcha</b> w tle.<br>Bez karty w przeglądarce i bez odtwarzania wideo.</p>
<p align="center"><a href="../../README.md" title="English"><img src="../flags/en.svg" width="24" alt="English"></a> <a href="README.pl.md" title="Polski"><img src="../flags/pl.svg" width="24" alt="Polski"></a> <a href="README.de.md" title="Deutsch"><img src="../flags/de.svg" width="24" alt="Deutsch"></a> <a href="README.fr.md" title="Français"><img src="../flags/fr.svg" width="24" alt="Français"></a> <a href="README.es.md" title="Español"><img src="../flags/es.svg" width="24" alt="Español"></a> <a href="README.pt.md" title="Português (BR)"><img src="../flags/pt.svg" width="24" alt="Português (BR)"></a> <a href="README.it.md" title="Italiano"><img src="../flags/it.svg" width="24" alt="Italiano"></a> <a href="README.ru.md" title="Русский"><img src="../flags/ru.svg" width="24" alt="Русский"></a> <a href="README.uk.md" title="Українська"><img src="../flags/uk.svg" width="24" alt="Українська"></a> <a href="README.tr.md" title="Türkçe"><img src="../flags/tr.svg" width="24" alt="Türkçe"></a> <a href="README.ja.md" title="日本語"><img src="../flags/ja.svg" width="24" alt="日本語"></a> <a href="README.zh.md" title="简体中文"><img src="../flags/zh.svg" width="24" alt="简体中文"></a></p>
<p align="center">
  <a href="https://github.com/Yasaiek/Loorek-Twitch-Kick-Points-Drops/releases/latest/download/Loorek.exe"><b>⬇ Pobierz Loorek.exe</b></a> &nbsp;·&nbsp;
  <a href="https://yasaiek.github.io/Loorek-Twitch-Kick-Points-Drops/"><b>🌐 Strona</b></a> &nbsp;·&nbsp;
  <a href="../../CHANGELOG.md">Lista zmian</a> &nbsp;·&nbsp;
  <a href="https://github.com/Yasaiek/Loorek-Twitch-Kick-Points-Drops/issues/new">Zgłoś błąd</a>
</p>
<p align="center"><img src="../img/demo.gif" alt="Loorek" width="860"></p>

> **Beta.** Loorek jest młody i był testowany na niewielu komputerach. Coś może nie działać – zgłoś to wtedy w zakładce Issues.

## Co robi

- **Punkty kanału na Twitchu** – ogląda najwyżej dwa kanały naraz (tyle liczy Twitch) i sam odbiera każdy bonus.
- **Obecność na czacie** – siedzi na czatach wszystkich kanałów z Twojej listy.
- **Dropy Twitcha** – ustawiasz kolejkę gier, widzisz każdą kampanię i nagrodę, a Loorek sam wybiera stream, zmienia go, gdy się skończy albo stanie, i odbiera drop.
- **Śledzone gry** – wyszukaj dowolną grę i ją śledź; wbijanie ruszy samo, gdy pojawi się kampania.
- **Punkty kanału na Kicku** – zbiera punkty na wybranych kanałach Kicka.
- **Łatwa konfiguracja** – podpowiedzi kanałów z awatarami już podczas wpisywania.
- **Działa w zasobniku** – zamknij panel, a wszystko działa dalej. Nie dekoduje wideo i nie dotyka innych procesów.
- **Sam się aktualizuje** z wydań tego repozytorium (sprawdzana suma SHA-256).
- **12 języków**, do zmiany w każdej chwili.

| Kampanie dropów | Kolejka |
| --- | --- |
| ![Kampanie dropów](../img/camps.png) | ![Kolejka](../img/queue.png) |
| **Punkty na Twitchu** | **Punkty na Kicku** |
| ![Punkty na Twitchu](../img/points.png) | ![Punkty na Kicku](../img/kick.png) |

## Pierwsze kroki

1. Pobierz **Loorek.exe** i uruchom. Nie ma instalatora; ikona pojawi się w zasobniku obok zegara, a panel otworzy się sam.
2. Kliknij **Zaloguj się**. Otworzy się osobne okno logowania Twitcha – zaloguj się jak zwykle, okno zamknie się samo. Z Kickiem jest tak samo.
3. Dodaj kanały w sekcji **Punkty** i wybierz gry w sekcji **Dropy**. Kolejność to priorytet.

Windows może ostrzec o nieznanym wydawcy, bo plik nie jest podpisany. Każde wydanie buduje z tego kodu GitHub Actions.

## Gdzie są Twoje dane

Wszystko, co Loorek zapisuje, jest w folderze `data` obok `Loorek.exe`:

| Plik | Co to jest |
| --- | --- |
| `settings.json` | ustawienia, listy kanałów, kolejka |
| `auth.json` | Twoje sesje Twitcha i Kicka – nikomu nie wysyłaj tego pliku |
| `login-profile/` | profil okna logowania – jego też nie wysyłaj |
| `log.txt` | dziennik |

Sesje trafiają wyłącznie do Twitcha i Kicka. Okno logowania to Edge/Chrome zainstalowany na Twoim komputerze, uruchomiony z osobnym profilem; Loorek nie zagląda do Twojego codziennego profilu przeglądarki.

Loorek łączy się z: Twitchem, Kickiem i GitHubem (sprawdzanie aktualizacji). Z niczym więcej.

## Budowanie ze źródeł

Go 1.24 lub nowsze:

```
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-H=windowsgui -s -w" -o Loorek.exe .
```

Panel to zwykły HTML w `ui/`, tłumaczenia to pliki JSON w `ui/lang/`.

## Podziękowania

Wiedza o protokołach pochodzi z tych projektów – dziękuję:

- [TwitchDropsMiner](https://github.com/DevilXD/TwitchDropsMiner) – DevilXD (MIT)
- [Kick_Channel_Points_Miner](https://github.com/Baillora/Kick_Channel_Points_Miner) – Baillora (MIT)
- [fyne.io/systray](https://github.com/fyne-io/systray), [bogdanfinn/tls-client](https://github.com/bogdanfinn/tls-client)

Wolisz rozszerzenie do przeglądarki? Zobacz [Dropomat](https://github.com/Yasaiek/Dropomat-Auto-Twitch-Drops).

## Zastrzeżenie

Automatyczne oglądanie jest niezgodne z regulaminami Twitcha i Kicka. Loorek nie jest z nimi powiązany. Używasz go na własną odpowiedzialność.

<sub>MIT</sub>
