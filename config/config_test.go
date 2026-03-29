package config

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaults(t *testing.T) {
	cfg := Defaults()
	if cfg.Settings.MaxConcurrentFetches != 4 {
		t.Errorf("MaxConcurrentFetches = %d, want 4", cfg.Settings.MaxConcurrentFetches)
	}
	if cfg.Settings.MetadataTimeoutS != 5 {
		t.Errorf("MetadataTimeoutS = %d, want 5", cfg.Settings.MetadataTimeoutS)
	}
	if cfg.Settings.CacheTTLHours != 24 {
		t.Errorf("CacheTTLHours = %d, want 24", cfg.Settings.CacheTTLHours)
	}
}

func TestLoad_Missing(t *testing.T) {
	cfg, err := Load("/nonexistent/path/config.toml")
	if err != nil {
		t.Fatalf("Load missing: %v", err)
	}
	// Should return defaults.
	if cfg.Settings.MaxConcurrentFetches != 4 {
		t.Errorf("MaxConcurrentFetches = %d, want 4", cfg.Settings.MaxConcurrentFetches)
	}
}

func TestLoad_Valid(t *testing.T) {
	toml := `
[settings]
max_concurrent_fetches = 8
metadata_timeout_s = 10
cache_ttl_hours = 48

[[server]]
name = "home"
url = "https://media.home.example.com/"
[server.auth]
type = "basic"
username = "admin"
password = "secret"
`
	path := filepath.Join(t.TempDir(), "config.toml")
	os.WriteFile(path, []byte(toml), 0o600)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Settings.MaxConcurrentFetches != 8 {
		t.Errorf("MaxConcurrentFetches = %d, want 8", cfg.Settings.MaxConcurrentFetches)
	}
	if len(cfg.Servers) != 1 {
		t.Fatalf("len(Servers) = %d, want 1", len(cfg.Servers))
	}
	srv := cfg.Servers[0]
	if srv.Name != "home" || srv.Auth.Username != "admin" {
		t.Errorf("server = %+v", srv)
	}
}

func TestLoad_InvalidTOML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.toml")
	os.WriteFile(path, []byte("this is [not valid toml @@@@"), 0o600)
	_, err := Load(path)
	if err == nil {
		t.Error("want error for invalid TOML")
	}
}

func TestFindServer(t *testing.T) {
	cfg := Config{
		Servers: []Server{
			{Name: "home", URL: "https://media.home.example.com/"},
		},
	}
	if s := cfg.FindServer("home"); s == nil || s.Name != "home" {
		t.Errorf("FindServer by name failed: %v", s)
	}
	if s := cfg.FindServer("https://media.home.example.com/"); s == nil {
		t.Errorf("FindServer by URL failed")
	}
	if s := cfg.FindServer("unknown"); s != nil {
		t.Errorf("FindServer(unknown) = %v, want nil", s)
	}
}

func TestBuildClient_Anonymous(t *testing.T) {
	client := BuildClient(nil, Defaults().Settings)
	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if client.Timeout == 0 {
		t.Error("expected non-zero timeout")
	}
}

func TestBuildClient_BasicAuth(t *testing.T) {
	var gotUser, gotPass string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser, gotPass, _ = r.BasicAuth()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	server := &Server{
		Auth: Auth{Type: "basic", Username: "user", Password: "pass"},
	}
	client := BuildClient(server, Defaults().Settings)
	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	resp.Body.Close()

	if gotUser != "user" || gotPass != "pass" {
		t.Errorf("basic auth: got user=%q pass=%q", gotUser, gotPass)
	}
}

func TestBuildClient_BearerAuth(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	server := &Server{
		Auth: Auth{Type: "bearer", Token: "my-token"},
	}
	client := BuildClient(server, Defaults().Settings)
	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	resp.Body.Close()

	if gotAuth != "Bearer my-token" {
		t.Errorf("bearer auth: got %q, want %q", gotAuth, "Bearer my-token")
	}
}

func TestBuildClient_RedirectGuard(t *testing.T) {
	// Server A redirects to server B (different host).
	srvB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srvB.Close()

	srvA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, srvB.URL+"/", http.StatusFound)
	}))
	defer srvA.Close()

	server := &Server{
		Auth: Auth{Type: "basic", Username: "u", Password: "p"},
	}
	client := BuildClient(server, Defaults().Settings)
	_, err := client.Get(srvA.URL + "/")
	if err == nil {
		t.Error("expected error for cross-host redirect, got nil")
	}
}
