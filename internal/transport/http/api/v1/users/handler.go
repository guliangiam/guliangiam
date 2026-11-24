package users

import (
	"context"
	"io"

	appauth "github.com/guliangiam/guliangiam/internal/app/auth"
	apporders "github.com/guliangiam/guliangiam/internal/app/orders"
	apppayments "github.com/guliangiam/guliangiam/internal/app/payments"
	appsessions "github.com/guliangiam/guliangiam/internal/app/sessions"
	appusers "github.com/guliangiam/guliangiam/internal/app/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/scopeauthorizer"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
	"github.com/xuri/excelize/v2"
)

type Handler struct {
	Service         appusers.Service
	OrdersService   apporders.Service
	PaymentsService apppayments.Service
	SessionService  appsessions.Service
	// Phase C-Step 3: impersonate 需要 ScopeAuthorizer 校验（RiskCritical）
	ScopeAuthorizer scopeauthorizer.ScopeAuthorizer
	// AuthService backs the trusted-device admin sub-resource. Nil only in
	// tests that never exercise those routes.
	AuthService *appauth.Service
}

// operatorID returns the authenticated session's user id, or "" for system
// paths that carry no session. Callers write it into the command so the audit
// provider can attribute the write to the operator.
func operatorID(ctx context.Context) string {
	if session, ok := authtransport.SessionContextFromContext(ctx); ok {
		return session.UserID
	}
	return ""
}

// GetMaskedContact returns masked contact info (email/phone) for a user.
func (h Handler) GetMaskedContact(ctx context.Context, organizationID, userID string) (*appusers.MaskedContactResult, error) {
	return h.Service.GetMaskedContact(ctx, userID, organizationID)
}

func (h Handler) List(ctx context.Context, request ListUsersRequest) (ListUsersResponse, error) {
	items, total, err := h.Service.ListUsers(ctx, appusers.ListUsersQuery(request))
	if err != nil {
		return ListUsersResponse{}, err
	}

	return presenter.NewListResponse(items, request.Page, request.PageSize, total), nil
}

func (h Handler) Get(ctx context.Context, request GetUserRequest) (*UserResponse, error) {
	response, err := h.Service.GetUser(ctx, request.ID, request.OrganizationID)
	if err != nil {
		return nil, err
	}

	typed := UserResponse(*response)
	return &typed, nil
}

func (h Handler) Create(ctx context.Context, request CreateUserRequest) (*CreateUserResponse, error) {
	command := request.toCommand()
	user, err := h.Service.CreateUser(ctx, appusers.CreateUserWithProfileCommand{
		CreateUserCommand: appusers.CreateUserCommand{
			OrganizationID: command.OrganizationID,
			ActorID:        operatorID(ctx),
			Username:       command.Username,
			ExternalID:     command.ExternalID,
			UserType:       command.UserType,
			Status:         command.Status,
			IsAdmin:        command.IsAdmin,
		},
		Profile: appusers.CreateUserProfileCommand{
			DisplayName: command.Profile.DisplayName,
			FirstName:   command.Profile.FirstName,
			LastName:    command.Profile.LastName,
			AvatarURL:   command.Profile.AvatarURL,
			Email:       command.Profile.Email,
			Phone:       command.Profile.Phone,
			CountryCode: command.Profile.CountryCode,
			Region:      command.Profile.Region,
			Location:    command.Profile.Location,
			Gender:      command.Profile.Gender,
			Birthday:    command.Profile.Birthday,
			Bio:         command.Profile.Bio,
			Homepage:    command.Profile.Homepage,
			Education:   command.Profile.Education,
			Language:    command.Profile.Language,
			RealName:    command.Profile.RealName,
			IDCardType:  command.Profile.IDCardType,
			IDCard:      command.Profile.IDCard,
			Affiliation: command.Profile.Affiliation,
			Title:       command.Profile.Title,
		},
		InitialPassword:       command.Password,
		RequirePasswordChange: command.NeedUpdatePassword,
		IsForbidden:           command.IsForbidden,
		IsDeleted:             command.IsDeleted,
		IpWhitelist:           command.IpWhitelist,
		Tag:                   command.Tag,
		Score:                 command.Score,
		Karma:                 command.Karma,
		Ranking:               command.Ranking,
		Balance:               command.Balance,
		BalanceCredit:         command.BalanceCredit,
		BalanceCurrency:       command.BalanceCurrency,
		SignupApplicationID:   command.SignupApplicationID,
		RoleBindingIDs:        command.RoleBindingIDs,
		GroupIDs:              command.GroupIDs,
	})
	if err != nil {
		return nil, err
	}

	return &CreateUserResponse{ID: user.ID, Status: "created"}, nil
}

