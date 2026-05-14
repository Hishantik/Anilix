package Allanime

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// AllAnime's encryption key passphrase (same as ani-cli)
const keyPassphrase = "Xot36i3lK3:v1"

// AllAnimeKey is the AES-256 key derived from the passphrase (same as ani-cli)
// Generated from: printf 'Xot36i3lK3:v1' | openssl dgst -sha256 -binary | od -A n -t x1 | tr -d ' \n'
var AllAnimeKey string

func init() {
	AllAnimeKey = generateAllAnimeKey()
}

// generateAllAnimeKey generates the AES key from passphrase using SHA256
// This matches the ani-cli behavior exactly
func generateAllAnimeKey() string {
	hash := sha256.Sum256([]byte(keyPassphrase))
	return bytesToHex(hash[:])
}

// decodeToBeParsed decrypts AllAnime's encrypted "tobeparsed" payload
// This implements the same logic as the shell script's decode_tobeparsed() function
func decodeToBeParsed(encoded string) ([]SourceUrl, error) {
	// Step 1: Decode base64
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		// Try URL-safe base64
		data, err = base64.URLEncoding.DecodeString(encoded)
		if err != nil {
			return nil, err
		}
	}

	if len(data) < 29 {
		return nil, nil
	}

	// Step 2: Extract IV (12 bytes from offset 1)
	iv := data[1:13]

	// Step 3: Create CTR counter (IV + 00000002)
	ctr := make([]byte, 16)
	copy(ctr, iv)
	ctr[15] = 0x02

	// Step 4: Extract ciphertext (skip 13 bytes header, remove 16 bytes for tag)
	ciphertext := data[13 : len(data)-16]

	// Step 5: AES-256-CTR decrypt
	plaintext, err := decryptAESCTR(AllAnimeKey, ctr, ciphertext)
	if err != nil {
		return nil, err
	}

	// Step 6: Parse JSON - extract sourceUrl and sourceName
	return parseSourceUrls(string(plaintext))
}

// parseSourceUrls extracts sourceUrl and sourceName from decrypted JSON
// Format: {"sourceUrl":"url","sourceName":"name",...}
func parseSourceUrls(jsonStr string) ([]SourceUrl, error) {
	var results []SourceUrl

	// Use regex to find all sourceUrl and sourceName pairs
	// Handle both "sourceUrl" and just "url" in JSON
	urlRe := regexp.MustCompile(`"sourceUrl"\s*:\s*"([^"]*)"`)
	nameRe := regexp.MustCompile(`"sourceName"\s*:\s*"([^"]*)"`)

	urlMatches := urlRe.FindAllStringSubmatch(jsonStr, -1)
	nameMatches := nameRe.FindAllStringSubmatch(jsonStr, -1)

	for i, urlMatch := range urlMatches {
		if len(urlMatch) < 2 {
			continue
		}

		url := urlMatch[1]

		// Get corresponding sourceName if available
		var name string
		if i < len(nameMatches) && len(nameMatches[i]) >= 2 {
			name = nameMatches[i][1]
		}

		// Skip empty URLs
		if url == "" {
			continue
		}

		// Handle hex-encoded provider IDs (start with --)
		if strings.HasPrefix(url, "--") {
			decoded := decodeHexProviderID(url[2:])
			if decoded != "" {
				results = append(results, SourceUrl{
					SourceName: name,
					SourceUrl:  decoded,
				})
			}
		} else {
			results = append(results, SourceUrl{
				SourceName: name,
				SourceUrl:  url,
			})
		}
	}

	// If no matches from sourceUrl, try "url" field directly
	if len(results) == 0 {
		// Look for URL-like patterns
		directUrlRe := regexp.MustCompile(`"url"\s*:\s*"([^"]+\.(?:mp4|m3u8)[^"]*)"`)
		directMatches := directUrlRe.FindAllStringSubmatch(jsonStr, -1)
		for _, match := range directMatches {
			if len(match) >= 2 && match[1] != "" {
				results = append(results, SourceUrl{
					SourceName: "default",
					SourceUrl:  match[1],
				})
			}
		}
	}

	return results, nil
}

