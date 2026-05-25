package tui

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	xdraw "golang.org/x/image/draw"
)

// Protocol represents a terminal image rendering protocol.
type Protocol int

const (
	ProtocolNone     Protocol = iota // No image protocol available
	ProtocolKitty                    // Kitty graphics protocol (kitty, ghostty)
	ProtocolITerm2                   // iTerm2 inline images (iTerm2, WezTerm, Rio)
	ProtocolSixel                    // Sixel graphics (foot, mlterm, xterm+sixel)
	ProtocolChafa                    // chafa unicode art (universal fallback)
	ProtocolHalfBlock                // ANSI half-block characters (universal fallback)
)

// currentProtocol caches the detected protocol for the session.
var currentProtocol Protocol = ProtocolNone
var protocolDetected bool

// DetectTerminalProtocol returns the best image protocol for the current terminal.
func DetectTerminalProtocol() Protocol {
	if protocolDetected {
		return currentProtocol
	}
	protocolDetected = true

	termProgram := strings.ToLower(os.Getenv("TERM_PROGRAM"))
	term := strings.ToLower(os.Getenv("TERM"))

	// Kitty protocol: kitty, ghostty
	if os.Getenv("KITTY_WINDOW_ID") != "" || termProgram == "kitty" || termProgram == "ghostty" {
		currentProtocol = ProtocolKitty
		return currentProtocol
	}

	// iTerm2 protocol: iTerm.app, WezTerm, Rio
	if termProgram == "iterm.app" || termProgram == "wezterm" || termProgram == "rio" {
		currentProtocol = ProtocolITerm2
		return currentProtocol
	}

	// Sixel: foot, mlterm, or TERM containing "sixel"
	if strings.Contains(term, "sixel") || strings.Contains(termProgram, "mlterm") || strings.Contains(termProgram, "foot") {
		currentProtocol = ProtocolSixel
		return currentProtocol
	}

	// Detect sixel via infocmp
	if detectSixelSupport() {
		currentProtocol = ProtocolSixel
		return currentProtocol
	}

	// Chafa fallback: if chafa binary is available
	if hasBinary("chafa") {
		currentProtocol = ProtocolChafa
		return currentProtocol
	}

	// Half-block works everywhere with truecolor support
	if DetectTrueColorSupport() {
		currentProtocol = ProtocolHalfBlock
		return currentProtocol
	}

	currentProtocol = ProtocolNone
	return currentProtocol
}

func detectSixelSupport() bool {
	term := os.Getenv("TERM")
	if term == "" || term == "dumb" {
		return false
	}
	out, err := exec.Command("infocmp", "-1", term).Output()
	if err != nil {
		return false
	}
	s := strings.ToLower(string(out))
	return strings.Contains(s, "sixel")
}

func hasBinary(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// DetectTrueColorSupport checks if the terminal supports 24-bit color.
func DetectTrueColorSupport() bool {
	ct := os.Getenv("COLORTERM")
	if ct == "truecolor" || ct == "24bit" {
		return true
	}
	term := os.Getenv("TERM")
	switch term {
	case "vt100", "vt220", "linux", "dumb", "":
		return false
	}
	return true
}

// coverCachePath returns the cache file path for a cover image URL.
func coverCachePath(url string) string {
	hash := sha256.Sum256([]byte(url))
	ext := ".png"
	if strings.Contains(url, ".jpg") || strings.Contains(url, ".jpeg") {
		ext = ".jpg"
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "/tmp"
	}
	dir := filepath.Join(home, ".anilix", "cache", "covers")
	os.MkdirAll(dir, 0o755)
	return filepath.Join(dir, hex.EncodeToString(hash[:8])+ext)
}

// DownloadCoverImage downloads an image from a URL and returns both the decoded image
// and the local cache file path.
func DownloadCoverImage(url string) (image.Image, string, error) {
	cachePath := coverCachePath(url)

	// Try cache first
	if data, err := os.ReadFile(cachePath); err == nil {
		img, _, decErr := image.Decode(bytes.NewReader(data))
		if decErr == nil {
			return img, cachePath, nil
		}
	}

	data, err := curlGetBytes(url)
	if err != nil {
		return nil, "", fmt.Errorf("failed to download cover: %w", err)
	}

	// Save to cache
	os.WriteFile(cachePath, data, 0o644)

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "", fmt.Errorf("failed to decode cover image: %w", err)
	}

	return img, cachePath, nil
}

