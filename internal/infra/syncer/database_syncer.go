package syncer

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	_ "github.com/jackc/pgx/v5/stdlib" // needed for database/sql driver registration
)

// validIdentifier matches safe SQL identifiers (letters, digits, underscores, dots for schema.table).
var validIdentifier = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_.]*$`)

// isValidIdentifier checks that a SQL identifier contains only safe characters.
func isValidIdentifier(name string) bool {
	return validIdentifier.MatchString(name)
}

// DatabaseSyncer syncs users and groups from an external database.
// It supports bidirectional sync with field mapping and incremental sync based on updated_at.
type DatabaseSyncer struct{}

// SyncUsers synchronizes users from the external database.
// It reads rows using the configured table/filter/field mapping,
// and performs incremental sync when LastSyncAt is available.
func (s *DatabaseSyncer) SyncUsers(ctx context.Context, config *SyncerConfig) (*SyncResult, error) {
	db, err := s.connect(config)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	result := &SyncResult{}

	query, args, err := s.buildUserQuery(config)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "database syncer build user query")
	}

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "database syncer query users")
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "database syncer get columns")
	}

	for rows.Next() {
		result.TotalUsers++

		// Scan row values into a map
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range columns {
			valuePtrs[i] = &values[i]
		}
		if err := rows.Scan(valuePtrs...); err != nil {
			result.FailedUsers++
			result.Errors = append(result.Errors, fmt.Sprintf("scan user row: %v", err))
			continue
		}

		// Apply field mapping
		mapped := s.applyFieldMapping(columns, values, config.AttributeMap)

		// Extract external ID for write-back
		externalID := ""
		if idVal, ok := mapped["id"]; ok {
			externalID = fmt.Sprintf("%v", idVal)
		}

		if config.OnUserSynced != nil {
			created, err := config.OnUserSynced(ctx, "database", externalID, mapped)
			if err != nil {
				result.FailedUsers++
				result.Errors = append(result.Errors, fmt.Sprintf("database user write-back failed: id=%s, err=%v", externalID, err))
				continue
			}
			if created {
				result.AddedUsers++
			} else {
				result.UpdatedUsers++
			}
		} else {
			// Read-only mode: count without persisting
			if _, hasID := mapped["id"]; hasID {
				result.UpdatedUsers++
			} else {
				result.AddedUsers++
			}
		}
	}

	if err := rows.Err(); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("iterate user rows: %v", err))
	}

	return result, nil
}

// SyncGroups synchronizes groups from the external database.
func (s *DatabaseSyncer) SyncGroups(ctx context.Context, config *SyncerConfig) (*SyncResult, error) {
	db, err := s.connect(config)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	result := &SyncResult{}

	query, args, err := s.buildGroupQuery(config)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "database syncer build group query")
	}

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "database syncer query groups")
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "database syncer get group columns")
	}

	for rows.Next() {
		result.TotalGroups++

		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range columns {
			valuePtrs[i] = &values[i]
		}
		if err := rows.Scan(valuePtrs...); err != nil {
			result.FailedGroups++
			result.Errors = append(result.Errors, fmt.Sprintf("scan group row: %v", err))
			continue
		}

		mapped := s.applyFieldMapping(columns, values, config.AttributeMap)

		// Extract external ID for write-back
		externalID := ""
		if idVal, ok := mapped["id"]; ok {
			externalID = fmt.Sprintf("%v", idVal)
		}

		if config.OnGroupSynced != nil {
			created, err := config.OnGroupSynced(ctx, config.OrganizationID, externalID, mapped)
			if err != nil {
				result.FailedGroups++
				result.Errors = append(result.Errors, fmt.Sprintf("database group write-back failed: id=%s, err=%v", externalID, err))
				continue
			}
			if created {
				result.AddedGroups++
			} else {
				result.UpdatedGroups++
			}
		} else {
			// Read-only mode: count without persisting
			if _, hasID := mapped["id"]; hasID {
				result.UpdatedGroups++
			} else {
				result.AddedGroups++
			}
		}
	}

	if err := rows.Err(); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("iterate group rows: %v", err))
	}

	return result, nil
}

// TestConnection tests the connection to the external database.
func (s *DatabaseSyncer) TestConnection(ctx context.Context, config *SyncerConfig) error {
	db, err := s.connect(config)
	if err != nil {
		return err
	}
	defer db.Close()
	return db.PingContext(ctx)
}

// connect opens a database connection using the provided config.
// config.Host = database host
// config.Port = database port
// config.Username = database user
// config.Password = database password
// config.BaseDN = database name (overloaded field)
func (s *DatabaseSyncer) connect(config *SyncerConfig) (*sql.DB, error) {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		config.Host, config.Port, config.Username, config.Password, config.BaseDN)
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "database syncer connect")
	}
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(5 * time.Minute)
	return db, nil
}

// buildUserQuery constructs a parameterized SQL query for syncing users.
// config.Filter is used as the table name (e.g., "public.users").
// config.AttributeMap maps source columns to target fields.
// If LastSyncAt is set, adds an incremental WHERE clause with parameterized time value.
func (s *DatabaseSyncer) buildUserQuery(config *SyncerConfig) (queryOut string, argsOut []interface{}, err error) {
	table := "users"
	if config.Filter != "" {
		if !isValidIdentifier(config.Filter) {
			return "", nil, apperrors.NewInvalidArgumentf("invalid table name: %q", config.Filter)
		}
		table = config.Filter
	}

	var fields []string
	for src := range config.AttributeMap {
		if !isValidIdentifier(src) {
			return "", nil, apperrors.NewInvalidArgumentf("invalid column name: %q", src)
		}
		fields = append(fields, fmt.Sprintf("%q", src))
	}
	if len(fields) == 0 {
		fields = []string{"id", "username", "display_name", "email", "phone", "updated_at"}
	}

	query := fmt.Sprintf("SELECT %s FROM %s", strings.Join(fields, ", "), table)
	var args []interface{}

	// Incremental sync: only fetch rows updated after the last sync time.
	if config.LastSyncAt != nil {
		query += " WHERE updated_at > $1"
		args = append(args, config.LastSyncAt)
	}

	return query, args, nil
}

// buildGroupQuery constructs a parameterized SQL query for syncing groups.
func (s *DatabaseSyncer) buildGroupQuery(config *SyncerConfig) (queryOut string, argsOut []interface{}, err error) {
	table := "groups"
	if config.Filter != "" {
		if !isValidIdentifier(config.Filter) {
			return "", nil, apperrors.NewInvalidArgumentf("invalid table name: %q", config.Filter)
		}
		table = config.Filter
	}

	query := fmt.Sprintf("SELECT id, name, display_name, description, parent_id, updated_at FROM %s", table)
	var args []interface{}

	if config.LastSyncAt != nil {
		query += " WHERE updated_at > $1"
		args = append(args, config.LastSyncAt)
	}

	return query, args, nil
}

// applyFieldMapping maps source column values to target field names using AttributeMap.
// If AttributeMap is empty, returns all columns with their original names.
func (s *DatabaseSyncer) applyFieldMapping(columns []string, values []interface{}, attributeMap map[string]string) map[string]interface{} {
	result := make(map[string]interface{}, len(columns))

	if len(attributeMap) == 0 {
		// No mapping configured: use column names as-is
		for i, col := range columns {
			result[col] = values[i]
		}
		return result
	}

	// Apply mapping: source column -> target field
	for i, col := range columns {
		if targetField, ok := attributeMap[col]; ok {
			result[targetField] = values[i]
		} else {
			// Keep unmapped columns with original names
			result[col] = values[i]
		}
	}

	return result
}

// Ensure DatabaseSyncer implements Engine.
var _ Engine = (*DatabaseSyncer)(nil)
