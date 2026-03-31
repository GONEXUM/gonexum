package main

// TorrentResult is returned after creating a .torrent file
type TorrentResult struct {
	FilePath string `json:"filePath"`
	InfoHash string `json:"infoHash"`
	Name     string `json:"name"`
	Size     int64  `json:"size"`
}

// MediaInfo holds extracted media metadata from a video file
type MediaInfo struct {
	Resolution     string  `json:"resolution"`
	VideoCodec     string  `json:"videoCodec"`
	AudioCodec     string  `json:"audioCodec"`
	AudioLanguages string  `json:"audioLanguages"`
	HDRFormat      string  `json:"hdrFormat"`
	Source         string  `json:"source"`
	Duration       string  `json:"duration"`
	FileSize       int64   `json:"fileSize"`
	Width          int     `json:"width"`
	Height         int     `json:"height"`
	Bitrate        int64   `json:"bitrate"`
	FrameRate      float64 `json:"frameRate"`
}

// TMDBResult is a search result from TheMovieDB
type TMDBResult struct {
	ID          int     `json:"id"`
	Title       string  `json:"title"`
	Year        string  `json:"year"`
	PosterPath  string  `json:"posterPath"`
	MediaType   string  `json:"mediaType"`
	Overview    string  `json:"overview"`
	Popularity  float64 `json:"popularity"`
}

// TMDBDetails holds full details of a movie or TV show
type TMDBDetails struct {
	ID          int      `json:"id"`
	Title       string   `json:"title"`
	Year        string   `json:"year"`
	Overview    string   `json:"overview"`
	PosterPath  string   `json:"posterPath"`
	MediaType   string   `json:"mediaType"`
	Genres      []string `json:"genres"`
	Director    string   `json:"director"`
	Rating      float64  `json:"rating"`
	Runtime     int      `json:"runtime"`
}

// UploadParams holds all parameters needed for the nexum upload
type UploadParams struct {
	TorrentPath    string `json:"torrentPath"`
	NFOContent     string `json:"nfoContent"`
	Name           string `json:"name"`
	CategoryID     int    `json:"categoryId"`
	Description    string `json:"description"`
	TMDBId         int    `json:"tmdbId"`
	TMDBType       string `json:"tmdbType"`
	Resolution     string `json:"resolution"`
	VideoCodec     string `json:"videoCodec"`
	AudioCodec     string `json:"audioCodec"`
	AudioLanguages string `json:"audioLanguages"`
	HDRFormat      string `json:"hdrFormat"`
	Source         string `json:"source"`
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
