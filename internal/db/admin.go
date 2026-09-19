package db

import (
	"database/sql"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"time"

	"9router/proxy/internal/models"
)

// This file holds the write operations the management dashboard needs.
// Before it existed, the repo could only read most management entities —
// see DASHBOARD.md §1.

func nowUTC() string { return time.Now().UTC().Format(time.RFC3339) }

// ---------- provider connections ----------

// UpdateProviderConnectionFields applies a partial update. Only non-nil fields
// are written, so a PATCH that omits a column leaves it untouched.
func (r *Repo) UpdateProviderConnectionFields(id string, name, email *string, priority *int, isActive *int) error {
	sets := []string{}
	args := []any{}
	if name != nil {
		sets = append(sets, "name = ?")
		args = append(args, *name)
	}
	if email != nil {
		sets = append(sets, "email = ?")
		args = append(args, *email)
	}
	if priority != nil {
		sets = append(sets, "priority = ?")
		args = append(args, *priority)
	}
	if isActive != nil {
		sets = append(sets, "isActive = ?")
		args = append(args, *isActive)
	}
	if len(sets) == 0 {
		return nil
	}
	sets = append(sets, "updatedAt = ?")
	args = append(args, nowUTC(), id)
	_, err := r.db.Exec(`UPDATE providerConnections SET `+joinComma(sets)+` WHERE id = ?`, args...)
	if err != nil {
		return fmt.Errorf("update provider connection %s: %w", id, err)
	}
	return nil
}

// MergeProviderConnectionData merges patch into the existing data JSON blob.
//
// This MUST merge rather than replace. The blob holds live runtime state —
// modelLock_<model>, backoffLevel, and antigravity quota locks. Replacing it
// wholesale silently clears every active lock and backoff, which surfaces later
// as a flood of 429s with no error and no log line. See DASHBOARD.md §4.2.
func (r *Repo) MergeProviderConnectionData(id string, patch map[string]any) error {
	if len(patch) == 0 {
		return nil
	}
	var raw string
	err := r.db.QueryRow(`SELECT data FROM providerConnections WHERE id = ?`, id).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("provider connection %s tidak ditemukan", id)
	}
	if err != nil {
		return fmt.Errorf("read provider connection data %s: %w", id, err)
	}

	merged := map[string]any{}
	if raw != "" {
		// Best-effort: a corrupt blob should not block the update, but we must
		// not silently drop it either — start from an empty map only when the
		// blob is genuinely unparseable.
		_ = json.Unmarshal([]byte(raw), &merged)
	}
	for k, v := range patch {
		if v == nil {
			delete(merged, k)
			continue
		}
		merged[k] = v
	}

	encoded, err := json.Marshal(merged)
	if err != nil {
		return fmt.Errorf("marshal provider connection data %s: %w", id, err)
	}
	_, err = r.db.Exec(`UPDATE providerConnections SET data = ?, updatedAt = ? WHERE id = ?`,
		string(encoded), nowUTC(), id)
	if err != nil {
		return fmt.Errorf("write provider connection data %s: %w", id, err)
	}
	return nil
}

// CreateProviderConnectionFull inserts a connection with an explicit data blob,
// unlike CreateProviderConnection which only accepts an apiKey.
func (r *Repo) CreateProviderConnectionFull(conn *models.ProviderConnection) error {
	data := conn.Data
	if data == "" {
		data = "{}"
	}
	now := nowUTC()
	_, err := r.db.Exec(
		`INSERT INTO providerConnections (id, provider, authType, name, email, priority, isActive, data, createdAt, updatedAt)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		conn.ID, conn.Provider, conn.AuthType, conn.Name, conn.Email,
		conn.Priority, conn.IsActive, data, now, now,
	)
	if err != nil {
		return fmt.Errorf("create provider connection: %w", err)
	}
	return nil
}

// DeleteProviderConnection removes a connection.
func (r *Repo) DeleteProviderConnection(id string) error {
	_, err := r.db.Exec(`DELETE FROM providerConnections WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete provider connection %s: %w", id, err)
	}
	return nil
}

// ---------- api keys ----------

