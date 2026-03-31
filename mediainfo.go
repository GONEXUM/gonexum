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

// largestVideoFile returns the path of the largest video file under a directory.
func largestVideoFile(dir string) string {
	fi, err := os.Stat(dir)
	if err != nil || !fi.IsDir() {
		return dir
	}
	var best string
	var bestSize int64
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if videoExts[strings.ToLower(filepath.Ext(path))] && info.Size() > bestSize {
			bestSize = info.Size()
			best = path
		}
		return nil
	})
	return best
}

// GetMediaInfo extracts media info from a video file or the largest video in a directory.
// Uses the pure Go parser first (no dependencies).
// Falls back to ffprobe if the Go parser fails or the format is not supported.
func (a *App) GetMediaInfo(filePath string) (MediaInfo, error) {
	// If path is a directory, find the largest video file inside
	if fi, err := os.Stat(filePath); err == nil && fi.IsDir() {
		filePath = largestVideoFile(filePath)
		if filePath == "" {
			return MediaInfo{}, fmt.Errorf("aucun fichier vidéo trouvé dans le dossier")
		}
	}

	// Try pure Go parser first
	tracks, err := parseMediaFile(filePath)
	if err == nil && len(tracks) > 0 {
		info := tracksToMediaInfo(tracks, filePath)
		return info, nil
	}

	// Fall back to ffprobe
	info, ffErr := getMediaInfoFFprobe(filePath)
	if ffErr != nil {
		if err != nil {
			return MediaInfo{}, fmt.Errorf("go parser: %v | ffprobe: %v", err, ffErr)
		}
		return MediaInfo{}, fmt.Errorf("ffprobe non disponible (%v). Installez ffmpeg ou renseignez les infos manuellement.", ffErr)
	}
	return info, nil
}

// ─── ffprobe fallback ────────────────────────────────────────────────────────

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
	Tags           map[string]string `json:"tags"`
	SideDataList   []ffprobeSideData `json:"side_data_list"`
	Channels       int               `json:"channels"`
}

type ffprobeSideData struct {
	SideDataType string `json:"side_data_type"`
}

type ffprobeFormat struct {
	Duration string `json:"duration"`
	Size     string `json:"size"`
	BitRate  string `json:"bit_rate"`
}

