package log

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"
)

// resetGlobals tears down package state between subtests so that each test
// starts from a clean slate. Tests must call this in t.Cleanup.
func resetGlobals(t *testing.T) {
	t.Helper()
	Shutdown()
}

func TestConfig_withDefaults(t *testing.T) {
	t.Setenv(EnvAppName, "")
	t.Setenv(EnvLogDir, "")
	t.Setenv(EnvLogLevel, "")

	tests := []struct {
		name    string
		in      Config
		wantApp string
		wantDir string
		wantLvl string
		wantBuf int
	}{
		{
			name:    "all empty falls back to built-ins",
			in:      Config{},
			wantApp: "app",
			wantDir: defaultLogDir,
			wantLvl: defaultLevel,
			wantBuf: defaultBufSize,
		},
		{
			name:    "explicit values win",
			in:      Config{AppName: "svc", LogDir: "/tmp/x", Level: "debug", BufSize: 64},
			wantApp: "svc",
			wantDir: "/tmp/x",
			wantLvl: "debug",
			wantBuf: 64,
		},
		{
			name:    "negative bufsize coerced to default",
			in:      Config{AppName: "svc", BufSize: -1},
			wantApp: "svc",
			wantDir: defaultLogDir,
			wantLvl: defaultLevel,
			wantBuf: defaultBufSize,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.in.withDefaults()
			if got.AppName != tt.wantApp {
				t.Errorf("AppName: got %q, want %q", got.AppName, tt.wantApp)
			}
			if got.LogDir != tt.wantDir {
				t.Errorf("LogDir: got %q, want %q", got.LogDir, tt.wantDir)
			}
			if got.Level != tt.wantLvl {
				t.Errorf("Level: got %q, want %q", got.Level, tt.wantLvl)
			}
			if got.BufSize != tt.wantBuf {
				t.Errorf("BufSize: got %d, want %d", got.BufSize, tt.wantBuf)
			}
			if got.Console == nil || !*got.Console {
				t.Errorf("Console: want default true, got %v", got.Console)
			}
		})
	}
}

func TestEnv_filledWhenConfigEmpty(t *testing.T) {
	t.Setenv(EnvAppName, "envapp")
	t.Setenv(EnvLogDir, "/var/envdir")
	t.Setenv(EnvLogLevel, "warn")

	got := Config{}.withDefaults()
	if got.AppName != "envapp" {
		t.Errorf("AppName: got %q, want envapp", got.AppName)
	}
	if got.LogDir != "/var/envdir" {
		t.Errorf("LogDir: got %q, want /var/envdir", got.LogDir)
	}
	if got.Level != "warn" {
		t.Errorf("Level: got %q, want warn", got.Level)
	}
}

func TestEnv_doesNotOverrideExplicit(t *testing.T) {
	t.Setenv(EnvAppName, "envapp")
	t.Setenv(EnvLogDir, "/var/envdir")
	t.Setenv(EnvLogLevel, "warn")

	got := Config{AppName: "explicit", LogDir: "/explicit", Level: "error"}.withDefaults()
	if got.AppName != "explicit" {
		t.Errorf("AppName: explicit value should win, got %q", got.AppName)
	}
	if got.LogDir != "/explicit" {
		t.Errorf("LogDir: explicit value should win, got %q", got.LogDir)
	}
	if got.Level != "error" {
		t.Errorf("Level: explicit value should win, got %q", got.Level)
	}
}

func TestParseLevel(t *testing.T) {
	tests := map[string]zap.AtomicLevel{
		"debug":   zap.NewAtomicLevelAt(zap.DebugLevel),
		"info":    zap.NewAtomicLevelAt(zap.InfoLevel),
		"warn":    zap.NewAtomicLevelAt(zap.WarnLevel),
		"error":   zap.NewAtomicLevelAt(zap.ErrorLevel),
		"unknown": zap.NewAtomicLevelAt(zap.InfoLevel),
	}
	for in, want := range tests {
		if got := parseLevel(in); got != want.Level() {
			t.Errorf("parseLevel(%q): got %v, want %v", in, got, want.Level())
		}
	}
}

