package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

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
