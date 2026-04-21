package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// HistoryEntry represents one upload event in the local history database.
type HistoryEntry struct {
	ID           int64     `json:"id"`
	CreatedAt    time.Time `json:"createdAt"`
	SourcePath   string    `json:"sourcePath"`
	ReleaseName  string    `json:"releaseName"`
	TorrentPath  string    `json:"torrentPath"`
	NFOPath      string    `json:"nfoPath"`
	InfoHash     string    `json:"infoHash"`
	Size         int64     `json:"size"`
	CategoryID   int       `json:"categoryId"`
	CategoryName string    `json:"categoryName"`
	TMDBId       int       `json:"tmdbId"`
	TMDBType     string    `json:"tmdbType"`
	TMDBTitle    string    `json:"tmdbTitle"`
	UploadURL    string    `json:"uploadUrl"`
	UploadID     int       `json:"uploadId"`
	Status       string    `json:"status"` // "done", "error"
	ErrorMsg     string    `json:"errorMsg"`
	NoUpload     bool      `json:"noUpload"`
}

var (
	historyDB     *sql.DB
	historyDBOnce sync.Once
	historyDBErr  error
)

// historyDBPath returns the path to the local history.db (alongside settings.json).
func historyDBPath() (string, error) {
	p, err := settingsPath()
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(p)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	return filepath.Join(dir, "history.db"), nil
}

// getHistoryDB returns a lazily-initialized singleton connection to the history DB.
func getHistoryDB() (*sql.DB, error) {
	historyDBOnce.Do(func() {
		path, err := historyDBPath()
		if err != nil {
			historyDBErr = err
			return
		}
		db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
		if err != nil {
			historyDBErr = fmt.Errorf("open history.db: %w", err)
			return
		}
		if _, err := db.Exec(`
			CREATE TABLE IF NOT EXISTS uploads (
				id             INTEGER PRIMARY KEY AUTOINCREMENT,
				created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
				source_path    TEXT NOT NULL,
				release_name   TEXT NOT NULL,
				torrent_path   TEXT,
				nfo_path       TEXT,
				info_hash      TEXT,
				size           INTEGER,
				category_id    INTEGER,
				category_name  TEXT,
				tmdb_id        INTEGER,
				tmdb_type      TEXT,
				tmdb_title     TEXT,
				upload_url     TEXT,
				upload_id      INTEGER,
				status         TEXT NOT NULL,
				error_msg      TEXT,
				no_upload      INTEGER DEFAULT 0
			);
			CREATE INDEX IF NOT EXISTS idx_uploads_created_at ON uploads(created_at DESC);
			CREATE INDEX IF NOT EXISTS idx_uploads_release_name ON uploads(release_name);
		`); err != nil {
			historyDBErr = fmt.Errorf("init schema: %w", err)
			return
		}
		historyDB = db
	})
	return historyDB, historyDBErr
}

// saveHistory inserts a new entry. Errors are non-fatal (logged caller-side).
func saveHistory(e HistoryEntry) error {
	db, err := getHistoryDB()
	if err != nil {
		return err
	}
	_, err = db.Exec(`
		INSERT INTO uploads (
			source_path, release_name, torrent_path, nfo_path, info_hash, size,
			category_id, category_name, tmdb_id, tmdb_type, tmdb_title,
			upload_url, upload_id, status, error_msg, no_upload
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		e.SourcePath, e.ReleaseName, e.TorrentPath, e.NFOPath, e.InfoHash, e.Size,
		e.CategoryID, e.CategoryName, e.TMDBId, e.TMDBType, e.TMDBTitle,
		e.UploadURL, e.UploadID, e.Status, e.ErrorMsg, e.NoUpload,
	)
	return err
}

// listHistory returns entries sorted by most recent first, optional text search.
func listHistory(limit, offset int, search string) ([]HistoryEntry, error) {
	db, err := getHistoryDB()
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	query := `SELECT id, created_at, source_path, release_name, torrent_path, nfo_path,
		info_hash, size, category_id, category_name, tmdb_id, tmdb_type, tmdb_title,
		upload_url, upload_id, status, error_msg, no_upload
		FROM uploads`
	args := []interface{}{}
	if search != "" {
		query += ` WHERE release_name LIKE ? OR tmdb_title LIKE ?`
		s := "%" + search + "%"
		args = append(args, s, s)
	}
	query += ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []HistoryEntry
	for rows.Next() {
		var e HistoryEntry
		var noUp int
		if err := rows.Scan(&e.ID, &e.CreatedAt, &e.SourcePath, &e.ReleaseName,
			&e.TorrentPath, &e.NFOPath, &e.InfoHash, &e.Size, &e.CategoryID,
			&e.CategoryName, &e.TMDBId, &e.TMDBType, &e.TMDBTitle,
			&e.UploadURL, &e.UploadID, &e.Status, &e.ErrorMsg, &noUp); err != nil {
			return nil, err
		}
		e.NoUpload = noUp != 0
		out = append(out, e)
	}
	return out, rows.Err()
}

// countHistory returns the total number of entries (for pagination).
func countHistory(search string) (int, error) {
	db, err := getHistoryDB()
	if err != nil {
		return 0, err
	}
	var n int
	if search != "" {
		s := "%" + search + "%"
		err = db.QueryRow(`SELECT COUNT(*) FROM uploads WHERE release_name LIKE ? OR tmdb_title LIKE ?`, s, s).Scan(&n)
	} else {
		err = db.QueryRow(`SELECT COUNT(*) FROM uploads`).Scan(&n)
	}
	return n, err
}

func deleteHistory(id int64) error {
	db, err := getHistoryDB()
	if err != nil {
		return err
	}
	_, err = db.Exec(`DELETE FROM uploads WHERE id = ?`, id)
	return err
}

func clearHistory() error {
	db, err := getHistoryDB()
	if err != nil {
		return err
	}
	_, err = db.Exec(`DELETE FROM uploads`)
	return err
}
