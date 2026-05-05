package log

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

// asyncFileWriter is a non-blocking, channel-backed, daily-rotating file writer.
//
// The single run goroutine owns the file handle, so no mutex is needed for
// rotation or write. Writes that arrive when the channel is full are dropped
// so a slow disk never blocks the calling goroutine.
type asyncFileWriter struct {
	dir     string
	appName string
	ch      chan []byte
	done    chan struct{}
	file    *os.File
	curDate string
	nowFn   func() time.Time
}

func newAsyncFileWriter(dir, appName string, bufSize int) (*asyncFileWriter, error) {
	if bufSize <= 0 {
		bufSize = defaultBufSize
	}
	w := &asyncFileWriter{
		dir:     expandTilde(dir),
		appName: appName,
		ch:      make(chan []byte, bufSize),
		done:    make(chan struct{}),
		nowFn:   time.Now,
	}
	if err := w.openFile(); err != nil {
		return nil, err
	}
	go w.run()
	return w, nil
}

// Write implements zapcore.WriteSyncer. Non-blocking: drops on full channel.
func (w *asyncFileWriter) Write(p []byte) (int, error) {
	buf := make([]byte, len(p))
	copy(buf, p)
	select {
	case w.ch <- buf:
	default:
	}
	return len(p), nil
}

// Sync implements zapcore.WriteSyncer.
func (w *asyncFileWriter) Sync() error {
	if w.file != nil {
		return w.file.Sync()
	}
	return nil
}

// run owns all file I/O. It checks the calendar date on every batch and
// reopens the file when the day rolls over.
func (w *asyncFileWriter) run() {
	defer close(w.done)
	for p := range w.ch {
		if today := w.nowFn().Format(dateLayout); today != w.curDate {
			_ = w.openFile()
		}
		if w.file != nil {
			_, _ = w.file.Write(p)
		}
	}
}

func (w *asyncFileWriter) openFile() error {
	date := w.nowFn().Format(dateLayout)
	path := filepath.Join(w.dir, sanitizeAppName(w.appName)+"-"+date+".log")

	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if w.file != nil {
		_ = w.file.Close()
	}
	w.file = f
	w.curDate = date
	return nil
}

// close drains the channel, flushes, and closes the file. Safe to call once.
func (w *asyncFileWriter) close() {
	close(w.ch)
	<-w.done
	if w.file != nil {
		_ = w.file.Sync()
		_ = w.file.Close()
	}
}

const (
	dateLayout     = "2006-01-02"
	defaultBufSize = 4096
)

func expandTilde(p string) string {
	if p == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
		return p
	}
	if strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, p[2:])
		}
	}
	return p
}

// sanitizeAppName strips path separators and a few control chars so the
// AppName cannot escape LogDir or break filename parsing.
func sanitizeAppName(name string) string {
	if name == "" {
		return "app"
	}
	var b strings.Builder
	b.Grow(len(name))
	for _, r := range name {
		switch r {
		case '/', '\\', 0:
			b.WriteByte('-')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
