package users

import (
	"errors"
	"strings"
	"testing"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	"layeh.com/radius"
	"layeh.com/radius/rfc2865"
)

func TestGenerateClientID(t *testing.T) {
	id1 := generateClientID()
	id2 := generateClientID()
	if len(id1) != 10 {
		t.Errorf("len(generateClientID()) = %d, want 10", len(id1))
	}
	if id1 == id2 {
		t.Error("two generated client IDs should differ")
	}
}

func TestGenerateClientSecret(t *testing.T) {
	secret := generateClientSecret()
	if len(secret) != 20 {
		t.Errorf("len(generateClientSecret()) = %d, want 20", len(secret))
	}
}

func TestGenerateHex(t *testing.T) {
	tests := []struct {
		name    string
		charLen int
	}{
		{"10 chars", 10},
		{"20 chars", 20},
		{"1 char", 1},
		{"odd length", 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateHex(tt.charLen)
			if len(result) != tt.charLen {
				t.Errorf("len(generateHex(%d)) = %d, want %d", tt.charLen, len(result), tt.charLen)
			}
		})
	}
}

func TestGetMfaUtil(t *testing.T) {
	tests := []struct {
		name    string
		mfaType string
		isNil   bool
	}{
		{"sms", "sms", false},
		{"email", "email", false},
		{"app", "app", false},
		{"totp", "totp", false},
		{"radius", "radius", false},
		{"push", "push", false},
		{"unknown", "unknown", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			util := getMfaUtil(tt.mfaType, nil)
			if (util == nil) != tt.isNil {
				t.Errorf("getMfaUtil(%q) nil = %v, want %v", tt.mfaType, util == nil, tt.isNil)
			}
		})
	}
}

func TestNewTotpMfaUtil_NilConfig(t *testing.T) {
	util := newTotpMfaUtil(nil)
	if util == nil {
		t.Error("expected non-nil totpMfaUtil")
	}
	if util.props.MfaType != "app" {
		t.Errorf("MfaType = %q, want app", util.props.MfaType)
	}
}

func TestTotpMfaUtil_Initiate(t *testing.T) {
	util := newTotpMfaUtil(&mfaSetupProps{MfaType: "app"})
	props, err := util.Initiate("user-1", "TestIssuer")
	if err != nil {
		t.Fatalf("Initiate() error = %v", err)
	}
	if props.Secret == "" {
		t.Error("expected non-empty secret")
	}
	if props.URL == "" {
		t.Error("expected non-empty URL")
	}
}

func TestTotpMfaUtil_Initiate_DefaultIssuer(t *testing.T) {
	util := newTotpMfaUtil(&mfaSetupProps{MfaType: "app"})
	props, err := util.Initiate("user-1", "")
	if err != nil {
		t.Fatalf("Initiate() error = %v", err)
	}
	if props == nil {
		t.Error("expected non-nil props")
	}
}

func TestNewSmsMfaUtil_NilConfig(t *testing.T) {
	util := newSmsMfaUtil(nil)
	if util == nil {
		t.Error("expected non-nil smsMfaUtil")
	}
	if util.props.MfaType != "sms" {
		t.Errorf("MfaType = %q, want sms", util.props.MfaType)
	}
}

func TestSmsMfaUtil_Initiate(t *testing.T) {
	util := newSmsMfaUtil(&mfaSetupProps{MfaType: "sms"})
	props, err := util.Initiate("user-1", "")
	if err != nil {
		t.Fatalf("Initiate() error = %v", err)
	}
	if props.MfaType != "sms" {
		t.Errorf("MfaType = %q, want sms", props.MfaType)
	}
}

func TestNewEmailMfaUtil_NilConfig(t *testing.T) {
	util := newEmailMfaUtil(nil)
	if util == nil {
		t.Error("expected non-nil emailMfaUtil")
	}
}

func TestEmailMfaUtil_Initiate(t *testing.T) {
	util := newEmailMfaUtil(&mfaSetupProps{MfaType: "email"})
	props, err := util.Initiate("user-1", "")
	if err != nil {
		t.Fatalf("Initiate() error = %v", err)
	}
	if props.MfaType != "email" {
		t.Errorf("MfaType = %q, want email", props.MfaType)
	}
}

func TestNewRadiusMfaUtil_NilConfig(t *testing.T) {
	util := newRadiusMfaUtil(nil)
	if util == nil {
		t.Error("expected non-nil radiusMfaUtil")
	}
}

func TestRadiusMfaUtil_Initiate(t *testing.T) {
	util := newRadiusMfaUtil(&mfaSetupProps{MfaType: "radius"})
	props, err := util.Initiate("user-1", "")
	if err != nil {
		t.Fatalf("Initiate() error = %v", err)
	}
	if props.MfaType != "radius" {
		t.Errorf("MfaType = %q, want radius", props.MfaType)
	}
}

func TestNewPushMfaUtil_NilConfig(t *testing.T) {
	util := newPushMfaUtil(nil)
	if util == nil {
		t.Error("expected non-nil pushMfaUtil")
	}
}

func TestPushMfaUtil_Initiate(t *testing.T) {
	util := newPushMfaUtil(&mfaSetupProps{MfaType: "push"})
	props, err := util.Initiate("user-1", "")
	if err != nil {
		t.Fatalf("Initiate() error = %v", err)
	}
	if props.MfaType != "push" {
		t.Errorf("MfaType = %q, want push", props.MfaType)
	}
}

// Contract update: the default push verifier previously accepted any
// challengeId ("无法验证就默认通过"). It now refuses with 422 because no
// server-side push challenge exists in the admin enrollment path.
func TestPushMfaUtil_SetupVerify(t *testing.T) {
	util := newPushMfaUtil(&mfaSetupProps{MfaType: "push"})
	err := util.SetupVerify("challenge-123")
	var appErr *apperrors.Error
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeUnprocessableEntity {
		t.Errorf("SetupVerify() error = %v, want 422 unprocessable entity", err)
	}
}

func TestPushMfaUtil_SetupVerify_EmptyPasscode(t *testing.T) {
	util := newPushMfaUtil(&mfaSetupProps{MfaType: "push"})
	err := util.SetupVerify("")
	if err == nil {
		t.Error("expected error for empty passcode")
	}
}

func TestBuildAccessRequest_SetsAttributes(t *testing.T) {
	packet, err := buildAccessRequest("shared-secret", "alice", "passcode-123")
	if err != nil {
		t.Fatalf("buildAccessRequest() error = %v, want nil", err)
	}
	if got := rfc2865.UserName_GetString(packet); got != "alice" {
		t.Errorf("UserName = %q, want alice", got)
	}
	if packet.Code != radius.CodeAccessRequest {
		t.Errorf("Code = %v, want Access-Request", packet.Code)
	}
}

// 口令超过 RADIUS 属性上限时必须返回错误，而不是静默发出缺属性请求。
func TestBuildAccessRequest_RejectsOverlongPassword(t *testing.T) {
	long := make([]byte, 140)
	for i := range long {
		long[i] = 'a'
	}
	_, err := buildAccessRequest("shared-secret", "alice", string(long))
	if err == nil {
		t.Fatal("expected error for overlong password")
	}
	if !strings.Contains(err.Error(), "radius set password") {
		t.Errorf("error = %v, want it to mention radius set password", err)
	}
}
