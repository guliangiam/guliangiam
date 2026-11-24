package captcha

import (
	"context"
	"encoding/json"
	"image"
	"math"
	"strconv"
	"strings"
	"sync"

	"github.com/golang/freetype/truetype"
	"github.com/wenlng/go-captcha-assets/bindata/chars"
	"github.com/wenlng/go-captcha-assets/resources/fonts/fzshengsksjw"
	"github.com/wenlng/go-captcha-assets/resources/imagesv2"
	"github.com/wenlng/go-captcha-assets/resources/tiles"
	"github.com/wenlng/go-captcha/v2/click"
	"github.com/wenlng/go-captcha/v2/slide"

	"github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/uuid"
)

// go-captcha/v2 形态常量（闭集：click 文字点选 / slide 基础滑动拼图）。
const (
	GoCaptchaShapeClick = "click"
	GoCaptchaShapeSlide = "slide"

	// 未指定形态时的默认值。
	goCaptchaDefaultShape = GoCaptchaShapeClick
)

const (
	// 点击命中/滑动到位的像素容差，与官方示例一致。
	goCaptchaClickPadding = 5
	goCaptchaSlidePadding = 5

	// 答案在共享 Store 中的键前缀，避免与 base64Captcha 的 id 冲突。
	goCaptchaStorePrefix = "gc:"
)

// GoCaptchaGenerateResult 是一次 go_captcha 挑战的服务端产物。
// MasterImage 为 JPEG 主图，SubImage 形态相关：click=提示缩略图（PNG），
// slide=待拖动瓦片（PNG）。
type GoCaptchaGenerateResult struct {
	ID          string
	Shape       string
	MasterImage []byte
	SubImage    []byte
}

// GoCaptchaProvider 是基于 github.com/wenlng/go-captcha/v2 的本地图形验证码
// provider（provider_type="go_captcha"），同一实例支持点选（click）与滑动拼图
// （slide）两种形态；答案 JSON（含形态与主图尺寸，自描述）存入共享 Store
// （Redis / 内存降级），Verify 按存储的形态分发校验，token 无需携带形态。
//
// token 契约（与 default 的 "id:code" 同构）：
//   - click: "id:fx,fy;fx,fy" —— 按缩略图顺序点击，坐标为相对主图的百分比（0-100）
//   - slide: "id:frac" —— 瓦片最终左缘 X 相对主图宽度的百分比（0-100）
type GoCaptchaProvider struct {
	store     Store
	clickCapt click.Captcha
	slideCapt slide.Captcha
}

var _ Provider = (*GoCaptchaProvider)(nil)

// 资源解码昂贵（bindata 图片解码），进程级加载一次。
var (
	goCaptchaResOnce sync.Once
	goCaptchaFonts   []*truetype.Font
	goCaptchaChars   []string
	goCaptchaBGs     []image.Image
	goCaptchaTiles   []*slide.GraphImage
)

func loadGoCaptchaResources() {
	goCaptchaResOnce.Do(func() {
		if font, err := fzshengsksjw.GetFont(); err == nil && font != nil {
			goCaptchaFonts = []*truetype.Font{font}
		}
		goCaptchaChars = chars.GetChineseChars()
		if imgs, err := imagesv2.GetImages(); err == nil {
			goCaptchaBGs = imgs
		}
		if ts, err := tiles.GetTiles(); err == nil {
			converted := make([]*slide.GraphImage, 0, len(ts))
			for _, t := range ts {
				converted = append(converted, &slide.GraphImage{
					OverlayImage: t.OverlayImage,
					ShadowImage:  t.ShadowImage,
					MaskImage:    t.MaskImage,
				})
			}
			goCaptchaTiles = converted
		}
	})
}

// NewGoCaptchaProvider 构造形态无关的 provider：click/slide 各自在资源可用时
// 构建；两组资源都不可用时返回 error（调用方 fail-open 决定是否降级）。
func NewGoCaptchaProvider(store Store) (*GoCaptchaProvider, error) {
	loadGoCaptchaResources()

	p := &GoCaptchaProvider{store: store}
	if len(goCaptchaFonts) > 0 && len(goCaptchaChars) > 0 {
		builder := click.NewBuilder()
		builder.SetResources(
			click.WithChars(goCaptchaChars),
			click.WithFonts(goCaptchaFonts),
		)
		if len(goCaptchaBGs) > 0 {
			builder.SetResources(click.WithBackgrounds(goCaptchaBGs))
		}
		p.clickCapt = builder.Make()
	}
	if len(goCaptchaBGs) > 0 && len(goCaptchaTiles) > 0 {
		builder := slide.NewBuilder()
		builder.SetResources(
			slide.WithBackgrounds(goCaptchaBGs),
			slide.WithGraphImages(goCaptchaTiles),
		)
		p.slideCapt = builder.Make()
	}
	if p.clickCapt == nil && p.slideCapt == nil {
		return nil, errors.NewInternal("go-captcha resources unavailable")
	}
	return p, nil
}

// goCaptchaAnswer 是存入共享 Store 的答案（自描述形态与主图尺寸，
// Verify 据此分发并把百分比坐标换算回像素）。
type goCaptchaAnswer struct {
	Shape  string             `json:"shape"`
	Width  int                `json:"width"`
	Height int                `json:"height"`
	Dots   map[int]*click.Dot `json:"dots,omitempty"`
	Block  *slide.Block       `json:"block,omitempty"`
}