// b64urlToHex converts URL-safe base64 to hex bytes
// This implements the same logic as the shell script's b64url_to_hex() function
func b64urlToHex(b64url string) string {
	// Step 1: Calculate padding
	mod := len(b64url) % 4
	padding := ""
	switch mod {
	case 2:
		padding = "=="
	case 3:
		padding = "="
	}

	// Step 2: Convert URL-safe base64 to standard base64 (-_ -> +/)
	standard := strings.ReplaceAll(strings.ReplaceAll(b64url, "-", "+"), "_", "/")

	// Step 3: Decode and convert to hex
	decoded, err := base64.StdEncoding.DecodeString(standard + padding)
	if err != nil {
		return ""
	}

	return bytesToHex(decoded)
}

// bytesToHex converts byte slice to hex string
func bytesToHex(data []byte) string {
	hexChars := "0123456789abcdef"
	result := make([]byte, len(data)*2)
	for i, b := range data {
		result[i*2] = hexChars[b>>4]
		result[i*2+1] = hexChars[b&0x0f]
	}
	return string(result)
}

// hexToBytes converts hex string to byte slice
func hexToBytes(hex string) ([]byte, error) {
	if len(hex)%2 != 0 {
		return nil, nil
	}

	result := make([]byte, len(hex)/2)
	for i := 0; i < len(hex); i += 2 {
		var b byte
		_, err := fmt.Sscanf(hex[i:i+2], "%02x", &b)
		if err != nil {
			return nil, err
		}
		result[i/2] = b
	}
	return result, nil
}