func (h Handler) Patch(ctx context.Context, organizationID, id string, request PatchUserRequest) error {
	command := request.toCommand(id)
	command.OrganizationID = organizationID
	return h.Service.PatchUser(ctx, command)
}

func (h Handler) Delete(ctx context.Context, request GetUserRequest) error {
	command := appusers.DeleteUserCommand{
		ID:             request.ID,
		OrganizationID: request.OrganizationID,
		ActorID:        operatorID(ctx),
	}
	// 平台管理员判定来自会话（principalType），绝不接受请求体伪造
	//（SetPassword 同款口径）。
	if session, ok := authtransport.SessionContextFromContext(ctx); ok {
		command.ActorIsPlatformAdmin = session.PrincipalType == string(scopeauthorizer.PrincipalPlatformAdmin)
	}
	return h.Service.DeleteUser(ctx, command)
}

func (h Handler) UpdateProfile(ctx context.Context, organizationID, id string, request UpdateUserProfileRequest) error {
	command := request.toCommand(id)
	command.OrganizationID = organizationID
	return h.Service.UpdateUserProfile(ctx, command)
}

// UpdateSelfProfile is the session-bound partial-merge profile edit. The
// caller identity (organizationID/id) comes from the trusted session in the
// route guard, never from the payload; email/phone are not part of the DTO.
func (h Handler) UpdateSelfProfile(ctx context.Context, organizationID, id string, request UpdateSelfProfileRequest) error {
	command := request.toCommand(id)
	command.OrganizationID = organizationID
	return h.Service.UpdateSelfProfile(ctx, command)
}

// UpdateCollections replaces the user's writable JSONB collections (addresses,
// managed accounts, cart, application scopes). The server-owned MFA entries are
// derived from the real factor state and cannot be set from the payload.
func (h Handler) UpdateCollections(ctx context.Context, organizationID, id string, request UpdateUserCollectionsRequest) error {
	return h.Service.UpdateUserCollections(ctx, request.toCommand(organizationID, id))
}

// SendContactVerification sends a verification code to the profile's current
// email or phone. Returns the masked destination only.
func (h Handler) SendContactVerification(ctx context.Context, organizationID, id string, request ContactVerificationSendRequest) (*appusers.ContactVerificationTicket, error) {
	return h.Service.SendUserContactVerification(ctx, organizationID, id, request.Channel)
}

// VerifyContactVerification consumes the code and marks the channel verified.
func (h Handler) VerifyContactVerification(ctx context.Context, organizationID, id string, request ContactVerificationVerifyRequest) error {
	return h.Service.VerifyUserContactVerification(ctx, organizationID, id, request.Channel, request.Code)
}

// RecognizeIdentityDocument runs OCR on an uploaded identity document and
// returns the recognized fields. Nothing is persisted: the operator confirms the
// values through the normal profile update.
func (h Handler) RecognizeIdentityDocument(ctx context.Context, organizationID, id, documentType, side, mediaType string, data []byte) (appusers.IdentityDocumentFields, error) {
	return h.Service.RecognizeUserIdentityDocument(ctx, appusers.IdentityDocumentRecognitionRequest{
		OrganizationID: organizationID,
		UserID:         id,
		DocumentType:   documentType,
		Side:           side,
		MediaType:      mediaType,
		Data:           data,
	})
}

// EnrollFaceID registers a liveness-checked face template for the user.
func (h Handler) EnrollFaceID(ctx context.Context, organizationID, id, providerID, deviceInfo, mediaType string, data []byte) (FaceIDItem, error) {
	faceID, err := h.Service.EnrollUserFaceID(ctx, appusers.FaceIDEnrollmentRequest{
		OrganizationID: organizationID,
		UserID:         id,
		ProviderID:     providerID,
		MimeType:       mediaType,
		DeviceInfo:     deviceInfo,
		Data:           data,
	})
	if err != nil {
		return FaceIDItem{}, err
	}
	return FaceIDItem{ID: faceID.ID, ImageUrl: faceID.ImageUrl, CreatedAt: faceID.CreatedAt}, nil
}

