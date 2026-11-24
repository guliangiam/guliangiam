package auth

import (
	"context"
	"testing"

	applicationdomain "github.com/guliangiam/guliangiam/internal/domain/applications"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubApplicationRepository records which lookups a resolution attempt made, so
// the tests can prove a UUID never falls through to a slug query and vice
// versa.
type stubApplicationRepository struct {
	byID         map[string]*applicationdomain.Application
	byOrgAndSlug map[string]*applicationdomain.Application
	idLookups    []string
	slugLookups  []string
}

func (s *stubApplicationRepository) GetByID(_ context.Context, id string) (*applicationdomain.Application, error) {
	s.idLookups = append(s.idLookups, id)
	if application, ok := s.byID[id]; ok {
		return application, nil
	}

	return nil, apperrors.NewNotFound("application not found")
}

func (s *stubApplicationRepository) GetByOrganizationAndSlug(_ context.Context, organizationID, slug string) (*applicationdomain.Application, error) {
	s.slugLookups = append(s.slugLookups, organizationID+"/"+slug)
	if application, ok := s.byOrgAndSlug[organizationID+"/"+slug]; ok {
		return application, nil
	}

	return nil, apperrors.NewNotFound("application not found")
}

const (
	builtinOrgID   = "01a0ea76-0c00-7000-8000-000000000001"
	builtinAppID   = "01a0ea76-0c00-7000-8000-000000000002"
	builtinOrgSlug = "built-in"
	builtinAppSlug = "app-built-in"
)

func builtinFixture() (*stubApplicationRepository, *orgdomain.Organization, *applicationdomain.Application) {
	application := &applicationdomain.Application{
		ID:             builtinAppID,
		OrganizationID: builtinOrgID,
		Slug:           builtinAppSlug,
	}
	repository := &stubApplicationRepository{
		byID: map[string]*applicationdomain.Application{builtinAppID: application},
		byOrgAndSlug: map[string]*applicationdomain.Application{
			builtinOrgID + "/" + builtinAppSlug: application,
		},
	}

	return repository, &orgdomain.Organization{ID: builtinOrgID, Slug: builtinOrgSlug}, application
}

func TestResolveApplicationReference_UUID(t *testing.T) {
	repository, organization, expected := builtinFixture()

	application, err := resolveApplicationReference(context.Background(), repository, organization, builtinAppID)

	require.NoError(t, err)
	assert.Equal(t, expected, application)
	assert.Equal(t, []string{builtinAppID}, repository.idLookups)
	assert.Empty(t, repository.slugLookups, "a UUID must never be re-queried as a slug")
}

func TestResolveApplicationReference_ApplicationSlug(t *testing.T) {
	repository, organization, expected := builtinFixture()

	application, err := resolveApplicationReference(context.Background(), repository, organization, builtinAppSlug)

	require.NoError(t, err)
	assert.Equal(t, expected, application)
	assert.Empty(t, repository.idLookups, "a slug must never be cast to uuid")
}

// The regression this whole resolver exists for: legacy clients send
// the organization name where the v1 protocol expects an application
// reference. That used to reach `WHERE id = $1::uuid` and raise
// SQLSTATE 22P02.
func TestResolveApplicationReference_OrganizationSlugFallsBackToDefaultApplication(t *testing.T) {
	repository, organization, expected := builtinFixture()

	application, err := resolveApplicationReference(context.Background(), repository, organization, builtinOrgSlug)

	require.NoError(t, err)
	assert.Equal(t, expected, application)
	assert.Empty(t, repository.idLookups)
}

func TestResolveApplicationReference_UnknownNameIsInvalidArgument(t *testing.T) {
	repository, organization, _ := builtinFixture()

	application, err := resolveApplicationReference(context.Background(), repository, organization, "nope")

	assert.Nil(t, application)
	require.Error(t, err)
	typed, ok := err.(*apperrors.Error)
	require.True(t, ok)
	assert.Equal(t, apperrors.CodeInvalidArgument, typed.Code)
	assert.Equal(t, 400, typed.HTTPStatus(), "a bad reference is a client error, not a 500")
}

func TestResolveApplicationReference_EmptyReference(t *testing.T) {
	repository, organization, _ := builtinFixture()

	application, err := resolveApplicationReference(context.Background(), repository, organization, "   ")

	assert.Nil(t, application)
	require.Error(t, err)
	typed, ok := err.(*apperrors.Error)
	require.True(t, ok)
	assert.Equal(t, apperrors.CodeInvalidArgument, typed.Code)
	assert.Empty(t, repository.idLookups)
	assert.Empty(t, repository.slugLookups)
}

func TestResolveApplicationReference_NonUUIDWithoutOrganization(t *testing.T) {
	repository, _, _ := builtinFixture()

	application, err := resolveApplicationReference(context.Background(), repository, nil, builtinAppSlug)

	assert.Nil(t, application)
	require.Error(t, err)
	typed, ok := err.(*apperrors.Error)
	require.True(t, ok)
	assert.Equal(t, apperrors.CodeInvalidArgument, typed.Code)
}

func TestResolveApplicationReference_OrganizationLookupFailureIsPropagated(t *testing.T) {
	repository := &stubApplicationRepository{}
	organization := &orgdomain.Organization{ID: builtinOrgID, Slug: builtinOrgSlug}

	application, err := resolveApplicationReference(context.Background(), repository, organization, builtinOrgSlug)

	assert.Nil(t, application)
	require.Error(t, err)
	typed, ok := err.(*apperrors.Error)
	require.True(t, ok)
	assert.Equal(t, apperrors.CodeInvalidArgument, typed.Code)
}

func TestResolveApplicationReference_UsesOrganizationDefaultApplication(t *testing.T) {
	repository, organization, _ := builtinFixture()
	configured := &applicationdomain.Application{
		ID:             "01a0ea76-0c00-7000-8000-000000000003",
		OrganizationID: builtinOrgID,
		Slug:           "configured-console",
	}
	repository.byID[configured.ID] = configured
	organization.Settings.DefaultApplicationID = configured.ID

	application, source, err := resolveApplicationReferenceWithSource(
		context.Background(), repository, organization, builtinOrgSlug)

	require.NoError(t, err)
	assert.Equal(t, configured, application)
	assert.Equal(t, applicationSourceOrganizationDefault, source)
	assert.Equal(t, []string{configured.ID}, repository.idLookups)
}

func TestResolveApplicationReference_FallsBackToLegacySlugWhenConfiguredDefaultMissing(t *testing.T) {
	repository, organization, legacy := builtinFixture()
	organization.Settings.DefaultApplicationID = "01a0ea76-0c00-7000-8000-0000000000ff"

	application, source, err := resolveApplicationReferenceWithSource(
		context.Background(), repository, organization, builtinOrgSlug)

	require.NoError(t, err)
	assert.Equal(t, legacy, application)
	assert.Equal(t, applicationSourceLegacySlug, source)
}
