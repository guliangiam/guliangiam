package crypto

import (
	"encoding/base64"
	"testing"
)

func TestFieldEncryptor_DecryptInvalidBase64(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	t.Setenv("ENCRYPTION_KEY", base64.StdEncoding.EncodeToString(key))
	t.Setenv("IS_PRODUCTION", "false")

	encryptor := NewFieldEncryptor()

	_, err := encryptor.DecryptField("!!!invalid-base64!!!")
	if err == nil {
		t.Error("expected error for invalid base64")
	}
}

func TestFieldEncryptor_DecryptCiphertextTooShort(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	t.Setenv("ENCRYPTION_KEY", base64.StdEncoding.EncodeToString(key))
	t.Setenv("IS_PRODUCTION", "false")

	encryptor := NewFieldEncryptor()

	shortCiphertext := base64.StdEncoding.EncodeToString([]byte("tooshort"))
	_, err := encryptor.DecryptField(shortCiphertext)
	if err == nil {
		t.Error("expected error for ciphertext too short")
	}
}

func TestFieldEncryptor_DecryptCorruptedCiphertext(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	t.Setenv("ENCRYPTION_KEY", base64.StdEncoding.EncodeToString(key))
	t.Setenv("IS_PRODUCTION", "false")

	encryptor := NewFieldEncryptor()

	encrypted, err := encryptor.EncryptField("test-value")
	if err != nil {
		t.Fatalf("EncryptField error: %v", err)
	}

	corrupted := encrypted[:len(encrypted)-5] + "XXXXX"
	_, err = encryptor.DecryptField(corrupted)
	if err == nil {
		t.Error("expected error for corrupted ciphertext")
	}
}

func TestFieldEncryptor_RoundTrip_Table(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	t.Setenv("ENCRYPTION_KEY", base64.StdEncoding.EncodeToString(key))
	t.Setenv("IS_PRODUCTION", "false")

	encryptor := NewFieldEncryptor()

	tests := []struct {
		name      string
		plaintext string
	}{
		{"simple", "hello"},
		{"unicode", "你好世界"},
		{"special chars", "p@$$w0rd!#%^&*()"},
		{"long", "this-is-a-very-long-plaintext-value-that-should-still-encrypt-and-decrypt-correctly"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encrypted, err := encryptor.EncryptField(tt.plaintext)
			if err != nil {
				t.Fatalf("EncryptField error: %v", err)
			}
			decrypted, err := encryptor.DecryptField(encrypted)
			if err != nil {
				t.Fatalf("DecryptField error: %v", err)
			}
			if decrypted != tt.plaintext {
				t.Errorf("round-trip: got %q, want %q", decrypted, tt.plaintext)
			}
		})
	}
}

func TestFieldEncryptor_MissingKeyRefusesStartup(t *testing.T) {
	t.Setenv("ENCRYPTION_KEY_CURRENT", "")
	t.Setenv("ENCRYPTION_KEY", "")

	defer func() {
		if recover() == nil {
			t.Error("expected panic when no encryption key is configured (DB-058: no plaintext fallback)")
		}
	}()

	NewFieldEncryptor()
}

// --- Key Rotation Tests ---

