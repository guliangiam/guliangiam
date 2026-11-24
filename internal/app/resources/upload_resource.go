package resources

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"path"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/resources"
	platformconfig "github.com/guliangiam/guliangiam/internal/platform/config"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

// UploadResourceCommand contains the parameters for uploading a file.
type UploadResourceCommand struct {
	Owner    string // trusted organization ID or platform-admin-selected slug
	User     string // username or user ID
	Tag      string // avatar, logo, favicon, idCard, etc.
	FileName string // original file name
	FileSize int64
	MimeType string
	Reader   io.Reader
}

// UploadResource stores the file via the storage provider and creates a Resource record.
// It returns the URL of the uploaded file.
func (s Service) UploadResource(ctx context.Context, command UploadResourceCommand) (string, error) {
	// A service with neither a resolver nor a deployment backend cannot upload
	// anywhere; report that before validating request fields so a misconfigured
	// deployment stays a 503 rather than a field error.
	if s.Storage == nil && s.StorageResolver == nil {
		return "", apperrors.NewUnavailable("storage provider not configured")
	}

	organizationID, err := s.resolveOrganizationID(ctx, command.Owner)
	if err != nil {
		return "", err
	}

	storageProvider, err := s.resolveStorageProvider(ctx, organizationID)
	if err != nil {
		return "", err
	}

	id, err := s.UUID.NewString()
	if err != nil {
		return "", err
	}
	filePath, err := uploadObjectKey(organizationID, id, command.FileName)
	if err != nil {
		return "", err
	}

	// Store the file via storage provider.
	if err := storageProvider.PutStream(ctx, filePath, command.Reader, command.MimeType); err != nil {
		return "", apperrors.WrapInternal(err, "upload resource")
	}

	// Determine resource type from mime type or extension.
	resourceType := detectType(command.MimeType, filePath)

	now := s.Clock.Now()

	resource := &domain.Resource{
		ID:             id,
		OrganizationID: command.Owner,
		Name:           filePath,
		DisplayName:    command.FileName,
		Type:           resourceType,
		Tag:            command.Tag,
		FilePath:       filePath,
		FileSize:       command.FileSize,
		MimeType:       command.MimeType,
		CreatedAt:      now,
		UpdatedAt:      now,
		Revision:       1,
	}

	if err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		resource.OrganizationID = organizationID
		slog.InfoContext(ctx, "upload: org resolved", "owner", command.Owner, "organization_id", organizationID)

		if err := tx.Resources().Create(ctx, resource); err != nil {
			slog.ErrorContext(ctx, "upload: Resources().Create FAILED", "error", err)
			return err
		}
		slog.InfoContext(ctx, "upload: resource created, now audit", "resource_id", resource.ID)

		if err := s.embedAuditEvent(
			tx,
			ctx,
			resource.OrganizationID,
			"resources.created",
			resource.ID,
			"resources.created",
			map[string]any{},
		); err != nil {
			slog.ErrorContext(ctx, "upload: embedAuditEvent FAILED", "error", err)
			return err
		}
		return nil
	}); err != nil {
		return "", apperrors.WrapInternal(err, "upload resource create record")
	}

	// Return a presigned URL for the stored object. The lifetime comes from
	// STORAGE_PRESIGN_PUT_TTL_SECONDS so read and direct-upload presigns share
	// one operator-facing knob (手册 4.17.2).
	url, err := storageProvider.GetURL(ctx, filePath, platformconfig.PresignPutTTL())
	if err != nil {
		return "", apperrors.WrapInternal(err, "upload resource get url")
	}

	return url, nil
}

func uploadObjectKey(organizationID, id, fileName string) (string, error) {
	if !platformuuid.IsValid(organizationID) {
		return "", apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "organizationId must be a UUID", map[string]any{
			"field": "organizationId", "value": organizationID, "format": "uuid",
		})
	}
	if !platformuuid.IsValid(id) {
		return "", apperrors.NewInternal("generated upload resource ID is invalid")
	}

	extension := strings.ToLower(filepath.Ext(fileName))
	if !platformconfig.IsAllowedUploadExtension(extension) {
		return "", apperrors.NewUnprocessableEntityf("file extension %q is not an allowed upload extension", extension)
	}

	key := path.Join("organizations", organizationID, "uploads", id+extension)
	if path.IsAbs(key) || path.Clean(key) != key || key == "." ||
		strings.Contains(key, "\\") || strings.HasPrefix(key, "../") {
		return "", apperrors.NewInvalidArgument("invalid storage object key")
	}
	return key, nil
}

// resolveOrganizationID turns the console's org slug or UUID into the UUID used
// by both the storage resolution and the resource row.
func (s Service) resolveOrganizationID(ctx context.Context, owner string) (string, error) {
	owner = strings.TrimSpace(owner)
	if owner == "" {
		return "", apperrors.NewInvalidArgument("owner is required")
	}
	if _, err := uuid.Parse(owner); err == nil {
		return owner, nil
	}
	org, err := s.UOW.Organizations().GetBySlug(ctx, owner)
	if err != nil {
		slog.ErrorContext(ctx, "upload: resolve org slug failed", "slug", owner, "error", err)
		return "", apperrors.WrapInternalf(err, "resolve organization owner %q", owner)
	}
	return org.ID, nil
}

// resolveStorageProvider prefers the organization's enabled storage provider
// row and falls back to the deployment backend only when the organization has
// no row. A disabled or malformed row is returned as an unavailable error so
// the upload fails closed instead of silently using another tenant's storage.
func (s Service) resolveStorageProvider(ctx context.Context, organizationID string) (domain.StorageProvider, error) {
	if s.StorageResolver != nil {
		provider, err := s.StorageResolver.Resolve(ctx, organizationID)
		if err == nil {
			return provider, nil
		}
		if !errors.Is(err, domain.ErrStorageProviderUnconfigured) {
			return nil, err
		}
	}
	if s.Storage == nil {
		return nil, apperrors.NewUnavailable("storage provider not configured")
	}
	return s.Storage, nil
}

// detectType determines the resource type from MIME type or file extension.
func detectType(mimeType, filePath string) string {
	if strings.HasPrefix(mimeType, "image/") {
		return "image"
	}
	if strings.HasPrefix(mimeType, "video/") {
		return "video"
	}
	if strings.HasPrefix(mimeType, "audio/") {
		return "audio"
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".css":
		return "css"
	case ".js":
		return "js"
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg", ".bmp", ".ico":
		return "image"
	case ".mp4", ".webm", ".avi", ".mov":
		return "video"
	case ".mp3", ".wav", ".ogg", ".flac":
		return "audio"
	default:
		return "file"
	}
}
