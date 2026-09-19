package admin

import (
	"crypto/rand"
	"encoding/hex"
	json "encoding/json/v2"
	"io"
	"net/http"
	"strings"

	"9router/proxy/internal/constants"
	"9router/proxy/internal/db"
	"9router/proxy/internal/handlerutil"
	"9router/proxy/internal/models"
)

// Handler serves the management API consumed by the dashboard UI.
// Every route here is mounted behind RequireApiKey + RequireAdmin.
type Handler struct {
	repo *db.Repo
}

func NewAdminHandler(repo *db.Repo) *Handler {
	return &Handler{repo: repo}
}

// ---------- helpers ----------

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set(constants.HeaderContentType, constants.ContentTypeJSON)
	w.WriteHeader(status)
	_ = json.MarshalWrite(w, payload)
}

func ok(w http.ResponseWriter, payload any) { writeJSON(w, http.StatusOK, payload) }

// decodeBody reads a JSON object. Returns nil (and writes the error) on failure.
func decodeBody(w http.ResponseWriter, r *http.Request) (map[string]any, bool) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 4<<20))
	if err != nil {
		handlerutil.WriteJSONError(w, http.StatusBadRequest, "Gagal membaca body.")
		return nil, false
	}
	if len(body) == 0 {
		return map[string]any{}, true
	}
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		handlerutil.WriteJSONError(w, http.StatusBadRequest, "Body harus berupa objek JSON.")
		return nil, false
	}
	return m, true
}

func strPtr(m map[string]any, key string) *string {
	if v, ok := m[key]; ok {
		if v == nil {
			return nil
		}
		if s, ok := v.(string); ok {
			return &s
		}
	}
	return nil
}

func intPtr(m map[string]any, key string) *int {
	if v, ok := m[key]; ok {
		if f, ok := v.(float64); ok {
			i := int(f)
			return &i
		}
	}
	return nil
}

func boolToIntPtr(m map[string]any, key string) *int {
	if v, ok := m[key]; ok {
		if b, ok := v.(bool); ok {
			i := 0
			if b {
				i = 1
			}
			return &i
		}
	}
	return nil
}

func mapPtr(m map[string]any, key string) map[string]any {
	if v, ok := m[key]; ok {
		if mm, ok := v.(map[string]any); ok {
			return mm
		}
	}
	return nil
}

func newID(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return prefix + hex.EncodeToString(b)
}

// ---------- connections ----------

