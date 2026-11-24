package auth

import (
	"strings"
	"time"

	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	sessiondomain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	"github.com/guliangiam/guliangiam/internal/platform/phonefmt"
)

func toAccountContext(organization *orgdomain.Organization, user *userdomain.User, accessToken string) AccountContext {
	navItems, userNavItems, navGroupOrder := extractNavItems(organization)

	return AccountContext{
		Account: SessionAccount{
			ID:          user.ID,
			Owner:       organization.Slug,
			Name:        user.Username,
			DisplayName: user.Profile.DisplayName,
			Avatar:      user.Profile.AvatarURL,
			IsAdmin:     user.IsAdmin,
		},
		Organization: SessionOrganization{
			ID:            organization.ID,
			Name:          organization.Slug,
			DisplayName:   organization.DisplayName,
			LogoUrl:       organization.LogoURL,
			FaviconUrl:    organization.FaviconURL,
			NavItems:      navItems,
			UserNavItems:  userNavItems,
			NavGroupOrder: navGroupOrder,
		},
		AccessToken:    accessToken,
		PasswordPolicy: resolvePasswordPolicy(organization),
		SessionPolicy:  resolveSessionPolicy(organization),
		// 与登录响应同源：直接读 user_credentials.need_update_password。
		NeedUpdatePassword: user.Credentials.NeedUpdatePassword,
	}
}

// extractNavItems pulls navItem lists from organization.Settings.NavPreferences.
// nav_preferences is a flexible JSONB field; the documented shape is:
//
//	{"navItems": ["/users", "/roles"], "userNavItems": ["/users"], "navGroupOrder": ["identity", "audit"]}
//
// Legacy fallback: older organization settings wrote the admin menu under the
// "adminNavItems" key; it is read when "navItems" is absent so historical data
// keeps working until the next save rewrites the canonical key.
//
// Semantic contract with the frontend:
//   - NavPreferences is empty (nil or zero-length map) → return nil slices
//     → frontend sees configured === null → interprets as "no restrictions, show everything".
//   - NavPreferences is set → the arrays are delivered verbatim (no system-level
//     union), so an administrator can hide any menu group they choose.
//   - navItems/userNavItems are guaranteed non-nil so JSON produces [] rather
//     than null; navGroupOrder is optional and keeps nil when unset (omitempty).
func extractNavItems(org *orgdomain.Organization) (navItemsOut, userNavItemsOut, navGroupOrderOut []string) {
	// Early return: no nav_preferences configured means "show everything".
	if org == nil || org.Settings.NavPreferences == nil || len(org.Settings.NavPreferences) == 0 {
		return nil, nil, nil
	}

	var navItems, userNavItems, navGroupOrder []string
	raw := org.Settings.NavPreferences

	if navRaw, ok := raw["navItems"]; ok {
		navItems = toStringSlice(navRaw)
	} else if legacyRaw, ok := raw["adminNavItems"]; ok {
		navItems = toStringSlice(legacyRaw)
	}
	if userRaw, ok := raw["userNavItems"]; ok {
		userNavItems = toStringSlice(userRaw)
	}
	if orderRaw, ok := raw["navGroupOrder"]; ok {
		navGroupOrder = toStringSlice(orderRaw)
	}

	// Ensure non-nil so JSON serialization produces [] rather than null —
	// the frontend Zod schemas expect arrays, never null.
	if navItems == nil {
		navItems = []string{}
	}
	if userNavItems == nil {
		userNavItems = []string{}
	}

	return navItems, userNavItems, navGroupOrder
}

