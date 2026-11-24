package captcha

import (
	"bytes"
	"context"
	"image"
	_ "image/png"
	"testing"

	"github.com/stretchr/testify/assert"
)

type mockStore struct {
	data map[string]string
	err  error
}

func newMockStore() *mockStore {
	return &mockStore{data: make(map[string]string)}
}

func (m *mockStore) Set(ctx context.Context, id, code string) error {
	if m.err != nil {
		return m.err
	}
	m.data[id] = code
	return nil
}

func (m *mockStore) Get(ctx context.Context, id string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.data[id], nil
}

func (m *mockStore) Delete(ctx context.Context, id string) error {
	delete(m.data, id)
	return nil
}

func TestDefaultProvider_Verify_ValidToken(t *testing.T) {
	store := newMockStore()
	store.data["captcha1"] = "ABC123"
	p := NewDefaultProvider(store)

	ok, err := p.Verify(context.Background(), "captcha1:ABC123", "")
	assert.NoError(t, err)
	assert.True(t, ok)
}

func TestDefaultProvider_Verify_CaseInsensitive(t *testing.T) {
	store := newMockStore()
	store.data["captcha1"] = "ABC123"
	p := NewDefaultProvider(store)

	ok, err := p.Verify(context.Background(), "captcha1:abc123", "")
	assert.NoError(t, err)
	assert.True(t, ok)
}

func TestDefaultProvider_Verify_WrongCode(t *testing.T) {
	store := newMockStore()
	store.data["captcha1"] = "ABC123"
	p := NewDefaultProvider(store)

	ok, err := p.Verify(context.Background(), "captcha1:WRONG", "")
	assert.NoError(t, err)
	assert.False(t, ok)
}

func TestDefaultProvider_Verify_InvalidFormat(t *testing.T) {
	store := newMockStore()
	p := NewDefaultProvider(store)

	_, err := p.Verify(context.Background(), "invalidtoken", "")
	assert.Error(t, err)
}

func TestDefaultProvider_Verify_StoreError(t *testing.T) {
	store := &mockStore{data: make(map[string]string), err: assert.AnError}
	p := NewDefaultProvider(store)

	ok, err := p.Verify(context.Background(), "captcha1:ABC123", "")
	assert.NoError(t, err)
	assert.False(t, ok)
}

func TestDefaultProvider_Generate(t *testing.T) {
	store := newMockStore()
	p := NewDefaultProvider(store)

	id, img, err := p.Generate(context.Background())
	assert.NoError(t, err)
	assert.NotEmpty(t, id)
	assert.NotEmpty(t, img)
	assert.Contains(t, store.data, id)
}

func TestDefaultProvider_Generate_StoreError(t *testing.T) {
	store := &mockStore{data: make(map[string]string), err: assert.AnError}
	p := NewDefaultProvider(store)

	_, _, err := p.Generate(context.Background())
	assert.Error(t, err)
}

func TestDefaultProvider_Generate_ReturnsPNG(t *testing.T) {
	store := newMockStore()
	p := NewDefaultProvider(store)

	id, img, err := p.Generate(context.Background())
	assert.NoError(t, err)
	assert.NotEmpty(t, id)
	assert.Contains(t, store.data, id)

	decoded, _, err := image.Decode(bytes.NewReader(img))
	assert.NoError(t, err)
	assert.Equal(t, defaultWidth, decoded.Bounds().Dx())
	assert.Equal(t, defaultHeight, decoded.Bounds().Dy())
}

func TestDefaultProvider_GenerateVerify_RoundTrip(t *testing.T) {
	store := newMockStore()
	p := NewDefaultProvider(store)

	id, _, err := p.Generate(context.Background())
	assert.NoError(t, err)
	answer := store.data[id]

	ok, err := p.Verify(context.Background(), id+":"+answer, "")
	assert.NoError(t, err)
	assert.True(t, ok)

	// one-time consumption: the answer must be gone after a successful verify
	ok, err = p.Verify(context.Background(), id+":"+answer, "")
	assert.NoError(t, err)
	assert.False(t, ok)
}

func TestDefaultProvider_ConfigVariants(t *testing.T) {
	for _, cfg := range []struct {
		kind   string
		length int
	}{
		{"digit", 6},
		{"string", 4},
		{"math", 0},
	} {
		t.Run(cfg.kind, func(t *testing.T) {
			store := newMockStore()
			p := NewDefaultProviderWithConfig(store, cfg.kind, cfg.length)
			id, img, err := p.Generate(context.Background())
			assert.NoError(t, err)
			assert.NotEmpty(t, id)
			assert.NotEmpty(t, img)
			assert.Contains(t, store.data, id)
		})
	}
}

func TestNewDefaultProvider(t *testing.T) {
	store := newMockStore()
	p := NewDefaultProvider(store)
	assert.NotNil(t, p)
}
