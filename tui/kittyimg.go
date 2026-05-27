package tui

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"math/rand/v2"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// combiningMarks is a table of Unicode combining marks used by Kitty's
// Unicode placeholder protocol to encode row, column, and mask indices.
var combiningMarks = []string{
	"\u0305", "\u030D", "\u030E", "\u0310", "\u0312", "\u033D", "\u033E", "\u033F",
	"\u0346", "\u034A", "\u034B", "\u034C", "\u0350", "\u0351", "\u0352", "\u0357",
	"\u035B", "\u0363", "\u0364", "\u0365", "\u0366", "\u0367", "\u0368", "\u0369",
	"\u036A", "\u036B", "\u036C", "\u036D", "\u036E", "\u036F", "\u0483", "\u0484",
	"\u0485", "\u0486", "\u0487", "\u0592", "\u0593", "\u0594", "\u0595", "\u0597",
	"\u0598", "\u0599", "\u059C", "\u059D", "\u059E", "\u059F", "\u05A0", "\u05A1",
	"\u05A8", "\u05A9", "\u05AB", "\u05AC", "\u05AF", "\u05C4", "\u0610", "\u0611",
	"\u0612", "\u0613", "\u0614", "\u0615", "\u0616", "\u0617", "\u0657", "\u0658",
	"\u0659", "\u065A", "\u065B", "\u065D", "\u065E", "\u06D6", "\u06D7", "\u06D8",
	"\u06D9", "\u06DA", "\u06DB", "\u06DC", "\u06DF", "\u06E0", "\u06E1", "\u06E2",
	"\u06E4", "\u06E7", "\u06E8", "\u06EB", "\u06EC", "\u0730", "\u0732", "\u0733",
	"\u0735", "\u0736", "\u073A", "\u073D", "\u073F", "\u0740", "\u0741", "\u0743",
	"\u0745", "\u0747", "\u0749", "\u074A", "\u07EB", "\u07EC", "\u07ED", "\u07EE",
	"\u07EF", "\u07F0", "\u07F1", "\u07F3", "\u0816", "\u0817", "\u0818", "\u0819",
	"\u081B", "\u081C", "\u081D", "\u081E", "\u081F", "\u0820", "\u0821", "\u0822",
	"\u0823", "\u0825", "\u0826", "\u0827", "\u0829", "\u082A", "\u082B", "\u082C",
	"\u082D", "\u0951", "\u0953", "\u0954", "\u0F82", "\u0F83", "\u0F86", "\u0F87",
	"\u135D", "\u135E", "\u135F", "\u17DD", "\u193A", "\u1A17", "\u1A75", "\u1A76",
	"\u1A77", "\u1A78", "\u1A79", "\u1A7A", "\u1A7B", "\u1A7C", "\u1B6B", "\u1B6D",
	"\u1B6E", "\u1B6F", "\u1B70", "\u1B71", "\u1B72", "\u1B73", "\u1CD0", "\u1CD1",
	"\u1CD2", "\u1CDA", "\u1CDB", "\u1CE0", "\u1DC0", "\u1DC1", "\u1DC3", "\u1DC4",
	"\u1DC5", "\u1DC6", "\u1DC7", "\u1DC8", "\u1DC9", "\u1DCB", "\u1DCC", "\u1DD1",
	"\u1DD2", "\u1DD3", "\u1DD4", "\u1DD5", "\u1DD6", "\u1DD7", "\u1DD8", "\u1DD9",
	"\u1DDA", "\u1DDB", "\u1DDC", "\u1DDD", "\u1DDE", "\u1DDF", "\u1DE0", "\u1DE1",
	"\u1DE2", "\u1DE3", "\u1DE4", "\u1DE5", "\u1DE6", "\u1DFE", "\u20D0", "\u20D1",
	"\u20D4", "\u20D5", "\u20D6", "\u20D7", "\u20DB", "\u20DC", "\u20E1", "\u20E7",
	"\u20E9", "\u20F0", "\u2CEF", "\u2CF0", "\u2CF1", "\u2DE0", "\u2DE1", "\u2DE2",
	"\u2DE3", "\u2DE4", "\u2DE5", "\u2DE6", "\u2DE7", "\u2DE8", "\u2DE9", "\u2DEA",
	"\u2DEB", "\u2DEC", "\u2DED", "\u2DEE", "\u2DEF", "\u2DF0", "\u2DF1", "\u2DF2",
	"\u2DF3", "\u2DF4", "\u2DF5", "\u2DF6", "\u2DF7", "\u2DF8", "\u2DF9", "\u2DFA",
	"\u2DFB", "\u2DFC", "\u2DFD", "\u2DFE", "\u2DFF", "\uA66F", "\uA67C", "\uA67D",
	"\uA6F0", "\uA6F1", "\uA8E0", "\uA8E1", "\uA8E2", "\uA8E3", "\uA8E4", "\uA8E5",
	"\uA8E6", "\uA8E7", "\uA8E8", "\uA8E9", "\uA8EA", "\uA8EB", "\uA8EC", "\uA8ED",
	"\uA8EE", "\uA8EF", "\uA8F0", "\uA8F1", "\uAAB0", "\uAAB2", "\uAAB3", "\uAAB7",
	"\uAAB8", "\uAABE", "\uAABF", "\uAAC1", "\uFE20", "\uFE21", "\uFE22", "\uFE23",
	"\uFE24", "\uFE25", "\uFE26",
}