func toStringSlice(v any) []string {
	switch arr := v.(type) {
	case []any:
		out := make([]string, 0, len(arr))
		for _, item := range arr {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return arr
	default:
		return []string{}
	}
}

func toSessionIdentity(session *sessiondomain.Session) SessionIdentity {
	return SessionIdentity{
		ID:               session.ID,
		UserID:           session.UserID,
		OrganizationID:   session.OrganizationID,
		OrganizationSlug: session.OrganizationSlug,
		Username:         session.Username,
		Sid:              session.Sid,
		AuthTime:         session.CreatedAt,
		IsPrivileged:     session.IsPrivileged,
		LifecycleID:      session.LifecycleID,
		TicketID:         session.TicketID,
	}
}

func toSuccessfulLoginResponse(account AccountContext) LoginResponse {
	accountCopy := account.Account
	organizationCopy := account.Organization
	passwordPolicy := account.PasswordPolicy
	sessionPolicy := account.SessionPolicy

	return LoginResponse{
		Status:             LoginStatusSuccess,
		Account:            &accountCopy,
		Organization:       &organizationCopy,
		AccessToken:        account.AccessToken,
		PasswordPolicy:     &passwordPolicy,
		SessionPolicy:      &sessionPolicy,
		NeedUpdatePassword: account.NeedUpdatePassword,
	}
}

func toMfaRequiredLoginResponse(user *userdomain.User, preferredMfaType string, methods []userdomain.UserMfaMethod, challengeID, challengeState string, challengeExpiresAt time.Time) LoginResponse {
	availableMethods := make([]LoginMfaMethod, 0, len(methods))
	requiredMfa := make([]string, 0, len(methods))
	seen := make(map[string]struct{}, len(methods))
	nextMfa := ""

	for _, method := range methods {
		target, countryCode := loginMfaMethodTarget(user, method.MethodType)
		availableMethods = append(availableMethods, LoginMfaMethod{
			ID:          method.ID,
			MethodType:  method.MethodType,
			DisplayName: method.DisplayName,
			Target:      target,
			CountryCode: countryCode,
		})

		if _, ok := seen[method.MethodType]; !ok {
			seen[method.MethodType] = struct{}{}
			requiredMfa = append(requiredMfa, method.MethodType)
			if nextMfa == "" {
				nextMfa = method.MethodType
			}
		}
	}

	if preferredMfaType != "" {
		for _, methodType := range requiredMfa {
			if methodType == preferredMfaType {
				nextMfa = preferredMfaType
				break
			}
		}
	}

	expiresAt := challengeExpiresAt

	return LoginResponse{
		Status:              LoginStatusMfaRequired,
		NextMfa:             nextMfa,
		ChallengeID:         challengeID,
		ChallengeState:      challengeState,
		ChallengeExpiresAt:  &expiresAt,
		RequiredMfa:         requiredMfa,
		AvailableMfaMethods: availableMethods,
	}
}

// loginMfaMethodTarget returns the delivery target and, for SMS only, a legacy
// calling-code hint. `profile.CountryCode` is ISO 3166-1 alpha-2; an ISO value
// must never leak into `countryCode` here because the login UI concatenates it
// with the phone number. New rows already store E.164 in `profile.Phone`, so the
// hint is empty; only legacy dial-prefixed rows keep a hint.
func loginMfaMethodTarget(user *userdomain.User, methodType string) (target, countryCode string) {
	if user == nil {
		return "", ""
	}

	switch methodType {
	case contactEmail:
		return user.Profile.Email, ""
	case contactSMS:
		return user.Profile.Phone, legacyCallingCodeHint(user.Profile.CountryCode)
	case "radius":
		return user.MfaRadiusUsername, ""
	case contactPush:
		return user.MfaPushReceiver, ""
	default:
		return "", ""
	}
}

func legacyCallingCodeHint(value string) string {
	trimmed := strings.TrimSpace(value)
	if !phonefmt.IsDialCode(trimmed) {
		return ""
	}
	return trimmed
}

func toMfaSetupRequiredLoginResponse(requiredMfa []string, challengeID string, challengeExpiresAt time.Time) LoginResponse {
	expiresAt := challengeExpiresAt

	return LoginResponse{
		Status:             LoginStatusMfaSetup,
		ChallengeID:        challengeID,
		ChallengeExpiresAt: &expiresAt,
		RequiredMfa:        requiredMfa,
	}
}
