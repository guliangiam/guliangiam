package users

import (
	"context"
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// MaskedContactResult contains masked email and phone for password reset flows.
type MaskedContactResult struct {
	Email string `json:"email"`
	Phone string `json:"phone"`
}

// GetMaskedContact returns masked contact information (email/phone) for a user.
// Used by the forgot-password flow to show the user where a verification code was sent
// without revealing the full contact details.
func (s *Service) GetMaskedContact(ctx context.Context, userID, organizationID string) (*MaskedContactResult, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	// PII is still tenant data: resolve ownership through the scoped lookup so
	// a foreign user id surfaces exactly like a missing one.
	user, err := s.UOW.Users().GetByIDForOrganization(ctx, userID, organizationID)
	if err != nil {
		return nil, apperrors.NewNotFound("user not found")
	}

	result := &MaskedContactResult{}

	// Mask email: show first character and domain, e.g. "a***@example.com"
	email := user.Profile.Email
	if email != "" {
		parts := strings.SplitN(email, "@", 2)
		if len(parts) == 2 {
			local := parts[0]
			domain := parts[1]
			if len(local) > 1 {
				result.Email = string(local[0]) + "***@" + domain
			} else {
				result.Email = "***@" + domain
			}
		}
	}

	// Mask phone: show first 3 and last 4 digits, e.g. "138****1234"
	phone := user.Profile.Phone
	if phone != "" {
		if len(phone) >= 7 {
			result.Phone = phone[:3] + "****" + phone[len(phone)-4:]
		} else if phone != "" {
			result.Phone = strings.Repeat("*", len(phone))
		}
	}

	return result, nil
}
