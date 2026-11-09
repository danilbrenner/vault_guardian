package bw

import (
	"encoding/json"
	"errors"
	"fmt"
)

const (
	StatusUnauthenticated = "unauthenticated"
	StatusLocked          = "locked"
	StatusUnlocked        = "unlocked"
)

// Status is the JSON object returned by bw status.
// UserID is empty when the CLI is unauthenticated.
type Status struct {
	ServerURL string
	LastSync  string
	UserEmail string
	UserID    string
	Status    string
}

// UnmarshalJSON accepts null userId, userEmail, and lastSync.
func (s *Status) UnmarshalJSON(data []byte) error {
	var raw struct {
		ServerURL string  `json:"serverUrl"`
		LastSync  *string `json:"lastSync"`
		UserEmail *string `json:"userEmail"`
		UserID    *string `json:"userId"`
		Status    string  `json:"status"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	s.ServerURL = raw.ServerURL
	s.Status = raw.Status
	if raw.LastSync != nil {
		s.LastSync = *raw.LastSync
	}
	if raw.UserEmail != nil {
		s.UserEmail = *raw.UserEmail
	}
	if raw.UserID != nil {
		s.UserID = *raw.UserID
	}
	return nil
}

// Vault is the model CLI.Read returns while the CLI is open.
// It is the only vault model. Callers propagate it; they do not decode bw JSON again.
// Logins still hold passwords. Zero them when the passwords have been used.
// The session token is not part of this model.
type Vault struct {
	UserID string
	Logins []Login
}

// Login is one vault login that still holds its password.
// It is not safe to log, encode, or return from an MCP tool.
// Call Zero after the password has been used.
type Login struct {
	ID       string
	Name     string
	Username string
	Folder   *string
	AgeDays  int
	password []byte
}

// Item is a login that is safe to leave the MCP process.
// It has no password, username, TOTP, notes, URIs, custom fields,
// attachments, or password history. Metric fields are filled later.
type Item struct {
	ID      string  `json:"id"`
	Name    string  `json:"name"`
	Folder  *string `json:"folder"`
	AgeDays int     `json:"age_days"`
}

// Password returns the password bytes owned by l.
// Do not log or retain them. Zero clears this same buffer.
func (l Login) Password() []byte { return l.password }

// Zero clears the password bytes. It is safe to call more than once.
func (l *Login) Zero() {
	if l == nil {
		return
	}
	for i := range l.password {
		l.password[i] = 0
	}
	l.password = nil
}

// Public returns the decoded fields that do not include the password or username.
func (l Login) Public() Item {
	return Item{
		ID:      l.ID,
		Name:    l.Name,
		Folder:  cloneStringPtr(l.Folder),
		AgeDays: l.AgeDays,
	}
}

// Format keeps the password and username out of logs.
func (l Login) Format(f fmt.State, verb rune) {
	fmt.Fprintf(f, "login(%s)", l.ID)
}

// String keeps the password and username out of logs.
func (l Login) String() string {
	return "login(" + l.ID + ")"
}

// MarshalJSON refuses to encode a login. Use Public for the MCP type.
func (Login) MarshalJSON() ([]byte, error) {
	return nil, errors.New("bw: login cannot be encoded")
}

// Items returns the public view of logins. Passwords are not copied.
func Items(logins []Login) []Item {
	out := make([]Item, len(logins))
	for i := range logins {
		out[i] = logins[i].Public()
	}
	return out
}

func cloneStringPtr(p *string) *string {
	if p == nil {
		return nil
	}
	s := *p
	return &s
}

func zeroLogins(logins []Login) {
	for i := range logins {
		logins[i].Zero()
	}
}
