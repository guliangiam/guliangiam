package users

import (
	"encoding/json"

	appusers "github.com/guliangiam/guliangiam/internal/app/users"
)

// The collections payload is described by app-layer input aliases. The
// transport layer must not import the domain package (scripts/check-layers.sh
// Rule 2), so the wire shape is referenced through appusers.

// ContactVerificationSendRequest asks the server to send a code to the contact
// already stored on the profile. The client never supplies the destination.
type ContactVerificationSendRequest struct {
	Channel string `json:"channel"`
}

// ContactVerificationVerifyRequest consumes a code for the stored contact.
type ContactVerificationVerifyRequest struct {
	Channel string `json:"channel"`
	Code    string `json:"code"`
}

// UpdateUserCollectionsRequest is a full replacement of the writable JSONB
// collections. Secret fields are write-only: an omitted value keeps the stored
// secret. The server-owned mfaItems / mfaAccounts keys are ignored on the write
// path — they are derived from the real MFA factor state.
type UpdateUserCollectionsRequest struct {
	Addresses         []appusers.AddressInput        `json:"addresses"`
	ManagedAccounts   []appusers.ManagedAccountInput `json:"managedAccounts"`
	FaceIds           []appusers.FaceIDInput         `json:"faceIds"`
	Cart              []appusers.ProductInfoInput    `json:"cart"`
	ApplicationScopes []appusers.ConsentRecordInput  `json:"applicationScopes"`
}

func (r UpdateUserCollectionsRequest) toCommand(organizationID, userID string) appusers.UpdateUserCollectionsCommand {
	return appusers.UpdateUserCollectionsCommand{
		OrganizationID: organizationID,
		UserID:         userID,
		Collections: appusers.UserCollectionsInput{
			Addresses:         r.Addresses,
			ManagedAccounts:   r.ManagedAccounts,
			FaceIds:           r.FaceIds,
			Cart:              r.Cart,
			ApplicationScopes: r.ApplicationScopes,
		},
	}
}

type ListUsersRequest struct {
	Page           int
	PageSize       int
	OrganizationID string
	Keyword        string
	Status         string
	UserType       string
	SortField      string
	SortOrder      string
	SearchField    string
	SearchValue    string
}

type GetUserRequest struct {
	ID             string
	OrganizationID string
}

type CreateUserRequest struct {
	OrganizationID      string                   `json:"organizationId"`
	Username            string                   `json:"username"`
	ExternalID          string                   `json:"externalId,omitempty"`
	UserType            string                   `json:"userType"`
	Status              string                   `json:"status"`
	IsAdmin             bool                     `json:"isAdmin"`
	IsForbidden         bool                     `json:"isForbidden,omitempty"`
	IsDeleted           bool                     `json:"isDeleted,omitempty"`
	NeedUpdatePassword  bool                     `json:"needUpdatePassword,omitempty"`
	IpWhitelist         string                   `json:"ipWhitelist,omitempty"`
	Tag                 string                   `json:"tag,omitempty"`
	Score               int                      `json:"score,omitempty"`
	Karma               int                      `json:"karma,omitempty"`
	Ranking             int                      `json:"ranking,omitempty"`
	Balance             float64                  `json:"balance,omitempty"`
	BalanceCredit       float64                  `json:"balanceCredit,omitempty"`
	BalanceCurrency     string                   `json:"balanceCurrency,omitempty"`
	SignupApplicationID string                   `json:"signupApplicationId,omitempty"`
	Password            string                   `json:"password,omitempty"`
	Profile             CreateUserProfileRequest `json:"profile"`
	RoleBindingIDs      []string                 `json:"roleBindingIds,omitempty"`
	GroupIDs            []string                 `json:"groupIds,omitempty"`
}

