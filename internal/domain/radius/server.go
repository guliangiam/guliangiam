package radius

import "context"

// Server is the domain port for the RADIUS packet server.
// infra/radius.Server satisfies this interface directly;
// transport handlers depend on this interface, never the concrete type.
type Server interface {
	SetAuthHandler(authFunc func(ctx context.Context, username, password string) (bool, error))
	Start(ctx context.Context) error
	Stop() error
}

// ServerFactory creates a new RADIUS Server bound to addr + shared secret.
// Implemented by infra/radius.NewServer (返回值 *infra.Server 直接实现 Server 接口).
type ServerFactory func(addr, secret string) Server
