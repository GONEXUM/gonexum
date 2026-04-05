package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

var videoExts = map[string]bool{
	".mkv": true, ".mp4": true, ".avi": true, ".mov": true,
	".ts": true, ".m2ts": true, ".wmv": true, ".m4v": true,
}

// largestVideoFile returns the path of the largest video file in a directory,
// or the path itself if it's already a file.
func largestVideoFile(path string) (string, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !fi.IsDir() {
		return path, nil
	}
	var best string
	var bestSize int64
	filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(p))
		if videoExts[ext] && info.Size() > bestSize {
			bestSize = info.Size()
			best = p
		}
		return nil
	})
	if best == "" {
		return "", fmt.Errorf("aucun fichier vidéo trouvé dans le dossier")
	}
	return best, nil
}

// getMediaInfo extrait les infos média.
// Essaie d'abord mediainfo (plus répandu sur les seedboxes), puis ffprobe en fallback.
// Retourne aussi le texte CLI de mediainfo pour le NFO (si disponible).
// Le bool indique si l'analyse a réussi (même résultat vide = SDR confirmé, pas besoin de redemander).
func getMediaInfo(path string) (MediaInfo, string, bool, error) {
	// Priorité 1 : mediainfo
	mi, err := getMediaInfoFromMediainfo(path)
	if err == nil {
		cliText := getMediaInfoCLIText(path)
		return mi, cliText, true, nil
	}
	miErr := err

	// Fallback : ffprobe
	mi, err = getMediaInfoFFprobe(path)
	if err != nil {
		return MediaInfo{}, "", false, fmt.Errorf("mediainfo: %v — ffprobe: %v", miErr, err)
	}
	return mi, "", true, nil
}

// detectSourceFromName tente de détecter la source depuis le nom du release.
// Ex: "Fight.Club.1999.1080p.BluRay.x264" → "BluRay"
func detectSourceFromName(name string) string {
	upper := strings.ToUpper(name)

	// Patterns avec tiret en premier (plus spécifiques)
	switch {
	case strings.Contains(upper, "WEB-DL"):
		return "WEB-DL"
	case strings.Contains(upper, "BLU-RAY"):
		return "BluRay"
	case strings.Contains(upper, "WEB-RIP"):
		return "WEBRip"
	}

	// Détection par token (séparateurs: . - _ espace)
	tokens := strings.FieldsFunc(upper, func(r rune) bool {
		return r == '.' || r == '-' || r == '_' || r == ' '
	})
	for _, t := range tokens {
		switch t {
		case "BLURAY", "BDRIP", "BDREMUX", "REMUX", "BDMV":
			return "BluRay"
		case "WEBDL":
			return "WEB-DL"
		case "WEBRIP", "WEB":
			return "WEBRip"
		case "HDTV":
			return "HDTV"
		case "DVDRIP", "DVDSCR":
			return "DVDRip"
		case "DCP":
			return "DCP"
		}
	}
	return ""
}

// ════════════════════════════════════════════════════════════════
//  mediainfo --Output=JSON
// ════════════════════════════════════════════════════════════════

type mediainfoJSON struct {
	Media struct {
		Track []mediainfoTrack `json:"track"`
	} `json:"media"`
}

type mediainfoTrack struct {
	Type                    string `json:"@type"`
	Format                  string `json:"Format"`
	FormatProfile           string `json:"Format_Profile"`
	FormatCommercial        string `json:"Format_Commercial_IfAny"`
	CodecID                 string `json:"CodecID"`
	FileSize                string `json:"FileSize"`
	Duration                string `json:"Duration"`
	OverallBitRate          string `json:"OverallBitRate"`
	Width                   string `json:"Width"`
	Height                  string `json:"Height"`
	FrameRate               string `json:"FrameRate"`
	HDRFormat               string `json:"HDR_Format"`
	HDRFormatCompatibility  string `json:"HDR_Format_Compatibility"`
	TransferCharacteristics string `json:"transfer_characteristics"`
	ColourPrimaries         string `json:"colour_primaries"`
	Channels                string `json:"Channels"`
	Language                string `json:"Language"`
	Default                 string `json:"Default"`
	Forced                  string `json:"Forced"`
}