type CreateUserProfileRequest struct {
	DisplayName string `json:"displayName,omitempty"`
	FirstName   string `json:"firstName,omitempty"`
	LastName    string `json:"lastName,omitempty"`
	AvatarURL   string `json:"avatarUrl,omitempty"`
	Email       string `json:"email,omitempty"`
	Phone       string `json:"phone,omitempty"`
	CountryCode string `json:"countryCode,omitempty"`
	Region      string `json:"region,omitempty"`
	Location    string `json:"location,omitempty"`
	Gender      string `json:"gender,omitempty"`
	Birthday    string `json:"birthday,omitempty"`
	Bio         string `json:"bio,omitempty"`
	Homepage    string `json:"homepage,omitempty"`
	Education   string `json:"education,omitempty"`
	Language    string `json:"language,omitempty"`
	RealName    string `json:"realName,omitempty"`
	IDCardType  string `json:"idCardType,omitempty"`
	IDCard      string `json:"idCard,omitempty"`
	Affiliation string `json:"affiliation,omitempty"`
	Title       string `json:"title,omitempty"`
}

type PatchUserRequest struct {
	ExternalID          *string  `json:"externalId,omitempty"`
	SignupApplicationID *string  `json:"signupApplicationId,omitempty"`
	UserType            *string  `json:"userType,omitempty"`
	Status              *string  `json:"status,omitempty"`
	IsAdmin             *bool    `json:"isAdmin,omitempty"`
	IsForbidden         *bool    `json:"isForbidden,omitempty"`
	IsDeleted           *bool    `json:"isDeleted,omitempty"`
	IsOnline            *bool    `json:"isOnline,omitempty"`
	CreatedIp           *string  `json:"createdIp,omitempty"`
	Tag                 *string  `json:"tag,omitempty"`
	Score               *int     `json:"score,omitempty"`
	Karma               *int     `json:"karma,omitempty"`
	Ranking             *int     `json:"ranking,omitempty"`
	Language            *string  `json:"language,omitempty"`
	Title               *string  `json:"title,omitempty"`
	Balance             *float64 `json:"balance,omitempty"`
	BalanceCredit       *float64 `json:"balanceCredit,omitempty"`
	BalanceCurrency     *string  `json:"balanceCurrency,omitempty"`
	IpWhitelist         *string  `json:"ipWhitelist,omitempty"`
	Custom1             *string  `json:"custom1,omitempty"`
	Custom2             *string  `json:"custom2,omitempty"`
	Custom3             *string  `json:"custom3,omitempty"`
	Custom4             *string  `json:"custom4,omitempty"`
	Custom5             *string  `json:"custom5,omitempty"`
	Custom6             *string  `json:"custom6,omitempty"`
	Custom7             *string  `json:"custom7,omitempty"`
	Custom8             *string  `json:"custom8,omitempty"`
	Custom9             *string  `json:"custom9,omitempty"`
	Custom10            *string  `json:"custom10,omitempty"`
	Revision            int64    `json:"revision,omitempty"`
}

type UpdateUserProfileRequest struct {
	DisplayName           string         `json:"displayName,omitempty"`
	FirstName             string         `json:"firstName,omitempty"`
	LastName              string         `json:"lastName,omitempty"`
	AvatarURL             string         `json:"avatarUrl,omitempty"`
	Email                 string         `json:"email,omitempty"`
	Phone                 string         `json:"phone,omitempty"`
	CountryCode           string         `json:"countryCode,omitempty"`
	Region                string         `json:"region,omitempty"`
	Location              string         `json:"location,omitempty"`
	Birthday              string         `json:"birthday,omitempty"`
	Gender                string         `json:"gender,omitempty"`
	Bio                   string         `json:"bio,omitempty"`
	RealName              string         `json:"realName,omitempty"`
	IDCardType            string         `json:"idCardType,omitempty"`
	IDCard                string         `json:"idCard,omitempty"`
	PassportNo            string         `json:"passportNo,omitempty"`
	DriverLicenseNo       string         `json:"driverLicenseNo,omitempty"`
	IDCardExpiry          string         `json:"idCardExpiry,omitempty"`
	IssueCountry          string         `json:"issueCountry,omitempty"`
	IssuingAuthority      string         `json:"issuingAuthority,omitempty"`
	PassportPhotoURL      string         `json:"passportPhotoUrl,omitempty"`
	DriverLicenseFrontURL string         `json:"driverLicenseFrontUrl,omitempty"`
	DriverLicenseBackURL  string         `json:"driverLicenseBackUrl,omitempty"`
	Affiliation           string         `json:"affiliation,omitempty"`
	Homepage              string         `json:"homepage,omitempty"`
	Education             string         `json:"education,omitempty"`
	Title                 string         `json:"title,omitempty"`
	IDCardFront           string         `json:"idCardFront,omitempty"`
	IDCardBack            string         `json:"idCardBack,omitempty"`
	IDCardWithPerson      string         `json:"idCardWithPerson,omitempty"`
	Language              string         `json:"language,omitempty"`
	Properties            map[string]any `json:"properties,omitempty"`
}