func TestSanitizeAppName(t *testing.T) {
	tests := map[string]string{
		"":            "app",
		"clean-name":  "clean-name",
		"a/b":         "a-b",
		"a\\b":        "a-b",
		"path/to/svc": "path-to-svc",
	}
	for in, want := range tests {
		if got := sanitizeAppName(in); got != want {
			t.Errorf("sanitizeAppName(%q): got %q, want %q", in, got, want)
		}
	}
}

func TestExpandTilde(t *testing.T) {
	home, _ := os.UserHomeDir()
	tests := map[string]string{
		"~":         home,
		"~/foo":     filepath.Join(home, "foo"),
		"/abs/path": "/abs/path",
		"./rel":     "./rel",
		"":          "",
	}
	for in, want := range tests {
		if got := expandTilde(in); got != want {
			t.Errorf("expandTilde(%q): got %q, want %q", in, got, want)
		}
	}
}

func TestInit_writesToFile(t *testing.T) {
	t.Cleanup(func() { resetGlobals(t) })

	dir := t.TempDir()
	if err := Init(Config{AppName: "testsvc", LogDir: dir, DisableFile: false}); err != nil {
		t.Fatalf("Init: %v", err)
	}
	Info(context.Background(), "hello", zap.String("k", "v"))
	Shutdown()

	files, err := filepath.Glob(filepath.Join(dir, "testsvc-*.log"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 log file, got %d: %v", len(files), files)
	}
	data, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(data), "hello") {
		t.Errorf("log file missing message: %q", data)
	}
	if !strings.Contains(string(data), "\"k\":\"v\"") {
		t.Errorf("log file missing field: %q", data)
	}
}

func TestInit_idempotent(t *testing.T) {
	t.Cleanup(func() { resetGlobals(t) })

	dir := t.TempDir()
	if err := Init(Config{AppName: "first", LogDir: dir}); err != nil {
		t.Fatalf("first Init: %v", err)
	}
	first := L()
	if err := Init(Config{AppName: "second", LogDir: t.TempDir()}); err != nil {
		t.Fatalf("second Init: %v", err)
	}
	second := L()
	if first != second {
		t.Errorf("Init should be idempotent — second call returned a new logger")
	}
}

func TestL_autoInitWhenNotCalled(t *testing.T) {
	t.Cleanup(func() { resetGlobals(t) })

	if got := L(); got == nil {
		t.Fatal("L() returned nil before Init")
	}
}

func TestAsyncFileWriter_rotatesOnDayRollover(t *testing.T) {
	dir := t.TempDir()
	w, err := newAsyncFileWriter(dir, "rot", 16)
	if err != nil {
		t.Fatalf("newAsyncFileWriter: %v", err)
	}

	// atomic.Pointer keeps the test free of data races on the simulated clock,
	// since the writer's run goroutine reads nowFn concurrently with the test.
	day1 := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)
	day2 := day1.Add(24 * time.Hour)
	var clock atomic.Pointer[time.Time]
	clock.Store(&day1)
	w.nowFn = func() time.Time { return *clock.Load() }
	if err := w.openFile(); err != nil {
		t.Fatalf("openFile day1: %v", err)
	}

	if _, err := w.Write([]byte("day1\n")); err != nil {
		t.Fatalf("write day1: %v", err)
	}

	day1File := filepath.Join(dir, "rot-"+day1.Format(dateLayout)+".log")
	if !waitForNonEmpty(t, day1File, 2*time.Second) {
		t.Fatalf("day1 log file empty after 2s; run goroutine may be stuck")
	}
	clock.Store(&day2)

	if _, err := w.Write([]byte("day2\n")); err != nil {
		t.Fatalf("write day2: %v", err)
	}

	w.close()

	files, err := filepath.Glob(filepath.Join(dir, "rot-*.log"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 rotated files, got %d: %v", len(files), files)
	}

	var sawDay1, sawDay2 bool
	for _, f := range files {
		data, _ := os.ReadFile(f)
		if strings.Contains(string(data), "day1") {
			sawDay1 = true
		}
		if strings.Contains(string(data), "day2") {
			sawDay2 = true
		}
	}
	if !sawDay1 || !sawDay2 {
		t.Errorf("missing rotated content: day1=%v day2=%v files=%v", sawDay1, sawDay2, files)
	}
}

