package secrets

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestFileSourceReadsOnlyRequestedSecret(t *testing.T) {
	dir := t.TempDir()
	writeSecret(t, dir, ClientIDFile, "client-id\n")
	source := NewFileSource(dir)

	got, err := source.LoadClientID()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, []byte("client-id")) {
		t.Fatalf("client id = %q", got)
	}

	// The other files are absent, but constructing the loader and reading the
	// client ID did not require them.
	if _, err := source.LoadClientSecret(); err == nil {
		t.Fatal("expected missing client secret error")
	}
	if _, err := source.LoadMasterPassword(); err == nil {
		t.Fatal("expected missing master password error")
	}
}

func TestFileSourceTrimsOnlyOneNewline(t *testing.T) {
	dir := t.TempDir()
	writeSecret(t, dir, ClientIDFile, "id\n\n")
	writeSecret(t, dir, ClientSecretFile, "secret\r\n\r\n")
	writeSecret(t, dir, MasterPasswordFile, "pass\n")
	source := NewFileSource(dir)

	got, err := source.LoadClientID()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, []byte("id\n")) {
		t.Fatalf("client id = %q", got)
	}
	got, err = source.LoadClientSecret()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, []byte("secret\r\n")) {
		t.Fatalf("client secret = %q", got)
	}
	got, err = source.LoadMasterPassword()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, []byte("pass")) {
		t.Fatalf("master password = %q", got)
	}
}

func TestFileSourceMissingFile(t *testing.T) {
	_, err := NewFileSource(t.TempDir()).LoadMasterPassword()
	if err == nil {
		t.Fatal("expected error")
	}
	if !bytes.Contains([]byte(err.Error()), []byte(MasterPasswordFile)) {
		t.Fatalf("error = %v, want file name", err)
	}
}

func TestFileSourceRequiresDirectory(t *testing.T) {
	_, err := NewFileSource("").LoadClientID()
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("directory is required")) {
		t.Fatalf("error = %v, want required directory", err)
	}
}

func TestFileSourceEmptyFile(t *testing.T) {
	dir := t.TempDir()
	writeSecret(t, dir, ClientSecretFile, "\n")
	_, err := NewFileSource(dir).LoadClientSecret()
	if err == nil {
		t.Fatal("expected error")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("empty")) {
		t.Fatalf("error = %v", err)
	}
}

func TestFileSourceDoesNotExportEnv(t *testing.T) {
	t.Setenv("BW_CLIENTSECRET", "")
	t.Setenv("BW_PASSWORD", "")
	os.Unsetenv("BW_CLIENTSECRET")
	os.Unsetenv("BW_PASSWORD")

	dir := t.TempDir()
	writeSecret(t, dir, ClientSecretFile, "secret")
	if _, err := NewFileSource(dir).LoadClientSecret(); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("BW_CLIENTSECRET") != "" || os.Getenv("BW_PASSWORD") != "" {
		t.Fatal("loader exported credential environment variables")
	}
}

func writeSecret(t *testing.T, dir, name, value string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(value), 0o600); err != nil {
		t.Fatal(err)
	}
}
