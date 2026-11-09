package secrets

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	writeSecret(t, dir, ClientIDFile, "client-id\n")
	writeSecret(t, dir, ClientSecretFile, "client-secret\r\n")
	writeSecret(t, dir, MasterPasswordFile, "master-password")

	got, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.ClientID, []byte("client-id")) {
		t.Fatalf("client id = %q", got.ClientID)
	}
	if !bytes.Equal(got.ClientSecret, []byte("client-secret")) {
		t.Fatalf("client secret = %q", got.ClientSecret)
	}
	if !bytes.Equal(got.MasterPassword, []byte("master-password")) {
		t.Fatalf("master password = %q", got.MasterPassword)
	}
	if err := got.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestLoadTrimsOnlyOneNewline(t *testing.T) {
	dir := t.TempDir()
	writeSecret(t, dir, ClientIDFile, "id\n\n")
	writeSecret(t, dir, ClientSecretFile, "secret\r\n\r\n")
	writeSecret(t, dir, MasterPasswordFile, "pass\n")

	got, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.ClientID, []byte("id\n")) {
		t.Fatalf("client id = %q", got.ClientID)
	}
	if !bytes.Equal(got.ClientSecret, []byte("secret\r\n")) {
		t.Fatalf("client secret = %q", got.ClientSecret)
	}
}

func TestLoadMissingFile(t *testing.T) {
	dir := t.TempDir()
	writeSecret(t, dir, ClientIDFile, "id\n")
	writeSecret(t, dir, ClientSecretFile, "secret\n")

	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error")
	}
	if !bytes.Contains([]byte(err.Error()), []byte(MasterPasswordFile)) {
		t.Fatalf("error = %v, want file name", err)
	}
}

func TestLoadEmptyFile(t *testing.T) {
	dir := t.TempDir()
	writeSecret(t, dir, ClientIDFile, "id")
	writeSecret(t, dir, ClientSecretFile, "\n")
	writeSecret(t, dir, MasterPasswordFile, "pass")

	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("empty")) {
		t.Fatalf("error = %v", err)
	}
}

func TestLoadDoesNotExportEnv(t *testing.T) {
	t.Setenv("BW_CLIENTSECRET", "")
	t.Setenv("BW_PASSWORD", "")
	os.Unsetenv("BW_CLIENTSECRET")
	os.Unsetenv("BW_PASSWORD")

	dir := t.TempDir()
	writeSecret(t, dir, ClientIDFile, "id")
	writeSecret(t, dir, ClientSecretFile, "secret")
	writeSecret(t, dir, MasterPasswordFile, "pass")
	if _, err := Load(dir); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("BW_CLIENTSECRET") != "" || os.Getenv("BW_PASSWORD") != "" {
		t.Fatal("Load exported credential environment variables")
	}
}

func TestValidate(t *testing.T) {
	if err := (Secrets{}).Validate(); err == nil {
		t.Fatal("expected error")
	}
}

func writeSecret(t *testing.T, dir, name, value string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(value), 0o600); err != nil {
		t.Fatal(err)
	}
}
