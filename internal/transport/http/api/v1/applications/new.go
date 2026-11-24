package applications

import (
	appapplications "github.com/guliangiam/guliangiam/internal/app/applications"
)

func NewHandler(service appapplications.Service, jwtIssuer TokenVerifier) Handler {
	return Handler{Service: service, JWTIssuer: jwtIssuer}
}
