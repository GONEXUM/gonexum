package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// GET /api/browse?path=/some/dir
func handleBrowse(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", 405)
		return
	}

	homeDir, _ := os.UserHomeDir()
	if homeDir == "" {
		homeDir = "/"
	}

	reqPath := r.URL.Query().Get("path")
	if reqPath == "" {
		reqPath = homeDir
	}

	// Clean and absolute
	reqPath = filepath.Clean(reqPath)

	// Clamp to home directory — prevent browsing above it
	sep := string(filepath.Separator)
	if reqPath != homeDir && !strings.HasPrefix(reqPath, homeDir+sep) {
		reqPath = homeDir
	}

	fi, err := os.Stat(reqPath)
	if err != nil {
		jsonErr(w, 400, "Chemin invalide: "+err.Error())
		return
	}
	if !fi.IsDir() {
		jsonErr(w, 400, "Le chemin n'est pas un dossier")
		return
	}

	entries, err := os.ReadDir(reqPath)
	if err != nil {
		jsonErr(w, 500, "Impossible de lire le dossier: "+err.Error())
		return
	}

	type Entry struct {
		Name  string `json:"name"`
		Path  string `json:"path"`
		IsDir bool   `json:"isDir"`
		Size  int64  `json:"size"`
		Ext   string `json:"ext"`
	}

	var dirs, files []Entry
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue // skip hidden
		}
		fullPath := filepath.Join(reqPath, e.Name())
		info, err := e.Info()
		if err != nil {
			continue
		}
		entry := Entry{
			Name:  e.Name(),
			Path:  fullPath,
			IsDir: e.IsDir(),
			Size:  info.Size(),
			Ext:   strings.ToLower(filepath.Ext(e.Name())),
		}
		if e.IsDir() {
			dirs = append(dirs, entry)
		} else {
			files = append(files, entry)
		}
	}

	sort.Slice(dirs, func(i, j int) bool { return dirs[i].Name < dirs[j].Name })
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })

	parent := ""
	if reqPath != homeDir {
		p := filepath.Dir(reqPath)
		// Only allow going up within home directory
		if p == homeDir || strings.HasPrefix(p, homeDir+sep) {
			parent = p
		} else {
			parent = homeDir
		}
	}

	jsonOK(w, map[string]interface{}{
		"path":    reqPath,
		"parent":  parent,
		"entries": append(dirs, files...),
	})
}

// POST /api/nfo/validate
// Body: {"template": "..."}
func handleNFOValidate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}
	var req struct {
		Template string `json:"template"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, 400, "JSON invalide")
		return
	}
	_, err := renderCustomNFO(req.Template, NFOTemplateData{
		TMDB:  TMDBDetails{Genres: []string{}},
		Media: MediaInfo{},
	})
	if err != nil {
		jsonOK(w, map[string]interface{}{"valid": false, "error": err.Error()})
		return
	}
	jsonOK(w, map[string]interface{}{"valid": true})
}

// POST /api/nfo/preview
// Body: {"template": "..."} — empty template uses settings default
func handleNFOPreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}
	var req struct {
		Template string `json:"template"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, 400, "JSON invalide")
		return
	}

	demoData := NFOTemplateData{
		TMDB: TMDBDetails{
			ID: 550, Title: "Fight Club", Year: "1999",
			Director: "David Fincher",
			Genres:   []string{"Drame", "Thriller"},
			Overview: "Un cadre insomniaque et un savonnier charismatique fondent un club de combat clandestin qui évolue en quelque chose de bien plus inquiétant.",
			Rating:   8.4, Runtime: 139, MediaType: "movie",
		},
		Media: MediaInfo{
			Resolution: "1080p", VideoCodec: "H.264", AudioCodec: "DTS",
			AudioLanguages: "Français, Anglais", Source: "BluRay",
			Duration: "2h19m", FrameRate: 23.976,
		},
		MediaInfoCLI: "General\nFormat                   : Matroska\nDuration                 : 2 h 19 min\n\nVideo\nFormat                   : AVC\nWidth                    : 1 920 pixels\nHeight                   : 800 pixels\n\nAudio\nFormat                   : DTS\nChannel(s)               : 6 channels\n",
	}

	var content string
	if req.Template != "" {
		var err error
		content, err = renderCustomNFO(req.Template, demoData)
		if err != nil {
			jsonErr(w, 400, err.Error())
			return
		}
	} else {
		s, _ := loadSettings()
		content = generateNFO(demoData.TMDB, demoData.Media, demoData.MediaInfoCLI, s)
	}

	jsonOK(w, map[string]string{"content": content})
}

