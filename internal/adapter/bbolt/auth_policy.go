package bbolt

import (
	"encoding/json"
	"fmt"
	"log/slog"

	casbinmodel "github.com/casbin/casbin/v2/model"
	"github.com/casbin/casbin/v2/persist"
	bolt "go.etcd.io/bbolt"
)

const minPartsLen = 3

// PolicyRepo stores and retrieves Casbin policy rules in bbolt.
// Each rule is stored as a separate entry: key = JSON-encoded []string{sec, ptype, ...fields},
// value = empty bytes. This allows incremental AddPolicy/RemovePolicy without full rewrites.
type PolicyRepo struct {
	store *Store
}

// NewPolicyRepo creates a new PolicyRepo backed by the given Store.
func NewPolicyRepo(store *Store) *PolicyRepo {
	return &PolicyRepo{store: store}
}

// ruleKey encodes a policy rule into a bbolt key.
// The key is JSON-encoded []string{sec, ptype, field0, field1, ...}.
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

// LoadPolicy reads all rules from bbolt and populates the casbin model.
func (r *PolicyRepo) LoadPolicy(model casbinmodel.Model) error {
	err := r.store.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketAuthPolicy))

		return b.ForEach(func(k, _ []byte) error {
			var parts []string
			if err := json.Unmarshal(k, &parts); err != nil {
				slog.Warn("bbolt: malformed policy rule key", "error", err, "key", string(k))

				return nil
			}

			if len(parts) < minPartsLen {
				return nil
			}

			sec := parts[0]
			ptype := parts[1]
			fields := parts[2:]

			if err := persist.LoadPolicyArray(append([]string{ptype}, fields...), model); err != nil {
				slog.Warn("bbolt: invalid policy rule, skipping", "error", err, "parts", parts)

				return nil
			}

			_ = sec

			return nil
		})
	})
	if err != nil {
		return fmt.Errorf("load policy: %w", err)
	}

	return nil
}

// SavePolicy replaces all stored rules with the current model state.
func (r *PolicyRepo) SavePolicy(model casbinmodel.Model) error {
	err := r.store.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketAuthPolicy))

		// Delete all existing rules.
		if err := b.ForEach(func(k, _ []byte) error {
			return b.Delete(k)
		}); err != nil {
			return fmt.Errorf("clear policy bucket: %w", err)
		}

		// Write all rules from model.
		for sec, ptypes := range model {
			for ptype, assertion := range ptypes {
				for _, rule := range assertion.Policy {
					key, err := ruleKey(sec, ptype, rule)
					if err != nil {
						return err
					}

					if err := b.Put(key, []byte{}); err != nil {
						return fmt.Errorf("put policy rule: %w", err)
					}
				}
			}
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("save policy: %w", err)
	}

	return nil
}

// AddPolicy persists a single p-rule addition.
func (r *PolicyRepo) AddPolicy(sec, ptype string, rule []string) error {
	key, err := ruleKey(sec, ptype, rule)
	if err != nil {
		return err
	}

	err = r.store.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketAuthPolicy))

		return b.Put(key, []byte{})
	})
	if err != nil {
		return fmt.Errorf("add policy rule: %w", err)
	}

	return nil
}

// RemovePolicy removes a single p-rule.
func (r *PolicyRepo) RemovePolicy(sec, ptype string, rule []string) error {
	key, err := ruleKey(sec, ptype, rule)
	if err != nil {
		return err
	}

	err = r.store.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketAuthPolicy))

		return b.Delete(key)
	})
	if err != nil {
		return fmt.Errorf("remove policy rule: %w", err)
	}

	return nil
}

// RemoveFilteredPolicy removes all rules where sec and ptype match, and the rule
// fields at indices fieldIndex..fieldIndex+len(fieldValues)-1 equal fieldValues.
func (r *PolicyRepo) RemoveFilteredPolicy(sec, ptype string, fieldIndex int, fieldValues ...string) error {
	err := r.store.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketAuthPolicy))

		var toDelete [][]byte

		err := b.ForEach(func(k, _ []byte) error {
			var parts []string
			if err := json.Unmarshal(k, &parts); err != nil {
				slog.Warn("bbolt: malformed policy rule key during filtered removal", "error", err, "key", string(k))

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
			return fmt.Errorf("scan policy bucket: %w", err)
		}

		for _, key := range toDelete {
			if err := b.Delete(key); err != nil {
				return fmt.Errorf("delete filtered policy rule: %w", err)
			}
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("remove filtered policy: %w", err)
	}

	return nil
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