// UpdateSelfProfileRequest is the session-bound partial-merge DTO for
// PATCH /api/v1/auth/profile. Pointer semantics: nil = keep the stored value.
// It deliberately has NO email/phone fields — contact binding must go through
// /api/v1/auth/account with a verification code. route.DecodeJSON is strict
// (DisallowUnknownFields), so a payload carrying email/phone/userID/
// organizationId is rejected with 400 before any app work.
type UpdateSelfProfileRequest struct {
	DisplayName           *string        `json:"displayName,omitempty"`
	FirstName             *string        `json:"firstName,omitempty"`
	LastName              *string        `json:"lastName,omitempty"`
	AvatarURL             *string        `json:"avatarUrl,omitempty"`
	CountryCode           *string        `json:"countryCode,omitempty"`
	Region                *string        `json:"region,omitempty"`
	Location              *string        `json:"location,omitempty"`
	Birthday              *string        `json:"birthday,omitempty"`
	Gender                *string        `json:"gender,omitempty"`
	Bio                   *string        `json:"bio,omitempty"`
	RealName              *string        `json:"realName,omitempty"`
	IDCardType            *string        `json:"idCardType,omitempty"`
	IDCard                *string        `json:"idCard,omitempty"`
	PassportNo            *string        `json:"passportNo,omitempty"`
	DriverLicenseNo       *string        `json:"driverLicenseNo,omitempty"`
	IDCardExpiry          *string        `json:"idCardExpiry,omitempty"`
	IssueCountry          *string        `json:"issueCountry,omitempty"`
	IssuingAuthority      *string        `json:"issuingAuthority,omitempty"`
	PassportPhotoURL      *string        `json:"passportPhotoUrl,omitempty"`
	DriverLicenseFrontURL *string        `json:"driverLicenseFrontUrl,omitempty"`
	DriverLicenseBackURL  *string        `json:"driverLicenseBackUrl,omitempty"`
	Affiliation           *string        `json:"affiliation,omitempty"`
	Homepage              *string        `json:"homepage,omitempty"`
	Education             *string        `json:"education,omitempty"`
	Title                 *string        `json:"title,omitempty"`
	Language              *string        `json:"language,omitempty"`
	IDCardFront           *string        `json:"idCardFront,omitempty"`
	IDCardBack            *string        `json:"idCardBack,omitempty"`
	IDCardWithPerson      *string        `json:"idCardWithPerson,omitempty"`
	Properties            map[string]any `json:"properties,omitempty"`
}

type ReplaceUserRoleBindingsRequest struct {
	Items []ReplaceUserRoleBindingItemRequest `json:"items"`
}

type ReplaceUserRoleBindingItemRequest struct {
	RoleID string `json:"roleId"`
}

type ReplaceUserGroupMembershipsRequest struct {
	Items []ReplaceUserGroupMembershipItemRequest `json:"items"`
}

type ReplaceUserGroupMembershipItemRequest struct {
	GroupID string `json:"groupId"`
}

type UpdateUserCredentialsRequest struct {
	Password           string `json:"password,omitempty"`
	RotateAccessSecret bool   `json:"rotateAccessSecret,omitempty"`
	// 指针语义：缺省 = 不修改，false = 显式清除「下次登录须改密」标记。
	NeedUpdatePassword *bool `json:"needUpdatePassword,omitempty"`
}

type ReplaceUserIdentitiesRequest struct {
	Items []ReplaceUserIdentityItemRequest `json:"items"`
}

type ReplaceUserIdentityItemRequest struct {
	ID              string         `json:"id,omitempty"`
	ProviderID      string         `json:"providerId"`
	IdentityType    string         `json:"identityType"`
	Subject         string         `json:"subject"`
	ProviderAccount string         `json:"providerAccount,omitempty"`
	ProfileSnapshot map[string]any `json:"profileSnapshot,omitempty"`
}

