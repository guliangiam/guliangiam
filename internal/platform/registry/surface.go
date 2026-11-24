package registry

import "strings"

// Surface 是 route 面的 API 脸（手册 4.1.3）。
// 三脸：management / authentication / protocol。
//
// 本文件是单一事实数据源 —— transport/http/route/surface.go 通过
// Type-Alias Bridge 复用这里的定义，middleware / registry 层直接依赖本包，
// 避免 platform 反向依赖 transport（Rule 7 violation）。

type Surface string

const (
	SurfaceManagement     Surface = "management"
	SurfaceAuthentication Surface = "authentication"
	SurfaceProtocol       Surface = "protocol"
)

var AllSurfaces = []Surface{SurfaceManagement, SurfaceAuthentication, SurfaceProtocol}

func (s Surface) Valid() bool {
	for _, c := range AllSurfaces {
		if s == c {
			return true
		}
	}
	return false
}

// protocolPathPrefixes 是 protocol 面的根级路径族（手册 4.1.3 families ①–④, ⑥）。
// 新增 ⑦ /api/v1/ai/faces/* — AI Gateway 数据平面（OpenAI-compatible）。
var protocolPathPrefixes = []string{
	"/oauth2/",          // ① DCR + userinfo
	"/.well-known/",     // ② OIDC/OAuth discovery
	"/cas/",             // ③ CAS protocol
	"/scim/v2/",         // ④ SCIM 2.0
	"/api/v1/saml/",     // ⑥ SAML
	"/api/v1/ai/faces/", // ⑦ AI Gateway 数据平面
}

// protocolAuthSegments 是 /api/v1/auth/* 下属于 protocol 面的 segment（手册 4.1.3 family ⑤）。
var protocolAuthSegments = []string{
	"authorize", "tokens/", "device/", "oauth2/", "webauthn/",
	"cas/", "kerberos/", "saml/", "client/authenticate",
	"dpop/nonce", "dpop/validate", "backchannel-logout", "consents/",
}

const authPrefix = "/api/v1/auth/"

// SurfaceFor 根据 path pattern 判定 surface —— 唯一事实数据源。
func SurfaceFor(pattern string) Surface {
	for _, prefix := range protocolPathPrefixes {
		if strings.HasPrefix(pattern, prefix) {
			return SurfaceProtocol
		}
	}
	if strings.HasPrefix(pattern, authPrefix) {
		rest := strings.TrimPrefix(pattern, authPrefix)
		for _, seg := range protocolAuthSegments {
			if strings.HasPrefix(rest, seg) {
				return SurfaceProtocol
			}
		}
		return SurfaceAuthentication
	}
	return SurfaceManagement
}
