package idv

import "context"

type VerificationStatus string

const (
	StatusPending    VerificationStatus = "pending"
	StatusInProgress VerificationStatus = "in_progress"
	StatusApproved   VerificationStatus = "approved"
	StatusRejected   VerificationStatus = "rejected"
	StatusExpired    VerificationStatus = "expired"
	// StatusError is a provider-side transient failure. It is never persisted:
	// identity_verifications.ck_idv_status only allows pending/in_progress/
	// approved/rejected/expired, so callers must leave the row pending and
	// surface the platform error instead.
	StatusError VerificationStatus = "error"
)

// IsTerminal reports whether no further provider polling is required.
func (s VerificationStatus) IsTerminal() bool {
	switch s {
	case StatusApproved, StatusRejected, StatusExpired:
		return true
	default:
		return false
	}
}

// Persistable reports whether the status satisfies ck_idv_status.
func (s VerificationStatus) Persistable() bool {
	switch s {
	case StatusPending, StatusInProgress, StatusApproved, StatusRejected, StatusExpired:
		return true
	default:
		return false
	}
}

type VerificationResult struct {
	VerificationID string
	Status         VerificationStatus
	ProviderName   string
	FirstName      string
	LastName       string
	DateOfBirth    string
	Nationality    string
	IDType         string
	IDNumber       string
	Confidence     float64
	Reason         string
	// GatewayCode is the raw provider sub-code (e.g. Aliyun SubCode). It is
	// audit-safe (no PII) and complements the human-readable Reason.
	GatewayCode string
}

type InitiateResult struct {
	VerificationID string
	RedirectURL    string
	SessionToken   string
	ExpiresAt      string
	// ProductCode is the provider product actually used for this session
	// (ID_PRO/ID_PLUS/LR_FR/eKYC_PRO/...), persisted for result-query routing.
	ProductCode string
}

type InitiateOptions struct {
	UserID         string
	OrganizationID string
	RedirectURL    string
	Locale         string
	Country        string
	IDType         string
	// FullName and IDNumber are forwarded to providers that require the
	// claimed identity in the initiate call (Aliyun CloudAuth). Providers with
	// a hosted capture flow (Jumio) ignore them. They must never be logged.
	FullName string
	IDNumber string
	// Action selects the business semantics; empty defaults to domestic
	// real-name verification for backward compatibility.
	Action FaceAction
	// BizSeq is the server-generated 32-char hex merchant business sequence.
	// It maps to Aliyun OuterOrderNo / MerchantBizId and is persisted under a
	// unique constraint for idempotency and anti-replay.
	BizSeq string
	// MetaInfo is the device environment JSON obtained by the frontend SDK
	// (getMetaInfo). Aliyun requires the real client value; hardcoding it can
	// prevent CertifyUrl issuance.
	MetaInfo string
	// TargetFaceURL is the enrolled face reference for 1:1 login verification
	// (Aliyun PV_FV FaceContrastPictureUrl on Web/H5; LR_FR is the native-SDK
	// equivalent). Ignored by real-name actions.
	TargetFaceURL string
}

// ProviderCredentials is the tenant-scoped provider configuration resolved
// from the encrypted providers row plus non-secret provider_configs options.
// It never carries plaintext secrets outside the app/infra boundary beyond the
// short-lived verifier instance.
type ProviderCredentials struct {
	// Provider is the registration key persisted on identity_verifications
	// (jumio / aliyun_idv / tencent_idv).
	Provider  string
	Endpoint  string
	Region    string
	APIKey    string
	APISecret string
	// WebhookSecret is optional. Jumio signs callbacks; Aliyun CloudAuth has no
	// native callback signature (polling is the authoritative path).
	WebhookSecret string
	Sandbox       bool
	// SceneID is required by Aliyun CloudAuth InitFaceVerify and carries the
	// Tencent faceid RuleId (业务流程 ID).
	SceneID string
	// ReturnURL is the post-verification destination configured for the
	// provider (provider_configs.idvReturnUrl). It is used when the caller
	// does not supply one.
	ReturnURL string
	// ProductCode hints the product of an existing session when refreshing a
	// persisted verification (e.g. eKYC_PRO routes result queries to the
	// international CheckResult API). Empty means the provider default.
	ProductCode string
}

type IdentityVerifier interface {
	InitiateVerification(ctx context.Context, opts InitiateOptions) (*InitiateResult, error)
	GetVerificationResult(ctx context.Context, verificationID string) (*VerificationResult, error)
	VerifyCallback(ctx context.Context, payload []byte, headers map[string]string) (*VerificationResult, error)
	// Probe performs a read-only authenticated call that proves the endpoint is
	// reachable and the configured credential is accepted. It must not create
	// a verification session or change provider state.
	Probe(ctx context.Context) error
}

// OrderResultQuerier is an optional extension for providers whose result query
// requires the merchant business sequence in addition to the provider
// reference (Aliyun Cloudauth-intl CheckResult needs MerchantBizId +
// TransactionId). Callers that hold the persisted BizSeq should prefer this
// method and fall back to GetVerificationResult otherwise.
type OrderResultQuerier interface {
	GetVerificationResultWithOrder(ctx context.Context, verificationID, merchantBizID string) (*VerificationResult, error)
}

// VerifierFactory builds a verifier for one resolved provider configuration.
// Implemented by infra and injected at the composition root so app never
// imports infra.
type VerifierFactory interface {
	Build(credentials ProviderCredentials) (IdentityVerifier, error)
	// CallbackReference extracts the opaque provider reference from an
	// unverified callback body. The public callback route needs it to resolve
	// the owning tenant before the signature can be checked with that tenant's
	// secret. Unknown providers must be rejected as not-found.
	CallbackReference(provider string, payload []byte) (string, error)
}
