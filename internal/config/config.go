package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const maxConfigBytes = 64 << 10

// Config contains file locations, never secret values. No field is populated
// from an implicit environment fallback.
type Config struct {
	ListenAddress   string `json:"listen_address"`
	BaseURL         string `json:"base_url"`
	DatabaseDSNFile string `json:"database_dsn_file"`
	RootKeyFile     string `json:"root_key_file"`
	BlobRoot        string `json:"blob_root"`
	TLSCertFile     string `json:"tls_cert_file,omitempty"`
	TLSKeyFile      string `json:"tls_key_file,omitempty"`
}

func Load(path string) (Config, error) {
	var cfg Config
	f, err := os.Open(path)
	if err != nil {
		return cfg, fmt.Errorf("open config: %w", err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return Config{}, fmt.Errorf("stat config: %w", err)
	}
	if info.Size() > maxConfigBytes {
		return Config{}, errors.New("config exceeds 64 KiB")
	}

	data, err := io.ReadAll(io.LimitReader(f, maxConfigBytes+1))
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	if len(data) > maxConfigBytes {
		return Config{}, errors.New("config exceeds 64 KiB")
	}
	if err := rejectDuplicateKeys(data); err != nil {
		return Config{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return Config{}, errors.New("config contains multiple JSON values")
		}
		return Config{}, fmt.Errorf("decode trailing config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func rejectDuplicateKeys(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	start, err := decoder.Token()
	if err != nil {
		return fmt.Errorf("decode config: %w", err)
	}
	if start != json.Delim('{') {
		return errors.New("config must be a JSON object")
	}
	seen := make(map[string]bool)
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return fmt.Errorf("decode config key: %w", err)
		}
		key, ok := token.(string)
		if !ok {
			return errors.New("config key must be a string")
		}
		if seen[key] {
			return fmt.Errorf("duplicate config key %q", key)
		}
		seen[key] = true
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return fmt.Errorf("decode config value: %w", err)
		}
	}
	if _, err := decoder.Token(); err != nil {
		return fmt.Errorf("decode config: %w", err)
	}
	return nil
}

func (c Config) Validate() error {
	host, port, err := net.SplitHostPort(c.ListenAddress)
	if err != nil || port == "" {
		return errors.New("listen_address must be host:port")
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return errors.New("listen_address has an invalid port")
	}
	loopback := strings.EqualFold(host, "localhost") || (net.ParseIP(host) != nil && net.ParseIP(host).IsLoopback())
	if (c.TLSCertFile == "") != (c.TLSKeyFile == "") {
		return errors.New("tls_cert_file and tls_key_file must be provided together")
	}
	if !loopback && c.TLSCertFile == "" {
		return errors.New("non-loopback listen_address requires TLS certificate and key")
	}
	base, err := url.Parse(c.BaseURL)
	if err != nil || base.Hostname() == "" || base.User != nil || base.RawQuery != "" || base.Fragment != "" || base.Path != "" {
		return errors.New("base_url must be an origin URL without credentials, path, query or fragment")
	}
	// A remote base URL must be HTTPS even when a reverse proxy terminates TLS.
	if base.Scheme != "https" && !(loopback && base.Scheme == "http" && (strings.EqualFold(base.Hostname(), "localhost") || (net.ParseIP(base.Hostname()) != nil && net.ParseIP(base.Hostname()).IsLoopback()))) {
		return errors.New("base_url must use HTTPS except for loopback development")
	}
	for name, path := range map[string]string{
		"database_dsn_file": c.DatabaseDSNFile,
		"root_key_file":     c.RootKeyFile,
		"blob_root":         c.BlobRoot,
	} {
		if !filepath.IsAbs(path) {
			return fmt.Errorf("%s must be an absolute path", name)
		}
	}
	for name, path := range map[string]string{
		"database_dsn_file": c.DatabaseDSNFile,
		"root_key_file":     c.RootKeyFile,
		"tls_cert_file":     c.TLSCertFile,
		"tls_key_file":      c.TLSKeyFile,
	} {
		if path == "" {
			continue
		}
		if !filepath.IsAbs(path) {
			return fmt.Errorf("%s must be an absolute path", name)
		}
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("%s must name an existing regular file", name)
		}
	}
	return nil
}