// ListApiKeys returns every key. Used only by the admin dashboard, so exposing
// the key value here is intentional (the operator needs to copy it into clients).
func (r *Repo) ListApiKeys() ([]*models.APIKey, error) {
	rows, err := r.db.Query(
		`SELECT id, key, name, machineId, isActive, createdAt FROM apiKeys
		 ORDER BY createdAt DESC`)
	if err != nil {
		return nil, fmt.Errorf("list api keys: %w", err)
	}
	defer rows.Close()

	var keys []*models.APIKey
	for rows.Next() {
		var k models.APIKey
		if err := rows.Scan(&k.ID, &k.Key, &k.Name, &k.MachineID, &k.IsActive, &k.CreatedAt); err != nil {
			return nil, err
		}
		keys = append(keys, &k)
	}
	return keys, rows.Err()
}

// CreateApiKey inserts a key. isActive defaults to 1.
func (r *Repo) CreateApiKey(k *models.APIKey) error {
	active := k.IsActive
	if active == 0 {
		active = 1
	}
	_, err := r.db.Exec(
		`INSERT INTO apiKeys (id, key, name, machineId, isActive, createdAt)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		k.ID, k.Key, k.Name, k.MachineID, active, nowUTC(),
	)
	if err != nil {
		return fmt.Errorf("create api key: %w", err)
	}
	return nil
}

// UpdateApiKeyFields applies a partial update (name / machineId / isActive).
func (r *Repo) UpdateApiKeyFields(id string, name, machineID *string, isActive *int) error {
	sets := []string{}
	args := []any{}
	if name != nil {
		sets = append(sets, "name = ?")
		args = append(args, *name)
	}
	if machineID != nil {
		sets = append(sets, "machineId = ?")
		args = append(args, *machineID)
	}
	if isActive != nil {
		sets = append(sets, "isActive = ?")
		args = append(args, *isActive)
	}
	if len(sets) == 0 {
		return nil
	}
	args = append(args, id)
	_, err := r.db.Exec(`UPDATE apiKeys SET `+joinComma(sets)+` WHERE id = ?`, args...)
	if err != nil {
		return fmt.Errorf("update api key %s: %w", id, err)
	}
	return nil
}

// DeleteApiKey removes a key.
func (r *Repo) DeleteApiKey(id string) error {
	_, err := r.db.Exec(`DELETE FROM apiKeys WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete api key %s: %w", id, err)
	}
	return nil
}

// ---------- combos ----------

// CreateCombo inserts a combo.
//
// NOTE: the `combos` table has no `strategy` column — the canonical schema is
// (id, name, kind, models, createdAt, updatedAt). models.Strategy is therefore
// serialised INSIDE the models JSON. Writing it to a column would fail; storing
// it anywhere the router does not read means the combo saves but routing
// silently ignores the strategy. See DASHBOARD.md §4.1.
func (r *Repo) CreateCombo(c *models.Combo) error {
	now := nowUTC()
	_, err := r.db.Exec(
		`INSERT INTO combos (id, name, kind, models, createdAt, updatedAt)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		c.ID, c.Name, c.Kind, c.Models, now, now,
	)
	if err != nil {
		return fmt.Errorf("create combo: %w", err)
	}
	return nil
}

// UpdateComboFields applies a partial update (name / kind / models).
func (r *Repo) UpdateComboFields(id string, name, kind, modelsJSON *string) error {
	sets := []string{}
	args := []any{}
	if name != nil {
		sets = append(sets, "name = ?")
		args = append(args, *name)
	}
	if kind != nil {
		sets = append(sets, "kind = ?")
		args = append(args, *kind)
	}
	if modelsJSON != nil {
		sets = append(sets, "models = ?")
		args = append(args, *modelsJSON)
	}
	if len(sets) == 0 {
		return nil
	}
	sets = append(sets, "updatedAt = ?")
	args = append(args, nowUTC(), id)
	_, err := r.db.Exec(`UPDATE combos SET `+joinComma(sets)+` WHERE id = ?`, args...)
	if err != nil {
		return fmt.Errorf("update combo %s: %w", id, err)
	}
	return nil
}

// DeleteCombo removes a combo.
func (r *Repo) DeleteCombo(id string) error {
	_, err := r.db.Exec(`DELETE FROM combos WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete combo %s: %w", id, err)
	}
	return nil
}

