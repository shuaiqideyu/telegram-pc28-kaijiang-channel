package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"kan28/service"
	"log"
	"os"
	"path/filepath"

	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

func main() {
	root, _ := os.Getwd()
	// allow run from repo root or tools/gen_examples
	for _, cand := range []string{root, filepath.Join(root, "../..")} {
		if _, err := os.Stat(filepath.Join(cand, "assets/draw_bg.jpg")); err == nil {
			root = cand
			break
		}
	}
	if err := os.Chdir(root); err != nil {
		log.Fatal(err)
	}
	if err := service.InitDrawImage(service.DefaultDrawBg, service.DefaultDrawFont); err != nil {
		log.Fatal(err)
	}

	samples := []service.DrawResult{
		{Qihao: 3200001, Numbers: [3]int{0, 0, 0}, Sum: 0, SizeType: "小", ParityType: "双", Pattern: "豹子"},
		{Qihao: 3200002, Numbers: [3]int{9, 9, 9}, Sum: 27, SizeType: "大", ParityType: "单", Pattern: "豹子"},
	}
	names := []string{"example_0+0+0=00.jpg", "example_9+9+9=27.jpg"}

	fontBytes, err := os.ReadFile(service.DefaultDrawFont)
	if err != nil {
		log.Fatal(err)
	}
	fnt, err := truetype.Parse(fontBytes)
	if err != nil {
		log.Fatal(err)
	}
	faceCaption := truetype.NewFace(fnt, &truetype.Options{Size: 36, DPI: 72, Hinting: font.HintingFull})
	faceBtn := truetype.NewFace(fnt, &truetype.Options{Size: 28, DPI: 72, Hinting: font.HintingFull})
	defer faceCaption.Close()
	defer faceBtn.Close()

	outDir := "assets/examples"
	if err := os.MkdirAll(outDir, 0755); err != nil {
		log.Fatal(err)
	}

	for i, r := range samples {
		card, err := service.RenderDraw(&r)
		if err != nil {
			log.Fatal(err)
		}
		img, err := jpeg.Decode(bytes.NewReader(card))
		if err != nil {
			log.Fatal(err)
		}

		caption := fmt.Sprintf("第%d期  %d+%d+%d=%02d  %s%s %s",
			r.Qihao, r.Numbers[0], r.Numbers[1], r.Numbers[2], r.Sum, r.SizeType, r.ParityType, r.Pattern)

		preview := composeTelegramPreview(img, caption, faceCaption, faceBtn)
		path := filepath.Join(outDir, names[i])
		f, err := os.Create(path)
		if err != nil {
			log.Fatal(err)
		}
		if err := jpeg.Encode(f, preview, &jpeg.Options{Quality: 92}); err != nil {
			f.Close()
			log.Fatal(err)
		}
		f.Close()
		fmt.Println("wrote", path)
	}
}

func composeTelegramPreview(card image.Image, caption string, faceCaption, faceBtn font.Face) *image.RGBA {
	const (
		pad     = 28
		gap     = 22
		btnH    = 56
		btnGap  = 12
		captionH = 70
		bottom  = 36
	)
	cb := card.Bounds()
	w := cb.Dx() + pad*2
	h := pad + cb.Dy() + gap + captionH + gap + btnH + bottom

	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	bg := color.RGBA{0xF0, 0xF2, 0xF5, 0xFF} // TG-like light gray
	draw.Draw(dst, dst.Bounds(), &image.Uniform{bg}, image.Point{}, draw.Src)

	// white message card
	msg := image.Rect(pad/2, pad/2, w-pad/2, h-pad/2)
	roundedFill(dst, msg, color.RGBA{0xFF, 0xFF, 0xFF, 0xFF})

	cardRect := image.Rect(pad, pad, pad+cb.Dx(), pad+cb.Dy())
	draw.Draw(dst, cardRect, card, cb.Min, draw.Src)

	cy := pad + cb.Dy() + gap + 40
	drawText(dst, faceCaption, caption, pad+8, cy, color.RGBA{0x11, 0x11, 0x11, 0xFF})

	btns := []struct {
		label string
		col   color.RGBA
	}{
		{"遗漏", color.RGBA{0xE5, 0x3E, 0x3E, 0xFF}},
		{"统计", color.RGBA{0x31, 0xB5, 0x4A, 0xFF}},
		{"对应", color.RGBA{0x2A, 0xAB, 0xEE, 0xFF}},
	}
	by := pad + cb.Dy() + gap + captionH + 4
	bw := (cb.Dx() - btnGap*2) / 3
	for i, b := range btns {
		x0 := pad + i*(bw+btnGap)
		r := image.Rect(x0, by, x0+bw, by+btnH)
		roundedFill(dst, r, b.col)
		tw := font.MeasureString(faceBtn, b.label).Ceil()
		tx := x0 + (bw-tw)/2
		ty := by + btnH/2 + 10
		drawText(dst, faceBtn, b.label, tx, ty, color.RGBA{0xFF, 0xFF, 0xFF, 0xFF})
	}
	return dst
}

func drawText(dst *image.RGBA, face font.Face, s string, x, y int, col color.RGBA) {
	d := &font.Drawer{Dst: dst, Src: image.NewUniform(col), Face: face}
	d.Dot = fixed.P(x, y)
	d.DrawString(s)
}

func roundedFill(dst *image.RGBA, r image.Rectangle, col color.RGBA) {
	// simple soft rectangle (no real radius lib); fill rect is enough for example
	draw.Draw(dst, r, &image.Uniform{col}, image.Point{}, draw.Src)
}