func TestFieldEncryptor_KeyRotation_DecryptWithPreviousKey(t *testing.T) {
	// Step 1: Encrypt with the "old" key. Clear the rotated-key variables so an
	// ambient ENCRYPTION_KEY_CURRENT cannot silently win over ENCRYPTION_KEY.
	t.Setenv("ENCRYPTION_KEY_CURRENT", "")
	t.Setenv("ENCRYPTION_KEY_PREVIOUS", "")
	oldKey := make([]byte, 32)
	for i := range oldKey {
		oldKey[i] = byte(i)
	}
	t.Setenv("ENCRYPTION_KEY", base64.StdEncoding.EncodeToString(oldKey))
	t.Setenv("IS_PRODUCTION", "false")

	oldEncryptor := NewFieldEncryptor()
	secret := "my-rotated-secret"
	encrypted, err := oldEncryptor.EncryptField(secret)
	if err != nil {
		t.Fatalf("EncryptField with old key: %v", err)
	}

	// Step 2: Set up new key with previous key fallback
	newKey := make([]byte, 32)
	for i := range newKey {
		newKey[i] = byte(i + 100)
	}
	t.Setenv("ENCRYPTION_KEY_CURRENT", base64.StdEncoding.EncodeToString(newKey))
	t.Setenv("ENCRYPTION_KEY_PREVIOUS", base64.StdEncoding.EncodeToString(oldKey))
	t.Setenv("ENCRYPTION_KEY", "") // clear legacy env

	rotatedEncryptor := NewFieldEncryptor()

	// Decrypt should fall back to previous key
	decrypted, err := rotatedEncryptor.DecryptField(encrypted)
	if err != nil {
		t.Fatalf("DecryptField with previous key fallback: %v", err)
	}
	if decrypted != secret {
		t.Errorf("decryption with previous key: got %q, want %q", decrypted, secret)
	}

	// New encryptions should use current key
	newEncrypted, err := rotatedEncryptor.EncryptField("new-value")
	if err != nil {
		t.Fatalf("EncryptField with current key: %v", err)
	}
	newDecrypted, err := rotatedEncryptor.DecryptField(newEncrypted)
	if err != nil {
		t.Fatalf("DecryptField new encryption: %v", err)
	}
	if newDecrypted != "new-value" {
		t.Errorf("new encryption round-trip: got %q, want %q", newDecrypted, "new-value")
	}
}

func TestFieldEncryptor_KeyRotation_CurrentKeyTakesPriority(t *testing.T) {
	// When both ENCRYPTION_KEY_CURRENT and ENCRYPTION_KEY are set,
	// ENCRYPTION_KEY_CURRENT should take priority
	currentKey := make([]byte, 32)
	for i := range currentKey {
		currentKey[i] = byte(i + 50)
	}
	legacyKey := make([]byte, 32)
	for i := range legacyKey {
		legacyKey[i] = byte(i + 200)
	}

	t.Setenv("ENCRYPTION_KEY_CURRENT", base64.StdEncoding.EncodeToString(currentKey))
	t.Setenv("ENCRYPTION_KEY", base64.StdEncoding.EncodeToString(legacyKey))
	t.Setenv("IS_PRODUCTION", "false")

	encryptor := NewFieldEncryptor()

	// Encrypt with the encryptor (should use current key)
	encrypted, err := encryptor.EncryptField("priority-test")
	if err != nil {
		t.Fatalf("EncryptField: %v", err)
	}

	// Now verify it can be decrypted with the same encryptor (current key)
	decrypted, err := encryptor.DecryptField(encrypted)
	if err != nil {
		t.Fatalf("DecryptField: %v", err)
	}
	if decrypted != "priority-test" {
		t.Errorf("got %q, want %q", decrypted, "priority-test")
	}

	// Verify the legacy key alone cannot decrypt this
	t.Setenv("ENCRYPTION_KEY", base64.StdEncoding.EncodeToString(legacyKey))
	t.Setenv("ENCRYPTION_KEY_CURRENT", "")
	t.Setenv("ENCRYPTION_KEY_PREVIOUS", "")

	legacyEncryptor := NewFieldEncryptor()
	_, err = legacyEncryptor.DecryptField(encrypted)
	if err == nil {
		t.Error("expected error when decrypting with legacy key (data was encrypted with current key)")
	}
}

