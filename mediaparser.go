package main

// Pure Go media parser for MKV and MP4 files.
// Zero external dependencies — reads only the header portion of the file.

import (
	"bytes"
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// parsedTrack holds raw track data extracted from the container
type parsedTrack struct {
	isVideo  bool
	isAudio  bool
	codecID  string // raw container codec ID (e.g. "V_MPEGH/ISO/HEVC", "avc1")
	width    uint32
	height   uint32
	channels uint32
	language string
	hdrType  string // "HDR10", "HDR", ""
}

// parseMediaFile detects format by extension and dispatches to the right parser
func parseMediaFile(filePath string) ([]parsedTrack, error) {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".mkv", ".webm", ".mk3d", ".mka":
		return parseMKV(filePath)
	case ".mp4", ".m4v", ".mov", ".m4a", ".m4b":
		return parseMP4(filePath)
	}
	return nil, errUnsupportedFormat
}

var errUnsupportedFormat = &parseError{"unsupported container format"}

type parseError struct{ msg string }

func (e *parseError) Error() string { return e.msg }

// ────────────────────────────────────────────────────────────────────
// EBML / MKV parser
// ────────────────────────────────────────────────────────────────────

const mkvReadLimit = 8 * 1024 * 1024 // 8 MB — always contains Tracks

// ebmlReadID reads a variable-length EBML element ID (keeps marker bits)
func ebmlReadID(r *bytes.Reader) (uint32, error) {
	b0, err := r.ReadByte()
	if err != nil {
		return 0, err
	}
	var width int
	switch {
	case b0&0x80 != 0:
		width = 1
	case b0&0x40 != 0:
		width = 2
	case b0&0x20 != 0:
		width = 3
	case b0&0x10 != 0:
		width = 4
	default:
		return 0, &parseError{"invalid EBML ID"}
	}
	id := uint32(b0)
	for i := 1; i < width; i++ {
		b, err := r.ReadByte()
		if err != nil {
			return 0, err
		}
		id = (id << 8) | uint32(b)
	}
	return id, nil
}

// ebmlReadSize reads a variable-length EBML data size (strips marker bit)
// Returns -1 for unknown/streaming size.
func ebmlReadSize(r *bytes.Reader) (int64, error) {
	b0, err := r.ReadByte()
	if err != nil {
		return 0, err
	}
	var width int
	var mask byte
	switch {
	case b0&0x80 != 0:
		width, mask = 1, 0x7F
	case b0&0x40 != 0:
		width, mask = 2, 0x3F
	case b0&0x20 != 0:
		width, mask = 3, 0x1F
	case b0&0x10 != 0:
		width, mask = 4, 0x0F
	case b0&0x08 != 0:
		width, mask = 5, 0x07
	case b0&0x04 != 0:
		width, mask = 6, 0x03
	case b0&0x02 != 0:
		width, mask = 7, 0x01
	case b0&0x01 != 0:
		width, mask = 8, 0x00
	default:
		return -1, nil
	}
	val := int64(b0 & mask)
	for i := 1; i < width; i++ {
		b, err := r.ReadByte()
		if err != nil {
			return 0, err
		}
		val = (val << 8) | int64(b)
	}
	// All-ones value = unknown size
	maxVal := int64(1<<(7*width) - 1)
	if val == maxVal {
		return -1, nil
	}
	return val, nil
}

// ebmlUint decodes a big-endian unsigned integer from a byte slice
func ebmlUint(b []byte) uint32 {
	var v uint32
	for _, x := range b {
		v = (v << 8) | uint32(x)
	}
	return v
}

const (
	ebmlIDEBML      = uint32(0x1A45DFA3)
	ebmlIDSegment   = uint32(0x18538067)
	ebmlIDTracks    = uint32(0x1654AE6B)
	ebmlIDTrackEntry = uint32(0xAE)
	ebmlIDTrackType = uint32(0x83)
	ebmlIDCodecID   = uint32(0x86)
	ebmlIDLanguage  = uint32(0x22B59C)
	ebmlIDLangBCP47 = uint32(0x55B4)
	ebmlIDVideo     = uint32(0xE0)
	ebmlIDPixelW    = uint32(0xB0)
	ebmlIDPixelH    = uint32(0xBA)
	ebmlIDColour    = uint32(0x55B0)
	ebmlIDTransfer  = uint32(0x55BA)
	ebmlIDAudio     = uint32(0xE1)
	ebmlIDChannels  = uint32(0x9F)
)

