package crypto

import "testing"

// GM R6: guliangiam.keys.private_key must stay registered so both the generic
// backfill and key rotation cover tenant signing material.
func TestDefaultEncryptedColumns_IncludesKeysPrivateKey(t *testing.T) {
	want := EncryptedColumn{Table: "guliangiam.keys", Column: "private_key", PKColumn: "id"}
	for _, col := range DefaultEncryptedColumns {
		if col == want {
			return
		}
	}
	t.Fatalf("%+v missing from DefaultEncryptedColumns", want)
}
