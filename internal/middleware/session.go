package middleware

import (
	"context"
	"net/http"

	"9router/proxy/internal/db"
)

// SessionCookieName is the dashboard session cookie. HttpOnly so injected
// script cannot read it, SameSite=Lax so it is not sent on cross-site POSTs.
const SessionCookieName = "9router_session"

// sessionAuthContextKey marks a request authenticated by session cookie rather
// than by API key. RequireAdmin consults it: a logged-in operator is already
// privileged, while a Bearer key still has to be on the admin list.
type sessionAuthContextKey struct{}

// IsSessionAuthenticated reports whether the request came from a logged-in
// dashboard session.
func IsSessionAuthenticated(r *http.Request) bool {
	v, _ := r.Context().Value(sessionAuthContextKey{}).(bool)
	return v
}

// RequireDashboardSession gates routes that only a logged-in operator may call.
func RequireDashboardSession(repo *db.Repo) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !validSession(repo, r) {
				writeUnauthorized(w)
				return
			}
			ctx := context.WithValue(r.Context(), sessionAuthContextKey{}, true)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireApiKeyOrSession accepts EITHER a dashboard session cookie OR a Bearer
// API key.
//
// The dashboard UI authenticates with a cookie, but non-browser clients (curl,
// scripts, CI) have always used a Bearer key against /api/admin/*. Accepting
// both keeps those callers working after the console moved off API keys.
func RequireApiKeyOrSession(repo *db.Repo) func(http.Handler) http.Handler {
	apiKeyOnly := RequireApiKey(repo)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if validSession(repo, r) {
				ctx := context.WithValue(r.Context(), sessionAuthContextKey{}, true)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			apiKeyOnly(next).ServeHTTP(w, r)
		})
	}
}

func validSession(repo *db.Repo, r *http.Request) bool {
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil || cookie.Value == "" {
		return false
	}
	valid, err := repo.ValidateSession(cookie.Value)
	if err != nil {
		return false
	}
	return valid
}

func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"error":{"code":"unauthorized","message":"Sesi tidak valid atau sudah kedaluwarsa.","type":"auth_error"}}`))
}
