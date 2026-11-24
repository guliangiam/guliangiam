package syncer

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSha256Hex(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		result := sha256Hex([]byte(""))
		// SHA256 of empty string
		assert.Equal(t, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", result)
	})
	t.Run("hello", func(t *testing.T) {
		result := sha256Hex([]byte("hello"))
		assert.Equal(t, "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824", result)
	})
}

func TestHmacSHA256(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		key := []byte("secret")
		data := []byte("message")
		result := hmacSHA256(key, data)
		// Just verify it produces a non-empty result
		assert.Len(t, result, 32) // SHA256 produces 32 bytes
	})
	t.Run("empty_key", func(t *testing.T) {
		result := hmacSHA256([]byte{}, []byte("data"))
		assert.Len(t, result, 32)
	})
	t.Run("empty_data", func(t *testing.T) {
		result := hmacSHA256([]byte("key"), []byte{})
		assert.Len(t, result, 32)
	})
}

func TestHmacSHA256Hex(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		key := []byte("secret")
		data := []byte("message")
		result := hmacSHA256Hex(key, data)
		// Verify it's a valid hex string of correct length
		decoded, err := hex.DecodeString(result)
		assert.NoError(t, err)
		assert.Len(t, decoded, 32)
	})
	t.Run("consistent", func(t *testing.T) {
		key := []byte("key123")
		data := []byte("data456")
		result1 := hmacSHA256Hex(key, data)
		result2 := hmacSHA256Hex(key, data)
		assert.Equal(t, result1, result2)
	})
}

func TestGetSignatureKey(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		result := getSignatureKey("wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY", "20151229", "us-east-1")
		assert.Len(t, result, 32)
	})
	t.Run("different_inputs", func(t *testing.T) {
		result1 := getSignatureKey("secret1", "20240101", "us-east-1")
		result2 := getSignatureKey("secret2", "20240101", "us-east-1")
		assert.NotEqual(t, result1, result2)
	})
	t.Run("same_inputs", func(t *testing.T) {
		result1 := getSignatureKey("secret", "20240101", "us-east-1")
		result2 := getSignatureKey("secret", "20240101", "us-east-1")
		assert.Equal(t, result1, result2)
	})
}

func TestAWSIAMSyncer_MapUser_CustomMapping(t *testing.T) {
	s := &AWSIAMSyncer{}
	item := awsIAMUserItem{
		UserName:   "jdoe",
		UserID:     "AIDA123",
		ARN:        "arn:aws:iam::123:user/jdoe",
		CreateDate: "2024-01-01",
		Path:       "/",
	}
	attributeMap := map[string]string{
		"UserName":   "custom_username",
		"UserId":     "custom_id",
		"Arn":        "custom_arn",
		"Path":       "custom_path",
		"CreateDate": "custom_created",
	}
	result := s.mapUser(item, attributeMap)
	assert.Equal(t, "jdoe", result["custom_username"])
	assert.Equal(t, "AIDA123", result["custom_id"])
	assert.Equal(t, "arn:aws:iam::123:user/jdoe", result["custom_arn"])
	assert.Equal(t, "/", result["custom_path"])
	assert.Equal(t, "2024-01-01", result["custom_created"])
	// Should not have default keys
	_, hasID := result["id"]
	assert.False(t, hasID)
}

func TestAWSIAMSyncer_MapUser_PartialMapping(t *testing.T) {
	s := &AWSIAMSyncer{}
	item := awsIAMUserItem{
		UserName: "jdoe",
		UserID:   "AIDA123",
	}
	attributeMap := map[string]string{
		"UserName": "login",
	}
	result := s.mapUser(item, attributeMap)
	assert.Equal(t, "jdoe", result["login"])
	// Only mapped fields should be present
	_, hasArn := result["custom_arn"]
	assert.False(t, hasArn)
}

func TestAWSIAMSyncer_MapGroup_CustomMapping(t *testing.T) {
	s := &AWSIAMSyncer{}
	item := awsIAMGroupItem{
		GroupName:  "Admins",
		GroupID:    "AGPA123",
		ARN:        "arn:aws:iam::123:group/Admins",
		CreateDate: "2024-01-01",
		Path:       "/",
	}
	attributeMap := map[string]string{
		"GroupName":  "custom_name",
		"GroupId":    "custom_id",
		"Arn":        "custom_arn",
		"Path":       "custom_path",
		"CreateDate": "custom_created",
	}
	result := s.mapGroup(item, attributeMap)
	assert.Equal(t, "Admins", result["custom_name"])
	assert.Equal(t, "AGPA123", result["custom_id"])
	assert.Equal(t, "arn:aws:iam::123:group/Admins", result["custom_arn"])
	assert.Equal(t, "/", result["custom_path"])
	assert.Equal(t, "2024-01-01", result["custom_created"])
}

func TestAWSIAMSyncer_MapGroup_PartialMapping(t *testing.T) {
	s := &AWSIAMSyncer{}
	item := awsIAMGroupItem{
		GroupName: "Admins",
		GroupID:   "AGPA123",
	}
	attributeMap := map[string]string{
		"GroupName": "group_name",
	}
	result := s.mapGroup(item, attributeMap)
	assert.Equal(t, "Admins", result["group_name"])
}
