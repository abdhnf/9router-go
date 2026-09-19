package middleware

import (
	"net/http"

	"9router/proxy/internal/db"
	"9router/proxy/internal/handlerutil"
)

// RequireAdmin gates management endpoints. RequireApiKey alone only proves a key
// is active — it says nothing about privilege, so mounting CRUD behind it would
// let any key delete every provider connection.
//
// Privilege is stored in the `kv` table (scope "admin", key "keyIds") as a JSON
// array of apiKeys.id. Storing it there keeps the canonical schema untouched —
// see DESIGN.md §1 rule 2 (new fields go into JSON/tables that already exist,
// never new columns).
//
// Bootstrap: when the `kv` entry has never been written, the first key that
// authenticates is promoted and persisted. Once written — even as an empty
// array — no further auto-promotion happens, so an operator can revoke all
// admins deliberately without the next request silently re-granting it.
func RequireAdmin(repo *db.Repo) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := GetAuthenticatedApiKey(r)
			if key == nil {
				handlerutil.WriteJSONError(w, http.StatusUnauthorized, "Authentication required.")
				return
			}

			ids, exists, err := repo.GetAdminKeyIDs()
			if err != nil {
				handlerutil.WriteJSONError(w, http.StatusInternalServerError, "Gagal membaca daftar admin.")
				return
			}

			if !exists {
				// First-ever admin request: promote and persist this key.
				if err := repo.SetAdminKeyIDs([]string{key.ID}); err != nil {
					handlerutil.WriteJSONError(w, http.StatusInternalServerError, "Gagal menyimpan admin awal.")
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			for _, id := range ids {
				if id == key.ID {
					next.ServeHTTP(w, r)
					return
				}
			}

			handlerutil.WriteJSONError(w, http.StatusForbidden,
				"API key ini tidak memiliki hak admin.")
		})
	}
}