// RenderCoverImage renders an image for the terminal, using the best available protocol.
// It takes the decoded image, max width, detected protocol, and the cache file path.
func RenderCoverImage(img image.Image, maxWidth int, protocol Protocol, cachePath string) string {
	// Try protocol-specific renderers that need a file
	if cachePath != "" {
		switch protocol {
		case ProtocolKitty:
			if result, err := renderKittyCover(cachePath, maxWidth); err == nil && result != "" {
				return result
			}
		case ProtocolITerm2:
			if result, err := renderITerm2Cover(cachePath, maxWidth); err == nil && result != "" {
				return result
			}
		case ProtocolSixel:
			if result, err := renderSixelCover(cachePath, maxWidth); err == nil && result != "" {
				return result
			}
		case ProtocolChafa:
			if result, err := renderChafaCover(cachePath, maxWidth); err == nil && result != "" {
				return result
			}
		}
	}

	// Fallback: ANSI half-block renderer
	return renderHalfBlock(img, maxWidth)
}

// renderKittyCover renders an image using the Kitty graphics protocol.
// The Kitty protocol only supports PNG (f=100), RGB (f=24), and RGBA (f=32).
// Non-PNG images must be decoded and re-encoded as PNG before transmission.
func renderKittyCover(path string, width int) (string, error) {
	lowerPath := strings.ToLower(path)
	isPNG := strings.HasSuffix(lowerPath, ".png")

	var pngData []byte
	if isPNG {
		// PNG can be sent directly
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		pngData = data
	} else {
		// Decode any format (JPEG, GIF, etc.) and re-encode as PNG
		f, err := os.Open(path)
		if err != nil {
			return "", err
		}
		defer f.Close()

		img, _, err := image.Decode(f)
		if err != nil {
			return "", err
		}

		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			return "", err
		}
		pngData = buf.Bytes()
	}

	b64 := base64.StdEncoding.EncodeToString(pngData)
	chunks := splitBase64(b64, 4096)

	var result strings.Builder
	// Approximate height from width (cover art ~3:2 aspect)
	height := width * 2

	for i, chunk := range chunks {
		more := 0
		if i < len(chunks)-1 {
			more = 1
		}
		if i == 0 {
			fmt.Fprintf(&result, "\033_Ga=T,f=100,c=%d,r=%d,m=%d;%s\033\\", width, height, more, chunk)
		} else {
			fmt.Fprintf(&result, "\033_Gm=%d;%s\033\\", more, chunk)
		}
	}

	return result.String(), nil
}

// renderITerm2Cover renders an image using the iTerm2 inline image protocol.
func renderITerm2Cover(path string, width int) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	b64 := base64.StdEncoding.EncodeToString(data)
	pixelWidth := width * 8

	return fmt.Sprintf("\033]1337;File=size=%d;width=%dpx;inline=1:%s\a",
		len(data), pixelWidth, b64), nil
}

// renderSixelCover renders an image in Sixel format.
func renderSixelCover(path string, width int) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return "", err
	}

	pixelW := width * 8
	pixelH := pixelW * 2
	if pixelW <= 0 {
		pixelW = 200
	}

	resized := resizeForSixel(img, pixelW, pixelH)
	return encodeSixel(resized, pixelW, pixelH)
}

// renderChafaCover renders an image as unicode art using the chafa binary.
func renderChafaCover(path string, width int) (string, error) {
	height := width * 2
	size := fmt.Sprintf("%dx%d", width, height)

	cmd := exec.Command("chafa",
		"--size="+size,
		"--format=symbols",
		"--color-space=rgb",
		"--work=9",
		"--align=left",
		path,
	)

	out, err := cmd.Output()
	if err != nil {
		return "", err
	}

	result := strings.TrimRight(string(out), "\n")
	lines := strings.Split(result, "\n")
	if len(lines) > height {
		lines = lines[:height]
		result = strings.Join(lines, "\n")
	}

	return result, nil
}

// resizeForSixel resizes an image using nearest-neighbor scaling.
func resizeForSixel(src image.Image, w, h int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	xdraw.NearestNeighbor.Scale(dst, dst.Bounds(), src, src.Bounds(), xdraw.Over, nil)
	return dst
}

