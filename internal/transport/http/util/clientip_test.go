package util

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClientIP_XForwardedForUsesRightmostUntrustedHop(t *testing.T) {
	r := &http.Request{
		Header: http.Header{
			"X-Forwarded-For": []string{"203.0.113.1, 70.41.3.18, 150.172.238.178"},
		},
		RemoteAddr: "192.168.1.1:12345",
	}
	assert.Equal(t, "150.172.238.178", ClientIP(r))
}

func TestClientIP_XRealIP(t *testing.T) {
	r := &http.Request{
		Header: http.Header{
			"X-Real-Ip": []string{"203.0.113.2"},
		},
		RemoteAddr: "192.168.1.1:12345",
	}
	assert.Equal(t, "203.0.113.2", ClientIP(r))
}

func TestClientIP_RemoteAddr(t *testing.T) {
	r := &http.Request{
		RemoteAddr: "192.168.1.1:12345",
	}
	assert.Equal(t, "192.168.1.1", ClientIP(r))
}

func TestClientIP_RemoteAddrNoPort(t *testing.T) {
	r := &http.Request{
		RemoteAddr: "192.168.1.1",
	}
	assert.Equal(t, "192.168.1.1", ClientIP(r))
}

func TestClientIP_EmptyXFF(t *testing.T) {
	r := &http.Request{
		Header: http.Header{
			"X-Forwarded-For": []string{""},
		},
		RemoteAddr: "10.0.0.1:54321",
	}
	assert.Equal(t, "10.0.0.1", ClientIP(r))
}
