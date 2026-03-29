package cache

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/steadyfall/fspeek/fetcher"
	_ "modernc.org/sqlite"
)

// ErrCacheMiss is returned when a cache entry is not found or has expired.
var ErrCacheMiss = errors.New("cache miss")

const (
	currentSchemaVersion = 1
)

// SQLiteCache implements Cache using SQLite with WAL mode.
type SQLiteCache struct {
	db  *sql.DB
	ttl time.Duration
}

// Open opens (or creates) the SQLite cache at path with the given TTL for entries
// that have no ETag. Pass 0 to use the default of 24 hours.
func Open(path string, ttl time.Duration) (*SQLiteCache, error) {
	if ttl == 0 {
		ttl = 24 * time.Hour
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	if _, err := db.Exec("PRAGMA journal_mode=WAL; PRAGMA busy_timeout=1000;"); err != nil {
		db.Close()
		return nil, fmt.Errorf("pragma: %w", err)
	}

	c := &SQLiteCache{db: db, ttl: ttl}
	if err := c.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return c, nil
}

func (c *SQLiteCache) migrate() error {
	_, err := c.db.Exec(`CREATE TABLE IF NOT EXISTS schema_version (version INTEGER NOT NULL)`)
	if err != nil {
		return err
	}

	var version int
	err = c.db.QueryRow(`SELECT version FROM schema_version LIMIT 1`).Scan(&version)
	if errors.Is(err, sql.ErrNoRows) {
		version = 0
	} else if err != nil {
		return err
	}

	if version < 1 {
		if err := c.migrateV1(); err != nil {
			return err
		}
	}
	return nil
}

func (c *SQLiteCache) migrateV1() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS listings (
			url          TEXT PRIMARY KEY,
			etag         TEXT,
			fetched_at   INTEGER NOT NULL,
			entries_json TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS listings_fetched_at ON listings(fetched_at)`,
		`CREATE TABLE IF NOT EXISTS metadata (
			url           TEXT PRIMARY KEY,
			etag          TEXT,
			fetched_at    INTEGER NOT NULL,
			metadata_json TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS metadata_fetched_at ON metadata(fetched_at)`,
		`DELETE FROM schema_version`,
		`INSERT INTO schema_version (version) VALUES (1)`,
	}
	for _, s := range stmts {
		if _, err := c.db.Exec(s); err != nil {
			return fmt.Errorf("migrateV1 %q: %w", s, err)
		}
	}
	return nil
}

func (c *SQLiteCache) Close() error {
	return c.db.Close()
}

// GetListing returns cached directory entries. Returns ErrCacheMiss if not
// found, expired (>TTL with no ETag), or the stored ETag is different from
// the provided currentETag (pass "" to skip ETag check).
func (c *SQLiteCache) GetListing(rawURL string) ([]Entry, string, error) {
	key := Canonicalize(rawURL)
	var etag sql.NullString
	var fetchedAt int64
	var entriesJSON string

	err := c.db.QueryRow(
		`SELECT etag, fetched_at, entries_json FROM listings WHERE url = ?`, key,
	).Scan(&etag, &fetchedAt, &entriesJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "", ErrCacheMiss
	}
	if err != nil {
		return nil, "", err
	}

	// TTL check (only if no ETag stored).
	if !etag.Valid || etag.String == "" {
		age := time.Since(time.Unix(fetchedAt, 0))
		if age > c.ttl {
			return nil, "", ErrCacheMiss
		}
	}

	var entries []Entry
	if err := json.Unmarshal([]byte(entriesJSON), &entries); err != nil {
		return nil, "", fmt.Errorf("unmarshal listings: %w", err)
	}
	etagStr := ""
	if etag.Valid {
		etagStr = etag.String
	}
	return entries, etagStr, nil
}

func (c *SQLiteCache) SetListing(rawURL string, entries []Entry, etag string) error {
	key := Canonicalize(rawURL)
	data, err := json.Marshal(entries)
	if err != nil {
		return err
	}
	_, err = c.db.Exec(
		`INSERT OR REPLACE INTO listings (url, etag, fetched_at, entries_json) VALUES (?, ?, ?, ?)`,
		key, nullString(etag), time.Now().Unix(), string(data),
	)
	return err
}

func (c *SQLiteCache) GetMetadata(rawURL string) (*fetcher.Metadata, string, error) {
	key := Canonicalize(rawURL)
	var etag sql.NullString
	var fetchedAt int64
	var metaJSON string

	err := c.db.QueryRow(
		`SELECT etag, fetched_at, metadata_json FROM metadata WHERE url = ?`, key,
	).Scan(&etag, &fetchedAt, &metaJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "", ErrCacheMiss
	}
	if err != nil {
		return nil, "", err
	}

	if !etag.Valid || etag.String == "" {
		age := time.Since(time.Unix(fetchedAt, 0))
		if age > c.ttl {
			return nil, "", ErrCacheMiss
		}
	}

	var m fetcher.Metadata
	if err := json.Unmarshal([]byte(metaJSON), &m); err != nil {
		return nil, "", fmt.Errorf("unmarshal metadata: %w", err)
	}
	etagStr := ""
	if etag.Valid {
		etagStr = etag.String
	}
	return &m, etagStr, nil
}

func (c *SQLiteCache) SetMetadata(rawURL string, m *fetcher.Metadata, etag string) error {
	key := Canonicalize(rawURL)
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	_, err = c.db.Exec(
		`INSERT OR REPLACE INTO metadata (url, etag, fetched_at, metadata_json) VALUES (?, ?, ?, ?)`,
		key, nullString(etag), time.Now().Unix(), string(data),
	)
	return err
}

func (c *SQLiteCache) Invalidate(rawURL string) error {
	key := Canonicalize(rawURL)
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	if _, err := tx.Exec(`DELETE FROM listings WHERE url = ?`, key); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM metadata WHERE url = ?`, key); err != nil {
		return err
	}
	return tx.Commit()
}

// ComputeDirSize recursively computes directory size from cached listings.
// Returns nil if the directory has no cached listing.
func (c *SQLiteCache) ComputeDirSize(rawURL string) *DirSize {
	entries, _, err := c.GetListing(rawURL)
	if err != nil {
		return nil
	}

	result := &DirSize{}
	for _, e := range entries {
		if e.IsDir {
			sub := c.ComputeDirSize(e.URL)
			if sub == nil {
				result.Partial = true
			} else {
				result.FileCount += sub.FileCount
				result.TotalSize += sub.TotalSize
				if sub.Partial {
					result.Partial = true
				}
			}
		} else {
			result.FileCount++
			if e.Size >= 0 {
				result.TotalSize += e.Size
			}
		}
	}
	return result
}

func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}
