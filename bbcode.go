package main

import (
	"fmt"
	"strings"
)

type miSection struct {
	name   string
	fields map[string]string
}

func parseMediaInfoSections(text string) []miSection {
	var sections []miSection
	cur := -1
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimRight(line, "\r ")
		if line == "" {
			cur = -1
			continue
		}
		idx := strings.Index(line, " : ")
		if idx < 0 {
			// Section header
			sections = append(sections, miSection{name: strings.TrimSpace(line), fields: make(map[string]string)})
			cur = len(sections) - 1
			continue
		}
		if cur < 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+3:])
		// Keep only the first occurrence of each key
		if _, exists := sections[cur].fields[key]; !exists {
			sections[cur].fields[key] = val
		}
	}
	return sections
}

// generateBBCodeDescription generates a BBCode technical description from
// the release name and the raw mediainfo CLI output.
func generateBBCodeDescription(releaseName, mediaInfoCLI string) string {
	if mediaInfoCLI == "" {
		return ""
	}

	sections := parseMediaInfoSections(mediaInfoCLI)

	// Clean release name for display
	cleanName := releaseName
	for _, ext := range []string{".mkv", ".mp4", ".avi", ".ts", ".m2ts", ".MKV", ".MP4"} {
		cleanName = strings.TrimSuffix(cleanName, ext)
	}
	cleanName = strings.NewReplacer(".", " ", "_", " ").Replace(cleanName)

	var lines []string
	lines = append(lines, fmt.Sprintf("[h1][b]%s[/b][/h1]", cleanName))
	lines = append(lines, "[hr]")

	// ── General ──
	for _, s := range sections {
		if s.name != "General" {
			continue
		}
		lines = append(lines, "[b][color=#1E90FF]► INFORMATIONS GÉNÉRALES[/color][/b]")
		lines = append(lines, "[list]")
		if v := firstOf(s, "File name", "Complete name"); v != "" {
			// Extract just the filename from full path
			if i := strings.LastIndexAny(v, "/\\"); i >= 0 {
				v = v[i+1:]
			}
			lines = append(lines, "[*][b]Nom du fichier :[/b] "+v)
		}
		addField(&lines, s, "Format", "Format")
		addField(&lines, s, "File size", "Taille")
		addField(&lines, s, "Duration", "Durée")
		addField(&lines, s, "Overall bit rate", "Débit global")
		lines = append(lines, "[/list]")
		break
	}

	// ── Video (first track) ──
	for _, s := range sections {
		if s.name != "Video" {
			continue
		}
		lines = append(lines, "[b][color=#1E90FF]► VIDÉO[/color][/b]")
		lines = append(lines, "[list]")
		codec := firstOf(s, "Commercial name", "Format")
		if codec != "" {
			lines = append(lines, "[*][b]Codec :[/b] "+codec)
		}
		if v := firstOf(s, "HDR format"); v != "" {
			lines = append(lines, "[*][b]HDR :[/b] "+v)
		}
		w := s.fields["Width"]
		h := s.fields["Height"]
		if w != "" && h != "" {
			lines = append(lines, fmt.Sprintf("[*][b]Résolution :[/b] %s x %s", w, h))
		}
		addField(&lines, s, "Display aspect ratio", "Ratio d'affichage")
		addField(&lines, s, "Frame rate", "Fréquence d'images")
		addField(&lines, s, "Bit rate", "Débit vidéo")
		addField(&lines, s, "Bit depth", "Profondeur des couleurs")
		lines = append(lines, "[/list]")
		break
	}

	// ── Audio tracks ──
	var audios []miSection
	for _, s := range sections {
		if s.name == "Audio" {
			audios = append(audios, s)
		}
	}
	if len(audios) > 0 {
		pl := ""
		if len(audios) > 1 {
			pl = "s"
		}
		lines = append(lines, fmt.Sprintf("[b][color=#1E90FF]► AUDIO (%d Piste%s)[/color][/b]", len(audios), pl))
		lines = append(lines, "[list]")
		for _, a := range audios {
			lang := a.fields["Language"]
			if lang == "" {
				lang = "Inconnu"
			}
			format := firstOf(a, "Commercial name", "Format")
			if format == "" {
				format = "Inconnu"
			}
			ch := firstOf(a, "Channel(s)", "Channels")
			br := ""
			if v := a.fields["Bit rate"]; v != "" {
				br = " @ " + v
			}
			def := ""
			if a.fields["Default"] == "Yes" {
				def = " [i](Par défaut)[/i]"
			}
			entry := fmt.Sprintf("[*][b]%s :[/b] %s", lang, format)
			if ch != "" {
				entry += " - " + ch
			}
			entry += br + def
			lines = append(lines, entry)
		}
		lines = append(lines, "[/list]")
	}

	// ── Subtitles ──
	var subs []miSection
	for _, s := range sections {
		if s.name == "Text" {
			subs = append(subs, s)
		}
	}
	if len(subs) > 0 {
		pl := ""
		if len(subs) > 1 {
			pl = "s"
		}
		lines = append(lines, fmt.Sprintf("[b][color=#1E90FF]► SOUS-TITRES (%d Piste%s)[/color][/b]", len(subs), pl))
		lines = append(lines, "[list]")

		fmtSet := map[string]bool{}
		var trackLabels []string
		for _, s := range subs {
			if f := s.fields["Format"]; f != "" {
				fmtSet[f] = true
			}
			lang := s.fields["Language"]
			if lang == "" {
				lang = "Inconnu"
			}
			var extras []string
			if s.fields["Forced"] == "Yes" {
				extras = append(extras, "Forcés")
			}
			if s.fields["Default"] == "Yes" && len(subs) > 1 {
				extras = append(extras, "Par défaut")
			}
			if len(extras) > 0 {
				trackLabels = append(trackLabels, lang+" ("+strings.Join(extras, ", ")+")")
			} else {
				trackLabels = append(trackLabels, lang)
			}
		}
		var fmtList []string
		for f := range fmtSet {
			fmtList = append(fmtList, f)
		}
		if len(fmtList) > 0 {
			lines = append(lines, "[*][b]Format :[/b] "+strings.Join(fmtList, ", "))
		}
		lines = append(lines, "[*][b]Pistes incluses :[/b] "+strings.Join(trackLabels, ", "))
		lines = append(lines, "[/list]")
	}

	lines = append(lines, "[hr]")
	return strings.Join(lines, "\n")
}

// bbcodeOrOverview returns BBCode description if mediainfo is available,
// otherwise falls back to TMDB overview.
func bbcodeOrOverview(releaseName, mediaInfoCLI, overview string) string {
	if desc := generateBBCodeDescription(releaseName, mediaInfoCLI); desc != "" {
		return desc
	}
	return overview
}

func firstOf(s miSection, keys ...string) string {
	for _, k := range keys {
		if v := s.fields[k]; v != "" {
			return v
		}
	}
	return ""
}

func addField(lines *[]string, s miSection, key, label string) {
	if v := s.fields[key]; v != "" {
		*lines = append(*lines, fmt.Sprintf("[*][b]%s :[/b] %s", label, v))
	}
}
