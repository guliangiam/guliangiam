package captcha

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustAnswer(t *testing.T, store *mockStore, id string) *goCaptchaAnswer {
	t.Helper()
	var ans goCaptchaAnswer
	require.NoError(t, json.Unmarshal([]byte(store.data[goCaptchaStorePrefix+id]), &ans))
	return &ans
}

func TestGoCaptchaProvider_Generate_UnsupportedShape(t *testing.T) {
	p, err := NewGoCaptchaProvider(newMockStore())
	require.NoError(t, err)

	_, err = p.Generate(context.Background(), "rotate")
	assert.Error(t, err)
}

func TestGoCaptchaProvider_Click_GenerateVerify_RoundTrip(t *testing.T) {
	store := newMockStore()
	p, err := NewGoCaptchaProvider(store)
	require.NoError(t, err)

	res, err := p.Generate(context.Background(), GoCaptchaShapeClick)
	require.NoError(t, err)
	assert.NotEmpty(t, res.ID)
	assert.Equal(t, GoCaptchaShapeClick, res.Shape)
	assert.NotEmpty(t, res.MasterImage)
	assert.NotEmpty(t, res.SubImage)

	ans := mustAnswer(t, store, res.ID)
	require.Equal(t, GoCaptchaShapeClick, ans.Shape)
	require.NotEmpty(t, ans.Dots)
	require.Positive(t, ans.Width)
	require.Positive(t, ans.Height)

	var points []string
	for i := 0; i < len(ans.Dots); i++ {
		dot := ans.Dots[i]
		require.NotNil(t, dot)
		fx := (float64(dot.X) + float64(dot.Width)/2) / float64(ans.Width) * 100
		fy := (float64(dot.Y) + float64(dot.Height)/2) / float64(ans.Height) * 100
		points = append(points, strconv.FormatFloat(fx, 'f', 4, 64)+","+strconv.FormatFloat(fy, 'f', 4, 64))
	}
	token := res.ID + ":" + strings.Join(points, ";")

	ok, err := p.Verify(context.Background(), token, "")
	assert.NoError(t, err)
	assert.True(t, ok)

	// one-time consumption
	ok, err = p.Verify(context.Background(), token, "")
	assert.NoError(t, err)
	assert.False(t, ok)
}

func TestGoCaptchaProvider_Click_DefaultShape(t *testing.T) {
	store := newMockStore()
	p, err := NewGoCaptchaProvider(store)
	require.NoError(t, err)

	res, err := p.Generate(context.Background(), "")
	require.NoError(t, err)
	assert.Equal(t, GoCaptchaShapeClick, res.Shape)
}

func TestGoCaptchaProvider_Click_WrongPoints(t *testing.T) {
	store := newMockStore()
	p, err := NewGoCaptchaProvider(store)
	require.NoError(t, err)

	res, err := p.Generate(context.Background(), GoCaptchaShapeClick)
	require.NoError(t, err)
	ans := mustAnswer(t, store, res.ID)

	// top-left corner is outside every dot box (dots are clamped with padding)
	points := make([]string, len(ans.Dots))
	for i := range points {
		points[i] = "0,0"
	}
	ok, err := p.Verify(context.Background(), res.ID+":"+strings.Join(points, ";"), "")
	assert.NoError(t, err)
	assert.False(t, ok)
}

func TestGoCaptchaProvider_Click_PointCountMismatch(t *testing.T) {
	store := newMockStore()
	p, err := NewGoCaptchaProvider(store)
	require.NoError(t, err)

	res, err := p.Generate(context.Background(), GoCaptchaShapeClick)
	require.NoError(t, err)

	ok, err := p.Verify(context.Background(), res.ID+":50,50", "")
	assert.NoError(t, err)
	assert.False(t, ok)
}

func TestGoCaptchaProvider_Slide_GenerateVerify_RoundTrip(t *testing.T) {
	store := newMockStore()
	p, err := NewGoCaptchaProvider(store)
	require.NoError(t, err)

	res, err := p.Generate(context.Background(), GoCaptchaShapeSlide)
	require.NoError(t, err)
	assert.NotEmpty(t, res.ID)
	assert.Equal(t, GoCaptchaShapeSlide, res.Shape)
	assert.NotEmpty(t, res.MasterImage)
	assert.NotEmpty(t, res.SubImage)

	ans := mustAnswer(t, store, res.ID)
	require.Equal(t, GoCaptchaShapeSlide, ans.Shape)
	require.NotNil(t, ans.Block)

	frac := float64(ans.Block.X) / float64(ans.Width) * 100
	token := res.ID + ":" + strconv.FormatFloat(frac, 'f', 4, 64)

	ok, err := p.Verify(context.Background(), token, "")
	assert.NoError(t, err)
	assert.True(t, ok)

	// one-time consumption
	ok, err = p.Verify(context.Background(), token, "")
	assert.NoError(t, err)
	assert.False(t, ok)
}

func TestGoCaptchaProvider_Slide_WrongPosition(t *testing.T) {
	store := newMockStore()
	p, err := NewGoCaptchaProvider(store)
	require.NoError(t, err)

	res, err := p.Generate(context.Background(), GoCaptchaShapeSlide)
	require.NoError(t, err)

	// the hole is always at least ~25px from the left edge; 0 must miss
	ok, err := p.Verify(context.Background(), res.ID+":0", "")
	assert.NoError(t, err)
	assert.False(t, ok)
}

// 同一实例混合两种形态：Verify 按存储答案自描述的形态分发。
func TestGoCaptchaProvider_MixedShapes(t *testing.T) {
	store := newMockStore()
	p, err := NewGoCaptchaProvider(store)
	require.NoError(t, err)

	clickRes, err := p.Generate(context.Background(), GoCaptchaShapeClick)
	require.NoError(t, err)
	slideRes, err := p.Generate(context.Background(), GoCaptchaShapeSlide)
	require.NoError(t, err)

	slideAns := mustAnswer(t, store, slideRes.ID)
	frac := float64(slideAns.Block.X) / float64(slideAns.Width) * 100
	ok, err := p.Verify(context.Background(), slideRes.ID+":"+strconv.FormatFloat(frac, 'f', 4, 64), "")
	assert.NoError(t, err)
	assert.True(t, ok)

	clickAns := mustAnswer(t, store, clickRes.ID)
	var points []string
	for i := 0; i < len(clickAns.Dots); i++ {
		dot := clickAns.Dots[i]
		fx := (float64(dot.X) + float64(dot.Width)/2) / float64(clickAns.Width) * 100
		fy := (float64(dot.Y) + float64(dot.Height)/2) / float64(clickAns.Height) * 100
		points = append(points, strconv.FormatFloat(fx, 'f', 4, 64)+","+strconv.FormatFloat(fy, 'f', 4, 64))
	}
	ok, err = p.Verify(context.Background(), clickRes.ID+":"+strings.Join(points, ";"), "")
	assert.NoError(t, err)
	assert.True(t, ok)
}

func TestGoCaptchaProvider_Verify_MissingID(t *testing.T) {
	store := newMockStore()
	p, err := NewGoCaptchaProvider(store)
	require.NoError(t, err)

	ok, err := p.Verify(context.Background(), "no-such-id:50", "")
	assert.NoError(t, err)
	assert.False(t, ok)
}

func TestGoCaptchaProvider_Verify_InvalidFormat(t *testing.T) {
	store := newMockStore()
	p, err := NewGoCaptchaProvider(store)
	require.NoError(t, err)

	_, err = p.Verify(context.Background(), "invalidtoken", "")
	assert.Error(t, err)
}
