package admin

import (
	"database/sql"
	json "encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"9router/proxy/internal/db"
	"9router/proxy/internal/dbtest"
)

func newTestRepo(t *testing.T) *db.Repo {
	t.Helper()
	conn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	// A single connection keeps an in-memory SQLite alive across queries.
	conn.SetMaxOpenConns(1)
	if err := dbtest.CreateTables(conn); err != nil {
		t.Fatalf("create tables: %v", err)
	}
	return db.NewRepo(conn)
}

func do(t *testing.T, h *Handler, method, target, body string, handler func(http.ResponseWriter, *http.Request)) *httptest.ResponseRecorder {
	t.Helper()
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, target, reader)
	rec := httptest.NewRecorder()
	handler(rec, req)
	return rec
}

// TestConnectionPatchMergesData is the regression guard for DASHBOARD.md §4.2.
//
// providerConnections.data carries live runtime state: modelLock_<model>,
// backoffLevel, and antigravity quota locks. If PATCH replaces the blob instead
// of merging, every active lock and backoff is silently cleared and the engine
// starts hammering rate-limited providers — with no error and no log line.
func TestConnectionPatchMergesData(t *testing.T) {
	repo := newTestRepo(t)
	h := NewAdminHandler(repo)

	seed := `{"apiKey":"sk-live","modelLock_gemini-3.8-flash":1700000000000,"backoffLevel":3}`
	if _, err := repo.RawDB().Exec(
		`INSERT INTO providerConnections (id, provider, authType, name, isActive, data, createdAt, updatedAt)
		 VALUES ('c1','antigravity','oauth','Utama',1,?,?,?)`,
		seed, "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z",
	); err != nil {
		t.Fatalf("seed connection: %v", err)
	}

	// Operator edits only the display name in the UI.
	rec := do(t, h, http.MethodPatch, "/api/admin/connections/c1",
		`{"name":"Utama (baru)"}`,
		func(w http.ResponseWriter, r *http.Request) { h.HandleConnectionByID(w, r, "c1") })
	if rec.Code != http.StatusOK {
		t.Fatalf("patch status = %d, body = %s", rec.Code, rec.Body.String())
	}

	conn, err := repo.GetProviderConnectionByID("c1")
	if err != nil || conn == nil {
		t.Fatalf("reload connection: %v", err)
	}

	var blob map[string]any
	if err := json.Unmarshal([]byte(conn.Data), &blob); err != nil {
		t.Fatalf("data blob unparseable after patch: %v (raw=%s)", err, conn.Data)
	}

	// The runtime state MUST survive.
	if _, ok := blob["modelLock_gemini-3.8-flash"]; !ok {
		t.Errorf("modelLock_* hilang setelah PATCH — lock aktif terhapus (raw=%s)", conn.Data)
	}
	if got, ok := blob["backoffLevel"]; !ok || got.(float64) != 3 {
		t.Errorf("backoffLevel berubah/hilang: got=%v want=3 (raw=%s)", got, conn.Data)
	}
	// The credential must survive too.
	if got, ok := blob["apiKey"]; !ok || got.(string) != "sk-live" {
		t.Errorf("apiKey hilang setelah PATCH: got=%v", got)
	}
	// And the name must actually have changed.
	if conn.Name == nil || *conn.Name != "Utama (baru)" {
		t.Errorf("name tidak ter-update: got=%v", conn.Name)
	}
}

// TestConnectionPatchDataMergesNewKeys verifies new keys are added and explicit
// nulls delete, without touching siblings.
func TestConnectionPatchDataMergesNewKeys(t *testing.T) {
	repo := newTestRepo(t)
	h := NewAdminHandler(repo)

	_, _ = repo.RawDB().Exec(
		`INSERT INTO providerConnections (id, provider, authType, isActive, data, createdAt, updatedAt)
		 VALUES ('c2','openai','api-key',1,'{"apiKey":"sk-x","staleKey":"drop-me"}','2026-01-01T00:00:00Z','2026-01-01T00:00:00Z')`)

	rec := do(t, h, http.MethodPatch, "/api/admin/connections/c2",
		`{"data":{"region":"id-jkt","staleKey":null}}`,
		func(w http.ResponseWriter, r *http.Request) { h.HandleConnectionByID(w, r, "c2") })
	if rec.Code != http.StatusOK {
		t.Fatalf("patch status = %d body = %s", rec.Code, rec.Body.String())
	}

	conn, _ := repo.GetProviderConnectionByID("c2")
	var blob map[string]any
	_ = json.Unmarshal([]byte(conn.Data), &blob)

	if blob["region"] != "id-jkt" {
		t.Errorf("key baru tidak masuk: %v", blob["region"])
	}
	if _, still := blob["staleKey"]; still {
		t.Errorf("null seharusnya menghapus key: %v", blob["staleKey"])
	}
	if blob["apiKey"] != "sk-x" {
		t.Errorf("apiKey sibling ikut rusak: %v", blob["apiKey"])
	}
}

