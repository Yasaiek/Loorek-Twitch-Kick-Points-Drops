<p align="center"><img src="../img/icon.png" width="96" alt=""></p>
<h1 align="center">Loorek – Twitch &amp; Kick Points and Drops Collector</h1>
<p align="center">Невеликий застосунок для Windows в області сповіщень, який збирає <b>бали каналу на Twitch і Kick</b> та отримує <b>Twitch Drops</b> у фоні.<br>Без вкладки браузера і без відтворення відео.</p>
<p align="center"><a href="../../README.md" title="English"><img src="../flags/en.svg" width="24" alt="English"></a> <a href="README.pl.md" title="Polski"><img src="../flags/pl.svg" width="24" alt="Polski"></a> <a href="README.de.md" title="Deutsch"><img src="../flags/de.svg" width="24" alt="Deutsch"></a> <a href="README.fr.md" title="Français"><img src="../flags/fr.svg" width="24" alt="Français"></a> <a href="README.es.md" title="Español"><img src="../flags/es.svg" width="24" alt="Español"></a> <a href="README.pt.md" title="Português (BR)"><img src="../flags/pt.svg" width="24" alt="Português (BR)"></a> <a href="README.it.md" title="Italiano"><img src="../flags/it.svg" width="24" alt="Italiano"></a> <a href="README.ru.md" title="Русский"><img src="../flags/ru.svg" width="24" alt="Русский"></a> <a href="README.uk.md" title="Українська"><img src="../flags/uk.svg" width="24" alt="Українська"></a> <a href="README.tr.md" title="Türkçe"><img src="../flags/tr.svg" width="24" alt="Türkçe"></a> <a href="README.ja.md" title="日本語"><img src="../flags/ja.svg" width="24" alt="日本語"></a> <a href="README.zh.md" title="简体中文"><img src="../flags/zh.svg" width="24" alt="简体中文"></a></p>
<p align="center">
  <a href="https://github.com/Yasaiek/Loorek-Twitch-Kick-Points-Drops/releases/latest/download/Loorek.exe"><b>⬇ Завантажити Loorek.exe</b></a> &nbsp;·&nbsp;
  <a href="https://yasaiek.github.io/Loorek-Twitch-Kick-Points-Drops/"><b>🌐 Сайт</b></a> &nbsp;·&nbsp;
  <a href="../../CHANGELOG.md">Список змін</a> &nbsp;·&nbsp;
  <a href="https://github.com/Yasaiek/Loorek-Twitch-Kick-Points-Drops/issues/new">Повідомити про помилку</a>
</p>
<p align="center"><img src="../img/demo.gif" alt="Loorek" width="860"></p>

> **Beta.** Loorek ще молодий і перевірений на невеликій кількості комп'ютерів. Можливі недоліки – якщо щось не працює, створіть issue.

## Що він робить

- **Бали каналу Twitch** – дивиться до двох каналів одночасно (більше Twitch не зараховує) і сам забирає кожен бонус.
- **Присутність у чаті** – залишається в чатах усіх каналів із вашого списку.
- **Twitch Drops** – поставте ігри в чергу, бачте кожну кампанію й нагороду; Loorek сам обирає стрим, змінює його, коли той закінчився чи завис, і забирає дроп.
- **Відстежувані ігри** – знайдіть будь-яку гру й відстежуйте її; отримання почнеться саме, коли з'явиться кампанія.
- **Бали каналу Kick** – збирає бали на вибраних каналах Kick.
- **Просте налаштування** – підказки каналів з аватарами під час введення.
- **Працює в області сповіщень** – закрийте панель, усе працює далі. Не декодує відео й не чіпає інші процеси.
- **Оновлюється сам** із релізів цього репозиторію (з перевіркою SHA-256).
- **12 мов**, перемикаються будь-коли.

| Кампанії дропів | Черга |
| --- | --- |
| ![Кампанії дропів](../img/camps.png) | ![Черга](../img/queue.png) |
| **Бали Twitch** | **Бали Kick** |
| ![Бали Twitch](../img/points.png) | ![Бали Kick](../img/kick.png) |

## Початок роботи

1. Завантажте **Loorek.exe** і запустіть. Інсталятора немає; значок з'явиться біля годинника, і відкриється панель.
2. Натисніть **Увійти**. Відкриється окреме вікно входу Twitch – увійдіть як завжди, вікно закриється саме. З Kick так само.
3. Додайте канали в розділі **Бали** й виберіть ігри в розділі **Дропи**. Порядок – це пріоритет.

Windows може попередити про невідомого видавця, бо файл не підписано. Кожен реліз збирається з цього коду в GitHub Actions.

## Де зберігаються ваші дані

Усе, що зберігає Loorek, лежить у `%LOCALAPPDATA%\Yasaiek\Loorek` (інше місце можна вибрати в Налаштування → Тека даних):

| Файл | Що це |
| --- | --- |
| `loorek-settings.json` | налаштування, списки каналів, черга |
| `auth.json` | ваші сесії Twitch і Kick – нікому не надсилайте цей файл |
| `login-profile/` | профіль вікна входу – його теж не надсилайте |
| `log.txt` | журнал |

Сесії надсилаються лише самим Twitch і Kick. Вікно входу – це вже встановлений Edge/Chrome з окремим профілем; Loorek не читає ваш звичайний профіль браузера.

Loorek звертається до: Twitch, Kick і GitHub (перевірка оновлень). Більше ні до чого.

## Збирання з вихідного коду

Go 1.24 або новіше:

```
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-H=windowsgui -s -w" -o Loorek.exe .
```

Панель – звичайний HTML у `ui/`, переклади – файли JSON у `ui/lang/`.

## Подяки

Знання про протоколи взято з цих проєктів – дякую:

- [TwitchDropsMiner](https://github.com/DevilXD/TwitchDropsMiner) – DevilXD (MIT)
- [Kick_Channel_Points_Miner](https://github.com/Baillora/Kick_Channel_Points_Miner) – Baillora (MIT)
- [fyne.io/systray](https://github.com/fyne-io/systray), [bogdanfinn/tls-client](https://github.com/bogdanfinn/tls-client)

Потрібне розширення для браузера? Дивіться [Dropomat](https://github.com/Yasaiek/Dropomat-Auto-Twitch-Drops).

## Застереження

Автоматичний перегляд порушує умови використання Twitch і Kick. Loorek не пов'язаний із жодним із них. Ви використовуєте його на власний ризик.

<sub>MIT</sub>