func parseMKV(filePath string) ([]parsedTrack, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	buf := make([]byte, mkvReadLimit)
	n, _ := io.ReadAtLeast(f, buf, 8)
	buf = buf[:n]

	// Verify EBML magic
	if len(buf) < 4 || buf[0] != 0x1A || buf[1] != 0x45 || buf[2] != 0xDF || buf[3] != 0xA3 {
		return nil, &parseError{"not an EBML/MKV file"}
	}

	// Find Tracks element by signature (16 54 AE 6B)
	tracksID := []byte{0x16, 0x54, 0xAE, 0x6B}
	idx := bytes.Index(buf, tracksID)
	if idx < 0 {
		return nil, &parseError{"Tracks element not found in first 8 MB"}
	}

	r := bytes.NewReader(buf[idx:])

	// Read and discard the Tracks ID (we already know what it is)
	if _, err := ebmlReadID(r); err != nil {
		return nil, err
	}
	tracksSize, err := ebmlReadSize(r)
	if err != nil {
		return nil, err
	}

	var tracksData []byte
	if tracksSize > 0 && tracksSize <= int64(r.Len()) {
		tracksData = make([]byte, tracksSize)
		if _, err := io.ReadFull(r, tracksData); err != nil {
			return nil, err
		}
	} else {
		// Unknown or oversized — just use the rest of the buffer
		rem := make([]byte, r.Len())
		io.ReadFull(r, rem)
		tracksData = rem
	}

	return ebmlParseTrackEntries(tracksData), nil
}

func ebmlParseTrackEntries(data []byte) []parsedTrack {
	var tracks []parsedTrack
	r := bytes.NewReader(data)
	for r.Len() > 2 {
		id, err := ebmlReadID(r)
		if err != nil {
			break
		}
		size, err := ebmlReadSize(r)
		if err != nil {
			break
		}
		if size < 0 || size > int64(r.Len()) {
			break
		}
		elem := make([]byte, size)
		if _, err := io.ReadFull(r, elem); err != nil {
			break
		}
		if id == ebmlIDTrackEntry {
			if t, ok := ebmlParseTrack(elem); ok {
				tracks = append(tracks, t)
			}
		}
	}
	return tracks
}

func ebmlParseTrack(data []byte) (parsedTrack, bool) {
	var t parsedTrack
	r := bytes.NewReader(data)
	for r.Len() > 2 {
		id, err := ebmlReadID(r)
		if err != nil {
			break
		}
		size, err := ebmlReadSize(r)
		if err != nil {
			break
		}
		if size < 0 || size > int64(r.Len()) {
			break
		}
		elem := make([]byte, size)
		if _, err := io.ReadFull(r, elem); err != nil {
			break
		}
		switch id {
		case ebmlIDTrackType:
			if len(elem) > 0 {
				t.isVideo = elem[0] == 1
				t.isAudio = elem[0] == 2
			}
		case ebmlIDCodecID:
			t.codecID = strings.TrimRight(string(elem), "\x00 ")
		case ebmlIDLanguage, ebmlIDLangBCP47:
			if t.language == "" {
				t.language = strings.TrimRight(string(elem), "\x00 ")
			}
		case ebmlIDVideo:
			ebmlParseVideo(elem, &t)
		case ebmlIDAudio:
			ebmlParseAudio(elem, &t)
		}
	}
	return t, t.isVideo || t.isAudio
}

func ebmlParseVideo(data []byte, t *parsedTrack) {
	r := bytes.NewReader(data)
	for r.Len() > 2 {
		id, err := ebmlReadID(r)
		if err != nil {
			break
		}
		size, err := ebmlReadSize(r)
		if err != nil {
			break
		}
		if size < 0 || size > int64(r.Len()) {
			break
		}
		elem := make([]byte, size)
		if _, err := io.ReadFull(r, elem); err != nil {
			break
		}
		switch id {
		case ebmlIDPixelW:
			t.width = ebmlUint(elem)
		case ebmlIDPixelH:
			t.height = ebmlUint(elem)
		case ebmlIDColour:
			ebmlParseColour(elem, t)
		}
	}
}

func ebmlParseColour(data []byte, t *parsedTrack) {
	r := bytes.NewReader(data)
	for r.Len() > 2 {
		id, err := ebmlReadID(r)
		if err != nil {
			break
		}
		size, err := ebmlReadSize(r)
		if err != nil {
			break
		}
		if size < 0 || size > int64(r.Len()) {
			break
		}
		elem := make([]byte, size)
		if _, err := io.ReadFull(r, elem); err != nil {
			break
		}
		if id == ebmlIDTransfer {
			val := ebmlUint(elem)
			switch val {
			case 16: // SMPTE ST 2084 PQ
				t.hdrType = "HDR10"
			case 18: // HLG (Arib STD-B67)
				t.hdrType = "HDR"
			}
		}
	}
}

