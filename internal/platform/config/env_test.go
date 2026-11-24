package config

import (
	"testing"
	"time"
)

// 池大小是 int32 消费方；超界环境值必须钳位而不是回绕成负数。
func TestPostgresPoolBounds(t *testing.T) {
	t.Setenv("POSTGRES_MAX_CONNS", "999999999999")
	if got := PostgresPoolMax(); got != maxInt32 {
		t.Errorf("PostgresPoolMax() = %d, want %d", got, maxInt32)
	}

	t.Setenv("POSTGRES_MAX_CONNS", "17")
	if got := PostgresPoolMax(); got != 17 {
		t.Errorf("PostgresPoolMax() = %d, want 17", got)
	}

	t.Setenv("POSTGRES_MAX_CONNS", "0")
	if got := PostgresPoolMax(); got != 0 {
		t.Errorf("PostgresPoolMax() = %d, want 0 for non-positive", got)
	}

	t.Setenv("POSTGRES_MIN_CONNS", "-5")
	if got := PostgresPoolMin(); got != 0 {
		t.Errorf("PostgresPoolMin() = %d, want 0 for negative", got)
	}

	t.Setenv("POSTGRES_MIN_CONNS", "3")
	if got := PostgresPoolMin(); got != 3 {
		t.Errorf("PostgresPoolMin() = %d, want 3", got)
	}
}

