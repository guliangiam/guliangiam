package auth

import (
	appauth "github.com/guliangiam/guliangiam/internal/app/auth"
)

func NewHandler(service *appauth.Service, jwtIssuer TokenVerifier) Handler {
	return Handler{Service: service, JWTIssuer: jwtIssuer}
}
