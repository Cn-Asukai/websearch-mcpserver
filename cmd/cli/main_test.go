package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"websearch/pkg/config"
)

func TestRunInit_WritesExample(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "config.yaml")
	if err := runInit(path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(config.ExampleConfig) {
		t.Fatalf("wrote %d bytes, example is %d", len(data), len(config.ExampleConfig))
	}
	if !strings.Contains(string(data), "mode:") {
		t.Error("example config should contain mode")
	}
}

func TestRunInit_DoesNotOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("keep-me"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := runInit(path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "keep-me" {
		t.Fatalf("overwrote existing config: %q", data)
	}
}

func TestRunInit_DefaultPath(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := runInit(""); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "config.yaml")); err != nil {
		t.Fatal(err)
	}
}
