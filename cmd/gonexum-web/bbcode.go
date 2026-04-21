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
// Output uses nexum-core.com banner images as section separators.
func generateBBCodeDescription(releaseName, mediaInfoCLI string) string {
	if mediaInfoCLI == "" {
		return ""
	}

	sections := parseMediaInfoSections(mediaInfoCLI)
	var lines []string

	// ── General ──
	for _, s := range sections {
		if s.name != "General" {
			continue
		}
		lines = append(lines, "[img]https://nexum-core.com/img/banners/general.svg[/img]")
		lines = append(lines, "")
		if v := firstOf(s, "File name", "Complete name"); v != "" {
			if i := strings.LastIndexAny(v, "/\\"); i >= 0 {
				v = v[i+1:]
			}
			lines = append(lines, "[b]Nom du fichier :[/b] [i]"+v+"[/i]")
		}
		if v := s.fields["Format"]; v != "" {
			lines = append(lines, "[b]Format :[/b] [i]"+v+"[/i]")
		}
		if v := s.fields["File size"]; v != "" {
			lines = append(lines, "[b]Taille :[/b] [i]"+v+"[/i]")
		}
		if v := s.fields["Duration"]; v != "" {
			lines = append(lines, "[b]Durée :[/b] [i]"+v+"[/i]")
		}
		if v := s.fields["Overall bit rate"]; v != "" {
			lines = append(lines, "[b]Débit global :[/b] [i]"+v+"[/i]")
		}
		break
	}

	// ── Video (first track) ──
	for _, s := range sections {
		if s.name != "Video" {
			continue
		}
		lines = append(lines, "")
		lines = append(lines, "[img]https://nexum-core.com/img/banners/video.svg[/img]")
		lines = append(lines, "")
		codec := firstOf(s, "Format")
		profile := firstOf(s, "Format profile")
		if codec != "" {
			label := codec
			if profile != "" {
				label += " - Profil " + profile
			}
			lines = append(lines, "[b]Codec :[/b] [i]"+label+"[/i]")
		}
		w := stripNonDigit(s.fields["Width"])
		h := stripNonDigit(s.fields["Height"])
		if w != "" && h != "" {
			lines = append(lines, "[b]Résolution :[/b] [i]"+w+"x"+h+"[/i]")
		}
		if v := firstOf(s, "HDR format"); v != "" {
			lines = append(lines, "[b]HDR :[/b] [i]"+v+"[/i]")
		}
		if v := s.fields["Frame rate"]; v != "" {
			lines = append(lines, "[b]Fréquence d'images :[/b] [i]"+v+"[/i]")
		}
		if v := s.fields["Bit rate"]; v != "" {
			lines = append(lines, "[b]Débit vidéo :[/b] [i]"+v+"[/i]")
		}
		if v := s.fields["Bit depth"]; v != "" {
			lines = append(lines, "[b]Profondeur des couleurs :[/b] [i]"+v+"[/i]")
		}
		break
	}

	// ── Audio tracks ──
	var audios []miSection
	for _, s := range sections {
		if s.name == "Audio" || strings.HasPrefix(s.name, "Audio #") || strings.HasPrefix(s.name, "Audio ") {
			audios = append(audios, s)
		}
	}
	if len(audios) > 0 {
		lines = append(lines, "")
		lines = append(lines, "[img]https://nexum-core.com/img/banners/audio.svg[/img]")
		lines = append(lines, "")
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
			entry := lang + " : " + format
			if ch != "" {
				entry += " - " + ch
			}
			entry += br
			lines = append(lines, entry)
		}
	}

	// ── Subtitles ──
	var subs []miSection
	for _, s := range sections {
		if s.name == "Text" || strings.HasPrefix(s.name, "Text #") || strings.HasPrefix(s.name, "Text ") {
			subs = append(subs, s)
		}
	}
	if len(subs) > 0 {
		lines = append(lines, "")
		lines = append(lines, "[img]https://nexum-core.com/img/banners/subtitles.svg[/img]")
		lines = append(lines, "")
		var trackLabels []string
		for _, s := range subs {
			lang := s.fields["Language"]
			if lang == "" {
				lang = "Inconnu"
			}
			var extras []string
			if s.fields["Forced"] == "Yes" {
				extras = append(extras, "Forcés")
			}
			if len(extras) > 0 {
				trackLabels = append(trackLabels, lang+" ("+strings.Join(extras, ", ")+")")
			} else {
				trackLabels = append(trackLabels, lang)
			}
		}
		lines = append(lines, strings.Join(trackLabels, ", "))
	}

	lines = append(lines, "")
	return strings.Join(lines, "\n")
}

// stripNonDigit removes non-digit characters (spaces, "pixels", etc.) from a value.
func stripNonDigit(s string) string {
	// "1 920 pixels" → "1920"
	s = strings.ReplaceAll(s, " ", "")
	s = strings.TrimSuffix(s, "pixels")
	return strings.TrimSpace(s)
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
