<p align="center"><img src="../img/icon.png" width="96" alt=""></p>
<h1 align="center">Loorek – Twitch &amp; Kick Points and Drops Collector</h1>
<p align="center">Sistem tepsisinde duran, arka planda <b>Twitch ve Kick kanal puanlarını</b> toplayan ve <b>Twitch Drops</b> kazanan küçük bir Windows uygulaması.<br>Tarayıcı sekmesi yok, video oynatma yok.</p>
<p align="center"><a href="../../README.md" title="English"><img src="../flags/en.svg" width="24" alt="English"></a> <a href="README.pl.md" title="Polski"><img src="../flags/pl.svg" width="24" alt="Polski"></a> <a href="README.de.md" title="Deutsch"><img src="../flags/de.svg" width="24" alt="Deutsch"></a> <a href="README.fr.md" title="Français"><img src="../flags/fr.svg" width="24" alt="Français"></a> <a href="README.es.md" title="Español"><img src="../flags/es.svg" width="24" alt="Español"></a> <a href="README.pt.md" title="Português (BR)"><img src="../flags/pt.svg" width="24" alt="Português (BR)"></a> <a href="README.it.md" title="Italiano"><img src="../flags/it.svg" width="24" alt="Italiano"></a> <a href="README.ru.md" title="Русский"><img src="../flags/ru.svg" width="24" alt="Русский"></a> <a href="README.uk.md" title="Українська"><img src="../flags/uk.svg" width="24" alt="Українська"></a> <a href="README.tr.md" title="Türkçe"><img src="../flags/tr.svg" width="24" alt="Türkçe"></a> <a href="README.ja.md" title="日本語"><img src="../flags/ja.svg" width="24" alt="日本語"></a> <a href="README.zh.md" title="简体中文"><img src="../flags/zh.svg" width="24" alt="简体中文"></a></p>
<p align="center">
  <a href="https://github.com/Yasaiek/Loorek-Twitch-Kick-Points-Drops/releases/latest/download/Loorek.exe"><b>⬇ Loorek.exe indir</b></a> &nbsp;·&nbsp;
  <a href="https://yasaiek.github.io/Loorek-Twitch-Kick-Points-Drops/"><b>🌐 Web sitesi</b></a> &nbsp;·&nbsp;
  <a href="../../CHANGELOG.md">Değişiklikler</a> &nbsp;·&nbsp;
  <a href="https://github.com/Yasaiek/Loorek-Twitch-Kick-Points-Drops/issues/new">Hata bildir</a>
</p>
<p align="center"><img src="../img/demo.gif" alt="Loorek" width="860"></p>

> **Beta.** Loorek henüz genç ve az sayıda bilgisayarda denendi. Pürüzler olabilir; bir şey çalışmazsa lütfen issue aç.

## Ne yapar

- **Twitch kanal puanları** – aynı anda en fazla iki kanalı izler (Twitch daha fazlasını saymaz) ve her bonusu kendisi alır.
- **Sohbette bulunma** – listendeki tüm kanalların sohbetinde kalır.
- **Twitch Drops** – oyunları sıraya koy, her kampanyayı ve ödülü gör; Loorek yayını seçer, bitince veya takılınca değiştirir ve drop'u alır.
- **Takip edilen oyunlar** – istediğin oyunu ara ve takip et; kampanya çıkınca kendiliğinden başlar.
- **Kick kanal puanları** – seçtiğin Kick kanallarında puan toplar.
- **Kolay kurulum** – yazarken avatarlı kanal önerileri.
- **Sistem tepsisinde çalışır** – paneli kapat, her şey devam eder. Video çözmez ve başka işlemlere dokunmaz.
- **Kendini günceller** – bu deponun sürümlerinden (SHA-256 doğrulamalı).
- **12 dil**, istediğin an değiştirilebilir.

| Drop kampanyaları | Sıra |
| --- | --- |
| ![Drop kampanyaları](../img/camps.png) | ![Sıra](../img/queue.png) |
| **Twitch puanları** | **Kick puanları** |
| ![Twitch puanları](../img/points.png) | ![Kick puanları](../img/kick.png) |

## Başlarken

1. **Loorek.exe** dosyasını indir ve çalıştır. Kurulum yok; simge saatin yanında belirir ve panel açılır.
2. **Giriş yap**'a tıkla. Ayrı bir Twitch giriş penceresi açılır – her zamanki gibi giriş yap, pencere kendiliğinden kapanır. Kick de aynı şekilde çalışır.
3. **Puanlar** bölümünden kanal ekle, **Droplar** bölümünden oyun seç. Sıra önceliktir.

Dosya imzalı olmadığı için Windows bilinmeyen yayıncı uyarısı verebilir. Her sürüm bu kaynak koddan GitHub Actions tarafından derlenir.

## Verilerin nerede

Loorek'in kaydettiği her şey `%LOCALAPPDATA%\Yasaiek\Loorek` içindedir (başka bir yer Ayarlar → Veri klasörü'nden seçilebilir):

| Dosya | Nedir |
| --- | --- |
| `loorek-settings.json` | ayarlar, kanal listeleri, sıra |
| `auth.json` | Twitch ve Kick oturumların – bu dosyayı asla paylaşma |
| `login-profile/` | giriş penceresinin profili – bunu da paylaşma |
| `log.txt` | günlük |

Oturumlar yalnızca Twitch ve Kick'in kendisine gönderilir. Giriş penceresi, bilgisayarında kurulu Edge/Chrome'un ayrı bir profille açılmış halidir; Loorek günlük tarayıcı profilini okumaz.

Loorek şunlarla konuşur: Twitch, Kick ve GitHub (güncelleme denetimi). Başka hiçbir şeyle.

## Kaynaktan derleme

Go 1.24 veya daha yenisi:

```
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-H=windowsgui -s -w" -o Loorek.exe .
```

Panel `ui/` içindeki düz HTML'dir, çeviriler `ui/lang/` içindeki JSON dosyalarıdır.

## Teşekkürler

Protokol bilgisi şu projelerden geliyor – teşekkürler:

- [TwitchDropsMiner](https://github.com/DevilXD/TwitchDropsMiner) – DevilXD (MIT)
- [Kick_Channel_Points_Miner](https://github.com/Baillora/Kick_Channel_Points_Miner) – Baillora (MIT)
- [fyne.io/systray](https://github.com/fyne-io/systray), [bogdanfinn/tls-client](https://github.com/bogdanfinn/tls-client)

Tarayıcı eklentisi mi istiyorsun? Bak: [Dropomat](https://github.com/Yasaiek/Dropomat-Auto-Twitch-Drops).

## Uyarı

Otomatik izleme, Twitch ve Kick hizmet şartlarına aykırıdır. Loorek ikisiyle de bağlantılı değildir. Kullanım riski sana aittir.

<sub>MIT</sub>
