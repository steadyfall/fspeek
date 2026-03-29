// Package config loads fspeek configuration from a TOML file and builds
// per-server http.Client instances with auth and redirect-guard middleware.
package config

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/BurntSushi/toml"
)

// Config holds the full parsed configuration.
type Config struct {
	Settings Settings `toml:"settings"`
	Servers  []Server `toml:"server"`
}

// Settings contains global operational parameters.
type Settings struct {
	MaxConcurrentFetches int `toml:"max_concurrent_fetches"`
	MetadataTimeoutS     int `toml:"metadata_timeout_s"`
	CacheTTLHours        int `toml:"cache_ttl_hours"`
}

// Server describes a remote file server and its authentication.
type Server struct {
	Name string `toml:"name"`
	URL  string `toml:"url"`
	Auth Auth   `toml:"auth"`
}

// Auth holds authentication parameters for a server.
type Auth struct {
	Type     string `toml:"type"`     // "basic", "bearer", or "" for anonymous
	Username string `toml:"username"` // for type=basic
	Password string `toml:"password"` // for type=basic
	Token    string `toml:"token"`    // for type=bearer
}

// Defaults returns a Config populated with sensible defaults.
func Defaults() Config {
	return Config{
		Settings: Settings{
			MaxConcurrentFetches: 4,
			MetadataTimeoutS:     5,
			CacheTTLHours:        24,
		},
	}
}

// Load reads a TOML config file from path, merging over defaults.
// If the file does not exist, the default config is returned without error.
func Load(path string) (Config, error) {
	cfg := Defaults()
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return cfg, fmt.Errorf("config: parse %q: %w", path, err)
	}
	// Apply defaults for zero values after decode.
	if cfg.Settings.MaxConcurrentFetches == 0 {
		cfg.Settings.MaxConcurrentFetches = 4
	}
	if cfg.Settings.MetadataTimeoutS == 0 {
		cfg.Settings.MetadataTimeoutS = 5
	}
	if cfg.Settings.CacheTTLHours == 0 {
		cfg.Settings.CacheTTLHours = 24
	}
	return cfg, nil
}

// FindServer returns the first server whose Name or URL matches key.
// Returns nil if not found.
func (c Config) FindServer(key string) *Server {
	for i := range c.Servers {
		if c.Servers[i].Name == key || c.Servers[i].URL == key {
			return &c.Servers[i]
		}
	}
	return nil
}

// BuildClient constructs an http.Client for the given server config (or for
// anonymous access if srv is nil). The client:
//   - Applies Basic or Bearer auth via a RoundTripper wrapper.
//   - Uses a CheckRedirect guard that refuses redirects changing the host,
//     preventing credential leakage.
//   - Uses the global settings for timeout.
func BuildClient(srv *Server, s Settings) *http.Client {
	timeout := time.Duration(s.MetadataTimeoutS) * time.Second
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	var transport http.RoundTripper = http.DefaultTransport
	if srv != nil {
		switch srv.Auth.Type {
		case "basic":
			transport = &basicAuthTransport{
				username: srv.Auth.Username,
				password: srv.Auth.Password,
				base:     http.DefaultTransport,
			}
		case "bearer":
			transport = &bearerAuthTransport{
				token: srv.Auth.Token,
				base:  http.DefaultTransport,
			}
		}
	}

	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) == 0 {
				return nil
			}
			// Refuse redirect if it crosses to a different host.
			if req.URL.Host != via[0].URL.Host {
				return fmt.Errorf("redirect to different host %q blocked (credential leak prevention)", req.URL.Host)
			}
			if len(via) >= 10 {
				return errors.New("too many redirects")
			}
			return nil
		},
	}
}

// basicAuthTransport adds HTTP Basic auth to every request.
type basicAuthTransport struct {
	username, password string
	base               http.RoundTripper
}

func (t *basicAuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r := req.Clone(req.Context())
	r.SetBasicAuth(t.username, t.password)
	return t.base.RoundTrip(r)
}

// bearerAuthTransport adds an Authorization: Bearer header to every request.
type bearerAuthTransport struct {
	token string
	base  http.RoundTripper
}

func (t *bearerAuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r := req.Clone(req.Context())
	r.Header.Set("Authorization", "Bearer "+t.token)
	return t.base.RoundTrip(r)
}
