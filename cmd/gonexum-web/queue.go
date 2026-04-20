package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type QueueStatus string

const (
	QueuePending QueueStatus = "pending"
	QueueRunning QueueStatus = "running"
	QueueDone    QueueStatus = "done"
	QueueError   QueueStatus = "error"
)

// QueueItem is one entry in the processing queue
type QueueItem struct {
	ID          string      `json:"id"`
	SourcePath  string      `json:"sourcePath"`
	ReleaseName string      `json:"releaseName"`
	CategoryID  int         `json:"categoryId"`
	NoUpload    bool        `json:"noUpload"`
	TMDBId      int         `json:"tmdbId"`
	TMDBType    string      `json:"tmdbType"`
	Status      QueueStatus `json:"status"`
	Log         []string    `json:"log"`
	Progress    float64     `json:"progress"`
	ErrorMsg    string      `json:"error,omitempty"`
	AddedAt     time.Time   `json:"addedAt"`
	// result
	UploadURL string `json:"uploadUrl,omitempty"`
	TorrentID int    `json:"torrentId,omitempty"`
	InfoHash  string `json:"infoHash,omitempty"`
	NFOPath   string `json:"nfoPath,omitempty"`
}

// AppQueue manages the sequential processing queue
type AppQueue struct {
	mu   sync.RWMutex
	list []*QueueItem
	work chan *QueueItem
	subs sync.Map // chan []byte → true
}

var globalQueue = &AppQueue{
	work: make(chan *QueueItem, 1024),
}

func init() {
	go globalQueue.worker()
}

func newQueueID() string {
	return fmt.Sprintf("q%d%04d", time.Now().UnixNano(), rand.Intn(9999))
}

func (q *AppQueue) Add(item *QueueItem) {
	item.ID = newQueueID()
	item.Status = QueuePending
	item.AddedAt = time.Now()
	if item.TMDBType == "" {
		item.TMDBType = "movie"
	}
	if item.CategoryID == 0 {
		item.CategoryID = 1
	}
	q.mu.Lock()
	q.list = append(q.list, item)
	q.mu.Unlock()
	q.work <- item
	q.broadcast()
}

func (q *AppQueue) Remove(id string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	for i, it := range q.list {
		if it.ID == id && it.Status == QueuePending {
			q.list = append(q.list[:i], q.list[i+1:]...)
			q.broadcastLocked()
			return true
		}
	}
	return false
}

func (q *AppQueue) ClearDone() {
	q.mu.Lock()
	var kept []*QueueItem
	for _, it := range q.list {
		if it.Status == QueuePending || it.Status == QueueRunning {
			kept = append(kept, it)
		}
	}
	q.list = kept
	q.mu.Unlock()
	q.broadcast()
}

func (q *AppQueue) List() []*QueueItem {
	q.mu.RLock()
	defer q.mu.RUnlock()
	out := make([]*QueueItem, len(q.list))
	copy(out, q.list)
	return out
}

func (q *AppQueue) Subscribe() chan []byte {
	ch := make(chan []byte, 64)
	q.subs.Store(ch, true)
	return ch
}

func (q *AppQueue) Unsubscribe(ch chan []byte) {
	q.subs.Delete(ch)
	close(ch)
}

func (q *AppQueue) broadcast() {
	q.mu.RLock()
	defer q.mu.RUnlock()
	q.broadcastLocked()
}

func (q *AppQueue) broadcastLocked() {
	data, _ := json.Marshal(q.list)
	q.subs.Range(func(k, _ interface{}) bool {
		ch := k.(chan []byte)
		select {
		case ch <- data:
		default:
		}
		return true
	})
}

func (q *AppQueue) update(id string, fn func(*QueueItem)) {
	q.mu.Lock()
	for _, it := range q.list {
		if it.ID == id {
			fn(it)
			break
		}
	}
	q.mu.Unlock()
	q.broadcast()
}

func (q *AppQueue) log(id, msg string) {
	q.update(id, func(it *QueueItem) {
		it.Log = append(it.Log, msg)
	})
}

func (q *AppQueue) worker() {
	for item := range q.work {
		// Skip if removed or no longer pending
		q.mu.RLock()
		found := false
		for _, it := range q.list {
			if it.ID == item.ID && it.Status == QueuePending {
				found = true
				break
			}
		}
		q.mu.RUnlock()
		if !found {
			continue
		}
		q.processItem(item)
	}
}

