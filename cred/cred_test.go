package cred

import (
	"testing"
)

func TestGetCredManager(t *testing.T) {
	tests := []struct {
		name         string
		passwordType string
		wantNil      bool
	}{
		{"plain", "plain", false},
		{"salt", "salt", false},
		{"sha512-salt", "sha512-salt", false},
		{"md5-salt", "md5-salt", false},
		{"bcrypt", "bcrypt", false},
		{"pbkdf2-salt", "pbkdf2-salt", false},
		{"argon2id", "argon2id", false},
		{"pbkdf2-django", "pbkdf2-django", false},
		{"unknown", "unknown", true},
		{"empty", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := GetCredManager(tt.passwordType)
			if (mgr == nil) != tt.wantNil {
				t.Errorf("GetCredManager(%q) nil = %v, want %v", tt.passwordType, mgr == nil, tt.wantNil)
			}
		})
	}
}

func TestPlainCredManager(t *testing.T) {
	cm := NewPlainCredManager()
	tests := []struct {
		name     string
		password string
		hash     string
		salt     string
		want     bool
	}{
		{"correct", "password123", "password123", "", true},
		{"incorrect", "password123", "wrong", "", false},
		{"with salt ignored", "pass", "pass", "salt", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cm.IsPasswordCorrect(tt.password, tt.hash, tt.salt)
			if got != tt.want {
				t.Errorf("IsPasswordCorrect() = %v, want %v", got, tt.want)
			}
		})
	}
	hashed := cm.GetHashedPassword("mypassword", "")
	if hashed != "mypassword" {
		t.Errorf("GetHashedPassword() = %q, want %q", hashed, "mypassword")
	}
}

func TestBcryptCredManager(t *testing.T) {
	cm := NewBcryptCredManager()
	password := "test-password-123"
	hashed := cm.GetHashedPassword(password, "")
	if hashed == "" {
		t.Fatal("GetHashedPassword returned empty")
	}
	if !cm.IsPasswordCorrect(password, hashed, "") {
		t.Error("IsPasswordCorrect should return true for correct password")
	}
	if cm.IsPasswordCorrect("wrong-password", hashed, "") {
		t.Error("IsPasswordCorrect should return false for wrong password")
	}
}

func TestArgon2idCredManager(t *testing.T) {
	cm := NewArgon2idCredManager()
	password := "test-argon2-password"
	hashed := cm.GetHashedPassword(password, "")
	if hashed == "" {
		t.Fatal("GetHashedPassword returned empty")
	}
	if !cm.IsPasswordCorrect(password, hashed, "") {
		t.Error("IsPasswordCorrect should return true for correct password")
	}
	if cm.IsPasswordCorrect("wrong", hashed, "") {
		t.Error("IsPasswordCorrect should return false for wrong password")
	}
}

func TestHashArgon2id(t *testing.T) {
	hash, err := HashArgon2id("password123")
	if err != nil {
		t.Fatalf("HashArgon2id error: %v", err)
	}
	if hash == "" {
		t.Error("expected non-empty hash")
	}
	ok, err := VerifyArgon2id("password123", hash)
	if err != nil {
		t.Fatalf("VerifyArgon2id error: %v", err)
	}
	if !ok {
		t.Error("VerifyArgon2id should return true")
	}
}

func TestMd5UserSaltCredManager_WithSalt(t *testing.T) {
	cm := NewMd5UserSaltCredManager()
	password := "test-md5-password"
	salt := "testsalt"
	hashed := cm.GetHashedPassword(password, salt)
	if hashed == "" {
		t.Fatal("GetHashedPassword returned empty")
	}
	if !cm.IsPasswordCorrect(password, hashed, salt) {
		t.Error("IsPasswordCorrect should return true")
	}
	if cm.IsPasswordCorrect("wrong", hashed, salt) {
		t.Error("IsPasswordCorrect should return false for wrong password")
	}
}

