package bbolt

import (
	"context"
	"encoding/json"
	"fmt"

	bolt "go.etcd.io/bbolt"
)

const policyKey = "policy"

// PolicyRepo stores and retrieves Casbin policy rules in bbolt.
// The entire policy is stored as a single JSON value under the key "policy".
type PolicyRepo struct {
	store *Store
}

// NewPolicyRepo creates a new PolicyRepo backed by the given Store.
func NewPolicyRepo(store *Store) *PolicyRepo {
	return &PolicyRepo{store: store}
}

// Load returns all stored policy rules. Returns an empty slice if none are stored yet.
func (r *PolicyRepo) Load(_ context.Context) ([][]string, error) {
	var rules [][]string

	err := r.store.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketAuthPolicy))
		data := b.Get([]byte(policyKey))

		if data == nil {
			return nil
		}

		if err := json.Unmarshal(data, &rules); err != nil {
			return fmt.Errorf("unmarshal policy: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("load policy: %w", err)
	}

	if rules == nil {
		rules = [][]string{}
	}

	return rules, nil
}

// Save overwrites the entire stored policy with the provided rules.
func (r *PolicyRepo) Save(_ context.Context, rules [][]string) error {
	err := r.store.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketAuthPolicy))

		data, err := json.Marshal(rules)
		if err != nil {
			return fmt.Errorf("marshal policy: %w", err)
		}

		return b.Put([]byte(policyKey), data)
	})
	if err != nil {
		return fmt.Errorf("save policy: %w", err)
	}

	return nil
}
