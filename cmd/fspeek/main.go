// Command fspeek is a TUI browser for remote HTTP file servers that extracts
// media metadata via HTTP range requests without downloading full files.
package main

import (
	"flag"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/steadyfall/fspeek/cache"
	"github.com/steadyfall/fspeek/config"
	"github.com/steadyfall/fspeek/fetcher"
	"github.com/steadyfall/fspeek/parser"
	"github.com/steadyfall/fspeek/ui"
)

var version = "0.1.1.0"

func main() {
	var (
		flagURL     = flag.String("url", "", "Base URL of the remote file server to browse")
		flagServer  = flag.String("server", "", "Named server from config file")
		flagConfig  = flag.String("config", "", "Path to config file (default: ~/.config/fspeek/config.toml)")
		flagNoCache = flag.Bool("no-cache", false, "Disable cache for this session")
		flagBytes   = flag.Bool("bytes", false, "Display sizes in bytes instead of human-readable")
		flagVersion = flag.Bool("version", false, "Print version and exit")
	)
	flag.Parse()

	if *flagVersion {
		fmt.Println("fspeek", version)
		os.Exit(0)
	}

	// --- Load config ---
	cfgPath := *flagConfig
	if cfgPath == "" {
		cfgPath = defaultConfigPath()
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fspeek: config: %v\n", err)
		os.Exit(1)
	}

	// --- Resolve target URL and server ---
	rootURL := *flagURL
	var srv *config.Server
	if *flagServer != "" {
		srv = cfg.FindServer(*flagServer)
		if srv == nil {
			fmt.Fprintf(os.Stderr, "fspeek: server %q not found in config\n", *flagServer)
			os.Exit(1)
		}
		if rootURL == "" {
			rootURL = srv.URL
		}
	}
	if rootURL == "" {
		fmt.Fprintln(os.Stderr, "fspeek: --url or --server required")
		flag.Usage()
		os.Exit(1)
	}

	// Guard: refuse to send named-server credentials to a different host.
	if srv != nil && *flagURL != "" {
		srvU, e1 := url.Parse(srv.URL)
		rootU, e2 := url.Parse(rootURL)
		if e1 == nil && e2 == nil && srvU.Host != rootU.Host {
			fmt.Fprintf(os.Stderr,
				"fspeek: --url host %q differs from server %q host %q — credentials would leak\n",
				rootU.Host, *flagServer, srvU.Host)
			os.Exit(1)
		}
	}

	// Ensure URL ends with / (it's a directory).
	if len(rootURL) > 0 && rootURL[len(rootURL)-1] != '/' {
		rootURL += "/"
	}

	// --- Build HTTP client ---
	client := config.BuildClient(srv, cfg.Settings)

	// --- Open (or skip) SQLite cache ---
	var c cache.Cache
	if !*flagNoCache {
		dbPath := defaultCachePath()
		if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
			fmt.Fprintf(os.Stderr, "fspeek: cache dir: %v\n", err)
			os.Exit(1)
		}
		sc, err := cache.Open(dbPath, time.Duration(cfg.Settings.CacheTTLHours)*time.Hour)
		if err != nil {
			// Non-fatal: run without cache.
			fmt.Fprintf(os.Stderr, "fspeek: cache unavailable (%v) — running without cache\n", err)
		} else {
			c = sc
			defer sc.Close()
		}
	}

	// --- Wire lister cascade and fetchers ---
	lister := parser.NewCascade(
		parser.NginxJSONLister{},
		parser.NginxHTMLLister{},
		parser.GenericHrefLister{},
	)

	fetchers := []fetcher.MetadataFetcher{
		fetcher.MP4Fetcher{},
		fetcher.MKVFetcher{},
		fetcher.SRTFetcher{},
	}

	// --- Build and run the TUI ---
	model := ui.New(rootURL, ui.Options{
		Cache:      c,
		Client:     client,
		Lister:     lister,
		Fetchers:   fetchers,
		MaxFetches: cfg.Settings.MaxConcurrentFetches,
		ShowBytes:  *flagBytes,
	})

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "fspeek: %v\n", err)
		os.Exit(1)
	}
}

// defaultConfigPath returns the OS-appropriate default config path.
func defaultConfigPath() string {
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".config", "fspeek", "config.toml")
	}
	return "config.toml"
}

// defaultCachePath returns the OS-appropriate default cache DB path.
func defaultCachePath() string {
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".cache", "fspeek", "cache.db")
	}
	return "cache.db"
}
