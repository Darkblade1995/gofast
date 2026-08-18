# ADR 0005 — Semantic versioning policy before v1.0.0

## Status
Accepted

## Context
GoFast has already made two breaking changes to its core contract
during development: adding `context.Context` as the first
parameter of the function passed to `Handler`, and changing
`Handler`'s return type from `http.HandlerFunc` to `HandlerInfo`
(to carry reflected type information for OpenAPI generation).

Neither change was announced as a "breaking change" in any formal
sense, because no version number or stability promise existed yet
to break. As GoFast approaches a point where it might be published
and used by others, this needs to be made explicit rather than
left as an implicit assumption.

## Decision
GoFast follows Semantic Versioning (semver.org) starting from its
first tagged release, with these specific rules for the pre-v1.0.0
period:

- All releases before v1.0.0 use the `v0.x.y` scheme.
- Any `v0.x.0` release (minor version bump) MAY contain breaking
  changes to the public API, including the core `Handler`/`Router`
  contract. This is standard semver behavior: under v1.0.0, minor
  version bumps are not guaranteed to be backward compatible.
- `v0.x.y` patch releases (bumping only y) will not contain
  breaking changes — only bug fixes, documentation, or additions
  that do not alter existing public signatures.
- v1.0.0 will be tagged only once the core contract (`Handler`,
  `Router`, the three binder interfaces, and the error types) has
  been stable — unchanged in a breaking way — across at least two
  consecutive minor releases.
- After v1.0.0, standard semver applies in full: breaking changes
  require a major version bump.

## Alternatives considered

- **Commit to API stability immediately**
  Rejected. GoFast's core contract changed twice already during
  active design work; promising stability now would either be
  dishonest or would freeze the design before it has had enough
  real usage to validate the current shape.

- **No versioning policy at all, ad-hoc releases**
  Rejected. This is what GoFast has effectively been doing during
  development, and it is exactly what makes external adoption
  unsafe — nobody can depend on a project with no stated policy at
  all, even an informal one.

## Consequences

**Gained:**
- Anyone integrating GoFast pre-v1.0.0 knows explicitly that minor
  version bumps might break their code, and can pin their
  dependency accordingly (`go.mod` version pinning already handles
  this mechanically once tags exist).
- A concrete, checkable bar for v1.0.0: two consecutive minor
  releases without breaking the core contract.

**Sacrificed:**
- No committed timeline for v1.0.0 — this policy explicitly avoids
  promising a date, only a condition.

**Risk accepted consciously:**
- Nothing currently enforces this policy mechanically (no CI check
  compares public API surface between releases). This is a
  documented intention, not yet a guarantee backed by tooling.
  Adding an API-diff check to CI is a reasonable future addition,
  not built today.