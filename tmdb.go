package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const nexumTMDBBase = "<TMDB_PROXY_URL>"

// nexumSearchResult matches the API response from <TMDB_PROXY>
// Genres can be either a space-separated string or a JSON array of strings
type nexumSearchResult struct {
	Title         string          `json:"title"`
	Years         string          `json:"years"`
	EnglishTitle  string          `json:"english_title"`
	OriginalTitle string          `json:"original_title"`
	Poster        string          `json:"poster"`
	PosterPath    string          `json:"poster_path"`
	Genres        json.RawMessage `json:"genres"`
	Countries     json.RawMessage `json:"countries"`
	Runtime       string          `json:"runtime"`
	ImdbID        string          `json:"imdb_id"`
	ImdbURL       string          `json:"imdb_url"`
	NoteImdb      float64         `json:"note_imdb"`
	VoteImdb      int             `json:"vote_imdb"`
	TmdbID        string          `json:"tmdb_id"`
	TmdbURL       string          `json:"tmdb_url"`
	ApiURL        string          `json:"api_url"`
	NoteTmdb      float64         `json:"note_tmdb"`
	VoteTmdb      int             `json:"vote_tmdb"`
	Tagline       string          `json:"tagline"`
	Overview      string          `json:"overview"`
}

// nexusDetailResult matches the raw TMDB format returned by ?t=movie&q={id} or ?t=tv&q={id}
type nexusDetailResult struct {
	ID          int     `json:"id"`
	Title       string  `json:"title"`
	Name        string  `json:"name"`        // TV shows use "name"
	OriginalTitle string `json:"original_title"`
	OriginalName  string `json:"original_name"`
	Overview    string  `json:"overview"`
	PosterPath  string  `json:"poster_path"`
	ReleaseDate string  `json:"release_date"`
	FirstAirDate string `json:"first_air_date"` // TV
	Runtime     int     `json:"runtime"`
	VoteAverage float64 `json:"vote_average"`
	Tagline     string  `json:"tagline"`
	Genres      []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"genres"`
}

const tmdbImageBase = "https://image.tmdb.org/t/p/w200"

// rawToString converts a json.RawMessage that may be a string or []string into a plain string
func rawToString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	// Try string first
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	// Try array of strings
	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil {
		return strings.Join(arr, " ")
	}
	return ""
}

// SearchTMDB searches using the custom nexum TMDB proxy (no API key required)
// The query should be the full release name, e.g. "Fireworks.1997.MULTi.1080p.BluRay.x264-FiDELiO"
func (a *App) SearchTMDB(query string, mediaType string) ([]TMDBResult, error) {
	params := url.Values{}
	params.Set("t", "search")
	params.Set("q", query)

	endpoint := nexumTMDBBase + "?" + params.Encode()

	resp, err := http.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("erreur de connexion TMDB: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("erreur lecture réponse: %w", err)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	// Try array response first, then object
	var raw struct {
		Results []nexumSearchResult `json:"results"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("erreur parsing: %w", err)
	}

	var results []TMDBResult
	for _, r := range raw.Results {
		title := bestTitle(r)
		id := 0
		if r.TmdbID != "" {
			id, _ = strconv.Atoi(r.TmdbID)
		}
		if id == 0 {
			continue
		}

		// Determine media type from api_url
		mt := mediaType
		if mt == "" {
			mt = "movie"
		}
		if strings.Contains(r.ApiURL, "t=tv") || strings.Contains(r.TmdbURL, "/tv/") {
			mt = "tv"
		}

		results = append(results, TMDBResult{
			ID:         id,
			Title:      title,
			Year:       r.Years,
			PosterPath: r.Poster,
			MediaType:  mt,
			Overview:   r.Overview,
			Popularity: r.NoteTmdb,
		})
	}
	return results, nil
}

// GetTMDBDetails fetches full details for a movie or TV show by TMDB ID
// The detail endpoint returns raw TMDB format (different from search results)
func (a *App) GetTMDBDetails(id int, mediaType string) (TMDBDetails, error) {
	t := "movie"
	if mediaType == "tv" {
		t = "tv"
	}

	params := url.Values{}
	params.Set("t", t)
	params.Set("q", strconv.Itoa(id))

	endpoint := nexumTMDBBase + "?" + params.Encode()

	resp, err := http.Get(endpoint)
	if err != nil {
		return TMDBDetails{}, fmt.Errorf("erreur de connexion TMDB: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return TMDBDetails{}, fmt.Errorf("erreur lecture: %w", err)
	}
	if resp.StatusCode != 200 {
		return TMDBDetails{}, fmt.Errorf("API error %d", resp.StatusCode)
	}

	var raw nexusDetailResult
	if err := json.Unmarshal(body, &raw); err != nil {
		return TMDBDetails{}, fmt.Errorf("erreur parsing: %w", err)
	}

	// Pick best title (TV uses "name")
	title := raw.Title
	if title == "" {
		title = raw.Name
	}
	if title == "" {
		title = raw.OriginalTitle
	}
	if title == "" {
		title = raw.OriginalName
	}

	// Extract year from release_date or first_air_date
	year := raw.ReleaseDate
	if year == "" {
		year = raw.FirstAirDate
	}
	if len(year) > 4 {
		year = year[:4]
	}

	// Build genres list
	var genres []string
	for _, g := range raw.Genres {
		if g.Name != "" {
			genres = append(genres, g.Name)
		}
	}

	// poster_path needs base URL
	poster := ""
	if raw.PosterPath != "" {
		poster = tmdbImageBase + raw.PosterPath
	}

	return TMDBDetails{
		ID:         raw.ID,
		Title:      title,
		Year:       year,
		Overview:   raw.Overview,
		PosterPath: poster,
		MediaType:  mediaType,
		Genres:     genres,
		Director:   "",
		Rating:     raw.VoteAverage,
		Runtime:    raw.Runtime,
	}, nil
}

// bestTitle picks the most appropriate title (french > english > original)
func bestTitle(r nexumSearchResult) string {
	if r.Title != "" {
		return r.Title
	}
	if r.EnglishTitle != "" {
		return r.EnglishTitle
	}
	return r.OriginalTitle
}

// parseRuntime converts "1 h 43 min" → 103
func parseRuntime(s string) int {
	if s == "" {
		return 0
	}
	s = strings.ToLower(s)
	total := 0
	// Find hours
	if idx := strings.Index(s, "h"); idx > 0 {
		part := strings.TrimSpace(s[:idx])
		if h, err := strconv.Atoi(part); err == nil {
			total += h * 60
		}
		s = s[idx+1:]
	}
	// Find minutes
	if idx := strings.Index(s, "min"); idx > 0 {
		part := strings.TrimSpace(s[:idx])
		if m, err := strconv.Atoi(part); err == nil {
			total += m
		}
	}
	return total
}
