// Package secrets loads Bitwarden credentials from Docker secret files.
// It does not read or export credential environment variables in the server process.
package secrets

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const (
	ClientIDFile       = "bw_client_id"
	ClientSecretFile   = "bw_client_secret"
	MasterPasswordFile = "bw_master_password"
)

// Source loads one credential at a time. Each method returns a new byte slice
// owned by the caller, which must wipe it when it is no longer needed.
type Source interface {
	LoadClientID() ([]byte, error)
	LoadClientSecret() ([]byte, error)
	LoadMasterPassword() ([]byte, error)
}

// FileSource implements Source by reading credential files on demand.
// It holds only a directory path and does not retain credential bytes.
type FileSource struct {
	dir string
}

var _ Source = FileSource{}

// NewFileSource returns a file-backed source rooted at dir.
func NewFileSource(dir string) FileSource {
	return FileSource{dir: dir}
}

// LoadClientID reads the client ID secret file.
func (f FileSource) LoadClientID() ([]byte, error) {
	return f.readSecret(ClientIDFile)
}

// LoadClientSecret reads the client secret file.
func (f FileSource) LoadClientSecret() ([]byte, error) {
	return f.readSecret(ClientSecretFile)
}

// LoadMasterPassword reads the master password secret file.
func (f FileSource) LoadMasterPassword() ([]byte, error) {
	return f.readSecret(MasterPasswordFile)
}

func (f FileSource) readSecret(name string) ([]byte, error) {
	if f.dir == "" {
		return nil, errors.New("secrets: directory is required")
	}
	raw, err := os.ReadFile(filepath.Join(f.dir, name))
	if err != nil {
		return nil, fmt.Errorf("secrets: read %s: %w", name, err)
	}
	trimmed := trimOneNewline(raw)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("secrets: %s is empty", name)
	}
	return trimmed, nil
}

// trimOneNewline removes one trailing newline, including a preceding CR.
func trimOneNewline(b []byte) []byte {
	if bytes.HasSuffix(b, []byte("\r\n")) {
		return b[:len(b)-2]
	}
	if len(b) > 0 && b[len(b)-1] == '\n' {
		return b[:len(b)-1]
	}
	return b
}
