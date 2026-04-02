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
	MediaInfoCLI string // sortie complète style mediainfo CLI
}

// nfoFuncMap exposes helper functions available inside custom templates
var nfoFuncMap = template.FuncMap{
	"padRight": func(s string, width int) string { return padRight(s, width) },
	"center":   func(s string, width int) string { return center(s, width) },
	"truncate": func(s string, max int) string { return truncate(s, max) },
	"repeat":   strings.Repeat,
	"join":     func(sep string, items []string) string { return strings.Join(items, sep) },
	"printf":   fmt.Sprintf,
}

// renderCustomNFO parses and executes a user-defined text/template NFO template.
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

// ValidateNFOTemplate parses a template string and returns any syntax error.
func (a *App) ValidateNFOTemplate(tmpl string) error {
	_, err := template.New("nfo").Funcs(nfoFuncMap).Parse(tmpl)
	return err
}

// GenerateNFO generates the NFO file content.
// If a custom NFOTemplate is saved in settings it is used; otherwise the built-in layout is applied.
// mediaInfoCLI is the optional full CLI-style mediainfo text, available as {{.MediaInfoCLI}} in templates.
func (a *App) GenerateNFO(details TMDBDetails, media MediaInfo, mediaInfoCLI string) string {
	s, _ := loadSettings()
	if s.NFOTemplate != "" {
		result, err := renderCustomNFO(s.NFOTemplate, NFOTemplateData{TMDB: details, Media: media, MediaInfoCLI: mediaInfoCLI})
		if err == nil {
			return result
		}
		// Fall back to default on execution error
	}
	return generateDefaultNFO(details, media)
}

// generateDefaultNFO produces the built-in box-drawing NFO layout.
func generateDefaultNFO(details TMDBDetails, media MediaInfo) string {
	var sb strings.Builder

	line := func(s string) { sb.WriteString(s + "\n") }

	sep := strings.Repeat("─", 60)

	line("╔" + strings.Repeat("═", 60) + "╗")
	line("║" + center("GONEXUM NFO", 60) + "║")
	line("╠" + strings.Repeat("═", 60) + "╣")
	line("║" + "                                                            " + "║")

	if details.Title != "" {
		line("║  " + padRight("Titre:", 16) + padRight(truncate(details.Title, 40), 40) + "  ║")
		if details.Year != "" {
			line("║  " + padRight("Année:", 16) + padRight(details.Year, 40) + "  ║")
		}
		if len(details.Genres) > 0 {
			genres := strings.Join(details.Genres, ", ")
			line("║  " + padRight("Genre:", 16) + padRight(truncate(genres, 40), 40) + "  ║")
		}
		if details.Director != "" {
			line("║  " + padRight("Réalisateur:", 16) + padRight(truncate(details.Director, 40), 40) + "  ║")
		}
		if details.Rating > 0 {
			rating := fmt.Sprintf("%.1f/10", details.Rating)
			line("║  " + padRight("Note:", 16) + padRight(rating, 40) + "  ║")
		}
		if details.Runtime > 0 {
			rt := fmt.Sprintf("%d min", details.Runtime)
			line("║  " + padRight("Durée:", 16) + padRight(rt, 40) + "  ║")
		}
		if details.ID > 0 {
			mediaType := details.MediaType
			if mediaType == "" {
				mediaType = "movie"
			}
			tmdbURL := fmt.Sprintf("https://www.themoviedb.org/%s/%d", mediaType, details.ID)
			line("║  " + padRight("TMDB:", 16) + padRight(truncate(tmdbURL, 40), 40) + "  ║")
		}
	}

	line("║" + "                                                            " + "║")
	line("╠" + strings.Repeat("═", 60) + "╣")
	line("║" + center("INFORMATIONS TECHNIQUES", 60) + "║")
	line("╠" + strings.Repeat("═", 60) + "╣")
	line("║" + "                                                            " + "║")

	if media.Resolution != "" {
		line("║  " + padRight("Résolution:", 16) + padRight(media.Resolution, 40) + "  ║")
	}
	if media.VideoCodec != "" {
		line("║  " + padRight("Vidéo:", 16) + padRight(media.VideoCodec, 40) + "  ║")
	}
	if media.AudioCodec != "" {
		line("║  " + padRight("Audio:", 16) + padRight(media.AudioCodec, 40) + "  ║")
	}
	if media.AudioLanguages != "" {
		line("║  " + padRight("Langues audio:", 16) + padRight(truncate(media.AudioLanguages, 40), 40) + "  ║")
	}
	if media.SubtitleLanguages != "" {
		line("║  " + padRight("Sous-titres:", 16) + padRight(truncate(media.SubtitleLanguages, 40), 40) + "  ║")
	}
	if media.HDRFormat != "" {
		line("║  " + padRight("HDR:", 16) + padRight(media.HDRFormat, 40) + "  ║")
	}
	if media.Source != "" {
		line("║  " + padRight("Source:", 16) + padRight(media.Source, 40) + "  ║")
	}
	if media.Duration != "" {
		line("║  " + padRight("Durée fichier:", 16) + padRight(media.Duration, 40) + "  ║")
	}
	if media.FrameRate > 0 {
		fps := fmt.Sprintf("%.2f fps", media.FrameRate)
		line("║  " + padRight("FPS:", 16) + padRight(fps, 40) + "  ║")
	}

	line("║" + "                                                            " + "║")

	if details.Overview != "" {
		line("╠" + strings.Repeat("═", 60) + "╣")
		line("║" + center("SYNOPSIS", 60) + "║")
		line("╠" + strings.Repeat("═", 60) + "╣")
		line("║" + "                                                            " + "║")
		words := strings.Fields(details.Overview)
		currentLine := "  "
		for _, w := range words {
			if len(currentLine)+len(w)+1 > 58 {
				line("║" + padRight(currentLine, 60) + "║")
				currentLine = "  " + w
			} else {
				if currentLine == "  " {
					currentLine += w
				} else {
					currentLine += " " + w
				}
			}
		}
		if currentLine != "  " {
			line("║" + padRight(currentLine, 60) + "║")
		}
		line("║" + "                                                            " + "║")
	}

	line("╠" + strings.Repeat("═", 60) + "╣")
	line("║" + center("Généré par GONEXUM — nexum-core.com", 60) + "║")
	line("╚" + strings.Repeat("═", 60) + "╝")
	_ = sep

	return sb.String()
}

// runeWidth retourne la largeur visuelle d'un rune en monospace :
// 2 pour les caractères CJK/pleine largeur, 1 pour les autres.
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

// displayWidth retourne la largeur visuelle d'une chaîne en monospace.
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

// truncateToWidth coupe s à exactement width colonnes visuelles.
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
