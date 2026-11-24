package users

import (
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// UserSigninTracking holds signin-related tracking fields.
// Embedded in User to preserve field access (user.SigninWrongTimes, etc.)
type UserSigninTracking struct {
	SigninWrongTimes    int
	LastSigninWrongTime *time.Time
	LastSigninAt        *time.Time
	LastSigninIP        string
}

// UserMfaConfig holds MFA configuration fields.
// Embedded in User to preserve field access (user.PreferredMfaType, etc.)
type UserMfaConfig struct {
	PreferredMfaType    string
	RecoveryCodeHashes  []string
	TotpSecret          string
	MfaRadiusUsername   string
	MfaRadiusProvider   string
	MfaPushEnabled      bool
	MfaPushReceiver     string
	MfaPushProvider     string
	MfaRememberDeadline *time.Time
}

// UserFinancials holds financial/balance fields.
// Embedded in User to preserve field access (user.Balance, etc.)
type UserFinancials struct {
	Balance         float64
	BalanceCredit   float64
	BalanceCurrency string
}

// UserCustomAttributes holds custom extension fields.
// Embedded in User to preserve field access (user.Custom1, etc.)
type UserCustomAttributes struct {
	Custom1  string
	Custom2  string
	Custom3  string
	Custom4  string
	Custom5  string
	Custom6  string
	Custom7  string
	Custom8  string
	Custom9  string
	Custom10 string
}

// UserCollections holds JSONB array fields that represent collections.
// Embedded in User to preserve field access (user.Addresses, etc.)
type UserCollections struct {
	Addresses         []Address
	ManagedAccounts   []ManagedAccount
	FaceIds           []FaceId
	Cart              []ProductInfo
	MfaItems          []MfaItem
	MfaAccounts       []MfaAccount
	ApplicationScopes []ConsentRecord
}

// User is the aggregate root for the identity domain.
// Fields are FLAT — not embedded value objects.
//
// Rationale: Go composite literals CANNOT initialize promoted fields directly
// (you must use `EmbeddedType: EmbeddedType{Field: value}` syntax). This project
// has 100+ callers that initialize User with flat field names (mappers, tests,
// seeders). Embedded structs would require updating every single one.
// Keeping flat fields keeps the composite literal syntax: User{Balance: 100}.
//
// Value-object types (UserSigninTracking, UserMfaConfig, …) remain below as
// documentation / future extraction candidates — they are not embedded.
type User struct {
	// ── Core identity ──
	ID                  string
	OrganizationID      string
	Username            string
	ExternalID          string
	UserType            string
	Status              string
	IsAdmin             bool
	IsForbidden         bool
	IsVerified          bool
	IsOnline            bool
	IsDeleted           bool
	CreatedIp           string
	SignupApplicationID string
	RegisterType        string
	RegisterSource      string
	Tag                 string
	Score               int
	IpWhitelist         string
	CreatedAt           time.Time
	UpdatedAt           time.Time
	DeletedAt           *time.Time
	Revision            int64

	// ── Signin tracking (was UserSigninTracking) ──
	SigninWrongTimes    int
	LastSigninWrongTime *time.Time
	LastSigninAt        *time.Time
	LastSigninIP        string

	// ── MFA config (was UserMfaConfig) ──
	PreferredMfaType    string
	RecoveryCodeHashes  []string
	TotpSecret          string
	MfaRadiusUsername   string
	MfaRadiusProvider   string
	MfaPushEnabled      bool
	MfaPushReceiver     string
	MfaPushProvider     string
	MfaRememberDeadline *time.Time

	// ── Financials (was UserFinancials) ──
	Balance         float64
	BalanceCredit   float64
	BalanceCurrency string

	// ── Custom attributes (was UserCustomAttributes) ──
	Custom1  string
	Custom2  string
	Custom3  string
	Custom4  string
	Custom5  string
	Custom6  string
	Custom7  string
	Custom8  string
	Custom9  string
	Custom10 string

	// ── Collections (was UserCollections) ──
	Addresses         []Address
	ManagedAccounts   []ManagedAccount
	FaceIds           []FaceId
	Cart              []ProductInfo
	MfaItems          []MfaItem
	MfaAccounts       []MfaAccount
	ApplicationScopes []ConsentRecord

	// ── Associated entities ──
	Profile          UserProfile
	Credentials      UserCredentials
	Identities       []UserIdentity
	MfaMethods       []UserMfaMethod
	RoleBindings     []UserRoleBinding
	GroupMemberships []UserGroupMembership
}

// Address represents a user address entry.
//
// The canonical shape is a single country selector (CountryCode) followed by
// province/city/district/street/postalCode. Region/Details are retained only so
// rows written by older clients keep rendering; new writes should use the
// canonical fields.
type Address struct {
	ID          string `json:"id,omitempty"`
	CountryCode string `json:"countryCode,omitempty"`
	Province    string `json:"province,omitempty"`
	City        string `json:"city,omitempty"`
	District    string `json:"district,omitempty"`
	PostalCode  string `json:"postalCode,omitempty"`
	Street      string `json:"street"`
	Details     string `json:"details,omitempty"`
	Region      string `json:"region,omitempty"`
	IsDefault   bool   `json:"isDefault,omitempty"`
}

// ManagedAccount represents a third-party account managed by the user.
type ManagedAccount struct {
	Application string `json:"application"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	SigninUrl   string `json:"signinUrl"`
}

// FaceId represents a face recognition enrollment.
type FaceId struct {
	ID        string `json:"id"`
	ImageUrl  string `json:"imageUrl"`
	CreatedAt string `json:"createdAt"`
}

// ProductInfo represents an item in the user's cart.
type ProductInfo struct {
	ProductID string  `json:"productId"`
	Quantity  int     `json:"quantity"`
	Price     float64 `json:"price"`
}

// MfaItem represents a user-configured MFA entry.
type MfaItem struct {
	MfaType string `json:"mfaType"`
	Secret  string `json:"secret"`
}

// MfaAccount represents a TOTP account entry.
type MfaAccount struct {
	AccountName string `json:"accountName"`
	Issuer      string `json:"issuer"`
	SecretKey   string `json:"secretKey"`
	Origin      string `json:"origin"`
}

// ConsentRecord represents an OAuth consent granted by the user.
type ConsentRecord struct {
	Application string   `json:"application"`
	Scopes      []string `json:"scopes"`
	GrantedAt   string   `json:"grantedAt"`
}

// Validate checks the user fields for business rule violations.
func (u *User) Validate() error {
	if u.ID == "" {
		return apperrors.NewInvalidArgument("id is required")
	}
	if u.OrganizationID == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	if u.Username == "" {
		return apperrors.NewInvalidArgument("username is required")
	}
	return nil
}

// SoftDeleted returns true if the user has been soft-deleted.
func (u *User) SoftDeleted() bool {
	return u.DeletedAt != nil
}

// CanLogin returns true if the user is allowed to login (not forbidden, not deleted).
func (u *User) CanLogin() bool {
	return !u.IsForbidden && !u.SoftDeleted()
}

// IsVerifiedIdentified returns true if the user has passed real-name verification.
func (u *User) IsVerifiedIdentified() bool {
	return u.IsVerified
}
