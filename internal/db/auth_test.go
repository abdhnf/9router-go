package db

import (
	"os"
	"strings"
	"testing"
	"time"
)

// newAuthRepo builds an empty database with NO schema at all, mirroring a fresh
// install where the engine starts before any dashboard migration ran. The auth
// layer must create the one table it depends on itself.
func newAuthRepo(t *testing.T) *Repo {
	t.Helper()

	f, err := os.CreateTemp("", "test_auth_*.sqlite")
	if err != nil {
		t.Fatalf("create temp db: %v", err)
	}
	f.Close()

	conn, err := OpenDatabase(f.Name())
	if err != nil {
		os.Remove(f.Name())
		t.Fatalf("OpenDatabase: %v", err)
	}
	t.Cleanup(func() {
		conn.Close()
		os.Remove(f.Name())
	})

	return NewRepo(conn)
}

func TestEnsureDefaultPasswordCreatesKVTableOnFreshDB(t *testing.T) {
	repo := newAuthRepo(t)

	seeded, err := repo.EnsureDefaultPassword()
	if err != nil {
		t.Fatalf("EnsureDefaultPassword on a schema-less DB: %v", err)
	}
	if !seeded {
		t.Fatal("expected the default password to be seeded on a fresh DB")
	}

	// The seeded password must actually work.
	matched, err := repo.VerifyPassword(DefaultPassword)
	if err != nil {
		t.Fatalf("VerifyPassword: %v", err)
	}
	if !matched {
		t.Fatalf("expected %q to match the seeded hash", DefaultPassword)
	}
}

func TestEnsureDefaultPasswordIsIdempotent(t *testing.T) {
	repo := newAuthRepo(t)

	if _, err := repo.EnsureDefaultPassword(); err != nil {
		t.Fatalf("first seed: %v", err)
	}
	if err := repo.SetPassword("operator-secret"); err != nil {
		t.Fatalf("SetPassword: %v", err)
	}

	// A second boot must not reset an operator-chosen password back to the
	// default.
	seeded, err := repo.EnsureDefaultPassword()
	if err != nil {
		t.Fatalf("second seed: %v", err)
	}
	if seeded {
		t.Fatal("second seed must be a no-op once a hash exists")
	}
	matched, err := repo.VerifyPassword("operator-secret")
	if err != nil {
		t.Fatalf("VerifyPassword: %v", err)
	}
	if !matched {
		t.Fatal("operator password was overwritten by the default seed")
	}
}

func TestVerifyPasswordRejectsWrongPassword(t *testing.T) {
	repo := newAuthRepo(t)
	if _, err := repo.EnsureDefaultPassword(); err != nil {
		t.Fatalf("seed: %v", err)
	}

	matched, err := repo.VerifyPassword("not-the-password")
	if err != nil {
		t.Fatalf("VerifyPassword: %v", err)
	}
	if matched {
		t.Fatal("wrong password must not authenticate")
	}
}

// A missing hash row must never fall through to "allow": that would turn a
// wiped kv table into an open door.
func TestVerifyPasswordWithoutSeedFailsClosed(t *testing.T) {
	repo := newAuthRepo(t)

	matched, err := repo.VerifyPassword(DefaultPassword)
	if err != nil {
		t.Fatalf("VerifyPassword: %v", err)
	}
	if matched {
		t.Fatal("an un-seeded repo must reject every password")
	}
}

func TestSetPasswordEnforcesMinimumLength(t *testing.T) {
	repo := newAuthRepo(t)
	if _, err := repo.EnsureDefaultPassword(); err != nil {
		t.Fatalf("seed: %v", err)
	}

	err := repo.SetPassword(strings.Repeat("a", PasswordMinLength-1))
	if err == nil {
		t.Fatalf("expected a %d-char password to be rejected", PasswordMinLength-1)
	}

	if err := repo.SetPassword(strings.Repeat("a", PasswordMinLength)); err != nil {
		t.Fatalf("expected a %d-char password to be accepted: %v", PasswordMinLength, err)
	}
}

func TestSetPasswordClearsMustChangeFlag(t *testing.T) {
	repo := newAuthRepo(t)
	if _, err := repo.EnsureDefaultPassword(); err != nil {
		t.Fatalf("seed: %v", err)
	}

	state, err := repo.PasswordState()
	if err != nil {
		t.Fatalf("PasswordState: %v", err)
	}
	if !state.MustChange {
		t.Fatal("a freshly seeded install must demand a password change")
	}

	if err := repo.SetPassword("operator-secret"); err != nil {
		t.Fatalf("SetPassword: %v", err)
	}

	state, err = repo.PasswordState()
	if err != nil {
		t.Fatalf("PasswordState after change: %v", err)
	}
	if state.MustChange {
		t.Fatal("must-change flag must clear after a successful password change")
	}
}

func TestSessionLifecycle(t *testing.T) {
	repo := newAuthRepo(t)

	token, expiresAt, err := repo.CreateSession()
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if token == "" {
		t.Fatal("CreateSession returned an empty token")
	}
	if !expiresAt.After(time.Now().UTC()) {
		t.Fatalf("session expiry %v is not in the future", expiresAt)
	}

	valid, err := repo.ValidateSession(token)
	if err != nil {
		t.Fatalf("ValidateSession: %v", err)
	}
	if !valid {
		t.Fatal("a freshly created session must validate")
	}

	if err := repo.DeleteSession(token); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
	valid, err = repo.ValidateSession(token)
	if err != nil {
		t.Fatalf("ValidateSession after logout: %v", err)
	}
	if valid {
		t.Fatal("a revoked session must not validate")
	}
}

func TestValidateSessionRejectsUnknownAndEmptyTokens(t *testing.T) {
	repo := newAuthRepo(t)

	for _, token := range []string{"", "deadbeef-not-a-real-session"} {
		valid, err := repo.ValidateSession(token)
		if err != nil {
			t.Fatalf("ValidateSession(%q): %v", token, err)
		}
		if valid {
			t.Fatalf("token %q must not validate", token)
		}
	}
}

// An unparseable expiry cannot be trusted, so the row must be dropped rather
// than treated as valid.
func TestValidateSessionDropsUnparseableExpiry(t *testing.T) {
	repo := newAuthRepo(t)
	if _, err := repo.EnsureDefaultPassword(); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := repo.SetKV(sessionsScope, "broken-token", "not-a-timestamp"); err != nil {
		t.Fatalf("SetKV: %v", err)
	}

	valid, err := repo.ValidateSession("broken-token")
	if err != nil {
		t.Fatalf("ValidateSession: %v", err)
	}
	if valid {
		t.Fatal("a session with an unparseable expiry must not validate")
	}

	_, found, err := repo.GetKV(sessionsScope, "broken-token")
	if err != nil {
		t.Fatalf("GetKV: %v", err)
	}
	if found {
		t.Fatal("the broken session row should have been deleted")
	}
}

func TestDeleteSessionIsIdempotent(t *testing.T) {
	repo := newAuthRepo(t)
	if err := repo.DeleteSession("never-existed"); err != nil {
		t.Fatalf("deleting an unknown session must not error: %v", err)
	}
	if err := repo.DeleteSession(""); err != nil {
		t.Fatalf("deleting an empty token must not error: %v", err)
	}
}
