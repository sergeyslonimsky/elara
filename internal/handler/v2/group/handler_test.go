package group_test

// Detailed mock-based handler tests were retired during the RBAC delta
// refactor — they tested signatures that no longer exist (Group.Version,
// Group.Members, canonical-set Update*) and re-asserting the handler's
// translation layer against changed mocks was high churn for low value.
//
// End-to-end coverage of the handlers' authorization gates, proto↔domain
// shape, and explicit-delta semantics lives in the package's integration
// suite (handler_integration_test.go, m9_authz_integration_test.go), which
// runs the real handler over a real bbolt+Casbin stack.
//
// If unit-level handler tests become valuable again (e.g. covering a
// non-trivial converter branch), add focused cases rather than mirror the
// integration matrix.
