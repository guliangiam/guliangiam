package users

import (
	"database/sql"
	"time"
)

// userRow 是 MySQL users 聚合读的扫描目标。
// 与 PG 版 internal/infra/postgres/users/models.go 字段顺序一致：基础列来自 `users`，
// 其余列来自 ADR-022 拆分的 user_signin_tracking / user_mfa_config / user_financials /
// user_collections / user_custom_attributes（LEFT JOIN 后 COALESCE）。
// SQL 已 COALESCE 的列用原生 string；SQL 未 COALESCE 的可 NULL 列（TIMESTAMP NULL）用 sql.Null*。
type userRow struct {
	ID                  string
	OrganizationID      string
	Username            string
	ExternalID          string // COALESCE(external_id, '')
	UserType            string
	Status              string
	IsAdmin             bool
	IsForbidden         bool
	IsVerified          bool
	IsOnline            bool
	CreatedIp           string // COALESCE(created_ip, '')
	SignupApplicationID string // COALESCE(signup_application_id, '')
	RegisterSource      string // COALESCE(register_source, '')
	RegisterType        string
	Tag                 string
	Score               int
	SigninWrongTimes    int          // COALESCE(st.signin_wrong_times, 0)
	LastSigninWrongTime sql.NullTime // TIMESTAMP NULL
	PreferredMfaType    string       // COALESCE(preferred_mfa_type, '')
	RecoveryCodeHashes  []byte       // COALESCE(recovery_code_hashes, '[]')
	TotpSecret          string       // COALESCE(totp_secret, '')
	MfaRadiusUsername   string       // COALESCE(mfa_radius_username, '')
	MfaRadiusProvider   string       // COALESCE(mfa_radius_provider, '')
	MfaPushReceiver     string
	MfaPushProvider     string
	MfaPushEnabled      bool
	MfaRememberDeadline sql.NullTime // TIMESTAMP NULL
	LastSigninAt        sql.NullTime // TIMESTAMP NULL
	LastSigninIP        string       // COALESCE(last_signin_ip, '')
	Balance             float64
	BalanceCredit       float64
	BalanceCurrency     string
	IsDeleted           bool
	IpWhitelist         string
	Addresses           []byte // COALESCE(addresses, '[]')
	ManagedAccounts     []byte
	FaceIds             []byte
	Cart                []byte
	MfaItems            []byte
	MfaAccounts         []byte
	ApplicationScopes   []byte
	Custom1             string
	Custom2             string
	Custom3             string
	Custom4             string
	Custom5             string
	Custom6             string
	Custom7             string
	Custom8             string
	Custom9             string
	Custom10            string
	CreatedAt           time.Time
	UpdatedAt           time.Time
	DeletedAt           sql.NullTime // TIMESTAMP NULL
	Revision            int64
}

type userProfileRow struct {
	UserID                string
	DisplayName           string // COALESCE(display_name, '')
	FirstName             string // COALESCE(first_name, '')
	LastName              string // COALESCE(last_name, '')
	AvatarURL             string // COALESCE(avatar_url, '')
	Email                 string // COALESCE(email, '')
	EmailVerified         bool
	Phone                 string // COALESCE(phone, '')
	PhoneVerified         bool
	CountryCode           string // COALESCE(country_code, '')
	Region                string // COALESCE(region, '')
	Location              string // COALESCE(location, '')
	Birthday              string // COALESCE(CAST(birthday AS CHAR), '')
	Gender                string // COALESCE(gender, '')
	Bio                   string // COALESCE(bio, '')
	RealName              string // COALESCE(real_name, '')
	IDCardType            string // COALESCE(id_card_type, '')
	IDCard                string // COALESCE(id_card, '')
	PassportNo            string // COALESCE(passport_no, '')
	DriverLicenseNo       string // COALESCE(driver_license_no, '')
	IDCardExpiry          string // COALESCE(CAST(id_card_expiry AS CHAR), '')
	IssueCountry          string // COALESCE(issue_country, '')
	IssuingAuthority      string // COALESCE(issuing_authority, '')
	PassportPhotoURL      string // COALESCE(passport_photo_url, '')
	DriverLicenseFrontURL string // COALESCE(driver_license_front_url, '')
	DriverLicenseBackURL  string // COALESCE(driver_license_back_url, '')
	Properties            []byte // JSON NOT NULL
}

type userCredentialsRow struct {
	UserID                    string
	PasswordHash              string
	PasswordAlgo              string
	PasswordUpdatedAt         sql.NullTime // TIMESTAMP NULL
	AccessKey                 string
	AccessSecretHash          string
	AccessSecretRotatedAt     sql.NullTime // TIMESTAMP NULL
	NeedUpdatePassword        bool
	AccessSecretHashPrev      string
	AccessSecretPrevExpiresAt sql.NullTime // TIMESTAMP NULL
}

type userIdentityRow struct {
	ID              string
	UserID          string
	ProviderID      string
	IdentityType    string
	Subject         string
	ProviderAccount string
	ProfileSnapshot []byte // JSON NOT NULL
	LinkedAt        time.Time
}

type userMfaMethodRow struct {
	ID          string
	UserID      string
	MethodType  string
	Enabled     bool
	Verified    bool
	DisplayName string
	LastUsedAt  sql.NullTime // TIMESTAMP NULL
}
