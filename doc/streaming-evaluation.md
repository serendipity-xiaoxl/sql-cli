# Streaming Evaluation

## Context

The library forces a maximum LIMIT (default 1000) on all SELECT queries. With this cap in place, a single query can return at most 1000 rows at once. The question: is streaming (`QueryStream`) still valuable alongside paginated queries (`QueryWithOffset` / `QueryWithOptions`)?

## Analysis

### Memory Bounds

With a LIMIT cap of 1000 rows, the absolute memory consumption of a non-streaming query is bounded:

- 1000 rows x ~1 KB/row (typical) = ~1 MB
- 1000 rows x ~100 KB/row (wide row, e.g. JSON blobs) = ~100 MB

The worst case is bounded but not negligible. Streaming reduces peak memory to a single row's worth of data regardless of row count or width.

### Use Cases Where Streaming Wins

1. **Large-in-count results** (1000 wide rows): Even with LIMIT 1000, a query returning 1000 rows of 50 KB each uses ~50 MB in memory. Streaming uses ~50 KB.

2. **Single-pass analysis**: An Agent processing rows one by one (filtering, aggregating, classifying) does not need the entire result set in memory. Streaming delivers rows as the database produces them.

3. **First-row latency**: Streaming returns the first row immediately. Pagination requires the entire page to be buffered before any row is returned.

4. **Early termination**: An Agent may only need the first N rows that match a predicate. With streaming, it calls `Close()` and the connection is released. With pagination, it has already paid the cost of fetching all rows.

5. **Consistency**: Streaming uses a single database cursor. Pagination with OFFSET can miss or duplicate rows if data changes between page fetches.

### Complexity Cost

Streaming adds:
- A background goroutine per stream (channel-based row delivery)
- A `StreamResult` iterator type with `Next()/Scan()/Err()/Close()` lifecycle
- Concurrency slot management to limit parallel streams
- The goroutine must be carefully managed: context cancellation, channel closure, producer-done signaling, error propagation

This complexity is confined to `pkg/db/stream.go` and `pkg/result/result.go` (StreamResult). It does not affect the rest of the codebase.

### Usage Statistics

From the test suite and CLI:
- `QueryStream` is used in 2 test files (stream_test.go, session_test.go)
- The CLI has a dedicated `stream` command
- The Go library API exposes it as a first-class method on the `Database` interface

### Alternatives Considered

| Alternative | Pro | Con |
|-------------|-----|-----|
| Remove streaming entirely | Simplifies library | Loses memory safety for wide rows, higher latency, no early termination |
| Make streaming opt-in via config | Backward compatible | Every user must know to configure it; default path loses streaming benefits |
| Keep streaming as-is (current) | Best of both worlds | Slight API surface increase, goroutine overhead (only when streaming is used) |

## Decision

**Keep streaming as-is.** The value proposition is clear:

- It is memory-bounded per-row rather than per-page, which matters for wide rows
- It provides lower first-row latency than pagination
- It enables early termination without wasted work
- It uses a single cursor for consistency

The goroutine complexity is encapsulated entirely within the streaming implementation and does not leak into the rest of the system. The cost of maintaining the streaming path is low relative to the benefit it provides.

Users who do not need streaming simply do not call `QueryStream` and are unaffected by its existence.

### Recommendation for Documentation

The operation manual already documents the tradeoff. The guidance should be:

> Use `QueryStream` for bulk processing, wide rows, single-pass analysis, and when you need the first row as fast as possible. Use paginated queries (`QueryWithOffset`) for random page access, stateless or parallel operations, and restartable batch jobs.
