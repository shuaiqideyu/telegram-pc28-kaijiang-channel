package service

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"os"
	"strconv"

	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

const (
	DefaultDrawBg   = "assets/draw_bg.jpg"
	DefaultDrawFont = "assets/AlimamaShuHeiTi-Bold.ttf"
	jpegQuality     = 92

	issueFontPx     = 90
	ballFontPx      = 100
	ballFontPxMulti = 78
)

type drawPoint struct{ x, y int }

var drawLayout = struct {
	issueCenterX, issueCenterY int
	ballCenters                [4]drawPoint
}{
	issueCenterX: 505,
	issueCenterY: 91,
	ballCenters: [4]drawPoint{
		{174, 260},
		{444, 260},
		{710, 260},
		{1084, 260},
	},
}

var (
	colIssueRed     = color.RGBA{0xD0, 0x10, 0x10, 0xFF}
	colBallNum      = color.RGBA{0x22, 0x22, 0x22, 0xFF}
	errDrawNotReady = errors.New("开奖图渲染器未就绪")

	drawBase      *image.RGBA
	faceIssue     font.Face
	faceBall      font.Face
	faceBallMulti font.Face
	drawReady     bool
)

// InitDrawImage 预加载底图与字体。失败时调用方降级为纯文字播报。
func InitDrawImage(bgPath, fontPath string) error {
	f, err := os.Open(bgPath)
	if err != nil {
		return err
	}
	src, err := jpeg.Decode(f)
	f.Close()
	if err != nil {
		return err
	}
	b := src.Bounds()
	rgba := image.NewRGBA(b)
	draw.Draw(rgba, b, src, b.Min, draw.Src)

	fontBytes, err := os.ReadFile(fontPath)
	if err != nil {
		return err
	}
	fnt, err := truetype.Parse(fontBytes)
	if err != nil {
		return err
	}

	drawBase = rgba
	faceIssue = newDrawFace(fnt, issueFontPx)
	faceBall = newDrawFace(fnt, ballFontPx)
	faceBallMulti = newDrawFace(fnt, ballFontPxMulti)
	drawReady = true
	_, _ = RenderDraw(&DrawResult{})
	return nil
}

func RenderDraw(r *DrawResult) ([]byte, error) {
	if !drawReady || drawBase == nil {
		return nil, errDrawNotReady
	}

	canvas := image.NewRGBA(drawBase.Bounds())
	copy(canvas.Pix, drawBase.Pix)

	drawCentered(canvas, faceIssue, strconv.Itoa(r.Qihao),
		drawLayout.issueCenterX, drawLayout.issueCenterY, colIssueRed)

	vals := [4]string{
		strconv.Itoa(r.Numbers[0]),
		strconv.Itoa(r.Numbers[1]),
		strconv.Itoa(r.Numbers[2]),
		strconv.Itoa(r.Sum),
	}
	for i, v := range vals {
		face := faceBall
		if len(v) > 1 {
			face = faceBallMulti
		}
		drawCentered(canvas, face, v, drawLayout.ballCenters[i].x, drawLayout.ballCenters[i].y, colBallNum)
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, canvas, &jpeg.Options{Quality: jpegQuality}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// drawCentered 将文字墨迹几何中心对齐到 (cx,cy)。
func drawCentered(dst *image.RGBA, face font.Face, text string, cx, cy int, col color.RGBA) {
	d := &font.Drawer{Dst: dst, Src: image.NewUniform(col), Face: face}
	b, _ := font.BoundString(face, text)
	d.Dot = fixed.Point26_6{
		X: fixed.I(cx) - (b.Min.X+b.Max.X)/2,
		Y: fixed.I(cy) - (b.Min.Y+b.Max.Y)/2,
	}
	d.DrawString(text)
}

func newDrawFace(fnt *truetype.Font, px float64) font.Face {
	return truetype.NewFace(fnt, &truetype.Options{Size: px, DPI: 72, Hinting: font.HintingFull})
}
