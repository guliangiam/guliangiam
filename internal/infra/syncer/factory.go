package syncer

import (
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Factory creates sync Engine instances based on provider type.
type Factory struct{}

// NewFactory creates a new syncer factory.
func NewFactory() *Factory {
	return &Factory{}
}

// GetEngine creates a sync Engine based on the provider type.
func (f *Factory) GetEngine(providerType string) (Engine, error) {
	switch strings.ToLower(strings.TrimSpace(providerType)) {
	case "database":
		return &DatabaseSyncer{}, nil
	case "ldap":
		return NewLDAPSyncer(), nil
	case "scim":
		return NewSCIMSyncer(), nil
	case "dingtalk":
		return &DingTalkSyncer{}, nil
	case "lark":
		return &LarkSyncer{}, nil
	case "wecom":
		return &WeComSyncer{}, nil
	case "google":
		return &GoogleSyncer{}, nil
	case "okta":
		return &OktaSyncer{}, nil
	case "azuread":
		return &AzureADSyncer{}, nil
	case "keycloak":
		return &KeycloakSyncer{}, nil
	case "activedirectory", "ad":
		return NewActiveDirectorySyncer(), nil
	case "awsiam":
		return &AWSIAMSyncer{}, nil
	default:
		return nil, apperrors.NewInvalidArgumentf("unsupported syncer provider type: %s", providerType)
	}
}