func (h *Handler) HandleConnections(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		provider := r.URL.Query().Get("provider")
		activeOnly := r.URL.Query().Get("activeOnly") == "true"
		conns, err := h.repo.GetProviderConnections(provider, activeOnly)
		if err != nil {
			handlerutil.WriteJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		ok(w, map[string]any{"connections": conns})

	case http.MethodPost:
		body, valid := decodeBody(w, r)
		if !valid {
			return
		}
		provider := strPtr(body, "provider")
		authType := strPtr(body, "authType")
		if provider == nil || *provider == "" || authType == nil || *authType == "" {
			handlerutil.WriteJSONError(w, http.StatusBadRequest, "provider dan authType wajib diisi.")
			return
		}
		id := strPtr(body, "id")
		if id == nil || *id == "" {
			generated := newID("conn_")
			id = &generated
		}
		dataBlob := mapPtr(body, "data")
		encoded := "{}"
		if dataBlob != nil {
			b, err := json.Marshal(dataBlob)
			if err != nil {
				handlerutil.WriteJSONError(w, http.StatusBadRequest, "data bukan objek JSON valid.")
				return
			}
			encoded = string(b)
		}
		active := 1
		if v := intPtr(body, "isActive"); v != nil {
			active = *v
		}
		conn := &models.ProviderConnection{
			ID: *id, Provider: *provider, AuthType: *authType,
			Name: strPtr(body, "name"), Email: strPtr(body, "email"),
			Priority: intPtr(body, "priority"), IsActive: active, Data: encoded,
		}
		if err := h.repo.CreateProviderConnectionFull(conn); err != nil {
			handlerutil.WriteJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		created, _ := h.repo.GetProviderConnectionByID(*id)
		writeJSON(w, http.StatusCreated, map[string]any{"connection": created})

	default:
		handlerutil.WriteJSONError(w, http.StatusMethodNotAllowed, "Method tidak didukung.")
	}
}

func (h *Handler) HandleConnectionByID(w http.ResponseWriter, r *http.Request, id string) {
	switch r.Method {
	case http.MethodGet:
		conn, err := h.repo.GetProviderConnectionByID(id)
		if err != nil {
			handlerutil.WriteJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if conn == nil {
			handlerutil.WriteJSONError(w, http.StatusNotFound, "Koneksi tidak ditemukan.")
			return
		}
		ok(w, map[string]any{"connection": conn})

	case http.MethodPatch, http.MethodPut:
		body, valid := decodeBody(w, r)
		if !valid {
			return
		}
		// Column-level partial update.
		if err := h.repo.UpdateProviderConnectionFields(
			id, strPtr(body, "name"), strPtr(body, "email"),
			intPtr(body, "priority"), boolToIntPtr(body, "isActive"),
		); err != nil {
			handlerutil.WriteJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		// data blob is MERGED, never replaced — it carries live modelLock_*
		// and backoffLevel state. See DASHBOARD.md §4.2.
		if patch := mapPtr(body, "data"); patch != nil {
			if err := h.repo.MergeProviderConnectionData(id, patch); err != nil {
				handlerutil.WriteJSONError(w, http.StatusInternalServerError, err.Error())
				return
			}
		}
		updated, _ := h.repo.GetProviderConnectionByID(id)
		if updated == nil {
			handlerutil.WriteJSONError(w, http.StatusNotFound, "Koneksi tidak ditemukan.")
			return
		}
		ok(w, map[string]any{"connection": updated})

	case http.MethodDelete:
		if err := h.repo.DeleteProviderConnection(id); err != nil {
			handlerutil.WriteJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		ok(w, map[string]any{"status": "ok"})

	default:
		handlerutil.WriteJSONError(w, http.StatusMethodNotAllowed, "Method tidak didukung.")
	}
}

// HandleConnectionHealthReset clears the health/lock state for one connection.
func (h *Handler) HandleConnectionHealthReset(w http.ResponseWriter, r *http.Request, id string) {
	conn, err := h.repo.GetProviderConnectionByID(id)
	if err != nil || conn == nil {
		handlerutil.WriteJSONError(w, http.StatusNotFound, "Koneksi tidak ditemukan.")
		return
	}
	if err := h.repo.ResetConnectionHealthState(id); err != nil {
		handlerutil.WriteJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, map[string]any{"status": "ok"})
}

// ---------- api keys ----------

func (h *Handler) HandleApiKeys(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		keys, err := h.repo.ListApiKeys()
		if err != nil {
			handlerutil.WriteJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		ok(w, map[string]any{"apiKeys": keys})

	case http.MethodPost:
		body, valid := decodeBody(w, r)
		if !valid {
			return
		}
		keyVal := strPtr(body, "key")
		if keyVal == nil || *keyVal == "" {
			generated := "sk-" + newID("")
			keyVal = &generated
		}
		id := strPtr(body, "id")
		if id == nil || *id == "" {
			generated := newID("key_")
			id = &generated
		}
		k := &models.APIKey{
			ID: *id, Key: *keyVal, Name: strPtr(body, "name"),
			MachineID: strPtr(body, "machineId"),
		}
		if err := h.repo.CreateApiKey(k); err != nil {
			handlerutil.WriteJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"apiKey": k})

	default:
		handlerutil.WriteJSONError(w, http.StatusMethodNotAllowed, "Method tidak didukung.")
	}
}

func (h *Handler) HandleApiKeyByID(w http.ResponseWriter, r *http.Request, id string) {
	switch r.Method {
	case http.MethodPatch, http.MethodPut:
		body, valid := decodeBody(w, r)
		if !valid {
			return
		}
		if err := h.repo.UpdateApiKeyFields(id, strPtr(body, "name"),
			strPtr(body, "machineId"), boolToIntPtr(body, "isActive")); err != nil {
			handlerutil.WriteJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		ok(w, map[string]any{"status": "ok"})

	case http.MethodDelete:
		if err := h.repo.DeleteApiKey(id); err != nil {
			handlerutil.WriteJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		ok(w, map[string]any{"status": "ok"})

	default:
		handlerutil.WriteJSONError(w, http.StatusMethodNotAllowed, "Method tidak didukung.")
	}
}

// ---------- combos ----------

// validComboStrategies mirrors what internal/handlers/chat/combo.go switches on.
var validComboStrategies = map[string]bool{
	"sticky": true, "round-robin": true, "fallback": true,
	"capacity": true, "fusion": true,
}

func (h *Handler) HandleCombos(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		combos, err := h.repo.GetCombos()
		if err != nil {
			handlerutil.WriteJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		ok(w, map[string]any{"combos": combos})

	case http.MethodPost:
		body, valid := decodeBody(w, r)
		if !valid {
			return
		}
		name := strPtr(body, "name")
		if name == nil || *name == "" {
			handlerutil.WriteJSONError(w, http.StatusBadRequest, "name wajib diisi.")
			return
		}
		strategy := strPtr(body, "strategy")
		if strategy != nil && !validComboStrategies[*strategy] {
			handlerutil.WriteJSONError(w, http.StatusBadRequest,
				"strategy harus salah satu: sticky, round-robin, fallback, capacity, fusion.")
			return
		}
		id := strPtr(body, "id")
		if id == nil || *id == "" {
			generated := newID("combo_")
			id = &generated
		}
		// strategy lives INSIDE the models JSON — there is no strategy column.
		modelsJSON := "{}"
		if payload, exists := body["models"]; exists {
			b, err := json.Marshal(payload)
			if err != nil {
				handlerutil.WriteJSONError(w, http.StatusBadRequest, "models bukan JSON valid.")
				return
			}
			modelsJSON = string(b)
		}
		c := &models.Combo{ID: *id, Name: *name, Kind: strPtr(body, "kind"), Models: modelsJSON}
		if strategy != nil {
			c.Strategy = *strategy
		}
		if err := h.repo.CreateCombo(c); err != nil {
			handlerutil.WriteJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		// Strategy has no column in the canonical schema, so it is persisted in
		// kv. CreateCombo already wrote it into the models blob for readers that
		// parse it from there.
		if strategy != nil && *strategy != "" {
			if err := h.repo.SetKV(db.ComboStrategiesScope(), *name, *strategy); err != nil {
				handlerutil.WriteJSONError(w, http.StatusInternalServerError, err.Error())
				return
			}
		}
		writeJSON(w, http.StatusCreated, map[string]any{"combo": c})

	default:
		handlerutil.WriteJSONError(w, http.StatusMethodNotAllowed, "Method tidak didukung.")
	}
}

func (h *Handler) HandleComboByID(w http.ResponseWriter, r *http.Request, id string) {
	switch r.Method {
	case http.MethodGet:
		c, err := h.repo.GetComboById(id)
		if err != nil || c == nil {
			handlerutil.WriteJSONError(w, http.StatusNotFound, "Combo tidak ditemukan.")
			return
		}
		ok(w, map[string]any{"combo": c})

	case http.MethodPatch, http.MethodPut:
		body, valid := decodeBody(w, r)
		if !valid {
			return
		}
		if s := strPtr(body, "strategy"); s != nil && !validComboStrategies[*s] {
			handlerutil.WriteJSONError(w, http.StatusBadRequest,
				"strategy harus salah satu: sticky, round-robin, fallback, capacity, fusion.")
			return
		}
		var modelsJSON *string
		if payload, exists := body["models"]; exists {
			b, err := json.Marshal(payload)
			if err != nil {
				handlerutil.WriteJSONError(w, http.StatusBadRequest, "models bukan JSON valid.")
				return
			}
			s := string(b)
			modelsJSON = &s
		}
		if err := h.repo.UpdateComboFields(id, strPtr(body, "name"), strPtr(body, "kind"), modelsJSON); err != nil {
			handlerutil.WriteJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if s := strPtr(body, "strategy"); s != nil && *s != "" {
			existing, err := h.repo.GetComboById(id)
			if err != nil || existing == nil {
				handlerutil.WriteJSONError(w, http.StatusNotFound, "Combo tidak ditemukan.")
				return
			}
			if err := h.repo.SetKV(db.ComboStrategiesScope(), existing.Name, *s); err != nil {
				handlerutil.WriteJSONError(w, http.StatusInternalServerError, err.Error())
				return
			}
		}
		ok(w, map[string]any{"status": "ok"})

	case http.MethodDelete:
		existing, _ := h.repo.GetComboById(id)
		if err := h.repo.DeleteCombo(id); err != nil {
			handlerutil.WriteJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		// Drop the orphaned strategy entry so a future combo reusing this name
		// does not inherit a stale strategy.
		if existing != nil {
			_ = h.repo.DeleteKV(db.ComboStrategiesScope(), existing.Name)
		}
		ok(w, map[string]any{"status": "ok"})

	default:
		handlerutil.WriteJSONError(w, http.StatusMethodNotAllowed, "Method tidak didukung.")
	}
}

// ---------- settings ----------

func (h *Handler) HandleSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s, err := h.repo.GetSettings()
		if err != nil {
			handlerutil.WriteJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		ok(w, map[string]any{"settings": s})

	case http.MethodPut, http.MethodPatch:
		body, valid := decodeBody(w, r)
		if !valid {
			return
		}
		current, err := h.repo.GetSettings()
		if err != nil {
			handlerutil.WriteJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		// Merge onto current so a partial PUT cannot wipe unrelated settings.
		encoded, err := json.Marshal(current)
		if err != nil {
			handlerutil.WriteJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		merged := map[string]any{}
		_ = json.Unmarshal(encoded, &merged)
		for k, v := range body {
			merged[k] = v
		}
		finalBytes, err := json.Marshal(merged)
		if err != nil {
			handlerutil.WriteJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		var final db.SettingsData
		if err := json.Unmarshal(finalBytes, &final); err != nil {
			handlerutil.WriteJSONError(w, http.StatusBadRequest, "Nilai settings tidak valid.")
			return
		}
		if err := h.repo.SaveSettings(&final); err != nil {
			handlerutil.WriteJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		ok(w, map[string]any{"settings": &final})

	default:
		handlerutil.WriteJSONError(w, http.StatusMethodNotAllowed, "Method tidak didukung.")
	}
}

// ---------- kv ----------

// kvScopes are the scopes the engine actually reads. Writing elsewhere is
// allowed but flagged, since a typo here is a silent no-op.
var kvScopes = map[string]bool{"modelAliases": true, "customModels": true, "admin": true}

func (h *Handler) HandleKV(w http.ResponseWriter, r *http.Request, scope, key string) {
	if scope == "" {
		handlerutil.WriteJSONError(w, http.StatusBadRequest, "scope wajib diisi.")
		return
	}
	switch r.Method {
	case http.MethodGet:
		value, found, err := h.repo.GetKV(scope, key)
		if err != nil {
			handlerutil.WriteJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !found {
			handlerutil.WriteJSONError(w, http.StatusNotFound, "Entry tidak ditemukan.")
			return
		}
		ok(w, map[string]any{"scope": scope, "key": key, "value": value})

	case http.MethodPut, http.MethodPost:
		body, valid := decodeBody(w, r)
		if !valid {
			return
		}
		raw, exists := body["value"]
		if !exists {
			handlerutil.WriteJSONError(w, http.StatusBadRequest, "value wajib diisi.")
			return
		}
		var value string
		if s, isStr := raw.(string); isStr {
			value = s
		} else {
			b, err := json.Marshal(raw)
			if err != nil {
				handlerutil.WriteJSONError(w, http.StatusBadRequest, "value tidak bisa diserialisasi.")
				return
			}
			value = string(b)
		}
		if err := h.repo.SetKV(scope, key, value); err != nil {
			handlerutil.WriteJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		warn := ""
		if !kvScopes[scope] {
			warn = "scope ini tidak dibaca engine; penulisan tidak akan berpengaruh."
		}
		ok(w, map[string]any{"status": "ok", "warning": warn})

	case http.MethodDelete:
		if err := h.repo.DeleteKV(scope, key); err != nil {
			handlerutil.WriteJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		ok(w, map[string]any{"status": "ok"})

	default:
		handlerutil.WriteJSONError(w, http.StatusMethodNotAllowed, "Method tidak didukung.")
	}
}

// ---------- provider nodes ----------

func (h *Handler) HandleProviderNodes(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		nodes, err := h.repo.ListProviderNodes()
		if err != nil {
			handlerutil.WriteJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		ok(w, map[string]any{"providerNodes": nodes})

	case http.MethodPost:
		body, valid := decodeBody(w, r)
		if !valid {
			return
		}
		id := strPtr(body, "id")
		if id == nil || *id == "" {
			generated := newID("node_")
			id = &generated
		}
		dataBlob := mapPtr(body, "data")
		encoded := "{}"
		if dataBlob != nil {
			b, err := json.Marshal(dataBlob)
			if err != nil {
				handlerutil.WriteJSONError(w, http.StatusBadRequest, "data bukan objek JSON valid.")
				return
			}
			encoded = string(b)
		}
		n := &models.ProviderNode{
			ID: *id, Type: strPtr(body, "type"),
			Name: strPtr(body, "name"), Data: encoded,
		}
		if err := h.repo.CreateProviderNode(n); err != nil {
			handlerutil.WriteJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"providerNode": n})

	default:
		handlerutil.WriteJSONError(w, http.StatusMethodNotAllowed, "Method tidak didukung.")
	}
}

func (h *Handler) HandleProviderNodeByID(w http.ResponseWriter, r *http.Request, id string) {
	switch r.Method {
	case http.MethodGet:
		n, data, err := h.repo.GetProviderNodeByID(id)
		if err != nil || n == nil {
			handlerutil.WriteJSONError(w, http.StatusNotFound, "Provider node tidak ditemukan.")
			return
		}
		ok(w, map[string]any{"providerNode": n, "data": data})

	case http.MethodPatch, http.MethodPut:
		body, valid := decodeBody(w, r)
		if !valid {
			return
		}
		if err := h.repo.UpdateProviderNodeFields(id, strPtr(body, "type"), strPtr(body, "name")); err != nil {
			handlerutil.WriteJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if patch := mapPtr(body, "data"); patch != nil {
			if err := h.repo.MergeProviderNodeData(id, patch); err != nil {
				handlerutil.WriteJSONError(w, http.StatusInternalServerError, err.Error())
				return
			}
		}
		ok(w, map[string]any{"status": "ok"})

	case http.MethodDelete:
		if err := h.repo.DeleteProviderNode(id); err != nil {
			handlerutil.WriteJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		ok(w, map[string]any{"status": "ok"})

	default:
		handlerutil.WriteJSONError(w, http.StatusMethodNotAllowed, "Method tidak didukung.")
	}
}

// ---------- meta ----------

// HandleMeta exposes the vocabulary the UI needs to render selects correctly,
// so the frontend never hardcodes strategy names or scope names.
func (h *Handler) HandleMeta(w http.ResponseWriter, r *http.Request) {
	ok(w, map[string]any{
		"comboStrategies": []string{"sticky", "round-robin", "fallback", "capacity", "fusion"},
		"kvScopes":        []string{"modelAliases", "customModels", "admin"},
		"cavemanLevels":   []string{"lite", "full", "ultra", "wenyan-ultra"},
		"ponytailLevels":  []string{"lite", "full", "ultra"},
		"authTypes":       []string{"api-key", "oauth", "cookie"},
		"notes": map[string]string{
			"comboStrategy": "Disimpan di dalam JSON models, bukan kolom terpisah.",
			"connectionData": "Field data di-merge, bukan diganti. Berisi modelLock_* dan backoffLevel.",
		},
	})
}

// IsAdminPath reports whether a path belongs to the management API. Used by
// tests and by the router to keep the admin surface explicit.
func IsAdminPath(p string) bool { return strings.HasPrefix(p, "/api/admin/") }
