package crypto

// LegacySealedColumn maps one legacy plaintext column to the `_sealed` column
// that carries its v2 envelope. The pair is the input of
// `migrate backfill-pii-sealed-columns`: the command encrypts the source text
// with EncryptColumn(col.AAD(pk)) into the target and never overwrites an
// existing ciphertext.
//
// The source column stays authoritative for reads until the target is sealed;
// this backfill does not clear it, because clearing legacy PII is a destructive
// step that belongs to a separately reviewed migration.
type LegacySealedColumn struct {
	Table        string
	SourceColumn string // legacy plaintext column
	TargetColumn string // registered v2 column ending in _sealed
	PKColumn     string // primary key column of Table
}

// Validate returns nil iff all four identifiers are safe for SQL construction.
// The source and target share the table and PK, so the existing identifier
// rule is applied to each pair instead of duplicating it.
func (c LegacySealedColumn) Validate() error {
	if err := validateIdentifiers(c.Table, c.SourceColumn, c.PKColumn); err != nil {
		return err
	}
	return validateIdentifiers(c.Table, c.TargetColumn, c.PKColumn)
}

// AAD builds the v2 row binding of the sealed column. The target column name
// (never the legacy source) is part of the AAD, matching the write path that
// reads the two columns with DecryptColumn.
func (c LegacySealedColumn) AAD(pk string) AADContext {
	return AADContext{Table: c.Table, Column: c.TargetColumn, PKValue: pk}
}

// Target returns the DefaultEncryptedColumns view of the sealed column.
func (c LegacySealedColumn) Target() EncryptedColumn {
	return EncryptedColumn{Table: c.Table, Column: c.TargetColumn, PKColumn: c.PKColumn}
}

// LegacySealedColumns lists every legacy plaintext PII column that has a
// dedicated v2 target. The targets are also registered in
// DefaultEncryptedColumns (A-6.3B) so key rotation reaches them;
// scripts/check-encrypted-columns.py asserts that both registries agree and
// that every source/target still exists in contracts/schema.
var LegacySealedColumns = []LegacySealedColumn{
	{Table: "guliangiam.user_collections", SourceColumn: "managed_accounts", TargetColumn: "managed_accounts_sealed", PKColumn: pkUserID},
	{Table: "guliangiam.user_collections", SourceColumn: "mfa_items", TargetColumn: "mfa_items_sealed", PKColumn: pkUserID},
	{Table: "guliangiam.user_collections", SourceColumn: "mfa_accounts", TargetColumn: "mfa_accounts_sealed", PKColumn: pkUserID},
	{Table: "guliangiam.identity_verifications", SourceColumn: "id_number", TargetColumn: "id_number_sealed", PKColumn: "id"},
	{Table: "guliangiam.identity_verifications", SourceColumn: "raw_result", TargetColumn: "raw_result_sealed", PKColumn: "id"},
	{Table: "guliangiam.user_profiles", SourceColumn: "id_card", TargetColumn: "id_card_sealed", PKColumn: pkUserID},
	{Table: "guliangiam.user_profiles", SourceColumn: "passport_no", TargetColumn: "passport_no_sealed", PKColumn: pkUserID},
	{Table: "guliangiam.user_profiles", SourceColumn: "driver_license_no", TargetColumn: "driver_license_no_sealed", PKColumn: pkUserID},
}