func ebmlParseAudio(data []byte, t *parsedTrack) {
	r := bytes.NewReader(data)
	for r.Len() > 2 {
		id, err := ebmlReadID(r)
		if err != nil {
			break
		}
		size, err := ebmlReadSize(r)
		if err != nil {
			break
		}
		if size < 0 || size > int64(r.Len()) {
			break
		}
		elem := make([]byte, size)
		if _, err := io.ReadFull(r, elem); err != nil {
			break
		}
		if id == ebmlIDChannels {
			t.channels = ebmlUint(elem)
		}
	}
}

// Codec ID conversions for MKV
func mkvVideoCodec(id string) string {
	switch id {
	case "V_MPEG4/ISO/AVC":
		return "H.264"
	case "V_MPEGH/ISO/HEVC":
		return "H.265"
	case "V_AV1":
		return "AV1"
	case "V_VP9":
		return "VP9"
	case "V_VP8":
		return "VP8"
	case "V_MS/VFW/FOURCC":
		return "XviD"
	}
	return ""
}

func mkvAudioCodec(id string, channels uint32) string {
	switch id {
	case "A_TRUEHD":
		if channels >= 8 {
			return "Atmos"
		}
		return "TrueHD"
	case "A_AC3":
		return "AC3"
	case "A_EAC3":
		return "EAC3"
	case "A_DTS":
		// DTS subtype (MA/HD) requires reading the bitstream; report DTS
		return "DTS"
	case "A_FLAC":
		return "FLAC"
	case "A_AAC", "A_AAC/MPEG2/LC", "A_AAC/MPEG4/LC", "A_AAC/MPEG4/LC/SBR":
		return "AAC"
	case "A_MPEG/L3":
		return "MP3"
	case "A_OPUS":
		return "Opus"
	case "A_VORBIS":
		return "Vorbis"
	}
	return ""
}

// ────────────────────────────────────────────────────────────────────
// MP4 / MOV / M4V parser
// ────────────────────────────────────────────────────────────────────

const mp4ReadLimit = 32 * 1024 * 1024 // 32 MB — moov is usually near the start

type mp4Atom struct {
	atomType string
	data     []byte
}

// mp4ReadAtoms parses a flat list of atoms from a byte slice
func mp4ReadAtoms(data []byte) []mp4Atom {
	var atoms []mp4Atom
	for len(data) >= 8 {
		size := int(binary.BigEndian.Uint32(data[:4]))
		atype := string(data[4:8])
		headerSize := 8

		switch size {
		case 0:
			size = len(data) // extends to end of file
		case 1:
			if len(data) < 16 {
				return atoms
			}
			size64 := binary.BigEndian.Uint64(data[8:16])
			if size64 > uint64(len(data)) {
				return atoms
			}
			size = int(size64)
			headerSize = 16
		default:
			if size < 8 || size > len(data) {
				return atoms
			}
		}

		atoms = append(atoms, mp4Atom{
			atomType: atype,
			data:     data[headerSize:size],
		})
		data = data[size:]
	}
	return atoms
}

// mp4FindAtom finds the first atom of the given type in a slice
func mp4FindAtom(atoms []mp4Atom, atomType string) *mp4Atom {
	for i := range atoms {
		if atoms[i].atomType == atomType {
			return &atoms[i]
		}
	}
	return nil
}

func parseMP4(filePath string) ([]parsedTrack, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	fi, _ := f.Stat()
	fileSize := fi.Size()

	// Read up to 32 MB from start
	limit := int64(mp4ReadLimit)
	if fileSize < limit {
		limit = fileSize
	}
	buf := make([]byte, limit)
	n, _ := io.ReadAtLeast(f, buf, 8)
	buf = buf[:n]

	topAtoms := mp4ReadAtoms(buf)
	moov := mp4FindAtom(topAtoms, "moov")

	// If not found in first 32 MB, try the last 8 MB (moov-at-end files)
	if moov == nil && int64(limit) < fileSize {
		seekPos := fileSize - 8*1024*1024
		if seekPos < 0 {
			seekPos = 0
		}
		f.Seek(seekPos, io.SeekStart)
		tail := make([]byte, 8*1024*1024)
		tn, _ := f.Read(tail)
		topAtoms = mp4ReadAtoms(tail[:tn])
		moov = mp4FindAtom(topAtoms, "moov")
	}

	if moov == nil {
		return nil, &parseError{"moov atom not found"}
	}

	return mp4ParseMoov(moov.data), nil
}

