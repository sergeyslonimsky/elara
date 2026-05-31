package authctx

// Claims carries identity attributes injected into request context by the
// etcd v3 token interceptor (service-credential auth path). The user/session
// auth path uses WithSession / UserFromContext instead.
type Claims struct {
	Email      string
	Name       string
	Namespaces []string
	Role       string
}
