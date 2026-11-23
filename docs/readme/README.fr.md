<p align="center"><img src="../img/icon.png" width="96" alt=""></p>
<h1 align="center">Loorek – Twitch &amp; Kick Points and Drops Collector</h1>
<p align="center">Une petite application Windows dans la zone de notification qui collecte les <b>points de chaîne Twitch et Kick</b> et récupère les <b>Twitch Drops</b> en arrière-plan.<br>Sans onglet de navigateur, sans lecture vidéo.</p>
<p align="center"><a href="../../README.md" title="English"><img src="../flags/en.svg" width="24" alt="English"></a> <a href="README.pl.md" title="Polski"><img src="../flags/pl.svg" width="24" alt="Polski"></a> <a href="README.de.md" title="Deutsch"><img src="../flags/de.svg" width="24" alt="Deutsch"></a> <a href="README.fr.md" title="Français"><img src="../flags/fr.svg" width="24" alt="Français"></a> <a href="README.es.md" title="Español"><img src="../flags/es.svg" width="24" alt="Español"></a> <a href="README.pt.md" title="Português (BR)"><img src="../flags/pt.svg" width="24" alt="Português (BR)"></a> <a href="README.it.md" title="Italiano"><img src="../flags/it.svg" width="24" alt="Italiano"></a> <a href="README.ru.md" title="Русский"><img src="../flags/ru.svg" width="24" alt="Русский"></a> <a href="README.uk.md" title="Українська"><img src="../flags/uk.svg" width="24" alt="Українська"></a> <a href="README.tr.md" title="Türkçe"><img src="../flags/tr.svg" width="24" alt="Türkçe"></a> <a href="README.ja.md" title="日本語"><img src="../flags/ja.svg" width="24" alt="日本語"></a> <a href="README.zh.md" title="简体中文"><img src="../flags/zh.svg" width="24" alt="简体中文"></a></p>
<p align="center">
  <a href="https://github.com/Yasaiek/Loorek-Twitch-Kick-Points-Drops/releases/latest/download/Loorek.exe"><b>⬇ Télécharger Loorek.exe</b></a> &nbsp;·&nbsp;
  <a href="https://yasaiek.github.io/Loorek-Twitch-Kick-Points-Drops/"><b>🌐 Site web</b></a> &nbsp;·&nbsp;
  <a href="../../CHANGELOG.md">Journal des modifications</a> &nbsp;·&nbsp;
  <a href="https://github.com/Yasaiek/Loorek-Twitch-Kick-Points-Drops/issues/new">Signaler un bug</a>
</p>
<p align="center"><img src="../img/demo.gif" alt="Loorek" width="860"></p>

> **Beta.** Loorek est jeune et n'a été testé que sur peu de machines. Attendez-vous à quelques défauts et ouvrez une issue si quelque chose ne fonctionne pas.

## Ce qu'il fait

- **Points de chaîne Twitch** – regarde jusqu'à deux chaînes à la fois (Twitch n'en compte pas plus) et récupère chaque bonus tout seul.
- **Présence dans le chat** – reste dans les chats de toutes les chaînes de votre liste.
- **Twitch Drops** – mettez des jeux en file, voyez chaque campagne et récompense ; Loorek choisit un stream, en change s'il se termine ou stagne, et récupère le drop.
- **Jeux suivis** – cherchez n'importe quel jeu et suivez-le ; la collecte démarre toute seule dès qu'une campagne apparaît.
- **Points de chaîne Kick** – collecte les points sur les chaînes Kick de votre choix.
- **Facile à configurer** – suggestions de chaînes avec avatars pendant la saisie.
- **Tourne dans la zone de notification** – fermez le panneau, tout continue. Ne décode pas la vidéo et ne touche à aucun autre processus.
- **Se met à jour tout seul** depuis les releases de ce dépôt (SHA-256 vérifié).
- **12 langues**, modifiables à tout moment.

| Campagnes de drops | File d'attente |
| --- | --- |
| ![Campagnes de drops](../img/camps.png) | ![File d'attente](../img/queue.png) |
| **Points Twitch** | **Points Kick** |
| ![Points Twitch](../img/points.png) | ![Points Kick](../img/kick.png) |

## Pour commencer

1. Téléchargez **Loorek.exe** et lancez-le. Pas d'installateur ; l'icône apparaît près de l'horloge et le panneau s'ouvre.
2. Cliquez sur **Se connecter**. Une fenêtre de connexion Twitch séparée s'ouvre – connectez-vous comme d'habitude, elle se ferme toute seule. Kick fonctionne de la même façon.
3. Ajoutez des chaînes dans **Points** et choisissez des jeux dans **Drops**. L'ordre fait la priorité.

Windows peut signaler un éditeur inconnu, car le fichier n'est pas signé. Chaque release est compilée à partir de ce code par GitHub Actions.

## Où se trouvent vos données

Tout ce que Loorek enregistre se trouve dans `%LOCALAPPDATA%\Yasaiek\Loorek` (un autre emplacement se choisit dans Réglages → Dossier de données) :

| Fichier | Ce que c'est |
| --- | --- |
| `loorek-settings.json` | réglages, listes de chaînes, file d'attente |
| `auth.json` | vos sessions Twitch et Kick – ne partagez jamais ce fichier |
| `login-profile/` | le profil de la fenêtre de connexion – ne le partagez pas non plus |
| `log.txt` | le journal |

Les sessions ne sont envoyées qu'à Twitch et Kick. La fenêtre de connexion est le Edge/Chrome déjà installé, lancé avec un profil séparé ; Loorek ne lit pas votre profil de navigateur habituel.

Loorek communique avec : Twitch, Kick et GitHub (vérification des mises à jour). Rien d'autre.

## Compiler depuis les sources

Go 1.24 ou plus récent :

```
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-H=windowsgui -s -w" -o Loorek.exe .
```

Le panneau est du HTML simple dans `ui/`, les traductions sont des fichiers JSON dans `ui/lang/`.

## Remerciements

La connaissance des protocoles vient de ces projets – merci :

- [TwitchDropsMiner](https://github.com/DevilXD/TwitchDropsMiner) – DevilXD (MIT)
- [Kick_Channel_Points_Miner](https://github.com/Baillora/Kick_Channel_Points_Miner) – Baillora (MIT)
- [fyne.io/systray](https://github.com/fyne-io/systray), [bogdanfinn/tls-client](https://github.com/bogdanfinn/tls-client)

Vous préférez une extension de navigateur ? Voir [Dropomat](https://github.com/Yasaiek/Dropomat-Auto-Twitch-Drops).

## Avertissement

Le visionnage automatisé est contraire aux conditions d'utilisation de Twitch et de Kick. Loorek n'est affilié à aucun des deux. Vous l'utilisez à vos risques et périls.

<sub>MIT</sub>
