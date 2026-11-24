package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/guliangiam/guliangiam/internal/platform/ssrf"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// SynologyNASStorage implements storage using Synology NAS File Station API.
type SynologyNASStorage struct {
	baseURL    string
	account    string
	password   string
	sid        string // Session ID from login
	HTTPClient *http.Client
}

// NewSynologyNASStorage creates a new Synology NAS storage provider.
func NewSynologyNASStorage(baseURL, account, password string) *SynologyNASStorage {
	return &SynologyNASStorage{
		baseURL:    strings.TrimRight(baseURL, "/"),
		account:    account,
		password:   password,
		HTTPClient: ssrf.DefaultHTTPClient,
	}
}

// synologyAPIResponse is the generic Synology API response structure.
type synologyAPIResponse struct {
	Success bool            `json:"success"`
	Error   *synologyError  `json:"error,omitempty"`
	Data    json.RawMessage `json:"data,omitempty"`
}

type synologyError struct {
	Code int `json:"code"`
}

// login authenticates with the Synology NAS and stores the session ID.
func (s *SynologyNASStorage) login(ctx context.Context) error {
	apiURL := fmt.Sprintf("%s/webapi/auth.cgi?api=SYNO.API.Auth&version=6&method=login&account=%s&passwd=%s&session=FileStation&format=sid",
		s.baseURL, url.QueryEscape(s.account), url.QueryEscape(s.password))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, http.NoBody)
	if err != nil {
		return apperrors.WrapInternalf(err, "synology: create login request")
	}

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return apperrors.WrapInternalf(err, "synology: login request")
	}
	defer resp.Body.Close()

	var result struct {
		Success bool `json:"success"`
		Data    struct {
			SID string `json:"sid"`
		} `json:"data"`
		Error *synologyError `json:"error,omitempty"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return apperrors.WrapInternalf(err, "synology: parse login response")
	}

	if !result.Success {
		code := 0
		if result.Error != nil {
			code = result.Error.Code
		}
		return apperrors.NewInternalf("synology: login failed (error code %d)", code)
	}

	s.sid = result.Data.SID
	return nil
}

// ensureSession logs in if no session is active.
func (s *SynologyNASStorage) ensureSession(ctx context.Context) error {
	if s.sid != "" {
		return nil
	}
	return s.login(ctx)
}

func (s *SynologyNASStorage) Put(ctx context.Context, key string, data []byte, contentType string) error {
	if err := s.ensureSession(ctx); err != nil {
		return err
	}

	// Build the upload URL
	uploadURL := fmt.Sprintf("%s/webapi/entry.cgi?_sid=%s", s.baseURL, s.sid)

	// Create multipart form
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add API parameters as form fields
	fields := map[string]string{
		"api":     "SYNO.FileStation.Upload",
		"version": "2",
		"method":  "upload",
	}
	for k, v := range fields {
		if err := writer.WriteField(k, v); err != nil {
			return apperrors.WrapInternalf(err, k, "synology: write form field %s")
		}
	}

	// Destination folder path and overwrite behavior
	dir := filepath.Dir(key)
	if dir == "." {
		dir = "/"
	}
	if err := writer.WriteField("dest_folder_path", dir); err != nil {
		return apperrors.WrapInternalf(err, "synology: write dest_folder_path")
	}
	if err := writer.WriteField("overwrite", "true"); err != nil {
		return apperrors.WrapInternalf(err, "synology: write overwrite")
	}

	// Create the file part
	filename := filepath.Base(key)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return apperrors.WrapInternalf(err, "synology: create form file")
	}
	if _, err := part.Write(data); err != nil {
		return apperrors.WrapInternalf(err, "synology: write file data")
	}

	if err := writer.Close(); err != nil {
		return apperrors.WrapInternalf(err, "synology: close multipart writer")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, &buf)
	if err != nil {
		return apperrors.WrapInternalf(err, "synology: create upload request")
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return apperrors.WrapInternalf(err, "synology: upload request")
	}
	defer resp.Body.Close()

	var apiResp synologyAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return apperrors.WrapInternalf(err, "synology: parse upload response")
	}

	if !apiResp.Success {
		code := 0
		if apiResp.Error != nil {
			code = apiResp.Error.Code
		}
		// If session expired (error 119), retry once
		if code == 119 {
			s.sid = ""
			if err := s.login(ctx); err != nil {
				return apperrors.WrapInternalf(err, "synology: re-login after session expired")
			}
			return s.Put(ctx, key, data, contentType)
		}
		return apperrors.NewInternalf("synology: upload failed (error code %d)", code)
	}

	return nil
}

func (s *SynologyNASStorage) Get(ctx context.Context, key string) ([]byte, error) {
	if err := s.ensureSession(ctx); err != nil {
		return nil, err
	}

	apiURL := fmt.Sprintf("%s/webapi/entry.cgi?api=SYNO.FileStation.Download&version=2&method=download&path=%s&mode=download&_sid=%s",
		s.baseURL, url.QueryEscape(key), s.sid)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, http.NoBody)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "synology: create download request")
	}

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "synology: download request")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, apperrors.NewInternalf("synology: download failed with status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "synology: read download response")
	}
	return data, nil
}

func (s *SynologyNASStorage) Delete(ctx context.Context, key string) error {
	if err := s.ensureSession(ctx); err != nil {
		return err
	}

	apiURL := fmt.Sprintf("%s/webapi/entry.cgi?api=SYNO.FileStation.Delete&version=2&method=delete&path=%s&_sid=%s",
		s.baseURL, url.QueryEscape(key), s.sid)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, http.NoBody)
	if err != nil {
		return apperrors.WrapInternalf(err, "synology: create delete request")
	}

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return apperrors.WrapInternalf(err, "synology: delete request")
	}
	defer resp.Body.Close()

	var apiResp synologyAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return apperrors.WrapInternalf(err, "synology: parse delete response")
	}

	if !apiResp.Success {
		code := 0
		if apiResp.Error != nil {
			code = apiResp.Error.Code
		}
		return apperrors.NewInternalf("synology: delete failed (error code %d)", code)
	}

	return nil
}

func (s *SynologyNASStorage) List(ctx context.Context, prefix string) ([]ObjectInfo, error) {
	if err := s.ensureSession(ctx); err != nil {
		return nil, err
	}

	apiURL := fmt.Sprintf("%s/webapi/entry.cgi?api=SYNO.FileStation.List&version=2&method=list&folder_path=%s&_sid=%s",
		s.baseURL, url.QueryEscape(prefix), s.sid)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, http.NoBody)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "synology: create list request")
	}

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "synology: list request")
	}
	defer resp.Body.Close()

	var result struct {
		Success bool            `json:"success"`
		Error   *synologyError  `json:"error,omitempty"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, apperrors.WrapInternalf(err, "synology: parse list response")
	}

	if !result.Success {
		code := 0
		if result.Error != nil {
			code = result.Error.Code
		}
		return nil, apperrors.NewInternalf("synology: list failed (error code %d)", code)
	}

	// Parse the files from the data
	var listData struct {
		Files []struct {
			Name       string `json:"name"`
			IsDir      bool   `json:"isdir"`
			Size       int64  `json:"size"`
			Additional struct {
				Time struct {
					ModifiedTime string `json:"mtime"`
				} `json:"time"`
				Type string `json:"type"`
			} `json:"additional"`
		} `json:"files"`
	}
	if err := json.Unmarshal(result.Data, &listData); err != nil {
		return nil, apperrors.WrapInternalf(err, "synology: parse file list")
	}

	var objects []ObjectInfo
	for _, f := range listData.Files {
		if f.IsDir {
			continue
		}
		modTime, _ := time.Parse(time.RFC3339, f.Additional.Time.ModifiedTime)
		objects = append(objects, ObjectInfo{
			Key:          filepath.Join(prefix, f.Name),
			Size:         f.Size,
			LastModified: modTime,
		})
	}
	return objects, nil
}

func (s *SynologyNASStorage) GetURL(ctx context.Context, key string, expires time.Duration) (string, error) {
	// Synology NAS doesn't natively support pre-signed URLs
	return "", apperrors.NewUnimplemented("synology NAS does not support pre-signed URLs")
}

func (s *SynologyNASStorage) PutStream(ctx context.Context, key string, reader io.Reader, contentType string) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return apperrors.WrapInternalf(err, "synology: read stream")
	}
	return s.Put(ctx, key, data, contentType)
}