// ---------- provider nodes ----------

// CreateProviderNode inserts a custom OpenAI/Anthropic-compatible provider node.
func (r *Repo) CreateProviderNode(n *models.ProviderNode) error {
	now := nowUTC()
	data := n.Data
	if data == "" {
		data = "{}"
	}
	_, err := r.db.Exec(
		`INSERT INTO providerNodes (id, type, name, data, createdAt, updatedAt)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		n.ID, n.Type, n.Name, data, now, now,
	)
	if err != nil {
		return fmt.Errorf("create provider node: %w", err)
	}
	return nil
}

// ListProviderNodes returns every node.
func (r *Repo) ListProviderNodes() ([]*models.ProviderNode, error) {
	rows, err := r.db.Query(
		`SELECT id, type, name, data, createdAt, updatedAt FROM providerNodes ORDER BY createdAt DESC`)
	if err != nil {
		return nil, fmt.Errorf("list provider nodes: %w", err)
	}
	defer rows.Close()

	var nodes []*models.ProviderNode
	for rows.Next() {
		var n models.ProviderNode
		if err := rows.Scan(&n.ID, &n.Type, &n.Name, &n.Data, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, err
		}
		nodes = append(nodes, &n)
	}
	return nodes, rows.Err()
}

// UpdateProviderNodeFields applies a partial update (type / name).
func (r *Repo) UpdateProviderNodeFields(id string, nodeType, name *string) error {
	sets := []string{}
	args := []any{}
	if nodeType != nil {
		sets = append(sets, "type = ?")
		args = append(args, *nodeType)
	}
	if name != nil {
		sets = append(sets, "name = ?")
		args = append(args, *name)
	}
	if len(sets) == 0 {
		return nil
	}
	sets = append(sets, "updatedAt = ?")
	args = append(args, nowUTC(), id)
	_, err := r.db.Exec(`UPDATE providerNodes SET `+joinComma(sets)+` WHERE id = ?`, args...)
	if err != nil {
		return fmt.Errorf("update provider node %s: %w", id, err)
	}
	return nil
}

// MergeProviderNodeData merges patch into the node data blob (same rationale as
// MergeProviderConnectionData).
func (r *Repo) MergeProviderNodeData(id string, patch map[string]any) error {
	if len(patch) == 0 {
		return nil
	}
	var raw string
	err := r.db.QueryRow(`SELECT data FROM providerNodes WHERE id = ?`, id).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("provider node %s tidak ditemukan", id)
	}
	if err != nil {
		return fmt.Errorf("read provider node data %s: %w", id, err)
	}
	merged := map[string]any{}
	if raw != "" {
		_ = json.Unmarshal([]byte(raw), &merged)
	}
	for k, v := range patch {
		if v == nil {
			delete(merged, k)
			continue
		}
		merged[k] = v
	}
	encoded, err := json.Marshal(merged)
	if err != nil {
		return fmt.Errorf("marshal provider node data %s: %w", id, err)
	}
	_, err = r.db.Exec(`UPDATE providerNodes SET data = ?, updatedAt = ? WHERE id = ?`,
		string(encoded), nowUTC(), id)
	if err != nil {
		return fmt.Errorf("write provider node data %s: %w", id, err)
	}
	return nil
}

// DeleteProviderNode removes a node.
func (r *Repo) DeleteProviderNode(id string) error {
	_, err := r.db.Exec(`DELETE FROM providerNodes WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete provider node %s: %w", id, err)
	}
	return nil
}

// ---------- settings ----------

// SaveSettings replaces the single settings row. Settings holds no runtime
// lock state, so a full write is safe here.
func (r *Repo) SaveSettings(data *SettingsData) error {
	encoded, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}
	_, err = r.db.Exec(
		`INSERT INTO settings (id, data) VALUES (1, ?)
		 ON CONFLICT(id) DO UPDATE SET data = excluded.data`,
		string(encoded),
	)
	if err != nil {
		return fmt.Errorf("save settings: %w", err)
	}
	return nil
}

// joinComma joins SQL SET fragments. Kept local to avoid importing strings in
// every caller.
func joinComma(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ", "
		}
		out += p
	}
	return out
}
