package system

import (
	"context"
	"fmt"
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

const (
	cliReleaseOwner = "guliangtech"
	cliReleaseRepo  = "guliangiam-cli"
	cliVersion      = "v0.0.1"
	cliReleaseBase  = "https://github.com/" + cliReleaseOwner + "/" + cliReleaseRepo + "/releases/download/" + cliVersion
)

type CLILanguage string

const (
	CLILangGo     CLILanguage = "go"
	CLILangPython CLILanguage = "python"
	CLILangJava   CLILanguage = "java"
	CLILangNodeJS CLILanguage = "nodejs"
	CLILangCSharp CLILanguage = "csharp"
)

type Platform struct {
	OS   string
	Arch string
}

type CLIArtifact struct {
	DownloadURL string `json:"downloadUrl"`
	SHA256      string `json:"sha256"`
	Filename    string `json:"filename"`
}

var supportedLanguages = map[CLILanguage]bool{
	CLILangGo:     true,
	CLILangPython: true,
	CLILangJava:   true,
	CLILangNodeJS: true,
	CLILangCSharp: true,
}

var languageArchiveExt = map[CLILanguage]func(os, arch string) string{
	CLILangGo: func(os, arch string) string {
		if os == osWindows {
			return ".exe"
		}
		return ""
	},
	CLILangPython: func(os, arch string) string {
		return ".whl"
	},
	CLILangJava: func(os, arch string) string {
		return ".jar"
	},
	CLILangNodeJS: func(os, arch string) string {
		if os == osWindows {
			return ".exe"
		}
		return ""
	},
	CLILangCSharp: func(os, arch string) string {
		return ".dll"
	},
}

var languagePrefix = map[CLILanguage]string{
	CLILangGo:     "guliangiam-cli-go",
	CLILangPython: "guliangiam-cli-python",
	CLILangJava:   "guliangiam-cli-java",
	CLILangNodeJS: "guliangiam-cli-nodejs",
	CLILangCSharp: "guliangiam-cli-csharp",
}

var knownSHA256 = map[CLILanguage]map[string]string{
	CLILangGo: {
		"darwin-amd64":  "placeholder-sha256-go-darwin-amd64",
		"darwin-arm64":  "placeholder-sha256-go-darwin-arm64",
		"linux-amd64":   "placeholder-sha256-go-linux-amd64",
		"linux-arm64":   "placeholder-sha256-go-linux-arm64",
		"windows-amd64": "placeholder-sha256-go-windows-amd64",
		"windows-arm64": "placeholder-sha256-go-windows-arm64",
	},
	CLILangPython: {
		platformAny: "placeholder-sha256-python-any",
	},
	CLILangJava: {
		platformAny: "placeholder-sha256-java-any",
	},
	CLILangNodeJS: {
		"darwin-amd64":  "placeholder-sha256-nodejs-darwin-amd64",
		"darwin-arm64":  "placeholder-sha256-nodejs-darwin-arm64",
		"linux-amd64":   "placeholder-sha256-nodejs-linux-amd64",
		"linux-arm64":   "placeholder-sha256-nodejs-linux-arm64",
		"windows-amd64": "placeholder-sha256-nodejs-windows-amd64",
		"windows-arm64": "placeholder-sha256-nodejs-windows-arm64",
	},
	CLILangCSharp: {
		platformAny: "placeholder-sha256-csharp-any",
	},
}

type CLIService struct{}

func NewCLIService() CLIService {
	return CLIService{}
}

func (s CLIService) DownloadCLI(_ context.Context, language string, platform Platform) (*CLIArtifact, error) {
	lang := CLILanguage(strings.ToLower(language))
	if !supportedLanguages[lang] {
		return nil, apperrors.NewInvalidArgumentf("unsupported CLI language: %s (supported: go, python, java, nodejs, csharp)", language)
	}

	os, arch := normalizePlatform(platform.OS, platform.Arch)

	if !isValidOS(os) {
		return nil, apperrors.NewInvalidArgumentf("unsupported OS: %s (supported: darwin, linux, windows)", os)
	}
	if !isValidArch(arch) {
		return nil, apperrors.NewInvalidArgumentf("unsupported architecture: %s (supported: amd64, arm64)", arch)
	}

	filename := buildFilename(lang, os, arch)
	downloadURL := fmt.Sprintf("%s/%s", cliReleaseBase, filename)
	sha256 := lookupSHA256(lang, os, arch)

	return &CLIArtifact{
		DownloadURL: downloadURL,
		SHA256:      sha256,
		Filename:    filename,
	}, nil
}

func normalizePlatform(os, arch string) (platformName, archName string) {
	os = strings.ToLower(os)
	arch = strings.ToLower(arch)

	switch os {
	case "macos":
		os = osDarwin
	case "mac":
		os = osDarwin
	}

	switch arch {
	case "x86_64":
		arch = archAMD64
	case "x64":
		arch = archAMD64
	case "aarch64":
		arch = archARM64
	}

	return os, arch
}

func isValidOS(os string) bool {
	switch os {
	case osDarwin, osLinux, osWindows:
		return true
	default:
		return false
	}
}

func isValidArch(arch string) bool {
	switch arch {
	case archAMD64, archARM64:
		return true
	default:
		return false
	}
}

func buildFilename(lang CLILanguage, os, arch string) string {
	prefix := languagePrefix[lang]
	extFn := languageArchiveExt[lang]

	switch lang {
	case CLILangPython, CLILangJava, CLILangCSharp:
		return prefix + extFn(os, arch)
	case CLILangGo, CLILangNodeJS:
		return fmt.Sprintf("%s-%s-%s%s", prefix, os, arch, extFn(os, arch))
	default:
		return fmt.Sprintf("%s-%s-%s%s", prefix, os, arch, extFn(os, arch))
	}
}

func lookupSHA256(lang CLILanguage, os, arch string) string {
	if checksums, ok := knownSHA256[lang]; ok {
		platformKey := os + "-" + arch
		if hash, ok := checksums[platformKey]; ok {
			return hash
		}
		if hash, ok := checksums[platformAny]; ok {
			return hash
		}
	}
	return ""
}

func DetectPlatformFromUserAgent(ua string) Platform {
	os, arch := osLinux, archAMD64

	uaLower := strings.ToLower(ua)

	switch {
	case strings.Contains(uaLower, osWindows):
		os = osWindows
	case strings.Contains(uaLower, "mac") || strings.Contains(uaLower, osDarwin):
		os = osDarwin
	case strings.Contains(uaLower, osLinux):
		os = osLinux
	}

	switch {
	case strings.Contains(uaLower, archARM64) || strings.Contains(uaLower, "aarch64"):
		arch = archARM64
	case strings.Contains(uaLower, "x86_64") || strings.Contains(uaLower, "x64") || strings.Contains(uaLower, archAMD64):
		arch = archAMD64
	}

	return Platform{OS: os, Arch: arch}
}
