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
	// TmdbID peut être un string "550" ou un entier 550 selon le proxy
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

func tmdbGet(rawURL string) ([]byte, error) {
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Referer", "https://nexum-core.com/")
	req.Header.Set("Origin", "https://nexum-core.com")
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; GONEXUM/1.0)")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

func searchTMDB(query string, mediaType string) ([]TMDBResult, error) {
	results, err := searchTMDBProxy(query, mediaType)
	if err == nil && len(results) > 0 {
		return results, nil
	}
	if direct, derr := searchTMDBDirect(query, mediaType); derr == nil && len(direct) > 0 {
		return direct, nil
	}
	if err != nil {
		return nil, err
	}
	return results, nil
}

func searchTMDBProxy(query string, mediaType string) ([]TMDBResult, error) {
	params := url.Values{}
	params.Set("t", "search")
	params.Set("q", query)

	body, err := tmdbGet(nexumTMDBBase + "?" + params.Encode())
	if err != nil {
		return nil, fmt.Errorf("erreur TMDB: %w", err)
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
			Year:       r.Years,
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
	if d, derr := getTMDBDetailsDirect(id, mediaType); derr == nil && d.Title != "" {
		return d, nil
	}
	if err != nil {
		return TMDBDetails{}, err
	}
	return details, nil
}

func getTMDBDetailsProxy(id int, mediaType string) (TMDBDetails, error) {
	t := "movie"
	if mediaType == "tv" {
		t = "tv"
	}

	params := url.Values{}
	params.Set("t", t)
	params.Set("q", strconv.Itoa(id))

	body, err := tmdbGet(nexumTMDBBase + "?" + params.Encode())
	if err != nil {
		return TMDBDetails{}, fmt.Errorf("erreur TMDB: %w", err)
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

// parseTmdbID handles both string ("550") and integer (550) JSON values.
func parseTmdbID(raw json.RawMessage) int {
	if len(raw) == 0 {
		return 0
	}
	// Try integer first
	var n int
	if err := json.Unmarshal(raw, &n); err == nil {
		return n
	}
	// Try quoted string
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		n, _ = strconv.Atoi(s)
		return n
	}
	return 0
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
