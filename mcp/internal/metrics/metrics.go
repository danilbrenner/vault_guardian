// Package metrics defines the public password-metric interfaces.
package metrics

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
	return Classification{}
}

// Strength returns the categorical strength for a password.
func Strength(password []byte) StrengthLevel {
	return ""
}

// Entropy returns the Shannon entropy of a password in bits.
func Entropy(password []byte) float64 {
	return 0
}

// MaskUsername returns a privacy-preserving username representation.
func MaskUsername(username string) string {
	return ""
}

// DuplicateHash returns the account-scoped password hash.
func DuplicateHash(password []byte, userID string) string {
	return ""
}

// HIBPPrefix returns the HIBP range prefix and clears password before returning.
func HIBPPrefix(password []byte) string {
	return ""
}
