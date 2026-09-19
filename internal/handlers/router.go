package handlers

import (
	json "encoding/json/v2"
	"github.com/go-chi/chi/v5"
	"net/http"
	"net/http/pprof"

	"9router/proxy/internal/constants"
	"9router/proxy/internal/db"
	"9router/proxy/internal/handlers/admin"
	"9router/proxy/internal/handlers/chat"
	"9router/proxy/internal/handlers/media"
	"9router/proxy/internal/handlers/oauth"
	"9router/proxy/internal/handlers/shared"
	"9router/proxy/internal/handlerutil"
	"9router/proxy/internal/log"
	"9router/proxy/internal/middleware"
	"9router/proxy/web"
)

// Re-export TokenSaverConfig for root compatibility
type TokenSaverConfig = shared.TokenSaverConfig

// NewTokenSaverConfig re-exports shared.NewTokenSaverConfig.
func NewTokenSaverConfig(rtk, caveman, ponytail bool) *TokenSaverConfig {
	return shared.NewTokenSaverConfig(rtk, caveman, ponytail)
}

// SetupRoutes mounts all domain handlers on the provided router.
func SetupRoutes(r interface {
	Get(pattern string, handlerFn http.HandlerFunc)
	Post(pattern string, handlerFn http.HandlerFunc)
	Delete(pattern string, handlerFn http.HandlerFunc)
	HandleFunc(pattern string, handlerFn http.HandlerFunc)
}, repo *db.Repo, ts *TokenSaverConfig) {
	chatH := chat.NewChatHandler(repo, ts)
	mediaH := media.NewMediaHandler(repo, ts, chatH)
	oauthH := oauth.NewOAuthHandler(repo)

	// Chat, Version & Models Domain
	r.Get("/version", chatH.HandleVersion)
	r.Get("/api/version", chatH.HandleVersion)
	r.Get("/api/version/status", chatH.HandleVersionStatus)
	r.Get("/api/version/check", chatH.HandleCheckUpdate)
	r.Post("/api/version/update", chatH.HandleTriggerUpdate)
	r.Post("/api/version/auto-update", chatH.HandleToggleAutoUpdate)
	r.Get("/models", chatH.HandleModels)
	r.Get("/models/info", chatH.HandleModelsInfo)
	r.Get("/models/{kind}", chatH.HandleModelsByKind)
	r.Get("/models/*", chatH.HandleModelLookup)
	r.Get("/v1/models", chatH.HandleModels)
	r.Get("/v1/models/*", chatH.HandleModelLookup)
	r.Get("/api/v1/models", chatH.HandleModels)
	r.Get("/api/v1/models/*", chatH.HandleModelLookup)
	r.Get("/api/models", chatH.HandleModels)
	r.Get("/api/models/*", chatH.HandleModelLookup)
	r.Get("/api/models/catalog-sync", chatH.HandleCatalogSyncStatus)
	r.Post("/api/models/catalog-sync", chatH.HandleCatalogSyncTrigger)
	r.Post("/chat/completions", chatH.HandleChatCompletions)
	r.Post("/messages", chatH.HandleMessages)
	r.Post("/messages/count_tokens", chatH.HandleCountTokens)
	r.Post("/api/chat", chatH.HandleOllamaChat)

	// Media, Audio, Video & Web Tools Domain
	r.Post("/embeddings", mediaH.HandleEmbeddings)
	r.Post("/responses", mediaH.HandleResponses)
	r.Post("/responses/compact", mediaH.HandleResponsesCompact)
	r.Post("/images/generations", mediaH.HandleImages)
	r.Post("/audio/speech", mediaH.HandleAudioSpeech)
	r.Get("/audio/voices", mediaH.HandleAudioVoices)
	r.Post("/audio/transcriptions", mediaH.HandleAudioTranscriptions)
	r.Post("/videos/generations", mediaH.HandleVideoGenerations)
	r.Post("/videos/edits", mediaH.HandleVideoEdits)
	r.Post("/videos/extensions", mediaH.HandleVideoExtensions)
	r.Get("/videos/{id}", mediaH.HandleVideoGet)
	r.Post("/search", mediaH.HandleSearch)
	r.Post("/scrape", mediaH.HandleScrape)
	r.Post("/web/fetch", mediaH.HandleWebFetch)

	// Proxy Pool Deploy Domain
	r.Post("/proxy-pools/vercel-deploy", mediaH.HandleVercelDeploy)
	r.Post("/proxy-pools/deno-deploy", mediaH.HandleDenoDeploy)
	r.Post("/proxy-pools/cloudflare-deploy", mediaH.HandleCloudflareDeploy)

	// CLI Tools Status Domain (dashboard batch status for installed CLI tools)
	r.Get("/cli-tools/all-statuses", media.NewCLIToolsHandler().HandleAllStatuses)

	// Headroom Management Domain (token-compression proxy lifecycle + dashboard proxy)
	headroomH := media.NewHeadroomHandler(repo)
	r.Post("/headroom/start", headroomH.HandleHeadroomStart)
	r.Post("/headroom/stop", headroomH.HandleHeadroomStop)
	r.Post("/headroom/restart", headroomH.HandleHeadroomRestart)
	r.Get("/headroom/status", headroomH.HandleHeadroomStatus)
	r.Get("/headroom/extras", headroomH.HandleHeadroomExtras)
	r.Post("/headroom/extras", headroomH.HandleHeadroomExtras)
	r.Delete("/headroom/extras", headroomH.HandleHeadroomExtras)
	r.HandleFunc("/headroom/proxy", headroomH.HandleHeadroomProxy)
	r.HandleFunc("/headroom/proxy/*", headroomH.HandleHeadroomProxy)

	// OAuth & Import Tokens Domain
	r.Post("/api/oauth/{provider}/import", oauthH.HandleOAuthImport)
	r.Get("/api/oauth/kiro/social-authorize", oauthH.HandleOAuthKiroSocialAuthorize)
	r.Post("/api/oauth/kiro/social-exchange", oauthH.HandleOAuthKiroSocialExchange)
	r.Post("/api/oauth/codex/bulk-import", oauthH.HandleOAuthCodexBulkImport)
	r.Post("/api/oauth/grok-cli/bulk-import", oauthH.HandleOAuthGrokCliBulkImport)

	// Live Console Logs Domain (dashboard "Monitor Console Log")
	r.Get("/translator/console-logs", HandleConsoleLogsGet)
	r.Delete("/translator/console-logs", HandleConsoleLogsDelete)
	r.Get("/translator/console-logs/stream", HandleConsoleLogsStream)

	// Usage Real-time SSE Stream & Stats Domain (dashboard topology animation + recent requests)
	r.Get("/usage/stream", HandleUsageStream(repo))
	r.Get("/api/usage/stream", HandleUsageStream(repo))
	r.Get("/usage/stats", HandleUsageStats(repo))
	r.Get("/api/usage/stats", HandleUsageStats(repo))

	// Debug Tracing Domain (p50/p95 latency per provider+model)
	r.Get("/debug/traces", HandleDebugTraces)
}