func mp4ParseMoov(data []byte) []parsedTrack {
	var tracks []parsedTrack
	atoms := mp4ReadAtoms(data)
	for _, a := range atoms {
		if a.atomType == "trak" {
			if t, ok := mp4ParseTrak(a.data); ok {
				tracks = append(tracks, t)
			}
		}
	}
	return tracks
}

func mp4ParseTrak(data []byte) (parsedTrack, bool) {
	var t parsedTrack
	atoms := mp4ReadAtoms(data)

	// tkhd — track header (width/height for video)
	if tkhd := mp4FindAtom(atoms, "tkhd"); tkhd != nil {
		mp4ParseTkhd(tkhd.data, &t)
	}

	// mdia — media container
	mdia := mp4FindAtom(atoms, "mdia")
	if mdia == nil {
		return t, false
	}
	mdiaAtoms := mp4ReadAtoms(mdia.data)

	// hdlr — handler type: 'vide' or 'soun'
	hdlr := mp4FindAtom(mdiaAtoms, "hdlr")
	if hdlr != nil && len(hdlr.data) >= 12 {
		handler := string(hdlr.data[8:12])
		t.isVideo = handler == "vide"
		t.isAudio = handler == "soun"
	}

	// mdhd — language
	mdhd := mp4FindAtom(mdiaAtoms, "mdhd")
	if mdhd != nil {
		mp4ParseMdhd(mdhd.data, &t)
	}

	// minf → stbl → stsd
	minf := mp4FindAtom(mdiaAtoms, "minf")
	if minf != nil {
		minfAtoms := mp4ReadAtoms(minf.data)
		stbl := mp4FindAtom(minfAtoms, "stbl")
		if stbl != nil {
			stblAtoms := mp4ReadAtoms(stbl.data)
			stsd := mp4FindAtom(stblAtoms, "stsd")
			if stsd != nil && len(stsd.data) >= 8 {
				// stsd: 4 bytes version+flags, 4 bytes entry count, then entries
				mp4ParseStsd(stsd.data[8:], &t)
			}
		}
	}

	return t, t.isVideo || t.isAudio
}

func mp4ParseTkhd(data []byte, t *parsedTrack) {
	// tkhd version 0: 4+4+8+4+8+8+4+4+4+4+4+4+4+4+9*4 bytes
	// Width is at offset 76, height at 80 (both 16.16 fixed-point)
	// Version 1: add 8 bytes for timestamps — width at 84, height at 88
	if len(data) < 4 {
		return
	}
	version := data[0]
	var widthOff, heightOff int
	if version == 0 {
		widthOff, heightOff = 76, 80
	} else {
		widthOff, heightOff = 84, 88
	}
	if len(data) > heightOff+4 {
		w := binary.BigEndian.Uint32(data[widthOff:])
		h := binary.BigEndian.Uint32(data[heightOff:])
		t.width = w >> 16  // integer part of 16.16 fixed-point
		t.height = h >> 16
	}
}

func mp4ParseMdhd(data []byte, t *parsedTrack) {
	// mdhd: version(1)+flags(3)... language at offset 20 (v0) or 28 (v1)
	if len(data) < 4 {
		return
	}
	var langOff int
	if data[0] == 0 {
		langOff = 20
	} else {
		langOff = 28
	}
	if len(data) >= langOff+2 {
		// Language packed as 3×5-bit chars (ISO-639-2/T)
		packed := binary.BigEndian.Uint16(data[langOff:])
		if packed != 0 && packed != 0x7FFF {
			lang := [3]byte{
				byte((packed>>10)&0x1F) + 0x60,
				byte((packed>>5)&0x1F) + 0x60,
				byte(packed&0x1F) + 0x60,
			}
			t.language = string(lang[:])
		}
	}
}

func mp4ParseStsd(data []byte, t *parsedTrack) {
	// Each entry: 4 bytes size, 4 bytes codec type, ...
	if len(data) < 8 {
		return
	}
	codec := string(data[4:8])
	t.codecID = codec

	// Check for Dolby Vision or HDR10 inside video sample entries
	if t.isVideo {
		mp4CheckVideoHDR(data, codec, t)
	}
}