type ReplaceUserMfaMethodsRequest struct {
	// Revision is the users.revision from the read API. It is required: a
	// stale or missing value is rejected with 409 instead of overwriting a
	// concurrent change.
	Revision int64                             `json:"revision"`
	Items    []ReplaceUserMfaMethodItemRequest `json:"items"`
}

type UpdateUserPreferredMfaRequest struct {
	MfaType  string `json:"mfaType"`
	Revision int64  `json:"revision,omitempty"`
}

type InitiateUserMfaSetupRequest struct {
	MfaType string `json:"mfaType"`
}

type VerifyUserMfaSetupRequest struct {
	MfaType     string `json:"mfaType"`
	Passcode    string `json:"passcode"`
	Secret      string `json:"secret,omitempty"`
	Dest        string `json:"dest,omitempty"`
	CountryCode string `json:"countryCode,omitempty"`
}

type EnableUserMfaSetupRequest struct {
	MfaType       string   `json:"mfaType"`
	Secret        string   `json:"secret,omitempty"`
	Dest          string   `json:"dest,omitempty"`
	CountryCode   string   `json:"countryCode,omitempty"`
	Passcode      string   `json:"passcode"`
	RecoveryCodes []string `json:"recoveryCodes"`
}

type ImportUserItem struct {
	Username       string `json:"username"`
	DisplayName    string `json:"displayName"`
	Email          string `json:"email"`
	Phone          string `json:"phone"`
	Password       string `json:"password"`
	OrganizationID string `json:"organizationId"`
}

type ImportUsersRequest struct {
	Users []ImportUserItem `json:"users"`
}

type ReplaceUserMfaMethodItemRequest struct {
	ID          string `json:"id,omitempty"`
	MethodType  string `json:"methodType"`
	Enabled     bool   `json:"enabled"`
	Verified    bool   `json:"verified"`
	DisplayName string `json:"displayName,omitempty"`
}

// SetPasswordRequest is the request body for the set-password action.
// Reason 仅写入审计 payload，不回显到任何响应。
type SetPasswordRequest struct {
	Password string `json:"password"`
	Reason   string `json:"reason,omitempty"`
}

// RenameCredentialRequest is the request body for renaming a passkey.
type RenameCredentialRequest struct {
	Name string `json:"name"`
}

// AdminRegistrationBeginRequest starts a server-driven passkey registration.
type AdminRegistrationBeginRequest struct {
	Name string `json:"name,omitempty"`
}

// AdminRegistrationFinishRequest completes a server-driven passkey registration.
// publicKey/aaguid/signCount are declared only so the handler can reject them
// explicitly: the server derives the credential ID and COSE public key from
// authenticatorData (embedded in attestationObject) and never trusts a
// client-submitted public key.
type AdminRegistrationFinishRequest struct {
	ChallengeID       string          `json:"challengeId"`
	Challenge         string          `json:"challenge"`
	CredentialID      string          `json:"credentialId,omitempty"`
	ClientDataJSON    string          `json:"clientDataJSON"`
	AttestationObject string          `json:"attestationObject"`
	Transports        []string        `json:"transports,omitempty"`
	Name              string          `json:"name,omitempty"`
	PublicKey         json.RawMessage `json:"publicKey,omitempty"`
	AAGUID            json.RawMessage `json:"aaguid,omitempty"`
	SignCount         json.RawMessage `json:"signCount,omitempty"`
}

// forbiddenRegistrationFields lists the client-supplied trust-boundary fields
// that must never be accepted on this endpoint.
func (r AdminRegistrationFinishRequest) forbiddenRegistrationFields() []string {
	forbidden := make([]string, 0, 3)
	if len(r.PublicKey) > 0 {
		forbidden = append(forbidden, "publicKey")
	}
	if len(r.AAGUID) > 0 {
		forbidden = append(forbidden, "aaguid")
	}
	if len(r.SignCount) > 0 {
		forbidden = append(forbidden, "signCount")
	}
	return forbidden
}

func (r RenameCredentialRequest) toCommand(organizationID, userID, credentialID string) appusers.RenameUserWebAuthnCredentialCommand {
	return appusers.RenameUserWebAuthnCredentialCommand{
		OrganizationID: organizationID,
		UserID:         userID,
		CredentialID:   credentialID,
		Name:           r.Name,
	}
}