// ffprobePath returns the path to ffprobe: bundled next to the executable first, then PATH
func ffprobePath() string {
	exe, err := os.Executable()
	if err == nil {
		dir := filepath.Dir(exe)
		// macOS .app bundle: Contents/MacOS/ffprobe
		candidates := []string{
			filepath.Join(dir, "ffprobe"),
			filepath.Join(dir, "ffprobe.exe"),
		}
		for _, p := range candidates {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}
	return "ffprobe" // fallback to PATH
}

func getMediaInfoFFprobe(filePath string) (MediaInfo, error) {
	cmd := exec.Command(ffprobePath(),
		"-v", "quiet",
		"-print_format", "json",
		"-show_streams",
		"-show_format",
		filePath,
	)
	out, err := cmd.Output()
	if err != nil {
		return MediaInfo{}, fmt.Errorf("ffprobe: %w", err)
	}
	var probe ffprobeOutput
	if err := json.Unmarshal(out, &probe); err != nil {
		return MediaInfo{}, fmt.Errorf("parse ffprobe output: %w", err)
	}
	return parseFFprobeOutput(probe, filePath), nil
}

func parseFFprobeOutput(probe ffprobeOutput, filePath string) MediaInfo {
	info := MediaInfo{}

	if probe.Format.Duration != "" {
		if d, err := strconv.ParseFloat(probe.Format.Duration, 64); err == nil {
			h := int(d) / 3600
			m := (int(d) % 3600) / 60
			s := int(d) % 60
			info.Duration = fmt.Sprintf("%02d:%02d:%02d", h, m, s)
		}
	}
	if probe.Format.Size != "" {
		if sz, err := strconv.ParseInt(probe.Format.Size, 10, 64); err == nil {
			info.FileSize = sz
		}
	}
	if probe.Format.BitRate != "" {
		if br, err := strconv.ParseInt(probe.Format.BitRate, 10, 64); err == nil {
			info.Bitrate = br
		}
	}

	var audioStreams []ffprobeStream
	for _, s := range probe.Streams {
		switch s.CodecType {
		case "video":
			if info.Width == 0 {
				info.Width = s.Width
				info.Height = s.Height
				info.Resolution = detectResolution(s.Width, s.Height)
				info.VideoCodec = normalizeVideoCodec(s.CodecName, s.Profile)
				info.HDRFormat = detectHDRFFprobe(s)
				if s.RFrameRate != "" {
					info.FrameRate = parseFrameRate(s.RFrameRate)
				}
			}
		case "audio":
			audioStreams = append(audioStreams, s)
		}
	}

	if len(audioStreams) > 0 {
		info.AudioCodec = bestFFprobeAudioCodec(audioStreams)
		info.AudioLanguages = collectFFprobeLanguages(audioStreams)
	}
	info.Source = guessSource(filePath)
	return info
}

// ─── shared helpers ──────────────────────────────────────────────────────────

func detectResolution(w, h int) string {
	switch {
	case h >= 2160 || w >= 3840:
		return "2160p"
	case h >= 1080 || w >= 1920:
		return "1080p"
	case h >= 720 || w >= 1280:
		return "720p"
	default:
		if w > 0 || h > 0 {
			return "SD"
		}
		return ""
	}
}

func normalizeVideoCodec(codec, profile string) string {
	switch strings.ToLower(codec) {
	case "hevc", "h265":
		return "H.265"
	case "h264", "avc":
		return "H.264"
	case "av1":
		return "AV1"
	case "vp9":
		return "VP9"
	case "mpeg4", "xvid", "divx":
		return "XviD"
	default:
		return strings.ToUpper(codec)
	}
}

func detectHDRFFprobe(s ffprobeStream) string {
	ct := strings.ToLower(s.ColorTransfer)
	isDV := false
	isHDR10Plus := false
	for _, sd := range s.SideDataList {
		sdl := strings.ToLower(sd.SideDataType)
		if strings.Contains(sdl, "dolby") {
			isDV = true
		}
		if strings.Contains(sdl, "hdr10+") || strings.Contains(sdl, "hdr10 plus") {
			isHDR10Plus = true
		}
	}
	if isDV && (ct == "smpte2084" || isHDR10Plus) {
		return "HDR DV"
	}
	if isDV {
		return "DV"
	}
	if isHDR10Plus {
		return "HDR10+"
	}
	if ct == "smpte2084" {
		return "HDR10"
	}
	if ct == "arib-std-b67" {
		return "HDR"
	}
	return ""
}

func bestFFprobeAudioCodec(streams []ffprobeStream) string {
	priority := map[string]int{
		"truehd": 10, "eac3": 8, "dts": 7, "flac": 6,
		"ac3": 5, "aac": 4, "mp3": 3,
	}
	best := ""
	bestPrio := -1
	for _, s := range streams {
		codec := strings.ToLower(s.CodecName)
		prio := priority[codec]
		if prio > bestPrio {
			bestPrio = prio
			best = s.CodecName
			if codec == "truehd" && s.Channels >= 8 {
				best = "truehd_atmos"
			}
			if codec == "dts" {
				switch strings.ToUpper(s.Profile) {
				case "DTS-HD MA":
					best = "dts_hd_ma"
				case "DTS-HD":
					best = "dts_hd"
				}
			}
		}
	}
	return normalizeAudioCodec(best)
}

func normalizeAudioCodec(codec string) string {
	switch strings.ToLower(codec) {
	case "truehd_atmos":
		return "Atmos"
	case "truehd":
		return "TrueHD"
	case "dts_hd_ma":
		return "DTS-HD MA"
	case "dts_hd":
		return "DTS-HD"
	case "dts":
		return "DTS"
	case "eac3":
		return "EAC3"
	case "ac3":
		return "AC3"
	case "flac":
		return "FLAC"
	case "aac":
		return "AAC"
	case "mp3":
		return "MP3"
	default:
		return strings.ToUpper(codec)
	}
}

var langNames = map[string]string{
	"fre": "Français (France)", "fra": "Français (France)", "fr": "Français (France)",
	"eng": "Anglais", "en": "Anglais",
	"spa": "Espagnol", "es": "Espagnol",
	"ger": "Allemand", "deu": "Allemand", "de": "Allemand",
	"ita": "Italien", "it": "Italien",
	"por": "Portugais", "pt": "Portugais",
	"jpn": "Japonais", "ja": "Japonais",
	"chi": "Chinois", "zho": "Chinois", "zh": "Chinois",
	"kor": "Coréen", "ko": "Coréen",
	"ara": "Arabe", "ar": "Arabe",
	"rus": "Russe", "ru": "Russe",
	"und": "",
}

func collectFFprobeLanguages(streams []ffprobeStream) string {
	seen := map[string]bool{}
	var langs []string
	for _, s := range streams {
		lang := ""
		if s.Tags != nil {
			lang = s.Tags["language"]
		}
		name := normalizeLangCode(lang)
		if name != "" && !seen[name] {
			seen[name] = true
			langs = append(langs, name)
		}
	}
	return strings.Join(langs, ", ")
}

func guessSource(filePath string) string {
	name := strings.ToLower(filepath.Base(filePath))
	switch {
	case strings.Contains(name, "bluray") || strings.Contains(name, "blu-ray") ||
		strings.Contains(name, "bdremux") || strings.Contains(name, "bdrip"):
		return "BluRay"
	case strings.Contains(name, "web-dl") || strings.Contains(name, "webdl"):
		return "WEB-DL"
	case strings.Contains(name, "webrip"):
		return "WEBRip"
	case strings.Contains(name, "hdtv"):
		return "HDTV"
	case strings.Contains(name, "dvdrip") || strings.Contains(name, "dvd"):
		return "DVDRip"
	case strings.Contains(name, "dcp"):
		return "DCP"
	}
	return ""
}

func parseFrameRate(s string) float64 {
	parts := strings.Split(s, "/")
	if len(parts) != 2 {
		return 0
	}
	num, err1 := strconv.ParseFloat(parts[0], 64)
	den, err2 := strconv.ParseFloat(parts[1], 64)
	if err1 != nil || err2 != nil || den == 0 {
		return 0
	}
	return math.Round(num/den*100) / 100
}
