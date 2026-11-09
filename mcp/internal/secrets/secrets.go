// Package secrets loads Bitwarden credentials from Docker secret files.
// It does not read or export BW_CLIENTSECRET or BW_PASSWORD on the server process.
package secrets

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const (
	// DefaultDir is the Docker secrets mount point.
	DefaultDir = "/run/secrets"

	ClientIDFile       = "bw_client_id"
	ClientSecretFile   = "bw_client_secret"
	MasterPasswordFile = "bw_master_password"
)

// Secrets are credential bytes. They are not safe to log.
type Secrets struct {
	ClientID       []byte
	ClientSecret   []byte
	MasterPassword []byte
}

// Validate reports a missing credential without including its value.
func (s Secrets) Validate() error {
	switch {
	case len(s.ClientID) == 0:
		return errors.New("secrets: client id is empty")
	case len(s.ClientSecret) == 0:
		return errors.New("secrets: client secret is empty")
	case len(s.MasterPassword) == 0:
		return errors.New("secrets: master password is empty")
	default:
		return nil
	}
}

// Load reads the three Docker secret files from dir.
// An empty dir uses DefaultDir. A single trailing newline is trimmed.
// A missing or empty file is an error.
func Load(dir string) (Secrets, error) {
	if dir == "" {
		dir = DefaultDir
	}
	clientID, err := readSecret(dir, ClientIDFile)
	if err != nil {
		return Secrets{}, err
	}
	clientSecret, err := readSecret(dir, ClientSecretFile)
	if err != nil {
		return Secrets{}, err
	}
	masterPassword, err := readSecret(dir, MasterPasswordFile)
	if err != nil {
		return Secrets{}, err
	}
	return Secrets{
		ClientID:       clientID,
		ClientSecret:   clientSecret,
		MasterPassword: masterPassword,
	}, nil
}

func readSecret(dir, name string) ([]byte, error) {
	path := filepath.Join(dir, name)
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("secrets: read %s: %w", name, err)
	}
	b = trimOneNewline(b)
	if len(b) == 0 {
		return nil, fmt.Errorf("secrets: %s is empty", name)
	}
	return b, nil
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