func getMediaInfoFromMediainfo(path string) (MediaInfo, error) {
	cmd := exec.Command("mediainfo", "--Output=JSON", path)
	out, err := cmd.Output()
	if err != nil {
		return MediaInfo{}, fmt.Errorf("mediainfo non disponible: %w", err)
	}

	var data mediainfoJSON
	if err := json.Unmarshal(out, &data); err != nil {
		return MediaInfo{}, fmt.Errorf("parsing mediainfo JSON: %w", err)
	}

	var mi MediaInfo

	var audioCodecs []string
	var audioLangs []string
	var subLangs []string

	for _, t := range data.Media.Track {
		switch t.Type {
		case "General":
			if sz, err := strconv.ParseInt(t.FileSize, 10, 64); err == nil {
				mi.FileSize = sz
			}
			if br, err := strconv.ParseInt(t.OverallBitRate, 10, 64); err == nil {
				mi.Bitrate = br
			}
			if dur, err := strconv.ParseFloat(t.Duration, 64); err == nil {
				h := int(dur) / 3600
				m := (int(dur) % 3600) / 60
				if h > 0 {
					mi.Duration = fmt.Sprintf("%dh %02dmin", h, m)
				} else {
					mi.Duration = fmt.Sprintf("%dmin", m)
				}
			}

		case "Video":
			if w, err := strconv.Atoi(t.Width); err == nil {
				mi.Width = w
			}
			if h, err := strconv.Atoi(t.Height); err == nil {
				mi.Height = h
			}
			mi.Resolution = detectResolution(mi.Width, mi.Height)
			mi.VideoCodec = normalizeVideoCodecMediainfo(t.Format, t.FormatProfile)
			if fps, err := strconv.ParseFloat(t.FrameRate, 64); err == nil {
				mi.FrameRate = math.Round(fps*1000) / 1000
			}
			mi.HDRFormat = detectHDRMediainfo(t)

		case "Audio":
			codec := normalizeAudioCodecMediainfo(t.Format, t.FormatProfile, t.FormatCommercial)
			if codec != "" && !sliceContains(audioCodecs, codec) {
				audioCodecs = append(audioCodecs, codec)
			}
			if lang := normalizeLanguage(t.Language); lang != "" {
				if !sliceContains(audioLangs, lang) {
					audioLangs = append(audioLangs, lang)
				}
			}

		case "Text":
			lang := normalizeLanguage(t.Language)
			if lang == "" {
				lang = "Inconnu"
			}
			forced := strings.EqualFold(t.Forced, "yes")
			entry := lang
			if forced {
				entry += " (forcé)"
			}
			if !sliceContains(subLangs, entry) {
				subLangs = append(subLangs, entry)
			}
		}
	}

	if len(audioCodecs) > 0 {
		mi.AudioCodec = audioCodecs[0]
	}
	if len(audioLangs) > 0 {
		mi.AudioLanguages = strings.Join(audioLangs, ", ")
	}
	if len(subLangs) > 0 {
		mi.SubtitleLanguages = strings.Join(subLangs, ", ")
	}

	return mi, nil
}

// getMediaInfoCLIText runs mediainfo without flags to get the human-readable output for the NFO.
func getMediaInfoCLIText(path string) string {
	cmd := exec.Command("mediainfo", path)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return string(out)
}

func detectHDRMediainfo(t mediainfoTrack) string {
	commercial := strings.ToLower(t.HDRFormat)

	// Dolby Vision (can coexist with HDR10)
	if strings.Contains(commercial, "dolby vision") {
		if strings.Contains(commercial, "hdr10+") {
			return "HDR DV"
		}
		return "DV"
	}
	// HDR10+
	if strings.Contains(commercial, "smpte st 2094") || strings.Contains(commercial, "hdr10+") {
		return "HDR10+"
	}
	// HDR10
	if strings.Contains(commercial, "smpte st 2086") ||
		strings.EqualFold(t.HDRFormatCompatibility, "HDR10") {
		return "HDR10"
	}
	// HLG
	if strings.EqualFold(t.TransferCharacteristics, "HLG") ||
		strings.EqualFold(t.TransferCharacteristics, "arib-std-b67") {
		return "HLG"
	}
	// PQ without explicit Dolby/HDR10 tag → generic HDR
	if strings.EqualFold(t.TransferCharacteristics, "PQ") {
		return "HDR"
	}
	return ""
}

func normalizeVideoCodecMediainfo(format, profile string) string {
	switch strings.ToUpper(format) {
	case "AVC":
		return "H.264"
	case "HEVC":
		return "H.265"
	case "AV1":
		return "AV1"
	case "VP9":
		return "VP9"
	case "XVID", "DIVX":
		return "XviD"
	case "MPEG-4 VISUAL":
		return "XviD"
	case "MPEG VIDEO":
		return "MPEG-2"
	default:
		return strings.ToUpper(format)
	}
}

