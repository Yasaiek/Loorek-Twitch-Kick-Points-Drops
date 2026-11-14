<p align="center"><img src="../img/icon.png" width="96" alt=""></p>
<h1 align="center">Loorek – Twitch &amp; Kick Points and Drops Collector</h1>
<p align="center">Небольшое приложение для Windows в области уведомлений, которое собирает <b>баллы канала на Twitch и Kick</b> и получает <b>Twitch Drops</b> в фоне.<br>Без вкладки браузера и без воспроизведения видео.</p>
<p align="center"><a href="../../README.md" title="English"><img src="../flags/en.svg" width="24" alt="English"></a> <a href="README.pl.md" title="Polski"><img src="../flags/pl.svg" width="24" alt="Polski"></a> <a href="README.de.md" title="Deutsch"><img src="../flags/de.svg" width="24" alt="Deutsch"></a> <a href="README.fr.md" title="Français"><img src="../flags/fr.svg" width="24" alt="Français"></a> <a href="README.es.md" title="Español"><img src="../flags/es.svg" width="24" alt="Español"></a> <a href="README.pt.md" title="Português (BR)"><img src="../flags/pt.svg" width="24" alt="Português (BR)"></a> <a href="README.it.md" title="Italiano"><img src="../flags/it.svg" width="24" alt="Italiano"></a> <a href="README.ru.md" title="Русский"><img src="../flags/ru.svg" width="24" alt="Русский"></a> <a href="README.uk.md" title="Українська"><img src="../flags/uk.svg" width="24" alt="Українська"></a> <a href="README.tr.md" title="Türkçe"><img src="../flags/tr.svg" width="24" alt="Türkçe"></a> <a href="README.ja.md" title="日本語"><img src="../flags/ja.svg" width="24" alt="日本語"></a> <a href="README.zh.md" title="简体中文"><img src="../flags/zh.svg" width="24" alt="简体中文"></a></p>
<p align="center">
  <a href="https://github.com/Yasaiek/Loorek-Twitch-Kick-Points-Drops/releases/latest/download/Loorek.exe"><b>⬇ Скачать Loorek.exe</b></a> &nbsp;·&nbsp;
  <a href="https://yasaiek.github.io/Loorek-Twitch-Kick-Points-Drops/"><b>🌐 Сайт</b></a> &nbsp;·&nbsp;
  <a href="../../CHANGELOG.md">Список изменений</a> &nbsp;·&nbsp;
  <a href="https://github.com/Yasaiek/Loorek-Twitch-Kick-Points-Drops/issues/new">Сообщить об ошибке</a>
</p>
<p align="center"><img src="../img/demo.gif" alt="Loorek" width="860"></p>

> **Beta.** Loorek ещё молод и проверен на небольшом числе компьютеров. Возможны шероховатости – если что-то не работает, создайте issue.

## Что он делает

- **Баллы канала Twitch** – смотрит до двух каналов одновременно (больше Twitch не засчитывает) и сам забирает каждый бонус.
- **Присутствие в чате** – остаётся в чатах всех каналов из вашего списка.
- **Twitch Drops** – поставьте игры в очередь, смотрите каждую кампанию и награду; Loorek сам выбирает стрим, меняет его, когда тот закончился или завис, и забирает дроп.
- **Отслеживаемые игры** – найдите любую игру и отслеживайте её; получение начнётся само, когда появится кампания.
- **Баллы канала Kick** – собирает баллы на выбранных каналах Kick.
- **Простая настройка** – подсказки каналов с аватарами прямо при вводе.
- **Работает в области уведомлений** – закройте панель, всё продолжит работать. Не декодирует видео и не трогает другие процессы.
- **Обновляется сам** из релизов этого репозитория (с проверкой SHA-256).
- **12 языков**, переключаются в любой момент.

| Кампании дропов | Очередь |
| --- | --- |
| ![Кампании дропов](../img/camps.png) | ![Очередь](../img/queue.png) |
| **Баллы Twitch** | **Баллы Kick** |
| ![Баллы Twitch](../img/points.png) | ![Баллы Kick](../img/kick.png) |

## Начало работы

1. Скачайте **Loorek.exe** и запустите. Установщика нет; значок появится рядом с часами, и откроется панель.
2. Нажмите **Войти**. Откроется отдельное окно входа Twitch – войдите как обычно, окно закроется само. С Kick всё так же.
3. Добавьте каналы в разделе **Баллы** и выберите игры в разделе **Дропы**. Порядок – это приоритет.

Windows может предупредить о неизвестном издателе, потому что файл не подписан. Каждый релиз собирается из этого кода в GitHub Actions.

## Где хранятся ваши данные

Всё, что сохраняет Loorek, лежит в папке `data` рядом с `Loorek.exe`:

| Файл | Что это |
| --- | --- |
| `settings.json` | настройки, списки каналов, очередь |
| `auth.json` | ваши сессии Twitch и Kick – никому не отправляйте этот файл |
| `login-profile/` | профиль окна входа – его тоже не отправляйте |
| `log.txt` | журнал |

Сессии отправляются только самим Twitch и Kick. Окно входа – это уже установленный Edge/Chrome с отдельным профилем; Loorek не читает ваш обычный профиль браузера.

Loorek обращается к: Twitch, Kick и GitHub (проверка обновлений). Больше ни к чему.

## Сборка из исходников

Go 1.24 или новее:

```
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-H=windowsgui -s -w" -o Loorek.exe .
```

Панель – обычный HTML в `ui/`, переводы – файлы JSON в `ui/lang/`.

## Благодарности

Знания о протоколах взяты из этих проектов – спасибо:

- [TwitchDropsMiner](https://github.com/DevilXD/TwitchDropsMiner) – DevilXD (MIT)
- [Kick_Channel_Points_Miner](https://github.com/Baillora/Kick_Channel_Points_Miner) – Baillora (MIT)
- [fyne.io/systray](https://github.com/fyne-io/systray), [bogdanfinn/tls-client](https://github.com/bogdanfinn/tls-client)

Нужно расширение для браузера? Смотрите [Dropomat](https://github.com/Yasaiek/Dropomat-Auto-Twitch-Drops).

## Отказ от ответственности

Автоматический просмотр нарушает условия использования Twitch и Kick. Loorek не связан ни с одним из них. Вы используете его на свой страх и риск.

<sub>MIT</sub>