// RevokeFaceID revokes the user's active face template (terminal for login) and
// clears the legacy face_ids marker, in one audited transaction.
func (h Handler) RevokeFaceID(ctx context.Context, organizationID, id string) error {
	return h.Service.RevokeUserFaceID(ctx, organizationID, id)
}

func (h Handler) ReplaceRoleBindings(ctx context.Context, organizationID, id string, request ReplaceUserRoleBindingsRequest) error {
	return h.Service.ReplaceUserRoleBindings(ctx, organizationID, id, request.toCommands())
}

func (h Handler) ReplaceGroupMemberships(ctx context.Context, organizationID, id string, request ReplaceUserGroupMembershipsRequest) error {
	return h.Service.ReplaceUserGroupMemberships(ctx, organizationID, id, request.toCommands())
}

func (h Handler) UpdateCredentials(ctx context.Context, organizationID, id string, request UpdateUserCredentialsRequest) (*UpdateUserCredentialsResponse, error) {
	command := request.toCommand(id)
	command.OrganizationID = organizationID
	command.ActorID = operatorID(ctx)
	result, err := h.Service.UpdateUserCredentials(ctx, command)
	if err != nil {
		return nil, err
	}

	return &UpdateUserCredentialsResponse{
		Status:       "updated",
		AccessKey:    result.RotatedAccessKey,
		AccessSecret: result.RotatedAccessSecret,
	}, nil
}

func (h Handler) ReplaceIdentities(ctx context.Context, organizationID, id string, request ReplaceUserIdentitiesRequest) error {
	return h.Service.ReplaceUserIdentities(ctx, organizationID, id, request.toCommands())
}

func (h Handler) ReplaceMfaMethods(ctx context.Context, organizationID, id string, request ReplaceUserMfaMethodsRequest) error {
	return h.Service.ReplaceUserMfaMethods(ctx, organizationID, id, request.Revision, request.toCommands())
}

func (h Handler) UpdatePreferredMfa(ctx context.Context, organizationID, id string, request UpdateUserPreferredMfaRequest) error {
	command := request.toCommand(id)
	command.OrganizationID = organizationID
	return h.Service.UpdateUserPreferredMfa(ctx, command)
}

func (h Handler) InitiateMfaSetup(ctx context.Context, organizationID, id string, request InitiateUserMfaSetupRequest) (*InitiateUserMfaSetupResponse, error) {
	command := request.toCommand(id)
	command.OrganizationID = organizationID
	result, err := h.Service.InitiateUserMfaSetup(ctx, command)
	if err != nil {
		return nil, err
	}

	return &InitiateUserMfaSetupResponse{
		Secret:             result.Secret,
		URL:                result.URL,
		RecoveryCodes:      result.RecoveryCodes,
		CountryCode:        result.CountryCode,
		MfaRememberInHours: result.MfaRememberInHours,
	}, nil
}

func (h Handler) VerifyMfaSetup(ctx context.Context, organizationID, id string, request VerifyUserMfaSetupRequest) error {
	command := request.toCommand(id)
	command.OrganizationID = organizationID
	return h.Service.VerifyUserMfaSetup(ctx, command)
}

func (h Handler) EnableMfaSetup(ctx context.Context, organizationID, id string, previousSessionID string, request EnableUserMfaSetupRequest) (string, error) {
	cmd := request.toCommand(id)
	cmd.OrganizationID = organizationID
	cmd.PreviousSessionID = previousSessionID
	return h.Service.EnableUserMfaSetup(ctx, cmd)
}

func (h Handler) DeleteGroupMembership(ctx context.Context, organizationID, id string, groupID string) error {
	return h.Service.DeleteUserGroupMembership(ctx, organizationID, id, groupID)
}

// DeleteIdentity removes a specific identity from a user by identity ID.
func (h Handler) DeleteIdentity(ctx context.Context, organizationID, id string, identityID string) error {
	return h.Service.DeleteUserIdentity(ctx, organizationID, id, identityID)
}

// DeleteRoleBinding removes a role binding from a user.
func (h Handler) DeleteRoleBinding(ctx context.Context, organizationID, id string, roleID string) error {
	return h.Service.DeleteUserRoleBinding(ctx, organizationID, id, roleID)
}

func (h Handler) RotateAccessKey(ctx context.Context, organizationID, id string) (*RotateAccessKeyResponse, error) {
	result, err := h.Service.RotateUserAccessKey(ctx, organizationID, id)
	if err != nil {
		return nil, err
	}

	return &RotateAccessKeyResponse{
		Status:       "rotated",
		AccessKey:    result.RotatedAccessKey,
		AccessSecret: result.RotatedAccessSecret,
	}, nil
}