// GET /api/version — current version + update check
func handleVersion(w http.ResponseWriter, r *http.Request) {
	info := map[string]interface{}{
		"current":   AppVersion,
		"latest":    "",
		"available": false,
		"url":       "",
	}
	resp, err := http.Get("https://api.github.com/repos/diabolino/gonexum-releases/releases/latest")
	if err == nil {
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err == nil {
			var release struct {
				TagName string `json:"tag_name"`
				HTMLURL string `json:"html_url"`
			}
			if json.Unmarshal(body, &release) == nil {
				latest := strings.TrimPrefix(release.TagName, "v")
				current := strings.TrimPrefix(AppVersion, "v")
				info["latest"] = latest
				info["url"] = release.HTMLURL
				info["available"] = latest != "" && latest != current && current != "dev"
			}
		}
	}
	jsonOK(w, info)
}

// ── Queue handlers ───────────────────────────────────────────────────────────

// GET  /api/queue       → list items
// POST /api/queue       → add item
func handleQueue(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		items := globalQueue.List()
		if items == nil {
			items = []*QueueItem{}
		}
		jsonOK(w, items)

	case http.MethodPost:
		var item QueueItem
		if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
			jsonErr(w, 400, "JSON invalide: "+err.Error())
			return
		}
		if item.SourcePath == "" {
			jsonErr(w, 400, "sourcePath requis")
			return
		}
		globalQueue.Add(&item)
		jsonOK(w, map[string]string{"id": item.ID})

	default:
		http.Error(w, "Method not allowed", 405)
	}
}

// POST /api/queue/remove  {"id":"..."}
func handleQueueRemove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		jsonErr(w, 400, "id requis")
		return
	}
	if globalQueue.Remove(req.ID) {
		jsonOK(w, map[string]string{"status": "ok"})
	} else {
		jsonErr(w, 404, "item introuvable ou déjà en cours")
	}
}

// POST /api/queue/clear — remove done/error items
func handleQueueClear(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}
	globalQueue.ClearDone()
	jsonOK(w, map[string]string{"status": "ok"})
}

// GET /api/queue/events — SSE stream of queue state
func handleQueueEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming non supporté", 500)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	ch := globalQueue.Subscribe()
	defer globalQueue.Unsubscribe(ch)

	// Send current state immediately
	items := globalQueue.List()
	if items == nil {
		items = []*QueueItem{}
	}
	initial, _ := json.Marshal(items)
	fmt.Fprintf(w, "data: %s\n\n", initial)
	flusher.Flush()

	ctx := r.Context()
	keepalive := time.NewTicker(20 * time.Second)
	defer keepalive.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-keepalive.C:
			fmt.Fprintf(w, ": keepalive\n\n")
			flusher.Flush()
		case data, more := <-ch:
			if !more {
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

var fallbackCategories = []map[string]interface{}{
	{"id": 1, "name": "Films", "slug": "films"},
	{"id": 2, "name": "Séries", "slug": "series"},
	{"id": 3, "name": "Documentaires", "slug": "documentaires"},
	{"id": 4, "name": "Animés", "slug": "animes"},
}

// GET /api/categories — proxies nexum /api/v1/categories with fallback
func handleCategories(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", 405)
		return
	}
	s, _ := loadSettings()
	if s.TrackerURL == "" {
		s.TrackerURL = "https://nexum-core.com"
	}
	catURL := s.TrackerURL + "/api/v1/categories"
	if s.APIKey != "" {
		catURL += "?apikey=" + s.APIKey
	}
	req, err := http.NewRequest("GET", catURL, nil)
	if err != nil {
		jsonOK(w, map[string]interface{}{"categories": fallbackCategories})
		return
	}
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != 200 {
		jsonOK(w, map[string]interface{}{"categories": fallbackCategories})
		return
	}
	defer resp.Body.Close()
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		jsonOK(w, map[string]interface{}{"categories": fallbackCategories})
		return
	}
	jsonOK(w, result)
}

