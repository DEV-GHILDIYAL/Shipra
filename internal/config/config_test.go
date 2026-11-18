package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T) Config {
	t.Helper()
	dir := t.TempDir()
	dsn := filepath.Join(dir, "dsn")
	key := filepath.Join(dir, "key")
	for _, path := range []string{dsn, key} {
		if err := os.WriteFile(path, []byte("fixture"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return Config{
		ListenAddress:   "127.0.0.1:8080",
		BaseURL:         "http://127.0.0.1:8080",
		DatabaseDSNFile: dsn,
		RootKeyFile:     key,
		BlobRoot:        filepath.Join(dir, "blobs"),
	}
}

func TestValidate(t *testing.T) {
	valid := fixture(t)
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid fixture: %v", err)
	}
	tests := []struct {
		name string
		edit func(*Config)
		want string
	}{
		{"public without TLS", func(c *Config) { c.ListenAddress = "0.0.0.0:8080" }, "requires TLS"},
		{"zero port", func(c *Config) { c.ListenAddress = "127.0.0.1:0" }, "invalid port"},
		{"remote HTTP", func(c *Config) { c.BaseURL = "http://example.com" }, "HTTPS"},
		{"URL credentials", func(c *Config) { c.BaseURL = "https://user:pass@example.com" }, "origin URL"},
		{"URL path", func(c *Config) { c.BaseURL = "https://example.com/path" }, "origin URL"},
		{"missing key", func(c *Config) { c.RootKeyFile = filepath.Join(t.TempDir(), "missing") }, "root_key_file"},
		{"relative blob", func(c *Config) { c.BlobRoot = "blobs" }, "blob_root"},
		{"TLS pair", func(c *Config) { c.TLSCertFile = c.RootKeyFile }, "provided together"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := valid
			tt.edit(&c)
			if err := c.Validate(); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q error, got %v", tt.want, err)
			}
		})
	}
}

func TestLoadRejectsUnknownFieldsAndOversize(t *testing.T) {
	c := fixture(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err != nil {
		t.Fatalf("valid load: %v", err)
	}
	for _, input := range []string{string(data[:len(data)-1]) + `,"unexpected":true}`, string(data) + `{}`} {
		if err := os.WriteFile(path, []byte(input), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(path); err == nil {
			t.Fatalf("accepted invalid input: %s", input)
		}
	}
	if err := os.WriteFile(path, []byte(strings.Repeat(" ", maxConfigBytes+1)), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("expected size limit, got %v", err)
	}
}

func TestLoadRejectsDuplicateKey(t *testing.T) {
	c := fixture(t)
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	data = append(data[:len(data)-1], []byte(",\"listen_address\":\"0.0.0.0:8080\"}")...)
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("expected duplicate key rejection, got %v", err)
	}
}