func (h Handler) Impersonate(ctx context.Context, callerUsername, callerOrgID, organizationID, id string) (*ImpersonateResponse, error) {
	result, err := h.Service.ImpersonateUser(ctx, callerUsername, callerOrgID, organizationID, id)
	if err != nil {
		return nil, err
	}

	return &ImpersonateResponse{Token: result.Token}, nil
}

// VerifyIdentificationRequest carries the optional client context for a
// real-name verification session. MetaInfo must be the real device value from
// the frontend SDK (getMetaInfo); Aliyun fail-closes without it.
type VerifyIdentificationRequest struct {
	MetaInfo  string `json:"metaInfo,omitempty"`
	Country   string `json:"country,omitempty"`
	ReturnURL string `json:"returnUrl,omitempty"`
	Action    string `json:"action,omitempty"`
}

func (h Handler) VerifyIdentification(ctx context.Context, organizationID, id string, request VerifyIdentificationRequest) (appusers.IdentityVerificationTicket, error) {
	return h.Service.VerifyUserIdentification(ctx, organizationID, id, appusers.IdentityVerificationMeta{
		MetaInfo:  request.MetaInfo,
		Country:   request.Country,
		ReturnURL: request.ReturnURL,
		Action:    request.Action,
	})
}

// UnlinkIdentity removes a third-party identity from a user.
func (h Handler) UnlinkIdentity(ctx context.Context, organizationID, id string, request UnlinkIdentityRequest) error {
	return h.Service.UnlinkIdentity(ctx, appusers.UnlinkIdentityCommand{
		OrganizationID: organizationID,
		UserID:         id,
		IdentityID:     request.IdentityID,
	})
}

func (h Handler) BulkImport(ctx context.Context, organizationID string, request ImportUsersRequest) (*appusers.BulkImportResult, error) {
	importUsers := make([]appusers.ImportUser, 0, len(request.Users))
	for _, u := range request.Users {
		importUsers = append(importUsers, appusers.ImportUser{
			Username:       u.Username,
			DisplayName:    u.DisplayName,
			Email:          u.Email,
			Phone:          u.Phone,
			Password:       u.Password,
			OrganizationID: u.OrganizationID,
		})
	}
	return h.Service.BulkImportUsers(ctx, organizationID, importUsers)
}

func (h Handler) ExportExcel(ctx context.Context, organizationID string) (*excelize.File, error) {
	return h.Service.ExportUsersToExcel(ctx, organizationID)
}

// UploadExcel handles multipart Excel file upload for bulk user import.
func (h Handler) UploadExcel(ctx context.Context, reader io.Reader, organizationID string) (*appusers.ImportUsersResult, error) {
	return h.Service.ImportUsersFromExcel(ctx, reader, organizationID)
}

// PreviewExcel handles multipart Excel file upload for preview without importing.
func (h Handler) PreviewExcel(ctx context.Context, reader io.Reader, organizationID string) (*appusers.PreviewExcelResult, error) {
	return h.Service.PreviewUsersFromExcel(ctx, reader, organizationID)
}

// UploadCSV handles multipart CSV file upload for bulk user import with field mapping.
func (h Handler) UploadCSV(ctx context.Context, reader io.Reader, organizationID string, fieldMapping map[string]string) (*appusers.CSVImportResult, error) {
	return h.Service.ImportUsersFromCSV(ctx, organizationID, reader, fieldMapping)
}

// SetPassword allows an admin to set a user's password.
// 平台管理员判定来自会话（principalType），绝不接受请求体伪造；组织归属取
// 会话的 organizationId 用于审计对照。
func (h Handler) SetPassword(ctx context.Context, organizationID, id string, request SetPasswordRequest) error {
	command := appusers.SetPasswordCommand{
		OrganizationID:      organizationID,
		UserID:              id,
		Password:            request.Password,
		Reason:              request.Reason,
		ActorID:             operatorID(ctx),
		ActorOrganizationID: sessionOrganizationID(ctx),
	}
	if session, ok := authtransport.SessionContextFromContext(ctx); ok {
		command.ActorIsPlatformAdmin = session.PrincipalType == string(scopeauthorizer.PrincipalPlatformAdmin)
	}
	return h.Service.SetUserPassword(ctx, command)
}

