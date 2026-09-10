package gofast

import "mime/multipart"

// UploadedFile wraps a single file submitted in a
// multipart/form-data request. It is a thin wrapper over
// *multipart.FileHeader — GoFast does not reimplement multipart
// parsing, only exposes it through the same tag-based binding
// pattern already used for path and query parameters. See ADR
// 0017.
type UploadedFile struct {
	header *multipart.FileHeader
}

// NewUploadedFile wraps header. Used by generated BindMultipart
// methods (see gofast generate); not typically called directly by
// application code.
func NewUploadedFile(header *multipart.FileHeader) *UploadedFile {
	return &UploadedFile{header: header}
}

// Filename returns the original filename as submitted by the
// client. This value comes from the client and should not be
// trusted as a safe filesystem path without validation.
func (u *UploadedFile) Filename() string {
	return u.header.Filename
}

// Size returns the file's size in bytes.
func (u *UploadedFile) Size() int64 {
	return u.header.Size
}

// Open opens the file for reading. The caller is responsible for
// closing the returned multipart.File.
func (u *UploadedFile) Open() (multipart.File, error) {
	return u.header.Open()
}

// MultipartBinder is implemented by input structs that bind values
// from a multipart/form-data request. Handler calls BindMultipart
// automatically, after parsing the form, if In implements this
// interface. See ADR 0017.
type MultipartBinder interface {
	BindMultipart(form *multipart.Form) error
}