package idv

// FaceAction identifies the business semantics of a face-verification request.
// The provider adapter routes to the concrete product by action:
//   - real_name_verify_cn: domestic authoritative-source real-name (ID_PRO/ID_PLUS)
//   - login_verify: liveness + 1:1 comparison against the enrolled face (LR_FR)
//   - real_name_verify_overseas: international eKYC (eKYC_PRO)
type FaceAction string

const (
	ActionRealNameVerifyCN       FaceAction = "real_name_verify_cn"
	ActionLoginVerify            FaceAction = "login_verify"
	ActionRealNameVerifyOverseas FaceAction = "real_name_verify_overseas"
)

// Valid reports whether the action satisfies ck_idv_action. The empty value is
// allowed for legacy rows and defaults to domestic real-name verification.
func (a FaceAction) Valid() bool {
	switch a {
	case "", ActionRealNameVerifyCN, ActionLoginVerify, ActionRealNameVerifyOverseas:
		return true
	default:
		return false
	}
}

// IsRealName reports whether a successful result marks the user real-name
// verified. Login verification never flips the verified flag.
func (a FaceAction) IsRealName() bool {
	return a == ActionRealNameVerifyCN || a == ActionRealNameVerifyOverseas || a == ""
}
