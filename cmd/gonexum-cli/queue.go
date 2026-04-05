package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type queueResult struct {
	source  string
	name    string
	success bool
	url     string
	err     string
}

// runQueue processes multiple source paths sequentially in automatic mode (--yes).
func runQueue(sources []string, category int, tmdbType, forceSource string,
	noUpload bool, output, config, nfoMode, nfoTemplate string) {

	printHeader()
	fmt.Printf("  Mode queue : %d fichiers à traiter\n\n", len(sources))

	// Load settings once
	settings, err := loadSettings()
	if err != nil {
		warnf("Impossible de charger les settings: %v\n", err)
		settings = Settings{TrackerURL: "https://nexum-core.com"}
	}
	if err := ensureSettings(&settings, noUpload); err != nil {
		fatalf("%v\n", err)
	}
	if nfoMode != "" {
		settings.NFOMode = nfoMode
	}
	if nfoTemplate != "" {
		tmplData, err := os.ReadFile(nfoTemplate)
		if err != nil {
			fatalf("Impossible de lire le template NFO: %v\n", err)
		}
		settings.NFOTemplate = string(tmplData)
	}

	var results []queueResult

	for i, sourcePath := range sources {
		fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		fmt.Printf("[%d/%d] %s\n\n", i+1, len(sources), filepath.Base(sourcePath))

		res := processOne(sourcePath, category, tmdbType, forceSource, noUpload, output, settings)
		results = append(results, res)

		if res.success {
			ok("  ✓ OK%s\n", func() string {
				if res.url != "" {
					return " — " + res.url
				}
				return ""
			}())
		} else {
			warnf("  ✗ Erreur: %s\n", res.err)
		}
		fmt.Println()
	}

	// Summary
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("  RÉSUMÉ QUEUE\n\n")
	ok2 := 0
	for _, r := range results {
		if r.success {
			ok2++
			fmt.Printf("  ✓  %s\n", r.name)
			if r.url != "" {
				fmt.Printf("     %s\n", r.url)
			}
		} else {
			fmt.Printf("  ✗  %s\n     %s\n", r.name, r.err)
		}
	}
	fmt.Printf("\n  %d/%d réussi(s)\n\n", ok2, len(sources))
}

func processOne(sourcePath string, category int, tmdbType, forceSource string,
	noUpload bool, output string, settings Settings) queueResult {

	res := queueResult{source: sourcePath}

	if _, err := os.Stat(sourcePath); err != nil {
		res.err = "chemin invalide: " + err.Error()
		return res
	}

	// Output dir
	s := settings
	if output != "" {
		s.OutputDir = output
	}
	if s.OutputDir == "" {
		s.OutputDir = filepath.Dir(sourcePath)
	}

	// Release name
	releaseName := filepath.Base(sourcePath)
	if ext := filepath.Ext(releaseName); ext != "" {
		fi, _ := os.Stat(sourcePath)
		if fi != nil && !fi.IsDir() {
			releaseName = strings.TrimSuffix(releaseName, ext)
		}
	}
	res.name = releaseName

	// ── 1. Media info ──────────────────────────────────────────────────
	fmt.Printf("  [1/4] Media info...\n")
	var mediaInfo MediaInfo
	var mediaInfoCLI string
	if videoPath, err := largestVideoFile(sourcePath); err == nil {
		if mi, cliText, _, err := getMediaInfo(videoPath); err == nil {
			mediaInfo = mi
			mediaInfoCLI = cliText
			fmt.Printf("        %s | %s | %s\n", mi.Resolution, mi.VideoCodec, mi.AudioCodec)
		}
	}
	if forceSource != "" {
		mediaInfo.Source = forceSource
	} else if mediaInfo.Source == "" {
		mediaInfo.Source = detectSourceFromName(releaseName)
	}

	// ── 2. TMDB ────────────────────────────────────────────────────────
	fmt.Printf("  [2/4] TMDB...\n")
	var tmdbDetails TMDBDetails
	resolvedType := tmdbType
	if results, err := searchTMDB(releaseName, tmdbType); err == nil && len(results) > 0 {
		r := results[0]
		if d, err := getTMDBDetails(r.ID, r.MediaType); err == nil {
			tmdbDetails = d
			resolvedType = r.MediaType
			fmt.Printf("        %s (%s)\n", tmdbDetails.Title, tmdbDetails.Year)
		}
	} else {
		fmt.Printf("        aucun résultat\n")
	}
	_ = resolvedType

	// ── 3. NFO + Torrent ───────────────────────────────────────────────
	fmt.Printf("  [3/4] NFO + torrent...\n")
	nfoContent := generateNFO(tmdbDetails, mediaInfo, mediaInfoCLI, s)

	var lastPct float64
	torrentResult, err := createTorrent(sourcePath, s, func(p TorrentProgress) {
		if p.Phase == "hashing" && p.Percent-lastPct >= 10 {
			fmt.Printf("\r        Hachage : %5.1f%%", p.Percent)
			lastPct = p.Percent
		}
		if p.Phase == "writing" {
			fmt.Printf("\r        Hachage : 100.0%%\n")
		}
	})
	if err != nil {
		res.err = "torrent: " + err.Error()
		return res
	}

	nfoPath := strings.TrimSuffix(torrentResult.FilePath, ".torrent") + ".nfo"
	_ = os.WriteFile(nfoPath, []byte(nfoContent), 0644)

	if noUpload {
		res.success = true
		return res
	}

	// ── 4. Upload ──────────────────────────────────────────────────────
	fmt.Printf("  [4/4] Upload...\n")
	uploadResult, err := uploadTorrent(UploadParams{
		TorrentPath:       torrentResult.FilePath,
		NFOContent:        nfoContent,
		Name:              torrentResult.Name,
		CategoryID:        category,
		TMDBId:            tmdbDetails.ID,
		TMDBType:          resolvedType,
		Resolution:        mediaInfo.Resolution,
		VideoCodec:        mediaInfo.VideoCodec,
		AudioCodec:        mediaInfo.AudioCodec,
		AudioLanguages:    mediaInfo.AudioLanguages,
		SubtitleLanguages: mediaInfo.SubtitleLanguages,
		HDRFormat:         mediaInfo.HDRFormat,
		Source:            mediaInfo.Source,
	}, s)
	if err != nil {
		res.err = "upload: " + err.Error()
		return res
	}

	res.success = true
	res.url = uploadResult.URL
	return res
}