// TestComboStrategyNotAColumn pins DASHBOARD.md §4.1: the combos table has no
// strategy column, so a combo must still save successfully.
func TestComboStrategyNotAColumn(t *testing.T) {
	repo := newTestRepo(t)
	h := NewAdminHandler(repo)

	rec := do(t, h, http.MethodPost, "/api/admin/combos",
		`{"name":"combo-wombo","strategy":"fusion","models":{"panels":["a","b"],"judge":"c"}}`,
		h.HandleCombos)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create combo status = %d body = %s", rec.Code, rec.Body.String())
	}

	combos, err := repo.GetCombos()
	if err != nil || len(combos) != 1 {
		t.Fatalf("expected 1 combo, got %d (err=%v)", len(combos), err)
	}
	if !strings.Contains(combos[0].Models, "panels") {
		t.Errorf("models JSON tidak tersimpan: %s", combos[0].Models)
	}

	// GetCombos does not read strategy (no column). The router path is
	// GetComboByName, so that is what must return the saved strategy.
	byName, err := repo.GetComboByName("combo-wombo")
	if err != nil || byName == nil {
		t.Fatalf("GetComboByName gagal: %v", err)
	}
	if byName.Strategy != "fusion" {
		t.Errorf("strategy tidak terbaca router: got=%q want=fusion "+
			"(strategy hidup di kv scope comboStrategies, bukan kolom)", byName.Strategy)
	}
}

// TestComboStrategyRoundTripsThroughRouterPath guards the silent-pin bug:
// GetComboByName used to always return "fallback" because the canonical schema
// has no strategy column, so every combo routed as fallback regardless of what
// the dashboard saved.
func TestComboStrategyRoundTripsThroughRouterPath(t *testing.T) {
	repo := newTestRepo(t)
	h := NewAdminHandler(repo)

	for _, strategy := range []string{"sticky", "round-robin", "capacity", "fusion"} {
		name := "c-" + strategy
		rec := do(t, h, http.MethodPost, "/api/admin/combos",
			`{"name":"`+name+`","strategy":"`+strategy+`","models":["a/b","c/d"]}`,
			h.HandleCombos)
		if rec.Code != http.StatusCreated {
			t.Fatalf("create %s: status %d body %s", strategy, rec.Code, rec.Body.String())
		}
		got, err := repo.GetComboByName(name)
		if err != nil || got == nil {
			t.Fatalf("GetComboByName(%s): %v", name, err)
		}
		if got.Strategy != strategy {
			t.Errorf("strategy %q ter-pin jadi %q — routing akan salah tanpa error",
				strategy, got.Strategy)
		}
	}
}

// TestComboDeleteClearsStrategy ensures a deleted combo does not leave a stale
// strategy behind for a future combo reusing the same name.
func TestComboDeleteClearsStrategy(t *testing.T) {
	repo := newTestRepo(t)
	h := NewAdminHandler(repo)

	rec := do(t, h, http.MethodPost, "/api/admin/combos",
		`{"name":"reuse-me","strategy":"fusion","models":["a/b"]}`, h.HandleCombos)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status %d", rec.Code)
	}
	created, _ := repo.GetComboByName("reuse-me")
	if created == nil {
		t.Fatal("combo tidak dibuat")
	}

	del := do(t, h, http.MethodDelete, "/api/admin/combos/"+created.ID, "",
		func(w http.ResponseWriter, r *http.Request) { h.HandleComboByID(w, r, created.ID) })
	if del.Code != http.StatusOK {
		t.Fatalf("delete status %d", del.Code)
	}

	// Recreate with the same name but a different strategy.
	rec2 := do(t, h, http.MethodPost, "/api/admin/combos",
		`{"name":"reuse-me","strategy":"sticky","models":["a/b"]}`, h.HandleCombos)
	if rec2.Code != http.StatusCreated {
		t.Fatalf("recreate status %d body %s", rec2.Code, rec2.Body.String())
	}
	again, _ := repo.GetComboByName("reuse-me")
	if again == nil {
		t.Fatal("combo tidak dibuat ulang")
	}
	if again.Strategy != "sticky" {
		t.Errorf("strategy basi terbawa: got=%q want=sticky", again.Strategy)
	}
}

