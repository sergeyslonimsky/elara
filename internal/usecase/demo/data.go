package demo

// Namespace names used across the seeded sample data. Extracted to constants so
// the config/schema/client tables reference a single source of truth.
const (
	nsProduction = "production"
	nsStaging    = "staging"
	nsDev        = "dev"
)

type sampleNamespace struct {
	name        string
	displayName string
	description string
}

type sampleSchema struct {
	namespace   string
	pathPattern string
	jsonSchema  string
}

type sampleConfig struct {
	namespace string
	path      string
	content   string
}

var sampleNamespaces = []sampleNamespace{
	{nsProduction, "Production", "Live production configuration served to customer-facing services."},
	{nsStaging, "Staging", "Pre-production configuration mirroring production for release validation."},
	{nsDev, "Development", "Local and shared development configuration; safe to experiment here."},
}

// sampleSchemas are attached before the configs are created so config writes are
// validated against them. The production api/limits.json schema is the one the
// welcome modal points the visitor at.
var sampleSchemas = []sampleSchema{
	{
		namespace:   nsProduction,
		pathPattern: "/api/limits.json",
		jsonSchema: `{
  "type": "object",
  "required": ["rate_limit_rps", "burst", "enabled"],
  "additionalProperties": false,
  "properties": {
    "rate_limit_rps": {"type": "integer", "minimum": 1},
    "burst": {"type": "integer", "minimum": 0},
    "enabled": {"type": "boolean"}
  }
}`,
	},
	{
		namespace:   nsProduction,
		pathPattern: "/features/flags.json",
		jsonSchema: `{
  "type": "object",
  "additionalProperties": {"type": "boolean"}
}`,
	},
}

var sampleConfigs = []sampleConfig{
	{nsProduction, "/api/limits.json", `{"rate_limit_rps": 1000, "burst": 200, "enabled": true}`},
	{nsProduction, "/api/timeouts.json", `{"connect_ms": 500, "read_ms": 2000, "write_ms": 2000}`},
	{nsProduction, "/features/flags.json", `{"new_dashboard": true, "beta_search": false, "dark_mode": true}`},
	{nsProduction, "/database/pool.yaml", "max_open_conns: 50\nmax_idle_conns: 10\nconn_max_lifetime: 30m\n"},
	{nsStaging, "/api/limits.json", `{"rate_limit_rps": 200, "burst": 50, "enabled": true}`},
	{nsStaging, "/features/flags.json", `{"new_dashboard": true, "beta_search": true, "dark_mode": true}`},
	{nsStaging, "/logging/config.yaml", "level: debug\nformat: json\nsampling: false\n"},
	{nsDev, "/api/limits.json", `{"rate_limit_rps": 100, "burst": 20, "enabled": false}`},
	{nsDev, "/features/flags.json", `{"new_dashboard": true, "beta_search": true, "dark_mode": false}`},
	{nsDev, "/debug/settings.json", `{"verbose": true, "pprof_enabled": true, "trace_sampling": 1.0}`},
}
