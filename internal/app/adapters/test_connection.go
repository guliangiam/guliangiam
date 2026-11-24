package adapters

import (
	"context"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// ConnectionTester is the outbound port for database connectivity probes.
// DSN construction, driver registration and SSRF checks live in
// internal/infra/adaptertest; the composition root injects the implementation
// (app layer must not import database/sql or infra directly).
type ConnectionTester interface {
	// TestDatabaseConnection returns whether the target accepted a connection
	// and a human-readable failure reason when it did not.
	TestDatabaseConnection(ctx context.Context, databaseType, host string, port int, username, password, databaseName string) (connected bool, failure string)
}

// testDatabaseConnection delegates the probe to the injected port. Without an
// implementation the endpoint reports an explicit configuration error instead
// of silently dialing from the app layer.
func (s *Service) testDatabaseConnection(ctx context.Context, databaseType, host string, port int, username, password, databaseName string) (*TestConnectionResult, error) {
	if s.ConnectionTester == nil {
		return nil, apperrors.NewUnimplemented("database connection tester is not configured")
	}
	connected, failure := s.ConnectionTester.TestDatabaseConnection(ctx, databaseType, host, port, username, password, databaseName)
	return &TestConnectionResult{Connected: connected, Error: failure}, nil
}
