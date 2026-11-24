package syncers

import (
	"sync"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/syncers"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

type Service struct {
	Clock   clock.Clock
	UUID    platformuuid.Generator
	UOW     appuow.UnitOfWork
	Factory domain.SyncEngineFactory

	// locks 跨进程互斥（DB-061）：nil 时仅保留进程内 running 守卫
	//（单副本部署与单测路径）；多副本装配必须注入 advisory lock provider。
	locks LockProvider

	mu      *sync.Mutex
	running map[string]struct{}
}

// WithLockProvider injects the cross-process sync lock (DB-061). Returns the
// receiver so it can be chained at wiring time.
func (s Service) WithLockProvider(locks LockProvider) Service {
	if locks != nil {
		s.locks = locks
	}
	return s
}

func NewService(clk clock.Clock, uuidGenerator platformuuid.Generator, unitOfWork appuow.UnitOfWork) Service {
	return Service{
		Clock:   clk,
		UUID:    uuidGenerator,
		UOW:     unitOfWork,
		Factory: nil,
		mu:      &sync.Mutex{},
		running: make(map[string]struct{}),
	}
}

func NewServiceWithFactory(clk clock.Clock, uuidGenerator platformuuid.Generator, unitOfWork appuow.UnitOfWork, factory domain.SyncEngineFactory) Service {
	return Service{
		Clock:   clk,
		UUID:    uuidGenerator,
		UOW:     unitOfWork,
		Factory: factory,
		mu:      &sync.Mutex{},
		running: make(map[string]struct{}),
	}
}