func mp4CheckVideoHDR(data []byte, codec string, t *parsedTrack) {
	// Look for 'colr' box inside the sample entry (after the base 70 bytes)
	if len(data) > 86 {
		inner := mp4ReadAtoms(data[86:])
		for _, a := range inner {
			if a.atomType == "colr" && len(a.data) >= 4 {
				colorType := string(a.data[:4])
				if colorType == "nclx" && len(a.data) >= 10 {
					transferChar := binary.BigEndian.Uint16(a.data[4:6])
					// 16 = SMPTE ST 2084 (HDR10)
					// 18 = HLG
					switch transferChar {
					case 16:
						t.hdrType = "HDR10"
					case 18:
						t.hdrType = "HDR"
					}
				}
			}
			// 'dvvC' or 'dvcC' = Dolby Vision
			if a.atomType == "dvvC" || a.atomType == "dvcC" {
				if t.hdrType != "" {
					t.hdrType = "HDR DV"
				} else {
					t.hdrType = "DV"
				}
			}
		}
	}
}

// MP4 codec ID → normalized name
func mp4VideoCodec(id string) string {
	switch id {
	case "avc1", "avc2", "avc3":
		return "H.264"
	case "hev1", "hvc1", "dvhe", "dvh1":
		return "H.265"
	case "av01":
		return "AV1"
	case "vp09":
		return "VP9"
	case "vp08":
		return "VP8"
	case "mp4v", "xvid", "divx":
		return "XviD"
	}
	return ""
}

func mp4AudioCodec(id string, channels uint32) string {
	switch id {
	case "ac-3", "ac3 ":
		return "AC3"
	case "ec-3", "eac3":
		return "EAC3"
	case "dtsc", "dtsh", "dtsl", "dtse", "dtsx":
		return "DTS"
	case "mlpa":
		if channels >= 8 {
			return "Atmos"
		}
		return "TrueHD"
	case "mp4a":
		return "AAC"
	case "fLaC":
		return "FLAC"
	case ".mp3", "mp3 ":
		return "MP3"
	case "Opus", "opus":
		return "Opus"
	}
	return ""
}

// ────────────────────────────────────────────────────────────────────
// Convert []parsedTrack → MediaInfo
// ────────────────────────────────────────────────────────────────────

func tracksToMediaInfo(tracks []parsedTrack, filePath string) MediaInfo {
	info := MediaInfo{}

	// Find first video track
	for _, t := range tracks {
		if t.isVideo && info.Width == 0 {
			info.Width = int(t.width)
			info.Height = int(t.height)
			info.Resolution = detectResolution(int(t.width), int(t.height))
			info.HDRFormat = t.hdrType

			ext := strings.ToLower(filepath.Ext(filePath))
			if ext == ".mkv" || strings.HasPrefix(ext, ".mk") {
				info.VideoCodec = mkvVideoCodec(t.codecID)
			} else {
				info.VideoCodec = mp4VideoCodec(t.codecID)
			}
			if info.VideoCodec == "" {
				info.VideoCodec = t.codecID
			}
		}
	}

	// Audio tracks — find best codec and collect languages
	type audioCandidate struct {
		codec    string
		priority int
		lang     string
	}
	audioPriority := map[string]int{
		"Atmos": 11, "TrueHD": 10, "DTS-HD MA": 9, "DTS-HD": 8,
		"DTS": 7, "EAC3": 6, "FLAC": 5, "AC3": 4, "AAC": 3, "MP3": 2,
	}
	var best audioCandidate
	seen := map[string]bool{}
	var langList []string

	ext := strings.ToLower(filepath.Ext(filePath))
	isMKV := ext == ".mkv" || strings.HasPrefix(ext, ".mk")

	for _, t := range tracks {
		if !t.isAudio {
			continue
		}
		var codec string
		if isMKV {
			codec = mkvAudioCodec(t.codecID, t.channels)
		} else {
			codec = mp4AudioCodec(t.codecID, t.channels)
		}
		if codec == "" {
			codec = t.codecID
		}

		prio := audioPriority[codec]
		if prio > best.priority {
			best.priority = prio
			best.codec = codec
		}

		// Language
		lang := ""
		if t.language != "" && t.language != "und" {
			lang = normalizeLangCode(t.language)
		}
		if lang != "" && !seen[lang] {
			seen[lang] = true
			langList = append(langList, lang)
		}
	}

	info.AudioCodec = best.codec
	if len(langList) > 0 {
		info.AudioLanguages = strings.Join(langList, ", ")
	}

	info.Source = guessSource(filePath)

	// File size
	if fi, err := os.Stat(filePath); err == nil {
		info.FileSize = fi.Size()
	}

	return info
}

func normalizeLangCode(code string) string {
	code = strings.ToLower(strings.TrimSpace(code))
	if name, ok := langNames[code]; ok && name != "" {
		return name
	}
	return strings.ToUpper(code)
}