// kittyBase is the Unicode Private Use Area character used as the base
// for Kitty's Unicode placeholder protocol.
const kittyBase = "\U0010EEEE"

const (
	kittyEsc = "\x1b"
	kittySt  = "\x1b\\"
)

// kittyRGB holds the RGB color used to identify a transmitted image.
type kittyRGB struct {
	r, g, b byte
}

// KittyRenderResult holds the output of rendering an image with the Kitty
// graphics protocol. TransmitSeq is sent via tea.Raw() to transmit the image
// to terminal memory. Placeholder is text with Unicode placeholders that
// tells the terminal where to display the stored image. ImageID is used
// to delete the image from terminal memory when no longer needed.
type KittyRenderResult struct {
	TransmitSeq string
	Placeholder string
	ImageID     uint32
}

// generateKittyID creates a random image ID, an RGB color for identification,
// and a mask index for the combining marks.
func generateKittyID() (uint32, kittyRGB, byte) {
	id := rand.Uint32()
	maskIndex := byte(id >> 24)
	r := byte(id >> 16)
	g := byte(id >> 8)
	b := byte(id)
	return id, kittyRGB{r: r, g: g, b: b}, maskIndex
}

// buildKittyTransmitSeq builds the APC escape sequence to transmit an image
// to the terminal's memory using Kitty's graphics protocol (a=T, transmit-only).
// Uses PNG encoding (f=100) for much smaller payloads than raw pixel data.
// The image data is chunked at 4096 bytes per the Kitty protocol spec.
func buildKittyTransmitSeq(img image.Image, cols, rows int) (string, uint32, kittyRGB, byte) {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()

	// Encode as PNG for compact transmission
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		// Fallback to raw RGBA if PNG encoding fails
		return buildKittyTransmitSeqRaw(img, cols, rows)
	}
	encData := base64.StdEncoding.EncodeToString(buf.Bytes())

	id, rgb, maskIndex := generateKittyID()

	chunkSize := 4096
	dataLen := len(encData)

	// First chunk includes full header: f=100 (PNG), a=T (transmit-only), U=1 (Unicode placeholders)
	m := 1
	firstChunkSize := chunkSize
	if dataLen <= firstChunkSize {
		m = 0
		firstChunkSize = dataLen
	}

	firstChunk := encData[:firstChunkSize]
	seq := fmt.Sprintf("%s_Gf=100,a=T,c=%d,r=%d,s=%d,v=%d,m=%d,U=1,i=%d,q=2;%s%s",
		kittyEsc, cols, rows, w, h, m, id, firstChunk, kittySt)

	// Remaining chunks
	for i := firstChunkSize; i < dataLen; {
		end := i + chunkSize
		m = 1
		if end >= dataLen {
			end = dataLen
			m = 0
		}
		chunk := fmt.Sprintf("%s_Gm=%d;%s%s", kittyEsc, m, encData[i:end], kittySt)
		seq += chunk
		i = end
	}

	return seq, id, rgb, maskIndex
}

