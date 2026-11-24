package partitioning

import (
	"context"
	"fmt"
	"regexp"

	domainpartitioning "github.com/guliangiam/guliangiam/internal/domain/partitioning"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
)

var tableNamePattern = regexp.MustCompile("^[a-z][a-z0-9_]*$")

// Prober reports physical table/index sizes via PostgreSQL catalog functions.
// table is the bare contract table name (for example "records"); it is
// schema-qualified here so callers cannot inject arbitrary regclass input.
type Prober struct {
	db shared.DBTX
}

func NewProber(db shared.DBTX) *Prober { return &Prober{db: db} }

func (p *Prober) Size(ctx context.Context, table string) (domainpartitioning.RelationSize, error) {
	if p == nil || p.db == nil {
		return domainpartitioning.RelationSize{}, fmt.Errorf("partitioning prober: database is not configured")
	}
	if !tableNamePattern.MatchString(table) {
		return domainpartitioning.RelationSize{}, fmt.Errorf("partitioning prober: invalid table name %q", table)
	}
	var size domainpartitioning.RelationSize
	err := p.db.QueryRow(ctx,
		"SELECT pg_total_relation_size($1::regclass), pg_indexes_size($1::regclass)",
		"guliangiam."+table,
	).Scan(&size.TotalBytes, &size.IndexBytes)
	if err != nil {
		return domainpartitioning.RelationSize{}, fmt.Errorf("partitioning prober: size of guliangiam.%s: %w", table, err)
	}
	return size, nil
}
