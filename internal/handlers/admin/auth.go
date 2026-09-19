package admin

import (
	"net/http"
	"strings"
	"time"

	"9router/proxy/internal/handlerutil"
	"9router/proxy/internal/middleware"
)

// ---------- dashboard auth ----------
//
// These endpoints are mounted OUTSIDE RequireDashboardSession — they are the
// way in. Everything else under /api/admin/* requires the session cookie they
// issue.

// HandleAuthStatus is public: the login screen needs to know whether the
// account still carries the built-in default password, without a session.
//
// It validates the cookie itself rather than reading a context value, because
// this route is mounted OUTSIDE RequireDashboardSession (the login screen must
// be able to call it while anonymous). The flag reveals only that the install
// still uses the default — which the login page prints anyway.
func (h *Handler) HandleAuthStatus(w http.ResponseWriter, r *http.Request) {
	state, err := h.repo.PasswordState()
	if err != nil {
		handlerutil.WriteJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	authenticated := false
	if cookie, err := r.Cookie(middleware.SessionCookieName); err == nil && cookie.Value != "" {
		if valid, err := h.repo.ValidateSession(cookie.Value); err == nil {
			authenticated = valid
		}
	}

	ok(w, map[string]any{
		"authenticated":      authenticated,
		"mustChangePassword": state.MustChange,
	})
}

// HandleLogin verifies the password and issues a session cookie.
func (h *Handler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		handlerutil.WriteJSONError(w, http.StatusMethodNotAllowed, "Method tidak didukung.")
		return
	}
	body, valid := decodeBody(w, r)
	if !valid {
		return
	}
	password := strPtr(body, "password")
	if password == nil || *password == "" {
		handlerutil.WriteJSONError(w, http.StatusBadRequest, "Password wajib diisi.")
		return
	}

	// Seed on first use so a fresh install is reachable with the documented
	// default. EnsureDefaultPassword is a no-op once a hash exists.
	if _, err := h.repo.EnsureDefaultPassword(); err != nil {
		handlerutil.WriteJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	matched, err := h.repo.VerifyPassword(*password)
	if err != nil {
		handlerutil.WriteJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !matched {
		handlerutil.WriteJSONError(w, http.StatusUnauthorized, "Password salah.")
		return
	}

	token, expiresAt, err := h.repo.CreateSession()
	if err != nil {
		handlerutil.WriteJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     middleware.SessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		// Secure stays off: the dev dashboard is reached over plain HTTP on
		// the LAN. Enable it once the console is served over TLS.
		Secure: false,
	})

	state, _ := h.repo.PasswordState()
	ok(w, map[string]any{
		"status":             "ok",
		"expiresAt":          expiresAt.Format(time.RFC3339),
		"mustChangePassword": state.MustChange,
	})
}

// HandleLogout revokes the current session and clears the cookie.
func (h *Handler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		handlerutil.WriteJSONError(w, http.StatusMethodNotAllowed, "Method tidak didukung.")
		return
	}
	if cookie, err := r.Cookie(middleware.SessionCookieName); err == nil {
		if err := h.repo.DeleteSession(cookie.Value); err != nil {
			handlerutil.WriteJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	http.SetCookie(w, &http.Cookie{
		Name:     middleware.SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	ok(w, map[string]any{"status": "ok"})
}

// HandleChangePassword sets a new password. Requires a valid session so an
// unauthenticated caller cannot lock the operator out.
func (h *Handler) HandleChangePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		handlerutil.WriteJSONError(w, http.StatusMethodNotAllowed, "Method tidak didukung.")
		return
	}
	body, valid := decodeBody(w, r)
	if !valid {
		return
	}

	current := strPtr(body, "currentPassword")
	next := strPtr(body, "newPassword")
	if current == nil || next == nil {
		handlerutil.WriteJSONError(w, http.StatusBadRequest, "currentPassword dan newPassword wajib diisi.")
		return
	}

	matched, err := h.repo.VerifyPassword(*current)
	if err != nil {
		handlerutil.WriteJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !matched {
		handlerutil.WriteJSONError(w, http.StatusUnauthorized, "Password saat ini salah.")
		return
	}

	if strings.TrimSpace(*next) == *current {
		handlerutil.WriteJSONError(w, http.StatusBadRequest, "Password baru harus berbeda dari password saat ini.")
		return
	}

	if err := h.repo.SetPassword(*next); err != nil {
		handlerutil.WriteJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	ok(w, map[string]any{"status": "ok"})
}
