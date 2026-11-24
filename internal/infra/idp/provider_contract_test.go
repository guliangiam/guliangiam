package idp

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"testing"

	"github.com/guliangiam/guliangiam/internal/domain/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// contractCall records one intercepted HTTP request.
type contractCall struct {
	Method string
	URL    *url.URL
	Header http.Header
	Body   []byte
}

// contractTransport answers requests from a responder function, letting each
// contract test assert endpoints, parameters and response parsing offline.
type contractTransport struct {
	calls []contractCall
	resp  func(call contractCall) (int, string)
}

func (t *contractTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var body []byte
	if req.Body != nil {
		body, _ = io.ReadAll(req.Body)
	}
	call := contractCall{Method: req.Method, URL: req.URL, Header: req.Header.Clone(), Body: body}
	t.calls = append(t.calls, call)
	status, payload := t.resp(call)
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(bytes.NewReader([]byte(payload))),
		Header:     make(http.Header),
		Request:    req,
	}, nil
}

func contractConfig(t *testing.T, responder func(contractCall) (int, string)) (*OAuthConfig, *contractTransport) {
	t.Helper()
	transport := &contractTransport{resp: responder}
	return &OAuthConfig{
		ClientID:     "fake-client-id",
		ClientSecret: "fake-client-secret",
		RedirectURI:  "https://app.example.com/callback",
		HTTPClient:   &http.Client{Transport: transport},
	}, transport
}

func jsonResponder(status int, payload string) func(contractCall) (int, string) {
	return func(contractCall) (int, string) { return status, payload }
}

// --- WeChat (qrconnect) -----------------------------------------------------

func TestWeChatContractRedirectUsesAppID(t *testing.T) {
	cfg, _ := contractConfig(t, jsonResponder(200, "{}"))
	p := NewWeChatProvider(cfg)

	redirect, err := p.GetRedirectURL("state-1")
	require.NoError(t, err)
	assert.Contains(t, redirect, "open.weixin.qq.com/connect/qrconnect")
	assert.Contains(t, redirect, "appid=fake-client-id")
	assert.Contains(t, redirect, "scope=snsapi_login")
	assert.Contains(t, redirect, "#wechat_redirect")
	assert.NotContains(t, redirect, "client_id=")
	assert.NotContains(t, redirect, "fake-client-secret")
}

func TestWeChatContractExchangeAndUserInfo(t *testing.T) {
	cfg, tr := contractConfig(t, jsonResponder(200, `{"access_token":"AT","expires_in":7200,"refresh_token":"RT","scope":"snsapi_login","openid":"oX-1","unionid":"uX-1"}`))
	p := NewWeChatProvider(cfg)

	token, err := p.ExchangeCode(context.Background(), "code-1")
	require.NoError(t, err)
	require.Len(t, tr.calls, 1)
	call := tr.calls[0]
	assert.Equal(t, http.MethodGet, call.Method)
	assert.Equal(t, "api.weixin.qq.com", call.URL.Host)
	assert.Equal(t, "/sns/oauth2/access_token", call.URL.Path)
	assert.Equal(t, "fake-client-id", call.URL.Query().Get("appid"))
	assert.Equal(t, "fake-client-secret", call.URL.Query().Get("secret"))
	assert.Equal(t, "code-1", call.URL.Query().Get("code"))
	assert.Equal(t, "authorization_code", call.URL.Query().Get("grant_type"))
	assert.Equal(t, "oX-1", token.OpenID)
	assert.Equal(t, "uX-1", token.UnionID)
	assert.Equal(t, "uX-1", token.ID)

	tr.resp = jsonResponder(200, `{"openid":"oX-1","unionid":"uX-1","nickname":"张三","headimgurl":"https://wx.qlogo.cn/a.png"}`)
	info, err := p.GetUserInfo(context.Background(), token)
	require.NoError(t, err)
	require.Len(t, tr.calls, 2)
	userCall := tr.calls[1]
	assert.Equal(t, "/sns/userinfo", userCall.URL.Path)
	assert.Equal(t, "oX-1", userCall.URL.Query().Get("openid"))
	assert.Equal(t, "AT", userCall.URL.Query().Get("access_token"))
	assert.Equal(t, "zh_CN", userCall.URL.Query().Get("lang"))
	assert.Equal(t, "uX-1", info.ID)
	assert.Equal(t, "张三", info.DisplayName)
	assert.Equal(t, "https://wx.qlogo.cn/a.png", info.AvatarURL)
}

func TestWeChatContractErrCodeMapping(t *testing.T) {
	cfg, _ := contractConfig(t, jsonResponder(200, `{"errcode":40029,"errmsg":"invalid code"}`))
	p := NewWeChatProvider(cfg)

	_, err := p.ExchangeCode(context.Background(), "bad")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid code")
}

// --- WeChat Mobile ----------------------------------------------------------

