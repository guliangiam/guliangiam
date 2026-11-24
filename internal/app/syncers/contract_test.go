package syncers

import (
	"testing"

	domainsyncers "github.com/guliangiam/guliangiam/internal/domain/syncers"
	infrasyncer "github.com/guliangiam/guliangiam/internal/infra/syncer"
)

func TestFactoryAdapterImplementsSyncEngineFactory(t *testing.T) {
	var _ domainsyncers.SyncEngineFactory = (*infrasyncer.FactoryAdapter)(nil)
}
