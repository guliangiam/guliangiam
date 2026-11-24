package users

import (
	"context"

	"net"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/mfatotp"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"

	"layeh.com/radius"
	"layeh.com/radius/rfc2865"
)

const mfaTotpPeriodInSeconds = 30

type mfaSetupProps struct {
	MfaType     string
	Secret      string
	CountryCode string
	URL         string
}

type mfaUtil interface {
	Initiate(userID, issuer string) (*mfaSetupProps, error)
	SetupVerify(passcode string) error
}

func getMfaUtil(mfaType string, config *mfaSetupProps) mfaUtil {
	switch mfaType {
	case mfaMethodSMS:
		return newSmsMfaUtil(config)
	case mfaMethodEmail:
		return newEmailMfaUtil(config)
	case mfaMethodApp, mfaMethodTOTP:
		return newTotpMfaUtil(config)
	case mfaMethodRadius:
		return newRadiusMfaUtil(config)
	case mfaMethodPush:
		return newPushMfaUtil(config)
	}
	return nil
}

type totpMfaUtil struct {
	props      *mfaSetupProps
	period     uint
	secretSize uint
	digits     otp.Digits
}

func newTotpMfaUtil(config *mfaSetupProps) *totpMfaUtil {
	if config == nil {
		config = &mfaSetupProps{MfaType: mfaMethodApp}
	}
	return &totpMfaUtil{
		props:      config,
		period:     mfaTotpPeriodInSeconds,
		secretSize: 20,
		digits:     otp.DigitsSix,
	}
}

func (m *totpMfaUtil) Initiate(userID, issuer string) (*mfaSetupProps, error) {
	if issuer == "" {
		issuer = "GuliangIAM"
	}
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: userID,
		Period:      m.period,
		SecretSize:  m.secretSize,
		Digits:      m.digits,
	})
	if err != nil {
		return nil, err
	}
	return &mfaSetupProps{
		MfaType: m.props.MfaType,
		Secret:  key.Secret(),
		URL:     key.URL(),
	}, nil
}

func (m *totpMfaUtil) SetupVerify(passcode string) error {
	_, ok, err := mfatotp.MatchStep(m.props.Secret, passcode, time.Now().UTC())
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	return apperrors.NewInvalidArgument("totp passcode error")
}

type smsMfaUtil struct {
	props *mfaSetupProps
}

func newSmsMfaUtil(config *mfaSetupProps) *smsMfaUtil {
	if config == nil {
		config = &mfaSetupProps{MfaType: mfaMethodSMS}
	}
	return &smsMfaUtil{props: config}
}

func (m *smsMfaUtil) Initiate(userID, issuer string) (*mfaSetupProps, error) {
	return &mfaSetupProps{MfaType: m.props.MfaType}, nil
}

func (m *smsMfaUtil) SetupVerify(passcode string) error {
	return smsSetupVerify(m.props.Secret, m.props.CountryCode, passcode)
}

type emailMfaUtil struct {
	props *mfaSetupProps
}

func newEmailMfaUtil(config *mfaSetupProps) *emailMfaUtil {
	if config == nil {
		config = &mfaSetupProps{MfaType: mfaMethodEmail}
	}
	return &emailMfaUtil{props: config}
}

func (m *emailMfaUtil) Initiate(userID, issuer string) (*mfaSetupProps, error) {
	return &mfaSetupProps{MfaType: m.props.MfaType}, nil
}

func (m *emailMfaUtil) SetupVerify(passcode string) error {
	return emailSetupVerify(m.props.Secret, passcode)
}

type radiusMfaUtil struct {
	props *mfaSetupProps
}

func newRadiusMfaUtil(config *mfaSetupProps) *radiusMfaUtil {
	if config == nil {
		config = &mfaSetupProps{MfaType: mfaMethodRadius}
	}
	return &radiusMfaUtil{props: config}
}

