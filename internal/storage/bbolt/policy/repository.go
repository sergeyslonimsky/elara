// Package policy is the bbolt-backed repository for Casbin policy rules.
//
// Repository implements two contracts:
//   - persist.Adapter — Casbin's persistence interface used at LoadPolicy
//     time and (with AutoSave=off project-wide) on the cold-path
//     SavePolicy / AddPolicy / RemovePolicy / RemoveFilteredPolicy calls.
//   - The project-defined casbin.PolicyRepository in
//     internal/service/auth/casbin/enforcer.go which requires ctx-aware
//     mutation methods so writes participate in usecase transactions.
//
// Storage layout: each rule is one key/value entry. The key is a
// JSON-encoded []string{sec, ptype, ...fields} and the value is empty.
// This allows incremental AddPolicy/RemovePolicy without rewriting the
// whole bucket and gives RemoveFilteredPolicy a single linear scan.
//
// All ctx-aware methods auto-join an outer transaction via the
// pkg/bbolt querier; non-ctx variants delegate to the ctx variant with
// context.Background so the bbolt manager opens a short-lived tx.
package policy

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	casbinmodel "github.com/casbin/casbin/v2/model"
	"github.com/casbin/casbin/v2/persist"

	"github.com/sergeyslonimsky/elara/pkg/bbolt"
)

const minPartsLen = 3

// Repository stores and retrieves Casbin policy rules in bbolt.
type Repository struct {
	dbm bbolt.Manager
}

// NewRepository creates a new Repository backed by the given Manager.
func NewRepository(dbm bbolt.Manager) *Repository {
	return &Repository{dbm: dbm}
}

// LoadPolicy reads all rules from bbolt and populates the casbin model.
func (r *Repository) LoadPolicy(model casbinmodel.Model) error {
	return r.LoadPolicyCtx(context.Background(), model)
}

