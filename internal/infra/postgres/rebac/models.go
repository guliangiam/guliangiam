package rebac

import (
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/rebac"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
)

// dbNamespace is the pgx scan target for relation_namespaces rows.
// RevisionCounter 是 Zookie 写后读一致性的单调递增计数器。
type dbNamespace struct {
	ID              string
	OrganizationID  string
	Name            string
	DisplayName     string
	RelationDefs    []byte
	Version         int
	CreatedAt       time.Time
	UpdatedAt       time.Time
	RevisionCounter int64
}

type dbTuple struct {
	ID              string
	OrganizationID  string
	NamespaceID     string
	Subject         string
	SubjectRelation *string
	Relation        string
	Object          string
	Zookie          string
	CreatedAt       time.Time
}

func (db dbNamespace) toDomain() (*domain.Namespace, error) {
	defs := map[string]any{}
	if err := shared.UnmarshalJSON(db.RelationDefs, &defs); err != nil {
		return nil, err
	}
	return &domain.Namespace{
		ID:             db.ID,
		OrganizationID: db.OrganizationID,
		Name:           db.Name,
		DisplayName:    db.DisplayName,
		RelationDefs:   defs,
		Version:        db.Version,
		CreatedAt:      db.CreatedAt,
		UpdatedAt:      db.UpdatedAt,
		Revision:       db.RevisionCounter,
	}, nil
}

func (db dbTuple) toDomain() *domain.Tuple {
	return &domain.Tuple{
		ID:              db.ID,
		OrganizationID:  db.OrganizationID,
		NamespaceID:     db.NamespaceID,
		Subject:         db.Subject,
		SubjectRelation: db.SubjectRelation,
		Relation:        db.Relation,
		Object:          db.Object,
		Zookie:          db.Zookie,
		CreatedAt:       db.CreatedAt,
	}
}
