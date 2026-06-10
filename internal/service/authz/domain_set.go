package authz

// DomainSet is the result of PDP.EffectiveDomains: the set of domains a
// subject has access to for a given (object, action) pair. Either Wildcard
// covers everything, or Explicit lists the named domains.
//
// Invariant for consumers: when Wildcard is true, Explicit MUST be ignored —
// wildcard subsumes any explicit entries. Repo filters and PDP checks should
// branch on Wildcard first.
type DomainSet struct {
	Wildcard bool
	Explicit map[string]struct{}
}

// NewDomainSet builds a DomainSet from explicit domain names. A "*" entry
// sets Wildcard=true; any other entries are still recorded in Explicit so
// callers can inspect them if needed.
func NewDomainSet(explicit ...string) DomainSet {
	ds := DomainSet{
		Explicit: make(map[string]struct{}),
	}

	for _, d := range explicit {
		if d == "*" {
			ds.Wildcard = true

			continue
		}

		ds.Explicit[d] = struct{}{}
	}

	return ds
}

func (ds DomainSet) Contains(domain string) bool {
	if ds.Wildcard {
		return true
	}

	_, ok := ds.Explicit[domain]

	return ok
}

func (ds DomainSet) IsEmpty() bool {
	return !ds.Wildcard && len(ds.Explicit) == 0
}
