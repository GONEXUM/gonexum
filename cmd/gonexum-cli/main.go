package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// AppVersion est injecté au build via -ldflags "-X main.AppVersion=x.x.x"
var AppVersion = "dev"

var defaultCategoriesMap = map[int]string{
	1: "Films",
	2: "Séries",
	3: "Documentaires",
	4: "Animés",
}

func fetchCategoriesMap(settings Settings) map[int]string {
	if settings.TrackerURL == "" {
		return defaultCategoriesMap
	}
	catURL := settings.TrackerURL + "/api/v1/categories"
	if settings.APIKey != "" {
		catURL += "?apikey=" + settings.APIKey
	}
	req, err := http.NewRequest("GET", catURL, nil)
	if err != nil {
		return defaultCategoriesMap
	}
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != 200 {
		return defaultCategoriesMap
	}
	defer resp.Body.Close()
	var result struct {
		Categories []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"categories"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil || len(result.Categories) == 0 {
		return defaultCategoriesMap
	}
	m := make(map[int]string, len(result.Categories))
	for _, c := range result.Categories {
		m[c.ID] = c.Name
	}
	return m
}

var sources = []string{"BluRay", "WEB-DL", "WEBRip", "HDTV", "DVDRip", "DCP"}

func main() {
	var (
		flagName       = flag.String("name", "", "Nom du release (défaut: nom du fichier/dossier)")
		flagTMDBID     = flag.Int("tmdb-id", 0, "ID TMDB (0 = recherche interactive)")
		flagTMDBType   = flag.String("tmdb-type", "movie", "Type TMDB: movie ou tv")
		flagCategory   = flag.Int("category", 1, "Catégorie: 1=Films 2=Séries 3=Docs 4=Animés")
		flagSource     = flag.String("source", "", "Source: BluRay WEB-DL WEBRip HDTV DVDRip DCP")
		flagNoUpload    = flag.Bool("no-upload", false, "Créer torrent+NFO sans uploader")
		flagOutput      = flag.String("output", "", "Dossier de sortie (défaut: config)")
		flagConfig      = flag.String("config", "", "Chemin vers settings.json")
		flagYes         = flag.Bool("yes", false, "Non-interactif: prend le premier résultat TMDB")
		flagNoMediaInfo = flag.Bool("no-mediainfo", false, "Ignorer l'extraction automatique des infos média")
		flagNFOTemplate = flag.String("nfo-template", "", "Chemin vers un fichier template NFO (Go template)")
		flagNFOMode     = flag.String("nfo-mode", "", "Mode NFO: nfo (défaut) ou mediainfo")
	)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `GONEXUM CLI — Création et upload de torrents vers nexum-core.com

Usage:
  gonexum-cli [options] <source>

Arguments:
  source          Fichier vidéo ou dossier à torrentifier

Options:
`)
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Exemples:
  gonexum-cli /data/Fight.Club.1999.1080p.BluRay.x264-GROUP
  gonexum-cli -tmdb-id 550 -category 1 -source BluRay /data/Fight.Club.1999.1080p.BluRay.x264-GROUP
  gonexum-cli -no-upload /data/Ma.Serie.S01E01.mkv
  gonexum-cli -yes -tmdb-id 1396 -tmdb-type tv -category 2 /data/Breaking.Bad.S01

  # Queue : plusieurs sources → traitement séquentiel automatique (-yes implicite)
  gonexum-cli /data/Film1 /data/Film2 /data/Film3

Config:
  Les paramètres (API key, passkey, etc.) sont lus depuis:
    Linux/seedbox : ~/.config/GONEXUM/settings.json
    macOS         : ~/Library/Application Support/GONEXUM/settings.json
`)
	}

	flag.Parse()

	if *flagConfig != "" {
		configFilePath = *flagConfig
	}

	// Source paths: un ou plusieurs arguments positionnels
	sources := flag.Args()
	if len(sources) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	// Mode queue : plusieurs sources → traitement séquentiel, --yes implicite
	if len(sources) > 1 {
		runQueue(sources, *flagCategory, *flagTMDBType, *flagSource, *flagNoUpload, *flagOutput, *flagConfig,
			*flagNFOMode, *flagNFOTemplate)
		return
	}

	sourcePath := sources[0]

	// Vérification source
	if _, err := os.Stat(sourcePath); err != nil {
		fatalf("Source invalide: %v\n", err)
	}

	// Chargement + vérification settings
	settings, err := loadSettings()
	if err != nil {
		warnf("Impossible de charger les settings: %v\n", err)
		settings = Settings{TrackerURL: "https://nexum-core.com"}
	}
	if err := ensureSettings(&settings, *flagNoUpload); err != nil {
		fatalf("%v\n", err)
	}
	if *flagNFOMode != "" {
		settings.NFOMode = *flagNFOMode
	}
	if *flagNFOTemplate != "" {
		tmplData, err := os.ReadFile(*flagNFOTemplate)
		if err != nil {
			fatalf("Impossible de lire le template NFO: %v\n", err)
		}
		settings.NFOTemplate = string(tmplData)
	}
	if *flagOutput != "" {
		settings.OutputDir = *flagOutput
	}
	if settings.OutputDir == "" {
		settings.OutputDir = filepath.Dir(sourcePath)
	}

	// Nom du release
	releaseName := *flagName
	if releaseName == "" {
		releaseName = filepath.Base(sourcePath)
		fi, _ := os.Stat(sourcePath)
		if fi != nil && !fi.IsDir() {
			releaseName = strings.TrimSuffix(releaseName, filepath.Ext(releaseName))
		}
	}

	printHeader()
	fmt.Printf("  Source : %s\n", sourcePath)
	fmt.Printf("  Nom    : %s\n", releaseName)
	fmt.Println()

	// ── Étape 1 : Media info ────────────────────────────────────────────────
	step(1, 4, "Extraction des informations média")
	var mediaInfo MediaInfo
	var mediaInfoCLI string
	var mediaDetected bool

	if !*flagNoMediaInfo {
		videoPath, vidErr := largestVideoFile(sourcePath)
		if vidErr != nil {
			warnf("  %v\n", vidErr)
		} else {
			mi, cliText, detected, miErr := getMediaInfo(videoPath)
			if miErr != nil {
				warnf("  mediainfo/ffprobe indisponibles: %v\n", miErr)
				warnf("  Les infos média devront être saisies manuellement.\n")
			} else {
				mediaInfo = mi
				mediaInfoCLI = cliText
				mediaDetected = detected
				ok("  Résolution : %s | Vidéo : %s | Audio : %s",
					mediaInfo.Resolution, mediaInfo.VideoCodec, mediaInfo.AudioCodec)
				if mediaInfo.Duration != "" {
					ok("  Durée : %s | Taille : %s", mediaInfo.Duration, formatBytes(mediaInfo.FileSize))
				}
				if mediaInfoCLI != "" {
					ok("  mediainfo CLI : OK")
				}
				if mediaInfo.HDRFormat != "" {
					ok("  HDR          : %s", mediaInfo.HDRFormat)
				}
			}
		}
	} else {
		warnf("  Extraction média désactivée (--no-mediainfo)\n")
	}

	// Auto-détection de la source depuis le nom du release
	if *flagSource != "" {
		mediaInfo.Source = *flagSource
	} else if mediaInfo.Source == "" {
		if detected := detectSourceFromName(releaseName); detected != "" {
			mediaInfo.Source = detected
			ok("  Source       : %s (détecté)", mediaInfo.Source)
		}
	}

	// Saisie manuelle si infos manquantes
	if !*flagYes {
		mediaInfo = promptMediaOverrides(mediaInfo, mediaDetected)
	}

	// Demander la source seulement si toujours inconnue
	if mediaInfo.Source == "" && !*flagYes {
		mediaInfo.Source = promptChoice("  Source", sources, "")
	}

	// ── Étape 2 : TMDB ─────────────────────────────────────────────────────
	step(2, 4, "Recherche TMDB")
	var tmdbDetails TMDBDetails
	tmdbType := *flagTMDBType

	if *flagTMDBID > 0 {
		tmdbDetails, err = getTMDBDetails(*flagTMDBID, tmdbType)
		if err != nil {
			warnf("  Erreur TMDB: %v\n", err)
		} else {
			ok("  %s (%s) — %s", tmdbDetails.Title, tmdbDetails.Year, tmdbDetails.MediaType)
		}
	} else {
		results, searchErr := searchTMDB(releaseName, tmdbType)
		if searchErr != nil {
			warnf("  Erreur recherche TMDB: %v\n", searchErr)
		} else if len(results) == 0 {
			warnf("  Aucun résultat pour \"%s\"\n", releaseName)
		} else {
			if *flagYes {
				// Mode non-interactif: premier résultat
				r := results[0]
				tmdbDetails, err = getTMDBDetails(r.ID, r.MediaType)
				if err != nil {
					warnf("  Erreur TMDB: %v\n", err)
				} else {
					tmdbType = r.MediaType
					ok("  (auto) %s (%s)", tmdbDetails.Title, tmdbDetails.Year)
				}
			} else {
				// Sélection interactive
				fmt.Printf("\n  Résultats TMDB pour \"%s\":\n\n", releaseName)
				max := len(results)
				if max > 8 {
					max = 8
				}
				for i, r := range results[:max] {
					fmt.Printf("    [%d] %-45s (%s)  %s\n",
						i+1,
						truncate(r.Title, 45),
						r.Year,
						r.MediaType,
					)
				}
				fmt.Printf("    [0] Ignorer TMDB\n\n")

				choice := promptInt("  Votre choix", 0, 0, max)
				if choice > 0 {
					r := results[choice-1]
					tmdbDetails, err = getTMDBDetails(r.ID, r.MediaType)
					if err != nil {
						warnf("  Erreur TMDB: %v\n", err)
					} else {
						tmdbType = r.MediaType
						ok("  Sélectionné: %s (%s)", tmdbDetails.Title, tmdbDetails.Year)
					}
				} else {
					warnf("  TMDB ignoré\n")
				}
			}
		}
	}

	// ── Étape 3 : NFO + Torrent ─────────────────────────────────────────────
	step(3, 4, "Génération NFO + création torrent")

	nfoContent := generateNFO(tmdbDetails, mediaInfo, mediaInfoCLI, settings)

	var lastPercent float64
	torrentResult, err := createTorrent(sourcePath, settings, func(p TorrentProgress) {
		switch p.Phase {
		case "start":
			fmt.Printf("  Hachage en cours...\n")
		case "hashing":
			if p.Percent-lastPercent >= 5 || p.Percent >= 100 {
				fmt.Printf("\r  Hachage : %5.1f%%  [%s / %s]",
					p.Percent,
					formatBytes(p.BytesDone),
					formatBytes(p.TotalBytes),
				)
				lastPercent = p.Percent
			}
		case "writing":
			fmt.Printf("\r  Hachage : 100.0%%  %-30s\n", "")
		}
	})
	if err != nil {
		fatalf("Erreur création torrent: %v\n", err)
	}
	ok("  Torrent  : %s", torrentResult.FilePath)
	ok("  InfoHash : %s", torrentResult.InfoHash)
	ok("  Taille   : %s", formatBytes(torrentResult.Size))

	// Sauvegarde NFO
	nfoPath := strings.TrimSuffix(torrentResult.FilePath, ".torrent") + ".nfo"
	if err := os.WriteFile(nfoPath, []byte(nfoContent), 0644); err != nil {
		warnf("  Impossible de sauvegarder le NFO: %v\n", err)
	} else {
		ok("  NFO      : %s", nfoPath)
	}

	if *flagNoUpload {
		fmt.Println()
		fmt.Println("  Upload désactivé (--no-upload). Terminé.")
		return
	}

	// ── Étape 4 : Upload ────────────────────────────────────────────────────
	step(4, 4, "Upload vers nexum-core.com")

	if settings.APIKey == "" {
		fmt.Println()
		fatalf("API key non configurée.\nModifiez ~/.config/GONEXUM/settings.json ou utilisez --config.\n")
	}

	categories := fetchCategoriesMap(settings)
	categoryID := *flagCategory
	if catLabel, ok2 := categories[categoryID]; ok2 {
		fmt.Printf("  Catégorie : %d — %s\n", categoryID, catLabel)
	}

	uploadResult, err := uploadTorrent(UploadParams{
		TorrentPath:       torrentResult.FilePath,
		NFOContent:        nfoContent,
		Name:              torrentResult.Name,
		CategoryID:        categoryID,
		Description:       bbcodeOrOverview(releaseName, mediaInfoCLI, tmdbDetails.Overview),
		TMDBId:            tmdbDetails.ID,
		TMDBType:          tmdbType,
		Resolution:        mediaInfo.Resolution,
		VideoCodec:        mediaInfo.VideoCodec,
		AudioCodec:        mediaInfo.AudioCodec,
		AudioLanguages:    mediaInfo.AudioLanguages,
		SubtitleLanguages: mediaInfo.SubtitleLanguages,
		HDRFormat:         mediaInfo.HDRFormat,
		Source:            mediaInfo.Source,
	}, settings)
	if err != nil {
		fatalf("Erreur upload: %v\n", err)
	}

	fmt.Println()
	ok("  Upload réussi!")
	ok("  ID      : %d", uploadResult.TorrentID)
	ok("  URL     : %s", uploadResult.URL)
	fmt.Println()
	fmt.Println("  Terminé.")
}

// ── Helpers d'affichage ──────────────────────────────────────────────────────

func printHeader() {
	fmt.Printf("╔══════════════════════════════════════╗\n")
	fmt.Printf("║         GONEXUM CLI %-16s ║\n", AppVersion)
	fmt.Printf("╚══════════════════════════════════════╝\n")
	fmt.Println()
	checkUpdate()
}

func checkUpdate() {
	resp, err := http.Get("https://api.github.com/repos/diabolino/gonexum-releases/releases/latest")
	if err != nil {
		return
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}
	var release struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.Unmarshal(body, &release); err != nil {
		return
	}
	latest := strings.TrimPrefix(release.TagName, "v")
	current := strings.TrimPrefix(AppVersion, "v")
	if latest != "" && latest != current && current != "dev" {
		fmt.Printf("  ★ Nouvelle version disponible : v%s → %s\n\n", latest, release.HTMLURL)
	}
}

func step(n, total int, label string) {
	fmt.Printf("\n[%d/%d] %s\n", n, total, label)
}

func ok(format string, args ...any) {
	fmt.Printf(format+"\n", args...)
}

func warnf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "! "+format, args...)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "ERREUR: "+format, args...)
	os.Exit(1)
}

func formatBytes(b int64) string {
	if b == 0 {
		return "0 B"
	}
	const k = 1024
	sizes := []string{"B", "KB", "MB", "GB", "TB"}
	i := 0
	f := float64(b)
	for f >= k && i < len(sizes)-1 {
		f /= k
		i++
	}
	return fmt.Sprintf("%.2f %s", f, sizes[i])
}

// ── Helpers interactifs ──────────────────────────────────────────────────────

var stdin = bufio.NewReader(os.Stdin)

func readLine(prompt string) string {
	fmt.Print(prompt + ": ")
	line, _ := stdin.ReadString('\n')
	return strings.TrimSpace(line)
}

func promptInt(label string, defaultVal, min, max int) int {
	for {
		raw := readLine(fmt.Sprintf("%s [%d-%d, défaut=%d]", label, min, max, defaultVal))
		if raw == "" {
			return defaultVal
		}
		n, err := strconv.Atoi(raw)
		if err != nil || n < min || n > max {
			fmt.Printf("  Valeur invalide, entrez un nombre entre %d et %d\n", min, max)
			continue
		}
		return n
	}
}

func promptChoice(label string, choices []string, defaultVal string) string {
	fmt.Printf("\n%s:\n", label)
	for i, c := range choices {
		marker := " "
		if c == defaultVal {
			marker = "*"
		}
		fmt.Printf("    [%d]%s %s\n", i+1, marker, c)
	}
	fmt.Printf("    [0]  Ignorer\n\n")
	for {
		raw := readLine(fmt.Sprintf("  Choix [0-%d]", len(choices)))
		if raw == "" && defaultVal != "" {
			return defaultVal
		}
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 || n > len(choices) {
			fmt.Printf("  Valeur invalide\n")
			continue
		}
		if n == 0 {
			return ""
		}
		return choices[n-1]
	}
}

// promptMediaOverrides asks user to confirm or override auto-detected media info.
// mediaDetected indique si mediainfo/ffprobe a tourné avec succès : dans ce cas
// le HDR vide = SDR confirmé, pas besoin de redemander.
func promptMediaOverrides(mi MediaInfo, mediaDetected bool) MediaInfo {
	resolutions := []string{"2160p", "1080p", "720p", "SD"}
	videoCodcecs := []string{"H.265", "H.264", "AV1", "VP9", "XviD"}
	audioCodcecs := []string{"Atmos", "TrueHD", "DTS-HD MA", "DTS-HD", "DTS", "EAC3", "AC3", "FLAC", "AAC", "MP3"}
	hdrFormats := []string{"", "HDR", "HDR10", "HDR10+", "DV", "HDR DV"}

	// Ne demander que si la valeur n'a pas été détectée automatiquement
	if mi.Resolution == "" {
		mi.Resolution = promptChoice("  Résolution", resolutions, "1080p")
	}
	if mi.VideoCodec == "" {
		mi.VideoCodec = promptChoice("  Codec vidéo", videoCodcecs, "H.264")
	}
	if mi.AudioCodec == "" {
		mi.AudioCodec = promptChoice("  Codec audio", audioCodcecs, "DTS")
	}
	// HDR : ne demander que si l'analyse n'a pas pu tourner (vide = on ne sait pas)
	// Si mediaDetected=true et HDRFormat="" → SDR confirmé, on ne redemande pas
	if mi.HDRFormat == "" && !mediaDetected {
		mi.HDRFormat = promptChoice("  Format HDR (optionnel)", hdrFormats, "")
	}
	if mi.AudioLanguages == "" {
		raw := readLine("  Langues audio (ex: Français, Anglais)")
		mi.AudioLanguages = raw
	}
	if mi.SubtitleLanguages == "" {
		raw := readLine("  Sous-titres (ex: Français, Anglais) [optionnel]")
		mi.SubtitleLanguages = raw
	}
	return mi
}
