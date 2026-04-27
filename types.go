package main

import "encoding/json"

// TorrentResult is returned after creating a .torrent file
type TorrentResult struct {
	FilePath string `json:"filePath"`
	InfoHash string `json:"infoHash"`
	Name     string `json:"name"`
	Size     int64  `json:"size"`
}

// MediaInfo holds extracted media metadata from a video file
type MediaInfo struct {
	Resolution        string  `json:"resolution"`
	VideoCodec        string  `json:"videoCodec"`
	AudioCodec        string  `json:"audioCodec"`
	AudioLanguages    string  `json:"audioLanguages"`
	SubtitleLanguages string  `json:"subtitleLanguages"`
	HDRFormat         string  `json:"hdrFormat"`
	Source            string  `json:"source"`
	Duration          string  `json:"duration"`
	FileSize          int64   `json:"fileSize"`
	Width             int     `json:"width"`
	Height            int     `json:"height"`
	Bitrate           int64   `json:"bitrate"`
	FrameRate         float64 `json:"frameRate"`
}

// TMDBResult is a search result from TheMovieDB
type TMDBResult struct {
	ID         int     `json:"id"`
	Title      string  `json:"title"`
	Year       string  `json:"year"`
	PosterPath string  `json:"posterPath"`
	MediaType  string  `json:"mediaType"`
	Overview   string  `json:"overview"`
	Popularity float64 `json:"popularity"`
}

// UnmarshalJSON accepte aussi les conventions TMDB (poster_path, media_type, release_date).
func (r *TMDBResult) UnmarshalJSON(data []byte) error {
	type alias TMDBResult
	aux := struct {
		*alias
		PosterPathSnake string `json:"poster_path"`
		MediaTypeSnake  string `json:"media_type"`
		Name            string `json:"name"`
		ReleaseDate     string `json:"release_date"`
		FirstAirDate    string `json:"first_air_date"`
	}{alias: (*alias)(r)}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if r.PosterPath == "" && aux.PosterPathSnake != "" {
		r.PosterPath = aux.PosterPathSnake
	}
	if r.MediaType == "" && aux.MediaTypeSnake != "" {
		r.MediaType = aux.MediaTypeSnake
	}
	if r.Title == "" && aux.Name != "" {
		r.Title = aux.Name
	}
	if r.Year == "" {
		if len(aux.ReleaseDate) >= 4 {
			r.Year = aux.ReleaseDate[:4]
		} else if len(aux.FirstAirDate) >= 4 {
			r.Year = aux.FirstAirDate[:4]
		}
	}
	return nil
}

// TMDBDetails holds full details of a movie or TV show
type TMDBDetails struct {
	ID         int      `json:"id"`
	Title      string   `json:"title"`
	Year       string   `json:"year"`
	Overview   string   `json:"overview"`
	PosterPath string   `json:"posterPath"`
	MediaType  string   `json:"mediaType"`
	Genres     []string `json:"genres"`
	Director   string   `json:"director"`
	Rating     float64  `json:"rating"`
	Runtime    int      `json:"runtime"`
}

// UnmarshalJSON accepte aussi le format brut TMDB (poster_path, name, release_date, first_air_date, vote_average).
func (d *TMDBDetails) UnmarshalJSON(data []byte) error {
	type alias TMDBDetails
	aux := struct {
		*alias
		PosterPathSnake string  `json:"poster_path"`
		MediaTypeSnake  string  `json:"media_type"`
		Name            string  `json:"name"`
		ReleaseDate     string  `json:"release_date"`
		FirstAirDate    string  `json:"first_air_date"`
		VoteAverage     float64 `json:"vote_average"`
	}{alias: (*alias)(d)}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if d.PosterPath == "" && aux.PosterPathSnake != "" {
		d.PosterPath = aux.PosterPathSnake
	}
	if d.MediaType == "" && aux.MediaTypeSnake != "" {
		d.MediaType = aux.MediaTypeSnake
	}
	if d.Title == "" && aux.Name != "" {
		d.Title = aux.Name
	}
	if d.Year == "" {
		if len(aux.ReleaseDate) >= 4 {
			d.Year = aux.ReleaseDate[:4]
		} else if len(aux.FirstAirDate) >= 4 {
			d.Year = aux.FirstAirDate[:4]
		}
	}
	if d.Rating == 0 && aux.VoteAverage > 0 {
		d.Rating = aux.VoteAverage
	}
	return nil
}

// UploadParams holds all parameters needed for the nexum upload
type UploadParams struct {
	TorrentPath       string `json:"torrentPath"`
	NFOContent        string `json:"nfoContent"`
	Name              string `json:"name"`
	CategoryID        int    `json:"categoryId"`
	Description       string `json:"description"`
	TMDBId            int    `json:"tmdbId"`
	TMDBType          string `json:"tmdbType"`
	Resolution        string `json:"resolution"`
	VideoCodec        string `json:"videoCodec"`
	AudioCodec        string `json:"audioCodec"`
	AudioLanguages    string `json:"audioLanguages"`
	SubtitleLanguages string `json:"subtitleLanguages"`
	HDRFormat         string `json:"hdrFormat"`
	Source            string `json:"source"`
}

// UploadMedia holds the media tags returned in UploadResponse
type UploadMedia struct {
	Resolution     string  `json:"resolution"`
	VideoCodec     string  `json:"video_codec"`
	AudioCodec     string  `json:"audio_codec"`
	AudioLanguages string  `json:"audio_languages"`
	HDRFormat      *string `json:"hdr_format"`
	Source         string  `json:"source"`
}

// UploadResponse is the response from the nexum upload API
type UploadResponse struct {
	Success   bool        `json:"success"`
	TorrentID int         `json:"torrent_id"`
	InfoHash  string      `json:"info_hash"`
	Name      string      `json:"name"`
	Size      int64       `json:"size"`
	Status    string      `json:"status"`
	URL       string      `json:"url"`
	Media     UploadMedia `json:"media"`
	Error     string      `json:"error,omitempty"`
}

// Settings holds all user-configurable API keys and preferences
type Settings struct {
	APIKey      string `json:"apiKey"`
	Passkey     string `json:"passkey"`
	TMDBAPIKey  string `json:"tmdbApiKey"`
	TrackerURL  string `json:"trackerUrl"`
	OutputDir   string `json:"outputDir"`
	NFOTemplate string `json:"nfoTemplate"`
	NFOMode     string `json:"nfoMode"` // "nfo" (défaut) ou "mediainfo"
}
