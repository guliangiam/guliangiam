package users

import (
	apporders "github.com/guliangiam/guliangiam/internal/app/orders"
	apppayments "github.com/guliangiam/guliangiam/internal/app/payments"
	appusers "github.com/guliangiam/guliangiam/internal/app/users"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

type ListUsersResponse = presenter.ListResponse[appusers.UserListItem]
type UserResponse = appusers.UserDetail
type ListUserOrdersResponse = presenter.ListResponse[apporders.OrderListItem]
type ListUserPaymentsResponse = presenter.ListResponse[apppayments.PaymentListItem]

type UpdateUserCredentialsResponse struct {
	Status       string `json:"status"`
	AccessKey    string `json:"accessKey,omitempty"`
	AccessSecret string `json:"accessSecret,omitempty"`
}

type CreateUserResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

type InitiateUserMfaSetupResponse struct {
	Secret             string   `json:"secret,omitempty"`
	URL                string   `json:"url,omitempty"`
	RecoveryCodes      []string `json:"recoveryCodes,omitempty"`
	CountryCode        string   `json:"countryCode,omitempty"`
	MfaRememberInHours int64    `json:"mfaRememberInHours,omitempty"`
}

type RotateAccessKeyResponse struct {
	Status       string `json:"status"`
	AccessKey    string `json:"accessKey,omitempty"`
	AccessSecret string `json:"accessSecret,omitempty"`
}

type ImpersonateResponse struct {
	Token string `json:"token"`
}

// WebAuthnCredentialResponse is a single passkey returned by detail, rename and
// registration-finish endpoints. The fields live on the app-level DTO.
type WebAuthnCredentialResponse = appusers.WebAuthnCredentialItem

// AdminRegistrationBeginResponse contains the creation options for a
// server-driven passkey registration ceremony.
type AdminRegistrationBeginResponse = appusers.AdminWebAuthnRegistrationBeginResult
