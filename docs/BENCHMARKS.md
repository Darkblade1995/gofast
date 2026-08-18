# GoFast — Benchmarks

This document tracks reproducible performance measurements
comparing GoFast against Huma, the closest comparable framework
(see [ADR 0001](adr/0001-build-time-codegen-vs-runtime-reflection.md)
for the architectural bet these benchmarks are meant to test).

## Methodology

- Benchmarks live in `benchmarks/`, an isolated Go module
  (`gofast/benchmarks`) with a `replace` directive pointing at the
  parent GoFast module. Huma is a dependency of this module only —
  it is never a dependency of GoFast itself.
- Every benchmarked function is marked `//go:noinline` to prevent
  the Go compiler from inlining it away entirely.
- Inputs are generated at runtime with varying values (not
  compile-time literals), to prevent the compiler from constant-
  folding the result and eliminating the work being measured.
- Both sides validate equivalent struct shapes and rules at every
  level measured below.

These two precautions were not theoretical — the first benchmark
attempt returned physically implausible numbers (~0.2 ns/op, faster
than a single CPU cycle) due to the compiler optimizing the
measured call away entirely. The methodology above is what fixed it.

## Results — isolated struct validation (simple struct)

Struct: an email + password login payload (`required`, `email`,
`min` rules).

| | GoFast | Huma v2.39.1 |
|---|---|---|
| Time per operation | 2.123 ns/op | 79.53 ns/op |
| Memory allocated | 0 B/op | 96 B/op |
| Allocations | 0 allocs/op | 3 allocs/op |

GoFast validated the struct roughly **37.5x faster**, with zero
heap allocations against three per call on the Huma side.

## Results — isolated struct validation (nested + slice)

Struct: `Amount int64`, a nested pointer field (`*ReceiverInfo`
with its own required field), and a slice validated per-element
(`Tags []string`, each element `min=3`).

| | GoFast | Huma v2.39.1 |
|---|---|---|
| Time per operation | 4.367 ns/op | 85.69 ns/op |
| Memory allocated | 0 B/op | 112 B/op |
| Allocations | 0 allocs/op | 3 allocs/op |

GoFast is roughly **19.6x faster** here, down from 37.5x on the
simple struct. GoFast's cost scales with the actual validation
work (roughly 2x from the simple to the complex struct); Huma's
cost stays nearly flat (79.53 ns to 85.69 ns) because it is
dominated by its runtime validation machinery rather than the
number of fields being checked. GoFast's relative advantage
narrows as structs grow more complex, and would likely be larger
on structs simpler than the login example above.

## Results — full HTTP request cycle

Measured with `httptest` (routing + JSON decode + validate + JSON
encode, no real network I/O), using the simple login struct.

| | GoFast | Huma v2.39.1 |
|---|---|---|
| Time per operation | 4400 ns/op | 6549 ns/op |
| Memory allocated | 8097 B/op | 8037 B/op |
| Allocations | 39 allocs/op | 53 allocs/op |

GoFast is roughly **1.49x faster** end-to-end, with about 26% fewer
allocations. Memory usage is comparable — both frameworks pay
similar costs for JSON decoding/encoding and net/http overhead,
which dominate the total budget and dilute the isolated validation
advantage measured above. This confirms the expectation stated
when only the isolated benchmark existed: **the isolated-validation
advantage does not translate directly to end-to-end request
latency**, because JSON decoding and HTTP machinery — identical
cost for both frameworks — make up the majority of total time and
memory in a real request.

## Results — concurrent throughput

Measured with `b.RunParallel` across all 12 threads, using the
simple login struct over the full HTTP cycle.

| | GoFast | Huma v2.39.1 |
|---|---|---|
| Time per operation | 2715 ns/op | 2850 ns/op |
| Memory allocated | 8103 B/op | 8090 B/op |
| Allocations | 39 allocs/op | 53 allocs/op |

