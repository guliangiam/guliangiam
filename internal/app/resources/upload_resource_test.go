package resources

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/require"
)

type uploadPathCaptureStorage struct {
	mockStorage
	key string
}

func (s *uploadPathCaptureStorage) PutStream(_ context.Context, key string, _ io.Reader, _ string) error {
	s.key = key
	return errors.New("stop before database write")
}

func TestUploadResourceNeverUsesClientFilePath(t *testing.T) {
	const (
		organizationID = "018f47b2-0000-7000-8000-000000000001"
		resourceID     = "01924f00-0000-7000-8000-000000000002"
	)
	storage := &uploadPathCaptureStorage{}
	service := Service{
		UUID:    fakeUUIDGenerator{fixedID: resourceID},
		Storage: storage,
	}

	_, err := service.UploadResource(context.Background(), UploadResourceCommand{
		Owner:    organizationID,
		FileName: "../../other-tenant/overwrite.png",
		MimeType: "image/png",
		Reader:   strings.NewReader("content"),
	})
	require.Error(t, err)
	require.Equal(t,
		"organizations/"+organizationID+"/uploads/"+resourceID+".png",
		storage.key,
	)
}

func TestUploadObjectKeyUsesTenantAndGeneratedID(t *testing.T) {
	key, err := uploadObjectKey(
		"018f47b2-0000-7000-8000-000000000001",
		"01924f00-0000-7000-8000-000000000002",
		"../../attacker.png",
	)
	require.NoError(t, err)
	require.Equal(t,
		"organizations/018f47b2-0000-7000-8000-000000000001/uploads/01924f00-0000-7000-8000-000000000002.png",
		key,
	)
}

func TestUploadObjectKeyRejectsUnsafeIdentifiersAndExtensions(t *testing.T) {
	tests := []struct {
		name           string
		organizationID string
		id             string
		fileName       string
		wantCode       apperrors.Code
	}{
		{
			name:           "invalid organization ID",
			organizationID: "../../tenant",
			id:             "01924f00-0000-7000-8000-000000000002",
			fileName:       "image.png",
			wantCode:       apperrors.CodeUnprocessableEntity,
		},
		{
			name:           "invalid generated ID",
			organizationID: "018f47b2-0000-7000-8000-000000000001",
			id:             "../object",
			fileName:       "image.png",
			wantCode:       apperrors.CodeInternal,
		},
		{
			name:           "disallowed extension",
			organizationID: "018f47b2-0000-7000-8000-000000000001",
			id:             "01924f00-0000-7000-8000-000000000002",
			fileName:       "image.svg",
			wantCode:       apperrors.CodeUnprocessableEntity,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := uploadObjectKey(tt.organizationID, tt.id, tt.fileName)
			require.Error(t, err)
			var appErr *apperrors.Error
			require.ErrorAs(t, err, &appErr)
			require.Equal(t, tt.wantCode, appErr.Code)
		})
	}
}