func TestWeChatMobileContractStoresOpenID(t *testing.T) {
	cfg, tr := contractConfig(t, jsonResponder(200, `{"access_token":"AT","expires_in":7200,"refresh_token":"RT","scope":"snsapi_userinfo","openid":"oM-1","unionid":"uM-1"}`))
	p := NewWeChatMobileProvider(cfg)

	token, err := p.ExchangeCode(context.Background(), "code-1")
	require.NoError(t, err)
	require.Len(t, tr.calls, 1)
	assert.Equal(t, "oM-1", token.OpenID)
	assert.Equal(t, "uM-1", token.UnionID)
	assert.Equal(t, "uM-1", token.ID)

	tr.resp = jsonResponder(200, `{"openid":"oM-1","nickname":"移动用户","headimgurl":"https://x/y.png","unionid":"uM-1"}`)
	info, err := p.GetUserInfo(context.Background(), token)
	require.NoError(t, err)
	userCall := tr.calls[1]
	assert.Equal(t, "oM-1", userCall.URL.Query().Get("openid"))
	assert.Equal(t, "uM-1", info.ID)
	assert.Equal(t, "oM-1", info.OpenID)
}

func TestWeChatMobileContractUserInfoErrCode(t *testing.T) {
	cfg, _ := contractConfig(t, jsonResponder(200, `{"errcode":40003,"errmsg":"invalid openid"}`))
	p := NewWeChatMobileProvider(cfg)

	_, err := p.GetUserInfo(context.Background(), &OAuthToken{AccessToken: "AT", OpenID: "missing"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid openid")
}

// --- WeChat MiniProgram -----------------------------------------------------

func TestWeChatMiniProgramContractCode2Session(t *testing.T) {
	cfg, tr := contractConfig(t, jsonResponder(200, `{"openid":"oP-1","session_key":"SK","unionid":"uP-1"}`))
	p := NewWeChatMiniProgramProvider(cfg)

	token, err := p.ExchangeCode(context.Background(), "js-code-1")
	require.NoError(t, err)
	require.Len(t, tr.calls, 1)
	call := tr.calls[0]
	assert.Equal(t, "api.weixin.qq.com", call.URL.Host)
	assert.Equal(t, "/sns/jscode2session", call.URL.Path)
	assert.Equal(t, "fake-client-id", call.URL.Query().Get("appid"))
	assert.Equal(t, "fake-client-secret", call.URL.Query().Get("secret"))
	assert.Equal(t, "js-code-1", call.URL.Query().Get("js_code"))
	assert.Equal(t, "authorization_code", call.URL.Query().Get("grant_type"))
	assert.Equal(t, "SK", token.AccessToken)
	assert.Equal(t, "oP-1", token.OpenID)
	assert.Equal(t, "uP-1", token.ID)

	_, err = p.GetUserInfo(context.Background(), token)
	require.NoError(t, err)
}

func TestWeChatMiniProgramContractErrCode(t *testing.T) {
	cfg, _ := contractConfig(t, jsonResponder(200, `{"errcode":40029,"errmsg":"invalid code"}`))
	p := NewWeChatMiniProgramProvider(cfg)

	_, err := p.GetSessionByCode(context.Background(), "bad")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid code")
}

// --- WeCom Internal ---------------------------------------------------------

func TestWeComInternalContractRedirectUsesAgentID(t *testing.T) {
	cfg, _ := contractConfig(t, jsonResponder(200, "{}"))
	cfg.AgentID = "1000002"
	p := NewWeComInternalProvider(cfg)

	redirect, err := p.GetRedirectURL("state-1")
	require.NoError(t, err)
	assert.Contains(t, redirect, "appid=fake-client-id")
	assert.Contains(t, redirect, "agentid=1000002")
	assert.NotContains(t, redirect, "fake-client-secret")
}

func TestWeComInternalContractRedirectRequiresAgentID(t *testing.T) {
	cfg, _ := contractConfig(t, jsonResponder(200, "{}"))
	p := NewWeComInternalProvider(cfg)

	_, err := p.GetRedirectURL("state-1")
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "agentid")
}

func TestWeComInternalContractTwoStepFlow(t *testing.T) {
	cfg, tr := contractConfig(t, func(call contractCall) (int, string) {
		if strings.HasSuffix(call.URL.Path, "/gettoken") {
			return 200, `{"errcode":0,"errmsg":"ok","access_token":"CT","expires_in":7200}`
		}
		return 200, `{"errcode":0,"errmsg":"ok","UserId":"sam"}`
	})
	cfg.AgentID = "1000002"
	p := NewWeComInternalProvider(cfg)

	token, err := p.ExchangeCode(context.Background(), "code-1")
	require.NoError(t, err)
	require.Len(t, tr.calls, 1)
	getToken := tr.calls[0]
	assert.Equal(t, "qyapi.weixin.qq.com", getToken.URL.Host)
	assert.Equal(t, "/cgi-bin/gettoken", getToken.URL.Path)
	assert.Equal(t, "fake-client-id", getToken.URL.Query().Get("corpid"))
	assert.Equal(t, "fake-client-secret", getToken.URL.Query().Get("corpsecret"))
	assert.Equal(t, "CT", token.AccessToken)

	tr.resp = func(call contractCall) (int, string) {
		switch call.URL.Path {
		case "/cgi-bin/user/getuserinfo":
			return 200, `{"errcode":0,"errmsg":"ok","UserId":"sam"}`
		case "/cgi-bin/user/get":
			return 200, `{"errcode":0,"errmsg":"ok","userid":"sam","name":"张三","email":"sam@corp","avatar":"https://a/b.png"}`
		}
		t.Fatalf("unexpected path %s", call.URL.Path)
		return 0, ""
	}
	info, err := p.GetUserInfo(context.Background(), token)
	require.NoError(t, err)
	require.Len(t, tr.calls, 3)
	userIDCall := tr.calls[1]
	assert.Equal(t, "code-1", userIDCall.URL.Query().Get("code"))
	assert.Equal(t, "CT", userIDCall.URL.Query().Get("access_token"))
	detailCall := tr.calls[2]
	assert.Equal(t, "sam", detailCall.URL.Query().Get("userid"))
	assert.Equal(t, "sam", info.ID)
	assert.Equal(t, "张三", info.DisplayName)
	assert.Equal(t, "sam@corp", info.Email)
}

// --- WeCom Third-party ------------------------------------------------------

func TestWeComThirdPartyContractProviderTokenAndLoginInfo(t *testing.T) {
	cfg, tr := contractConfig(t, func(call contractCall) (int, string) {
		if strings.HasSuffix(call.URL.Path, "/get_provider_token") {
			return 200, `{"errcode":0,"errmsg":"ok","provider_access_token":"PT","expires_in":7200}`
		}
		return 200, `{"errcode":0,"errmsg":"ok","usertype":1,"auth_corp_info":{"corpid":"wx-corp"}}`
	})
	p := NewWeComThirdPartyProvider(cfg)

	token, err := p.ExchangeCode(context.Background(), "auth-code-1")
	require.NoError(t, err)
	require.Len(t, tr.calls, 1)
	call := tr.calls[0]
	assert.Equal(t, http.MethodPost, call.Method)
	assert.Equal(t, "/cgi-bin/service/get_provider_token", call.URL.Path)
	var body map[string]string
	require.NoError(t, json.Unmarshal(call.Body, &body))
	assert.Equal(t, "fake-client-id", body["corpid"])
	assert.Equal(t, "fake-client-secret", body["provider_secret"])
	assert.Equal(t, "PT", token.AccessToken)

	tr.resp = func(call contractCall) (int, string) {
		return 200, `{"errcode":0,"errmsg":"ok","usertype":1,"user_info":{"userid":"sam","name":"张三","avatar":"https://a/b.png"},"corp_info":{"corpid":"wx-corp"}}`
	}
	info, err := p.GetUserInfo(context.Background(), token)
	require.NoError(t, err)
	require.Len(t, tr.calls, 2)
	loginCall := tr.calls[1]
	assert.Equal(t, "/cgi-bin/service/get_login_info", loginCall.URL.Path)
	assert.Equal(t, "PT", loginCall.URL.Query().Get("access_token"))
	var loginBody map[string]string
	require.NoError(t, json.Unmarshal(loginCall.Body, &loginBody))
	assert.Equal(t, "auth-code-1", loginBody["auth_code"])
	assert.Equal(t, "sam", info.ID)
	assert.Equal(t, "张三", info.DisplayName)
}

// --- DingTalk ---------------------------------------------------------------

func TestDingTalkContractJSONTokenAndHeaderUserInfo(t *testing.T) {
	cfg, tr := contractConfig(t, jsonResponder(200, `{"accessToken":"AT","expireIn":7200,"refreshToken":"RT","corpId":"dingX"}`))
	p := NewDingTalkProvider(cfg)

	redirect, err := p.GetRedirectURL("state-1")
	require.NoError(t, err)
	assert.Contains(t, redirect, "login.dingtalk.com/oauth2/auth")
	assert.Contains(t, redirect, "scope=openid")

	token, err := p.ExchangeCode(context.Background(), "code-1")
	require.NoError(t, err)
	require.Len(t, tr.calls, 1)
	call := tr.calls[0]
	assert.Equal(t, http.MethodPost, call.Method)
	assert.Equal(t, "api.dingtalk.com", call.URL.Host)
	assert.Equal(t, "/v1.0/oauth2/userAccessToken", call.URL.Path)
	assert.Equal(t, "application/json", call.Header.Get("Content-Type"))
	var body map[string]string
	require.NoError(t, json.Unmarshal(call.Body, &body))
	assert.Equal(t, "fake-client-id", body["clientId"])
	assert.Equal(t, "fake-client-secret", body["clientSecret"])
	assert.Equal(t, "authorization_code", body["grantType"])
	assert.Equal(t, "code-1", body["code"])
	assert.Equal(t, "AT", token.AccessToken)
	assert.Equal(t, int64(7200), token.ExpiresIn)

	tr.resp = jsonResponder(200, `{"nick":"张三","avatarUrl":"https://x/a.png","mobile":"13900000000","openId":"op-1","unionId":"un-1","email":"z@x.com","stateCode":"86"}`)
	info, err := p.GetUserInfo(context.Background(), token)
	require.NoError(t, err)
	require.Len(t, tr.calls, 2)
	userCall := tr.calls[1]
	assert.Equal(t, "/v1.0/contact/users/me", userCall.URL.Path)
	assert.Equal(t, "AT", userCall.Header.Get("x-acs-dingtalk-access-token"))
	assert.NotContains(t, strings.Join(userCall.Header.Values("Authorization"), ","), "Bearer")
	assert.Equal(t, "op-1", info.ID)
	assert.Equal(t, "un-1", info.UnionID)
	assert.Equal(t, "张三", info.DisplayName)
	assert.Equal(t, "13900000000", info.Phone)
	assert.Equal(t, "z@x.com", info.Email)
}

func TestDingTalkContractErrorMapping(t *testing.T) {
	cfg, _ := contractConfig(t, jsonResponder(400, `{"code":"invalidParameter","message":"auth code is invalid"}`))
	p := NewDingTalkProvider(cfg)

	_, err := p.ExchangeCode(context.Background(), "bad")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "auth code is invalid")
}

