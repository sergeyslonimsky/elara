package domain

type Permission struct {
	Object Object
	Action Action
	Domain string
}
