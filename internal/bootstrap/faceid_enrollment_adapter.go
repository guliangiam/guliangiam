package bootstrap

import (
	"context"
	"encoding/base64"
	"strings"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appusers "github.com/guliangiam/guliangiam/internal/app/users"
	faceiddomain "github.com/guliangiam/guliangiam/internal/domain/faceidproviders"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

// faceIDEnrollmentAdapter connects the users application service to the real
// FaceVerifier infrastructure: liveness detection must pass before an enrollment
// identity is minted. Fail-closed: a disabled or missing provider refuses
// enrollment instead of accepting an unverified face. The adapter deliberately
// does NOT store the image — the application service seals the captured bytes
// into user_face_templates (encrypted reference) so no plaintext底图 is persisted
// through the resource storage channel.
type faceIDEnrollmentAdapter struct {
	UOW   appuow.UnitOfWork
	Clock clock.Clock
	UUID  platformuuid.Generator
}

func (a faceIDEnrollmentAdapter) EnrollFaceID(ctx context.Context, request appusers.FaceIDEnrollmentRequest) (userdomain.FaceId, error) {
	provider, err := a.resolveProvider(ctx, request)
	if err != nil {
		return userdomain.FaceId{}, err
	}

	if _, ok := faceImageExtension(request.MimeType); !ok {
		return userdomain.FaceId{}, apperrors.NewInvalidArgument("unsupported face image media type")
	}

	verifier := newFaceVerifier(provider.ProviderType, provider.AccessKeyID, provider.AccessKeySecret, provider.Endpoint)
	liveness, err := verifier.VerifyLiveness(ctx, base64.StdEncoding.EncodeToString(request.Data), request.DeviceInfo)
	if err != nil {
		return userdomain.FaceId{}, err
	}
	if !liveness.IsLive {
		return userdomain.FaceId{}, apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "face liveness check failed", map[string]any{"score": liveness.Score})
	}

	id, err := a.UUID.NewString()
	if err != nil {
		return userdomain.FaceId{}, apperrors.NewInternal("generate face id")
	}
	return userdomain.FaceId{
		ID:        id,
		CreatedAt: a.Clock.Now().Format(time.RFC3339),
	}, nil
}

func (a faceIDEnrollmentAdapter) resolveProvider(ctx context.Context, request appusers.FaceIDEnrollmentRequest) (*faceiddomain.FaceIdProvider, error) {
	if providerID := strings.TrimSpace(request.ProviderID); providerID != "" {
		provider, err := a.UOW.FaceIdProviders().GetByID(ctx, providerID, request.OrganizationID)
		if err != nil {
			return nil, err
		}
		if !provider.IsEnabled {
			return nil, apperrors.NewUnimplemented("selected face id provider is disabled")
		}
		return provider, nil
	}

	isEnabled := true
	providers, _, err := a.UOW.FaceIdProviders().List(ctx, faceiddomain.ListQuery{
		OrganizationID: request.OrganizationID,
		IsEnabled:      &isEnabled,
		Page:           1,
		PageSize:       1,
	})
	if err != nil {
		return nil, apperrors.WrapInternal(err, "query face id providers")
	}
	if len(providers) == 0 {
		return nil, apperrors.NewUnimplemented("no enabled face id provider for organization")
	}
	return providers[0], nil
}

// faceImageExtension maps the sniffed/declared image media type to a stored
// file extension. Only raster photo types accepted by face providers are
// allowed; everything else is refused before any provider call.
func faceImageExtension(mediaType string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(mediaType)) {
	case "image/jpeg", "image/jpg":
		return ".jpg", true
	case "image/png":
		return ".png", true
	case "image/webp":
		return ".webp", true
	case "image/gif":
		return ".gif", true
	default:
		return "", false
	}
}

var _ appusers.FaceIDEnrollmentGateway = (*faceIDEnrollmentAdapter)(nil)
