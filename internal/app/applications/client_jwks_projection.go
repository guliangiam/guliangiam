package applications

import (
	"encoding/json"

	domain "github.com/guliangiam/guliangiam/internal/domain/applications"
)

// projectClientJWKS 把 app_client_jwks 行投影成只含公钥成员的视图。
//
// 白名单是结构体本身：encoding/json 只填充 ClientJWKPublicView 声明的字段，
// 库里若存在 d/p/q/dp/dq/qi 等越界成员也永远不会被读出。无法解释的行
// （JSON 损坏、kty 非 RSA/EC、缺少必需公有参数）直接跳过，不向前端暴露
// 一把不可用的密钥。
func projectClientJWKS(keys []*domain.ClientJWK) ClientJWKSView {
	view := emptyClientJWKSView()
	seen := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		if key == nil || !key.Usable() {
			continue
		}
		var public ClientJWKPublicView
		if err := json.Unmarshal([]byte(key.JWK), &public); err != nil {
			continue
		}
		if public.Kid == "" || public.Kty == "" {
			continue
		}
		switch public.Kty {
		case "RSA":
			if public.N == "" || public.E == "" {
				continue
			}
		case "EC":
			if public.Crv == "" || public.X == "" || public.Y == "" {
				continue
			}
		default:
			continue
		}
		if _, duplicate := seen[public.Kid]; duplicate {
			continue
		}
		seen[public.Kid] = struct{}{}
		view.Kids = append(view.Kids, public.Kid)
		view.Keys = append(view.Keys, public)
	}
	return view
}
