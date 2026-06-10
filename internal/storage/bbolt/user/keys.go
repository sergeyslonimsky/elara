package user

import "github.com/sergeyslonimsky/elara/internal/domain"

const (
	bucketName       = "auth_users"
	bucketIdentities = "users_by_identity"
	bucketEmails     = "users_by_email"

	identitySep = "\x00"
)

// identityKey encodes a (provider, subject) pair into the secondary-index key
// shape used by the identities bucket.
func identityKey(i domain.Identity) []byte {
	return []byte(string(i.Provider) + identitySep + i.Subject)
}

// identityKeySet returns the set of identity keys (as strings — map keys must
// be hashable) for fast membership testing in reconcile flows.
func identityKeySet(identities []domain.Identity) map[string]struct{} {
	out := make(map[string]struct{}, len(identities))
	for _, i := range identities {
		out[string(identityKey(i))] = struct{}{}
	}

	return out
}
