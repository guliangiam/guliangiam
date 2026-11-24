package notificationproviders

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainnotificationproviders "github.com/guliangiam/guliangiam/internal/domain/notificationproviders"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// providerType is a closed set (手册 4.14.3「枚举的开放扩展规则」第 4 条；开放项
// 注册表 OPEN-API-11). Every spelling the
// console used to send must be rejected: none of them matched a key the
// notification factory registers, so a provider created in the console could
// never deliver anything.
func TestValidateCreateNotificationProviderCommand_RejectsUnknownProviderType(t *testing.T) {
	rejected := []string{
		// Historically sent by the console.
		"DingTalk", "WeCom", "Lark", "Telegram", "Discord", "Slack", "Bark", "CustomHTTP",
		// customhttp/googlechat/cucloud are now registered through per-record
		// builders (V4.227), so they must not appear in the rejection list.
		"", "  ", "not-a-notifier",
	}
	for _, providerType := range rejected {
		t.Run("reject_"+providerType, func(t *testing.T) {
			cmd := CreateNotificationProviderCommand{
				OrganizationID: "org-001",
				Name:           "provider",
				ProviderType:   providerType,
			}
			require.Error(t, validateCreateNotificationProviderCommand(cmd),
				"providerType %q must not be accepted", providerType)
		})
	}
}

func TestValidateCreateNotificationProviderCommand_UnknownTypeIs422WithAllowedValues(t *testing.T) {
	cmd := CreateNotificationProviderCommand{
		OrganizationID: "org-001",
		Name:           "provider",
		ProviderType:   "DingTalk",
	}

	err := validateCreateNotificationProviderCommand(cmd)
	require.Error(t, err)

	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeUnprocessableEntity, appErr.Code)
	require.NotNil(t, appErr.Meta)
	assert.Equal(t, "providerType", appErr.Meta["field"])
	assert.Equal(t, "DingTalk", appErr.Meta["value"])
	allowed, ok := appErr.Meta["allowed"].([]string)
	require.True(t, ok)
	assert.ElementsMatch(t, domainnotificationproviders.ProviderTypes.Values(), allowed)
}

func TestValidateCreateNotificationProviderCommand_BlankTypeIs400(t *testing.T) {
	cmd := CreateNotificationProviderCommand{OrganizationID: "org-001", Name: "provider"}

	err := validateCreateNotificationProviderCommand(cmd)
	require.Error(t, err)

	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeInvalidArgument, appErr.Code)
}

func TestValidatePatchNotificationProviderCommand_RejectsUnknownButAllowsAbsent(t *testing.T) {
	unknown := "WeCom"
	err := validatePatchNotificationProviderCommand(PatchNotificationProviderCommand{ProviderType: &unknown})
	require.Error(t, err)

	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeUnprocessableEntity, appErr.Code)

	ok := "wecom"
	assert.NoError(t, validatePatchNotificationProviderCommand(PatchNotificationProviderCommand{ProviderType: &ok}))
	assert.NoError(t, validatePatchNotificationProviderCommand(PatchNotificationProviderCommand{}))
}

func TestValidateCreateNotificationProviderCommand_AcceptsEveryDeclaredType(t *testing.T) {
	for _, providerType := range domainnotificationproviders.ProviderTypes.Values() {
		t.Run(providerType, func(t *testing.T) {
			cmd := CreateNotificationProviderCommand{
				OrganizationID: "org-001",
				Name:           "provider",
				ProviderType:   providerType,
			}
			assert.NoError(t, validateCreateNotificationProviderCommand(cmd))
		})
	}
}