// SetupServerRouter mounts public endpoints (/health, /api/hello) and
// API-key protected routes (all engine + admin routes) on the chi router.
func SetupServerRouter(r chi.Router, repo *db.Repo, ts *TokenSaverConfig) {
	// Public (unauthenticated) endpoints
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(constants.HeaderContentType, constants.ContentTypeJSON)
		w.Write([]byte(`{"status":"ok"}`))
	})

	r.HandleFunc("/api/hello", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(constants.HeaderContentType, constants.ContentTypeJSON)
		w.WriteHeader(http.StatusOK)
		if r.Method != http.MethodHead {
			w.Write([]byte(`{"status":"ok","message":"hello"}`))
		}
	})

	// Profiling endpoints (pprof)
	r.HandleFunc("/debug/pprof/", pprof.Index)
	r.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	r.HandleFunc("/debug/pprof/profile", pprof.Profile)
	r.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	r.HandleFunc("/debug/pprof/trace", pprof.Trace)
	r.HandleFunc("/debug/pprof/*", pprof.Index)

	// ---------- dashboard authentication ----------
	//
	// Public by necessity: these ARE the way in. Password login is separate
	// from API-key auth on purpose — the LLM endpoints (/v1/*) keep working
	// with a Bearer key, while the browser gets an HttpOnly session cookie so
	// the admin credential never lives in localStorage.
	dashAuthH := admin.NewAdminHandler(repo)
	if _, err := repo.EnsureDefaultPassword(); err != nil {
		log.Error("auth", "seed default password", "error", err)
	}

	r.Post("/api/auth/login", dashAuthH.HandleLogin)
	r.Post("/api/auth/logout", dashAuthH.HandleLogout)
	// Public: the login screen reads `mustChangePassword` to decide whether to
	// keep showing the built-in default hint. The flag reveals only that the
	// install still uses the default — which the login page prints anyway.
	r.Get("/api/auth/status", dashAuthH.HandleAuthStatus)

	// Password change needs a session but must not sit behind the full admin
	// gate, otherwise an operator stuck on the forced-change screen could
	// never satisfy it.
	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireDashboardSession(repo))
		r.Post("/api/auth/password", dashAuthH.HandleChangePassword)
	})

	// Engine domain routes. Accepts a Bearer API key OR the dashboard session
	// cookie: /usage/stream lives in here and the console streams telemetry over
	// its cookie. Auth strength is unchanged — the cookie is only issued after a
	// correct password, and RequireApiKey still works exactly as before.
	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireApiKeyOrSession(repo))

		// Health reset endpoint — dashboard calls this via headroom proxy
		r.Post("/admin/health/reset", func(w http.ResponseWriter, r *http.Request) {
			provider := r.URL.Query().Get("provider")
			model := r.URL.Query().Get("model")
			if err := repo.ResetProviderHealth(provider, model); err != nil {
				handlerutil.WriteJSONError(w, http.StatusInternalServerError, err.Error())
				return
			}
			w.Header().Set(constants.HeaderContentType, constants.ContentTypeJSON)
			json.MarshalWrite(w, map[string]string{"status": "ok"})
		})

		SetupRoutes(r, repo, ts)
	})

	// Management API — dashboard UI. Accepts EITHER the dashboard session
	// cookie (browser) OR a Bearer API key (curl, scripts, CI), then requires
	// admin privilege. This cannot live inside the RequireApiKey group above:
	// the console authenticates with a cookie and no longer holds an API key.
	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireApiKeyOrSession(repo))
		r.Use(middleware.RequireAdmin(repo))
		adminH := admin.NewAdminHandler(repo)

		r.Get("/api/admin/meta", adminH.HandleMeta)

		r.Route("/api/admin/connections", func(r chi.Router) {
			r.Get("/", adminH.HandleConnections)
			r.Post("/", adminH.HandleConnections)
			r.Get("/{id}", func(w http.ResponseWriter, req *http.Request) {
				adminH.HandleConnectionByID(w, req, chi.URLParam(req, "id"))
			})
			r.Patch("/{id}", func(w http.ResponseWriter, req *http.Request) {
				adminH.HandleConnectionByID(w, req, chi.URLParam(req, "id"))
			})
			r.Delete("/{id}", func(w http.ResponseWriter, req *http.Request) {
				adminH.HandleConnectionByID(w, req, chi.URLParam(req, "id"))
			})
			r.Post("/{id}/health/reset", func(w http.ResponseWriter, req *http.Request) {
				adminH.HandleConnectionHealthReset(w, req, chi.URLParam(req, "id"))
			})
		})

		r.Route("/api/admin/api-keys", func(r chi.Router) {
			r.Get("/", adminH.HandleApiKeys)
			r.Post("/", adminH.HandleApiKeys)
			r.Patch("/{id}", func(w http.ResponseWriter, req *http.Request) {
				adminH.HandleApiKeyByID(w, req, chi.URLParam(req, "id"))
			})
			r.Delete("/{id}", func(w http.ResponseWriter, req *http.Request) {
				adminH.HandleApiKeyByID(w, req, chi.URLParam(req, "id"))
			})
		})

		r.Route("/api/admin/combos", func(r chi.Router) {
			r.Get("/", adminH.HandleCombos)
			r.Post("/", adminH.HandleCombos)
			r.Get("/{id}", func(w http.ResponseWriter, req *http.Request) {
				adminH.HandleComboByID(w, req, chi.URLParam(req, "id"))
			})
			r.Patch("/{id}", func(w http.ResponseWriter, req *http.Request) {
				adminH.HandleComboByID(w, req, chi.URLParam(req, "id"))
			})
			r.Delete("/{id}", func(w http.ResponseWriter, req *http.Request) {
				adminH.HandleComboByID(w, req, chi.URLParam(req, "id"))
			})
		})

		r.Route("/api/admin/provider-nodes", func(r chi.Router) {
			r.Get("/", adminH.HandleProviderNodes)
			r.Post("/", adminH.HandleProviderNodes)
			r.Patch("/{id}", func(w http.ResponseWriter, req *http.Request) {
				adminH.HandleProviderNodeByID(w, req, chi.URLParam(req, "id"))
			})
			r.Delete("/{id}", func(w http.ResponseWriter, req *http.Request) {
				adminH.HandleProviderNodeByID(w, req, chi.URLParam(req, "id"))
			})
		})

		r.Get("/api/admin/settings", adminH.HandleSettings)
		r.Put("/api/admin/settings", adminH.HandleSettings)

		r.Get("/api/admin/kv/{scope}/{key}", func(w http.ResponseWriter, req *http.Request) {
			adminH.HandleKV(w, req, chi.URLParam(req, "scope"), chi.URLParam(req, "key"))
		})
		r.Put("/api/admin/kv/{scope}/{key}", func(w http.ResponseWriter, req *http.Request) {
			adminH.HandleKV(w, req, chi.URLParam(req, "scope"), chi.URLParam(req, "key"))
		})
		r.Delete("/api/admin/kv/{scope}/{key}", func(w http.ResponseWriter, req *http.Request) {
			adminH.HandleKV(w, req, chi.URLParam(req, "scope"), chi.URLParam(req, "key"))
		})
	})

	// Embedded Dashboard UI (Web SPA)
	web.RegisterDashboardRoutes(r)
}
