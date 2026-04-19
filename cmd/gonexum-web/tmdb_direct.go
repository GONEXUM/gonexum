package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// tmdbAPIKey est injectée via -ldflags "-X main.tmdbAPIKey=..."
var tmdbAPIKey = ""

const tmdbAPIBase = "https://api.themoviedb.org/3"

// ── Release name parser ──────────────────────────────────────────────────────

var (
	reYear   = regexp.MustCompile(`\b(19|20)\d{2}\b`)
	reSeason = regexp.MustCompile(`(?i)\bs\d{1,2}(?:e\d{1,2})?\b`)
	reTech   = regexp.MustCompile(`(?i)\b(1080[ip]|720p|2160p|480p|4k|uhd|bluray|bdrip|bdremux|remux|web[-.]?dl|webrip|hdtv|dvdrip|dcp|x26[45]|hevc|avc|h\.?26[45]|dts(?:-hd(?:.ma)?)?|ac3|eac3|aac|truehd|flac|atmos|french|english|multi|vostfr|truefrench|proper|repack|internal|limited|10bit|8bit|hdr(?:10(?:\+)?)?|sdr|dv|dolby|vision|ita|spa|ger|jpn)\b`)
)

// parseReleaseName extracts the title and year from a scene-style release name.
// Stops at the first occurrence of: year, SxxExx, or a technical token.
// Ex: "Fight.Club.1999.1080p.BluRay.x264-GROUP" → ("Fight Club", 1999)
//     "Breaking.Bad.S01E01.720p.HDTV.x264-X" → ("Breaking Bad", 0)
func parseReleaseName(name string) (title string, year int) {
	// Strip extension
	for _, ext := range []string{".mkv", ".mp4", ".avi", ".ts", ".m2ts", ".MKV", ".MP4"} {
		name = strings.TrimSuffix(name, ext)
	}
	// Normalize underscores to dots
	norm := strings.ReplaceAll(name, "_", ".")

	// Find earliest stop position from year, season, or tech marker
	cut := len(norm)
	if m := reYear.FindStringIndex(norm); m != nil {
		if m[0] < cut {
			cut = m[0]
		}
		if y, err := strconv.Atoi(norm[m[0]:m[1]]); err == nil {
			year = y
		}
	}
	if m := reSeason.FindStringIndex(norm); m != nil && m[0] < cut {
		cut = m[0]
	}
	if m := reTech.FindStringIndex(norm); m != nil && m[0] < cut {
		cut = m[0]
	}

	title = strings.TrimRight(norm[:cut], ".-_ ")
	title = strings.NewReplacer(".", " ", "-", " ").Replace(title)
	title = strings.Join(strings.Fields(title), " ")
	return
}

// ── Direct TMDB API calls ────────────────────────────────────────────────────

type tmdbMovieResult struct {
	ID            int     `json:"id"`
	Title         string  `json:"title"`
	OriginalTitle string  `json:"original_title"`
	ReleaseDate   string  `json:"release_date"`
	PosterPath    string  `json:"poster_path"`
	Overview      string  `json:"overview"`
	VoteAverage   float64 `json:"vote_average"`
	Popularity    float64 `json:"popularity"`
}

type tmdbTVResult struct {
	ID            int     `json:"id"`
	Name          string  `json:"name"`
	OriginalName  string  `json:"original_name"`
	FirstAirDate  string  `json:"first_air_date"`
	PosterPath    string  `json:"poster_path"`
	Overview      string  `json:"overview"`
	VoteAverage   float64 `json:"vote_average"`
	Popularity    float64 `json:"popularity"`
}

type tmdbDetailResult struct {
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
	Genres        []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"genres"`
}

func tmdbDirectGet(path string, params url.Values) ([]byte, error) {
	if tmdbAPIKey == "" {
		return nil, fmt.Errorf("TMDB API key non configurée (fallback indisponible)")
	}
	params.Set("api_key", tmdbAPIKey)
	if params.Get("language") == "" {
		params.Set("language", "fr-FR")
	}
	resp, err := http.Get(tmdbAPIBase + path + "?" + params.Encode())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("TMDB API %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

// searchTMDBDirect queries the official TMDB API with title + year
func searchTMDBDirect(releaseName, mediaType string) ([]TMDBResult, error) {
	title, year := parseReleaseName(releaseName)
	if title == "" {
		return nil, fmt.Errorf("impossible d'extraire le titre du nom de release")
	}

	params := url.Values{}
	params.Set("query", title)
	params.Set("include_adult", "false")
	if year > 0 {
		if mediaType == "tv" {
			params.Set("first_air_date_year", strconv.Itoa(year))
		} else {
			params.Set("year", strconv.Itoa(year))
		}
	}

	path := "/search/movie"
	if mediaType == "tv" {
		path = "/search/tv"
	}
	body, err := tmdbDirectGet(path, params)
	if err != nil {
		return nil, err
	}

	var results []TMDBResult
	if mediaType == "tv" {
		var raw struct {
			Results []tmdbTVResult `json:"results"`
		}
		if err := json.Unmarshal(body, &raw); err != nil {
			return nil, err
		}
		for _, r := range raw.Results {
			name := r.Name
			if name == "" {
				name = r.OriginalName
			}
			yr := ""
			if len(r.FirstAirDate) >= 4 {
				yr = r.FirstAirDate[:4]
			}
			poster := ""
			if r.PosterPath != "" {
				poster = tmdbImageBase + r.PosterPath
			}
			results = append(results, TMDBResult{
				ID: r.ID, Title: name, Year: yr, PosterPath: poster,
				MediaType: "tv", Overview: r.Overview, Popularity: r.Popularity,
			})
		}
	} else {
		var raw struct {
			Results []tmdbMovieResult `json:"results"`
		}
		if err := json.Unmarshal(body, &raw); err != nil {
			return nil, err
		}
		for _, r := range raw.Results {
			t := r.Title
			if t == "" {
				t = r.OriginalTitle
			}
			yr := ""
			if len(r.ReleaseDate) >= 4 {
				yr = r.ReleaseDate[:4]
			}
			poster := ""
			if r.PosterPath != "" {
				poster = tmdbImageBase + r.PosterPath
			}
			results = append(results, TMDBResult{
				ID: r.ID, Title: t, Year: yr, PosterPath: poster,
				MediaType: "movie", Overview: r.Overview, Popularity: r.Popularity,
			})
		}
	}
	return results, nil
}

// getTMDBDetailsDirect fetches full details via the official TMDB API
func getTMDBDetailsDirect(id int, mediaType string) (TMDBDetails, error) {
	path := fmt.Sprintf("/movie/%d", id)
	if mediaType == "tv" {
		path = fmt.Sprintf("/tv/%d", id)
	}
	body, err := tmdbDirectGet(path, url.Values{})
	if err != nil {
		return TMDBDetails{}, err
	}

	var raw tmdbDetailResult
	if err := json.Unmarshal(body, &raw); err != nil {
		return TMDBDetails{}, err
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
		ID: raw.ID, Title: title, Year: year, Overview: raw.Overview,
		PosterPath: poster, MediaType: mediaType, Genres: genres,
		Rating: raw.VoteAverage, Runtime: raw.Runtime,
	}, nil
}
