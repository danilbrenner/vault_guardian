package bw

import (
	"encoding/json"

	"bw-mcp/internal/metrics"
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

// Login is a login projection that is safe to leave the CLI layer.
// It has no password, raw username, TOTP, notes, URIs, custom fields,
// attachments, or password history.
type Login struct {
	ID             string                `json:"id"`
	Name           string                `json:"name"`
	MaskedUsername string                `json:"masked_username"`
	Folder         *string               `json:"folder"`
	AgeDays        int                   `json:"age_days"`
	Length         int                   `json:"length"`
	Entropy        float64               `json:"entropy"`
	Strength       metrics.StrengthLevel `json:"strength"`
	Hash           string                `json:"hash"`
}