func (r AdminRegistrationBeginRequest) toCommand(organizationID, userID string) appusers.AdminWebAuthnRegistrationBeginCommand {
	return appusers.AdminWebAuthnRegistrationBeginCommand{
		OrganizationID: organizationID,
		UserID:         userID,
		Name:           r.Name,
	}
}

func (r AdminRegistrationFinishRequest) toCommand(organizationID, userID string) appusers.AdminWebAuthnRegistrationFinishCommand {
	return appusers.AdminWebAuthnRegistrationFinishCommand{
		OrganizationID:    organizationID,
		UserID:            userID,
		ChallengeID:       r.ChallengeID,
		Challenge:         r.Challenge,
		CredentialID:      r.CredentialID,
		ClientDataJSON:    r.ClientDataJSON,
		AttestationObject: r.AttestationObject,
		Transports:        r.Transports,
		Name:              r.Name,
	}
}

// UnlinkIdentityRequest is the request body for the unlink action.
type UnlinkIdentityRequest struct {
	IdentityID string `json:"identityId"`
}

func (r CreateUserRequest) toCommand() createUserCommand {
	return createUserCommand{
		OrganizationID:      r.OrganizationID,
		Username:            r.Username,
		ExternalID:          r.ExternalID,
		UserType:            r.UserType,
		Status:              r.Status,
		IsAdmin:             r.IsAdmin,
		IsForbidden:         r.IsForbidden,
		IsDeleted:           r.IsDeleted,
		NeedUpdatePassword:  r.NeedUpdatePassword,
		IpWhitelist:         r.IpWhitelist,
		Tag:                 r.Tag,
		Score:               r.Score,
		Karma:               r.Karma,
		Ranking:             r.Ranking,
		Balance:             r.Balance,
		BalanceCredit:       r.BalanceCredit,
		BalanceCurrency:     r.BalanceCurrency,
		SignupApplicationID: r.SignupApplicationID,
		Password:            r.Password,
		Profile: createUserProfileCommand{
			DisplayName: r.Profile.DisplayName,
			FirstName:   r.Profile.FirstName,
			LastName:    r.Profile.LastName,
			AvatarURL:   r.Profile.AvatarURL,
			Email:       r.Profile.Email,
			Phone:       r.Profile.Phone,
			CountryCode: r.Profile.CountryCode,
			Region:      r.Profile.Region,
			Location:    r.Profile.Location,
			Gender:      r.Profile.Gender,
			Birthday:    r.Profile.Birthday,
			Bio:         r.Profile.Bio,
			Homepage:    r.Profile.Homepage,
			Education:   r.Profile.Education,
			Language:    r.Profile.Language,
			RealName:    r.Profile.RealName,
			IDCardType:  r.Profile.IDCardType,
			IDCard:      r.Profile.IDCard,
			Affiliation: r.Profile.Affiliation,
			Title:       r.Profile.Title,
		},
		RoleBindingIDs: r.RoleBindingIDs,
		GroupIDs:       r.GroupIDs,
	}
}

func (r PatchUserRequest) toCommand(id string) appusers.PatchUserCommand {
	return appusers.PatchUserCommand{
		ID:                  id,
		ExternalID:          r.ExternalID,
		SignupApplicationID: r.SignupApplicationID,
		UserType:            r.UserType,
		Status:              r.Status,
		IsAdmin:             r.IsAdmin,
		IsForbidden:         r.IsForbidden,
		IsDeleted:           r.IsDeleted,
		IsOnline:            r.IsOnline,
		CreatedIp:           r.CreatedIp,
		Tag:                 r.Tag,
		Score:               r.Score,
		Karma:               r.Karma,
		Ranking:             r.Ranking,
		Language:            r.Language,
		Title:               r.Title,
		Balance:             r.Balance,
		BalanceCredit:       r.BalanceCredit,
		BalanceCurrency:     r.BalanceCurrency,
		IpWhitelist:         r.IpWhitelist,
		Custom1:             r.Custom1,
		Custom2:             r.Custom2,
		Custom3:             r.Custom3,
		Custom4:             r.Custom4,
		Custom5:             r.Custom5,
		Custom6:             r.Custom6,
		Custom7:             r.Custom7,
		Custom8:             r.Custom8,
		Custom9:             r.Custom9,
		Custom10:            r.Custom10,
		Revision:            r.Revision,
	}
}

