package db

import (
	"database/sql"
	json "encoding/json/v2"
	"errors"
	"fmt"
)

// adminScope / adminKeysKey locate the admin privilege list inside the shared
// `kv` table. Using kv keeps the canonical schema untouched (DESIGN.md §1 r2).
const (
	adminScope   = "admin"
	adminKeysKey = "keyIds"

	// comboStrategiesScope stores combo routing strategy, keyed by combo name.
	// The canonical `combos` table has no strategy column (see DASHBOARD.md
	// §4.1), and strategy must not be invented as a new column.
	comboStrategiesScope = "comboStrategies"
)

// ComboStrategiesScope exposes the kv scope holding combo routing strategy.
// The canonical `combos` table has no strategy column, so the strategy is
// stored in kv keyed by combo name (see DASHBOARD.md §4.1).
func ComboStrategiesScope() string { return comboStrategiesScope }

// GetKV returns the raw value for (scope, key). found=false when no row exists,
// which callers must distinguish from an empty value.
func (r *Repo) GetKV(scope, key string) (value string, found bool, err error) {
	var v string
	err = r.db.QueryRow(
		`SELECT value FROM kv WHERE scope = ? AND key = ? LIMIT 1`, scope, key,
	).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("get kv %s/%s: %w", scope, key, err)
	}
	return v, true, nil
}

// SetKV upserts a (scope, key) pair.
func (r *Repo) SetKV(scope, key, value string) error {
	_, err := r.db.Exec(
		`INSERT INTO kv (scope, key, value) VALUES (?, ?, ?)
		 ON CONFLICT(scope, key) DO UPDATE SET value = excluded.value`,
		scope, key, value,
	)
	if err != nil {
		return fmt.Errorf("set kv %s/%s: %w", scope, key, err)
	}
	return nil
}

// DeleteKV removes a (scope, key) pair. Missing rows are not an error.
func (r *Repo) DeleteKV(scope, key string) error {
	_, err := r.db.Exec(`DELETE FROM kv WHERE scope = ? AND key = ?`, scope, key)
	if err != nil {
		return fmt.Errorf("delete kv %s/%s: %w", scope, key, err)
	}
	return nil
}

// GetAdminKeyIDs returns the apiKeys.id list allowed to call management
// endpoints. found=false means the list was never initialised, which the admin
// middleware treats as "promote the first caller" (bootstrap).
func (r *Repo) GetAdminKeyIDs() (ids []string, found bool, err error) {
	raw, found, err := r.GetKV(adminScope, adminKeysKey)
	if err != nil || !found {
		return nil, found, err
	}
	if raw == "" {
		return []string{}, true, nil
	}
	if err := json.Unmarshal([]byte(raw), &ids); err != nil {
		return nil, false, fmt.Errorf("parse admin keyIds: %w", err)
	}
	return ids, true, nil
}

// SetAdminKeyIDs persists the admin privilege list. An empty slice is written
// as "[]" (not deleted) so a deliberate revocation is not re-bootstrapped.
func (r *Repo) SetAdminKeyIDs(ids []string) error {
	if ids == nil {
		ids = []string{}
	}
	encoded, err := json.Marshal(ids)
	if err != nil {
		return fmt.Errorf("marshal admin keyIds: %w", err)
	}
	return r.SetKV(adminScope, adminKeysKey, string(encoded))
}
