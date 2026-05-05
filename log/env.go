package log

import "os"

// Environment variable names recognized by the SDK. They act as fallbacks
// when the matching Config field is empty, so that an operator can override
// behaviour at deploy time without touching application code.
const (
	EnvAppName  = "AGENDA_APP_NAME"
	EnvLogDir   = "AGENDA_LOG_DIR"
	EnvLogLevel = "AGENDA_LOG_LEVEL"
)

// applyEnv fills empty Config fields from environment variables. It never
// overrides values the caller explicitly set on Config.
func applyEnv(c *Config) {
	if c.AppName == "" {
		if v := os.Getenv(EnvAppName); v != "" {
			c.AppName = v
		}
	}
	if c.LogDir == "" {
		if v := os.Getenv(EnvLogDir); v != "" {
			c.LogDir = v
		}
	}
	if c.Level == "" {
		if v := os.Getenv(EnvLogLevel); v != "" {
			c.Level = v
		}
	}
}
