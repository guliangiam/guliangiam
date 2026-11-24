package crypto

import "testing"

func TestLegacySealedColumnsMatchDefaultRegistry(t *testing.T) {
	want := map[string]string{
		"guliangiam.user_collections.managed_accounts": "guliangiam.user_collections.managed_accounts_sealed",
		"guliangiam.user_collections.mfa_items":        "guliangiam.user_collections.mfa_items_sealed",
		"guliangiam.user_collections.mfa_accounts":     "guliangiam.user_collections.mfa_accounts_sealed",
		"guliangiam.identity_verifications.id_number":  "guliangiam.identity_verifications.id_number_sealed",
		"guliangiam.identity_verifications.raw_result": "guliangiam.identity_verifications.raw_result_sealed",
		"guliangiam.user_profiles.id_card":             "guliangiam.user_profiles.id_card_sealed",
		"guliangiam.user_profiles.passport_no":         "guliangiam.user_profiles.passport_no_sealed",
		"guliangiam.user_profiles.driver_license_no":   "guliangiam.user_profiles.driver_license_no_sealed",
	}
	if len(LegacySealedColumns) != len(want) {
		t.Fatalf("LegacySealedColumns has %d entries, want %d", len(LegacySealedColumns), len(want))
	}

	defaults := map[string]EncryptedColumn{}
	for _, col := range DefaultEncryptedColumns {
		defaults[col.Table+"."+col.Column] = col
	}
	sealedPayloads := map[string]bool{}
	for _, col := range SealedPayloadColumns {
		sealedPayloads[col.Table+"."+col.Column] = true
	}

	seen := map[string]bool{}
	for _, col := range LegacySealedColumns {
		if err := col.Validate(); err != nil {
			t.Fatalf("%s.%s: %v", col.Table, col.SourceColumn, err)
		}
		sourceKey, targetKey := col.Table+"."+col.SourceColumn, col.Table+"."+col.TargetColumn
		if want[sourceKey] != targetKey {
			t.Errorf("%s maps to %s, want %s", sourceKey, targetKey, want[sourceKey])
		}
		if seen[targetKey] {
			t.Errorf("duplicate sealed target %s", targetKey)
		}
		seen[targetKey] = true
		if sealedPayloads[targetKey] {
			t.Errorf("%s is a v1 sealed payload; the PII backfill writes v2", targetKey)
		}
		registered, ok := defaults[targetKey]
		if !ok {
			t.Errorf("%s is missing from DefaultEncryptedColumns", targetKey)
			continue
		}
		if registered.PKColumn != col.PKColumn || col.Target() != registered {
			t.Errorf("%s registered as %+v, want Target() %+v", targetKey, registered, col.Target())
		}
	}
}