// --- Lark / Feishu ----------------------------------------------------------

func TestLarkContractV2TokenAndWrappedUserInfo(t *testing.T) {
	cfg, tr := contractConfig(t, jsonResponder(200, `{"access_token":"AT","token_type":"user_access_token","expires_in":7200,"refresh_token":"RT","scope":"contact:user.base:readonly"}`))
	p := NewLarkProvider(cfg)

	token, err := p.ExchangeCode(context.Background(), "code-1")
	require.NoError(t, err)
	require.Len(t, tr.calls, 1)
	call := tr.calls[0]
	assert.Equal(t, "open.feishu.cn", call.URL.Host)
	assert.Equal(t, "/open-apis/authen/v2/oauth/token", call.URL.Path)
	assert.Equal(t, "application/json", call.Header.Get("Content-Type"))
	var body map[string]string
	require.NoError(t, json.Unmarshal(call.Body, &body))
	assert.Equal(t, "fake-client-id", body["client_id"])
	assert.Equal(t, "fake-client-secret", body["client_secret"])
	assert.Equal(t, "authorization_code", body["grant_type"])
	assert.Equal(t, "code-1", body["code"])
	assert.Equal(t, "AT", token.AccessToken)

	tr.resp = jsonResponder(200, `{"code":0,"msg":"success","data":{"union_id":"onX","open_id":"ouX","name":"张三","en_name":"Zhang San","avatar_url":"https://x/a.png","email":"z@x.com","mobile":"+8613900000000"}}`)
	info, err := p.GetUserInfo(context.Background(), token)
	require.NoError(t, err)
	require.Len(t, tr.calls, 2)
	userCall := tr.calls[1]
	assert.Equal(t, "/open-apis/authen/v1/user_info", userCall.URL.Path)
	assert.Equal(t, "Bearer AT", userCall.Header.Get("Authorization"))
	assert.Equal(t, "onX", info.ID)
	assert.Equal(t, "ouX", info.OpenID)
	assert.Equal(t, "张三", info.DisplayName)
	assert.Equal(t, "Zhang San", info.Username)
}

