# Observability Cleanup — 2026-05-13

## Phase 2: Structured Logging

### convert stdlib log calls to structured Zap logging (`d4d285d`)

**Changed**
- `internal/pilots/stats_service.go` — replaced all `log.Printf`/`fmt.Printf` with `logging.Debug/Info/Warn/Error`; removed `"log"` import; verbose debug-dump log lines (raw JSON, schema fields) removed entirely since they were only useful during development
- `internal/pilots/linking_job.go` — replaced all `log.Printf` with structured Zap calls; removed `"log"` import

**Reused**
- `infra/logging` package functions — same pattern already established in `sync_job.go`, `repository.go`, `pireps/sync_job.go`, etc.

**Metrics/Logging added**
- No new metrics. Logging converted at appropriate levels: Debug for internal steps (cache checks, formula building), Warn for optional-data failures (game stats, provider data), Error only when returning an error to caller.

**Test surface**
- Functions introduced: none (conversion only)
- Behaviour to integration-test: none changed

**Live API compliance**
- Not applicable (no Live API polling changes)

**Build status**
`go build ./...` passed (vizburo excluded per plan)

**Notes**
- Several verbose `fmt.Printf` log blocks in `stats_service.go` that dumped raw Airtable JSON and schema field lists were removed entirely rather than converted — they were development-only debug output that would be noise in production JSON logs and added no structured value.
- `fetchRouteFromAirtablePIREP` silent-returns empty string on config/parse errors rather than logging — consistent with the existing pattern in sibling functions that treat Airtable fallback as best-effort.
