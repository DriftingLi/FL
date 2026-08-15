package captcha

import (
	"bytes"
	"crypto/rand"
	"image"
	"image/color"
	"image/png"
	"math/big"
	"strconv"
)

// glyphs 5x7 点阵字体：数字、运算符、等号、问号（'1' 为实心）。
var glyphs = map[rune][7]string{
	'0': {"01110", "10001", "10011", "10101", "11001", "10001", "01110"},
	'1': {"00100", "01100", "00100", "00100", "00100", "00100", "01110"},
	'2': {"01110", "10001", "00001", "00010", "00100", "01000", "11111"},
	'3': {"11110", "00001", "00001", "01110", "00001", "00001", "11110"},
	'4': {"00010", "00110", "01010", "10010", "11111", "00010", "00010"},
	'5': {"11111", "10000", "11110", "00001", "00001", "10001", "01110"},
	'6': {"00110", "01000", "10000", "11110", "10001", "10001", "01110"},
	'7': {"11111", "00001", "00010", "00100", "01000", "01000", "01000"},
	'8': {"01110", "10001", "10001", "01110", "10001", "10001", "01110"},
	'9': {"01110", "10001", "10001", "01111", "00001", "00010", "01100"},
	'+': {"00000", "00100", "00100", "11111", "00100", "00100", "00000"},
	'-': {"00000", "00000", "00000", "11111", "00000", "00000", "00000"},
	'×': {"00000", "10001", "01010", "00100", "01010", "10001", "00000"},
	'÷': {"00000", "00100", "00000", "11111", "00000", "00100", "00000"},
	'=': {"00000", "00000", "11111", "00000", "11111", "00000", "00000"},
	'?': {"01110", "10001", "00001", "00010", "00100", "00000", "00100"},
}

const (
	glyphW     = 5
	glyphH     = 7
	glyphScale = 6 // 缩放后单字符 30x42
	padding    = 16
)

// randInt 返回 [0, max) 的加密安全随机整数。
func randInt(max int) int {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil || n == nil {
		return 0
	}
	return int(n.Int64())
}

// Equation 算术验证码题目。
type Equation struct {
	Text   string // 展示文本，如 "7+3=?"
	Answer int
}

// NewEquation 生成随机算术式（加减乘除，保证非负整数结果）。
func NewEquation() Equation {
	switch randInt(4) {
	case 0:
		a, b := randInt(10), randInt(10)
		return Equation{Text: strconv.Itoa(a) + "+" + strconv.Itoa(b) + "=?", Answer: a + b}
	case 1:
		a, b := randInt(10), randInt(10)
		if a < b {
			a, b = b, a
		}
		return Equation{Text: strconv.Itoa(a) + "-" + strconv.Itoa(b) + "=?", Answer: a - b}
	case 2:
		a, b := randInt(10), randInt(10)
		return Equation{Text: strconv.Itoa(a) + "×" + strconv.Itoa(b) + "=?", Answer: a * b}
	default:
		b := 1 + randInt(9)
		q := 1 + randInt(9)
		return Equation{Text: strconv.Itoa(b*q) + "÷" + strconv.Itoa(b) + "=?", Answer: q}
	}
}

// RenderPNG 将文本渲染为验证码 PNG 图片（浅色背景 + 彩色字符 + 干扰线/噪点）。
func RenderPNG(text string) []byte {
	runes := []rune(text)
	w := len(runes)*glyphW*glyphScale + padding*2
	h := glyphH*glyphScale + padding*2
	img := image.NewRGBA(image.Rect(0, 0, w, h))

	fill(img, color.RGBA{uint8(235 + randInt(21)), uint8(235 + randInt(21)), uint8(235 + randInt(21)), 255})

	x := padding
	for _, r := range runes {
		drawGlyph(img, r, x, padding, darkColor())
		x += glyphW * glyphScale
	}

	// 干扰线 + 噪点
	for i := 0; i < 3; i++ {
		drawLine(img, randInt(w), randInt(h), randInt(w), randInt(h), darkColor())
	}
	for i := 0; i < 60; i++ {
		img.Set(randInt(w), randInt(h), darkColor())
	}

	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

// darkColor 返回一个随机深色（适合做前景）。
func darkColor() color.RGBA {
	return color.RGBA{uint8(randInt(161)), uint8(randInt(161)), uint8(randInt(161)), 255}
}

func fill(img *image.RGBA, c color.RGBA) {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			img.Set(x, y, c)
		}
	}
}

func drawGlyph(img *image.RGBA, r rune, x, y int, c color.RGBA) {
	rows, ok := glyphs[r]
	if !ok {
		rows = glyphs['?']
	}
	for gy, row := range rows {
		for gx := 0; gx < glyphW; gx++ {
			if row[gx] != '1' {
				continue
			}
			for dy := 0; dy < glyphScale; dy++ {
				for dx := 0; dx < glyphScale; dx++ {
					img.Set(x+gx*glyphScale+dx, y+gy*glyphScale+dy, c)
				}
			}
		}
	}
}

func drawLine(img *image.RGBA, x0, y0, x1, y1 int, c color.RGBA) {
	dx := x1 - x0
	dy := y1 - y0
	steps := dx
	if steps < 0 {
		steps = -steps
	}
	ady := dy
	if ady < 0 {
		ady = -ady
	}
	if ady > steps {
		steps = ady
	}
	if steps == 0 {
		img.Set(x0, y0, c)
		return
	}
	for i := 0; i <= steps; i++ {
		img.Set(x0+dx*i/steps, y0+dy*i/steps, c)
	}
}
