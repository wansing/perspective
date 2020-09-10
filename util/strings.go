package util

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"
)

const CutMoreStr = "<!-- more -->"

func CutMore(s string) (string, bool) {
	if i := strings.Index(s, CutMoreStr); i >= 0 {
		return s[:i], true
	}
	return s, false
}

// IsFirstUpper returns true if the first utf8 rune is uppercase.
func IsFirstUpper(s string) bool {
	firstRune, _ := utf8.DecodeRuneInString(s)
	return unicode.IsUpper(firstRune)
}

// RandomString32 returns a 32 bytes long string with 24 bytes (192 bits) of entropy.
func RandomString32() (string, error) {

	b := make([]byte, 24)

	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}

	result := base64.URLEncoding.EncodeToString(b)

	if len(result) < 32 {
		return "", errors.New("RandomString64 too short")
	}

	if len(result) > 32 {
		result = result[:32]
	}

	return result, nil
}

// Trunc truncates the input string to a specific length.
// It is UTF8-safe, but does not care for HTML.
func Trunc(s string, maxRunes int) string {
	s = strings.TrimSpace(s)
	var runes = 0
	for i := range s {
		runes++
		if runes == maxRunes {
			return strings.TrimSpace(s[:i]) // trim spaces again
		}
	}
	return s
}