func normalizeAudioCodecMediainfo(format, profile, commercial string) string {
	c := strings.ToLower(commercial)

	// Commercial name first (most specific)
	switch {
	case strings.Contains(c, "atmos"):
		return "Atmos"
	case strings.Contains(c, "dts:x"):
		return "DTS:X"
	case strings.Contains(c, "dts-hd master"):
		return "DTS-HD MA"
	case strings.Contains(c, "dts-hd high resolution"):
		return "DTS-HD"
	case strings.Contains(c, "truehd"):
		return "TrueHD"
	case strings.Contains(c, "dolby digital plus"):
		return "EAC3"
	case strings.Contains(c, "dolby digital"):
		return "AC3"
	}

	// Fallback on Format + Format_Profile
	f := strings.ToUpper(format)
	p := strings.ToLower(profile)
	switch f {
	case "DTS":
		switch {
		case strings.Contains(p, "ma"):
			return "DTS-HD MA"
		case strings.Contains(p, "hra"):
			return "DTS-HD"
		case strings.Contains(p, "x"):
			return "DTS:X"
		default:
			return "DTS"
		}
	case "AC-3":
		return "AC3"
	case "E-AC-3":
		return "EAC3"
	case "TRUEHD":
		return "TrueHD"
	case "AAC":
		return "AAC"
	case "FLAC":
		return "FLAC"
	case "MP3", "MPEG AUDIO":
		return "MP3"
	case "OPUS":
		return "Opus"
	case "VORBIS":
		return "Vorbis"
	default:
		if strings.HasPrefix(strings.ToLower(format), "pcm") {
			return "PCM"
		}
		return strings.ToUpper(format)
	}
}

// ════════════════════════════════════════════════════════════════
//  ffprobe (fallback)
// ════════════════════════════════════════════════════════════════

type ffprobeOutput struct {
	Streams []ffprobeStream `json:"streams"`
	Format  ffprobeFormat   `json:"format"`
}

type ffprobeStream struct {
	CodecType      string            `json:"codec_type"`
	CodecName      string            `json:"codec_name"`
	Profile        string            `json:"profile"`
	Width          int               `json:"width"`
	Height         int               `json:"height"`
	RFrameRate     string            `json:"r_frame_rate"`
	ColorTransfer  string            `json:"color_transfer"`
	ColorPrimaries string            `json:"color_primaries"`
	Channels       int               `json:"channels"`
	SideDataList   []ffprobeSideData `json:"side_data_list"`
	Tags           map[string]string `json:"tags"`
	Disposition    map[string]int    `json:"disposition"`
}

type ffprobeSideData struct {
	SideDataType string `json:"side_data_type"`
}

type ffprobeFormat struct {
	Duration string `json:"duration"`
	Size     string `json:"size"`
	BitRate  string `json:"bit_rate"`
}

func getMediaInfoFFprobe(path string) (MediaInfo, error) {
	cmd := exec.Command("ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_streams",
		"-show_format",
		path,
	)
	out, err := cmd.Output()
	if err != nil {
		return MediaInfo{}, fmt.Errorf("ffprobe non disponible: %w", err)
	}

	var probe ffprobeOutput
	if err := json.Unmarshal(out, &probe); err != nil {
		return MediaInfo{}, fmt.Errorf("parsing ffprobe output: %w", err)
	}

	var mi MediaInfo

	if size, err := strconv.ParseInt(probe.Format.Size, 10, 64); err == nil {
		mi.FileSize = size
	}
	if br, err := strconv.ParseInt(probe.Format.BitRate, 10, 64); err == nil {
		mi.Bitrate = br
	}
	if dur, err := strconv.ParseFloat(probe.Format.Duration, 64); err == nil {
		h := int(dur) / 3600
		m := (int(dur) % 3600) / 60
		if h > 0 {
			mi.Duration = fmt.Sprintf("%dh %02dmin", h, m)
		} else {
			mi.Duration = fmt.Sprintf("%dmin", m)
		}
	}

	for _, s := range probe.Streams {
		if s.CodecType != "video" {
			continue
		}
		mi.Width = s.Width
		mi.Height = s.Height
		mi.VideoCodec = normalizeVideoCodecFFprobe(s.CodecName)
		mi.Resolution = detectResolution(s.Width, s.Height)
		if s.RFrameRate != "" {
			parts := strings.SplitN(s.RFrameRate, "/", 2)
			if len(parts) == 2 {
				num, e1 := strconv.ParseFloat(parts[0], 64)
				den, e2 := strconv.ParseFloat(parts[1], 64)
				if e1 == nil && e2 == nil && den > 0 {
					mi.FrameRate = math.Round(num/den*1000) / 1000
				}
			}
		}
		mi.HDRFormat = detectHDRFFprobe(s)
		break
	}

	var audioCodecs, audioLangs []string
	for _, s := range probe.Streams {
		if s.CodecType != "audio" {
			continue
		}
		if codec := normalizeAudioCodecFFprobe(s.CodecName); codec != "" && !sliceContains(audioCodecs, codec) {
			audioCodecs = append(audioCodecs, codec)
		}
		if lang := normalizeLanguage(s.Tags["language"]); lang != "" && !sliceContains(audioLangs, lang) {
			audioLangs = append(audioLangs, lang)
		}
	}
	if len(audioCodecs) > 0 {
		mi.AudioCodec = audioCodecs[0]
	}
	if len(audioLangs) > 0 {
		mi.AudioLanguages = strings.Join(audioLangs, ", ")
	}

	var subLangs []string
	for _, s := range probe.Streams {
		if s.CodecType != "subtitle" {
			continue
		}
		lang := normalizeLanguage(s.Tags["language"])
		if lang == "" {
			lang = "Inconnu"
		}
		if s.Disposition["forced"] == 1 || s.Tags["forced"] == "1" {
			lang += " (forcé)"
		}
		if !sliceContains(subLangs, lang) {
			subLangs = append(subLangs, lang)
		}
	}
	if len(subLangs) > 0 {
		mi.SubtitleLanguages = strings.Join(subLangs, ", ")
	}

	return mi, nil
}