// decryptAESCTR decrypts data using AES-256-CTR mode
// key is expected as hex string, iv as byte slice
func decryptAESCTR(keyHex string, iv []byte, ciphertext []byte) ([]byte, error) {
	key, err := hexToBytes(keyHex)
	if err != nil {
		return nil, fmt.Errorf("failed to decode key: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	stream := cipher.NewCTR(block, iv)
	plaintext := make([]byte, len(ciphertext))
	stream.XORKeyStream(plaintext, ciphertext)

	return plaintext, nil
}

// ProviderConfig matches the ani-cli generate_link case statement
type ProviderConfig struct {
	ID       int    // 1=wixmp, 2=youtube, 3=sharepoint, 5=filemoon, 0=default(hianime)
	Name     string
	Pattern  string // sed pattern to match line
	Type     string // "m3u8" or "mp4"
}

// ProviderConfigs matches the order in ani-cli generate_link:
// 1) wixmp, 2) youtube, 3) sharepoint, 5) filemoon, default) hianime
var ProviderConfigs = []ProviderConfig{
	{ID: 1, Name: "wixmp", Pattern: "Default :", Type: "m3u8"},
	{ID: 2, Name: "youtube", Pattern: "Yt-mp4 :", Type: "mp4"},
	{ID: 3, Name: "sharepoint", Pattern: "S-mp4 :", Type: "mp4"},
	{ID: 5, Name: "filemoon", Pattern: "Fm-mp4 :", Type: "m3u8"},
	{ID: 0, Name: "hianime", Pattern: "Luf-Mp4 :", Type: "m3u8"}, // default
}

// isHexEncoded checks if a string contains only hex characters (0-9, a-f)
func isHexEncoded(s string) bool {
	if len(s) == 0 || len(s)%2 != 0 {
		return false
	}
	s = strings.ToLower(s)
	for _, r := range s {
		if !unicode.IsDigit(r) && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

// generateLink simulates the ani-cli generate_link function
// It extracts provider ID from response based on priority order
func generateLink(response string, providerID int) (string, *ProviderConfig) {
	// First try the specific provider ID if provided
	if providerID > 0 {
		for _, cfg := range ProviderConfigs {
			if cfg.ID == providerID {
				providerIDStr := extractProviderByPattern(response, cfg.Pattern)
				if providerIDStr != "" {
					decoded := decodeHexProviderID(providerIDStr)
					decoded = fixClockPath(decoded)
					return decoded, &cfg
				}
			}
		}
	}

	// Otherwise iterate through providers in priority order
	for _, cfg := range ProviderConfigs {
		providerIDStr := extractProviderByPattern(response, cfg.Pattern)
		if providerIDStr != "" {
			decoded := decodeHexProviderID(providerIDStr)
			decoded = fixClockPath(decoded)
			return decoded, &cfg
		}
	}

	return "", nil
}

// extractProviderByPattern extracts the provider ID from response using the sed pattern
// Similar to: sed -nE 's|.*pattern(.*)$|\1|p'
func extractProviderByPattern(response, pattern string) string {
	// Convert pattern to regex - pattern ends with " :" so we look for that
	// The pattern format is "Name :" and we extract what comes after
	pattern = strings.TrimSuffix(pattern, " :")
	re := regexp.MustCompile(`(?i)` + pattern + `.*?:\s*([0-9a-fA-F]+)`)
	matches := re.FindStringSubmatch(response)
	if len(matches) > 1 {
		return matches[1]
	}

	// Fallback: look for any hex string after the pattern
	re = regexp.MustCompile(`(?i)` + pattern + `.*?:\s*([0-9a-f]+)`)
	matches = re.FindStringSubmatch(response)
	if len(matches) > 1 {
		return matches[1]
	}

	return ""
}

// fixClockPath applies the sed fix: s/\/clock/\/clock\.json/
// This is the final transform in provider_init
func fixClockPath(path string) string {
	return strings.ReplaceAll(path, "/clock", "/clock.json")
}

// decodeHexProviderID decodes AllAnime's custom hex-encoded provider IDs
// This implements the same encoding as the shell script's provider_init() function
func decodeHexProviderID(encoded string) string {
	encoded = strings.TrimSpace(encoded)
	if len(encoded)%2 != 0 {
		return encoded
	}

	var result strings.Builder
	result.Grow(len(encoded) / 2)

	for i := 0; i < len(encoded); i += 2 {
		hexPair := encoded[i : i+2]
		result.WriteByte(decodeHexPair(hexPair))
	}

	return result.String()
}

// decodeHexPair converts a two-character hex string to its corresponding ASCII character
// using AllAnime's custom encoding
func decodeHexPair(hex string) byte {
	switch strings.ToLower(hex) {
	// Uppercase letters
	case "79": return 'A'
	case "7a": return 'B'
	case "7b": return 'C'
	case "7c": return 'D'
	case "7d": return 'E'
	case "7e": return 'F'
	case "7f": return 'G'
	case "70": return 'H'
	case "71": return 'I'
	case "72": return 'J'
	case "73": return 'K'
	case "74": return 'L'
	case "75": return 'M'
	case "76": return 'N'
	case "77": return 'O'
	case "68": return 'P'
	case "69": return 'Q'
	case "6a": return 'R'
	case "6b": return 'S'
	case "6c": return 'T'
	case "6d": return 'U'
	case "6e": return 'V'
	case "6f": return 'W'
	case "60": return 'X'
	case "61": return 'Y'
	case "62": return 'Z'
	// Lowercase letters
	case "59": return 'a'
	case "5a": return 'b'
	case "5b": return 'c'
	case "5c": return 'd'
	case "5d": return 'e'
	case "5e": return 'f'
	case "5f": return 'g'
	case "50": return 'h'
	case "51": return 'i'
	case "52": return 'j'
	case "53": return 'k'
	case "54": return 'l'
	case "55": return 'm'
	case "56": return 'n'
	case "57": return 'o'
	case "48": return 'p'
	case "49": return 'q'
	case "4a": return 'r'
	case "4b": return 's'
	case "4c": return 't'
	case "4d": return 'u'
	case "4e": return 'v'
	case "4f": return 'w'
	case "40": return 'x'
	case "41": return 'y'
	case "42": return 'z'
	// Digits
	case "08": return '0'
	case "09": return '1'
	case "0a": return '2'
	case "0b": return '3'
	case "0c": return '4'
	case "0d": return '5'
	case "0e": return '6'
	case "0f": return '7'
	case "00": return '8'
	case "01": return '9'
	// Special characters
	case "15": return '-'
	case "16": return '.'
	case "67": return '_'
	case "46": return '~'
	case "02": return ':'
	case "17": return '/'
	case "07": return '?'
	case "1b": return '#'
	case "63": return '['
	case "65": return ']'
	case "78": return '@'
	case "19": return '!'
	case "1c": return '$'
	case "1e": return '&'
	case "10": return '('
	case "11": return ')'
	case "12": return '*'
	case "13": return '+'
	case "14": return ','
	case "03": return ';'
	case "05": return '='
	case "1d": return '%'
	default:
		// Return first char if not recognized, or '?'
		if len(hex) >= 1 {
			return '?'
		}
		return '?'
	}
}