Under concurrent load, the time-per-operation gap nearly
disappears (~1.05x), as Go's scheduler and garbage collector
dominate cost more than either framework's own validation logic.
The allocation gap (~26% fewer allocations for GoFast) persists
across every scenario measured, and is the most consistent
advantage observed — likely to matter most under sustained
production load, where allocation pressure accumulates over time.

## Summary across all four scenarios

| Scenario | GoFast | Huma | Advantage |
|---|---|---|---|
| Isolated validation (simple struct) | 2.123 ns/op, 0 allocs | 79.53 ns/op, 3 allocs | ~37.5x |
| Isolated validation (nested + slice) | 4.367 ns/op, 0 allocs | 85.69 ns/op, 3 allocs | ~19.6x |
| Full HTTP cycle (sequential) | 4400 ns/op, 39 allocs | 6549 ns/op, 53 allocs | ~1.49x |
| Full HTTP cycle (concurrent) | 2715 ns/op, 39 allocs | 2850 ns/op, 53 allocs | ~1.05x |

The allocation gap (roughly 26% fewer allocations for GoFast) is
the most consistent advantage across all four scenarios. The
latency advantage is largest in isolation and shrinks sharply as
JSON decoding, routing, and Go's own scheduler/GC come to dominate
the cost of a real, concurrent request cycle.

## Scope — what this does and does not measure

**Measures:** validation cost in isolation (simple and complex
structs), and the full HTTP request/response cycle both
sequentially and under concurrent load, using Go's `httptest`
package (no real network I/O).

**Does not measure:** real network latency, TLS overhead, larger
payloads, deeply nested structures beyond one level, or sustained
load over time (garbage collector behavior under prolonged
pressure). These remain open for future measurement — see the
[roadmap](ROADMAP.md).


## Profiling — where the time actually goes

CPU profile of the full HTTP cycle benchmark
(`go tool pprof -top cpu.prof`), top contributors:

| Function | % of sampled time |
|---|---|
| `runtime.mallocgc` (allocator/GC) | 23.68% |
| `runtime.memmove` + `memclrNoHeapPointers` | ~13.2% |
| `runtime.scanObject` + `sweepone` (GC scanning) | ~10.5% |
| `encoding/json` (decode) | ~6.3% |
| `net/textproto` (HTTP header parsing) | 1.58% |

None of GoFast's own generated code (`Validate()`, `BindPath()`,
`BindQuery()`) appears among the top 15 CPU consumers. This is
direct profiling evidence for the claim already suggested by the
benchmark numbers above: in a full HTTP request, cost is dominated
by Go's runtime (garbage collector, memory allocation) and JSON
encoding/decoding — shared costs identical for GoFast and Huma —
not by GoFast's generated validation code itself.

## Hardware

- Machine: HP Victus 15-fb3xxx (laptop)
- CPU: AMD Ryzen 7 7445HS, 6 cores / 12 threads, up to 4.75 GHz
- RAM: 24 GiB (8 GiB + 16 GiB SODIMM, 5600 MHz)
- Storage: Samsung MZVL8512HFLU-00BH1 NVMe SSD
- OS: Ubuntu (Linux, amd64)
- Go: 1.26.5

## Reproducing these results

```bash
cd benchmarks
go test -bench=BenchmarkGoFastValidate -benchmem ./...
go test -bench=BenchmarkHumaValidate -benchmem ./...
go test -bench=BenchmarkGoFastValidateComplex -benchmem ./...
go test -bench=BenchmarkHumaValidateComplex -benchmem ./...
go test -bench=BenchmarkGoFastHTTPCycle -benchmem ./...
go test -bench=BenchmarkHumaHTTPCycle -benchmem ./...
go test -bench=BenchmarkGoFastConcurrent -benchmem ./...
go test -bench=BenchmarkHumaConcurrent -benchmem ./...
```

Each command runs against real, runnable code in `benchmarks/` —
nothing in this document is asserted without a corresponding
benchmark file in the repository.