func TestAsyncFileWriter_drainsOnClose(t *testing.T) {
	dir := t.TempDir()
	w, err := newAsyncFileWriter(dir, "drain", 64)
	if err != nil {
		t.Fatalf("newAsyncFileWriter: %v", err)
	}

	const n = 50
	for i := 0; i < n; i++ {
		if _, err := w.Write([]byte("line\n")); err != nil {
			t.Fatalf("write %d: %v", i, err)
		}
	}
	w.close()

	files, _ := filepath.Glob(filepath.Join(dir, "drain-*.log"))
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %v", files)
	}
	data, _ := os.ReadFile(files[0])
	got := strings.Count(string(data), "line\n")
	if got != n {
		t.Errorf("expected %d lines drained, got %d", n, got)
	}
}

func TestAsyncFileWriter_concurrentWrites(t *testing.T) {
	dir := t.TempDir()
	w, err := newAsyncFileWriter(dir, "race", 4096)
	if err != nil {
		t.Fatalf("newAsyncFileWriter: %v", err)
	}

	var wg sync.WaitGroup
	const goroutines, perG = 20, 100
	var written atomic.Int64
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perG; i++ {
				if _, err := w.Write([]byte("x\n")); err != nil {
					t.Errorf("Write: %v", err)
					return
				}
				written.Add(1)
			}
		}()
	}
	wg.Wait()
	w.close()

	if written.Load() != goroutines*perG {
		t.Errorf("expected %d writes, recorded %d", goroutines*perG, written.Load())
	}
}

func TestAsyncFileWriter_dropsWhenChannelFull(t *testing.T) {
	dir := t.TempDir()
	w, err := newAsyncFileWriter(dir, "drop", 1)
	if err != nil {
		t.Fatalf("newAsyncFileWriter: %v", err)
	}

	for i := 0; i < 1000; i++ {
		if _, err := w.Write([]byte("x")); err != nil {
			t.Fatalf("Write returned error (should drop, not error): %v", err)
		}
	}
	w.close()
}

// waitForNonEmpty polls path every 10ms until its file size is > 0 or timeout
// elapses. Returns true if the file became non-empty in time.
func waitForNonEmpty(t *testing.T, path string, timeout time.Duration) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fi, err := os.Stat(path); err == nil && fi.Size() > 0 {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

// TestConvenienceWrappers exercises the level-specific helpers and S() so
// they are not silently broken. Content assertions are minimal — zap itself
// is well-tested; we only care that our wiring delivers the message.
func TestConvenienceWrappers(t *testing.T) {
	t.Cleanup(func() { resetGlobals(t) })

	dir := t.TempDir()
	if err := Init(Config{AppName: "wrap", LogDir: dir, Level: "debug"}); err != nil {
		t.Fatalf("Init: %v", err)
	}

	ctx := context.Background()
	Debug(ctx, "dbg-msg")
	Info(ctx, "inf-msg")
	Warn(ctx, "wrn-msg")
	Error(ctx, "err-msg")
	S().Infow("sugar-msg", "k", "v")

	Shutdown()

	files, _ := filepath.Glob(filepath.Join(dir, "wrap-*.log"))
	if len(files) != 1 {
		t.Fatalf("expected 1 log file, got %v", files)
	}
	data, _ := os.ReadFile(files[0])
	for _, want := range []string{"dbg-msg", "inf-msg", "wrn-msg", "err-msg", "sugar-msg"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("log file missing %q: %s", want, data)
		}
	}
}

func TestInit_disableFile(t *testing.T) {
	t.Cleanup(func() { resetGlobals(t) })

	dir := t.TempDir()
	if err := Init(Config{AppName: "nofile", LogDir: dir, DisableFile: true}); err != nil {
		t.Fatalf("Init: %v", err)
	}
	Info(context.Background(), "hi")
	Shutdown()

	files, _ := filepath.Glob(filepath.Join(dir, "*.log"))
	if len(files) != 0 {
		t.Errorf("DisableFile=true should not create log files, got %v", files)
	}
}