func (r UpdateUserProfileRequest) toCommand(userID string) appusers.UpdateUserProfileCommand {
	properties := r.Properties
	if properties == nil {
		properties = make(map[string]any)
	}
	// 无独立列的档案字段统一落到 profile.properties（GET 会扁平化回显），
	// 与 app 层 validateUpdateUserProfileCommand 的 homepage 校验口径一致。
	for key, value := range map[string]string{
		"affiliation":      r.Affiliation,
		"homepage":         r.Homepage,
		"education":        r.Education,
		"title":            r.Title,
		"idCardFront":      r.IDCardFront,
		"idCardBack":       r.IDCardBack,
		"idCardWithPerson": r.IDCardWithPerson,
		// language 由 PATCH /users 写入 profile.properties；PUT /profile 是全量
		// 替换，若不带上就会在档案保存时把语言设置抹掉。
		"language": r.Language,
	} {
		if value != "" {
			properties[key] = value
		}
	}
	return appusers.UpdateUserProfileCommand{
		UserID:                userID,
		DisplayName:           r.DisplayName,
		FirstName:             r.FirstName,
		LastName:              r.LastName,
		AvatarURL:             r.AvatarURL,
		Email:                 r.Email,
		Phone:                 r.Phone,
		CountryCode:           r.CountryCode,
		Region:                r.Region,
		Location:              r.Location,
		Birthday:              r.Birthday,
		Gender:                r.Gender,
		Bio:                   r.Bio,
		RealName:              r.RealName,
		IDCardType:            r.IDCardType,
		IDCard:                r.IDCard,
		PassportNo:            r.PassportNo,
		DriverLicenseNo:       r.DriverLicenseNo,
		IDCardExpiry:          r.IDCardExpiry,
		IssueCountry:          r.IssueCountry,
		IssuingAuthority:      r.IssuingAuthority,
		PassportPhotoURL:      r.PassportPhotoURL,
		DriverLicenseFrontURL: r.DriverLicenseFrontURL,
		DriverLicenseBackURL:  r.DriverLicenseBackURL,
		Properties:            properties,
	}
}

// toCommand passes the optional pointers through verbatim; the app layer
// merges them onto the stored profile (nil = keep). The folded property keys
// are merged there as well, so no empty-string filtering happens here.
func (r UpdateSelfProfileRequest) toCommand(userID string) appusers.UpdateSelfProfileCommand {
	return appusers.UpdateSelfProfileCommand{
		UserID:                userID,
		DisplayName:           r.DisplayName,
		FirstName:             r.FirstName,
		LastName:              r.LastName,
		AvatarURL:             r.AvatarURL,
		CountryCode:           r.CountryCode,
		Region:                r.Region,
		Location:              r.Location,
		Birthday:              r.Birthday,
		Gender:                r.Gender,
		Bio:                   r.Bio,
		RealName:              r.RealName,
		IDCardType:            r.IDCardType,
		IDCard:                r.IDCard,
		PassportNo:            r.PassportNo,
		DriverLicenseNo:       r.DriverLicenseNo,
		IDCardExpiry:          r.IDCardExpiry,
		IssueCountry:          r.IssueCountry,
		IssuingAuthority:      r.IssuingAuthority,
		PassportPhotoURL:      r.PassportPhotoURL,
		DriverLicenseFrontURL: r.DriverLicenseFrontURL,
		DriverLicenseBackURL:  r.DriverLicenseBackURL,
		Affiliation:           r.Affiliation,
		Homepage:              r.Homepage,
		Education:             r.Education,
		Title:                 r.Title,
		Language:              r.Language,
		IDCardFront:           r.IDCardFront,
		IDCardBack:            r.IDCardBack,
		IDCardWithPerson:      r.IDCardWithPerson,
		Properties:            r.Properties,
	}
}

func (r ReplaceUserRoleBindingsRequest) toCommands() []appusers.ReplaceUserRoleBindingCommand {
	items := make([]appusers.ReplaceUserRoleBindingCommand, 0, len(r.Items))
	for _, item := range r.Items {
		items = append(items, appusers.ReplaceUserRoleBindingCommand{RoleID: item.RoleID})
	}
	return items
}

