# TODOS

Items explicitly deferred from the eng review. Each has context for whoever picks it up.

---

## TODO-1: goreleaser config + GitHub Actions CI/CD

**What:** Create `.goreleaser.yaml` + `.github/workflows/ci.yml` (test on push) + `.github/workflows/release.yml` (goreleaser on tag).

**Why:** The design's success criterion "go install works from day one" requires the module to be published and a release pipeline to exist. Pre-built binaries (linux/amd64, linux/arm64, darwin/amd64, darwin/arm64) via goreleaser on tag push.

**Pros:** Enables `go install github.com/steadyfall/fspeek@latest` from day one. Binary downloads for users who don't have Go installed. CI catches regressions on push.

**Cons:** `.goreleaser.yaml` has its own config surface. goreleaser version pinning needed in CI.

**Context:** Deferred during eng review (user chose to handle at release time). The design already specifies all 4 target platforms. goreleaser is Layer 1 for Go binary distribution — standard config, well-documented.

**Where to start:** `goreleaser init` in the repo root generates a starter `.goreleaser.yaml`. Pin goreleaser action to a specific version in the release workflow.

**Depends on:** Working `go build ./cmd/fspeek` (Steps 1-10 in Next Steps)

---

## TODO-2: v1.1 format expansion (MP3, FLAC, JPEG, PNG + ApacheLister, NginxHTMLLister)

**What:** Implement the 4 deferred fetchers (MP3Fetcher, FLACFetcher, JPEGFetcher, PNGFetcher) and 2 deferred parsers (ApacheLister, NginxHTMLLister) with full test coverage.

**Why:** The original v1 design includes all 6 fetchers and 4 parsers. The vertical slice (v1.0) was scoped to MKV/MP4/SRT + nginx JSON to validate HTTP compatibility first.

**Pros:** Completes the full v1 feature set. MP3/FLAC cover audio servers; JPEG/PNG cover photo galleries. Apache and NginxHTML parsers cover the majority of real servers in the wild.

**Cons:** 6 more files + tests. FLAC Vorbis comment parsing and EXIF JPEG have edge cases. ApacheLister HTML patterns vary across Apache versions.

**Context:** Deferred because test server has MKV/SRT only. Once the httpReadSeeker + range fetch layer is validated against the real server, adding new fetchers is mechanical. JPEG/PNG only need bytes 0-64KB. MP3/FLAC need bytes 0-128KB.

**Where to start:** Add `fetcher/mp3.go` first (simplest: ID3v2 at offset 0). Validate against a known-good MP3 URL.

**Depends on:** v1.0 vertical slice validated against production server

---

## TODO-3: Docker-based integration test suite (testcontainers-go)

**What:** Add `fetcher/integration_test.go` using `testcontainers-go` to spin up nginx + Apache containers with known test media files. Covers the end-to-end path: URL → parser → fetcher → cache → rendered output.

**Why:** CI on GitHub Actions can't rely on a production server. testcontainers-go is the standard Go approach for integration tests with external services.

**Pros:** Reproducible. Runs in CI without external dependencies. Covers all 4 parsers against real server responses. Catches HTTP compatibility regressions.

**Cons:** Requires Docker in CI. testcontainers-go is a test dependency (~large module). Integration tests are slower than unit tests (30-60s per suite).

**Context:** The manual test approach (against real production server) works for v1.0 development but doesn't scale to CI. The `/office-hours` design already suggested `docker run -p 8080:80 nginx` as a test setup — this formalizes it.

**Where to start:** `go get github.com/testcontainers/testcontainers-go`. Start with a single nginx JSON autoindex container + 2-3 test files. Cover the full parse → cache → hit cycle.

**Depends on:** v1.1 (all parsers + fetchers complete)

---

## TODO-4: Cache server capability (Accept-Ranges per host)

**What:** Add a `server_caps` table to SQLite that stores per-host capabilities: `Accept-Ranges` support, `ETag` support, `Content-Length` availability. TTL: 24h.

**Why:** Currently every metadata fetch issues a HEAD request to check Accept-Ranges. Once you know a server supports range requests, this HEAD is redundant on every file access.

**Pros:** Halves HTTP round-trips for metadata on known-good servers. On a slow connection (VPN, remote server), each saved RTT is 50-200ms — noticeable in scrolling UX.

**Cons:** Third cache table with its own TTL lifecycle. If server config changes within 24h, old capability data is stale. Adds complexity to cache invalidation.

**Context:** Low priority for v1 — the HEAD requests are cheap and parallelized. But on high-latency connections (the primary use case for this tool), the extra RTT per-file adds up. 24h TTL is conservative and safe.

**Schema addition:**
```sql
CREATE TABLE IF NOT EXISTS server_caps (
    host         TEXT PRIMARY KEY,  -- scheme + host (e.g., "https://media.example.com")
    accept_ranges INTEGER NOT NULL,  -- 0 = false, 1 = true
    fetched_at   INTEGER NOT NULL
);
```

**Where to start:** Add to `cache/sqlite.go` after core cache is working. Use `url.Host` as the key (not full URL).

**Depends on:** Core cache working (v1.0)
