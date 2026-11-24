package webauthn

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"
)

func TestParseAuthenticatorData_TooShort(t *testing.T) {
	_, err := parseAuthenticatorData([]byte{1, 2, 3})
	if err == nil {
		t.Error("expected error for too-short data")
	}
}

func TestParseAuthenticatorData_Minimal(t *testing.T) {
	data := make([]byte, 37)
	for i := 0; i < 32; i++ {
		data[i] = byte(i)
	}
	data[32] = 0x01
	result, err := parseAuthenticatorData(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.flags.userPresent {
		t.Error("expected userPresent=true for flag 0x01")
	}
	if result.flags.userVerified {
		t.Error("expected userVerified=false for flag 0x01")
	}
}

func TestParseAuthenticatorData_UserVerified(t *testing.T) {
	data := make([]byte, 37)
	data[32] = 0x05
	result, err := parseAuthenticatorData(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.flags.userPresent {
		t.Error("expected userPresent=true")
	}
	if !result.flags.userVerified {
		t.Error("expected userVerified=true for flag 0x05")
	}
}

func TestParseAuthenticatorData_SignCount(t *testing.T) {
	data := make([]byte, 37)
	data[32] = 0x01
	data[33] = 0x00
	data[34] = 0x00
	data[35] = 0x01
	data[36] = 0x00
	result, err := parseAuthenticatorData(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.signCount != 256 {
		t.Errorf("expected signCount=256, got %d", result.signCount)
	}
}

func TestEqualByteSlices(t *testing.T) {
	if !equalByteSlices([]byte{1, 2, 3}, []byte{1, 2, 3}) {
		t.Error("expected equal")
	}
	if equalByteSlices([]byte{1, 2}, []byte{1, 2, 3}) {
		t.Error("expected not equal (different length)")
	}
	if equalByteSlices([]byte{1, 2, 3}, []byte{1, 2, 4}) {
		t.Error("expected not equal (different content)")
	}
	if !equalByteSlices(nil, nil) {
		t.Error("expected equal for nil slices")
	}
}

func TestVerifyAssertion_InvalidBase64(t *testing.T) {
	_, err := VerifyAssertion("challenge", "rp", "!!!invalid!!!", "", "", nil, 0, nil, false)
	if err == nil {
		t.Error("expected error for invalid base64")
	}
}

func TestVerifyAssertion_InvalidClientDataJSON(t *testing.T) {
	_, err := VerifyAssertion(
		"challenge",
		"rp",
		base64.RawURLEncoding.EncodeToString([]byte("not json")),
		base64.RawURLEncoding.EncodeToString(make([]byte, 37)),
		base64.RawURLEncoding.EncodeToString([]byte{0}),
		nil,
		0,
		nil,
		false,
	)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

// TestVerifyAssertion_FailureSentinels 把每一类断言失败钉到稳定哨兵：
// app 层据此产出 details.reason，前端才能显示可操作文案而不是表单校验兜底。
func TestVerifyAssertion_FailureSentinels(t *testing.T) {
	key := mustGenerateP256(t)
	coseKey := buildCOSE(es256Fields(key))
	const rpID = "example.com"
	const challenge = "test-challenge"
	allowed := []string{"https://example.com"}
	expectedChallenge := base64.RawURLEncoding.EncodeToString([]byte(challenge))

	clientData := func(typ, ch, origin string) string {
		payload, err := json.Marshal(map[string]string{
			"type":      typ,
			"challenge": ch,
			"origin":    origin,
		})
		if err != nil {
			t.Fatalf("marshal clientData: %v", err)
		}
		return base64.RawURLEncoding.EncodeToString(payload)
	}
	authData := func(rpHash []byte, flags byte) string {
		data := make([]byte, 37)
		copy(data[:32], rpHash)
		data[32] = flags
		return base64.RawURLEncoding.EncodeToString(data)
	}
	goodHash := sha256.Sum256([]byte(rpID))
	badHash := make([]byte, 32)
	sig := base64.RawURLEncoding.EncodeToString([]byte{1, 2, 3})

	cases := []struct {
		name string
		want error
		call func() error
	}{
		{
			name: "malformed clientDataJSON base64",
			want: ErrMalformedAssertion,
			call: func() error {
				_, err := VerifyAssertion(challenge, rpID, "!!!", "", "", coseKey, 0, allowed, false)
				return err
			},
		},
		{
			name: "unexpected clientData type",
			want: ErrMalformedAssertion,
			call: func() error {
				_, err := VerifyAssertion(challenge, rpID,
					clientData("webauthn.create", expectedChallenge, allowed[0]),
					authData(goodHash[:], 0x01), sig, coseKey, 0, allowed, false)
				return err
			},
		},
		{
			name: "challenge mismatch",
			want: ErrChallengeMismatch,
			call: func() error {
				_, err := VerifyAssertion(challenge, rpID,
					clientData("webauthn.get", "other-challenge", allowed[0]),
					authData(goodHash[:], 0x01), sig, coseKey, 0, allowed, false)
				return err
			},
		},
		{
			name: "wire-format challenge reaches signature check",
			want: ErrSignatureInvalid,
			call: func() error {
				// 浏览器回传的就是 begin 签发的 base64url 字符串本身，不得再被二次编码拦下。
				_, err := VerifyAssertion(challenge, rpID,
					clientData("webauthn.get", challenge, allowed[0]),
					authData(goodHash[:], 0x01), sig, coseKey, 0, allowed, false)
				return err
			},
		},
		{
			name: "origin missing",
			want: ErrOriginNotAllowed,
			call: func() error {
				_, err := VerifyAssertion(challenge, rpID,
					clientData("webauthn.get", expectedChallenge, ""),
					authData(goodHash[:], 0x01), sig, coseKey, 0, allowed, false)
				return err
			},
		},
		{
			name: "origin not allowed",
			want: ErrOriginNotAllowed,
			call: func() error {
				_, err := VerifyAssertion(challenge, rpID,
					clientData("webauthn.get", expectedChallenge, "https://evil.example"),
					authData(goodHash[:], 0x01), sig, coseKey, 0, allowed, false)
				return err
			},
		},
		{
			name: "rpIdHash mismatch",
			want: ErrRPIDMismatch,
			call: func() error {
				_, err := VerifyAssertion(challenge, rpID,
					clientData("webauthn.get", expectedChallenge, allowed[0]),
					authData(badHash, 0x01), sig, coseKey, 0, allowed, false)
				return err
			},
		},
		{
			name: "user presence missing",
			want: ErrUserPresenceRequired,
			call: func() error {
				_, err := VerifyAssertion(challenge, rpID,
					clientData("webauthn.get", expectedChallenge, allowed[0]),
					authData(goodHash[:], 0x00), sig, coseKey, 0, allowed, false)
				return err
			},
		},
		{
			name: "user verification required",
			want: ErrUserVerificationRequired,
			call: func() error {
				_, err := VerifyAssertion(challenge, rpID,
					clientData("webauthn.get", expectedChallenge, allowed[0]),
					authData(goodHash[:], 0x01), sig, coseKey, 0, allowed, true)
				return err
			},
		},
		{
			name: "signature invalid",
			want: ErrSignatureInvalid,
			call: func() error {
				_, err := VerifyAssertion(challenge, rpID,
					clientData("webauthn.get", expectedChallenge, allowed[0]),
					authData(goodHash[:], 0x01), sig, coseKey, 0, allowed, false)
				return err
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call()
			if err == nil {
				t.Fatalf("expected %v, got nil", tc.want)
			}
			if !errors.Is(err, tc.want) {
				t.Fatalf("expected errors.Is(%v), got %v", tc.want, err)
			}
		})
	}
}
