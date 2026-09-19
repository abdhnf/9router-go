package admin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"9router/proxy/internal/db"
	"9router/proxy/internal/middleware"
)

// newTestHandler builds an admin handler over an in-memory database with the
// canonical schema already created and the default password seeded, mirroring
// what SetupServerRouter does at boot.
func newTestHandler(t *testing.T) (*Handler, *db.Repo) {
	t.Helper()
	repo := newTestRepo(t)
	if _, err := repo.EnsureDefaultPassword(); err != nil {
		t.Fatalf("seed default password: %v", err)
	}
	return NewAdminHandler(repo), repo
}

// login posts a password and returns the response plus any session cookie.
func login(t *testing.T, h *Handler, password string) (*httptest.ResponseRecorder, *http.Cookie) {
	t.Helper()
	body := strings.NewReader(`{"password":` + jsonString(password) + `}`)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.HandleLogin(rec, req)

	for _, c := range rec.Result().Cookies() {
		if c.Name == middleware.SessionCookieName {
			return rec, c
		}
	}
	return rec, nil
}

func jsonString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

func TestLoginWithDefaultPasswordSucceeds(t *testing.T) {
	h, _ := newTestHandler(t)

	rec, cookie := login(t, h, "123456")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if cookie == nil {
		t.Fatal("login must set the session cookie")
	}
	if !cookie.HttpOnly {
		t.Fatal("session cookie must be HttpOnly so page scripts cannot read it")
	}
	if cookie.Path != "/" {
		t.Fatalf("expected cookie path /, got %q", cookie.Path)
	}
}

func TestLoginRejectsWrongPassword(t *testing.T) {
	h, _ := newTestHandler(t)

	rec, cookie := login(t, h, "definitely-not-it")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	if cookie != nil {
		t.Fatal("a failed login must not issue a session cookie")
	}
}

func TestLoginRejectsEmptyPassword(t *testing.T) {
	h, _ := newTestHandler(t)

	rec, cookie := login(t, h, "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if cookie != nil {
		t.Fatal("an empty password must not issue a session cookie")
	}
}

func TestLoginRejectsNonPost(t *testing.T) {
	h, _ := newTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/login", nil)
	rec := httptest.NewRecorder()
	h.HandleLogin(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}

// The login screen needs this flag before it has a session.
func TestAuthStatusIsPublicAndReportsMustChange(t *testing.T) {
	h, _ := newTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/status", nil)
	rec := httptest.NewRecorder()
	h.HandleAuthStatus(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 without a session, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"mustChangePassword":true`) {
		t.Fatalf("fresh install must report mustChangePassword=true, got %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"authenticated":false`) {
		t.Fatalf("anonymous caller must report authenticated=false, got %s", rec.Body.String())
	}
}

// A valid cookie must be recognised even though this route sits outside
// RequireDashboardSession.
func TestAuthStatusRecognisesValidSession(t *testing.T) {
	h, _ := newTestHandler(t)

	_, cookie := login(t, h, "123456")
	if cookie == nil {
		t.Fatal("login did not issue a cookie")
	}

	req := httptest.NewRequest(http.MethodGet, "/api/auth/status", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	h.HandleAuthStatus(rec, req)

	if !strings.Contains(rec.Body.String(), `"authenticated":true`) {
		t.Fatalf("valid session must report authenticated=true, got %s", rec.Body.String())
	}
}

func TestChangePasswordRequiresCorrectCurrentPassword(t *testing.T) {
	h, _ := newTestHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/password",
		strings.NewReader(`{"currentPassword":"wrong","newPassword":"operator-secret"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.HandleChangePassword(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestChangePasswordRejectsShortPassword(t *testing.T) {
	h, _ := newTestHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/password",
		strings.NewReader(`{"currentPassword":"123456","newPassword":"short"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.HandleChangePassword(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for a too-short password, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestChangePasswordRejectsReusingCurrentPassword(t *testing.T) {
	h, _ := newTestHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/password",
		strings.NewReader(`{"currentPassword":"123456","newPassword":"123456"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.HandleChangePassword(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when reusing the current password, got %d", rec.Code)
	}
}

// The full flow: log in with the default, change it, then confirm the old
// password stops working and the new one starts working.
func TestChangePasswordFlowEndToEnd(t *testing.T) {
	h, _ := newTestHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/password",
		strings.NewReader(`{"currentPassword":"123456","newPassword":"operator-secret"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.HandleChangePassword(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	if rec, _ := login(t, h, "123456"); rec.Code != http.StatusUnauthorized {
		t.Fatalf("the old default password must stop working, got %d", rec.Code)
	}
	if rec, cookie := login(t, h, "operator-secret"); rec.Code != http.StatusOK || cookie == nil {
		t.Fatalf("the new password must work, got %d", rec.Code)
	}
}

// Once a real password is set, the forced-change screen must stop appearing.
func TestAuthStatusClearsMustChangeAfterPasswordSet(t *testing.T) {
	h, _ := newTestHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/password",
		strings.NewReader(`{"currentPassword":"123456","newPassword":"operator-secret"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.HandleChangePassword(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("change password: %d %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/auth/status", nil)
	rec = httptest.NewRecorder()
	h.HandleAuthStatus(rec, req)

	if !strings.Contains(rec.Body.String(), `"mustChangePassword":false`) {
		t.Fatalf("expected mustChangePassword=false, got %s", rec.Body.String())
	}
}

func TestLogoutRevokesSession(t *testing.T) {
	h, repo := newTestHandler(t)

	_, cookie := login(t, h, "123456")
	if cookie == nil {
		t.Fatal("login did not issue a cookie")
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	h.HandleLogout(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	valid, err := repo.ValidateSession(cookie.Value)
	if err != nil {
		t.Fatalf("ValidateSession: %v", err)
	}
	if valid {
		t.Fatal("the session must be revoked after logout")
	}
}

func TestLogoutWithoutCookieIsHarmless(t *testing.T) {
	h, _ := newTestHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	rec := httptest.NewRecorder()
	h.HandleLogout(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("logout without a session must be a no-op 200, got %d", rec.Code)
	}
}