func TestFieldEncryptor_RotateEncryptedData(t *testing.T) {
	// Step 1: Encrypt with old key. Clear the rotated-key variables so an
	// ambient ENCRYPTION_KEY_CURRENT cannot silently win over ENCRYPTION_KEY.
	t.Setenv("ENCRYPTION_KEY_CURRENT", "")
	t.Setenv("ENCRYPTION_KEY_PREVIOUS", "")
	oldKey := make([]byte, 32)
	for i := range oldKey {
		oldKey[i] = byte(i)
	}
	t.Setenv("ENCRYPTION_KEY", base64.StdEncoding.EncodeToString(oldKey))
	t.Setenv("IS_PRODUCTION", "false")

	oldEncryptor := NewFieldEncryptor()
	secret := "rotate-this-secret"
	encrypted, err := oldEncryptor.EncryptField(secret)
	if err != nil {
		t.Fatalf("EncryptField with old key: %v", err)
	}

	// Step 2: Set up rotation with new current key + previous key
	newKey := make([]byte, 32)
	for i := range newKey {
		newKey[i] = byte(i + 100)
	}
	t.Setenv("ENCRYPTION_KEY_CURRENT", base64.StdEncoding.EncodeToString(newKey))
	t.Setenv("ENCRYPTION_KEY_PREVIOUS", base64.StdEncoding.EncodeToString(oldKey))
	t.Setenv("ENCRYPTION_KEY", "")

	rotatedEncryptor := NewFieldEncryptor()

	// Rotate the encrypted data
	reEncrypted, wasRotated, err := rotatedEncryptor.RotateEncryptedData(encrypted)
	if err != nil {
		t.Fatalf("RotateEncryptedData: %v", err)
	}
	if !wasRotated {
		t.Error("expected wasRotated=true, data was encrypted with old key")
	}

	// Verify the re-encrypted data can be decrypted with the current key only
	currentOnlyKey := make([]byte, 32)
	copy(currentOnlyKey, newKey)
	t.Setenv("ENCRYPTION_KEY_CURRENT", base64.StdEncoding.EncodeToString(currentOnlyKey))
	t.Setenv("ENCRYPTION_KEY_PREVIOUS", "")
	t.Setenv("ENCRYPTION_KEY", "")

	currentEncryptor := NewFieldEncryptor()
	decrypted, err := currentEncryptor.DecryptField(reEncrypted)
	if err != nil {
		t.Fatalf("DecryptField after rotation: %v", err)
	}
	if decrypted != secret {
		t.Errorf("after rotation: got %q, want %q", decrypted, secret)
	}
}

func TestFieldEncryptor_RotateEncryptedData_AlreadyCurrentKey(t *testing.T) {
	currentKey := make([]byte, 32)
	for i := range currentKey {
		currentKey[i] = byte(i + 50)
	}
	prevKey := make([]byte, 32)
	for i := range prevKey {
		prevKey[i] = byte(i + 100)
	}
	t.Setenv("ENCRYPTION_KEY_CURRENT", base64.StdEncoding.EncodeToString(currentKey))
	t.Setenv("ENCRYPTION_KEY_PREVIOUS", base64.StdEncoding.EncodeToString(prevKey))
	t.Setenv("IS_PRODUCTION", "false")

	encryptor := NewFieldEncryptor()

	// Encrypt with current key
	encrypted, err := encryptor.EncryptField("already-current")
	if err != nil {
		t.Fatalf("EncryptField: %v", err)
	}

	// Rotation should detect it's already on current key
	reEncrypted, wasRotated, err := encryptor.RotateEncryptedData(encrypted)
	if err != nil {
		t.Fatalf("RotateEncryptedData: %v", err)
	}
	if wasRotated {
		t.Error("expected wasRotated=false, data was already encrypted with current key")
	}
	if reEncrypted != encrypted {
		t.Error("expected same ciphertext when already on current key")
	}
}

func TestFieldEncryptor_RotateEncryptedData_NoPreviousKey(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	t.Setenv("ENCRYPTION_KEY_CURRENT", base64.StdEncoding.EncodeToString(key))
	t.Setenv("ENCRYPTION_KEY_PREVIOUS", "")
	t.Setenv("IS_PRODUCTION", "false")

	encryptor := NewFieldEncryptor()

	encrypted, err := encryptor.EncryptField("no-previous")
	if err != nil {
		t.Fatalf("EncryptField: %v", err)
	}

	// Without previous key, rotation is a no-op
	reEncrypted, wasRotated, err := encryptor.RotateEncryptedData(encrypted)
	if err != nil {
		t.Fatalf("RotateEncryptedData: %v", err)
	}
	if wasRotated {
		t.Error("expected wasRotated=false when no previous key")
	}
	if reEncrypted != encrypted {
		t.Error("expected same ciphertext when no previous key")
	}
}

