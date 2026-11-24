package users

import (
	"context"
	"encoding/base64"
	"testing"
	"time"

	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type fakeFaceIDGateway struct {
	called  bool
	request FaceIDEnrollmentRequest
	faceID  userdomain.FaceId
	err     error
}

func (g *fakeFaceIDGateway) EnrollFaceID(_ context.Context, request FaceIDEnrollmentRequest) (userdomain.FaceId, error) {
	g.called = true
	g.request = request
	if g.err != nil {
		return userdomain.FaceId{}, g.err
	}
	return g.faceID, nil
}

type fakeFaceTemplateRepo struct {
	enrolled  *userdomain.FaceTemplate
	enrollErr error
	active    *userdomain.FaceTemplate
	getErr    error
	revoked   bool
	revokeErr error
}

func (r *fakeFaceTemplateRepo) Enroll(_ context.Context, template *userdomain.FaceTemplate) error {
	if r.enrollErr != nil {
		return r.enrollErr
	}
	stored := *template
	r.enrolled = &stored
	return nil
}

func (r *fakeFaceTemplateRepo) GetActiveByUser(_ context.Context, _, _ string) (*userdomain.FaceTemplate, error) {
	if r.getErr != nil {
		return nil, r.getErr
	}
	if r.active == nil {
		return nil, apperrors.NewNotFound("no active face template for user")
	}
	return r.active, nil
}

func (r *fakeFaceTemplateRepo) RevokeByUser(_ context.Context, _, _ string, _ time.Time) error {
	if r.revokeErr != nil {
		return r.revokeErr
	}
	r.revoked = true
	return nil
}

func enrolledFaceID() userdomain.FaceId {
	return userdomain.FaceId{ID: "face-2", CreatedAt: "2026-11-12T10:00:00Z"}
}

func verifiedUserRepo() *mockUserRepo {
	return &mockUserRepo{
		getByIDForOrganizationFn: func(_ context.Context, id, organizationID string) (*userdomain.User, error) {
			return &userdomain.User{ID: id, OrganizationID: organizationID, IsVerified: true}, nil
		},
	}
}

func TestEnrollUserFaceID_SealsTemplateAndAudits(t *testing.T) {
	var captured userdomain.UserCollections
	userRepo := verifiedUserRepo()
	userRepo.updateCollectionsFn = func(_ context.Context, _, _ string, collections userdomain.UserCollections) error {
		captured = collections
		return nil
	}
	templates := &fakeFaceTemplateRepo{}
	gateway := &fakeFaceIDGateway{faceID: enrolledFaceID()}
	records := &fakeRecordsRepo{}
	svc := Service{
		Clock:            svcFakeClock{fixedTime: time.Now()},
		UOW:              testUnitOfWork{users: userRepo, records: records, faceTemplates: templates},
		FaceIDEnrollment: gateway,
	}

	faceID, err := svc.EnrollUserFaceID(context.Background(), FaceIDEnrollmentRequest{
		OrganizationID: "org-1",
		UserID:         testUserID,
		MimeType:       "image/jpeg",
		Data:           []byte("jpeg-bytes"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if faceID.ID != "face-2" {
		t.Fatalf("returned face = %+v", faceID)
	}
	if !gateway.called || gateway.request.OrganizationID != "org-1" {
		t.Fatalf("gateway request = %+v", gateway.request)
	}
	if templates.enrolled == nil {
		t.Fatal("expected an encrypted face template to be enrolled")
	}
	if templates.enrolled.TemplateRef != base64.StdEncoding.EncodeToString([]byte("jpeg-bytes")) {
		t.Fatalf("template ref = %q, want base64 of the captured image", templates.enrolled.TemplateRef)
	}
	if templates.enrolled.UserID != testUserID || templates.enrolled.Status != userdomain.FaceTemplateStatusActive {
		t.Fatalf("template binding = %+v", templates.enrolled)
	}
	// Legacy JSONB keeps a single non-sensitive marker, never a plaintext URL.
	if len(captured.FaceIds) != 1 || captured.FaceIds[0].ID != "face-2" || captured.FaceIds[0].ImageUrl != "" {
		t.Fatalf("stored face ids = %+v, want one marker without image url", captured.FaceIds)
	}
	if len(records.created) != 1 || records.created[0].Action != "user.updated" {
		t.Fatalf("audit records = %+v, want one user.updated", records.created)
	}
}

func TestEnrollUserFaceID_GatewayNotWired(t *testing.T) {
	svc := Service{UOW: testUnitOfWork{users: verifiedUserRepo()}}

	_, err := svc.EnrollUserFaceID(context.Background(), FaceIDEnrollmentRequest{
		OrganizationID: "org-1", UserID: testUserID, Data: []byte("x"),
	})
	if appErr, ok := err.(*apperrors.Error); !ok || appErr.Code != apperrors.CodeUnavailable {
		t.Fatalf("expected Unavailable, got %v", err)
	}
}

func TestEnrollUserFaceID_RejectsWhenNotRealNameVerified(t *testing.T) {
	userRepo := &mockUserRepo{
		getByIDForOrganizationFn: func(_ context.Context, id, organizationID string) (*userdomain.User, error) {
			return &userdomain.User{ID: id, OrganizationID: organizationID, IsVerified: false}, nil
		},
	}
	templates := &fakeFaceTemplateRepo{}
	gateway := &fakeFaceIDGateway{faceID: enrolledFaceID()}
	svc := Service{UOW: testUnitOfWork{users: userRepo, faceTemplates: templates}, FaceIDEnrollment: gateway}

	_, err := svc.EnrollUserFaceID(context.Background(), FaceIDEnrollmentRequest{
		OrganizationID: "org-1", UserID: testUserID, Data: []byte("x"),
	})
	if appErr, ok := err.(*apperrors.Error); !ok || appErr.Code != apperrors.CodeUnprocessableEntity {
		t.Fatalf("expected UnprocessableEntity real-name gate, got %v", err)
	}
	if gateway.called {
		t.Fatal("provider must not be called for an unverified user")
	}
	if templates.enrolled != nil {
		t.Fatal("no template may be enrolled for an unverified user")
	}
}

func TestEnrollUserFaceID_GatewayFailureDoesNotPersist(t *testing.T) {
	userRepo := verifiedUserRepo()
	userRepo.updateCollectionsFn = func(context.Context, string, string, userdomain.UserCollections) error {
		t.Error("collections must not be updated when enrollment fails")
		return nil
	}
	templates := &fakeFaceTemplateRepo{}
	gateway := &fakeFaceIDGateway{err: apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "face liveness check failed", map[string]any{})}
	svc := Service{UOW: testUnitOfWork{users: userRepo, faceTemplates: templates}, FaceIDEnrollment: gateway}

	if _, err := svc.EnrollUserFaceID(context.Background(), FaceIDEnrollmentRequest{
		OrganizationID: "org-1", UserID: testUserID, Data: []byte("x"),
	}); err == nil {
		t.Fatal("expected liveness failure to surface")
	}
	if templates.enrolled != nil {
		t.Fatal("no template may be enrolled when liveness fails")
	}
}

func TestEnrollUserFaceID_CrossTenantUserRefusedBeforeProvider(t *testing.T) {
	userRepo := &mockUserRepo{
		getByIDForOrganizationFn: func(_ context.Context, _, _ string) (*userdomain.User, error) {
			return nil, apperrors.NewNotFound("user not found")
		},
	}
	gateway := &fakeFaceIDGateway{}
	svc := Service{UOW: testUnitOfWork{users: userRepo}, FaceIDEnrollment: gateway}

	if _, err := svc.EnrollUserFaceID(context.Background(), FaceIDEnrollmentRequest{
		OrganizationID: "org-1", UserID: "foreign-user", Data: []byte("x"),
	}); err == nil {
		t.Fatal("expected not found")
	}
	if gateway.called {
		t.Fatal("provider must not be called for a foreign user")
	}
}

func TestRevokeUserFaceID_MarksRevokedAndClearsMarker(t *testing.T) {
	var captured userdomain.UserCollections
	userRepo := verifiedUserRepo()
	userRepo.updateCollectionsFn = func(_ context.Context, _, _ string, collections userdomain.UserCollections) error {
		captured = collections
		return nil
	}
	templates := &fakeFaceTemplateRepo{active: &userdomain.FaceTemplate{ID: "face-2", UserID: testUserID, Status: userdomain.FaceTemplateStatusActive}}
	records := &fakeRecordsRepo{}
	svc := Service{
		Clock: svcFakeClock{fixedTime: time.Now()},
		UOW:   testUnitOfWork{users: userRepo, records: records, faceTemplates: templates},
	}

	if err := svc.RevokeUserFaceID(context.Background(), "org-1", testUserID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !templates.revoked {
		t.Fatal("expected the active template to be revoked")
	}
	if len(captured.FaceIds) != 0 {
		t.Fatalf("face ids = %+v, want cleared", captured.FaceIds)
	}
	if len(records.created) != 1 || records.created[0].Action != "user.updated" {
		t.Fatalf("audit records = %+v, want one user.updated", records.created)
	}
}
