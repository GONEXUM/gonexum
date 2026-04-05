package main

import (
	"fmt"
	"strings"
	"text/template"
)

// NFOTemplateData is passed to custom NFO templates
type NFOTemplateData struct {
	TMDB         TMDBDetails
	Media        MediaInfo
	MediaInfoCLI string
}

var nfoFuncMap = template.FuncMap{
	"padRight": func(s string, width int) string { return padRight(s, width) },
	"center":   func(s string, width int) string { return center(s, width) },
	"truncate": func(s string, max int) string { return truncate(s, max) },
	"repeat":   strings.Repeat,
	"join":     func(sep string, items []string) string { return strings.Join(items, sep) },
	"printf":   fmt.Sprintf,
}

func renderCustomNFO(tmpl string, data NFOTemplateData) (string, error) {
	t, err := template.New("nfo").Funcs(nfoFuncMap).Parse(tmpl)
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	if err := t.Execute(&sb, data); err != nil {
		return "", err
	}
	return sb.String(), nil
}

// generateNFO generates NFO content using custom template from settings or built-in default.
func generateNFO(details TMDBDetails, media MediaInfo, mediaInfoCLI string, settings Settings) string {
	if settings.NFOTemplate != "" {
		result, err := renderCustomNFO(settings.NFOTemplate, NFOTemplateData{
			TMDB:         details,
			Media:        media,
			MediaInfoCLI: mediaInfoCLI,
		})
		if err == nil {
			return result
		}
	}
	return generateDefaultNFO(details, mediaInfoCLI)
}

func generateDefaultNFO(details TMDBDetails, mediaInfoCLI string) string {
	const W = 90
	var sb strings.Builder

	line := func(s string) { sb.WriteString(s + "\n") }
	blank := func() { line("║" + strings.Repeat(" ", W) + "║") }
	sep := func() { line("╠" + strings.Repeat("═", W) + "╣") }

	const labelW = 16
	const valW = W - 4 - labelW

	line("╔" + strings.Repeat("═", W) + "╗")
	line("║" + center("GONEXUM NFO", W) + "║")
	sep()
	blank()

	if details.Title != "" {
		line("║  " + padRight("Titre:", labelW) + padRight(details.Title, valW) + "  ║")
		if details.Year != "" {
			line("║  " + padRight("Année:", labelW) + padRight(details.Year, valW) + "  ║")
		}
		if len(details.Genres) > 0 {
			line("║  " + padRight("Genre:", labelW) + padRight(strings.Join(details.Genres, ", "), valW) + "  ║")
		}
		if details.Director != "" {
			line("║  " + padRight("Réalisateur:", labelW) + padRight(details.Director, valW) + "  ║")
		}
		if details.Rating > 0 {
			line("║  " + padRight("Note:", labelW) + padRight(fmt.Sprintf("%.1f/10", details.Rating), valW) + "  ║")
		}
		if details.Runtime > 0 {
			line("║  " + padRight("Durée:", labelW) + padRight(fmt.Sprintf("%d min", details.Runtime), valW) + "  ║")
		}
		if details.ID > 0 {
			mediaType := details.MediaType
			if mediaType == "" {
				mediaType = "movie"
			}
			tmdbURL := fmt.Sprintf("https://www.themoviedb.org/%s/%d", mediaType, details.ID)
			line("║  " + padRight("TMDB:", labelW) + padRight(tmdbURL, valW) + "  ║")
		}
	}

	blank()

	if details.Overview != "" {
		sep()
		line("║" + center("SYNOPSIS", W) + "║")
		sep()
		blank()
		words := strings.Fields(details.Overview)
		cur := "  "
		for _, w := range words {
			if displayWidth(cur)+1+displayWidth(w) > W {
				line("║" + padRight(cur, W) + "║")
				cur = "  " + w
			} else if cur == "  " {
				cur += w
			} else {
				cur += " " + w
			}
		}
		if cur != "  " {
			line("║" + padRight(cur, W) + "║")
		}
		blank()
	}

	if mediaInfoCLI != "" {
		sep()
		line("║" + center("INFORMATIONS TECHNIQUES", W) + "║")
		sep()
		blank()
		for _, l := range strings.Split(mediaInfoCLI, "\n") {
			for _, chunk := range wrapLine(l, W-4) {
				line("║  " + padRight(chunk, W-4) + "  ║")
			}
		}
		blank()
	}

	sep()
	line("║" + center("Généré par GONEXUM — nexum-core.com", W) + "║")
	line("╚" + strings.Repeat("═", W) + "╝")

	return sb.String()
}

func wrapLine(s string, width int) []string {
	if s == "" {
		return []string{""}
	}
	var lines []string
	for displayWidth(s) > width {
		cut := truncateToWidth(s, width)
		lines = append(lines, cut)
		s = s[len(cut):]
	}
	lines = append(lines, s)
	return lines
}

func runeWidth(r rune) int {
	if r >= 0x1100 && (r <= 0x115F ||
		r == 0x2329 || r == 0x232A ||
		(r >= 0x2E80 && r <= 0x303E) ||
		(r >= 0x3040 && r <= 0x33FF) ||
		(r >= 0x3400 && r <= 0x4DBF) ||
		(r >= 0x4E00 && r <= 0xA4CF) ||
		(r >= 0xA960 && r <= 0xA97F) ||
		(r >= 0xAC00 && r <= 0xD7A3) ||
		(r >= 0xF900 && r <= 0xFAFF) ||
		(r >= 0xFE10 && r <= 0xFE1F) ||
		(r >= 0xFE30 && r <= 0xFE4F) ||
		(r >= 0xFF00 && r <= 0xFF60) ||
		(r >= 0xFFE0 && r <= 0xFFE6) ||
		(r >= 0x20000 && r <= 0x2FFFD) ||
		(r >= 0x30000 && r <= 0x3FFFD)) {
		return 2
	}
	return 1
}

func displayWidth(s string) int {
	w := 0
	for _, r := range s {
		w += runeWidth(r)
	}
	return w
}

func center(s string, width int) string {
	n := displayWidth(s)
	if n >= width {
		return truncateToWidth(s, width)
	}
	total := width - n
	left := total / 2
	right := total - left
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", right)
}

func padRight(s string, width int) string {
	n := displayWidth(s)
	if n >= width {
		return truncateToWidth(s, width)
	}
	return s + strings.Repeat(" ", width-n)
}

func truncateToWidth(s string, width int) string {
	w := 0
	for i, r := range s {
		rw := runeWidth(r)
		if w+rw > width {
			return s[:i]
		}
		w += rw
	}
	return s
}

func truncate(s string, max int) string {
	if displayWidth(s) <= max {
		return s
	}
	if max <= 3 {
		return truncateToWidth(s, max)
	}
	target := max - 3
	w := 0
	var result []rune
	for _, r := range s {
		rw := runeWidth(r)
		if w+rw > target {
			break
		}
		w += rw
		result = append(result, r)
	}
	return string(result) + "..."
}
