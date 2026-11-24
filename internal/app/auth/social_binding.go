package auth

import (
	"context"
	"log/slog"
	"regexp"
	"strings"

	"github.com/google/uuid"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	applicationdomain "github.com/guliangiam/guliangiam/internal/domain/applications"
	groupsdomain "github.com/guliangiam/guliangiam/internal/domain/groups"
	providerdomain "github.com/guliangiam/guliangiam/internal/domain/providers"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// ensureProviderUsable applies the row-level kill switches: the enabled flag
// and the Casdoor-compatible state value ("Disabled"). Both are checked before
// the authorization code is exchanged so a turned-off provider never reaches
// the IdP.
func ensureProviderUsable(provider *providerdomain.Provider) error {
	if provider == nil {
		return apperrors.NewUnavailable("provider is not configured")
	}
	if !provider.Enabled || strings.EqualFold(strings.TrimSpace(provider.State), "disabled") {
		return apperrors.NewUnavailablef("provider %q is disabled", provider.Name)
	}
	return nil
}

// validateProviderEmail applies the provider's emailRegex to the address the
// IdP vouched for. An empty pattern keeps the previous behaviour (no gate).
func validateProviderEmail(provider *providerdomain.Provider, email string) error {
	if provider == nil {
		return nil
	}
	pattern := strings.TrimSpace(provider.EmailRegex)
	if pattern == "" {
		return nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return apperrors.NewInternalf("provider emailRegex is invalid: %v", err)
	}
	if !re.MatchString(strings.TrimSpace(email)) {
		return apperrors.NewUnauthorized("provider returned an email that does not match emailRegex")
	}
	return nil
}

// providerItemForApplication finds the application row that binds this
// provider. The console stores provider IDs, while imported Casdoor data may
// still carry provider names, so both spellings match.
func providerItemForApplication(application *applicationdomain.Application, provider *providerdomain.Provider) *applicationdomain.ProviderItem {
	if application == nil || provider == nil {
		return nil
	}
	for i := range application.ProviderItems {
		item := &application.ProviderItems[i]
		if item.Provider == provider.ID || item.Provider == provider.Name {
			return item
		}
	}
	return nil
}

// socialBindingRules returns the account-matching rules for a provider item.
// A nil item or nil rule list keeps the historical email-only behaviour; an
// explicitly empty list disables automatic account binding, as Casdoor does.
func socialBindingRules(item *applicationdomain.ProviderItem) []string {
	if item == nil || item.BindingRule == nil {
		return []string{"Email"}
	}
	rules := make([]string, 0, len(item.BindingRule))
	for _, rule := range item.BindingRule {
		rule = strings.ToLower(strings.TrimSpace(rule))
		if rule != "" {
			rules = append(rules, rule)
		}
	}
	return rules
}

// matchExistingSocialUser applies the provider item's binding rules in order.
// Email is the historical default; Phone and Name are opt-in. The IdP adapters
// do not expose verification flags today, so a match only requires the value to
// be present — the same trust level the email binding already had.
func (s *Service) matchExistingSocialUser(ctx context.Context, tx appuow.UnitOfWork, organizationID string, userInfo *SocialUserInfo, item *applicationdomain.ProviderItem) (*userdomain.User, error) {
	for _, rule := range socialBindingRules(item) {
		var (
			user *userdomain.User
			err  error
		)
		switch rule {
		case "email":
			if strings.TrimSpace(userInfo.Email) == "" {
				continue
			}
			user, err = s.matchUserByEmail(ctx, tx, organizationID, userInfo.Email)
		case "phone":
			if strings.TrimSpace(userInfo.Phone) == "" {
				continue
			}
			user, err = s.matchUserByPhone(ctx, tx, organizationID, userInfo.Phone)
		case "name":
			if strings.TrimSpace(userInfo.Username) == "" {
				continue
			}
			user, err = s.matchUserByName(ctx, tx, organizationID, userInfo.Username)
		default:
			continue
		}
		if err == nil && user != nil {
			return user, nil
		}
		if err != nil && !isNotFoundError(err) {
			return nil, err
		}
	}
	return nil, nil
}

func (s *Service) matchUserByPhone(ctx context.Context, tx appuow.UnitOfWork, organizationID, phone string) (*userdomain.User, error) {
	users, _, err := tx.Users().List(ctx, userdomain.ListQuery{
		OrganizationID: organizationID,
		PageSize:       100,
	})
	if err != nil {
		return nil, err
	}
	phone = strings.TrimSpace(phone)
	for _, user := range users {
		if strings.TrimSpace(user.Profile.Phone) == phone {
			return user, nil
		}
	}
	return nil, apperrors.NewNotFound("user not found by phone")
}

func (s *Service) matchUserByName(ctx context.Context, tx appuow.UnitOfWork, organizationID, username string) (*userdomain.User, error) {
	users, _, err := tx.Users().List(ctx, userdomain.ListQuery{
		OrganizationID: organizationID,
		PageSize:       100,
		SearchField:    "username",
		SearchValue:    username,
	})
	if err != nil {
		return nil, err
	}
	for _, user := range users {
		if strings.EqualFold(user.Username, username) {
			return user, nil
		}
	}
	return nil, apperrors.NewNotFound("user not found by username")
}

// assignSignupGroup adds the newly registered user to the group named by the
// provider item. A stale group name is logged and skipped: it must not turn a
// successful social registration into a failed login.
func (s *Service) assignSignupGroup(ctx context.Context, tx appuow.UnitOfWork, userID, organizationID, group string) error {
	group = strings.TrimSpace(group)
	if group == "" {
		return nil
	}
	groupID, err := resolveGroupID(ctx, tx, organizationID, group)
	if err != nil {
		return err
	}
	if groupID == "" {
		slog.WarnContext(ctx, "provider signupGroup does not exist", "organization_id", organizationID, "group", group)
		return nil
	}
	return tx.Users().ReplaceGroupMemberships(ctx, userID, []userdomain.UserGroupMembership{
		{UserID: userID, GroupID: groupID},
	})
}

func resolveGroupID(ctx context.Context, tx appuow.UnitOfWork, organizationID, group string) (string, error) {
	if parsed, err := uuid.Parse(group); err == nil {
		found, err := tx.Groups().GetByID(ctx, parsed.String())
		if err != nil {
			if isNotFoundError(err) {
				return "", nil
			}
			return "", err
		}
		if found.OrganizationID == organizationID {
			return found.ID, nil
		}
		return "", nil
	}
	groups, _, err := tx.Groups().List(ctx, groupsdomain.ListQuery{
		OrganizationID: organizationID,
		Keyword:        group,
		PageSize:       100,
	})
	if err != nil {
		return "", err
	}
	for _, candidate := range groups {
		if candidate.Name == group {
			return candidate.ID, nil
		}
	}
	return "", nil
}
