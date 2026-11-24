package kerberos

import (
	appauth "github.com/guliangiam/guliangiam/internal/app/auth"
)

// Handler handles Kerberos authentication HTTP requests.
type Handler struct {
	AuthService *appauth.Service
}