// buildKittyTransmitSeqRaw is a fallback that transmits raw RGBA pixel data.
func buildKittyTransmitSeqRaw(img image.Image, cols, rows int) (string, uint32, kittyRGB, byte) {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	pixels := make([]byte, w*h*4)
	pix := 0
	for y := range h {
		for x := range w {
			r, g, b, a := img.At(x, y).RGBA()
			pixels[pix] = byte(r >> 8)
			pixels[pix+1] = byte(g >> 8)
			pixels[pix+2] = byte(b >> 8)
			pixels[pix+3] = byte(a >> 8)
			pix += 4
		}
	}
	encData := base64.StdEncoding.EncodeToString(pixels)
	id, rgb, maskIndex := generateKittyID()

	chunkSize := 4096
	dataLen := len(encData)
	m := 1
	firstChunkSize := chunkSize
	if dataLen <= firstChunkSize {
		m = 0
		firstChunkSize = dataLen
	}
	firstChunk := encData[:firstChunkSize]
	seq := fmt.Sprintf("%s_Gf=32,a=T,c=%d,r=%d,s=%d,v=%d,m=%d,U=1,i=%d,q=2;%s%s",
		kittyEsc, cols, rows, w, h, m, id, firstChunk, kittySt)
	for i := firstChunkSize; i < dataLen; {
		end := i + chunkSize
		m = 1
		if end >= dataLen {
			end = dataLen
			m = 0
		}
		chunk := fmt.Sprintf("%s_Gm=%d;%s%s", kittyEsc, m, encData[i:end], kittySt)
		seq += chunk
		i = end
	}
	return seq, id, rgb, maskIndex
}

// buildKittyPlaceholder builds the Unicode placeholder text that tells the
// terminal to display a previously transmitted image at the current position.
// Each cell uses the Kitty base character (\U0010EEEE) with combining marks
// for row, column, and mask encoding, plus an SGR color for identification.
func buildKittyPlaceholder(cols, rows int, _ uint32, rgb kittyRGB, maskIndex byte) string {
	var sb strings.Builder
	for row := range rows {
		sb.WriteString(fmt.Sprintf("\x1b[38;2;%d;%d;%dm", rgb.r, rgb.g, rgb.b))
		for col := range cols {
			sb.WriteString(kittyBase)
			sb.WriteString(combiningMarks[row])
			sb.WriteString(combiningMarks[col])
			sb.WriteString(combiningMarks[maskIndex])
		}
		if row < rows-1 {
			sb.WriteString("\x1b[39m\n")
		} else {
			sb.WriteString("\x1b[39m")
		}
	}
	return sb.String()
}

// RenderKittyCover renders an image using the Kitty graphics protocol with
// Unicode placeholders. Returns the APC transmit sequence (for tea.Raw) and
// the placeholder text (for view.Content).
func RenderKittyCover(img image.Image, cols, rows int) KittyRenderResult {
	seq, id, rgb, maskIndex := buildKittyTransmitSeq(img, cols, rows)
	placeholder := buildKittyPlaceholder(cols, rows, id, rgb, maskIndex)
	return KittyRenderResult{
		TransmitSeq: seq,
		Placeholder: placeholder,
		ImageID:     id,
	}
}

// DeleteKittyImageCmd returns a tea.Cmd that deletes a Kitty graphics image
// from terminal memory. Should be called when leaving the detail view or
// switching to a different image.
func DeleteKittyImageCmd(id uint32) tea.Cmd {
	if id == 0 {
		return nil
	}
	seq := fmt.Sprintf("%s_Ga=d,d=a,i=%d%s", kittyEsc, id, kittySt)
	return tea.Raw(seq)
}
