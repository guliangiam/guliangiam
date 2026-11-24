package records

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	"go.uber.org/mock/gomock"
)

func wantRecordError(t *testing.T, err error, code apperrors.Code, status int) *apperrors.Error {
	t.Helper()
	var appErr *apperrors.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("expected *apperrors.Error, got %T (%v)", err, err)
	}
	if appErr.Code != code {
		t.Fatalf("code = %q, want %q", appErr.Code, code)
	}
	if appErr.HTTPStatus() != status {
		t.Fatalf("HTTPStatus() = %d, want %d", appErr.HTTPStatus(), status)
	}
	return appErr
}

func validCreateRecordCommand() CreateRecordCommand {
	return CreateRecordCommand{Action: "user.login", ResourceType: "session"}
}

func TestValidateCreateRecordCommand_ActionRequired(t *testing.T) {
	if err := validateCreateRecordCommand(validCreateRecordCommand()); err != nil {
		t.Fatalf("valid command rejected: %v", err)
	}

	for name, action := range map[string]string{"missing": "", "blank": "   "} {
		t.Run(name, func(t *testing.T) {
			command := validCreateRecordCommand()
			command.Action = action
			appErr := wantRecordError(t, validateCreateRecordCommand(command), apperrors.CodeUnprocessableEntity, 422)
			if appErr.Meta["field"] != "action" {
				t.Fatalf("meta = %+v, want field=action", appErr.Meta)
			}
		})
	}
}

func TestValidateCreateRecordCommand_ResourceTypeRequired(t *testing.T) {
	if err := validateCreateRecordCommand(validCreateRecordCommand()); err != nil {
		t.Fatalf("valid command rejected: %v", err)
	}

	for name, resourceType := range map[string]string{"missing": "", "blank": "\t"} {
		t.Run(name, func(t *testing.T) {
			command := validCreateRecordCommand()
			command.ResourceType = resourceType
			appErr := wantRecordError(t, validateCreateRecordCommand(command), apperrors.CodeUnprocessableEntity, 422)
			if appErr.Meta["field"] != "resourceType" {
				t.Fatalf("meta = %+v, want field=resourceType", appErr.Meta)
			}
		})
	}
}

func TestValidateCreateRecordCommand_ResourceIDLength(t *testing.T) {
	atLimit := validCreateRecordCommand()
	atLimit.ResourceID = strings.Repeat("r", maxResourceIDLength)
	if err := validateCreateRecordCommand(atLimit); err != nil {
		t.Fatalf("resourceId at limit rejected: %v", err)
	}

	overLimit := validCreateRecordCommand()
	overLimit.ResourceID = strings.Repeat("r", maxResourceIDLength+1)
	appErr := wantRecordError(t, validateCreateRecordCommand(overLimit), apperrors.CodeUnprocessableEntity, 422)
	if appErr.Meta["field"] != "resourceId" || appErr.Meta["max"] != maxResourceIDLength {
		t.Fatalf("meta = %+v, want field=resourceId max=%d", appErr.Meta, maxResourceIDLength)
	}
}

func TestValidateCreateRecordCommand_UserAgentLength(t *testing.T) {
	atLimit := validCreateRecordCommand()
	atLimit.UserAgent = strings.Repeat("u", maxUserAgentLength)
	if err := validateCreateRecordCommand(atLimit); err != nil {
		t.Fatalf("userAgent at limit rejected: %v", err)
	}

	overLimit := validCreateRecordCommand()
	overLimit.UserAgent = strings.Repeat("u", maxUserAgentLength+1)
	appErr := wantRecordError(t, validateCreateRecordCommand(overLimit), apperrors.CodeUnprocessableEntity, 422)
	if appErr.Meta["field"] != "userAgent" || appErr.Meta["max"] != maxUserAgentLength {
		t.Fatalf("meta = %+v, want field=userAgent max=%d", appErr.Meta, maxUserAgentLength)
	}
}

func TestValidateCreateRecordCommand_MetadataSize(t *testing.T) {
	envelope := `{"k":""}`
	atLimit := validCreateRecordCommand()
	atLimit.Metadata = map[string]any{"k": strings.Repeat("m", maxMetadataBytes-len(envelope))}
	payload, err := json.Marshal(atLimit.Metadata)
	if err != nil {
		t.Fatalf("marshal at-limit metadata: %v", err)
	}
	if len(payload) != maxMetadataBytes {
		t.Fatalf("test fixture = %d bytes, want %d", len(payload), maxMetadataBytes)
	}
	if err := validateCreateRecordCommand(atLimit); err != nil {
		t.Fatalf("metadata at limit rejected: %v", err)
	}

	overLimit := validCreateRecordCommand()
	overLimit.Metadata = map[string]any{"k": strings.Repeat("m", maxMetadataBytes-len(envelope)+1)}
	appErr := wantRecordError(t, validateCreateRecordCommand(overLimit), apperrors.CodeUnprocessableEntity, 422)
	if appErr.Meta["field"] != "metadata" || appErr.Meta["max"] != maxMetadataBytes {
		t.Fatalf("meta = %+v, want field=metadata max=%d", appErr.Meta, maxMetadataBytes)
	}
}

func TestValidateCreateRecordCommand_MetadataNotSerializable(t *testing.T) {
	command := validCreateRecordCommand()
	command.Metadata = map[string]any{"bad": make(chan int)}
	appErr := wantRecordError(t, validateCreateRecordCommand(command), apperrors.CodeUnprocessableEntity, 422)
	if appErr.Meta["field"] != "metadata" {
		t.Fatalf("meta = %+v, want field=metadata", appErr.Meta)
	}
}

func TestCreateRecord_RejectsInvalidCommandBeforeRepositoryWrite(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	// No Records() expectation: a repository call would fail the test.
	uow := appuowmock.NewMockUnitOfWork(ctrl)

	_, err := Service{
		Clock: fakeClock{fixedTime: time.Unix(0, 0)},
		UUID:  fakeUUIDGenerator{fixedID: "rec-invalid"},
		UOW:   uow,
	}.CreateRecord(context.Background(), CreateRecordCommand{ResourceType: "session"})

	appErr := wantRecordError(t, err, apperrors.CodeUnprocessableEntity, 422)
	if appErr.Meta["field"] != "action" {
		t.Fatalf("meta = %+v, want field=action", appErr.Meta)
	}
}
