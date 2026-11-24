package users

import "time"

type userRow struct {
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
	CreatedIp           string
	SignupApplicationID string
	RegisterType        string
	RegisterSource      string
	Tag                 string
	Score               int
	SigninWrongTimes    int
	LastSigninWrongTime *time.Time
	PreferredMfaType    string
	RecoveryCodeHashes  []string
	TotpSecret          string
	MfaRadiusUsername   string
	MfaRadiusProvider   string
	MfaPushReceiver     string
	MfaPushProvider     string
	MfaPushEnabled      bool
	MfaRememberDeadline *time.Time
	LastSigninAt        *time.Time
	LastSigninIP        string
	Balance             float64
	BalanceCredit       float64
	BalanceCurrency     string
	IsDeleted           bool
	IpWhitelist         string
	Addresses           []byte
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
	DeletedAt           *time.Time
	Revision            int64
}

type userProfileRow struct {
	UserID          string
	DisplayName     string
	FirstName       string
	LastName        string
	AvatarURL       string
	Email           string
	EmailVerified   bool
	Phone           string
	PhoneVerified   bool
	CountryCode     string
	Region          string
	Location        string
	Birthday        string
	Gender          string
	Bio             string
	RealName        string
	IDCardType      string
	IDCard          string
	PassportNo      string
	DriverLicenseNo string
	// V19 `_sealed` twins of the three document numbers. NULL marks a legacy
	// row whose plaintext has not been backfilled yet.
	IDCardSealed          *string
	PassportNoSealed      *string
	DriverLicenseNoSealed *string
	IDCardExpiry          string
	IssueCountry          string
	IssuingAuthority      string
	PassportPhotoURL      string
	DriverLicenseFrontURL string
	DriverLicenseBackURL  string
	Properties            []byte
}

type userCredentialsRow struct {
	UserID                    string
	PasswordHash              string
	PasswordAlgo              string
	PasswordUpdatedAt         *time.Time
	AccessKey                 string
	AccessSecretHash          string
	AccessSecretRotatedAt     *time.Time
	NeedUpdatePassword        bool
	AccessSecretHashPrev      string
	AccessSecretPrevExpiresAt *time.Time
}

type userIdentityRow struct {
	ID              string
	UserID          string
	ProviderID      string
	IdentityType    string
	Subject         string
	ProviderAccount string
	ProfileSnapshot []byte
	LinkedAt        time.Time
}

type userMfaMethodRow struct {
	ID          string
	UserID      string
	MethodType  string
	Enabled     bool
	Verified    bool
	DisplayName string
	LastUsedAt  *time.Time
}
