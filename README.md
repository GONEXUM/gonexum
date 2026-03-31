# GONEXUM

Application desktop pour créer et uploader des torrents sur le tracker Nexum.

---

## Fonctionnalités

- **Création de torrent** — génère un `.torrent` privé à partir d'un fichier ou d'un dossier, avec barre de progression en temps réel
- **Analyse média** — extraction automatique des informations techniques (résolution, codec vidéo/audio, HDR, langues) sans dépendance externe via un parser Go natif (MKV, MP4)
- **Recherche TMDB** — recherche automatique des métadonnées au lancement via un proxy interne (aucune clé API requise)
- **Génération NFO** — création d'un fichier NFO formaté, ou import d'un fichier existant
- **Upload** — envoi du torrent et du NFO directement vers l'API du tracker

---

## Téléchargement

Les binaires compilés sont disponibles dans les [Releases GitHub](https://github.com/diabolino/gonexum/releases).

| Plateforme | Fichier |
|---|---|
| macOS Apple Silicon (M1/M2/M3) | `GONEXUM-macos-arm64.zip` |
| macOS Intel | `GONEXUM-macos-amd64.zip` |
| Windows 64-bit | `GONEXUM.exe` |

---

## Utilisation

### 1. Configuration

Au premier lancement, rendez-vous dans **Paramètres** et renseignez :

- **URL du tracker** — l'URL de base de nexum-core.com
- **Clé API** — votre clé API personnelle
- **Passkey** — votre passkey pour l'annonce du tracker
- **Dossier de sortie** — répertoire où seront enregistrés les fichiers `.torrent` générés

### 2. Workflow d'upload

L'application guide l'upload en 4 étapes :

**Étape 1 — Source**
Sélectionnez un fichier vidéo ou un dossier. Le torrent est créé automatiquement. Pour les dossiers, le fichier vidéo le plus volumineux est analysé.

**Étape 2 — Média**
Les informations techniques sont extraites automatiquement. Elles peuvent être corrigées manuellement si nécessaire.

**Étape 3 — Métadonnées**
La recherche TMDB se lance automatiquement. Si le premier résultat ne correspond pas, vous pouvez :
- Rechercher manuellement par nom
- Coller directement un lien `themoviedb.org/movie/...` ou `themoviedb.org/tv/...`

Pour le NFO, deux options : génération automatique à partir des données TMDB, ou import d'un fichier `.nfo` existant.

**Étape 4 — Upload**
Vérifiez le récapitulatif, ajoutez une description optionnelle, puis uploadez.

---

## Compilation depuis les sources

### Prérequis

- [Go 1.24+](https://go.dev/dl/)
- [Node.js 20+](https://nodejs.org/)
- [Wails v2](https://wails.io/docs/gettingstarted/installation)

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

### Lancer en mode développement

```bash
wails dev
```

### Compiler

```bash
# macOS
wails build -platform darwin/arm64

# Windows
wails build -platform windows/amd64
```

---

## CI/CD

Chaque push sur le dépôt déclenche automatiquement la compilation pour macOS (arm64 + amd64) et Windows via GitHub Actions.

Pour publier une nouvelle release avec les binaires en pièces jointes :

```bash
git tag v1.x.x
git push origin v1.x.x
```

---

## Licence

Projet privé — usage réservé aux membres du tracker.