func detectHDRFFprobe(s ffprobeStream) string {
	switch strings.ToLower(s.ColorTransfer) {
	case "smpte2084":
		for _, sd := range s.SideDataList {
			sdl := strings.ToLower(sd.SideDataType)
			if strings.Contains(sdl, "dolby") {
				return "DV"
			}
			if strings.Contains(sdl, "hdr dynamic") {
				return "HDR10+"
			}
		}
		return "HDR10"
	case "arib-std-b67":
		return "HLG"
	}
	for _, sd := range s.SideDataList {
		sdl := strings.ToLower(sd.SideDataType)
		if strings.Contains(sdl, "dolby") {
			return "DV"
		}
		if strings.Contains(sdl, "hdr dynamic") {
			return "HDR10+"
		}
	}
	return ""
}

func normalizeVideoCodecFFprobe(codec string) string {
	switch strings.ToLower(codec) {
	case "h264", "avc":
		return "H.264"
	case "h265", "hevc":
		return "H.265"
	case "av1":
		return "AV1"
	case "vp9":
		return "VP9"
	case "xvid", "divx", "mpeg4":
		return "XviD"
	case "mpeg2video":
		return "MPEG-2"
	default:
		return strings.ToUpper(codec)
	}
}

func normalizeAudioCodecFFprobe(codec string) string {
	switch strings.ToLower(codec) {
	case "dts":
		return "DTS"
	case "dtshd", "dts-hd":
		return "DTS-HD MA"
	case "truehd":
		return "TrueHD"
	case "ac3", "ac-3":
		return "AC3"
	case "eac3", "e-ac3", "ec-3":
		return "EAC3"
	case "aac":
		return "AAC"
	case "mp3":
		return "MP3"
	case "flac":
		return "FLAC"
	case "opus":
		return "Opus"
	default:
		if strings.HasPrefix(strings.ToLower(codec), "pcm") {
			return "PCM"
		}
		return strings.ToUpper(codec)
	}
}

// ════════════════════════════════════════════════════════════════
//  Helpers communs
// ════════════════════════════════════════════════════════════════

func detectResolution(width, height int) string {
	// La largeur est plus fiable que la hauteur : les rips BluRay croppent
	// souvent les bandes noires, donnant des hauteurs non-standard
	// (ex: 1280×696 = 720p cropé, 1920×800 = 1080p 2.40:1)
	switch {
	case width >= 3840 || height >= 2160:
		return "2160p"
	case width >= 1920 || height >= 1080:
		return "1080p"
	case width >= 1280 || height >= 720:
		return "720p"
	case width >= 854 || height >= 480:
		return "480p"
	default:
		return "SD"
	}
}

func normalizeLanguage(lang string) string {
	lang = strings.ToLower(strings.TrimSpace(lang))
	if lang == "" || lang == "und" {
		return ""
	}
	switch lang {
	case "fre", "fra", "fr":
		return "Français"
	case "eng", "en":
		return "Anglais"
	case "ger", "deu", "de":
		return "Allemand"
	case "spa", "es":
		return "Espagnol"
	case "ita", "it":
		return "Italien"
	case "jpn", "ja":
		return "Japonais"
	case "chi", "zho", "zh":
		return "Chinois"
	case "kor", "ko":
		return "Coréen"
	case "por", "pt":
		return "Portugais"
	case "rus", "ru":
		return "Russe"
	case "ara", "ar":
		return "Arabe"
	case "dut", "nld", "nl":
		return "Néerlandais"
	default:
		if len(lang) > 0 {
			return strings.ToUpper(lang[:1]) + lang[1:]
		}
		return lang
	}
}

func sliceContains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
