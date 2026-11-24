package faceid

import "context"

// LivenessResult represents the result of a liveness detection check.
type LivenessResult struct {
	IsLive    bool
	Score     float64
	SessionID string
}

// CompareResult represents the result of a face comparison.
type CompareResult struct {
	IsMatch    bool
	Confidence float64
}

// FaceVerifier is the interface for Face ID verification with liveness detection
// and face comparison capabilities.
type FaceVerifier interface {
	// VerifyLiveness performs a liveness detection check on the provided image data.
	// It returns whether the face is live, a confidence score, and a session ID.
	VerifyLiveness(ctx context.Context, imageData, deviceInfo string) (*LivenessResult, error)

	// CompareFace compares a face image against a stored face template.
	// It returns whether the faces match and a confidence score.
	CompareFace(ctx context.Context, imageData, storedTemplate string) (*CompareResult, error)
}

// FakeVerifier implements FaceVerifier for testing and development purposes.
// It always returns successful results with high confidence scores.
type FakeVerifier struct{}

// NewFakeVerifier creates a new FakeVerifier instance.
func NewFakeVerifier() *FakeVerifier {
	return &FakeVerifier{}
}

// VerifyLiveness always returns IsLive=true with Score=0.99.
func (v *FakeVerifier) VerifyLiveness(_ context.Context, _, _ string) (*LivenessResult, error) {
	return &LivenessResult{
		IsLive:    true,
		Score:     0.99,
		SessionID: "fake-session-id",
	}, nil
}

// CompareFace always returns IsMatch=true with Confidence=0.95.
func (v *FakeVerifier) CompareFace(_ context.Context, _, _ string) (*CompareResult, error) {
	return &CompareResult{
		IsMatch:    true,
		Confidence: 0.95,
	}, nil
}
