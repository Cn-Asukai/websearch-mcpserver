package log

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"websearch/pkg/config"
)

func TestNewLoggerTo_DoesNotWriteStdout(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = old }()

	var buf bytes.Buffer
	NewLoggerTo(&buf, "", config.LogConfig{})
	Info("stdio-safe")

	_ = w.Close()
	os.Stdout = old
	dumped, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(dumped, []byte("stdio-safe")) {
		t.Fatalf("console logger wrote to stdout: %s", dumped)
	}
	if !bytes.Contains(buf.Bytes(), []byte("stdio-safe")) {
		t.Fatalf("expected log in console writer, got %q", buf.Bytes())
	}
}

func TestNewLoggerTo_NilConsoleNoPanic(t *testing.T) {
	NewLoggerTo(nil, "", config.LogConfig{})
	Info("discard-ok")
	SetLoggerLevel("debug")
	Debug("also-ok")
}

func TestNewLoggerTo_WritesFile(t *testing.T) {
	dir := t.TempDir()
	NewLoggerTo(io.Discard, dir, config.LogConfig{MaxSize: 1, MaxAge: 1})
	Info("file-line")
	p := filepath.Join(dir, "websearch.log")
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte("file-line")) {
		t.Fatalf("log file missing line: %s", data)
	}
	if err := CloseFile(); err != nil {
		t.Fatal(err)
	}
}

func TestSetLoggerLevel_NilSafe(t *testing.T) {
	defaultlog = nil
	SetLoggerLevel("debug")
	Info("no-panic")
}