func TestMd5UserSaltCredManager_NoSalt(t *testing.T) {
	cm := NewMd5UserSaltCredManager()
	password := "test-md5-nosalt"
	hashed := cm.GetHashedPassword(password, "")
	if hashed == "" {
		t.Fatal("GetHashedPassword returned empty")
	}
	if !cm.IsPasswordCorrect(password, hashed, "") {
		t.Error("IsPasswordCorrect should return true for no-salt mode")
	}
}

func TestSha256SaltCredManager_WithSalt(t *testing.T) {
	cm := NewSha256SaltCredManager()
	password := "test-sha256-password"
	salt := "testsalt"
	hashed := cm.GetHashedPassword(password, salt)
	if hashed == "" {
		t.Fatal("GetHashedPassword returned empty")
	}
	if !cm.IsPasswordCorrect(password, hashed, salt) {
		t.Error("IsPasswordCorrect should return true")
	}
}

func TestSha256SaltCredManager_NoSalt(t *testing.T) {
	cm := NewSha256SaltCredManager()
	password := "test-sha256-nosalt"
	hashed := cm.GetHashedPassword(password, "")
	if hashed == "" {
		t.Fatal("GetHashedPassword returned empty")
	}
	if !cm.IsPasswordCorrect(password, hashed, "") {
		t.Error("IsPasswordCorrect should return true for no-salt mode")
	}
}

func TestSha512SaltCredManager_WithSalt(t *testing.T) {
	cm := NewSha512SaltCredManager()
	password := "test-sha512-password"
	salt := "testsalt"
	hashed := cm.GetHashedPassword(password, salt)
	if hashed == "" {
		t.Fatal("GetHashedPassword returned empty")
	}
	if !cm.IsPasswordCorrect(password, hashed, salt) {
		t.Error("IsPasswordCorrect should return true")
	}
}

func TestSha512SaltCredManager_NoSalt(t *testing.T) {
	cm := NewSha512SaltCredManager()
	password := "test-sha512-nosalt"
	hashed := cm.GetHashedPassword(password, "")
	if hashed == "" {
		t.Fatal("GetHashedPassword returned empty")
	}
	if !cm.IsPasswordCorrect(password, hashed, "") {
		t.Error("IsPasswordCorrect should return true for no-salt mode")
	}
}

func TestPbkdf2SaltCredManager(t *testing.T) {
	cm := NewPbkdf2SaltCredManager()
	password := "test-pbkdf2-password"
	salt := "dGVzdHNhbHQ="
	hashed := cm.GetHashedPassword(password, salt)
	if hashed == "" {
		t.Fatal("GetHashedPassword returned empty")
	}
	if !cm.IsPasswordCorrect(password, hashed, salt) {
		t.Error("IsPasswordCorrect should return true")
	}
	if cm.IsPasswordCorrect("wrong", hashed, salt) {
		t.Error("IsPasswordCorrect should return false for wrong password")
	}
}

func TestPbkdf2DjangoCredManager(t *testing.T) {
	cm := NewPbkdf2DjangoCredManager()
	password := "test-django-password"
	salt := "testsalt"
	hashed := cm.GetHashedPassword(password, salt)
	if hashed == "" {
		t.Fatal("GetHashedPassword returned empty")
	}
	if !cm.IsPasswordCorrect(password, hashed, "") {
		t.Error("IsPasswordCorrect should return true")
	}
	if cm.IsPasswordCorrect("wrong-password", hashed, "") {
		t.Error("IsPasswordCorrect should return false for wrong password")
	}
}

func TestPbkdf2DjangoCredManager_InvalidHash(t *testing.T) {
	cm := NewPbkdf2DjangoCredManager()
	tests := []struct {
		name string
		hash string
	}{
		{"empty hash", ""},
		{"wrong format", "invalid"},
		{"wrong algorithm", "sha1$260000$salt$hash"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if cm.IsPasswordCorrect("password", tt.hash, "") {
				t.Error("should return false for invalid hash format")
			}
		})
	}
}
