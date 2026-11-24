package syncer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewActiveDirectorySyncer(t *testing.T) {
	s := NewActiveDirectorySyncer()
	assert.NotNil(t, s)
	assert.NotNil(t, s.ldapSyncer)
}

func TestActiveDirectorySyncer_ApplyADDefaults(t *testing.T) {
	s := &ActiveDirectorySyncer{}

	t.Run("sets_default_port", func(t *testing.T) {
		config := &SyncerConfig{Host: "dc.example.com"}
		adConfig := s.applyADDefaults(config)
		assert.Equal(t, 389, adConfig.Port)
		// Original config should not be modified
		assert.Equal(t, 0, config.Port)
	})

	t.Run("preserves_existing_port", func(t *testing.T) {
		config := &SyncerConfig{Host: "dc.example.com", Port: 636}
		adConfig := s.applyADDefaults(config)
		assert.Equal(t, 636, adConfig.Port)
	})

	t.Run("sets_default_filter", func(t *testing.T) {
		config := &SyncerConfig{Host: "dc.example.com"}
		adConfig := s.applyADDefaults(config)
		assert.Equal(t, "(objectClass=user)", adConfig.Filter)
	})

	t.Run("preserves_existing_filter", func(t *testing.T) {
		config := &SyncerConfig{Host: "dc.example.com", Filter: "(objectClass=person)"}
		adConfig := s.applyADDefaults(config)
		assert.Equal(t, "(objectClass=person)", adConfig.Filter)
	})

	t.Run("does_not_mutate_original", func(t *testing.T) {
		config := &SyncerConfig{Host: "dc.example.com", Port: 0, Filter: ""}
		adConfig := s.applyADDefaults(config)
		assert.Equal(t, 389, adConfig.Port)
		assert.Equal(t, "(objectClass=user)", adConfig.Filter)
		assert.Equal(t, 0, config.Port)
		assert.Equal(t, "", config.Filter)
	})
}

func TestActiveDirectorySyncer_GetLDAPSyncer(t *testing.T) {
	s := NewActiveDirectorySyncer()
	ldapSyncer := s.getLDAPSyncer()
	assert.NotNil(t, ldapSyncer)
}
