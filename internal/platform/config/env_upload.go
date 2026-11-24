package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// WebAuthnRequireUV 报告 WebAuthn ceremony 是否强制用户验证（UV，《开发设计》
// 31.3.2(b)）。默认 false：存量认证器里有只做 UP 的形态（例如没有 PIN 的安全密钥），
// 默认强制会让这些用户突然无法登录；把 WebAuthn 用作第二因子或 MFA 替代的部署必须
// 显式设为 true——UP 只证明「有人碰了认证器」，UV 才证明「是本人」。
func WebAuthnRequireUV() bool {
	value, ok := parseBoolLiteral(os.Getenv("WEBAUTHN_REQUIRE_UV"))
	return ok && value
}

// MIME types shared by the extension binding (see uploadExtensionMime) and the
// raster-bitmap test (see isBitmapMimeType), kept as constants so a typo cannot
// diverge the two contract checks.
const (
	mimeImageJPEG      = "image/jpeg"
	mimeImagePNG       = "image/png"
	mimeImageGIF       = "image/gif"
	mimeImageWebP      = "image/webp"
	mimeApplicationPDF = "application/pdf"
)

// defaultUploadMaxBytes is the 32 MiB single-file cap the upload contract names
// (手册 4.17.2) when STORAGE_UPLOAD_MAX_BYTES is unset or malformed.
const defaultUploadMaxBytes int64 = 32 << 20

// UploadMaxBytes resolves the single-file upload size cap (手册 4.17.2, 环境变量注册表
// D5_storage). Requests over the cap must be rejected with 413.
func UploadMaxBytes() int64 {
	if v := strings.TrimSpace(os.Getenv("STORAGE_UPLOAD_MAX_BYTES")); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			return n
		}
	}
	return defaultUploadMaxBytes
}

// defaultUploadAllowedMimeTypes is the whitelist the registry fixes for
// UPLOAD_ALLOWED_MIME_TYPES (手册 4.17.2).
const defaultUploadAllowedMimeTypes = "image/jpeg,image/png,image/gif,image/webp,application/pdf"

// UploadAllowedMimeTypes resolves the accepted upload MIME whitelist, lowercased
// and without parameters. The registry default applies when unset.
func UploadAllowedMimeTypes() map[string]struct{} {
	raw := strings.TrimSpace(os.Getenv("UPLOAD_ALLOWED_MIME_TYPES"))
	if raw == "" {
		raw = defaultUploadAllowedMimeTypes
	}
	allowed := make(map[string]struct{})
	for _, item := range strings.Split(raw, ",") {
		if mime := normalizeMimeType(item); mime != "" {
			allowed[mime] = struct{}{}
		}
	}
	return allowed
}

// IsAllowedUploadMimeType reports whether the declared Content-Type passes the
// UPLOAD_ALLOWED_MIME_TYPES whitelist (手册 4.17.2).
func IsAllowedUploadMimeType(mimeType string) bool {
	_, ok := UploadAllowedMimeTypes()[normalizeMimeType(mimeType)]
	return ok
}

// normalizeMimeType lowercases a MIME type and drops any parameters
// (e.g. "image/png; charset=binary" -> "image/png").
func normalizeMimeType(mimeType string) string {
	if i := strings.IndexByte(mimeType, ';'); i >= 0 {
		mimeType = mimeType[:i]
	}
	return strings.ToLower(strings.TrimSpace(mimeType))
}

// uploadExtensionMime binds a file extension to the MIME type the upload
// contract accepts for it (手册 4.17.2 ② 扩展名白名单). An extension absent from
// this map is never acceptable even when an operator widens
// UPLOAD_ALLOWED_MIME_TYPES: an unknown extension has no magic-number contract
// to sniff against.
var uploadExtensionMime = map[string]string{
	"jpg":  mimeImageJPEG,
	"jpeg": mimeImageJPEG,
	"png":  mimeImagePNG,
	"gif":  mimeImageGIF,
	"webp": mimeImageWebP,
	"pdf":  mimeApplicationPDF,
}

// UploadMimeTypeForExtension returns the MIME type bound to ext (with or without
// the leading dot), or "" when the extension is not recognised.
func UploadMimeTypeForExtension(ext string) string {
	ext = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(ext), "."))
	return uploadExtensionMime[ext]
}