func TestFieldEncryptor_RotateEncryptedData_Disabled(t *testing.T) {
	t.Setenv("ENCRYPTION_KEY_CURRENT", "")
	t.Setenv("ENCRYPTION_KEY", "")

	defer func() {
		if recover() == nil {
			t.Error("expected panic when no encryption key is configured (DB-058: no plaintext fallback)")
		}
	}()

	NewFieldEncryptor()
}

func TestFieldEncryptor_RotateEncryptedData_Empty(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	t.Setenv("ENCRYPTION_KEY_CURRENT", base64.StdEncoding.EncodeToString(key))
	t.Setenv("ENCRYPTION_KEY_PREVIOUS", base64.StdEncoding.EncodeToString(key))
	t.Setenv("IS_PRODUCTION", "false")

	encryptor := NewFieldEncryptor()

	reEncrypted, wasRotated, err := encryptor.RotateEncryptedData("")
	if err != nil {
		t.Fatalf("RotateEncryptedData empty: %v", err)
	}
	if wasRotated {
		t.Error("expected wasRotated=false for empty string")
	}
	if reEncrypted != "" {
		t.Errorf("expected empty string, got %q", reEncrypted)
	}
}

func TestFieldEncryptor_HasPreviousKey(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}

	t.Run("with previous key", func(t *testing.T) {
		t.Setenv("ENCRYPTION_KEY_CURRENT", base64.StdEncoding.EncodeToString(key))
		t.Setenv("ENCRYPTION_KEY_PREVIOUS", base64.StdEncoding.EncodeToString(key))
		t.Setenv("IS_PRODUCTION", "false")

		encryptor := NewFieldEncryptor()
		if !encryptor.HasPreviousKey() {
			t.Error("expected HasPreviousKey=true when ENCRYPTION_KEY_PREVIOUS is set")
		}
	})

	t.Run("without previous key", func(t *testing.T) {
		t.Setenv("ENCRYPTION_KEY_CURRENT", base64.StdEncoding.EncodeToString(key))
		t.Setenv("ENCRYPTION_KEY_PREVIOUS", "")
		t.Setenv("IS_PRODUCTION", "false")

		encryptor := NewFieldEncryptor()
		if encryptor.HasPreviousKey() {
			t.Error("expected HasPreviousKey=false when ENCRYPTION_KEY_PREVIOUS is not set")
		}
	})
}

func TestFieldEncryptor_KeyRotation_InvalidPreviousKey(t *testing.T) {
	currentKey := make([]byte, 32)
	for i := range currentKey {
		currentKey[i] = byte(i + 1)
	}
	t.Setenv("ENCRYPTION_KEY_CURRENT", base64.StdEncoding.EncodeToString(currentKey))
	t.Setenv("ENCRYPTION_KEY_PREVIOUS", "not-valid-base64!!!")
	t.Setenv("IS_PRODUCTION", "false")

	defer func() {
		r := recover()
		if r == nil {
			t.Error("expected panic for invalid ENCRYPTION_KEY_PREVIOUS base64")
		}
	}()

	NewFieldEncryptor()
}

func TestFieldEncryptor_KeyRotation_PreviousKeyWrongSize(t *testing.T) {
	currentKey := make([]byte, 32)
	for i := range currentKey {
		currentKey[i] = byte(i + 1)
	}
	shortKey := make([]byte, 16) // wrong size
	t.Setenv("ENCRYPTION_KEY_CURRENT", base64.StdEncoding.EncodeToString(currentKey))
	t.Setenv("ENCRYPTION_KEY_PREVIOUS", base64.StdEncoding.EncodeToString(shortKey))
	t.Setenv("IS_PRODUCTION", "false")

	defer func() {
		r := recover()
		if r == nil {
			t.Error("expected panic for ENCRYPTION_KEY_PREVIOUS with wrong size")
		}
	}()

	NewFieldEncryptor()
}
