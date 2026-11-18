// Package metrics defines the public password-metric interfaces.
package metrics

import (
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"math"
	"strings"
	"unicode"
	"unicode/utf8"
)

// StrengthLevel is the categorical strength reported for an item.
type StrengthLevel string

const (
	StrengthVeryWeak   StrengthLevel = "VeryWeak"
	StrengthWeak       StrengthLevel = "Weak"
	StrengthModerate   StrengthLevel = "Moderate"
	StrengthStrong     StrengthLevel = "Strong"
	StrengthVeryStrong StrengthLevel = "VeryStrong"
)

// Classification contains the password length and character classes.
type Classification struct {
	Length    int
	HasDigit  bool
	HasLower  bool
	HasUpper  bool
	HasSymbol bool
}

// Classify returns the password length and character-class flags.
func Classify(password []byte) Classification {
	classification := Classification{Length: utf8.RuneCount(password)}
	for i := 0; i < len(password); {
		r, size := utf8.DecodeRune(password[i:])
		i += size
		switch {
		case unicode.IsDigit(r):
			classification.HasDigit = true
		case unicode.IsLower(r):
			classification.HasLower = true
		case unicode.IsUpper(r):
			classification.HasUpper = true
		}
		switch r {
		case '!', '@', '#', '$', '%', '^', '&', '*':
			classification.HasSymbol = true
		}
	}
	return classification
}

// Strength returns the categorical strength for a password.
func Strength(password []byte) StrengthLevel {
	c := Classify(password)
	length := c.Length

	switch {
	case c.HasDigit && c.HasLower && c.HasUpper && c.HasSymbol:
		if length > 15 {
			return StrengthVeryStrong
		}
		if length > 12 {
			return StrengthStrong
		}
		if length > 10 {
			return StrengthModerate
		}
		if length > 6 {
			return StrengthWeak
		}
	case c.HasDigit && c.HasLower && c.HasUpper:
		if length > 16 {
			return StrengthVeryStrong
		}
		if length > 13 {
			return StrengthStrong
		}
		if length > 10 {
			return StrengthModerate
		}
		if length > 6 {
			return StrengthWeak
		}
	case c.HasLower && c.HasUpper && !c.HasDigit && !c.HasSymbol:
		if length > 17 {
			return StrengthVeryStrong
		}
		if length > 14 {
			return StrengthStrong
		}
		if length > 11 {
			return StrengthModerate
		}
		if length > 6 {
			return StrengthWeak
		}
	case c.HasUpper && !c.HasLower && !c.HasDigit && !c.HasSymbol:
		if length > 17 {
			return StrengthStrong
		}
		if length > 13 {
			return StrengthModerate
		}
		if length > 8 {
			return StrengthWeak
		}
	case c.HasLower && !c.HasUpper && !c.HasDigit && !c.HasSymbol:
		if length > 17 {
			return StrengthStrong
		}
		if length > 13 {
			return StrengthModerate
		}
		if length > 8 {
			return StrengthWeak
		}
	case c.HasDigit && !c.HasLower && !c.HasUpper && !c.HasSymbol:
		if length > 11 {
			return StrengthWeak
		}
	}
	return StrengthVeryWeak
}

// Entropy returns the Shannon entropy of a password in bits.
func Entropy(password []byte) float64 {
	counts := make(map[rune]int)
	length := 0
	for i := 0; i < len(password); {
		r, size := utf8.DecodeRune(password[i:])
		i += size
		counts[r]++
		length++
	}
	if length == 0 {
		return 0
	}

	perCharacter := 0.0
	for _, count := range counts {
		p := float64(count) / float64(length)
		perCharacter -= p * math.Log2(p)
	}
	return perCharacter * float64(length)
}

// MaskUsername returns a privacy-preserving username representation.
func MaskUsername(username string) string {
	if username == "" {
		return ""
	}
	local, domain := username, ""
	if at := strings.IndexByte(username, '@'); at >= 0 {
		local, domain = username[:at], username[at:]
	}
	characters := []rune(local)
	if len(characters) == 0 {
		return "***" + domain
	}
	if len(characters) == 1 {
		return string(characters[0]) + "***" + domain
	}
	maskedCharacters := len(characters) - 2
	if maskedCharacters < 3 {
		maskedCharacters = 3
	}
	return string(characters[0]) + strings.Repeat("*", maskedCharacters) + string(characters[len(characters)-1]) + domain
}

// DuplicateHash returns the account-scoped password hash.
func DuplicateHash(password []byte, userID string) string {
	h := sha256.New()
	_, _ = h.Write(password)
	_, _ = h.Write([]byte(userID))
	return hex.EncodeToString(h.Sum(nil))
}

// HIBPPrefix returns the HIBP range prefix and clears password before returning.
func HIBPPrefix(password []byte) string {
	defer wipe(password)
	digest := sha1.Sum(password)
	defer func() { clear(digest[:]) }()
	const hexDigits = "0123456789abcdef"
	prefix := []byte{
		hexDigits[digest[0]>>4],
		hexDigits[digest[0]&0x0f],
		hexDigits[digest[1]>>4],
		hexDigits[digest[1]&0x0f],
		hexDigits[digest[2]>>4],
	}
	return string(prefix)
}

func wipe(password []byte) {
	clear(password[:cap(password)])
}
