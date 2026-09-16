# ADR 0018 — Manifest concurrent write safety via optimistic concurrency

## Status
Accepted

## Context
`.gofast/manifest.json` is a single shared file. If `gofast generate`
runs concurrently against different packages in the same monorepo
(a real scenario in parallel CI, or a multi-module repository), each
process loads the whole manifest, updates its own entry in memory,
and writes the entire file back. Two processes finishing close
together can race: the second process's write silently overwrites
the first's, discarding a correctly-generated entry. This was a
tracked Known Risk since Fase A, resolved now as follow-up cleanup
after the roadmap's completion.

## Decision

**Optimistic concurrency via a version counter, not file locking.**
`Manifest` gains a `Version int` field, incremented on every
successful `Save()`. A new function,
`UpdateAndSave(startDir, sourcePath, generatedFiles)`, performs the
full read-modify-write cycle with automatic retry:

1. Load the manifest fresh from disk.
2. Apply `Update(sourcePath, generatedFiles)` to the in-memory copy.
3. Attempt to save, but first re-read the file on disk and compare
   its `Version` to the version this process loaded in step 1.
4. If the on-disk version has changed (another process wrote in the
   meantime), reload the manifest fresh, re-apply this process's own
   `Update` on top of the new state, and retry from step 3.
5. After a small, bounded number of retries (5), return an error
   rather than retrying indefinitely — this scenario is expected to
   be exceedingly rare in practice, and an unbounded retry loop
   would be a worse failure mode than a clear error.

File locking (flock or a lockfile) was considered and rejected — see
Alternatives. This approach uses only the Go standard library, adds
no new dependency, and mirrors a widely-used, well-understood
pattern (optimistic concurrency, the same principle behind HTTP
ETags/If-Match).

**`cmd/gofast/main.go` switches from separate `Update` + `Save`
calls to the new `UpdateAndSave`.** The old `Update`/`Save` methods
remain available (existing tests and any code calling them directly
still work), but `UpdateAndSave` is the safe path going forward for
any caller writing to a manifest that might be touched concurrently.

## Alternatives considered

- **File locking (flock, or a `.lock` file)**
  Rejected — introduces platform-specific behavior (flock semantics
  differ subtly across OSes) and, for a lockfile approach, a stale
  lock left behind by a crashed process becomes its own failure mode
  requiring additional handling (timeout, force-unlock). Optimistic
  concurrency sidesteps both problems: no lock to go stale, and pure
  standard-library file I/O behaves consistently across platforms.

- **A separate manifest file per package**
  Rejected — this would avoid the race entirely, but changes the
  manifest's shape (one file becomes many), touching every existing
  consumer (`LoadManifest`, `verify.go`, the CLI) for a problem that
  optimistic concurrency solves without a structural change. Also
  loses the single-file convenience of seeing a whole monorepo's
  generation state at a glance.

- **Unbounded retry until success**
  Rejected — see Decision step 5. A bounded retry with a clear error
  is safer than a retry loop that could hang indefinitely under
  pathological concurrent load.

## Consequences

**Gained:**
- Concurrent `gofast generate` runs across different packages in a
  monorepo (or parallel CI) no longer risk silently losing another
  process's manifest entry.
- No new dependency, no platform-specific locking behavior.

**Sacrificed:**
- A process that hits the retry limit (extremely unlikely in
  practice — it requires 5 consecutive write collisions) must retry
  the whole `gofast generate` invocation itself; this is a coarser
  recovery than a lock-based approach might offer, but matches the
  low actual likelihood of the scenario.

**Risk accepted consciously:**
- This does not make concurrent generation of the *same* source
  file (two processes running `gofast generate` on the identical
  `.go` file at the same time) meaningfully safer against writing
  conflicting `.gen.go` file contents — it only protects the shared
  manifest.json from losing entries. Concurrent generation of the
  same file is a different, narrower scenario not addressed here;
  no evidence has emerged that it happens in practice (unlike the
  cross-package case, which is a normal outcome of parallel CI).