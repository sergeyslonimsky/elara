package user

// Detailed mock-based handler tests were retired during the RBAC delta
// refactor — handler now has no `authz` dependency (all per-target
// authorization is derived inside the usecase), so the old test fixtures
// (NewMockauthz, mocked usecase signatures pre-AuthInfo) no longer match
// the real interface.
//
// End-to-end coverage of the handler's auth-info extraction, auth-type
// feature gates, and explicit-delta payload translation lives in
// handler_integration_test.go, which runs the real handler against a real
// bbolt+Casbin stack.
//
// Reintroduce focused unit tests only when a specific handler-side branch
// gains non-trivial logic that's hard to assert from the integration suite.
