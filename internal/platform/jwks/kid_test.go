package jwks

import "testing"

func TestKidForAlgorithmNamespacesByFamily(t *testing.T) {
	cases := []struct {
		name      string
		algorithm string
		id        string
		want      string
	}{
		{"rsa plain id", SigningAlgorithmRS256, "key-1", KidPrefixRSA + "key-1"},
		{"gm plain id", SigningAlgorithmSM2SM3, "key-1", KidPrefixGM + "key-1"},
		{"rsa id already prefixed", SigningAlgorithmRS256, "rsa_key-1", "rsa_key-1"},
		{"gm id already prefixed", SigningAlgorithmSM2SM3, "gm_key-1", "gm_key-1"},
		{"rsa id cannot borrow gm prefix", SigningAlgorithmRS256, "gm_key-1", "rsa_gm_key-1"},
		{"gm id cannot borrow rsa prefix", SigningAlgorithmSM2SM3, "rsa_key-1", "gm_rsa_key-1"},
		{"rsa id is trimmed", SigningAlgorithmRS256, "  key-1  ", KidPrefixRSA + "key-1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := KidForAlgorithm(tc.algorithm, tc.id)
			if err != nil {
				t.Fatalf("KidForAlgorithm(%q, %q) error = %v", tc.algorithm, tc.id, err)
			}
			if got != tc.want {
				t.Fatalf("KidForAlgorithm(%q, %q) = %q, want %q", tc.algorithm, tc.id, got, tc.want)
			}
		})
	}
}

// One id under two algorithm families must never produce one kid: that is the
// collision the family prefix exists to prevent.
func TestKidForAlgorithmSeparatesFamiliesForSameID(t *testing.T) {
	rsaKid, err := KidForAlgorithm(SigningAlgorithmRS256, "shared-id")
	if err != nil {
		t.Fatalf("RSA kid error = %v", err)
	}
	gmKid, err := KidForAlgorithm(SigningAlgorithmSM2SM3, "shared-id")
	if err != nil {
		t.Fatalf("SM2 kid error = %v", err)
	}
	if rsaKid == gmKid {
		t.Fatalf("same id produced one kid across families: %q", rsaKid)
	}
	if rsaKid != KidPrefixRSA+"shared-id" || gmKid != KidPrefixGM+"shared-id" {
		t.Fatalf("kids = %q / %q, want family-prefixed values", rsaKid, gmKid)
	}
}

func TestKidForAlgorithmRejectsInvalidInput(t *testing.T) {
	cases := []struct {
		name      string
		algorithm string
		id        string
	}{
		{"empty id", SigningAlgorithmRS256, ""},
		{"blank id", SigningAlgorithmSM2SM3, "   "},
		{"unsupported algorithm", "ES256", "key-1"},
		{"empty algorithm", "", "key-1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got, err := KidForAlgorithm(tc.algorithm, tc.id); err == nil {
				t.Fatalf("KidForAlgorithm(%q, %q) = %q, want error", tc.algorithm, tc.id, got)
			}
		})
	}
}
