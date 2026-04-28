# Changelog

Toutes les modifications notables de GONEXUM sont documentées dans ce fichier.

Le format est basé sur [Keep a Changelog](https://keepachangelog.com/fr/1.1.0/)
et le projet suit [Semantic Versioning](https://semver.org/lang/fr/).

## [3.1.12] - 2026-04-27

### Ajouté
- **Badge source TMDB** sur la card item desktop : indique si le match vient du proxy nexum (badge vert "proxy") ou de l'API TMDB officielle en fallback (badge jaune "direct"). Utile pour diagnostiquer les cas où le proxy échoue silencieusement et qu'on tombe sur un mauvais résultat via TMDB direct.
- `GetLastTMDBSource()` exposé via Wails pour les besoins de debug.
- `Debug.OpenInspectorOnStartup` ajouté dans `main.go` (devtools restent strippés en build production, utiliser `wails dev` ou `wails build -debug`).

## [3.1.11] - 2026-04-27

### Corrigé
- **Poster manquant après sélection manuelle TMDB** : quand on changeait le match TMDB via le modal d'édition, seuls le titre et l'ID étaient stockés en override — le poster restait celui de l'analyse initiale (ou vide si l'analyse n'avait rien trouvé). Désormais le `posterPath` est aussi capturé lors du `pickTMDB` et la card priorise l'override sur l'analyse initiale.

## [3.1.10] - 2026-04-27

### Corrigé
- **Désérialisation TMDB plus tolérante** : `TMDBResult` et `TMDBDetails` acceptent désormais aussi le format brut TMDB (`poster_path`, `media_type`, `name`, `release_date`, `first_air_date`, `vote_average`) en plus de la convention camelCase utilisée par Wails. Custom `UnmarshalJSON` ajouté pour mapper les deux conventions au même champ Go. Évite les `posterPath` vides si une struct TMDBDetails est désérialisée depuis un JSON snake_case.

## [3.1.9] - 2026-04-27

### Corrigé
- **Sortie mediainfo desktop non conforme au format CLI** : la fonction `getMediaInfoCLIText` reconstruisait le format texte à partir de l'objet JSON (sections, padding, unités), ce qui produisait un résultat différent du binaire `mediainfo`. La sortie est maintenant produite directement par `mediainfo.js` en mode `text` — strictement identique au CLI standard. ~150 lignes de formatage manuel supprimées.

## [3.1.8] - 2026-04-22

### Sécurité
- **URL du proxy TMDB retirée du code source** : la valeur par défaut hardcodée dans `tmdb.go` (3 occurrences) annulait l'effet du secret GitHub `TMDB_PROXY_URL`. Désormais la valeur par défaut est `""` ; le proxy n'est utilisé que si la variable est injectée au build via ldflags. Sans secret, le code passe directement à l'API TMDB officielle (avec `TMDB_API_KEY`).

## [3.1.7] - 2026-04-22

### Corrigé
- **Proxy TMDB nexum cassé silencieusement** : le proxy renvoyait `years` et `tmdb_id` sous forme de **nombre** JSON (`2024`, `82452`) alors que les structs Go attendaient des `string`. Le `json.Unmarshal` échouait pour chaque résultat → "Aucun match" affiché et fallback API officielle déclenché inutilement. Les deux champs sont désormais en `json.RawMessage` avec helpers `parseYears` / `parseTmdbID` tolérants au type.

## [3.1.6] - 2026-04-22

### Corrigé
- **Détection movie/tv en fallback** : quand le proxy nexum ne renvoyait pas de résultat (ex: `Avatar.The.Last.Airbender.2024.S01.MULTI.1080p.WEB`), le fallback API officielle TMDB tapait par défaut sur `/search/movie` et ratait les séries. Une heuristique `detectMediaType` regarde maintenant les marqueurs `S01` / `S01E02` dans le nom pour basculer sur `/search/tv`. En dernier recours, le type opposé est aussi essayé.

## [3.1.5] - 2026-04-22

### Ajouté
- **Barre de progression** lors de la création du fichier .torrent (hashing) sur l'item en cours dans la queue desktop. Le step affiche le pourcentage en temps réel.

### Modifié
- **Performance hashing** : buffer de lecture passé de 256 KB à 4 MB (CLI, Web, Desktop). Réduit drastiquement le nombre de syscalls et accélère le hashing des gros fichiers (films 4K, séries).

## [3.1.4] - 2026-04-22

### Corrigé
- **Faux positifs de détection de doublons** : l'API nexum `/api/v1/torrents?q=...` fait un fuzzy match, ce qui renvoyait par exemple `WWE.RAW.2026.04.20...` comme doublon de `WWE.RAW.2026.04.13...`. `checkDuplicate` compare désormais le nom normalisé exact avant de marquer un doublon.

## [3.1.3] - 2026-04-21

### Corrigé
- **BBCode ne listait qu'une seule piste audio/sous-titre** : le parseur mediainfo ne matchait que les sections nommées exactement `Audio` ou `Text`, et ratait les pistes supplémentaires (`Audio #1`, `Audio #2`, `Text #1`…). Toutes les pistes sont désormais incluses dans la description.

## [3.1.2] - 2026-04-21

### Corrigé
- **Historique vide** : les entrées n'étaient pas sauvegardées car le champ `createdAt` envoyé sous forme de chaîne vide faisait échouer le `json.Unmarshal` côté Go (`time.Time` ne peut pas parser `""`). Désormais envoyé en ISO 8601. Les erreurs de save sont loggées dans la console au lieu d'être silencieusement ignorées.

## [3.1.1] - 2026-04-21

### Ajouté
- **Poster TMDB** affiché à droite de chaque item de la queue une fois l'analyse terminée (taille responsive via `clamp()` + `aspect-ratio: 2/3`). Placeholder 🎬 si aucun match TMDB.

## [3.1.0] - 2026-04-21

### Ajouté
- **Analyse automatique à l'ajout** : chaque fichier déposé déclenche immédiatement l'analyse (media info + recherche TMDB + pré-génération BBCode). Plus besoin d'ouvrir le modal d'édition pour voir ce qui a été détecté.
- **Affichage inline des infos détectées** : chaque item de la queue affiche sous son nom le match TMDB (avec année et type), la catégorie, la résolution / codec / source / HDR, et les langues audio. Vue d'ensemble immédiate.
- **Workflow de validation par item** : après analyse, les items passent en état "à valider" avec un bouton ✓ OK. Ils ne démarrent le traitement que lorsque l'utilisateur valide. Bouton "Tout valider" pour un batch rapide.
- **Bouton ↻ ré-analyser** sur les items en erreur.

### Modifié
- Le modal d'édition reste accessible via ✎ pour corriger les détections, mais n'est plus nécessaire dans la majorité des cas.
- Le compteur du bouton "▶ Lancer" reflète les items validés (prêts), plus les items en attente d'analyse.

## [3.0.1] - 2026-04-21

### Corrigé
- **Perte du state de la queue lors de la navigation** : le contenu de la queue (items en attente, en cours, terminés) est désormais préservé quand on bascule entre Uploader / Historique / Paramètres. Le traitement continue en arrière-plan même sur une autre page.
- Le drop de fichiers fonctionne maintenant depuis n'importe quelle page (les fichiers droppés s'ajoutent à la queue même si l'utilisateur est sur Historique).

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
