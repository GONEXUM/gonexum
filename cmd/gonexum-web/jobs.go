package main

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// JobEvent is sent to the SSE client
type JobEvent struct {
	Type string      `json:"type"` // "progress", "log", "done", "error"
	Data interface{} `json:"data"`
}

// DoneData is the payload of the "done" event
type DoneData struct {
	TorrentPath string `json:"torrentPath"`
	NFOPath     string `json:"nfoPath"`
	InfoHash    string `json:"infoHash"`
	Name        string `json:"name"`
	Size        int64  `json:"size"`
	UploadURL   string `json:"uploadUrl,omitempty"`
	TorrentID   int    `json:"torrentId,omitempty"`
	NoUpload    bool   `json:"noUpload"`
}

// Job represents a running pipeline job
type Job struct {
	ID     string
	Events chan JobEvent
}

var jobRegistry sync.Map

func newJob() *Job {
	id := fmt.Sprintf("%d%04d", time.Now().UnixNano(), rand.Intn(9999))
	j := &Job{
		ID:     id,
		Events: make(chan JobEvent, 256),
	}
	jobRegistry.Store(id, j)
	return j
}

func getJob(id string) (*Job, bool) {
	v, ok := jobRegistry.Load(id)
	if !ok {
		return nil, false
	}
	return v.(*Job), true
}

func (j *Job) emit(typ string, data interface{}) {
	select {
	case j.Events <- JobEvent{Type: typ, Data: data}:
	default:
	}
}

func (j *Job) log(msg string) {
	j.emit("log", map[string]string{"message": msg})
}

func (j *Job) finish() {
	close(j.Events)
	go func() {
		time.Sleep(10 * time.Minute)
		jobRegistry.Delete(j.ID)
	}()
}

// ProcessRequest is sent by the client to /api/process
type ProcessRequest struct {
	SourcePath  string    `json:"sourcePath"`
	ReleaseName string    `json:"releaseName"`
	MediaInfo   MediaInfo `json:"mediaInfo"`
	MediaInfoCLI string   `json:"mediaInfoCLI"`
	TMDBId      int       `json:"tmdbId"`
	TMDBType    string    `json:"tmdbType"`
	CategoryID  int       `json:"categoryId"`
	NoUpload    bool      `json:"noUpload"`
}

func runJob(job *Job, req ProcessRequest) {
	defer job.finish()

	settings, err := loadSettings()
	if err != nil {
		job.emit("error", map[string]string{"message": "Impossible de charger les settings: " + err.Error()})
		return
	}
	if settings.OutputDir == "" {
		settings.OutputDir = filepath.Dir(req.SourcePath)
	}

	// TMDB details (already fetched client-side, but we re-fetch for the NFO)
	var tmdbDetails TMDBDetails
	if req.TMDBId > 0 {
		job.log("Récupération des détails TMDB...")
		tmdbDetails, err = getTMDBDetails(req.TMDBId, req.TMDBType)
		if err != nil {
			job.log("Avertissement TMDB: " + err.Error())
		}
	}

	// Generate NFO
	nfoContent := generateNFO(tmdbDetails, req.MediaInfo, req.MediaInfoCLI, settings)
	job.log("NFO généré")

	// Create torrent
	job.log("Création du torrent en cours...")
	torrentResult, err := createTorrent(req.SourcePath, settings, func(p TorrentProgress) {
		job.emit("progress", p)
	})
	if err != nil {
		job.emit("error", map[string]string{"message": "Erreur création torrent: " + err.Error()})
		return
	}
	job.log(fmt.Sprintf("Torrent créé: %s", torrentResult.FilePath))

	// Save NFO next to the torrent
	nfoPath := strings.TrimSuffix(torrentResult.FilePath, ".torrent") + ".nfo"
	if err := os.WriteFile(nfoPath, []byte(nfoContent), 0644); err != nil {
		job.log("Avertissement: impossible de sauvegarder le NFO: " + err.Error())
		nfoPath = ""
	} else {
		job.log(fmt.Sprintf("NFO sauvegardé: %s", nfoPath))
	}

	if req.NoUpload {
		job.emit("done", DoneData{
			TorrentPath: torrentResult.FilePath,
			NFOPath:     nfoPath,
			InfoHash:    torrentResult.InfoHash,
			Name:        torrentResult.Name,
			Size:        torrentResult.Size,
			NoUpload:    true,
		})
		return
	}

	// Upload
	job.log("Upload vers nexum-core.com...")
	uploadResult, err := uploadTorrent(UploadParams{
		TorrentPath:       torrentResult.FilePath,
		NFOContent:        nfoContent,
		Name:              torrentResult.Name,
		CategoryID:        req.CategoryID,
		Description:       bbcodeOrOverview(req.ReleaseName, req.MediaInfoCLI, tmdbDetails.Overview),
		TMDBId:            req.TMDBId,
		TMDBType:          req.TMDBType,
		Resolution:        req.MediaInfo.Resolution,
		VideoCodec:        req.MediaInfo.VideoCodec,
		AudioCodec:        req.MediaInfo.AudioCodec,
		AudioLanguages:    req.MediaInfo.AudioLanguages,
		SubtitleLanguages: req.MediaInfo.SubtitleLanguages,
		HDRFormat:         req.MediaInfo.HDRFormat,
		Source:            req.MediaInfo.Source,
	}, settings)
	if err != nil {
		job.emit("error", map[string]string{"message": "Erreur upload: " + err.Error()})
		return
	}

	job.emit("done", DoneData{
		TorrentPath: torrentResult.FilePath,
		NFOPath:     nfoPath,
		InfoHash:    torrentResult.InfoHash,
		Name:        torrentResult.Name,
		Size:        torrentResult.Size,
		UploadURL:   uploadResult.URL,
		TorrentID:   uploadResult.TorrentID,
		NoUpload:    false,
	})
}