func TestLarkContractErrorAndInternationalDomain(t *testing.T) {
	cfg, _ := contractConfig(t, jsonResponder(400, `{"error":"invalid_grant","error_description":"the code is invalid"}`))
	p := NewLarkProvider(cfg)
	_, err := p.ExchangeCode(context.Background(), "bad")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "the code is invalid")

	intlCfg, intlTr := contractConfig(t, jsonResponder(200, `{"access_token":"AT"}`))
	intlCfg.Host = "open.larksuite.com"
	intl := NewLarkProvider(intlCfg)
	_, err = intl.ExchangeCode(context.Background(), "code-1")
	require.NoError(t, err)
	require.Len(t, intlTr.calls, 1)
	assert.Equal(t, "open.larksuite.com", intlTr.calls[0].URL.Host)
	assert.Equal(t, "/open-apis/authen/v2/oauth/token", intlTr.calls[0].URL.Path)
}

// --- Alipay -----------------------------------------------------------------

func TestAlipayContractSignedGatewayFlow(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	keyPEM := string(pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: mustMarshalPKCS8(t, key),
	}))

	cfg, tr := contractConfig(t, func(call contractCall) (int, string) {
		return 200, `{"alipay_system_oauth_token_response":{"access_token":"AT","expires_in":1234,"open_id":"oAlipay","user_id":"2088"}}`
	})
	cfg.ClientSecret = keyPEM
	p := NewAlipayProvider(cfg)

	redirect, err := p.GetRedirectURL("state-1")
	require.NoError(t, err)
	assert.Contains(t, redirect, "openauth.alipay.com/oauth2/publicAppAuthorize.htm")
	assert.Contains(t, redirect, "app_id=fake-client-id")
	assert.Contains(t, redirect, "scope=auth_user")

	token, err := p.ExchangeCode(context.Background(), "code-1")
	require.NoError(t, err)
	require.Len(t, tr.calls, 1)
	call := tr.calls[0]
	assert.Equal(t, http.MethodPost, call.Method)
	assert.Equal(t, "openapi.alipay.com", call.URL.Host)
	assert.Equal(t, "/gateway.do", call.URL.Path)

	form, err := url.ParseQuery(string(call.Body))
	require.NoError(t, err)
	assert.Equal(t, "alipay.system.oauth.token", form.Get("method"))
	assert.Equal(t, "authorization_code", form.Get("grant_type"))
	assert.Equal(t, "code-1", form.Get("code"))
	assert.Equal(t, "RSA2", form.Get("sign_type"))
	assert.Equal(t, "utf-8", form.Get("charset"))
	assert.NotEmpty(t, form.Get("sign"))
	assertAlipaySignature(t, &key.PublicKey, form)

	assert.Equal(t, "AT", token.AccessToken)
	assert.Equal(t, "oAlipay", token.ID)

	tr.resp = func(call contractCall) (int, string) {
		return 200, `{"alipay_user_info_share_response":{"code":"10000","msg":"Success","open_id":"oAlipay","nick_name":"张三","avatar":"https://a/b.png","city":"杭州"}}`
	}
	info, err := p.GetUserInfo(context.Background(), token)
	require.NoError(t, err)
	require.Len(t, tr.calls, 2)
	userForm, err := url.ParseQuery(string(tr.calls[1].Body))
	require.NoError(t, err)
	assert.Equal(t, "alipay.user.info.share", userForm.Get("method"))
	assert.Equal(t, "AT", userForm.Get("auth_token"))
	assert.Equal(t, "oAlipay", info.ID)
	assert.Equal(t, "张三", info.DisplayName)
	assert.Equal(t, "https://a/b.png", info.AvatarURL)
}

