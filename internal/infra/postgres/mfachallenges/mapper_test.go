package mfachallenges

import "testing"

func TestToDomainHydratesFailedAttemptsFromMetadata(t *testing.T) {
	challenge := toDomain(challengeRow{
		ID:       "challenge-1",
		Metadata: []byte(`{"mfa_failed_attempts":3}`),
	})
	if challenge.FailedAttempts != 3 {
		t.Fatalf("FailedAttempts = %d, want 3", challenge.FailedAttempts)
	}
	if challenge.Metadata == nil {
		t.Fatalf("expected metadata to be decoded")
	}
}

func TestToDomainToleratesInvalidMetadata(t *testing.T) {
	challenge := toDomain(challengeRow{ID: "challenge-2", Metadata: []byte(`not-json`)})
	if challenge.FailedAttempts != 0 {
		t.Fatalf("FailedAttempts = %d, want 0", challenge.FailedAttempts)
	}
}
