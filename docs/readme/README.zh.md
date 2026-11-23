<p align="center"><img src="../img/icon.png" width="96" alt=""></p>
<h1 align="center">Loorek – Twitch &amp; Kick Points and Drops Collector</h1>
<p align="center">一款驻留在系统托盘的小型 Windows 应用，在后台收集 <b>Twitch 和 Kick 频道积分</b>并获取 <b>Twitch 掉宝</b>。<br>无需浏览器标签页，也不播放视频。</p>
<p align="center"><a href="../../README.md" title="English"><img src="../flags/en.svg" width="24" alt="English"></a> <a href="README.pl.md" title="Polski"><img src="../flags/pl.svg" width="24" alt="Polski"></a> <a href="README.de.md" title="Deutsch"><img src="../flags/de.svg" width="24" alt="Deutsch"></a> <a href="README.fr.md" title="Français"><img src="../flags/fr.svg" width="24" alt="Français"></a> <a href="README.es.md" title="Español"><img src="../flags/es.svg" width="24" alt="Español"></a> <a href="README.pt.md" title="Português (BR)"><img src="../flags/pt.svg" width="24" alt="Português (BR)"></a> <a href="README.it.md" title="Italiano"><img src="../flags/it.svg" width="24" alt="Italiano"></a> <a href="README.ru.md" title="Русский"><img src="../flags/ru.svg" width="24" alt="Русский"></a> <a href="README.uk.md" title="Українська"><img src="../flags/uk.svg" width="24" alt="Українська"></a> <a href="README.tr.md" title="Türkçe"><img src="../flags/tr.svg" width="24" alt="Türkçe"></a> <a href="README.ja.md" title="日本語"><img src="../flags/ja.svg" width="24" alt="日本語"></a> <a href="README.zh.md" title="简体中文"><img src="../flags/zh.svg" width="24" alt="简体中文"></a></p>
<p align="center">
  <a href="https://github.com/Yasaiek/Loorek-Twitch-Kick-Points-Drops/releases/latest/download/Loorek.exe"><b>⬇ 下载 Loorek.exe</b></a> &nbsp;·&nbsp;
  <a href="https://yasaiek.github.io/Loorek-Twitch-Kick-Points-Drops/"><b>🌐 网站</b></a> &nbsp;·&nbsp;
  <a href="../../CHANGELOG.md">更新日志</a> &nbsp;·&nbsp;
  <a href="https://github.com/Yasaiek/Loorek-Twitch-Kick-Points-Drops/issues/new">报告问题</a>
</p>
<p align="center"><img src="../img/demo.gif" alt="Loorek" width="860"></p>

> **Beta.** Loorek 还很新，只在少量电脑上测试过。可能存在问题，遇到无法使用的情况请提交 issue。

## 功能

- **Twitch 频道积分** – 同时最多观看两个频道（Twitch 只计算这么多），并自动领取每个奖励。
- **聊天室在线** – 停留在列表中所有频道的聊天室。
- **Twitch 掉宝** – 将游戏加入队列，查看每个活动和奖励；Loorek 自动选择直播，在直播结束或卡住时切换，并领取掉宝。
- **关注的游戏** – 搜索任意游戏并关注；出现活动后自动开始。
- **Kick 频道积分** – 在你选择的 Kick 频道收集积分。
- **设置简单** – 输入时显示带头像的频道建议。
- **在托盘运行** – 关闭面板后一切照常运行。不解码视频，也不触碰其他进程。
- **自动更新** – 来自本仓库的发布版本（校验 SHA-256）。
- **12 种语言**，随时切换。

| 掉宝活动 | 队列 |
| --- | --- |
| ![掉宝活动](../img/camps.png) | ![队列](../img/queue.png) |
| **Twitch 积分** | **Kick 积分** |
| ![Twitch 积分](../img/points.png) | ![Kick 积分](../img/kick.png) |

## 开始使用

1. 下载并运行 **Loorek.exe**。无需安装；图标会出现在时钟旁，面板随即打开。
2. 点击**登录**。会打开一个单独的 Twitch 登录窗口，照常登录后窗口会自动关闭。Kick 也是一样。
3. 在**积分**中添加频道，在**掉宝**中选择游戏。顺序即优先级。

由于文件未签名，Windows 可能会提示未知发布者。每个版本均由 GitHub Actions 从此源代码构建。

## 数据存放位置

Loorek 保存的所有内容都在 `%LOCALAPPDATA%\Yasaiek\Loorek` 中（可在 设置 → 数据文件夹 中选择其他位置）：

| 文件 | 内容 |
| --- | --- |
| `loorek-settings.json` | 设置、频道列表、队列 |
| `auth.json` | 你的 Twitch 和 Kick 会话 – 切勿分享此文件 |
| `login-profile/` | 登录窗口的配置文件 – 同样不要分享 |
| `log.txt` | 日志 |

会话只发送给 Twitch 和 Kick 本身。登录窗口是电脑上已安装的 Edge/Chrome，以独立的配置文件启动；Loorek 不会读取你日常使用的浏览器配置文件。

Loorek 只与 Twitch、Kick 和 GitHub（检查更新）通信，别无其他。

## 从源代码构建

Go 1.24 或更高版本：

```
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-H=windowsgui -s -w" -o Loorek.exe .
```

面板是 `ui/` 中的普通 HTML，翻译是 `ui/lang/` 中的 JSON 文件。

## 致谢

协议相关知识来自以下项目，感谢：

- [TwitchDropsMiner](https://github.com/DevilXD/TwitchDropsMiner) – DevilXD (MIT)
- [Kick_Channel_Points_Miner](https://github.com/Baillora/Kick_Channel_Points_Miner) – Baillora (MIT)
- [fyne.io/systray](https://github.com/fyne-io/systray), [bogdanfinn/tls-client](https://github.com/bogdanfinn/tls-client)

想要浏览器扩展？请看 [Dropomat](https://github.com/Yasaiek/Dropomat-Auto-Twitch-Drops).

## 免责声明

自动观看违反 Twitch 和 Kick 的服务条款。Loorek 与两者均无关联。使用风险自负。

<sub>MIT</sub>
