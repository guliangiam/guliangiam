package privacy

import (
	"context"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/privacy"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
)

const queryOrganizationRegion = `
SELECT COALESCE(region_code, '')
FROM guliangiam.organizations
WHERE id = $1 AND deleted_at IS NULL`

const queryExportUser = `
SELECT username, COALESCE(external_id, ''), user_type, status,
       is_admin, is_forbidden, is_verified, created_at, updated_at
FROM guliangiam.users
WHERE id = $1 AND organization_id = $2 AND deleted_at IS NULL`

const queryExportUserProfile = `
SELECT COALESCE(display_name, ''), COALESCE(first_name, ''), COALESCE(last_name, ''),
       COALESCE(email, ''), COALESCE(phone, ''), COALESCE(country_code, ''),
       COALESCE(region, ''), COALESCE(location, ''), COALESCE(birthday::text, ''),
       COALESCE(gender, ''), COALESCE(bio, ''), COALESCE(real_name, ''),
       COALESCE(id_card_type, ''), COALESCE(id_card, ''),
       COALESCE(passport_no, ''), COALESCE(driver_license_no, '')
FROM guliangiam.user_profiles
WHERE user_id = $1`

const queryExportConsents = `
SELECT purpose, version, COALESCE(granted_at::text, ''), COALESCE(withdrawn_at::text, '')
FROM guliangiam.consent_records
WHERE organization_id = $1 AND subject_type = $2 AND subject_id = $3
ORDER BY purpose, version`

// CollectSubjectData aggregates one user subject's tenant-scoped records for the
// evidence package. Credentials, MFA secrets and identity provider secrets are
// intentionally excluded (design/privacy §4).
func (r Repository) CollectSubjectData(ctx context.Context, organizationID, subjectType, subjectID string) ([]domain.ExportRecord, string, error) {
	if subjectType != "user" {
		return nil, "", apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity,
			"only user subjects are supported for export",
			map[string]any{"field": "subjectType", "value": subjectType})
	}
	region := "default"
	if err := r.db.QueryRow(ctx, queryOrganizationRegion, organizationID).Scan(&region); err != nil {
		if err != pgx.ErrNoRows {
			return nil, "", apperrors.WrapInternalf(err, "get organization region")
		}
		region = "default"
	}
	if region == "" {
		region = "default"
	}

	var username, externalID, userType, status string
	var isAdmin, isForbidden, isVerified bool
	var createdAt, updatedAt time.Time
	if err := r.db.QueryRow(ctx, queryExportUser, subjectID, organizationID).Scan(
		&username, &externalID, &userType, &status,
		&isAdmin, &isForbidden, &isVerified, &createdAt, &updatedAt,
	); err != nil {
		if err == pgx.ErrNoRows {
			return nil, "", apperrors.NewNotFound("subject data not found")
		}
		return nil, "", apperrors.WrapInternalf(err, "get user subject")
	}
	records := []domain.ExportRecord{{
		Category:  "user",
		SubjectID: subjectID,
		Payload: map[string]any{
			"username":       username,
			"externalId":     externalID,
			"userType":       userType,
			"status":         status,
			"isAdmin":        isAdmin,
			"isForbidden":    isForbidden,
			"isVerified":     isVerified,
			"createdAt":      createdAt.UTC().Format(time.RFC3339),
			"updatedAt":      updatedAt.UTC().Format(time.RFC3339),
			"organizationId": organizationID,
		},
	}}

	var displayName, firstName, lastName, email, phone, countryCode, profileRegion, location, birthday, gender, bio, realName, idCardType, idCard, passportNo, driverLicenseNo string
	if err := r.db.QueryRow(ctx, queryExportUserProfile, subjectID).Scan(
		&displayName, &firstName, &lastName, &email, &phone, &countryCode,
		&profileRegion, &location, &birthday, &gender, &bio, &realName, &idCardType, &idCard,
		&passportNo, &driverLicenseNo,
	); err != nil && err != pgx.ErrNoRows {
		return nil, "", apperrors.WrapInternalf(err, "get user profile")
	} else if err == nil {
		records = append(records, domain.ExportRecord{
			Category:  "user_profile",
			SubjectID: subjectID,
			Payload: map[string]any{
				"displayName": displayName, "firstName": firstName, "lastName": lastName,
				"email": email, "phone": phone, "countryCode": countryCode,
				"region": profileRegion, "location": location, "birthday": birthday,
				"gender": gender, "bio": bio, "realName": realName,
				"idCardType": idCardType, "idCard": idCard,
				"passportNo": passportNo, "driverLicenseNo": driverLicenseNo,
			},
		})
	}

	rows, err := r.db.Query(ctx, queryExportConsents, organizationID, subjectType, subjectID)
	if err != nil {
		return nil, "", apperrors.WrapInternalf(err, "list subject consents")
	}
	defer rows.Close()
	for rows.Next() {
		var purpose, grantedAt, withdrawnAt string
		var version int
		if err := rows.Scan(&purpose, &version, &grantedAt, &withdrawnAt); err != nil {
			return nil, "", apperrors.WrapInternalf(err, "scan subject consent")
		}
		records = append(records, domain.ExportRecord{
			Category:  "consent",
			SubjectID: subjectID,
			Payload: map[string]any{
				"purpose": purpose, "version": version,
				"grantedAt": grantedAt, "withdrawnAt": withdrawnAt,
			},
		})
	}
	if err := rows.Err(); err != nil {
		return nil, "", apperrors.WrapInternalf(err, "iterate subject consents")
	}
	return records, region, nil
}