// sessionOrganizationID returns the authenticated session's organization id,
// or "" when the request carries no session.
func sessionOrganizationID(ctx context.Context) string {
	if session, ok := authtransport.SessionContextFromContext(ctx); ok {
		return session.OrganizationID
	}
	return ""
}

// ListWebAuthnCredentials returns WebAuthn credentials for a user.
func (h Handler) ListWebAuthnCredentials(ctx context.Context, organizationID, userID string) ([]appusers.WebAuthnCredentialItem, error) {
	return h.Service.ListUserWebAuthnCredentials(ctx, organizationID, userID)
}

// DeleteWebAuthnCredential removes a WebAuthn credential for a user.
func (h Handler) DeleteWebAuthnCredential(ctx context.Context, organizationID, userID, credentialID, previousSessionID string) (string, error) {
	return h.Service.DeleteUserWebAuthnCredential(ctx, organizationID, userID, credentialID, previousSessionID)
}

// UpdateWebAuthnCredentialStatus 迁移凭证状态（吊销 / 停用 / 恢复）。返回非空会话 ID
// 时调用方需改写会话 Cookie——吊销/停用会触发 AC-AUTH-009 级联，调用者会话随之轮换。
func (h Handler) UpdateWebAuthnCredentialStatus(ctx context.Context, organizationID, userID, credentialID string, status appusers.WebAuthnCredentialStatus, previousSessionID string) (string, error) {
	return h.Service.UpdateWebAuthnCredentialStatus(ctx, appusers.UpdateWebAuthnCredentialStatusCommand{
		OrganizationID:    organizationID,
		UserID:            userID,
		CredentialID:      credentialID,
		Status:            status,
		PreviousSessionID: previousSessionID,
	})
}

// GetWebAuthnCredential returns a single passkey of a user.
func (h Handler) GetWebAuthnCredential(ctx context.Context, organizationID, userID, credentialID string) (*WebAuthnCredentialResponse, error) {
	item, err := h.Service.GetUserWebAuthnCredential(ctx, organizationID, userID, credentialID)
	if err != nil {
		return nil, err
	}
	response := WebAuthnCredentialResponse(*item)
	return &response, nil
}

// RenameWebAuthnCredential changes a passkey's display name.
func (h Handler) RenameWebAuthnCredential(ctx context.Context, organizationID, userID, credentialID string, request RenameCredentialRequest) (*WebAuthnCredentialResponse, error) {
	item, err := h.Service.RenameUserWebAuthnCredential(ctx, request.toCommand(organizationID, userID, credentialID))
	if err != nil {
		return nil, err
	}
	response := WebAuthnCredentialResponse(*item)
	return &response, nil
}

// BeginWebAuthnCredentialRegistration starts a server-driven passkey registration.
func (h Handler) BeginWebAuthnCredentialRegistration(ctx context.Context, organizationID, userID string, request AdminRegistrationBeginRequest) (*AdminRegistrationBeginResponse, error) {
	result, err := h.Service.BeginAdminWebAuthnRegistration(ctx, request.toCommand(organizationID, userID))
	if err != nil {
		return nil, err
	}
	response := AdminRegistrationBeginResponse(*result)
	return &response, nil
}

// FinishWebAuthnCredentialRegistration verifies and stores a passkey registration.
func (h Handler) FinishWebAuthnCredentialRegistration(ctx context.Context, organizationID, userID string, request AdminRegistrationFinishRequest) (*WebAuthnCredentialResponse, error) {
	// Reject client-supplied trust-boundary fields before any DB work: the server
	// derives credentialId/publicKey from authenticatorData inside the attestationObject.
	if forbidden := request.forbiddenRegistrationFields(); len(forbidden) > 0 {
		return nil, apperrors.NewWithMeta(
			apperrors.CodeInvalidArgument,
			"publicKey, aaguid and signCount must not be supplied; the server derives them from attestationObject",
			map[string]any{"fields": forbidden},
		)
	}
	item, err := h.Service.FinishAdminWebAuthnRegistration(ctx, request.toCommand(organizationID, userID))
	if err != nil {
		return nil, err
	}
	response := WebAuthnCredentialResponse(*item)
	return &response, nil
}

// ListUserTransactions returns transactions for a user.
func (h Handler) ListUserTransactions(ctx context.Context, organizationID, userID string, page, pageSize int) ([]appusers.UserTransactionItem, int64, error) {
	return h.Service.ListUserTransactions(ctx, organizationID, userID, page, pageSize)
}

