package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// AppVersion is set at build time via -ldflags "-X main.AppVersion=x.x.x"
var AppVersion = "dev"

// App struct
type App struct {
	ctx context.Context
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// SelectFile opens a native file dialog and returns the selected path
func (a *App) SelectFile(title string, filterName string, filterPattern string) (string, error) {
	filters := []runtime.FileFilter{}
	if filterName != "" && filterPattern != "" {
		filters = append(filters, runtime.FileFilter{
			DisplayName: filterName,
			Pattern:     filterPattern,
		})
	}
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:   title,
		Filters: filters,
	})
	return path, err
}

// SelectDirectory opens a native directory dialog and returns the selected path
func (a *App) SelectDirectory(title string) (string, error) {
	path, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: title,
	})
	return path, err
}

// GetFileSize returns the size of a file in bytes
func (a *App) GetFileSize(path string) (int64, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return fi.Size(), nil
}

// ReadFileChunk reads a chunk of a file and returns it as base64-encoded string
func (a *App) ReadFileChunk(path string, offset int64, size int64) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := f.Seek(offset, 0); err != nil {
		return "", err
	}
	buf := make([]byte, size)
	n, err := f.Read(buf)
	if err != nil && err.Error() != "EOF" {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(buf[:n]), nil
}

// ReadTextFile reads a text file and returns its content (max 1 MB)
func (a *App) ReadTextFile(path string) (string, error) {
	const maxSize = 1 * 1024 * 1024
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	data := make([]byte, maxSize)
	n, err := f.Read(data)
	if err != nil && err.Error() != "EOF" {
		return "", err
	}
	return string(data[:n]), nil
}

// AppLoadSettings loads and returns the current settings
func (a *App) AppLoadSettings() (Settings, error) {
	return loadSettings()
}

// AppSaveSettings saves the provided settings
func (a *App) AppSaveSettings(s Settings) error {
	return saveSettings(s)
}

// UpdateInfo holds the result of the update check
type UpdateInfo struct {
	Current   string `json:"current"`
	Latest    string `json:"latest"`
	Available bool   `json:"available"`
	URL       string `json:"url"`
}

// CheckUpdate queries the GitHub Releases API and returns update info
func (a *App) CheckUpdate() UpdateInfo {
	info := UpdateInfo{Current: AppVersion}

	resp, err := http.Get("https://api.github.com/repos/diabolino/gonexum-releases/releases/latest")
	if err != nil {
		return info
	}
	defer resp.Body.Close()

	var release struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return info
	}

	latest := strings.TrimPrefix(release.TagName, "v")
	current := strings.TrimPrefix(AppVersion, "v")
	info.Latest = latest
	info.URL = release.HTMLURL
	info.Available = latest != "" && latest != current

	return info
}

// GetAppVersion returns the current app version
func (a *App) GetAppVersion() string {
	return "v" + strings.TrimPrefix(AppVersion, "v")
}