func (q *AppQueue) processItem(item *QueueItem) {
	q.update(item.ID, func(it *QueueItem) {
		it.Status = QueueRunning
		it.Log = nil
		it.Progress = 0
	})

	fail := func(msg string) {
		q.update(item.ID, func(it *QueueItem) {
			it.Status = QueueError
			it.ErrorMsg = msg
		})
	}

	settings, err := loadSettings()
	if err != nil {
		fail("Impossible de charger les settings: " + err.Error())
		return
	}
	if settings.OutputDir == "" {
		settings.OutputDir = filepath.Dir(item.SourcePath)
	}

	// ── 1. Release name ────────────────────────────────────────────────
	releaseName := item.ReleaseName
	if releaseName == "" {
		releaseName = filepath.Base(item.SourcePath)
		if ext := filepath.Ext(releaseName); ext != "" {
			releaseName = strings.TrimSuffix(releaseName, ext)
		}
	}

	// ── Duplicate check ─────────────────────────────────────────────────
	if !item.NoUpload {
		if dup, err := checkDuplicate(releaseName, settings); err == nil && dup.Found {
			fail(fmt.Sprintf("Doublon détecté sur nexum : %s (ID #%d)", dup.Name, dup.ID))
			return
		}
	}

	// ── 2. Media info ──────────────────────────────────────────────────
	q.log(item.ID, "Extraction des informations média...")
	var mediaInfo MediaInfo
	var mediaInfoCLI string
	if videoPath, err := largestVideoFile(item.SourcePath); err == nil {
		if mi, cliText, _, miErr := getMediaInfo(videoPath); miErr == nil {
			mediaInfo = mi
			mediaInfoCLI = cliText
		}
	}
	if mediaInfo.Source == "" {
		mediaInfo.Source = detectSourceFromName(releaseName)
	}

	// ── 3. TMDB ────────────────────────────────────────────────────────
	var tmdbDetails TMDBDetails
	if item.TMDBId > 0 {
		q.log(item.ID, "Récupération des détails TMDB...")
		if d, err := getTMDBDetails(item.TMDBId, item.TMDBType); err == nil {
			tmdbDetails = d
			q.log(item.ID, "TMDB: "+tmdbDetails.Title+" ("+tmdbDetails.Year+")")
		}
	} else {
		q.log(item.ID, "Recherche TMDB pour «"+releaseName+"»...")
		if results, err := searchTMDB(releaseName, item.TMDBType); err == nil && len(results) > 0 {
			r := results[0]
			if d, err := getTMDBDetails(r.ID, r.MediaType); err == nil {
				tmdbDetails = d
				q.update(item.ID, func(it *QueueItem) {
					it.TMDBId = r.ID
					it.TMDBType = r.MediaType
					it.Log = append(it.Log, "TMDB: "+tmdbDetails.Title+" ("+tmdbDetails.Year+")")
				})
			}
		} else {
			q.log(item.ID, "TMDB: aucun résultat")
		}
	}

	// ── 4. NFO ─────────────────────────────────────────────────────────
	nfoContent := generateNFO(tmdbDetails, mediaInfo, mediaInfoCLI, settings)
	q.log(item.ID, "NFO généré")

	// ── 5. Torrent ─────────────────────────────────────────────────────
	q.log(item.ID, "Création du torrent...")
	torrentResult, err := createTorrent(item.SourcePath, settings, func(p TorrentProgress) {
		if p.Phase == "hashing" {
			q.update(item.ID, func(it *QueueItem) { it.Progress = p.Percent })
		}
	})
	if err != nil {
		fail("Erreur torrent: " + err.Error())
		return
	}
	q.log(item.ID, "Torrent créé: "+filepath.Base(torrentResult.FilePath))

	// Save NFO
	nfoPath := strings.TrimSuffix(torrentResult.FilePath, ".torrent") + ".nfo"
	_ = os.WriteFile(nfoPath, []byte(nfoContent), 0644)

	if item.NoUpload {
		q.update(item.ID, func(it *QueueItem) {
			it.Status = QueueDone
			it.Progress = 100
			it.InfoHash = torrentResult.InfoHash
			it.NFOPath = nfoPath
		})
		return
	}

	// ── 6. Upload ──────────────────────────────────────────────────────
	q.log(item.ID, "Upload vers nexum-core.com...")
	uploadResult, err := uploadTorrent(UploadParams{
		TorrentPath:       torrentResult.FilePath,
		NFOContent:        nfoContent,
		Name:              torrentResult.Name,
		CategoryID:        item.CategoryID,
		Description:       bbcodeOrOverview(releaseName, mediaInfoCLI, tmdbDetails.Overview),
		TMDBId:            item.TMDBId,
		TMDBType:          item.TMDBType,
		Resolution:        mediaInfo.Resolution,
		VideoCodec:        mediaInfo.VideoCodec,
		AudioCodec:        mediaInfo.AudioCodec,
		AudioLanguages:    mediaInfo.AudioLanguages,
		SubtitleLanguages: mediaInfo.SubtitleLanguages,
		HDRFormat:         mediaInfo.HDRFormat,
		Source:            mediaInfo.Source,
	}, settings)
	if err != nil {
		fail("Erreur upload: " + err.Error())
		return
	}

	q.log(item.ID, fmt.Sprintf("Upload OK — ID %d", uploadResult.TorrentID))
	q.update(item.ID, func(it *QueueItem) {
		it.Status = QueueDone
		it.Progress = 100
		it.UploadURL = uploadResult.URL
		it.TorrentID = uploadResult.TorrentID
		it.InfoHash = torrentResult.InfoHash
		it.NFOPath = nfoPath
	})
}
