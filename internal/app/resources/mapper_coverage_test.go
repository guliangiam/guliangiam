package resources

import (
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/resources"
)

func TestDetectType(t *testing.T) {
	tests := []struct {
		name     string
		mimeType string
		filePath string
		want     string
	}{
		{"image_mime", "image/png", "photo.png", "image"},
		{"video_mime", "video/mp4", "clip.mp4", "video"},
		{"audio_mime", "audio/mpeg", "song.mp3", "audio"},
		{"css_ext", "", "style.css", "css"},
		{"js_ext", "", "app.js", "js"},
		{"jpg_ext", "", "photo.jpg", "image"},
		{"jpeg_ext", "", "photo.jpeg", "image"},
		{"png_ext", "", "icon.png", "image"},
		{"gif_ext", "", "anim.gif", "image"},
		{"webp_ext", "", "photo.webp", "image"},
		{"svg_ext", "", "logo.svg", "image"},
		{"mp4_ext", "", "video.mp4", "video"},
		{"webm_ext", "", "video.webm", "video"},
		{"mp3_ext", "", "audio.mp3", "audio"},
		{"wav_ext", "", "audio.wav", "audio"},
		{"unknown_ext", "", "document.pdf", "file"},
		{"no_ext", "", "README", "file"},
		{"uppercase_ext", "", "Photo.PNG", "image"},
		{"image_mime_overrides_ext", "image/png", "data.bin", "image"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectType(tt.mimeType, tt.filePath)
			if got != tt.want {
				t.Errorf("detectType(%q, %q) = %q, want %q", tt.mimeType, tt.filePath, got, tt.want)
			}
		})
	}
}

func TestToListItem(t *testing.T) {
	now := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	r := &domain.Resource{
		ID:             "res-1",
		OrganizationID: "org-1",
		ApplicationID:  "app-1",
		Name:           "logo",
		DisplayName:    "Logo",
		Type:           "image",
		Tag:            "brand",
		FilePath:       "/uploads/logo.png",
		FileSize:       1024,
		MimeType:       "image/png",
		CreatedAt:      now,
		UpdatedAt:      now,
		Revision:       1,
	}

	item := toListItem(r)
	if item.ID != "res-1" {
		t.Errorf("ID = %q, want res-1", item.ID)
	}
	if item.Name != "logo" {
		t.Errorf("Name = %q, want logo", item.Name)
	}
	if item.Type != "image" {
		t.Errorf("Type = %q, want image", item.Type)
	}
	if item.FileSize != 1024 {
		t.Errorf("FileSize = %d, want 1024", item.FileSize)
	}
}

func TestToDetail(t *testing.T) {
	now := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	r := &domain.Resource{
		ID:             "res-1",
		OrganizationID: "org-1",
		Name:           "logo",
		DisplayName:    "Logo",
		Type:           "image",
		FilePath:       "/uploads/logo.png",
		FileSize:       2048,
		MimeType:       "image/png",
		CreatedAt:      now,
		UpdatedAt:      now,
		Revision:       3,
	}

	detail := toDetail(r)
	if detail.ID != "res-1" {
		t.Errorf("ID = %q, want res-1", detail.ID)
	}
	if detail.Revision != 3 {
		t.Errorf("Revision = %d, want 3", detail.Revision)
	}
	if detail.FileSize != 2048 {
		t.Errorf("FileSize = %d, want 2048", detail.FileSize)
	}
}
