package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"strconv"
)

func uploadTorrent(params UploadParams, settings Settings) (UploadResponse, error) {
	if settings.APIKey == "" {
		return UploadResponse{}, fmt.Errorf("API key non configurée (utilisez --config ou modifiez settings.json)")
	}

	nfoPath := filepath.Join(os.TempDir(), "gonexum-upload.nfo")
	if err := os.WriteFile(nfoPath, []byte(params.NFOContent), 0600); err != nil {
		return UploadResponse{}, fmt.Errorf("failed to write NFO file: %w", err)
	}
	defer os.Remove(nfoPath)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	if err := addFilePart(writer, "torrent", params.TorrentPath); err != nil {
		return UploadResponse{}, fmt.Errorf("failed to attach torrent file: %w", err)
	}
	if err := addFilePart(writer, "nfo", nfoPath); err != nil {
		return UploadResponse{}, fmt.Errorf("failed to attach NFO file: %w", err)
	}

	desc := params.Description
	if len(desc) < 10 {
		desc = params.NFOContent
	}
	if len(desc) < 10 {
		desc = "Upload via GONEXUM"
	}
	if len(desc) > 10000 {
		desc = desc[:10000]
	}
	tmdbType := params.TMDBType
	if tmdbType == "" {
		tmdbType = "movie"
	}

	fields := map[string]string{
		"name":        normalizeName(params.Name),
		"category_id": strconv.Itoa(params.CategoryID),
		"description": desc,
		"tmdb_id":     strconv.Itoa(params.TMDBId),
		"tmdb_type":   tmdbType,
	}
	if params.Resolution != "" {
		fields["resolution"] = params.Resolution
	}
	if params.VideoCodec != "" {
		fields["video_codec"] = params.VideoCodec
	}
	if params.AudioCodec != "" {
		fields["audio_codec"] = params.AudioCodec
	}
	if params.AudioLanguages != "" {
		fields["audio_languages"] = params.AudioLanguages
	}
	if params.SubtitleLanguages != "" {
		fields["subtitle_languages"] = params.SubtitleLanguages
	}
	if params.HDRFormat != "" {
		fields["hdr_format"] = params.HDRFormat
	}
	if params.Source != "" {
		fields["source"] = params.Source
	}

	for k, v := range fields {
		if err := writer.WriteField(k, v); err != nil {
			return UploadResponse{}, fmt.Errorf("failed to write field %s: %w", k, err)
		}
	}
	if err := writer.Close(); err != nil {
		return UploadResponse{}, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	uploadURL := fmt.Sprintf("%s/api/v1/upload?apikey=%s", settings.TrackerURL, settings.APIKey)
	req, err := http.NewRequest("POST", uploadURL, &body)
	if err != nil {
		return UploadResponse{}, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return UploadResponse{}, fmt.Errorf("upload request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return UploadResponse{}, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		var errResp struct {
			Message string              `json:"message"`
			Error   string              `json:"error"`
			Errors  map[string][]string `json:"errors"`
		}
		_ = json.Unmarshal(respBody, &errResp)
		msg := errResp.Message
		if msg == "" {
			msg = errResp.Error
		}
		if len(errResp.Errors) > 0 {
			for field, msgs := range errResp.Errors {
				for _, m := range msgs {
					msg += "\n  " + field + ": " + m
				}
			}
		}
		if msg == "" {
			msg = string(respBody)
		}
		return UploadResponse{}, fmt.Errorf("server error %d: %s", resp.StatusCode, msg)
	}

	var result UploadResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return UploadResponse{}, fmt.Errorf("failed to parse server response: %w", err)
	}
	return result, nil
}

func normalizeName(name string) string {
	var buf []byte
	prevDot := false
	for i := 0; i < len(name); i++ {
		c := name[i]
		if c == '(' || c == ')' {
			continue
		}
		if c == ' ' {
			c = '.'
		}
		if c == '.' && prevDot {
			continue
		}
		prevDot = c == '.'
		buf = append(buf, c)
	}
	return string(buf)
}

func addFilePart(writer *multipart.Writer, field, filePath string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition",
		fmt.Sprintf(`form-data; name="%s"; filename="%s"`, field, filepath.Base(filePath)))
	h.Set("Content-Type", "application/octet-stream")

	part, err := writer.CreatePart(h)
	if err != nil {
		return err
	}
	_, err = io.Copy(part, f)
	return err
}
