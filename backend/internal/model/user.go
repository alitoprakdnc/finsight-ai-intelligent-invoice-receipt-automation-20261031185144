package model

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// Role represents a user's permission level within the system.
type Role string

const (
	// RoleAdmin has full access to all resources and can manage other users.
	RoleAdmin Role = "admin"
	// RoleUser is a regular user with access limited to their own data.
	RoleUser Role = "user"
)

// Preferences stores UI‑related user settings.
type Preferences struct {
	Theme    string `json:"theme"`    // e.g. "light" or "dark"
	Language string `json:"language"` // ISO 639‑1 code, e.g. "en", "es"
}

// MarshalJSON ensures Preferences are stored as JSONB in PostgreSQL.
func (p Preferences) MarshalJSON() ([]byte, error) {
	type alias Preferences
	return json.Marshal(alias(p))
}

// UnmarshalJSON parses the JSONB column back into Preferences.
func (p *Preferences) UnmarshalJSON(data []byte) error {
	type alias Preferences
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	*p = Preferences(a)
	return nil
}

// User represents an application user.
// It is used both by the data layer (SQL) and the API layer (JSON).
type User struct {
	ID           uuid.UUID   `db:"id" json:"id"`                         // primary key
	Email        string      `db:"email" json:"email"`                   // unique, validated
	PasswordHash string      `db:"password_hash" json:"-"`               // never exposed via JSON
	Role         Role        `db:"role" json:"role"`                     // admin or user
	Preferences  Preferences `db:"preferences" json:"preferences"`      // stored as JSONB
	CreatedAt    time.Time   `db:"created_at" json:"created_at"`         // UTC timestamps
	UpdatedAt    time.Time   `db:"updated_at" json:"updated_at"`         // UTC timestamps
}

// TableName overrides the default table name for GORM/SQLX usage.
func (User) TableName() string {
	return "users"
}

// NewUser creates a new User instance with a generated UUID and timestamps.
// The password is hashed using SetPassword before persisting.
func NewUser(email, plainPassword string, role Role, prefs Preferences) (*User, error) {
	if email == "" {
		return nil, errors.New("email must not be empty")
	}
	if plainPassword == "" {
		return nil, errors.New("password must not be empty")
	}
	if role != RoleAdmin && role != RoleUser {
		return nil, errors.New("invalid role")
	}
	u := &User{
		ID:          uuid.New(),
		Email:       email,
		Role:        role,
		Preferences: prefs,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	if err := u.SetPassword(plainPassword); err != nil {
		return nil, err
	}
	return u, nil
}

// SetPassword hashes the supplied plain‑text password and stores it in PasswordHash.
// It uses bcrypt with the default cost. Returns an error if hashing fails.
func (u *User) SetPassword(plainPassword string) error {
	if plainPassword == "" {
		return errors.New("password cannot be empty")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(plainPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.PasswordHash = string(hash)
	return nil
}

// CheckPassword compares the supplied plain‑text password with the stored hash.
// Returns true if they match.
func (u *User) CheckPassword(plainPassword string) bool {
	if u.PasswordHash == "" {
		return false
	}
	err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(plainPassword))
	return err == nil
}

// UpdateTimestamps refreshes the UpdatedAt field. Call this before persisting updates.
func (u *User) UpdateTimestamps() {
	u.UpdatedAt = time.Now().UTC()
}