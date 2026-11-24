package storage

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewLocalStorage(t *testing.T) {
	p := NewLocalStorage("/tmp/storage", "http://localhost:8080/storage")
	assert.NotNil(t, p)
}

func TestNewAzureBlobStorage(t *testing.T) {
	p := NewAzureBlobStorage("account", "key", "container")
	assert.NotNil(t, p)
}

func TestNewSynologyNASStorage(t *testing.T) {
	p := NewSynologyNASStorage("https://nas.example.com", "admin", "password")
	assert.NotNil(t, p)
}
