package metrics

import (
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"math"
	"strings"
	"testing"
)

func TestClassifyCharacterClasses(t *testing.T) {
	cases := []struct {
		password string
		want     Classification
	}{
		{
			password: "Ab3!",
			want: Classification{
				Length: 4, HasDigit: true, HasLower: true, HasUpper: true, HasSymbol: true,
			},
		},
		{
			password: "Ab3?",
			want: Classification{
				Length: 4, HasDigit: true, HasLower: true, HasUpper: true,
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.password, func(t *testing.T) {
			if got := Classify([]byte(tc.password)); got != tc.want {
				t.Errorf("Classify(%q) = %+v, want %+v", tc.password, got, tc.want)
			}
		})
	}
}

func TestStrengthTableBoundaries(t *testing.T) {
	cases := []struct {
		name   string
		prefix string
		fill   string
		length int
		want   StrengthLevel
	}{
		{name: "all classes: 16 is very strong", prefix: "aA1!", fill: "a", length: 16, want: StrengthVeryStrong},
		{name: "all classes: 15 is strong", prefix: "aA1!", fill: "a", length: 15, want: StrengthStrong},
		{name: "all classes: 13 is strong", prefix: "aA1!", fill: "a", length: 13, want: StrengthStrong},
		{name: "all classes: 12 is moderate", prefix: "aA1!", fill: "a", length: 12, want: StrengthModerate},
		{name: "all classes: 11 is moderate", prefix: "aA1!", fill: "a", length: 11, want: StrengthModerate},
		{name: "all classes: 10 is weak", prefix: "aA1!", fill: "a", length: 10, want: StrengthWeak},
		{name: "all classes: 7 is weak", prefix: "aA1!", fill: "a", length: 7, want: StrengthWeak},
		{name: "all classes: 6 is very weak", prefix: "aA1!", fill: "a", length: 6, want: StrengthVeryWeak},

		{name: "digit lower upper: 17 is very strong", prefix: "aA1", fill: "a", length: 17, want: StrengthVeryStrong},
		{name: "digit lower upper: 16 is strong", prefix: "aA1", fill: "a", length: 16, want: StrengthStrong},
		{name: "digit lower upper: 14 is strong", prefix: "aA1", fill: "a", length: 14, want: StrengthStrong},
		{name: "digit lower upper: 13 is moderate", prefix: "aA1", fill: "a", length: 13, want: StrengthModerate},
		{name: "digit lower upper: 11 is moderate", prefix: "aA1", fill: "a", length: 11, want: StrengthModerate},
		{name: "digit lower upper: 10 is weak", prefix: "aA1", fill: "a", length: 10, want: StrengthWeak},
		{name: "digit lower upper: 7 is weak", prefix: "aA1", fill: "a", length: 7, want: StrengthWeak},
		{name: "digit lower upper: 6 is very weak", prefix: "aA1", fill: "a", length: 6, want: StrengthVeryWeak},

		{name: "lower upper: 18 is very strong", prefix: "aA", fill: "a", length: 18, want: StrengthVeryStrong},
		{name: "lower upper: 17 is strong", prefix: "aA", fill: "a", length: 17, want: StrengthStrong},
		{name: "lower upper: 15 is strong", prefix: "aA", fill: "a", length: 15, want: StrengthStrong},
		{name: "lower upper: 14 is moderate", prefix: "aA", fill: "a", length: 14, want: StrengthModerate},
		{name: "lower upper: 12 is moderate", prefix: "aA", fill: "a", length: 12, want: StrengthModerate},
		{name: "lower upper: 11 is weak", prefix: "aA", fill: "a", length: 11, want: StrengthWeak},
		{name: "lower upper: 7 is weak", prefix: "aA", fill: "a", length: 7, want: StrengthWeak},
		{name: "lower upper: 6 is very weak", prefix: "aA", fill: "a", length: 6, want: StrengthVeryWeak},

		{name: "upper only: 18 is strong", prefix: "A", fill: "A", length: 18, want: StrengthStrong},
		{name: "upper only: 17 is moderate", prefix: "A", fill: "A", length: 17, want: StrengthModerate},
		{name: "upper only: 14 is moderate", prefix: "A", fill: "A", length: 14, want: StrengthModerate},
		{name: "upper only: 13 is weak", prefix: "A", fill: "A", length: 13, want: StrengthWeak},
		{name: "upper only: 9 is weak", prefix: "A", fill: "A", length: 9, want: StrengthWeak},
		{name: "upper only: 8 is very weak", prefix: "A", fill: "A", length: 8, want: StrengthVeryWeak},

		{name: "lower only: 18 is strong", prefix: "a", fill: "a", length: 18, want: StrengthStrong},
		{name: "lower only: 17 is moderate", prefix: "a", fill: "a", length: 17, want: StrengthModerate},
		{name: "lower only: 14 is moderate", prefix: "a", fill: "a", length: 14, want: StrengthModerate},
		{name: "lower only: 13 is weak", prefix: "a", fill: "a", length: 13, want: StrengthWeak},
		{name: "lower only: 9 is weak", prefix: "a", fill: "a", length: 9, want: StrengthWeak},
		{name: "lower only: 8 is very weak", prefix: "a", fill: "a", length: 8, want: StrengthVeryWeak},

		{name: "digit only: 12 is weak", prefix: "1", fill: "1", length: 12, want: StrengthWeak},
		{name: "digit only: 11 is very weak", prefix: "1", fill: "1", length: 11, want: StrengthVeryWeak},
		{name: "unclassified: very weak", prefix: "?", fill: "?", length: 20, want: StrengthVeryWeak},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			password := tc.prefix + strings.Repeat(tc.fill, tc.length-len(tc.prefix))
			if got := Strength([]byte(password)); got != tc.want {
				t.Errorf("Strength(%q) = %q, want %q", password, got, tc.want)
			}
		})
	}
}

