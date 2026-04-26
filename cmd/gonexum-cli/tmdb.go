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

// nexumTMDBBase est injecté via -ldflags "-X main.nexumTMDBBase=..."
var nexumTMDBBase = ""

type nexumSearchResult struct {
	Title         string          `json:"title"`
	Years         json.RawMessage `json:"years"`
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
	TmdbID        json.RawMessage `json:"tmdb_id"`
	TmdbURL       string          `json:"tmdb_url"`
	ApiURL        string          `json:"api_url"`
	NoteTmdb      float64         `json:"note_tmdb"`
	VoteTmdb      int             `json:"vote_tmdb"`
	Tagline       string          `json:"tagline"`
	Overview      string          `json:"overview"`
}

type nexusDetailResult struct {
	ID            int     `json:"id"`
	Title         string  `json:"title"`
	Name          string  `json:"name"`
	OriginalTitle string  `json:"original_title"`
	OriginalName  string  `json:"original_name"`
	Overview      string  `json:"overview"`
	PosterPath    string  `json:"poster_path"`
	ReleaseDate   string  `json:"release_date"`
	FirstAirDate  string  `json:"first_air_date"`
	Runtime       int     `json:"runtime"`
	VoteAverage   float64 `json:"vote_average"`
	Tagline       string  `json:"tagline"`
	Genres        []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"genres"`
}

const tmdbImageBase = "https://image.tmdb.org/t/p/w200"

func searchTMDB(query string, mediaType string) ([]TMDBResult, error) {
	results, err := searchTMDBProxy(query, mediaType)
	if err == nil && len(results) > 0 {
		return results, nil
	}
	// Fallback: official TMDB API — auto-détecte movie/tv si non précisé
	dt := mediaType
	if dt == "" {
		dt = detectMediaType(query)
	}
	if direct, derr := searchTMDBDirect(query, dt); derr == nil && len(direct) > 0 {
		return direct, nil
	}
	if mediaType == "" {
		other := "movie"
		if dt == "movie" {
			other = "tv"
		}
		if direct, derr := searchTMDBDirect(query, other); derr == nil && len(direct) > 0 {
			return direct, nil
		}
	}
	if err != nil {
		return nil, err
	}
	return results, nil
}

func searchTMDBProxy(query string, mediaType string) ([]TMDBResult, error) {
	if nexumTMDBBase == "" {
		return nil, nil // proxy non configuré → fallback direct
	}
	params := url.Values{}
	params.Set("t", "search")
	params.Set("q", query)

	resp, err := http.Get(nexumTMDBBase + "?" + params.Encode())
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

	var raw struct {
		Results []nexumSearchResult `json:"results"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("erreur parsing: %w", err)
	}

	var results []TMDBResult
	for _, r := range raw.Results {
		title := bestTitle(r)
		id := parseTmdbID(r.TmdbID)
		if id == 0 {
			continue
		}

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
			Year:       parseYears(r.Years),
			PosterPath: r.Poster,
			MediaType:  mt,
			Overview:   r.Overview,
			Popularity: r.NoteTmdb,
		})
	}
	return results, nil
}

func getTMDBDetails(id int, mediaType string) (TMDBDetails, error) {
	details, err := getTMDBDetailsProxy(id, mediaType)
	if err == nil && details.Title != "" {
		return details, nil
	}
	// Fallback: official TMDB API
	if d, derr := getTMDBDetailsDirect(id, mediaType); derr == nil && d.Title != "" {
		return d, nil
	}
	if err != nil {
		return TMDBDetails{}, err
	}
	return details, nil
}

func getTMDBDetailsProxy(id int, mediaType string) (TMDBDetails, error) {
	if nexumTMDBBase == "" {
		return TMDBDetails{}, nil
	}
	t := "movie"
	if mediaType == "tv" {
		t = "tv"
	}

	params := url.Values{}
	params.Set("t", t)
	params.Set("q", strconv.Itoa(id))

	resp, err := http.Get(nexumTMDBBase + "?" + params.Encode())
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

	year := raw.ReleaseDate
	if year == "" {
		year = raw.FirstAirDate
	}
	if len(year) > 4 {
		year = year[:4]
	}

	var genres []string
	for _, g := range raw.Genres {
		if g.Name != "" {
			genres = append(genres, g.Name)
		}
	}

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

func bestTitle(r nexumSearchResult) string {
	if r.Title != "" {
		return r.Title
	}
	if r.EnglishTitle != "" {
		return r.EnglishTitle
	}
	return r.OriginalTitle
}

// parseYears handles years field which may be a JSON number (2024)
// or a string ("2024") depending on the proxy version.
func parseYears(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var n int
	if err := json.Unmarshal(raw, &n); err == nil && n > 0 {
		return strconv.Itoa(n)
	}
	return ""
}

// parseTmdbID handles tmdb_id field which may be a JSON number or a string.
func parseTmdbID(raw json.RawMessage) int {
	if len(raw) == 0 {
		return 0
	}
	var n int
	if err := json.Unmarshal(raw, &n); err == nil {
		return n
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		n, _ = strconv.Atoi(s)
		return n
	}
	return 0
}
