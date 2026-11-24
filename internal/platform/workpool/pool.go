package workpool

import (
	"context"
	"sync"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

var ErrPoolFull = apperrors.NewTooManyRequests("workpool: queue full")

type task struct {
	ctx context.Context
	fn  func()
}

type Pool struct {
	tasks   chan task
	wg      sync.WaitGroup
	quit    chan struct{}
	closed  bool
	closeMu sync.RWMutex
}

func NewPool(workers, queueSize int) *Pool {
	p := &Pool{
		tasks: make(chan task, queueSize),
		quit:  make(chan struct{}),
	}
	p.wg.Add(workers)
	for i := 0; i < workers; i++ {
		go p.run()
	}
	return p
}

func (p *Pool) run() {
	defer p.wg.Done()
	for {
		select {
		case t, ok := <-p.tasks:
			if !ok {
				return
			}
			if t.ctx.Err() == nil {
				runTask(t.fn)
			}
		case <-p.quit:
			return
		}
	}
}

func (p *Pool) Submit(ctx context.Context, fn func()) error {
	p.closeMu.RLock()
	defer p.closeMu.RUnlock()
	if p.closed {
		return ErrPoolFull
	}
	select {
	case p.tasks <- task{ctx: ctx, fn: fn}:
		return nil
	default:
		return ErrPoolFull
	}
}

func (p *Pool) Shutdown() {
	p.ShutdownWithTimeout(30 * time.Second)
}

func (p *Pool) ShutdownWithTimeout(timeout time.Duration) {
	p.closeMu.Lock()
	if p.closed {
		p.closeMu.Unlock()
		return
	}
	p.closed = true
	p.closeMu.Unlock()

	close(p.quit)

	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(timeout):
	}
}
