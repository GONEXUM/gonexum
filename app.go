package main

import (
	"context"
	"os"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

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