// POST /api/nfo/bbcode — generate BBCode description from mediainfo
func handleNFOBBCode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}
	var req struct {
		ReleaseName  string `json:"releaseName"`
		MediaInfoCLI string `json:"mediaInfoCLI"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, 400, "JSON invalide")
		return
	}
	content := generateBBCodeDescription(req.ReleaseName, req.MediaInfoCLI)
	jsonOK(w, map[string]string{"content": content})
}

func jsonOK(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func jsonErr(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// GET /api/settings  → Settings
// POST /api/settings ← Settings
func handleSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s, err := loadSettings()
		if err != nil {
			jsonErr(w, 500, err.Error())
			return
		}
		jsonOK(w, s)

	case http.MethodPost:
		var s Settings
		if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
			jsonErr(w, 400, "JSON invalide: "+err.Error())
			return
		}
		if s.TrackerURL == "" {
			s.TrackerURL = "https://nexum-core.com"
		}
		if err := saveSettings(s); err != nil {
			jsonErr(w, 500, err.Error())
			return
		}
		jsonOK(w, map[string]string{"status": "ok"})

	default:
		http.Error(w, "Method not allowed", 405)
	}
}

// POST /api/mediainfo
// Body: {"path": "/data/Fight.Club.mkv"}
func handleMediaInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}
	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, 400, "JSON invalide")
		return
	}
	req.Path = strings.TrimSpace(req.Path)
	if req.Path == "" {
		jsonErr(w, 400, "path requis")
		return
	}

	videoPath, err := largestVideoFile(req.Path)
	if err != nil {
		jsonErr(w, 400, err.Error())
		return
	}

	// Auto-detect source from release name
	releaseName := filepath.Base(req.Path)
	ext := filepath.Ext(releaseName)
	if ext != "" {
		releaseName = releaseName[:len(releaseName)-len(ext)]
	}
	autoSource := detectSourceFromName(releaseName)

	mi, cliText, detected, miErr := getMediaInfo(videoPath)
	if autoSource != "" && mi.Source == "" {
		mi.Source = autoSource
	}

	if miErr != nil {
		jsonOK(w, map[string]interface{}{
			"mediaInfo":   mi,
			"cliText":     "",
			"detected":    false,
			"autoSource":  autoSource,
			"releaseName": releaseName,
			"warning":     miErr.Error(),
		})
		return
	}

	jsonOK(w, map[string]interface{}{
		"mediaInfo":   mi,
		"cliText":     cliText,
		"detected":    detected,
		"autoSource":  autoSource,
		"releaseName": releaseName,
	})
}

// POST /api/tmdb/search
// Body: {"query": "Fight Club", "type": "movie"}
func handleTMDBSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}
	var req struct {
		Query string `json:"query"`
		Type  string `json:"type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, 400, "JSON invalide")
		return
	}
	results, err := searchTMDB(req.Query, req.Type)
	if err != nil {
		jsonErr(w, 500, err.Error())
		return
	}
	if results == nil {
		results = []TMDBResult{}
	}
	jsonOK(w, results)
}

// POST /api/tmdb/details
// Body: {"id": 550, "type": "movie"}
func handleTMDBDetails(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}
	var req struct {
		ID   int    `json:"id"`
		Type string `json:"type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, 400, "JSON invalide")
		return
	}
	details, err := getTMDBDetails(req.ID, req.Type)
	if err != nil {
		jsonErr(w, 500, err.Error())
		return
	}
	jsonOK(w, details)
}

// POST /api/process → {"jobId": "..."}
func handleProcess(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}
	var req ProcessRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, 400, "JSON invalide: "+err.Error())
		return
	}
	if req.SourcePath == "" {
		jsonErr(w, 400, "sourcePath requis")
		return
	}

	job := newJob()
	go runJob(job, req)

	jsonOK(w, map[string]string{"jobId": job.ID})
}

// GET /api/events?jobId=xxx  (Server-Sent Events)
func handleEvents(w http.ResponseWriter, r *http.Request) {
	jobID := r.URL.Query().Get("jobId")
	job, ok := getJob(jobID)
	if !ok {
		http.Error(w, "Job introuvable", 404)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming non supporté", 500)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	ctx := r.Context()
	keepalive := time.NewTicker(20 * time.Second)
	defer keepalive.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-keepalive.C:
			fmt.Fprintf(w, ": keepalive\n\n")
			flusher.Flush()

		case evt, more := <-job.Events:
			if !more {
				// Channel closed — job finished
				fmt.Fprintf(w, "event: close\ndata: {}\n\n")
				flusher.Flush()
				return
			}
			data, _ := json.Marshal(evt)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
			if evt.Type == "done" || evt.Type == "error" {
				return
			}
		}
	}
}
