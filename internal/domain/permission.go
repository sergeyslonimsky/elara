package domain

type Permission struct {
	Object string
	Action string
	Domain string
}

type DomainSet struct {
	Wildcard bool
	Explicit map[string]struct{}
}

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
