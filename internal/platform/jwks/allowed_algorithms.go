package jwks

import (
	"fmt"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

var AllowedSigningAlgorithms = map[string]struct{}{
	"RS256":   {},
	"SM2-SM3": {},
	"RS384":   {},
	"RS512":   {},
	"ES256":   {},
	"ES384":   {},
	"ES512":   {},
	"PS256":   {},
	"PS384":   {},
	"PS512":   {},
}

var ForbiddenSigningAlgorithms = map[string]struct{}{
	"none":  {},
	"HS256": {},
	"HS384": {},
	"HS512": {},
}

func ValidateSigningAlgorithm(alg string) error {
	if _, ok := ForbiddenSigningAlgorithms[alg]; ok {
		return apperrors.NewForbiddenf("signing algorithm %q is explicitly forbidden", alg)
	}

	if _, ok := AllowedSigningAlgorithms[alg]; !ok {
		return apperrors.NewInvalidArgumentf("signing algorithm %q is not allowed; allowed: %v", alg, allowedAlgorithmList())
	}

	return nil
}

func allowedAlgorithmList() []string {
	list := make([]string, 0, len(AllowedSigningAlgorithms))
	for alg := range AllowedSigningAlgorithms {
		list = append(list, alg)
	}
	return list
}

func IsAlgorithmAllowed(alg string) bool {
	_, ok := AllowedSigningAlgorithms[alg]
	return ok
}

func FormatAllowedAlgorithms() string {
	return fmt.Sprintf("%v", allowedAlgorithmList())
}
