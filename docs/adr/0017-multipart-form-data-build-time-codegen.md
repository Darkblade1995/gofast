# ADR 0017 — Multipart/form-data: build-time codegen, consistent with ADR 0001

## Status
Accepted

## Context
GoFast today rejects multipart/form-data outright with 415
(checkContentType in handler.go) — this project's most common gap:
no file upload support. This is Fase C.2, the final piece of the
roadmap before v1.0.0.

Unlike WebSockets (ADR 0013) and SSE (ADR 0016), multipart/form-data
IS fundamentally request/response — a client submits a form once,
the server responds once. It fits Handler[In, Out]'s existing
contract. The gap is not the contract; it's that Handler[In, Out]
today only knows how to JSON-decode a body.

The real design question is how binding is implemented: at build
time (matching path/query — ADR 0001, 0003) or via runtime
reflection (contradicting this project's central bet).

## Decision

**Build-time codegen, via a new `form:"..."` tag, following the
exact same pattern already established for `path:"..."` and
`query:"..."`.** internal/codegen already has the general
machinery this needs: extractSimpleTag reads a tag by key,
tagFields collects tagged fields from a struct, and
GenerateBindPath/GenerateBindQuery are the direct template for a
new GenerateBindMultipart.

```go
type UploadInput struct {
	Title string              `form:"title"`
	File  *gofast.UploadedFile `form:"file"`
}
```

`gofast generate` produces:

```go
func (i *UploadInput) BindMultipart(form *multipart.Form) error {
	if vals, ok := form.Value["title"]; ok && len(vals) > 0 {
		i.Title = vals[0]
	}
	if files, ok := form.File["file"]; ok && len(files) > 0 {
		i.File = gofast.NewUploadedFile(files[0])
	}
	return nil
}
```

`UploadedFile` (new type in gofast/) wraps `*multipart.FileHeader`,
exposing `Filename()`, `Size()`, and `Open() (multipart.File, error)`
— a thin wrapper, not a reimplementation of multipart parsing
itself (which the standard library already does correctly).

**Handler[In, Out] gains a MultipartBinder interface**, mirroring
PathBinder/QueryBinder exactly:

```go
type MultipartBinder interface {
	BindMultipart(form *multipart.Form) error
}
```

When Content-Type is multipart/form-data, Handler parses the form
(`r.ParseMultipartForm`, bounded by the Router's existing
maxBodySize config — ADR from Phase 2) and calls BindMultipart if
the input type implements it, instead of rejecting with 415.
**checkContentType's 415 rejection is removed for
multipart/form-data specifically** — the whole reason that
rejection existed (ADR mentions in the original feature matrix) was
the absence of any handling; now that handling exists, the
rejection is obsolete for this one content type. application/
x-www-form-urlencoded remains rejected — no evidence yet that it's
needed, and expanding scope to it without a concrete case would be
the same kind of unforced addition already rejected elsewhere in
this project (e.g. ADR 0015's rejection of a full assertion DSL).

**Zero runtime reflection is preserved.** This is the load-bearing
reason for choosing this design over the simpler-to-implement
alternative — see Alternatives.

## Alternatives considered

- **Runtime reflection to populate `form:"..."` fields, without
  touching internal/codegen**
  Rejected — this would directly contradict ADR 0001 and ADR 0003's
  central bet (build-time codegen, never runtime reflection) for
  this one feature, undermining the project's own foundational
  claim in its very last roadmap item. Simpler to build, but
  inconsistent with everything already shipped; consistency was
  chosen over the shortcut.

- **A separate, non-generated multipart parsing API** (e.g.
  `gofast.ParseMultipart(r) (*MultipartData, error)` used manually
  inside a handler)
  Rejected — this would create a second, parallel binding mechanism
  alongside PathBinder/QueryBinder instead of extending the existing
  one, adding inconsistency for no benefit; the tag-based pattern
  already works well and generalizes cleanly to this case.

- **Supporting application/x-www-form-urlencoded in the same change**
  Rejected — no concrete need identified yet; scope stays to the
  gap actually documented in the roadmap (multipart/form-data).

## Consequences

**Gained:**
- File uploads (profile photos, document attachments, etc.) are
  finally possible through GoFast's normal Handler[In, Out]
  pattern, with the same build-time, zero-reflection guarantee as
  every other binding mechanism.
- The `form:"..."` tag is immediately familiar to anyone who has
  used `path:"..."` or `query:"..."` — same mental model, same
  generation step.

**Sacrificed:**
- No support for application/x-www-form-urlencoded — deliberately
  out of scope per Alternatives above.
- MultipartBinder, unlike PathBinder/QueryBinder, deals with actual
  file content (potentially large) — an application must still
  reason about where UploadedFile.Open() reads from (memory vs. a
  temp file on disk, per Go's own multipart.Form semantics) and
  size limits appropriately via the Router's existing
  WithMaxBodySize option.

**Risk accepted consciously:**
- Multipart forms with deeply nested or unusual structures (nested
  arrays of files, for instance) are not addressed by this initial
  implementation — the generator handles single-value and
  single-file-per-field cases, matching the complexity level
  path/query bindings already handle. More complex multipart shapes
  can be added incrementally later without revisiting this
  foundational decision.