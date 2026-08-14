package config

import "testing"

func boolPtr(b bool) *bool { return &b }

func TestCacheEnabled(t *testing.T) {
	tests := []struct {
		name    string
		enabled *bool
		path    string
		want    bool
	}{
		{
			name:    "nil enabled, empty path -> disabled",
			enabled: nil,
			path:    "",
			want:    false,
		},
		{
			name:    "nil enabled, non-empty path -> enabled (backward compat)",
			enabled: nil,
			path:    "/tmp/cache.db",
			want:    true,
		},
		{
			name:    "explicit true, non-empty path -> enabled",
			enabled: boolPtr(true),
			path:    "/tmp/cache.db",
			want:    true,
		},
		{
			name:    "explicit true, empty path -> enabled",
			enabled: boolPtr(true),
			path:    "",
			want:    true,
		},
		{
			name:    "explicit false, non-empty path -> disabled",
			enabled: boolPtr(false),
			path:    "/tmp/cache.db",
			want:    false,
		},
		{
			name:    "explicit false, empty path -> disabled",
			enabled: boolPtr(false),
			path:    "",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conf := Config{
				Cache: CacheConfig{
					Enabled:     tt.enabled,
					StoragePath: tt.path,
				},
			}
			got := conf.CacheEnabled()
			if got != tt.want {
				t.Errorf("CacheEnabled() = %v, want %v (enabled=%v, path=%q)",
					got, tt.want, tt.enabled, tt.path)
			}
		})
	}
}

func TestMinerUEnabled(t *testing.T) {
	tests := []struct {
		name  string
		cfg   PDFParserConfig
		want  bool
		wantOCR bool
	}{
		{"disabled", PDFParserConfig{}, false, false},
		{"enabled only", PDFParserConfig{Enabled: true}, false, false},
		{"token only", PDFParserConfig{MinerUToken: "tok"}, true, false},
		{"ocr only", PDFParserConfig{MinerUOcr: true}, true, true},
		{"token and ocr", PDFParserConfig{MinerUToken: "tok", MinerUOcr: true}, true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.MinerUEnabled(); got != tt.want {
				t.Errorf("MinerUEnabled() = %v, want %v", got, tt.want)
			}
			if got := tt.cfg.MinerUOCREnabled(); got != tt.wantOCR {
				t.Errorf("MinerUOCREnabled() = %v, want %v", got, tt.wantOCR)
			}
		})
	}
}
