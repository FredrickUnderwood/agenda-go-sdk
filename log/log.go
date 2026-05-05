// Package log is the agenda SDK's logging surface.
//
// It wraps zap with a tiny opinionated bootstrap so that any service deployed
// by agenda can drop in two lines and get:
//
//   - rotating per-day log files at <LogDir>/<AppName>-YYYY-MM-DD.log,
//     defaulting to ./logs/ (i.e. <localPath>/logs/ when the process runs in
//     an agenda-managed clone directory),
//   - a console encoder mirroring everything to stdout (so docker logs still
//     work),
//   - non-blocking writes (a slow disk never stalls the request goroutine),
//   - environment-variable overrides for ops (AGENDA_APP_NAME / AGENDA_LOG_DIR
//     / AGENDA_LOG_LEVEL).
//
// Typical use:
//
//	func main() {
//	    _ = log.Init(log.Config{AppName: "checkout"})
//	    defer log.Shutdown()
//	    log.L().Info("ready", zap.Int("port", 8080))
//	}
package log

import (
	"context"
	"os"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	mu       sync.RWMutex
	logger   *zap.Logger
	sugar    *zap.SugaredLogger
	writer   *asyncFileWriter
	inited   bool
	fallback = zap.NewNop()
)

// Init configures the global logger. It is safe to call exactly once at
// program startup; subsequent calls return nil without re-initializing
// (so libraries calling Init defensively do not clobber the host program's
// configuration).
//
// Init returns an error only if the file sink is requested but the log
// directory cannot be created. In that case the global logger is left
// pointing at a console-only logger so the caller can still emit messages.
func Init(cfg Config) error {
	mu.Lock()
	defer mu.Unlock()
	if inited {
		return nil
	}

	cfg = cfg.withDefaults()
	level := parseLevel(cfg.Level)

	encCfg := zap.NewProductionEncoderConfig()
	encCfg.TimeKey = "time"
	encCfg.EncodeTime = zapcore.ISO8601TimeEncoder

	cores := make([]zapcore.Core, 0, 2)
	if cfg.Console == nil || *cfg.Console {
		cores = append(cores, zapcore.NewCore(
			zapcore.NewConsoleEncoder(encCfg),
			zapcore.AddSync(os.Stdout),
			level,
		))
	}

	var fileErr error
	if !cfg.DisableFile {
		w, err := newAsyncFileWriter(cfg.LogDir, cfg.AppName, cfg.BufSize)
		if err != nil {
			fileErr = err
		} else {
			writer = w
			cores = append(cores, zapcore.NewCore(
				zapcore.NewJSONEncoder(encCfg),
				w,
				level,
			))
		}
	}

	if len(cores) == 0 {
		cores = append(cores, zapcore.NewNopCore())
	}

	logger = zap.New(
		zapcore.NewTee(cores...),
		zap.AddCaller(),
		zap.AddCallerSkip(1),
		zap.AddStacktrace(zap.ErrorLevel),
	)
	sugar = logger.Sugar()
	inited = true
	return fileErr
}

// Shutdown drains the file writer's buffer and syncs zap. Always pair with
// Init via defer in main.
func Shutdown() {
	mu.Lock()
	defer mu.Unlock()
	if writer != nil {
		writer.close()
		writer = nil
	}
	if logger != nil {
		_ = logger.Sync()
	}
	inited = false
	logger = nil
	sugar = nil
}

// L returns the global *zap.Logger. Auto-initializes with defaults when the
// caller forgot to call Init, so library code can log without coordinating
// startup. The returned logger has its caller-skip set so the file:line in
// log lines points at the call site, not into this package.
func L() *zap.Logger {
	mu.RLock()
	if logger != nil {
		l := logger
		mu.RUnlock()
		return l
	}
	mu.RUnlock()
	_ = Init(Config{})
	mu.RLock()
	defer mu.RUnlock()
	if logger == nil {
		return fallback
	}
	return logger
}

// S returns the global *zap.SugaredLogger.
func S() *zap.SugaredLogger {
	mu.RLock()
	if sugar != nil {
		s := sugar
		mu.RUnlock()
		return s
	}
	mu.RUnlock()
	_ = Init(Config{})
	mu.RLock()
	defer mu.RUnlock()
	if sugar == nil {
		return fallback.Sugar()
	}
	return sugar
}

// Convenience wrappers. The ctx argument is currently unused but reserved
// for the upcoming trace package — once trace lands, these functions will
// automatically attach trace_id / span_id fields without callers having to
// change anything.

func Debug(_ context.Context, msg string, fields ...zap.Field) {
	L().Debug(msg, fields...)
}

func Info(_ context.Context, msg string, fields ...zap.Field) {
	L().Info(msg, fields...)
}

func Warn(_ context.Context, msg string, fields ...zap.Field) {
	L().Warn(msg, fields...)
}

func Error(_ context.Context, msg string, fields ...zap.Field) {
	L().Error(msg, fields...)
}

func parseLevel(s string) zapcore.Level {
	switch s {
	case "debug":
		return zap.DebugLevel
	case "warn":
		return zap.WarnLevel
	case "error":
		return zap.ErrorLevel
	default:
		return zap.InfoLevel
	}
}