// Generate 按形态生成一次挑战：把答案 JSON 写入共享 Store 并返回图片产物。
// shape 为空时取默认形态（click）。
func (p *GoCaptchaProvider) Generate(ctx context.Context, shape string) (*GoCaptchaGenerateResult, error) {
	shape = strings.ToLower(strings.TrimSpace(shape))
	if shape == "" {
		shape = goCaptchaDefaultShape
	}

	id, err := uuid.V7Generator{}.NewString()
	if err != nil {
		return nil, errors.WrapInternalf(err, "generate go-captcha id")
	}

	res := &GoCaptchaGenerateResult{ID: id, Shape: shape}
	key := goCaptchaStorePrefix + id

	var ans *goCaptchaAnswer
	switch shape {
	case GoCaptchaShapeClick:
		if p.clickCapt == nil {
			return nil, errors.NewInternal("go-captcha click resources (fonts/chars) unavailable")
		}
		data, err := p.clickCapt.Generate()
		if err != nil {
			return nil, errors.WrapInternalf(err, "generate go-captcha click challenge")
		}
		master, err := data.GetMasterImage().ToBytes()
		if err != nil {
			return nil, errors.WrapInternalf(err, "encode go-captcha master image")
		}
		thumb, err := data.GetThumbImage().ToBytes()
		if err != nil {
			return nil, errors.WrapInternalf(err, "encode go-captcha thumb image")
		}
		size := p.clickCapt.GetOptions().GetImageSize()
		ans = &goCaptchaAnswer{Shape: shape, Width: size.Width, Height: size.Height, Dots: data.GetData()}
		res.MasterImage, res.SubImage = master, thumb
	case GoCaptchaShapeSlide:
		if p.slideCapt == nil {
			return nil, errors.NewInternal("go-captcha slide resources (backgrounds/tiles) unavailable")
		}
		data, err := p.slideCapt.Generate()
		if err != nil {
			return nil, errors.WrapInternalf(err, "generate go-captcha slide challenge")
		}
		master, err := data.GetMasterImage().ToBytes()
		if err != nil {
			return nil, errors.WrapInternalf(err, "encode go-captcha master image")
		}
		tileImg, err := data.GetTileImage().ToBytes()
		if err != nil {
			return nil, errors.WrapInternalf(err, "encode go-captcha tile image")
		}
		size := p.slideCapt.GetOptions().GetImageSize()
		ans = &goCaptchaAnswer{Shape: shape, Width: size.Width, Height: size.Height, Block: data.GetData()}
		res.MasterImage, res.SubImage = master, tileImg
	default:
		return nil, errors.NewInvalidArgumentf("unsupported go-captcha shape: %s", shape)
	}

	payload, err := json.Marshal(ans)
	if err != nil {
		return nil, errors.WrapInternalf(err, "encode go-captcha %s answer", shape)
	}
	if err := p.store.Set(ctx, key, string(payload)); err != nil {
		return nil, errors.WrapInternalf(err, "store go-captcha %s answer", shape)
	}
	return res, nil
}

// Verify 校验 "id:payload" token；形态由存储答案自描述，一次性消费（校验前删除答案）。
func (p *GoCaptchaProvider) Verify(ctx context.Context, token, remoteIP string) (bool, error) {
	parts := strings.SplitN(token, ":", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return false, errors.NewInvalidArgument("invalid captcha token format")
	}

	id := parts[0]
	payload := parts[1]
	key := goCaptchaStorePrefix + id

	stored, err := p.store.Get(ctx, key)
	if err != nil || stored == "" {
		// 缺失/过期视为校验失败，而非基础设施错误。
		return false, nil
	}
	_ = p.store.Delete(ctx, key)

	var ans goCaptchaAnswer
	if err := json.Unmarshal([]byte(stored), &ans); err != nil {
		return false, errors.WrapInternalf(err, "decode go-captcha answer")
	}

	switch ans.Shape {
	case GoCaptchaShapeClick:
		return p.verifyClick(payload, &ans)
	case GoCaptchaShapeSlide:
		return p.verifySlide(payload, &ans)
	default:
		return false, errors.NewInvalidArgumentf("unsupported go-captcha shape: %s", ans.Shape)
	}
}

func (p *GoCaptchaProvider) verifyClick(payload string, ans *goCaptchaAnswer) (bool, error) {
	if len(ans.Dots) == 0 || ans.Width <= 0 || ans.Height <= 0 {
		return false, nil
	}

	points := strings.Split(payload, ";")
	if len(points) != len(ans.Dots) {
		return false, nil
	}

	for i, point := range points {
		xy := strings.Split(point, ",")
		if len(xy) != 2 {
			return false, nil
		}
		fx, err1 := strconv.ParseFloat(strings.TrimSpace(xy[0]), 64)
		fy, err2 := strconv.ParseFloat(strings.TrimSpace(xy[1]), 64)
		if err1 != nil || err2 != nil {
			return false, nil
		}
		dot, ok := ans.Dots[i]
		if !ok {
			return false, nil
		}
		sx := int(math.Round(fx / 100 * float64(ans.Width)))
		sy := int(math.Round(fy / 100 * float64(ans.Height)))
		if !click.Validate(sx, sy, dot.X, dot.Y, dot.Width, dot.Height, goCaptchaClickPadding) {
			return false, nil
		}
	}
	return true, nil
}

func (p *GoCaptchaProvider) verifySlide(payload string, ans *goCaptchaAnswer) (bool, error) {
	if ans.Block == nil || ans.Width <= 0 {
		return false, nil
	}

	fx, err := strconv.ParseFloat(strings.TrimSpace(payload), 64)
	if err != nil {
		return false, nil
	}
	sx := int(math.Round(fx / 100 * float64(ans.Width)))
	// 基础滑动模式只校验水平位置，纵坐标由客户端保持不变。
	return slide.Validate(sx, ans.Block.Y, ans.Block.X, ans.Block.Y, goCaptchaSlidePadding), nil
}
