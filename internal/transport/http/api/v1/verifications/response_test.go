package verifications

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	uowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	appverifications "github.com/guliangiam/guliangiam/internal/app/verifications"
	domainverifications "github.com/guliangiam/guliangiam/internal/domain/verifications"
	verificationsmock "github.com/guliangiam/guliangiam/internal/domain/verifications/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestListVerifications_JSONHidesOTPCode(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	expiresAt := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)
	createdAt := expiresAt.Add(-10 * time.Minute)

	uowMock := uowmock.NewMockAuthUoW(ctrl)
	repo := verificationsmock.NewMockRepository(ctrl)
	uowMock.EXPECT().Verifications().Return(repo)
	repo.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*domainverifications.Verification{{
		ID:             "verification-1",
		OrganizationID: "org-1",
		Type:           "email",
		Recipient:      "user@example.com",
		Code:           "123456",
		Status:         "pending",
		ExpiresAt:      expiresAt,
		CreatedAt:      createdAt,
	}}, int64(1), nil)

	router := testRouter()
	RegisterChiRoutes(router, Handler{Service: appverifications.Service{UOW: uowMock}})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, forgedOrgRequest("/api/v1/verifications?page=1&pageSize=20"))
	resp := recorder.Result()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	defer resp.Body.Close()

	payload, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.NotContains(t, string(payload), "123456")
	assert.Contains(t, string(payload), `"code":""`)

	var body struct {
		Items []struct {
			ID             string    `json:"id"`
			OrganizationID string    `json:"organizationId"`
			Type           string    `json:"type"`
			Recipient      string    `json:"recipient"`
			Code           string    `json:"code"`
			Status         string    `json:"status"`
			ExpiresAt      time.Time `json:"expiresAt"`
			CreatedAt      time.Time `json:"createdAt"`
		} `json:"items"`
	}
	require.NoError(t, json.Unmarshal(payload, &body))
	require.Len(t, body.Items, 1)

	item := body.Items[0]
	assert.Equal(t, "verification-1", item.ID)
	assert.Equal(t, "org-1", item.OrganizationID)
	assert.Equal(t, "email", item.Type)
	assert.Equal(t, "user@example.com", item.Recipient)
	assert.Empty(t, item.Code)
	assert.Equal(t, "pending", item.Status)
	assert.Equal(t, expiresAt, item.ExpiresAt)
	assert.Equal(t, createdAt, item.CreatedAt)
}

func TestGetVerification_JSONHidesOTPCode(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	expiresAt := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)
	createdAt := expiresAt.Add(-10 * time.Minute)

	uowMock := uowmock.NewMockAuthUoW(ctrl)
	repo := verificationsmock.NewMockRepository(ctrl)
	uowMock.EXPECT().Verifications().Return(repo)
	repo.EXPECT().GetByIDForOrganization(gomock.Any(), "verification-1", trustedOrg).Return(&domainverifications.Verification{
		ID:             "verification-1",
		OrganizationID: trustedOrg,
		Type:           "email",
		Recipient:      "user@example.com",
		Code:           "123456",
		Status:         "pending",
		ExpiresAt:      expiresAt,
		CreatedAt:      createdAt,
	}, nil)

	router := testRouter()
	RegisterChiRoutes(router, Handler{Service: appverifications.Service{UOW: uowMock}})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, forgedOrgRequest("/api/v1/verifications/verification-1"))
	resp := recorder.Result()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	defer resp.Body.Close()

	payload, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.NotContains(t, string(payload), "123456")
	assert.Contains(t, string(payload), `"code":""`)

	var body struct {
		ID             string    `json:"id"`
		OrganizationID string    `json:"organizationId"`
		Type           string    `json:"type"`
		Recipient      string    `json:"recipient"`
		Code           string    `json:"code"`
		Status         string    `json:"status"`
		ExpiresAt      time.Time `json:"expiresAt"`
		CreatedAt      time.Time `json:"createdAt"`
	}
	require.NoError(t, json.Unmarshal(payload, &body))
	assert.Equal(t, "verification-1", body.ID)
	assert.Equal(t, trustedOrg, body.OrganizationID)
	assert.Equal(t, "email", body.Type)
	assert.Equal(t, "user@example.com", body.Recipient)
	assert.Empty(t, body.Code)
	assert.Equal(t, "pending", body.Status)
	assert.Equal(t, expiresAt, body.ExpiresAt)
	assert.Equal(t, createdAt, body.CreatedAt)
}
