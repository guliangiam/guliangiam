package scim

import (
	"context"
	"encoding/json"
	"errors" // stdlib errors retained for errors.Is/errors.As only; apperrors used for error creation
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	"github.com/guliangiam/guliangiam/internal/domain/scim"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) modelToSchema(model *ScimSchemaModel) (*scim.ScimSchema, error) {
	var attributes []scim.Attribute
	if len(model.Attributes) > 0 {
		err := json.Unmarshal(model.Attributes, &attributes)
		if err != nil {
			return nil, apperrors.WrapInternalf(err, "failed to unmarshal attributes")
		}
	}

	return &scim.ScimSchema{
		ID:             model.ID.String(),
		OrganizationID: model.OrganizationID.String(),
		SchemaID:       model.SchemaID,
		Name:           model.Name,
		Description:    stringPtrToString(model.Description),
		Attributes:     attributes,
		CreatedAt:      model.CreatedAt,
		UpdatedAt:      model.UpdatedAt,
		Revision:       model.Revision,
	}, nil
}

func (r *Repository) modelToUser(model *ScimUserModel, emails []scim.Email, phones []scim.PhoneNumber, groups []scim.GroupRef) (*scim.ScimUser, error) {
	_ = time.Now()
	return &scim.ScimUser{
		ID:             model.ID.String(),
		OrganizationID: model.OrganizationID.String(),
		UserName:       model.Username,
		DisplayName:    stringPtrToString(model.DisplayName),
		Emails:         emails,
		PhoneNumbers:   phones,
		Groups:         groups,
		Active:         &model.IsActive,
		Meta: &scim.Meta{
			ResourceType: resourceTypeUser,
			Created:      model.CreatedAt,
			LastModified: model.UpdatedAt,
		},
		CreatedAt: model.CreatedAt,
		UpdatedAt: model.UpdatedAt,
		Revision:  model.Revision,
	}, nil
}

func (r *Repository) modelToGroup(model *ScimGroupModel, members []scim.Member) (*scim.ScimGroup, error) {
	_ = time.Now()
	return &scim.ScimGroup{
		ID:             model.ID.String(),
		OrganizationID: model.OrganizationID.String(),
		DisplayName:    model.DisplayName,
		Members:        members,
		Meta: &scim.Meta{
			ResourceType: resourceTypeGroup,
			Created:      model.CreatedAt,
			LastModified: model.UpdatedAt,
		},
		CreatedAt: model.CreatedAt,
		UpdatedAt: model.UpdatedAt,
		Revision:  model.Revision,
	}, nil
}

func (r *Repository) getUserEmails(ctx context.Context, userID string) ([]scim.Email, error) {
	var email string
	err := r.pool.QueryRow(ctx, QueryGetUserEmail, userID).Scan(&email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return []scim.Email{}, nil
		}
		return nil, apperrors.WrapInternalf(err, "failed to get user emails")
	}

	return []scim.Email{
		{
			Value:   email,
			Type:    attrTypeWork,
			Primary: true,
		},
	}, nil
}

func (r *Repository) getUserPhones(ctx context.Context, userID string) ([]scim.PhoneNumber, error) {
	var phone string
	err := r.pool.QueryRow(ctx, QueryGetUserPhone, userID).Scan(&phone)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return []scim.PhoneNumber{}, nil
		}
		return nil, apperrors.WrapInternalf(err, "failed to get user phones")
	}

	return []scim.PhoneNumber{
		{
			Value:   phone,
			Type:    attrTypeWork,
			Primary: true,
		},
	}, nil
}

func (r *Repository) getUserGroups(ctx context.Context, userID string) ([]scim.GroupRef, error) {
	rows, err := r.pool.Query(ctx, QueryGetUserGroups, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []scim.GroupRef
	for rows.Next() {
		var id, name, displayName string
		err := rows.Scan(&id, &name, &displayName)
		if err != nil {
			return nil, err
		}
		groups = append(groups, scim.GroupRef{
			Value:   id,
			Display: displayName,
			Ref:     "/scim/v2/Groups/" + id,
			Type:    resourceTypeGroup,
		})
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, rowsErr
	}

	return groups, nil
}
