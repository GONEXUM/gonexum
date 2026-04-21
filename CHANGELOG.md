# Changelog

Toutes les modifications notables de GONEXUM sont documentées dans ce fichier.

Le format est basé sur [Keep a Changelog](https://keepachangelog.com/fr/1.1.0/)
et le projet suit [Semantic Versioning](https://semver.org/lang/fr/).

## [3.0.0] - 2026-04-20

### Ajouté
- **Historique SQLite** : chaque upload (succès ou échec) est enregistré localement dans `history.db` (à côté de `settings.json`). Page dédiée dans le desktop et le web, avec recherche par nom de release ou titre TMDB, lien vers nexum, suppression unitaire ou globale.
- **Éditeur d'item dans la queue** : bouton ✎ sur chaque item en attente pour personnaliser nom, catégorie, match TMDB (recherche avec posters), et description BBCode avant traitement.
- **Blocage en cas de version obsolète** : l'application refuse de fonctionner si une nouvelle version est publiée sur GitHub. Desktop : écran plein-écran + fermeture automatique au clic sur "Télécharger". CLI : exit(1). Web : overlay bloquant.
- **Fichier CHANGELOG.md** (ce fichier) et publication automatique des notes de version sur GitHub Releases.

### Modifié
- **Refonte de la page principale desktop** : plus de toggle "Unitaire / File d'attente". Une seule interface unifiée basée sur la queue, avec drag & drop multi-fichiers et édition optionnelle par item. (~1270 lignes → ~315)

## [2.6.x] - 2026-04-19/20

### Ajouté
- **Détection de doublons** via `GET /api/v1/torrents?q=<name>` avant chaque upload. Warning dès la sélection du fichier (web + desktop) et blocage en queue.
- **Fallback API TMDB officielle** si le proxy nexum ne renvoie pas de résultats. Parseur Go des noms de release (titre + année). Clé injectée via secret `TMDB_API_KEY` au build.
- **URL du proxy TMDB en secret** (`TMDB_PROXY_URL`) pour ne plus l'exposer en dur.

### Modifié
- **Migration vers l'organisation `GONEXUM`** : repo à `github.com/GONEXUM/gonexum`, releases publiées sur le même repo, image Docker `ghcr.io/gonexum/gonexum`.

## [2.5.x] - 2026-04-19

### Ajouté
- **Image Docker multi-arch** (`linux/amd64` + `linux/arm64`) publiée sur GHCR avec `mediainfo` et `ffprobe` pré-installés.
- **Flag `--browse-root`** pour définir la racine du navigateur de fichiers web (utile en Docker pour permettre `/series`, `/films`, etc.).
- **Tolérance de détection résolution** : ±50px largeur, ±200px hauteur pour gérer les rips BluRay croppés (ex: 1920×800 → 1080p).

## [2.4.x] - 2026-04-19

### Ajouté
- **Description d'upload en BBCode** : génération automatique depuis la sortie `mediainfo`, avec bannières nexum-core.com et détails techniques (codec, HDR, résolution, pistes audio/sous-titres).
- **Champ description éditable** pré-rempli automatiquement à l'étape Options (web) et Upload (desktop).
- **Affichage détaillé des erreurs 422** renvoyées par l'API nexum (champ `errors`, `message`, `warnings`, `name`).

### Modifié
- **Champs requis par l'API nexum** : `description`, `tmdb_id` et `tmdb_type` toujours envoyés. Fallback en cascade : description saisie → BBCode mediainfo → TMDB overview → NFO brut.

## [2.3.x] - 2026-04-05/19

### Ajouté
- **Système de queue** : mode séparé sur le web avec SSE pour le suivi en temps réel. CLI : plusieurs chemins en arguments → traitement séquentiel automatique.
- **Vérification de mise à jour** au démarrage (CLI + Web), non bloquante.

## [2.2.x] - 2026-04-05

### Ajouté
- **Templates NFO personnalisables** (CLI + Web) avec Go template, fonctions `padRight`/`center`/`truncate`/`join`/`printf`, mode toggle NFO/MediaInfo brut, aperçu live.
- **Catégories dynamiques** depuis `GET /api/v1/categories`, avec fallback hardcodé.
- **Auto-sélection du premier résultat TMDB** après recherche.

### Modifié
- **Normalisation du nom du torrent** avant upload : les espaces et parenthèses sont remplacés par des points (format scene), les doubles points collapse.

## [2.1.x] et antérieur

- **Version web** avec interface navigateur et file browser (clampé au home user).
- **Version CLI** pour seedbox/serveur.
- **Version desktop** (Wails) pour macOS, Windows, Linux avec wizard multi-étapes.