// ListUserConsents returns consents for a user.
func (h Handler) ListUserConsents(ctx context.Context, organizationID, userID string) ([]appusers.UserConsentItem, error) {
	return h.Service.ListUserConsents(ctx, organizationID, userID)
}

// RevokeConsent revokes a specific consent for a user.
func (h Handler) RevokeConsent(ctx context.Context, organizationID, userID string, consentID string) error {
	return h.Service.RevokeUserConsent(ctx, appusers.RevokeConsentCommand{
		OrganizationID: organizationID,
		UserID:         userID,
		ConsentID:      consentID,
	})
}

// ExitImpersonate revokes the impersonation token for a user.
func (h Handler) ExitImpersonate(ctx context.Context, organizationID, userID string) error {
	return h.Service.ExitImpersonate(ctx, appusers.ExitImpersonateCommand{
		OrganizationID: organizationID,
		UserID:         userID,
		ActorID:        operatorID(ctx),
	})
}

// GetUserCount returns the total number of users, optionally filtered by organization.
func (h Handler) GetUserCount(ctx context.Context, organizationID string) (int64, error) {
	return h.Service.GetUserCount(ctx, organizationID)
}

// RemoveUserFromGroup removes a user from a group.
func (h Handler) RemoveUserFromGroup(ctx context.Context, organizationID, userID, groupID string) error {
	return h.Service.RemoveUserFromGroup(ctx, appusers.RemoveFromGroupCommand{
		OrganizationID: organizationID,
		UserID:         userID,
		GroupID:        groupID,
		ActorID:        operatorID(ctx),
	})
}

// ListUserOrders lists orders for a specific user. The session user id is the
// only trusted caller identity; it is passed explicitly so the app layer can
// reject cross-user reads (the query/body never determines who is calling).
func (h Handler) ListUserOrders(ctx context.Context, orgID, userID string, page, pageSize int) (ListUserOrdersResponse, error) {
	items, total, err := h.OrdersService.GetUserOrders(ctx, orgID, userID, sessionUserID(ctx), page, pageSize)
	if err != nil {
		return ListUserOrdersResponse{}, err
	}
	return presenter.NewListResponse(items, page, pageSize, total), nil
}

// ListUserPayments lists payments for a specific user. Same ownership rule as
// ListUserOrders: the caller comes from the session, never from the request.
func (h Handler) ListUserPayments(ctx context.Context, orgID, userID string, page, pageSize int) (ListUserPaymentsResponse, error) {
	items, total, err := h.PaymentsService.GetUserPayments(ctx, orgID, userID, sessionUserID(ctx), page, pageSize)
	if err != nil {
		return ListUserPaymentsResponse{}, err
	}
	return presenter.NewListResponse(items, page, pageSize, total), nil
}

// sessionUserID returns the authenticated session's user id, or "" when the
// request has no session (the app layer then denies cross-user reads).
func sessionUserID(ctx context.Context) string {
	if session, ok := authtransport.SessionContextFromContext(ctx); ok {
		return session.UserID
	}
	return ""
}

// ListUserSessions returns paginated sessions for a given user.
// AC-AUTH-009: the parent user is resolved through the trusted tenant first;
// a foreign or missing user is a 404.
func (h Handler) ListUserSessions(ctx context.Context, organizationID, userID string, page, pageSize int) ([]appsessions.SessionListItem, int64, error) {
	return h.SessionService.ListUserSessionsForOrganization(ctx, organizationID, userID, page, pageSize)
}

// RevokeUserSession revokes a single session by ID for a given user.
// AC-AUTH-009: the parent user and the session's owner are both resolved
// through the trusted tenant; a foreign or mismatched id surfaces as 404.
func (h Handler) RevokeUserSession(ctx context.Context, organizationID, userID, sessionID string) error {
	return h.SessionService.RevokeUserSessionForOrganization(ctx, organizationID, userID, sessionID)
}

// ForceLogoutUserSessions revokes every active session + refresh tokens +
// access tokens for a given user. Full AC-AUTH-009 cascade.
func (h Handler) ForceLogoutUserSessions(ctx context.Context, organizationID, userID string) (*appsessions.ForceLogoutResult, error) {
	return h.SessionService.ForceLogoutUserForOrganization(ctx, organizationID, userID)
}
