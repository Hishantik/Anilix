package tui

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	termimg "github.com/blacktop/go-termimg"
	xdraw "golang.org/x/image/draw"
)

// Protocol represents a terminal image rendering protocol.
type Protocol int

const (
	ProtocolNone      Protocol = iota // No image protocol available
	ProtocolHalfBlock                 // ANSI half-block characters via go-termimg/mosaic
	ProtocolChafa                     // chafa unicode art (universal fallback)
)

// currentProtocol caches the detected protocol for the session.
var currentProtocol Protocol = ProtocolNone
var protocolDetected bool

// DetectTerminalProtocol returns the best image protocol for the current terminal.
//
// Bubbletea v2's ultraviolet cell-based renderer strips all zero-width escape
// sequences (APC, OSC, DCS) from view.Content. Only pure SGR text survives —
// so we use go-termimg's half-block renderer (via charmbracelet/x/mosaic)
// which produces ANSI color text that the cell buffer can represent.
func DetectTerminalProtocol() Protocol {
	if protocolDetected {
		return currentProtocol
	}
	protocolDetected = true

	// Half-block via go-termimg (mosaic) — works everywhere with truecolor
	if DetectTrueColorSupport() {
		currentProtocol = ProtocolHalfBlock
		return currentProtocol
	}

	// Chafa fallback: if chafa binary is available
	if hasBinary("chafa") {
		currentProtocol = ProtocolChafa
		return currentProtocol
	}

	currentProtocol = ProtocolNone
	return currentProtocol
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
func RenderCoverImage(img image.Image, maxWidth int, protocol Protocol, cachePath string) string {
	switch protocol {
	case ProtocolHalfBlock:
		return renderGoTermimg(img, maxWidth)
	case ProtocolChafa:
		if cachePath != "" {
			if result, err := renderChafaCover(cachePath, maxWidth); err == nil && result != "" {
				return result
			}
		}
		// Fall through to half-block if chafa fails
		return renderGoTermimg(img, maxWidth)
	}

	return renderGoTermimg(img, maxWidth)
}

// renderGoTermimg renders an image using go-termimg's half-block renderer
// (backed by charmbracelet/x/mosaic). Produces pure ANSI SGR text that
// survives bubbletea v2's ultraviolet cell buffer.
func renderGoTermimg(img image.Image, maxWidth int) string {
	maxHeight := maxWidth * 2
	if maxHeight > 30 {
		maxHeight = 30
	}

	ti := termimg.New(img).
		Protocol(termimg.Halfblocks).
		Width(maxWidth).
		Height(maxHeight).
		Scale(termimg.ScaleFit)

	output, err := ti.Render()
	if err != nil {
		// Ultimate fallback: manual half-block
		return renderHalfBlock(img, maxWidth)
	}

	return output
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

// renderHalfBlock is the ultimate fallback — manual half-block rendering
// without any external dependencies.
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