func TestAlipayContractErrorResponse(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	cfg, _ := contractConfig(t, func(call contractCall) (int, string) {
		return 200, `{"error_response":{"code":40002,"msg":"Invalid Arguments","sub_code":"isv.invalid-app-id","sub_msg":"应用ID无效"}}`
	})
	cfg.ClientSecret = string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: mustMarshalPKCS8(t, key)}))
	p := NewAlipayProvider(cfg)

	_, err = p.ExchangeCode(context.Background(), "code-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "isv.invalid-app-id")
}

func TestAlipayContractRejectsNonPEMSecret(t *testing.T) {
	cfg, _ := contractConfig(t, jsonResponder(200, "{}"))
	p := NewAlipayProvider(cfg)

	_, err := p.ExchangeCode(context.Background(), "code-1")
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "private key")
}

func mustMarshalPKCS8(t *testing.T, key *rsa.PrivateKey) []byte {
	t.Helper()
	der, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	return der
}

// assertAlipaySignature verifies the RSA2 signature over the sorted gateway
// parameters (excluding sign/sign_type) as mandated by the OpenAPI spec.
func assertAlipaySignature(t *testing.T, pub *rsa.PublicKey, form url.Values) {
	t.Helper()
	signB64 := form.Get("sign")
	require.NotEmpty(t, signB64)
	sig, err := base64.StdEncoding.DecodeString(signB64)
	require.NoError(t, err)

	keys := make([]string, 0, len(form))
	for k := range form {
		if k == "sign" || k == "sign_type" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	pairs := make([]string, 0, len(keys))
	for _, k := range keys {
		pairs = append(pairs, k+"="+form.Get(k))
	}
	digest := sha256.Sum256([]byte(strings.Join(pairs, "&")))
	require.NoError(t, rsa.VerifyPKCS1v15(pub, crypto.SHA256, digest[:], sig))
}

// --- Twitter / X ------------------------------------------------------------

func TestTwitterContractUnwrapsData(t *testing.T) {
	cfg, tr := contractConfig(t, jsonResponder(200, `{"data":{"id":"1234","name":"Foo Bar","username":"foobar"}}`))
	p := NewTwitterProvider(cfg)

	info, err := p.GetUserInfo(context.Background(), &OAuthToken{AccessToken: "AT"})
	require.NoError(t, err)
	require.Len(t, tr.calls, 1)
	assert.Equal(t, "api.twitter.com", tr.calls[0].URL.Host)
	assert.Equal(t, "/2/users/me", tr.calls[0].URL.Path)
	assert.Equal(t, "Bearer AT", tr.calls[0].Header.Get("Authorization"))
	assert.Equal(t, "1234", info.ID)
	assert.Equal(t, "foobar", info.Username)
	assert.Equal(t, "Foo Bar", info.DisplayName)
}

func TestTwitterContractErrorPayload(t *testing.T) {
	cfg, _ := contractConfig(t, jsonResponder(401, `{"errors":[{"code":215,"message":"Bad Authentication data"}]}`))
	p := NewTwitterProvider(cfg)

	_, err := p.GetUserInfo(context.Background(), &OAuthToken{AccessToken: "AT"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Bad Authentication data")
}

// --- QQ ---------------------------------------------------------------------

func TestQQContractFormTokenJSONPOpenID(t *testing.T) {
	cfg, tr := contractConfig(t, jsonResponder(200, "access_token=AT&expires_in=7200&refresh_token=RT"))
	p := NewQQProvider(cfg)

	token, err := p.ExchangeCode(context.Background(), "code-1")
	require.NoError(t, err)
	require.Len(t, tr.calls, 1)
	call := tr.calls[0]
	assert.Equal(t, http.MethodGet, call.Method)
	assert.Equal(t, "/oauth2.0/token", call.URL.Path)
	assert.Equal(t, "fake-client-id", call.URL.Query().Get("client_id"))
	assert.Equal(t, "code-1", call.URL.Query().Get("code"))
	assert.Equal(t, "AT", token.AccessToken)
	assert.Equal(t, int64(7200), token.ExpiresIn)

	tr.resp = func(call contractCall) (int, string) {
		if call.URL.Path == "/oauth2.0/me" {
			return 200, `callback( {"client_id":"123","openid":"OPEN-1"} );`
		}
		return 200, `{"ret":0,"msg":"","nickname":"Q友","figureurl_qq_1":"https://q1.png"}`
	}
	info, err := p.GetUserInfo(context.Background(), token)
	require.NoError(t, err)
	require.Len(t, tr.calls, 3)
	meCall := tr.calls[1]
	assert.Equal(t, "/oauth2.0/me", meCall.URL.Path)
	assert.Equal(t, "AT", meCall.URL.Query().Get("access_token"))
	userCall := tr.calls[2]
	assert.Equal(t, "/user/get_user_info", userCall.URL.Path)
	assert.Equal(t, "OPEN-1", userCall.URL.Query().Get("openid"))
	assert.Equal(t, "fake-client-id", userCall.URL.Query().Get("oauth_consumer_key"))
	assert.Equal(t, "OPEN-1", info.ID)
	assert.Equal(t, "Q友", info.DisplayName)
	assert.Equal(t, "https://q1.png", info.AvatarURL)
}

func TestQQContractRetError(t *testing.T) {
	cfg, _ := contractConfig(t, func(call contractCall) (int, string) {
		if call.URL.Path == "/oauth2.0/me" {
			return 200, `callback( {"client_id":"123","openid":"OPEN-1"} );`
		}
		return 200, `{"ret":50001,"msg":"invalid credentials"}`
	})
	p := NewQQProvider(cfg)

	_, err := p.GetUserInfo(context.Background(), &OAuthToken{AccessToken: "AT"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid credentials")
}

// --- Weibo ------------------------------------------------------------------

func TestWeiboContractCapturesUID(t *testing.T) {
	cfg, tr := contractConfig(t, jsonResponder(200, `{"access_token":"AT","expires_in":1576800,"remind_in":"7776000","uid":"10002"}`))
	p := NewWeiboProvider(cfg)

	token, err := p.ExchangeCode(context.Background(), "code-1")
	require.NoError(t, err)
	require.Len(t, tr.calls, 1)
	form, err := url.ParseQuery(string(tr.calls[0].Body))
	require.NoError(t, err)
	assert.Equal(t, "fake-client-id", form.Get("client_id"))
	assert.Equal(t, "code-1", form.Get("code"))
	assert.Equal(t, "10002", token.ID)

	tr.resp = jsonResponder(200, `{"id":10002,"idstr":"10002","screen_name":"微博用户","avatar_hd":"https://h.png"}`)
	info, err := p.GetUserInfo(context.Background(), token)
	require.NoError(t, err)
	require.Len(t, tr.calls, 2)
	userCall := tr.calls[1]
	assert.Equal(t, "/2/users/show.json", userCall.URL.Path)
	assert.Equal(t, "10002", userCall.URL.Query().Get("uid"))
	assert.Equal(t, "AT", userCall.URL.Query().Get("access_token"))
	assert.Equal(t, "10002", info.ID)
	assert.Equal(t, "微博用户", info.DisplayName)
	assert.Equal(t, "https://h.png", info.AvatarURL)
}

// --- Douyin -----------------------------------------------------------------

func TestDouyinContractClientKeyAndWrappedData(t *testing.T) {
	cfg, tr := contractConfig(t, jsonResponder(200, `{"data":{"access_token":"AT","expires_in":86400,"refresh_token":"RT","open_id":"od-1","scope":"user_info","error_code":0,"description":""},"message":"success"}`))
	p := NewDouyinProvider(cfg)

	redirect, err := p.GetRedirectURL("state-1")
	require.NoError(t, err)
	assert.Contains(t, redirect, "client_key=fake-client-id")
	assert.Contains(t, redirect, "scope=user_info")

	token, err := p.ExchangeCode(context.Background(), "code-1")
	require.NoError(t, err)
	require.Len(t, tr.calls, 1)
	form, err := url.ParseQuery(string(tr.calls[0].Body))
	require.NoError(t, err)
	assert.Equal(t, "fake-client-id", form.Get("client_key"))
	assert.Equal(t, "fake-client-secret", form.Get("client_secret"))
	assert.Equal(t, "authorization_code", form.Get("grant_type"))
	assert.Equal(t, "od-1", token.OpenID)
	assert.Equal(t, "AT", token.AccessToken)

	tr.resp = jsonResponder(200, `{"data":{"open_id":"od-1","union_id":"ud-1","nickname":"抖音用户","avatar":"https://a.png"},"message":"success"}`)
	info, err := p.GetUserInfo(context.Background(), token)
	require.NoError(t, err)
	require.Len(t, tr.calls, 2)
	userCall := tr.calls[1]
	assert.Equal(t, "AT", userCall.Header.Get("access-token"))
	var reqBody map[string]string
	require.NoError(t, json.Unmarshal(userCall.Body, &reqBody))
	assert.Equal(t, "od-1", reqBody["open_id"])
	assert.Equal(t, "AT", reqBody["access_token"])
	assert.Equal(t, "od-1", info.ID)
	assert.Equal(t, "ud-1", info.UnionID)
	assert.Equal(t, "抖音用户", info.DisplayName)
}

func TestDouyinContractErrorCode(t *testing.T) {
	cfg, _ := contractConfig(t, jsonResponder(200, `{"data":{"error_code":10009,"description":"code been used"},"message":"error"}`))
	p := NewDouyinProvider(cfg)

	_, err := p.ExchangeCode(context.Background(), "used")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "code been used")
}

// --- Kwai -------------------------------------------------------------------

func TestKwaiContractAppIDAndOpenID(t *testing.T) {
	cfg, tr := contractConfig(t, jsonResponder(200, `{"result":1,"access_token":"AT","expires_in":86400,"refresh_token":"RT","open_id":"ok-1"}`))
	p := NewKwaiProvider(cfg)

	token, err := p.ExchangeCode(context.Background(), "code-1")
	require.NoError(t, err)
	require.Len(t, tr.calls, 1)
	assert.Equal(t, "fake-client-id", tr.calls[0].URL.Query().Get("app_id"))
	assert.Equal(t, "authorization_code", tr.calls[0].URL.Query().Get("grant_type"))
	assert.Equal(t, "ok-1", token.OpenID)

	tr.resp = jsonResponder(200, `{"result":1,"user_info":{"head":"https://h.png","name":"快手用户"}}`)
	info, err := p.GetUserInfo(context.Background(), token)
	require.NoError(t, err)
	require.Len(t, tr.calls, 2)
	assert.Equal(t, "ok-1", info.ID)
	assert.Equal(t, "快手用户", info.DisplayName)
	assert.Equal(t, "https://h.png", info.AvatarURL)
}

func TestKwaiContractResultError(t *testing.T) {
	cfg, _ := contractConfig(t, jsonResponder(200, `{"result":0,"error_msg":"app_id is invalid"}`))
	p := NewKwaiProvider(cfg)

	_, err := p.ExchangeCode(context.Background(), "code-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "app_id is invalid")
}

// --- Bilibili ---------------------------------------------------------------

func TestBilibiliContractAccountOAuth2(t *testing.T) {
	cfg, tr := contractConfig(t, jsonResponder(200, `{"code":0,"message":"OK","data":{"access_token":"AT","expires_in":7200,"refresh_token":"RT"}}`))
	p := NewBilibiliProvider(cfg)

	token, err := p.ExchangeCode(context.Background(), "code-1")
	require.NoError(t, err)
	require.Len(t, tr.calls, 1)
	call := tr.calls[0]
	assert.Equal(t, http.MethodPost, call.Method)
	assert.Equal(t, "api.bilibili.com", call.URL.Host)
	assert.Equal(t, "/x/account-oauth2/v1/token", call.URL.Path)
	assert.Equal(t, "fake-client-id", call.URL.Query().Get("client_id"))
	assert.Equal(t, "authorization_code", call.URL.Query().Get("grant_type"))
	var body map[string]string
	require.NoError(t, json.Unmarshal(call.Body, &body))
	assert.Equal(t, "code-1", body["code"])
	assert.Equal(t, "AT", token.AccessToken)

	tr.resp = jsonResponder(200, `{"code":0,"message":"OK","data":{"openid":"mid-1","name":"B站用户","face":"https://f.png"}}`)
	info, err := p.GetUserInfo(context.Background(), token)
	require.NoError(t, err)
	require.Len(t, tr.calls, 2)
	userCall := tr.calls[1]
	assert.Equal(t, "/x/account-oauth2/v1/userinfo", userCall.URL.Path)
	assert.Equal(t, "AT", userCall.URL.Query().Get("access_token"))
	assert.Equal(t, "fake-client-id", userCall.URL.Query().Get("client_id"))
	assert.Equal(t, "mid-1", info.ID)
	assert.Equal(t, "B站用户", info.DisplayName)
	assert.Equal(t, "https://f.png", info.AvatarURL)
}

func TestBilibiliContractCodeError(t *testing.T) {
	cfg, _ := contractConfig(t, jsonResponder(200, `{"code":-101,"message":"用户未登录"}`))
	p := NewBilibiliProvider(cfg)

	_, err := p.ExchangeCode(context.Background(), "code-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "用户未登录")
}

// --- Baidu ------------------------------------------------------------------

func TestBaiduContractGetInfo(t *testing.T) {
	cfg, tr := contractConfig(t, jsonResponder(200, `{"openid":"ob-1","username":"百度用户","portrait":"1234567890abcdef"}`))
	p := NewBaiduProvider(cfg)

	info, err := p.GetUserInfo(context.Background(), &OAuthToken{AccessToken: "AT"})
	require.NoError(t, err)
	require.Len(t, tr.calls, 1)
	call := tr.calls[0]
	assert.Equal(t, "openapi.baidu.com", call.URL.Host)
	assert.Equal(t, "/rest/2.0/passport/users/getInfo", call.URL.Path)
	assert.Equal(t, "AT", call.URL.Query().Get("access_token"))
	assert.Equal(t, "ob-1", info.ID)
	assert.Equal(t, "百度用户", info.DisplayName)
	assert.Equal(t, "https://himg.bdimg.com/sys/portrait/item/1234567890abcdef", info.AvatarURL)
}

// --- Apple ------------------------------------------------------------------

func TestAppleContractFormPostAndIDToken(t *testing.T) {
	cfg, _ := contractConfig(t, jsonResponder(200, "{}"))
	p := NewAppleProvider(cfg)

	redirect, err := p.GetRedirectURL("state-1")
	require.NoError(t, err)
	assert.Contains(t, redirect, "appleid.apple.com/auth/authorize")
	assert.Contains(t, redirect, "response_mode=form_post")
	assert.Contains(t, redirect, "scope=name+email")

	userClaim := `{"name":{"firstName":"三","lastName":"张"}}`
	payload := map[string]any{
		"sub":   "001234.abcdef.12345",
		"email": "user@privaterelay.appleid.com",
		"user":  userClaim,
	}
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	idToken := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"ES256"}`)) + "." +
		base64.RawURLEncoding.EncodeToString(raw) + ".sig"

	info, err := p.GetUserInfo(context.Background(), &OAuthToken{IDToken: idToken})
	require.NoError(t, err)
	assert.Equal(t, "001234.abcdef.12345", info.ID)
	assert.Equal(t, "user@privaterelay.appleid.com", info.Email)
	assert.Equal(t, "三 张", info.DisplayName)
}

func TestAppleContractMissingIDToken(t *testing.T) {
	cfg, _ := contractConfig(t, jsonResponder(200, "{}"))
	p := NewAppleProvider(cfg)

	_, err := p.GetUserInfo(context.Background(), &OAuthToken{AccessToken: "AT"})
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "id_token")
}

// --- Factory wiring ---------------------------------------------------------

func TestFactoryWiresWeComAgentID(t *testing.T) {
	f := NewFactory()
	prov, err := f.GetProvider(&providers.Provider{
		Category:     "oauth",
		ProviderType: "wecom_internal",
		ClientID:     "wx-corpid",
		ClientSecret: "corp-secret",
		AgentID:      "1000002",
	})
	require.NoError(t, err)

	redirect, err := prov.GetRedirectURL("state-1")
	require.NoError(t, err)
	assert.Contains(t, redirect, "agentid=1000002")
	assert.NotContains(t, redirect, "corp-secret")
}
