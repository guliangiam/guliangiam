package auth

import (
	"context"
	"log/slog"
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// MaskedContactResult contains the masked contact information for a user.
type MaskedContactResult struct {
	MaskedEmail string `json:"maskedEmail,omitempty"`
	MaskedPhone string `json:"maskedPhone,omitempty"`
}

// GetMaskedContact returns the masked email and/or phone for a user,
// used in the password reset flow to show which contact method will receive the code.
func (s *Service) GetMaskedContact(ctx context.Context, organizationSlug, username string) (*MaskedContactResult, error) {
	if strings.TrimSpace(organizationSlug) == "" {
		return nil, apperrors.NewInvalidArgument("organizationSlug is required")
	}
	if strings.TrimSpace(username) == "" {
		return nil, apperrors.NewInvalidArgument("username is required")
	}

	org, err := s.UOW.Organizations().GetBySlug(ctx, organizationSlug)
	if err != nil {
		// Return empty to prevent org enumeration; log the lookup failure.
		slog.DebugContext(ctx, "masked contact organization lookup failed", "error", err)
		return &MaskedContactResult{}, nil
	}

	user, err := s.UOW.Users().GetByUsername(ctx, org.ID, username)
	if err != nil {
		// Return empty to prevent user enumeration; log the lookup failure.
		slog.DebugContext(ctx, "masked contact user lookup failed", "error", err)
		return &MaskedContactResult{}, nil
	}

	result := &MaskedContactResult{}
	if user.Profile.Email != "" {
		result.MaskedEmail = maskEmail(user.Profile.Email)
	}
	if user.Profile.Phone != "" {
		result.MaskedPhone = maskPhone(user.Profile.Phone)
	}

	return result, nil
}

// maskEmail masks an email address: "johndoe@gmail.com" -> "j*****@gmail.com"
func maskEmail(email string) string {
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 {
		return "***"
	}
	local := parts[0]
	domain := parts[1]
	if len(local) <= 1 {
		return "*" + "@" + domain
	}
	return string(local[0]) + strings.Repeat("*", len(local)-1) + "@" + domain
}

// maskPhone masks a phone number: "13812345678" -> "138****5678"
func maskPhone(phone string) string {
	if len(phone) <= 7 {
		return strings.Repeat("*", len(phone))
	}
	return phone[:3] + strings.Repeat("*", len(phone)-7) + phone[len(phone)-4:]
}
