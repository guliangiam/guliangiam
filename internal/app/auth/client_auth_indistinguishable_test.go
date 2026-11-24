package auth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	applicationdomain "github.com/guliangiam/guliangiam/internal/domain/applications"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// TestUnknownClientAndWrongSecretAreIndistinguishable 锁定“客户端不存在”与
// “密钥错误”在 token 端点对调用方完全不可区分：同一 invalid_client、同一
// 401、同一文案。实现上两条路径都会先烧掉一次 argon2id 校验
// （client_auth.go / client_credentials.go 的 BurnClientSecretVerification），
// 因此耗时口径可比。
func TestUnknownClientAndWrongSecretAreIndistinguishable(t *testing.T) {
	application := &applicationdomain.Application{
		ID:                      testAppID,
		OrganizationID:          testOrgID,
		ClientID:                "gl_cid_basic",
		ClientSecretHash:        mustClientSecretHash(t, "correct-secret"),
		TokenEndpointAuthMethod: applicationdomain.TokenEndpointAuthMethodClientSecretBasic,
		GrantTypes:              []string{clientCredentialsGrantType},
	}
	service, _ := newClientCredentialsFixture(t, nil, application)

	_, wrongSecretErr := service.ExchangeClientCredentials(context.Background(), ExchangeClientCredentialsCommand{
		ClientID:     application.ClientID,
		ClientSecret: "wrong-secret",
	})
	_, unknownClientErr := service.ExchangeClientCredentials(context.Background(), ExchangeClientCredentialsCommand{
		ClientID:     "gl_cid_unknown",
		ClientSecret: "wrong-secret",
	})

	require.Error(t, wrongSecretErr)
	require.Error(t, unknownClientErr)

	for name, err := range map[string]error{"wrong_secret": wrongSecretErr, "unknown_client": unknownClientErr} {
		var protocolErr apperrors.ProtocolCodeError
		require.ErrorAs(t, err, &protocolErr, name)
		assert.Equal(t, "invalid_client", protocolErr.ProtocolCode(), name)
		var statusErr interface{ HTTPStatus() int }
		require.ErrorAs(t, err, &statusErr, name)
		assert.Equal(t, 401, statusErr.HTTPStatus(), name)
	}

	// 文案必须逐字相同，调用方无法据此区分两种失败。
	assert.Equal(t, wrongSecretErr.Error(), unknownClientErr.Error())
}
