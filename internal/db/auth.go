package db

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// Dashboard login credentials live in the shared `kv` table so the canonical
// schema stays untouched (DESIGN.md §1 r2). Scope "auth" holds the password
// hash; scope "sessions" holds issued session tokens.
const (
	authScope         = "auth"
	passwordHashKey   = "passwordHash"
	mustChangePassKey = "mustChangePassword"
	sessionsScope     = "sessions"
	DefaultPassword   = "123456"
	sessionTTL        = 7 * 24 * time.Hour
	sessionTokenBytes = 32
	bcryptCost        = bcrypt.DefaultCost
	// PasswordMinLength is enforced server-side; the UI mirrors it so the
	// operator gets the error before a round-trip. Keep both in sync.
	PasswordMinLength  = 8
	passwordMaxLength  = 72 // bcrypt silently truncates beyond 72 bytes
	ErrPasswordTooWeak = "password minimal 8 karakter"
)

// createKVTable is the canonical `kv` definition. It matches the schema the
// dashboard migrations create, so running it against an existing database is a
// no-op and running it against a fresh one produces an identical table.
const createKVTable = `CREATE TABLE IF NOT EXISTS kv (
	scope TEXT NOT NULL,
	key TEXT NOT NULL,
	value TEXT NOT NULL,
	PRIMARY KEY (scope, key)
)`

// PasswordState reports what the login screen needs to know without ever
// leaking the hash itself.
type PasswordState struct {
	// MustChange is true while the account still carries the built-in default
	// password. The dashboard blocks navigation until it is changed.
	MustChange bool `json:"mustChange"`
}

// EnsureDefaultPassword seeds the built-in password on first boot.
//
// It is a no-op once a hash exists, so an operator who changed the password is
// never silently reset back to the default.
func (r *Repo) EnsureDefaultPassword() (seeded bool, err error) {
	_, found, err := r.GetKV(authScope, passwordHashKey)
	if err != nil {
		return false, err
	}
	if found {
		return false, nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(DefaultPassword), bcryptCost)
	if err != nil {
		return false, fmt.Errorf("hash default password: %w", err)
	}
	if err := r.SetKV(authScope, passwordHashKey, string(hash)); err != nil {
		return false, err
	}
	// Marked so the dashboard forces a change before anything else is usable.
	if err := r.SetKV(authScope, mustChangePassKey, "true"); err != nil {
		return false, err
	}
	return true, nil
}

// VerifyPassword checks a plaintext password against the stored hash.
//
// When no hash exists yet the caller is expected to have run
// EnsureDefaultPassword; this returns false rather than falling back to the
// default, so a missing row can never become an open door.
func (r *Repo) VerifyPassword(password string) (bool, error) {
	hash, found, err := r.GetKV(authScope, passwordHashKey)
	if err != nil {
		return false, err
	}
	if !found {
		return false, nil
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return false, nil
		}
		return false, fmt.Errorf("compare password: %w", err)
	}
	return true, nil
}

// SetPassword replaces the stored password and clears the must-change flag.
func (r *Repo) SetPassword(password string) error {
	if len(password) < PasswordMinLength {
		return errors.New(ErrPasswordTooWeak)
	}
	if len(password) > passwordMaxLength {
		return fmt.Errorf("password maksimal %d karakter", passwordMaxLength)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	if err := r.SetKV(authScope, passwordHashKey, string(hash)); err != nil {
		return err
	}
	return r.SetKV(authScope, mustChangePassKey, "false")
}

// PasswordState reports whether the built-in default password is still active.
func (r *Repo) PasswordState() (PasswordState, error) {
	raw, found, err := r.GetKV(authScope, mustChangePassKey)
	if err != nil {
		return PasswordState{}, err
	}
	if !found {
		// No marker yet: treat an un-seeded install as still on the default so
		// the operator is asked to set a real password rather than assumed safe.
		return PasswordState{MustChange: true}, nil
	}
	return PasswordState{MustChange: raw == "true"}, nil
}

// ---------- sessions ----------

// CreateSession issues a new session token valid for sessionTTL.
func (r *Repo) CreateSession() (token string, expiresAt time.Time, err error) {
	buf := make([]byte, sessionTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", time.Time{}, fmt.Errorf("generate session token: %w", err)
	}
	token = hex.EncodeToString(buf)
	expiresAt = time.Now().UTC().Add(sessionTTL)
	if err := r.SetKV(sessionsScope, token, expiresAt.Format(time.RFC3339)); err != nil {
		return "", time.Time{}, err
	}
	return token, expiresAt, nil
}

// ValidateSession reports whether a token exists and has not expired. Expired
// rows are deleted on sight so the table cannot grow without bound.
func (r *Repo) ValidateSession(token string) (bool, error) {
	if token == "" {
		return false, nil
	}
	raw, found, err := r.GetKV(sessionsScope, token)
	if err != nil || !found {
		return false, err
	}
	expiresAt, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		// An unparseable expiry cannot be trusted; drop the row.
		_ = r.DeleteKV(sessionsScope, token)
		return false, nil
	}
	if time.Now().UTC().After(expiresAt) {
		_ = r.DeleteKV(sessionsScope, token)
		return false, nil
	}
	return true, nil
}

// DeleteSession revokes a single token. Logout is idempotent.
func (r *Repo) DeleteSession(token string) error {
	if token == "" {
		return nil
	}
	return r.DeleteKV(sessionsScope, token)
}
