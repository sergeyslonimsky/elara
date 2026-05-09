package auth

// Role constants for RBAC policy assignments.
const (
	RoleAdmin  = "admin"
	RoleWriter = "writer"
	RoleReader = "reader"
)

// Object constants for RBAC resource identification.
const (
	ObjectAll       = "*"
	ObjectConfig    = "config"
	ObjectNamespace = "namespace"
	ObjectToken     = "token"
	ObjectClient    = "client"
	ObjectDashboard = "dashboard"
	ObjectUser      = "user"
	ObjectPolicy    = "policy"
	ObjectWebhook   = "webhook"
	ObjectSchema    = "schema"
	ObjectTransfer  = "transfer"
)

// Action constants for RBAC permission checks.
const (
	ActionAll   = "*"
	ActionRead  = "read"
	ActionWrite = "write"
)
