package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// DuplicateCheckResult is returned by checkDuplicate
type DuplicateCheckResult struct {
	Found bool   `json:"found"`
	ID    int    `json:"id,omitempty"`
	Name  string `json:"name,omitempty"`
	URL   string `json:"url,omitempty"`
}

// checkDuplicate queries nexum /api/v1/torrents?q=... to detect if a release
// with the same name already exists.
func checkDuplicate(releaseName string, settings Settings) (DuplicateCheckResult, error) {
	if settings.APIKey == "" || settings.TrackerURL == "" || releaseName == "" {
		return DuplicateCheckResult{}, nil
	}
	q := normalizeName(releaseName)

	params := url.Values{}
	params.Set("q", q)
	params.Set("apikey", settings.APIKey)

	endpoint := settings.TrackerURL + "/api/v1/torrents?" + params.Encode()
	resp, err := http.Get(endpoint)
	if err != nil {
		return DuplicateCheckResult{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return DuplicateCheckResult{}, err
	}
	if resp.StatusCode != 200 {
		return DuplicateCheckResult{}, fmt.Errorf("API %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Total    int `json:"total"`
		Torrents []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"torrents"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return DuplicateCheckResult{}, err
	}
	if result.Total == 0 || len(result.Torrents) == 0 {
		return DuplicateCheckResult{}, nil
	}
	// L'API fait du fuzzy match — on vérifie l'égalité stricte
	// après normalisation pour éviter les faux positifs (ex: dates différentes).
	for _, t := range result.Torrents {
		if normalizeName(t.Name) == q {
			return DuplicateCheckResult{
				Found: true,
				ID:    t.ID,
				Name:  t.Name,
				URL:   fmt.Sprintf("%s/torrents/%d", settings.TrackerURL, t.ID),
			}, nil
		}
	}
	return DuplicateCheckResult{}, nil
}
