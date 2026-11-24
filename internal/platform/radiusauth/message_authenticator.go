// Package radiusauth implements the RADIUS Message-Authenticator attribute
// (RFC 2869 section 5.14), the mitigation for Blast-RADIUS (CVE-2024-3596).
package radiusauth

import (
	"crypto/hmac"
	"crypto/md5"
	"errors"
	"fmt"

	"layeh.com/radius"
	"layeh.com/radius/rfc2869"
)

var (
	ErrMissing = errors.New("RADIUS Message-Authenticator is missing")
	ErrInvalid = errors.New("RADIUS Message-Authenticator is invalid")
)

// Sign sets the Message-Authenticator on a request (or on a response whose
// Authenticator field still holds the request authenticator).
func Sign(packet *radius.Packet) error {
	if packet == nil {
		return errors.New("RADIUS packet is nil")
	}
	if len(packet.Secret) == 0 {
		return errors.New("RADIUS shared secret is empty")
	}
	if err := rfc2869.MessageAuthenticator_Set(packet, make([]byte, md5.Size)); err != nil {
		return err
	}
	wire, err := packet.MarshalBinary()
	if err != nil {
		return err
	}
	return rfc2869.MessageAuthenticator_Set(packet, compute(wire, packet.Secret))
}

// VerifyResponse checks the response Message-Authenticator against the request
// authenticator and shared secret.
func VerifyResponse(response, request *radius.Packet) error {
	if response == nil || request == nil {
		return errors.New("RADIUS packet is nil")
	}
	if response.Identifier != request.Identifier {
		return errors.New("RADIUS response identifier does not match the request")
	}
	return verify(response, request.Authenticator)
}

func verify(packet *radius.Packet, requestAuthenticator [16]byte) error {
	wire, err := packet.MarshalBinary()
	if err != nil {
		return err
	}
	copy(wire[4:20], requestAuthenticator[:])

	var got []byte
	for i := 20; i < len(wire); {
		if i+2 > len(wire) {
			return fmt.Errorf("RADIUS packet has a truncated attribute")
		}
		attrType, attrLen := wire[i], int(wire[i+1])
		if attrLen < 2 || i+attrLen > len(wire) {
			return fmt.Errorf("RADIUS packet has an attribute with invalid length %d", attrLen)
		}
		if attrType == byte(rfc2869.MessageAuthenticator_Type) {
			if got != nil || attrLen != 2+md5.Size {
				return ErrInvalid
			}
			value := wire[i+2 : i+attrLen]
			got = append([]byte(nil), value...)
			clear(value)
		}
		i += attrLen
	}
	if got == nil {
		return ErrMissing
	}
	if !hmac.Equal(got, compute(wire, packet.Secret)) {
		return ErrInvalid
	}
	return nil
}

func compute(wire, secret []byte) []byte {
	mac := hmac.New(md5.New, secret)
	mac.Write(wire)
	return mac.Sum(nil)
}
