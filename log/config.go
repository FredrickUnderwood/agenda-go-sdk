package log

// Config controls how the SDK initializes the global logger.
//
// Field precedence is (highest first):
//  1. Explicit value on the Config passed to Init
//  2. Matching AGENDA_* environment variable (see env.go)
//  3. Built-in default
//
// AppName is the only field a typical caller has to set. Everything else
// has a sensible default tuned for "drop the SDK in, get logs in ./logs/".
type Config struct {
	// AppName becomes the filename prefix, e.g. "agenda-demo-2026-05-04.log".
	// Required, but if left empty Init reads AGENDA_APP_NAME and finally
	// falls back to "app".
	AppName string

	// LogDir is where rotating log files are written. Tilde-prefix paths
	// (~ or ~/...) are expanded to the caller's HOME. Default: "./logs".
	LogDir string

	// Level is one of: debug | info | warn | error. Default: "info".
	Level string

	// Console toggles a duplicate console encoder writing to stdout.
	// The pointer lets callers distinguish "I want it off" (false) from
	// "I didn't say" (nil → default true). Most users leave this nil.
	Console *bool

	// BufSize is the async file-writer's channel capacity. When the channel
	// is full, writes are dropped (never block the caller). Default: 4096.
	BufSize int

	// DisableFile turns off the file sink entirely (useful in tests or when
	// the caller only wants stdout). Default: false.
	DisableFile bool
}

const (
	defaultLogDir = "./logs"
	defaultLevel  = "info"
)

// withDefaults returns a copy of cfg with env-var fallbacks and built-in
// defaults applied. The receiver is not mutated.
func (c Config) withDefaults() Config {
	out := c
	applyEnv(&out)

	if out.AppName == "" {
		out.AppName = "app"
	}
	if out.LogDir == "" {
		out.LogDir = defaultLogDir
	}
	if out.Level == "" {
		out.Level = defaultLevel
	}
	if out.BufSize <= 0 {
		out.BufSize = defaultBufSize
	}
	if out.Console == nil {
		t := true
		out.Console = &t
	}
	return out
}