func TestCookieSecureDefaultsAndMalformedValue(t *testing.T) {
	tests := []struct {
		name    string
		appEnv  string
		setting string
		want    bool
	}{
		{name: "development default", appEnv: "development", setting: "", want: false},
		{name: "production default", appEnv: "production", setting: "", want: true},
		{name: "explicit development opt in", appEnv: "development", setting: "true", want: true},
		{name: "explicit development false", appEnv: "development", setting: "false", want: false},
		{name: "malformed fails secure", appEnv: "development", setting: "off", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("APP_ENV", tt.appEnv)
			t.Setenv("COOKIE_SECURE", tt.setting)
			if got := CookieSecure(); got != tt.want {
				t.Fatalf("CookieSecure() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUploadMaxBytes(t *testing.T) {
	t.Setenv("STORAGE_UPLOAD_MAX_BYTES", "")
	if got := UploadMaxBytes(); got != defaultUploadMaxBytes {
		t.Fatalf("UploadMaxBytes() = %d, want %d default", got, defaultUploadMaxBytes)
	}

	t.Setenv("STORAGE_UPLOAD_MAX_BYTES", "1048576")
	if got := UploadMaxBytes(); got != 1048576 {
		t.Fatalf("UploadMaxBytes() = %d, want 1048576", got)
	}

	t.Setenv("STORAGE_UPLOAD_MAX_BYTES", "not-a-number")
	if got := UploadMaxBytes(); got != defaultUploadMaxBytes {
		t.Fatalf("UploadMaxBytes() = %d, want default on malformed input", got)
	}
}

func TestIsAllowedUploadMimeType(t *testing.T) {
	t.Setenv("UPLOAD_ALLOWED_MIME_TYPES", "")
	if !IsAllowedUploadMimeType("image/png") {
		t.Fatal("image/png should be allowed by the default whitelist")
	}
	if !IsAllowedUploadMimeType("image/png; charset=binary") {
		t.Fatal("parameters must be stripped before matching")
	}
	if IsAllowedUploadMimeType("text/html") {
		t.Fatal("text/html must not be allowed by the default whitelist")
	}

	t.Setenv("UPLOAD_ALLOWED_MIME_TYPES", "image/png, application/pdf")
	if IsAllowedUploadMimeType("image/jpeg") {
		t.Fatal("image/jpeg should be rejected once the whitelist is overridden")
	}
	if !IsAllowedUploadMimeType("application/pdf") {
		t.Fatal("application/pdf should be allowed by the override")
	}
}

func TestIsAttachmentExtension(t *testing.T) {
	t.Setenv("STORAGE_ATTACHMENT_EXTENSIONS", "")
	for _, ext := range []string{".svg", "svg", ".HTML", ".js"} {
		if !IsAttachmentExtension(ext) {
			t.Fatalf("%q should be treated as active content by the default set", ext)
		}
	}
	if IsAttachmentExtension(".png") {
		t.Fatal(".png must not be forced to download by default")
	}

	t.Setenv("STORAGE_ATTACHMENT_EXTENSIONS", "png")
	if !IsAttachmentExtension(".png") {
		t.Fatal(".png should be forced to download once listed")
	}
	if IsAttachmentExtension(".svg") {
		t.Fatal(".svg should no longer match after the override")
	}
}

func TestUploadMimeTypeForExtensionAndWhitelist(t *testing.T) {
	t.Setenv("UPLOAD_ALLOWED_MIME_TYPES", "")

	if got := UploadMimeTypeForExtension(".PNG"); got != "image/png" {
		t.Fatalf("UploadMimeTypeForExtension(.PNG) = %q, want image/png", got)
	}
	if got := UploadMimeTypeForExtension("png"); got != "image/png" {
		t.Fatalf("UploadMimeTypeForExtension(png) = %q, want image/png", got)
	}
	if got := UploadMimeTypeForExtension(".exe"); got != "" {
		t.Fatalf("UploadMimeTypeForExtension(.exe) = %q, want empty", got)
	}

	for _, ext := range []string{".png", "png", ".JPG", ".jpeg", ".gif", ".webp", ".pdf"} {
		if !IsAllowedUploadExtension(ext) {
			t.Fatalf("%q should be an allowed upload extension", ext)
		}
	}
	for _, ext := range []string{".svg", ".html", ".js", ".exe", ""} {
		if IsAllowedUploadExtension(ext) {
			t.Fatalf("%q must not be an allowed upload extension", ext)
		}
	}

	// The extension whitelist follows UPLOAD_ALLOWED_MIME_TYPES: once the
	// operator drops image/png, .png is no longer acceptable.
	t.Setenv("UPLOAD_ALLOWED_MIME_TYPES", "application/pdf")
	if IsAllowedUploadExtension(".png") {
		t.Fatal(".png should be rejected once image/png leaves the whitelist")
	}
	if !IsAllowedUploadExtension(".pdf") {
		t.Fatal(".pdf should be allowed while application/pdf is whitelisted")
	}
}

func TestIsActiveContentMimeType(t *testing.T) {
	for _, mimeType := range []string{
		"text/html",
		"text/html; charset=utf-8",
		"image/svg+xml",
		"application/javascript",
		"application/x-msdownload",
	} {
		if !IsActiveContentMimeType(mimeType) {
			t.Fatalf("%q should be refused as active content", mimeType)
		}
	}
	for _, mimeType := range []string{"image/png", "application/pdf", ""} {
		if IsActiveContentMimeType(mimeType) {
			t.Fatalf("%q must not be classified as active content", mimeType)
		}
	}
}

func TestUploadMimeConsistent(t *testing.T) {
	cases := []struct {
		name              string
		declared, sniffed string
		extMime           string
		want              bool
	}{
		{"exact match", "image/png", "image/png", "image/png", true},
		{"declared carries parameters", "image/png; charset=binary", "image/png", "image/png", true},
		{"crop re-encode preserves original name and type", "image/jpeg", "image/png", "image/jpeg", true},
		{"html renamed to png", "image/png", "text/html", "image/png", false},
		{"pdf renamed to png", "image/png", "application/pdf", "image/png", false},
		{"declared disagrees with extension", "image/png", "image/png", "image/jpeg", false},
		{"empty declared", "", "image/png", "image/png", false},
		{"empty extension binding", "image/png", "image/png", "", false},
	}
	for _, tc := range cases {
		if got := UploadMimeConsistent(tc.declared, tc.sniffed, tc.extMime); got != tc.want {
			t.Fatalf("%s: UploadMimeConsistent(%q, %q, %q) = %v, want %v",
				tc.name, tc.declared, tc.sniffed, tc.extMime, got, tc.want)
		}
	}
}

func TestPresignPutTTL(t *testing.T) {
	t.Setenv("STORAGE_PRESIGN_PUT_TTL_SECONDS", "")
	if got := PresignPutTTL(); got != defaultPresignTTL {
		t.Fatalf("PresignPutTTL() = %v, want %v default", got, defaultPresignTTL)
	}

	t.Setenv("STORAGE_PRESIGN_PUT_TTL_SECONDS", "600")
	if got := PresignPutTTL(); got != 10*time.Minute {
		t.Fatalf("PresignPutTTL() = %v, want 10m", got)
	}

	t.Setenv("STORAGE_PRESIGN_PUT_TTL_SECONDS", "0")
	if got := PresignPutTTL(); got != defaultPresignTTL {
		t.Fatalf("PresignPutTTL() = %v, want default on non-positive input", got)
	}
}
