package auth

import (
	"context"
	"log/slog"
	"strings"

	applicationdomain "github.com/guliangiam/guliangiam/internal/domain/applications"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/uuid"
)

// applicationReferenceRepository is the slice of the applications repository a
// reference lookup needs.
type applicationReferenceRepository interface {
	GetByID(ctx context.Context, id string) (*applicationdomain.Application, error)
	GetByOrganizationAndSlug(ctx context.Context, organizationID string, slug string) (*applicationdomain.Application, error)
}

// resolveApplicationReference maps the application identifier carried by a
// public protocol request onto an application row.
//
// The protocol accepts two identifier formats: the legacy console and SDKs
// send the application *name*, while the v1 clients send the UUID. Only a
// UUID may be handed to `WHERE id = $1::uuid`; a name raised
// `SQLSTATE 22P02 (invalid input syntax for type uuid)` and leaked the
// raw PostgreSQL message to the browser.
//
// Resolution order — every lookup is scoped to the already-resolved
// organization, so a reference can never cross a tenant:
//
//  1. a syntactically valid UUID  -> GetByID
//  2. an application slug         -> GetByOrganizationAndSlug
//  3. the organization's own slug -> that organization's default application
//     (`app-<orgSlug>`), which is what callers mean when they
//     send the organization name instead of an application name
//
// Anything else is a client error: 400 invalid_argument, never a database
// failure.
// applicationReferenceSource records how a default application was chosen so
// the login path can log it (and later include it in session audit).
type applicationReferenceSource string

const (
	applicationSourceUUID                applicationReferenceSource = "explicit_uuid"
	applicationSourceSlug                applicationReferenceSource = "explicit_slug"
	applicationSourceOrganizationDefault applicationReferenceSource = "organization_default_application"
	applicationSourceLegacySlug          applicationReferenceSource = "legacy_app_slug"
)

func resolveApplicationReference(
	ctx context.Context,
	apps applicationReferenceRepository,
	organization *orgdomain.Organization,
	reference string,
) (*applicationdomain.Application, error) {
	application, _, err := resolveApplicationReferenceWithSource(ctx, apps, organization, reference)
	return application, err
}

// resolveApplicationReferenceWithSource additionally reports which rule picked
// the application. When callers send the organization name, the organization's
// configured default application wins; app-<orgSlug> is only a legacy fallback
// (OPEN-API-51).
func resolveApplicationReferenceWithSource(
	ctx context.Context,
	apps applicationReferenceRepository,
	organization *orgdomain.Organization,
	reference string,
) (*applicationdomain.Application, applicationReferenceSource, error) {
	ref := strings.TrimSpace(reference)
	if ref == "" {
		return nil, "", apperrors.NewInvalidArgument("applicationId is required")
	}

	if uuid.IsValid(ref) {
		app, err := apps.GetByID(ctx, ref)
		if err != nil {
			return nil, applicationSourceUUID, apperrors.WrapInternalf(err, "lookup application by UUID %q", ref)
		}
		return app, applicationSourceUUID, nil
	}

	if organization == nil {
		return nil, "", apperrors.NewInvalidArgument("applicationId must be a UUID when no organization is given")
	}

	application, err := apps.GetByOrganizationAndSlug(ctx, organization.ID, ref)
	if err == nil {
		return application, applicationSourceSlug, nil
	}
	if !isNotFoundError(err) {
		slog.ErrorContext(ctx, "application lookup by slug failed",
			"application", ref, "organization_id", organization.ID, "error", err)
		return nil, applicationSourceSlug, apperrors.WrapInternalf(err, "lookup application by slug %q in organization %s", ref, organization.ID)
	}

	orgSlug := strings.TrimSpace(organization.Slug)
	if orgSlug != "" && ref == orgSlug {
		if defaultID := strings.TrimSpace(organization.Settings.DefaultApplicationID); defaultID != "" {
			defaultApplication, defaultErr := apps.GetByID(ctx, defaultID)
			if defaultErr == nil && defaultApplication.OrganizationID == organization.ID {
				return defaultApplication, applicationSourceOrganizationDefault, nil
			}
			if defaultErr != nil && !isNotFoundError(defaultErr) {
				return nil, applicationSourceOrganizationDefault, apperrors.WrapInternalf(defaultErr, "lookup organization default application %s in organization %s", defaultID, organization.ID)
			}
		}
		defaultApplication, defaultErr := apps.GetByOrganizationAndSlug(ctx, organization.ID, "app-"+orgSlug)
		if defaultErr == nil {
			return defaultApplication, applicationSourceLegacySlug, nil
		}
		if !isNotFoundError(defaultErr) {
			return nil, applicationSourceLegacySlug, apperrors.WrapInternalf(defaultErr, "lookup default application app-%s in organization %s", orgSlug, organization.ID)
		}
	}

	return nil, "", apperrors.NewInvalidArgument("application not found: " + ref)
}

// isNotFoundError reports whether err means "the reference is unknown" as
// opposed to "the lookup itself failed". Callers use it to decide between
// trying a second spelling and returning a 400.
func isNotFoundError(err error) bool {
	typed, ok := err.(*apperrors.Error)
	if !ok {
		return false
	}
	return typed.Code == apperrors.CodeNotFound
}