func TestEntropyKnownInput(t *testing.T) {
	if got, want := Entropy([]byte("aabb")), 4.0; math.Abs(got-want) > 1e-9 {
		t.Errorf("Entropy(aabb) = %v, want %v bits", got, want)
	}
}

func TestMaskUsername(t *testing.T) {
	if got, want := MaskUsername("danil@gmail.com"), "d***l@gmail.com"; got != want {
		t.Errorf("MaskUsername() = %q, want %q", got, want)
	}
}

func TestDuplicateHashUsesSHA256OfPasswordAndUserID(t *testing.T) {
	password := []byte("correct horse")
	userID := "user-42"
	joined := append(append([]byte(nil), password...), []byte(userID)...)
	wantDigest := sha256.Sum256(joined)
	want := hex.EncodeToString(wantDigest[:])

	if got := DuplicateHash(password, userID); got != want {
		t.Errorf("DuplicateHash() = %q, want SHA-256(password || userID) %q", got, want)
	}
	passwordDigest := sha1.Sum(password)
	if got := DuplicateHash(password, userID); got == hex.EncodeToString(passwordDigest[:]) {
		t.Error("DuplicateHash() returned the HIBP SHA-1 hash")
	}
}

func TestHIBPPrefixAndWipesPassword(t *testing.T) {
	password := []byte("correct horse")
	digest := sha1.Sum(password)
	fullHash := hex.EncodeToString(digest[:])

	got := HIBPPrefix(password)
	if want := fullHash[:5]; got != want {
		t.Errorf("HIBPPrefix() = %q, want first five SHA-1 hex chars %q", got, want)
	}
	if len(got) == 40 || strings.Contains(got, fullHash) {
		t.Errorf("HIBPPrefix() returned the full SHA-1: %q", got)
	}
	for i, b := range password {
		if b != 0 {
			t.Errorf("password byte %d was not wiped: %x", i, b)
		}
	}
}