func TestComboRejectsUnknownStrategy(t *testing.T) {
	repo := newTestRepo(t)
	h := NewAdminHandler(repo)

	rec := do(t, h, http.MethodPost, "/api/admin/combos",
		`{"name":"bad","strategy":"magic"}`, h.HandleCombos)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, mau 400. body = %s", rec.Code, rec.Body.String())
	}
}

// TestSettingsPutMergesPartial ensures a partial PUT cannot wipe unrelated keys.
func TestSettingsPutMergesPartial(t *testing.T) {
	repo := newTestRepo(t)
	h := NewAdminHandler(repo)

	if err := repo.SaveSettings(&db.SettingsData{
		RTKEnabled: true, CavemanLevel: "full", HeadroomUrl: "http://localhost:8787",
	}); err != nil {
		t.Fatalf("seed settings: %v", err)
	}

	rec := do(t, h, http.MethodPut, "/api/admin/settings",
		`{"cavemanEnabled":true}`, h.HandleSettings)
	if rec.Code != http.StatusOK {
		t.Fatalf("put status = %d body = %s", rec.Code, rec.Body.String())
	}

	s, _ := repo.GetSettings()
	if !s.CavemanEnabled {
		t.Errorf("cavemanEnabled tidak ter-set")
	}
	if !s.RTKEnabled {
		t.Errorf("rtkEnabled terhapus oleh PUT parsial")
	}
	if s.HeadroomUrl != "http://localhost:8787" {
		t.Errorf("headroomUrl terhapus: %q", s.HeadroomUrl)
	}
}

func TestKVWarnsOnUnknownScope(t *testing.T) {
	repo := newTestRepo(t)
	h := NewAdminHandler(repo)

	rec := do(t, h, http.MethodPut, "/api/admin/kv/typoScope/someKey",
		`{"value":"x"}`,
		func(w http.ResponseWriter, r *http.Request) { h.HandleKV(w, r, "typoScope", "someKey") })
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "tidak dibaca engine") {
		t.Errorf("peringatan scope tidak muncul: %s", rec.Body.String())
	}

	// Known scope must not warn.
	rec2 := do(t, h, http.MethodPut, "/api/admin/kv/modelAliases/ag",
		`{"value":"antigravity"}`,
		func(w http.ResponseWriter, r *http.Request) { h.HandleKV(w, r, "modelAliases", "ag") })
	if strings.Contains(rec2.Body.String(), "tidak dibaca engine") {
		t.Errorf("scope dikenal malah diberi peringatan: %s", rec2.Body.String())
	}
}

func TestConnectionsCreateAndDelete(t *testing.T) {
	repo := newTestRepo(t)
	h := NewAdminHandler(repo)

	rec := do(t, h, http.MethodPost, "/api/admin/connections",
		`{"provider":"openai","authType":"api-key","name":"CS","data":{"apiKey":"sk-1"}}`,
		h.HandleConnections)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d body = %s", rec.Code, rec.Body.String())
	}
	conns, _ := repo.GetProviderConnections("", false)
	if len(conns) != 1 {
		t.Fatalf("expected 1 connection, got %d", len(conns))
	}
	id := conns[0].ID

	rec2 := do(t, h, http.MethodDelete, "/api/admin/connections/"+id, "",
		func(w http.ResponseWriter, r *http.Request) { h.HandleConnectionByID(w, r, id) })
	if rec2.Code != http.StatusOK {
		t.Fatalf("delete status = %d", rec2.Code)
	}
	after, _ := repo.GetProviderConnections("", false)
	if len(after) != 0 {
		t.Errorf("connection tidak terhapus, sisa %d", len(after))
	}
}

func TestConnectionsCreateRequiresProviderAndAuthType(t *testing.T) {
	repo := newTestRepo(t)
	h := NewAdminHandler(repo)

	rec := do(t, h, http.MethodPost, "/api/admin/connections",
		`{"name":"tanpa provider"}`, h.HandleConnections)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, mau 400. body = %s", rec.Code, rec.Body.String())
	}
}

func TestMetaExposesVocabulary(t *testing.T) {
	repo := newTestRepo(t)
	h := NewAdminHandler(repo)

	rec := do(t, h, http.MethodGet, "/api/admin/meta", "", h.HandleMeta)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"sticky", "round-robin", "fusion", "wenyan-ultra", "modelAliases"} {
		if !strings.Contains(body, want) {
			t.Errorf("meta tidak memuat %q", want)
		}
	}
}
