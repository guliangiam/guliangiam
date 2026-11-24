package web3identity

import "time"

// Mapping 表示 Passkey → EVM 地址的确定性映射。
// 一个 credential（webauthn_credentials）产生一个 EVM 地址，
// 同一个地址永远来自同一个 passkey 公钥。
type Mapping struct {
	ID               string
	UserID           string
	CredentialID     string
	WalletAddress    string
	DerivationMethod string
	DerivationSeed   string
	ChainIDs         []int64
	EnsName          *string
	IsPrimary        bool
	VerifiedAt       time.Time
	LastUsedAt       *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
	Revision         int64
}
