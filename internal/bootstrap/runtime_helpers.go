package bootstrap

import (
	"os"
	"strconv"
	"strings"
)

func envOrEmpty(key string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return ""
}

// envFloat parses an optional numeric environment value. An absent or malformed
// value returns 0 so the caller's default applies.
func envFloat(key string) float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(envOrEmpty(key)), 64)
	if err != nil {
		return 0
	}
	return v
}

// envInt parses an optional integer environment value; an absent or malformed
// value returns def so the caller's default applies.
func envInt(key string, def int) int {
	v, err := strconv.Atoi(strings.TrimSpace(envOrEmpty(key)))
	if err != nil {
		return def
	}
	return v
}