func (m *radiusMfaUtil) Initiate(userID, issuer string) (*mfaSetupProps, error) {
	return &mfaSetupProps{MfaType: m.props.MfaType}, nil
}

// radiusAuthenticate verifies a passcode against a RADIUS server.
// This is a function variable that can be replaced in tests.
var radiusAuthenticate = func(host, sharedSecret, username, passcode string) error {
	packet, err := buildAccessRequest(sharedSecret, username, passcode)
	if err != nil {
		return err
	}

	addr := host
	if _, _, err := net.SplitHostPort(host); err != nil {
		// No port specified, use default RADIUS auth port
		addr = net.JoinHostPort(host, "1812")
	}

	response, err := radius.Exchange(context.Background(), packet, "udp://"+addr)
	if err != nil {
		return apperrors.NewInternal("radius server connection failed: " + err.Error())
	}

	if response.Code != radius.CodeAccessAccept {
		return apperrors.NewUnauthorized("radius authentication rejected")
	}
	return nil
}

// buildAccessRequest 组装 RADIUS Access-Request 报文字段。
// 用户名/口令属性写入失败必须返回错误：静默丢弃会让认证请求缺少属性，
// 服务端只能给出「拒绝」，掩盖真实的编码问题（例如口令超过 128 字节）。
func buildAccessRequest(sharedSecret, username, passcode string) (*radius.Packet, error) {
	packet := radius.New(radius.CodeAccessRequest, []byte(sharedSecret))
	if err := rfc2865.UserName_SetString(packet, username); err != nil {
		return nil, apperrors.WrapInternalf(err, "radius set username")
	}
	if err := rfc2865.UserPassword_SetString(packet, passcode); err != nil {
		return nil, apperrors.WrapInternalf(err, "radius set password")
	}
	return packet, nil
}

func (m *radiusMfaUtil) SetupVerify(passcode string) error {
	// The Secret field holds the RADIUS config reference or server address.
	// For setup verification, we authenticate against the RADIUS server.
	if m.props == nil || m.props.Secret == "" {
		return apperrors.NewInvalidArgument("radius server configuration is required")
	}
	// m.props.Secret = "host:port:sharedSecret" or just "host"
	// m.props.URL may contain the full config reference
	return radiusAuthenticate(m.props.Secret, m.props.URL, m.props.CountryCode, passcode)
}

type pushMfaUtil struct {
	props *mfaSetupProps
}

func newPushMfaUtil(config *mfaSetupProps) *pushMfaUtil {
	if config == nil {
		config = &mfaSetupProps{MfaType: mfaMethodPush}
	}
	return &pushMfaUtil{props: config}
}

func (m *pushMfaUtil) Initiate(userID, issuer string) (*mfaSetupProps, error) {
	return &mfaSetupProps{MfaType: m.props.MfaType}, nil
}

// pushSetupVerifyFn verifies a push MFA setup by checking if the challenge was approved.
// This is a function variable that can be replaced in tests.
var pushSetupVerifyFn = func(challengeID string) (bool, error) {
	// No server-side push challenge is wired into the admin enrollment path yet.
	// Accepting any passcode here would let a client assert possession it never
	// proved, so the default refuses instead of defaulting to success.
	return false, apperrors.NewUnprocessableEntity("push mfa setup verification is not implemented")
}

func (m *pushMfaUtil) SetupVerify(passcode string) error {
	// For push MFA setup, the user must confirm receipt of a test push notification.
	// The passcode here is the challenge ID from the push notification.
	if passcode == "" {
		return apperrors.NewInvalidArgument("push confirmation is required")
	}
	approved, err := pushSetupVerifyFn(passcode)
	if err != nil {
		// Preserve the explicit "not implemented"/validation error instead of
		// masking it as an internal failure.
		return err
	}
	if !approved {
		return apperrors.NewUnauthorized("push mfa setup was not confirmed")
	}
	return nil
}
