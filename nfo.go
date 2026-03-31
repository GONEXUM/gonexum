package main

import (
	"fmt"
	"strings"
	"text/template"
	"unicode/utf8"
)

// NFOTemplateData is passed to custom NFO templates
type NFOTemplateData struct {
	TMDB  TMDBDetails
	Media MediaInfo
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
func (a *App) GenerateNFO(details TMDBDetails, media MediaInfo) string {
	s, _ := loadSettings()
	if s.NFOTemplate != "" {
		result, err := renderCustomNFO(s.NFOTemplate, NFOTemplateData{TMDB: details, Media: media})
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
		line("║  " + padRight("Langues:", 16) + padRight(truncate(media.AudioLanguages, 40), 40) + "  ║")
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

func center(s string, width int) string {
	n := utf8.RuneCountInString(s)
	if n >= width {
		return string([]rune(s)[:width])
	}
	total := width - n
	left := total / 2
	right := total - left
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", right)
}

func padRight(s string, width int) string {
	n := utf8.RuneCountInString(s)
	if n >= width {
		return string([]rune(s)[:width])
	}
	return s + strings.Repeat(" ", width-n)
}

func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	if max <= 3 {
		return string(runes[:max])
	}
	return string(runes[:max-3]) + "..."
}
