package users

type UserProfile struct {
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
	// IDCardExpiry 为证件有效期（YYYY-MM-DD 文本，与 Birthday 同模式）。
	IDCardExpiry          string
	IssueCountry          string
	IssuingAuthority      string
	PassportPhotoURL      string
	DriverLicenseFrontURL string
	DriverLicenseBackURL  string
	Affiliation           string
	Properties            map[string]any
}
