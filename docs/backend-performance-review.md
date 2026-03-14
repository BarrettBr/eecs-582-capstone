# Backend Performance Review

Date: March 14, 2026

This review focuses on the Go backend under `ingest/`, with emphasis on:

- unnecessary runtime bloat
- hot-path overhead
- O(N) and higher-cost operations in steady-state ingest flow
- places where the architecture takes extra hops without enough value

The review is based on static inspection of the current code and the current passing test suite. It is not based on production profiling data, so impact is estimated from the code paths and expected call frequency.

Note: this document reflects the current post-refactor runtime. The earlier `BufferManager` hot-path issues around shared overflow scans, full service-order rescans, pressure recomputation via full snapshots, and eager websocket payload encoding have already been addressed and are intentionally omitted from the open findings below.

## Executive Summary

The backend is in decent shape for a capstone-sized project, and the largest buffering-layer inefficiencies have already been cleaned up. The main remaining opportunities are:

1. The managed service loop still behaves like work-plus-sleep rather than true fixed-rate scheduling, which limits very high configured poll rates.
2. The shared fanout path still does avoidable allocation work in ML and SQL batching.
3. Websocket room batching and history response shaping are acceptable for now, but worth keeping in mind if measured load shifts there.

If only one subsystem gets optimized next, it should be the service scheduling and downstream batching hot path.

## Hot Path Overview

The steady-state ingest path today is:

1. Source service reads data in `internal/ingest/runtime/service.go`
2. Source adapter normalizes record in `internal/ingest/runtime/modbus_loop.go`
3. Validation runs in `internal/ingest/validation/engine.go`
4. Event enters the shared `BufferManager` in `internal/ingest/runtime/buffer_manager.go`
5. `BufferManager` dispatches into shared pipeline in `internal/ingest/runtime/pipeline.go`
6. Pipeline fans out to SQL, ML, and websocket in `internal/ingest/runtime/fanout.go`

The hottest code is still steps 3 through 6, because they run on every event regardless of source mode.

## Findings

### 1. Managed services are scheduled as work plus wait, so configured poll rates overstate the true throughput ceiling

Files:

- `ingest/internal/ingest/runtime/service.go`

Why this matters:

- The loop runs `handleTick()`, then waits on a timer, then schedules the next read.
- That means the real period is `read + validate + buffer submit + interval`, not just `interval`.
- For normal polling rates this is fine, but at very low intervals like `10us` the work dominates and the configured rate becomes misleading.

Suggested fix:

- Decide whether services should be fixed-delay or fixed-rate.
- If fixed-rate is the goal, schedule against a moving deadline instead of resetting the timer after each completed tick.
- Keep the current adaptive backpressure interval changes, but apply them to the next target deadline rather than adding pure post-work sleep.

Priority:

- Medium

### 2. Shared fanout batching still allocates grouped payload slices and temporary batch state on the hot path

Files:

- `ingest/internal/ingest/runtime/fanout.go`

Why this matters:

- ML delivery rebuilds `map[string][]any` for every flush.
- SQL and ML workers rebuild temporary batch slices every cycle.
- Those allocations are smaller than the earlier buffering issues, but they are still in the steady-state path for every admitted event.

Suggested fix:

- Reuse grouping buffers where practical.
- Consider dedicated typed batches per event type to avoid rebuilding `[]any`.
- Only do this after the service scheduling semantics are settled, since that shapes the real ingest rate ceiling.

Priority:

- Medium

### 3. Websocket server still does extra copying and room batch marshaling, but the tradeoff is probably acceptable

Files:

- `ingest/internal/stream/server.go`

Why this matters:

- `Publish()` clones `msg.Data`
- batch loop allocates and marshals websocket frames per room flush
- inbound payload parsing attempts batch JSON, then single message JSON
- service-room routing keeps fanout off the global client list, which is the right tradeoff, but it means bursty multi-room clients can receive multiple frames per flush cycle

This is real overhead, but compared to the remaining service scheduling and fanout work it is not the main bottleneck unless websocket traffic becomes dominant.

Suggested fix:

- Leave as-is for now.
- Only optimize if websocket broadcasting becomes a measured bottleneck.
- If that happens:
  - avoid unnecessary data copies for immutable payloads
  - use a pool for batch frame buffers

Priority:

- Low

### 4. History API row shaping is straightforward and not worth optimizing yet

Files:

- `ingest/internal/api/history.go`

Notes:

- Row iteration and response shaping are linear in result size, which is expected.
- `splitAnomalies()` allocates per row, but this is request path work, not ingest hot path work.

Suggested fix:

- No immediate change needed.
- If history endpoints become large-volume, pagination and tighter SQL result shaping will matter more than local slice tuning.

Priority:

- Low

## Recommended Fix Order

### Phase 1: Fix the remaining steady-state hot-path issues

1. Decide whether source services should run fixed-delay or fixed-rate and update `service.go` accordingly.
2. Reduce reusable batch allocation churn in `fanout.go`.

### Phase 2: Keep low-priority work measured and targeted

1. Only consider caching status snapshot rendering if status polling becomes measurably frequent.

## What Not To Optimize Yet

The following areas look fine for now:

- route registration
- history/report handler control flow
- file-watch reload debounce
- current package ownership split across `runtime`, `events`, and `validation`
- stream server client map management
- SQL history response slice building
- current direct field-based validation path
- current `BufferManager` queue layout and pressure bookkeeping
- lazy websocket encoding in the sink path

These are either cold-path, low-frequency, already simple enough, or have already received the main performance cleanup they needed.

## Final Recommendation

The backend should not be broadly rewritten. The right move is targeted simplification:

- tighten service scheduling semantics
- trim fanout allocation churn where it actually sits on the ingest hot path
- revisit websocket and history work only if profiling shows they matter

Those changes will remove most of the remaining backend bloat without changing the architecture the project now relies on.
