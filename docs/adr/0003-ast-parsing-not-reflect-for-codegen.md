# ADR 0003 — Code generation uses go/ast parsing, not reflect

## Status
Accepted

## Context
ADR 0001 established that GoFast generates validation code at
build time instead of using reflection at runtime. However, its
wording imprecisely suggested using the `reflect` package "over
the source code" — this is not technically possible, since
`reflect` operates on live runtime values, not on unparsed source
text.

## Decision
GoFast's code generator (`internal/codegen`) parses Go source
files directly using the standard library's `go/parser` and
`go/ast` packages. This produces an Abstract Syntax Tree (AST)
that the generator walks to discover structs, fields, and struct
tags — entirely at build time, without instantiating any values
and without using `reflect` anywhere in the generator itself.

This is consistent with how other well-known Go code generation
tools work (`stringer`, `mockgen`, `sqlc`).

## Alternatives considered

- **Use `reflect` on compiled/instantiated types**
  Rejected. Would require the generator to import and instantiate
  the user's own types, creating a circular build dependency
  (the generator would need to compile the very code it's meant
  to generate code for, before generating it).

## Consequences

**Gained:**
- Technically accurate: the generator never uses reflection,
  at any point, correcting the imprecision in ADR 0001.
- No circular build dependency — the generator only needs to
  read source text, not compile or import user packages.

**Sacrificed:**
- AST parsing is more verbose to work with than reflect would
  be — traversing syntax trees requires more code than reflecting
  over live values.

This ADR clarifies and supersedes the "reflect package" wording
in ADR 0001's Decision section. ADR 0001's core decision (build-time
generation vs runtime reflection) remains unchanged and correct.