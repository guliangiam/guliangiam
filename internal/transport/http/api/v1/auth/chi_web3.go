package auth

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

// registerWeb3Routes registers Web3 login routes.
//
// DEPRECATED — as of Phase 2 these endpoints are disabled. Web3 wallet login
// based on personal_sign is phased out because (a) it exposes users to
// phishing (no domain binding on the wallet sign dialog), and (b) every
// login requires an interactive wallet signature.
//
// The replacement path is Passkey (WebAuthn secp256r1), which binds to the
// exact origin that issued the challenge. After each successful Passkey
// registration the backend deterministically derives an EVM address from
// the passkey public key via keccak256(compressed_pubkey || "guliangiam-web3-v1")[12:]
// and stores it in guliangiam.web3_identity_mappings. The derived address
// is surfaced in Session.DeviceInfo["web3Address"].
//
// Frontend consumers should migrate to the standard Passkey endpoints:
//
//	POST /api/v1/auth/webauthn/register/init
//	POST /api/v1/auth/webauthn/register/complete
//	POST /api/v1/auth/webauthn/authenticate/init
//	POST /api/v1/auth/webauthn/authenticate/complete
func registerWeb3Routes(router chi.Router, handler Handler) {
	// Legacy wallet-sign endpoints — return 410 Gone with migration hint.
	deprecated := func(w http.ResponseWriter, r *http.Request) {
		route.WriteJSON(w, http.StatusGone, map[string]any{
			"code":    "WEB3_WALLET_LOGIN_DEPRECATED",
			"message": "Wallet-signature Web3 login is deprecated. Use Passkey (WebAuthn) with automatic EVM address derivation instead. See /docs/auth/web3-migration.",
		})
	}
	router.Post("/web3", deprecated)
	router.Post("/web3/verify", deprecated)
}
