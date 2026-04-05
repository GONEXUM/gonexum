package main

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/anacrolix/torrent/bencode"
)

type torrentInfo struct {
	Name        string        `bencode:"name"`
	PieceLength int64         `bencode:"piece length"`
	Pieces      []byte        `bencode:"pieces"`
	Length      int64         `bencode:"length,omitempty"`
	Files       []torrentFile `bencode:"files,omitempty"`
	Private     *bool         `bencode:"private,omitempty"`
}

type torrentFile struct {
	Length int64    `bencode:"length"`
	Path   []string `bencode:"path"`
}

type torrentMetaInfo struct {
	Announce     string     `bencode:"announce"`
	AnnounceList [][]string `bencode:"announce-list,omitempty"`
	Info         torrentInfo `bencode:"info"`
	Comment      string     `bencode:"comment,omitempty"`
	CreatedBy    string     `bencode:"created by,omitempty"`
	CreationDate int64      `bencode:"creation date,omitempty"`
}

type fileEntry struct {
	absPath string
	relPath string
	size    int64
}

// TorrentProgress is reported during hashing
type TorrentProgress struct {
	Phase       string  `json:"phase"`
	Percent     float64 `json:"percent"`
	BytesDone   int64   `json:"bytesDone"`
	TotalBytes  int64   `json:"totalBytes"`
	CurrentFile string  `json:"currentFile"`
}

func choosePieceLength(totalSize int64) int64 {
	switch {
	case totalSize < 512*1024*1024:
		return 256 * 1024
	case totalSize < 2*1024*1024*1024:
		return 512 * 1024
	case totalSize < 8*1024*1024*1024:
		return 1024 * 1024
	default:
		return 2 * 1024 * 1024
	}
}

func collectFiles(root string) ([]fileEntry, int64, error) {
	var entries []fileEntry
	var total int64

	err := filepath.Walk(root, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if fi.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		entries = append(entries, fileEntry{
			absPath: path,
			relPath: rel,
			size:    fi.Size(),
		})
		total += fi.Size()
		return nil
	})
	if err != nil {
		return nil, 0, err
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].relPath < entries[j].relPath
	})
	return entries, total, nil
}

func hashPieces(files []fileEntry, pieceLength int64, onProgress func(done int64, file string)) ([]byte, error) {
	var pieces []byte
	hasher := sha1.New()
	var accumulated int64
	var bytesDone int64
	var lastReported int64

	const reportEvery = 32 * 1024 * 1024

	for _, f := range files {
		fh, err := os.Open(f.absPath)
		if err != nil {
			return nil, err
		}

		buf := make([]byte, 256*1024)
		for {
			n, readErr := fh.Read(buf)
			if n > 0 {
				data := buf[:n]
				bytesDone += int64(n)
				for len(data) > 0 {
					remaining := pieceLength - accumulated
					if int64(len(data)) < remaining {
						hasher.Write(data)
						accumulated += int64(len(data))
						data = nil
					} else {
						hasher.Write(data[:remaining])
						data = data[remaining:]
						pieces = append(pieces, hasher.Sum(nil)...)
						hasher.Reset()
						accumulated = 0
					}
				}
				if onProgress != nil && bytesDone-lastReported >= reportEvery {
					lastReported = bytesDone
					onProgress(bytesDone, f.relPath)
				}
			}
			if readErr == io.EOF {
				break
			}
			if readErr != nil {
				fh.Close()
				return nil, readErr
			}
		}
		fh.Close()
	}

	if accumulated > 0 {
		pieces = append(pieces, hasher.Sum(nil)...)
	}
	return pieces, nil
}

// createTorrent creates a .torrent file from a file or directory.
// onProgress is called periodically during hashing; may be nil.
func createTorrent(sourcePath string, settings Settings, onProgress func(TorrentProgress)) (TorrentResult, error) {
	fi, err := os.Stat(sourcePath)
	if err != nil {
		return TorrentResult{}, fmt.Errorf("cannot stat source: %w", err)
	}

	baseName := filepath.Base(sourcePath)
	info := torrentInfo{Name: baseName}

	var files []fileEntry
	var totalSize int64

	if fi.IsDir() {
		files, totalSize, err = collectFiles(sourcePath)
		if err != nil {
			return TorrentResult{}, fmt.Errorf("failed to collect files: %w", err)
		}
		for _, f := range files {
			parts := strings.Split(filepath.ToSlash(f.relPath), "/")
			info.Files = append(info.Files, torrentFile{
				Length: f.size,
				Path:   parts,
			})
		}
	} else {
		totalSize = fi.Size()
		info.Length = totalSize
		files = []fileEntry{{absPath: sourcePath, relPath: fi.Name(), size: totalSize}}
	}

	info.PieceLength = choosePieceLength(totalSize)

	if onProgress != nil {
		onProgress(TorrentProgress{Phase: "start", Percent: 0, TotalBytes: totalSize})
	}

	emit := func(done int64, file string) {
		if onProgress != nil {
			pct := 0.0
			if totalSize > 0 {
				pct = float64(done) / float64(totalSize) * 100
			}
			onProgress(TorrentProgress{
				Phase:       "hashing",
				Percent:     pct,
				BytesDone:   done,
				TotalBytes:  totalSize,
				CurrentFile: file,
			})
		}
	}

	pieces, err := hashPieces(files, info.PieceLength, emit)
	if err != nil {
		return TorrentResult{}, fmt.Errorf("failed to hash pieces: %w", err)
	}

	if onProgress != nil {
		onProgress(TorrentProgress{Phase: "writing", Percent: 100, TotalBytes: totalSize})
	}

	info.Pieces = pieces
	private := true
	info.Private = &private

	announceURL := fmt.Sprintf("%s/announce?passkey=%s", settings.TrackerURL, settings.Passkey)
	mi := torrentMetaInfo{
		Announce:     announceURL,
		AnnounceList: [][]string{{announceURL}},
		Info:         info,
		CreatedBy:    "GONEXUM",
		CreationDate: 0,
	}

	infoBytes, err := bencode.Marshal(info)
	if err != nil {
		return TorrentResult{}, fmt.Errorf("failed to encode info: %w", err)
	}
	h := sha1.Sum(infoBytes)
	infoHash := hex.EncodeToString(h[:])

	outputDir := settings.OutputDir
	if outputDir == "" {
		outputDir = os.TempDir()
	}
	torrentBase := strings.TrimSuffix(info.Name, filepath.Ext(info.Name))
	outputPath := filepath.Join(outputDir, torrentBase+".torrent")

	f, err := os.Create(outputPath)
	if err != nil {
		return TorrentResult{}, fmt.Errorf("failed to create torrent file: %w", err)
	}
	defer f.Close()

	if err := bencode.NewEncoder(f).Encode(mi); err != nil {
		return TorrentResult{}, fmt.Errorf("failed to write torrent: %w", err)
	}

	displayName := info.Name
	if !fi.IsDir() {
		displayName = strings.TrimSuffix(info.Name, filepath.Ext(info.Name))
	}

	return TorrentResult{
		FilePath: outputPath,
		InfoHash: infoHash,
		Name:     displayName,
		Size:     totalSize,
	}, nil
}
