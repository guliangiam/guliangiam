package system

import (
	"context"
	"testing"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCLIService(t *testing.T) {
	t.Parallel()
	svc := NewCLIService()
	assert.NotNil(t, svc)
}

// ─── DownloadCLI ──────────────────────────────────────────────────────────

func TestCLIService_DownloadCLI(t *testing.T) {
	t.Parallel()
	svc := NewCLIService()

	tests := []struct {
		name     string
		language string
		platform Platform
		wantErr  bool
		errCode  apperrors.Code
	}{
		{
			name:     "Go darwin amd64",
			language: "go",
			platform: Platform{OS: "darwin", Arch: "amd64"},
			wantErr:  false,
		},
		{
			name:     "Go linux arm64",
			language: "go",
			platform: Platform{OS: "linux", Arch: "arm64"},
			wantErr:  false,
		},
		{
			name:     "Python any",
			language: "python",
			platform: Platform{OS: "linux", Arch: "amd64"},
			wantErr:  false,
		},
		{
			name:     "Java any",
			language: "java",
			platform: Platform{OS: "linux", Arch: "amd64"},
			wantErr:  false,
		},
		{
			name:     "NodeJS darwin arm64",
			language: "nodejs",
			platform: Platform{OS: "darwin", Arch: "arm64"},
			wantErr:  false,
		},
		{
			name:     "CSharp any",
			language: "csharp",
			platform: Platform{OS: "linux", Arch: "amd64"},
			wantErr:  false,
		},
		{
			name:     "unsupported language",
			language: "rust",
			platform: Platform{OS: "linux", Arch: "amd64"},
			wantErr:  true,
			errCode:  apperrors.CodeInvalidArgument,
		},
		{
			name:     "unsupported OS",
			language: "go",
			platform: Platform{OS: "freebsd", Arch: "amd64"},
			wantErr:  true,
			errCode:  apperrors.CodeInvalidArgument,
		},
		{
			name:     "unsupported arch",
			language: "go",
			platform: Platform{OS: "linux", Arch: "riscv"},
			wantErr:  true,
			errCode:  apperrors.CodeInvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := svc.DownloadCLI(context.Background(), tt.language, tt.platform)
			if tt.wantErr {
				require.Error(t, err)
				appErr := apperrors.FromError(err)
				assert.Equal(t, tt.errCode, appErr.Code)
			} else {
				require.NoError(t, err)
				assert.NotEmpty(t, result.DownloadURL)
				assert.NotEmpty(t, result.Filename)
			}
		})
	}
}

func TestCLIService_DownloadCLI_NormalizePlatform(t *testing.T) {
	t.Parallel()
	svc := NewCLIService()

	tests := []struct {
		name     string
		language string
		platform Platform
	}{
		{"macos normalized to darwin", "go", Platform{OS: "macos", Arch: "amd64"}},
		{"mac normalized to darwin", "go", Platform{OS: "mac", Arch: "amd64"}},
		{"x86_64 normalized to amd64", "go", Platform{OS: "linux", Arch: "x86_64"}},
		{"x64 normalized to amd64", "go", Platform{OS: "linux", Arch: "x64"}},
		{"aarch64 normalized to arm64", "go", Platform{OS: "linux", Arch: "aarch64"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := svc.DownloadCLI(context.Background(), tt.language, tt.platform)
			require.NoError(t, err)
			assert.NotEmpty(t, result.Filename)
		})
	}
}

func TestCLIService_DownloadCLI_CaseInsensitive(t *testing.T) {
	t.Parallel()
	svc := NewCLIService()

	result, err := svc.DownloadCLI(context.Background(), "GO", Platform{OS: "Darwin", Arch: "AMD64"})
	require.NoError(t, err)
	assert.NotEmpty(t, result.Filename)
}

// ─── DetectPlatformFromUserAgent ──────────────────────────────────────────

func TestDetectPlatformFromUserAgent(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		ua       string
		wantOS   string
		wantArch string
	}{
		{"Windows Chrome", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36", "windows", "amd64"},
		{"Mac Chrome", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36", "darwin", "amd64"},
		{"Linux Firefox", "Mozilla/5.0 (X11; Linux x86_64; rv:109.0) Gecko/20100101", "linux", "amd64"},
		{"Linux ARM64", "Mozilla/5.0 (X11; Linux aarch64)", "linux", "arm64"},
		{"Empty UA", "", "linux", "amd64"}, // defaults
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := DetectPlatformFromUserAgent(tt.ua)
			assert.Equal(t, tt.wantOS, p.OS)
			assert.Equal(t, tt.wantArch, p.Arch)
		})
	}
}

// ─── normalizePlatform ────────────────────────────────────────────────────

func TestNormalizePlatform(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		os, arch string
		wantOS   string
		wantArch string
	}{
		{"macos", "macos", "amd64", "darwin", "amd64"},
		{"mac", "mac", "x86_64", "darwin", "amd64"},
		{"x64", "linux", "x64", "linux", "amd64"},
		{"aarch64", "linux", "aarch64", "linux", "arm64"},
		{"already normalized", "darwin", "arm64", "darwin", "arm64"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os, arch := normalizePlatform(tt.os, tt.arch)
			assert.Equal(t, tt.wantOS, os)
			assert.Equal(t, tt.wantArch, arch)
		})
	}
}

// ─── isValidOS / isValidArch ──────────────────────────────────────────────

func TestIsValidOS(t *testing.T) {
	t.Parallel()
	assert.True(t, isValidOS("darwin"))
	assert.True(t, isValidOS("linux"))
	assert.True(t, isValidOS("windows"))
	assert.False(t, isValidOS("freebsd"))
	assert.False(t, isValidOS(""))
}

func TestIsValidArch(t *testing.T) {
	t.Parallel()
	assert.True(t, isValidArch("amd64"))
	assert.True(t, isValidArch("arm64"))
	assert.False(t, isValidArch("x86"))
	assert.False(t, isValidArch(""))
}

// ─── buildFilename ────────────────────────────────────────────────────────

func TestBuildFilename(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		lang     CLILanguage
		os, arch string
		want     string
	}{
		{"Go darwin amd64", CLILangGo, "darwin", "amd64", "guliangiam-cli-go-darwin-amd64"},
		{"Go windows amd64", CLILangGo, "windows", "amd64", "guliangiam-cli-go-windows-amd64.exe"},
		{"Python", CLILangPython, "linux", "amd64", "guliangiam-cli-python.whl"},
		{"Java", CLILangJava, "linux", "amd64", "guliangiam-cli-java.jar"},
		{"CSharp", CLILangCSharp, "linux", "amd64", "guliangiam-cli-csharp.dll"},
		{"NodeJS", CLILangNodeJS, "linux", "amd64", "guliangiam-cli-nodejs-linux-amd64"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filename := buildFilename(tt.lang, tt.os, tt.arch)
			assert.Equal(t, tt.want, filename)
		})
	}
}

// ─── lookupSHA256 ─────────────────────────────────────────────────────────

func TestLookupSHA256(t *testing.T) {
	t.Parallel()
	// Known platform should return a hash
	hash := lookupSHA256(CLILangGo, "darwin", "amd64")
	assert.NotEmpty(t, hash)

	// Python uses any-any
	hash = lookupSHA256(CLILangPython, "linux", "amd64")
	assert.NotEmpty(t, hash)

	// Unknown language returns empty
	hash = lookupSHA256(CLILanguage("unknown"), "linux", "amd64")
	assert.Empty(t, hash)
}
