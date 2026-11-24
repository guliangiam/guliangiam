package radius

import (
	"context"

	domainradius "github.com/guliangiam/guliangiam/internal/domain/radius"
)

// Server 是 domain radius.Server 在 app 层的别名。
// transport/radius handler 依赖此类型，避免直接 import domain。
type Server = domainradius.Server

// ServerFactory 是 domain radius.ServerFactory 在 app 层的别名。
// transport 从 app 层拿到 factory，调用后得到 Server 接口实例。
type ServerFactory = domainradius.ServerFactory

// Service 封装 RADIUS ServerFactory，供 transport 依赖。
// transport 通过 app/radius 调用，不直接 import domain/infra。
type Service struct {
	Factory ServerFactory
}

// NewService 用 domain ServerFactory 构造 app/radius Service。
// wire 层传入适配后的 infra/radius factory。
func NewService(factory ServerFactory) *Service {
	return &Service{Factory: factory}
}

// NewServer 调用底层 factory 创建一个 Server 接口实例。
func (s *Service) NewServer(addr, secret string) Server {
	return s.Factory(addr, secret)
}

// StartServer 是便捷方法，创建并启动一个新的 RADIUS Server。
func (s *Service) StartServer(ctx context.Context, addr, secret string, authFunc func(ctx context.Context, username, password string) (bool, error)) (Server, error) {
	srv := s.Factory(addr, secret)
	srv.SetAuthHandler(authFunc)
	if err := srv.Start(ctx); err != nil {
		return nil, err
	}
	return srv, nil
}
