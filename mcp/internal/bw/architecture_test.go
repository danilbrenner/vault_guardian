package bw

import (
	"reflect"
	"strings"
	"testing"
)

func TestItemDoesNotExposeCredentialFields(t *testing.T) {
	typ := reflect.TypeOf(Item{})
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.PkgPath != "" { // Ignore unexported implementation details.
			continue
		}
		if isSensitivePublicField(field.Name) {
			t.Errorf("public Item has sensitive field %q", field.Name)
		}

		jsonName := strings.Split(field.Tag.Get("json"), ",")[0]
		if jsonName == "-" {
			continue
		}
		if jsonName == "" {
			jsonName = field.Name
		}
		if isSensitivePublicField(jsonName) {
			t.Errorf("public Item exposes sensitive JSON field %q", jsonName)
		}
	}
}

func isSensitivePublicField(name string) bool {
	normalized := strings.ReplaceAll(strings.ToLower(name), "_", "")
	if strings.Contains(normalized, "password") || strings.Contains(normalized, "totp") || strings.Contains(normalized, "sha1") || strings.Contains(normalized, "hibp") {
		return true
	}
	return strings.Contains(normalized, "username") && normalized != "maskedusername"
}