// IsAllowedUploadExtension reports whether ext is a recognised extension whose
// bound MIME type passes UPLOAD_ALLOWED_MIME_TYPES (手册 4.17.2 ②).
func IsAllowedUploadExtension(ext string) bool {
	mimeType := UploadMimeTypeForExtension(ext)
	return mimeType != "" && IsAllowedUploadMimeType(mimeType)
}

// activeContentMimeTypes is the default-deny set from 手册 4.17.2「活跃内容」:
// types whose bytes can execute script when a browser renders them. They are
// refused even when an operator adds them to UPLOAD_ALLOWED_MIME_TYPES. SVG is
// refused because the repository ships no SVG sanitiser, and the contract only
// permits SVG after sanitisation and attachment-only delivery.
var activeContentMimeTypes = map[string]struct{}{
	"text/html":                {},
	"image/svg+xml":            {},
	"application/javascript":   {},
	"text/javascript":          {},
	"application/x-javascript": {},
	"application/x-msdownload": {},
}

// IsActiveContentMimeType reports whether mimeType is one the upload contract
// refuses by default (手册 4.17.2「活跃内容」).
func IsActiveContentMimeType(mimeType string) bool {
	_, deny := activeContentMimeTypes[normalizeMimeType(mimeType)]
	return deny
}

// isBitmapMimeType reports whether mimeType is a raster image type that cannot
// carry script. 手册 4.17.2 allows only these to be served inline.
func isBitmapMimeType(mimeType string) bool {
	switch normalizeMimeType(mimeType) {
	case mimeImageJPEG, mimeImagePNG, mimeImageGIF, mimeImageWebP:
		return true
	default:
		return false
	}
}

// UploadMimeConsistent reports whether the client-declared, magic-sniffed and
// extension-bound MIME types agree (手册 4.17.2 ③). The declared type must equal
// the type the extension binds to; the sniffed type must equal them too, except
// when all three are raster image bitmaps. That one relaxation is required by
// the bundled crop widget: it re-encodes pixels through a canvas (PNG bytes)
// while preserving the original file name and declared type, so a strict
// three-way identity would reject every non-PNG avatar/logo upload. A body whose
// bytes are HTML/JS/SVG renamed to an image extension still fails, because the
// sniffed type is not a bitmap and not on the whitelist at all.
func UploadMimeConsistent(declared, sniffed, extMime string) bool {
	declared = normalizeMimeType(declared)
	sniffed = normalizeMimeType(sniffed)
	extMime = normalizeMimeType(extMime)
	if declared == "" || extMime == "" || declared != extMime {
		return false
	}
	if sniffed == declared {
		return true
	}
	return isBitmapMimeType(sniffed) && isBitmapMimeType(declared)
}

// defaultAttachmentExtensions is the active-content extension set the registry
// fixes for STORAGE_ATTACHMENT_EXTENSIONS (手册 4.17.2).
const defaultAttachmentExtensions = "svg,html,htm,js"

// IsAttachmentExtension reports whether an extension (with or without the
// leading dot) belongs to STORAGE_ATTACHMENT_EXTENSIONS. Files carrying one of
// these extensions must only be offered as downloads, never inline.
func IsAttachmentExtension(ext string) bool {
	ext = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(ext), "."))
	if ext == "" {
		return false
	}
	raw := strings.TrimSpace(os.Getenv("STORAGE_ATTACHMENT_EXTENSIONS"))
	if raw == "" {
		raw = defaultAttachmentExtensions
	}
	for _, item := range strings.Split(raw, ",") {
		if strings.ToLower(strings.TrimPrefix(strings.TrimSpace(item), ".")) == ext {
			return true
		}
	}
	return false
}

// defaultPresignTTL is the presigned-URL lifetime fallback the registry fixes for
// STORAGE_PRESIGN_PUT_TTL_SECONDS (手册 4.17.2).
const defaultPresignTTL = 5 * time.Minute

// PresignPutTTL resolves STORAGE_PRESIGN_PUT_TTL_SECONDS (seconds), the lifetime
// the contract assigns to presigned direct-upload URLs. It is used wherever a
// presigned URL is minted without an explicit expiry; empty or malformed values
// fall back to the registry default.
func PresignPutTTL() time.Duration {
	if v := strings.TrimSpace(os.Getenv("STORAGE_PRESIGN_PUT_TTL_SECONDS")); v != "" {
		if seconds, err := strconv.Atoi(v); err == nil && seconds > 0 {
			return time.Duration(seconds) * time.Second
		}
	}
	return defaultPresignTTL
}
