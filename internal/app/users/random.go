package users

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

func generateClientID() string {
	return generateHex(10)
}

func generateClientSecret() string {
	return generateHex(20)
}

func generateHex(charLen int) string {
	byteLen := (charLen + 1) / 2
	b := make([]byte, byteLen)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%0*x", charLen, 0)
	}
	s := hex.EncodeToString(b)
	if len(s) > charLen {
		return s[:charLen]
	}
	return s
}