// LoadPolicyCtx reads all rules from bbolt and populates the casbin model.
func (r *Repository) LoadPolicyCtx(ctx context.Context, model casbinmodel.Model) error {
	err := r.dbm.GetQuerier(ctx).Bucket(bucketPolicy).ForEach(func(k, _ []byte) error {
		var parts []string
		if err := json.Unmarshal(k, &parts); err != nil {
			slog.Warn("bbolt: malformed policy rule key", "error", err, "key", string(k))

			return nil
		}

		if len(parts) < minPartsLen {
			return nil
		}

		ptype := parts[1]
		fields := parts[2:]

		if err := persist.LoadPolicyArray(append([]string{ptype}, fields...), model); err != nil {
			slog.Warn("bbolt: invalid policy rule, skipping", "error", err, "parts", parts)

			return nil
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("load policy: %w", err)
	}

	return nil
}

// SavePolicy replaces all stored rules with the current model state.
func (r *Repository) SavePolicy(model casbinmodel.Model) error {
	return r.SavePolicyCtx(context.Background(), model)
}

// SavePolicyCtx replaces all stored rules with the current model state.
//
// The bulk delete-then-write is NOT atomic on its own — wrap in
// Manager.WithTx when calling outside a usecase transaction.
func (r *Repository) SavePolicyCtx(ctx context.Context, model casbinmodel.Model) error {
	b := r.dbm.GetQuerier(ctx).Bucket(bucketPolicy)

	var existing [][]byte
	if err := b.ForEach(func(k, _ []byte) error {
		keyCopy := make([]byte, len(k))
		copy(keyCopy, k)
		existing = append(existing, keyCopy)

		return nil
	}); err != nil {
		return fmt.Errorf("save policy: scan: %w", err)
	}

	for _, k := range existing {
		if err := b.Delete(k); err != nil {
			return fmt.Errorf("save policy: clear: %w", err)
		}
	}

	for sec, ptypes := range model {
		for ptype, assertion := range ptypes {
			for _, rule := range assertion.Policy {
				key, err := ruleKey(sec, ptype, rule)
				if err != nil {
					return err
				}

				if err := b.Put(key, []byte{}); err != nil {
					return fmt.Errorf("save policy: put: %w", err)
				}
			}
		}
	}

	return nil
}

// AddPolicy persists a single p-rule addition.
func (r *Repository) AddPolicy(sec, ptype string, rule []string) error {
	return r.AddPolicyCtx(context.Background(), sec, ptype, rule)
}

// AddPolicyCtx persists a single p-rule addition.
func (r *Repository) AddPolicyCtx(ctx context.Context, sec, ptype string, rule []string) error {
	key, err := ruleKey(sec, ptype, rule)
	if err != nil {
		return err
	}

	if err := r.dbm.GetQuerier(ctx).Bucket(bucketPolicy).Put(key, []byte{}); err != nil {
		return fmt.Errorf("add policy rule: %w", err)
	}

	return nil
}

// RemovePolicy removes a single p-rule.
func (r *Repository) RemovePolicy(sec, ptype string, rule []string) error {
	return r.RemovePolicyCtx(context.Background(), sec, ptype, rule)
}

// RemovePolicyCtx removes a single p-rule.
func (r *Repository) RemovePolicyCtx(ctx context.Context, sec, ptype string, rule []string) error {
	key, err := ruleKey(sec, ptype, rule)
	if err != nil {
		return err
	}

	if err := r.dbm.GetQuerier(ctx).Bucket(bucketPolicy).Delete(key); err != nil {
		return fmt.Errorf("remove policy rule: %w", err)
	}

	return nil
}

// RemoveFilteredPolicy removes rules matching the given filter.
func (r *Repository) RemoveFilteredPolicy(
	sec, ptype string,
	fieldIndex int,
	fieldValues ...string,
) error {
	return r.RemoveFilteredPolicyCtx(context.Background(), sec, ptype, fieldIndex, fieldValues...)
}

// RemoveFilteredPolicyCtx removes rules matching the given filter.
//
// The scan-then-delete is NOT atomic on its own — wrap in Manager.WithTx
// when concurrent writers may touch the policy bucket.
func (r *Repository) RemoveFilteredPolicyCtx(
	ctx context.Context,
	sec, ptype string,
	fieldIndex int,
	fieldValues ...string,
) error {
	b := r.dbm.GetQuerier(ctx).Bucket(bucketPolicy)

	var toDelete [][]byte

	err := b.ForEach(func(k, _ []byte) error {
		var parts []string
		if err := json.Unmarshal(k, &parts); err != nil {
			slog.Warn(
				"Casbin: malformed policy rule key during filtered removal",
				"error",
				err,
				"key",
				string(k),
			)

			return nil
		}

		if len(parts) < minPartsLen {
			return nil
		}

		if parts[0] != sec || parts[1] != ptype {
			return nil
		}

		fields := parts[2:]

		if !matchesFilter(fields, fieldIndex, fieldValues) {
			return nil
		}

		keyCopy := make([]byte, len(k))
		copy(keyCopy, k)
		toDelete = append(toDelete, keyCopy)

		return nil
	})
	if err != nil {
		return fmt.Errorf("remove filtered policy: scan: %w", err)
	}

	for _, key := range toDelete {
		if err := b.Delete(key); err != nil {
			return fmt.Errorf("remove filtered policy: delete: %w", err)
		}
	}

	return nil
}

// ListPermissionsForSubject returns all p-rules matching the given subject.
// Returns rules as [sub, dom, obj, act] (no sec/ptype prefix).
func (r *Repository) ListPermissionsForSubject(ctx context.Context, subject string) ([][]string, error) {
	var rules [][]string

	err := r.dbm.GetQuerier(ctx).Bucket(bucketPolicy).ForEach(func(k, _ []byte) error {
		var parts []string
		if err := json.Unmarshal(k, &parts); err != nil {
			slog.Warn(
				"Casbin: malformed policy rule key during list",
				"error",
				err,
				"key",
				string(k),
			)

			return nil
		}

		if len(parts) < minPartsLen {
			return nil
		}

		if parts[0] != "p" || parts[1] != "p" {
			return nil
		}

		if parts[2] == subject {
			rules = append(rules, parts[2:])
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list permissions: %w", err)
	}

	return rules, nil
}

// ruleKey encodes the rule as a JSON []string{sec, ptype, ...fields} key.
func ruleKey(sec, ptype string, rule []string) ([]byte, error) {
	const basePartsCount = 2

	parts := make([]string, 0, basePartsCount+len(rule))
	parts = append(parts, sec, ptype)
	parts = append(parts, rule...)

	key, err := json.Marshal(parts)
	if err != nil {
		return nil, fmt.Errorf("marshal rule key: %w", err)
	}

	return key, nil
}

// matchesFilter returns true when the rule fields match fieldValues starting at fieldIndex.
func matchesFilter(fields []string, fieldIndex int, fieldValues []string) bool {
	if len(fieldValues) == 0 {
		return true
	}

	for i, val := range fieldValues {
		if val == "" {
			continue
		}

		idx := fieldIndex + i
		if idx >= len(fields) {
			return false
		}

		if fields[idx] != val {
			return false
		}
	}

	return true
}

// Compile-time check that Repository implements Casbin's persist.Adapter.
// The project-defined casbin.PolicyRepository assertion lives in the casbin
// package itself to avoid an import cycle.
var _ persist.Adapter = (*Repository)(nil)