// encodeSixel encodes an RGBA image as a sixel string.
func encodeSixel(img *image.RGBA, w, h int) (string, error) {
	var buf bytes.Buffer
	buf.WriteString("\033Pq")

	palette := buildSixelPalette()

	for i, c := range palette {
		r, g, b, _ := c.RGBA()
		buf.WriteString(fmt.Sprintf("#%d;2;%d;%d;%d;",
			i, int(r*100/65535), int(g*100/65535), int(b*100/65535)))
	}

	sixRows := (h + 5) / 6
	for sy := 0; sy < sixRows; sy++ {
		for ci, c := range palette {
			buf.WriteString(fmt.Sprintf("#%d", ci))
			var run byte
			count := 0

			for x := 0; x < w; x++ {
				var bits byte
				for dy := 0; dy < 6; dy++ {
					py := sy*6 + dy
					if py < h {
						r, g, b, _ := img.At(x, py).RGBA()
						cr, cg, cb, _ := c.RGBA()
						if sixelColorMatch(r, cr) && sixelColorMatch(g, cg) && sixelColorMatch(b, cb) {
							bits |= 1 << uint(dy)
						}
					}
				}
				bits += 63

				if count == 0 {
					run = bits
					count = 1
				} else if bits == run {
					count++
				} else {
					writeSixelRun(&buf, run, count)
					run = bits
					count = 1
				}
			}
			if count > 0 {
				writeSixelRun(&buf, run, count)
			}
			buf.WriteString("$")
		}
		buf.WriteString("-")
	}

	buf.WriteString("\033\\")
	return buf.String(), nil
}

func writeSixelRun(buf *bytes.Buffer, char byte, count int) {
	if count <= 2 {
		for i := 0; i < count; i++ {
			buf.WriteByte(char)
		}
	} else {
		buf.WriteString(fmt.Sprintf("!%d%c", count, char))
	}
}

func buildSixelPalette() []color.RGBA {
	var palette []color.RGBA
	for r := 0; r < 6; r++ {
		for g := 0; g < 6; g++ {
			for b := 0; b < 6; b++ {
				palette = append(palette, color.RGBA{
					R: uint8(r * 51), G: uint8(g * 51), B: uint8(b * 51), A: 255,
				})
			}
		}
	}
	return palette
}

func sixelColorMatch(a, b uint32) bool {
	diff := a - b
	if b > a {
		diff = b - a
	}
	return diff < 16000
}

func splitBase64(s string, n int) []string {
	if n <= 0 {
		return []string{s}
	}
	var chunks []string
	for len(s) > 0 {
		end := n
		if end > len(s) {
			end = len(s)
		}
		chunks = append(chunks, s[:end])
		s = s[end:]
	}
	if len(chunks) == 0 {
		chunks = []string{""}
	}
	return chunks
}

// ResizeImage resizes an image to fit within maxWidth x maxHeight while
// preserving aspect ratio, using high-quality BiLinear scaling.
func ResizeImage(img image.Image, maxWidth, maxHeight int) image.Image {
	bounds := img.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()

	ratioW := float64(maxWidth) / float64(srcW)
	ratioH := float64(maxHeight) / float64(srcH)
	ratio := ratioW
	if ratioH < ratioW {
		ratio = ratioH
	}

	targetW := int(float64(srcW) * ratio)
	targetH := int(float64(srcH) * ratio)
	if targetW < 1 {
		targetW = 1
	}
	if targetH < 1 {
		targetH = 1
	}

	dst := image.NewRGBA(image.Rect(0, 0, targetW, targetH))
	xdraw.BiLinear.Scale(dst, dst.Bounds(), img, bounds, xdraw.Over, nil)
	return dst
}

// renderHalfBlock renders an image as ANSI-colored half-block characters.
// Each terminal row uses the ▀ character to display two vertical pixels.
func renderHalfBlock(img image.Image, maxWidth int) string {
	maxRows := maxWidth * 2
	if maxRows > 30 {
		maxRows = 30
	}
	resized := ResizeImage(img, maxWidth, maxRows)

	bounds := resized.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	var sb strings.Builder
	for y := 0; y < h; y += 2 {
		for x := 0; x < w; x++ {
			r1, g1, b1, _ := resized.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			var r2, g2, b2 uint32
			if y+1 < h {
				r2, g2, b2, _ = resized.At(bounds.Min.X+x, bounds.Min.Y+y+1).RGBA()
			}
			fg := fmt.Sprintf("\033[38;2;%d;%d;%dm", r1>>8, g1>>8, b1>>8)
			bg := fmt.Sprintf("\033[48;2;%d;%d;%dm", r2>>8, g2>>8, b2>>8)
			sb.WriteString(fg)
			sb.WriteString(bg)
			sb.WriteRune('\u2580')
		}
		sb.WriteString("\033[0m")
		if y+2 < h {
			sb.WriteRune('\n')
		}
	}
	sb.WriteString("\033[0m")
	return sb.String()
}

// curlGetBytes downloads a URL and returns raw bytes using curl.
func curlGetBytes(url string) ([]byte, error) {
	cmd := exec.Command("curl", "-s", "-L",
		"-H", "User-Agent: Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
		url)
	return cmd.Output()
}