func (r ReplaceUserGroupMembershipsRequest) toCommands() []appusers.ReplaceUserGroupMembershipCommand {
	items := make([]appusers.ReplaceUserGroupMembershipCommand, 0, len(r.Items))
	for _, item := range r.Items {
		items = append(items, appusers.ReplaceUserGroupMembershipCommand{GroupID: item.GroupID})
	}
	return items
}

func (r UpdateUserCredentialsRequest) toCommand(userID string) appusers.UpdateUserCredentialsCommand {
	return appusers.UpdateUserCredentialsCommand{
		UserID:             userID,
		Password:           r.Password,
		RotateAccessSecret: r.RotateAccessSecret,
		NeedUpdatePassword: r.NeedUpdatePassword,
	}
}

func (r ReplaceUserIdentitiesRequest) toCommands() []appusers.ReplaceUserIdentityCommand {
	items := make([]appusers.ReplaceUserIdentityCommand, 0, len(r.Items))
	for _, item := range r.Items {
		items = append(items, appusers.ReplaceUserIdentityCommand{
			ID:              item.ID,
			ProviderID:      item.ProviderID,
			IdentityType:    item.IdentityType,
			Subject:         item.Subject,
			ProviderAccount: item.ProviderAccount,
			ProfileSnapshot: item.ProfileSnapshot,
		})
	}
	return items
}

func (r ReplaceUserMfaMethodsRequest) toCommands() []appusers.ReplaceUserMfaMethodCommand {
	items := make([]appusers.ReplaceUserMfaMethodCommand, 0, len(r.Items))
	for _, item := range r.Items {
		items = append(items, appusers.ReplaceUserMfaMethodCommand{
			ID:          item.ID,
			MethodType:  item.MethodType,
			Enabled:     item.Enabled,
			Verified:    item.Verified,
			DisplayName: item.DisplayName,
		})
	}
	return items
}

func (r UpdateUserPreferredMfaRequest) toCommand(userID string) appusers.UpdateUserPreferredMfaCommand {
	return appusers.UpdateUserPreferredMfaCommand{
		UserID:   userID,
		MfaType:  r.MfaType,
		Revision: r.Revision,
	}
}

func (r InitiateUserMfaSetupRequest) toCommand(userID string) appusers.InitiateUserMfaSetupCommand {
	return appusers.InitiateUserMfaSetupCommand{
		UserID:  userID,
		MfaType: r.MfaType,
	}
}

func (r VerifyUserMfaSetupRequest) toCommand(userID string) appusers.VerifyUserMfaSetupCommand {
	return appusers.VerifyUserMfaSetupCommand{
		UserID:      userID,
		MfaType:     r.MfaType,
		Passcode:    r.Passcode,
		Secret:      r.Secret,
		Dest:        r.Dest,
		CountryCode: r.CountryCode,
	}
}

func (r EnableUserMfaSetupRequest) toCommand(userID string) appusers.EnableUserMfaSetupCommand {
	return appusers.EnableUserMfaSetupCommand{
		UserID:        userID,
		MfaType:       r.MfaType,
		Secret:        r.Secret,
		Dest:          r.Dest,
		CountryCode:   r.CountryCode,
		Passcode:      r.Passcode,
		RecoveryCodes: r.RecoveryCodes,
	}
}

type createUserCommand struct {
	OrganizationID      string
	Username            string
	ExternalID          string
	UserType            string
	Status              string
	IsAdmin             bool
	IsForbidden         bool
	IsDeleted           bool
	NeedUpdatePassword  bool
	IpWhitelist         string
	Tag                 string
	Score               int
	Karma               int
	Ranking             int
	Balance             float64
	BalanceCredit       float64
	BalanceCurrency     string
	SignupApplicationID string
	Password            string
	Profile             createUserProfileCommand
	RoleBindingIDs      []string
	GroupIDs            []string
}

type createUserProfileCommand struct {
	DisplayName string
	FirstName   string
	LastName    string
	AvatarURL   string
	Email       string
	Phone       string
	CountryCode string
	Region      string
	Location    string
	Gender      string
	Birthday    string
	Bio         string
	Homepage    string
	Education   string
	Language    string
	RealName    string
	IDCardType  string
	IDCard      string
	Affiliation string
	Title       string
}
