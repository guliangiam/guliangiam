package jwks

// IDTokenSigningAlgorithms is the single source of the ID token signing
// algorithms advertised by OIDC discovery and the well-known fallback.
//
// Callers must copy the slice into their response document:
//
//	append([]string(nil), jwks.IDTokenSigningAlgorithms...)
//
// so a caller cannot mutate the package-level source of truth through a
// response struct.
var IDTokenSigningAlgorithms = []string{
	SigningAlgorithmRS256,
	SigningAlgorithmSM2SM3,
}
