package gofast

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"reflect"
	"strings"
)

// Validatable is implemented by input structs that need custom
// validation beyond what code generation covers. Handler calls
// Validate automatically if the decoded input implements this
// interface.
type Validatable interface {
	Validate() error
}

// PathBinder is implemented by input structs that bind values
// from URL path parameters. Handler calls BindPath automatically
// after decoding the request body, passing the path parameters
// extracted from the registered route pattern.
type PathBinder interface {
	BindPath(params map[string]string)
}

// QueryBinder is implemented by input structs that bind values
// from URL query parameters. Handler calls BindQuery automatically
// with the parsed query string of the incoming request.
type QueryBinder interface {
	BindQuery(values url.Values)
}

// HandlerInfo carries the compiled http.HandlerFunc along with the
// reflected input and output types of the original business
// function. Router.Register uses the type information to generate
// accurate OpenAPI documentation.
type HandlerInfo struct {
	Func    http.HandlerFunc
	InType  reflect.Type
	OutType reflect.Type
}

// Handler wraps a business function into a ready-to-register HTTP
// handler. It decodes the JSON request body into In, applies path
// and query binding if In implements PathBinder or QueryBinder,
// validates In if it implements Validatable, calls fn, and encodes
// the result as a JSON response.
func Handler[In any, Out any](fn func(context.Context, In) (Out, error)) HandlerInfo {
	var inZero In
	var outZero Out

	handlerFunc := func(w http.ResponseWriter, r *http.Request) {
		var in In

		cfg := configFromContext(r.Context())

		if r.Method != http.MethodGet && r.Body != nil && r.ContentLength != 0 {
			if err := checkContentType(r); err != nil {
				writeError(w, http.StatusUnsupportedMediaType, ErrCodeUnsupportedMedia, err.Error(), nil)
				return
			}

			r.Body = http.MaxBytesReader(w, r.Body, cfg.maxBodySize)
			decoder := json.NewDecoder(r.Body)
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&in); err != nil {
				writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid JSON body", err.Error())
				return
			}
		}

		if pb, ok := any(&in).(PathBinder); ok {
			params := extractPathValues(r)
			pb.BindPath(params)
		}

		if qb, ok := any(&in).(QueryBinder); ok {
			qb.BindQuery(r.URL.Query())
		}

		if v, ok := any(in).(Validatable); ok {
			if err := v.Validate(); err != nil {
				writeError(w, http.StatusBadRequest, ErrCodeValidation, err.Error(), nil)
				return
			}
		}

		out, err := fn(r.Context(), in)
		if err != nil {
			handleFuncError(w, err)
			return
		}

		respondJSON(w, http.StatusOK, out)
	}

	return HandlerInfo{
		Func:    handlerFunc,
		InType:  reflect.TypeOf(inZero),
		OutType: reflect.TypeOf(outZero),
	}
}

func checkContentType(r *http.Request) error {
	ct := r.Header.Get("Content-Type")
	if ct == "" {
		return nil
	}

	mediaType := strings.Split(ct, ";")[0]
	mediaType = strings.TrimSpace(mediaType)

	switch mediaType {
	case "application/json", "":
		return nil
	case "multipart/form-data":
		return &unsupportedMediaError{
			got:    mediaType,
			detail: "multipart/form-data is not supported in this version of GoFast (file uploads and form encoding are not yet implemented)",
		}
	case "application/x-www-form-urlencoded":
		return &unsupportedMediaError{
			got:    mediaType,
			detail: "application/x-www-form-urlencoded is not supported in this version of GoFast (only application/json is accepted)",
		}
	default:
		return &unsupportedMediaError{
			got:    mediaType,
			detail: "only application/json is supported in this version of GoFast",
		}
	}
}

type unsupportedMediaError struct {
	got    string
	detail string
}

func (e *unsupportedMediaError) Error() string {
	return e.detail
}

func extractPathValues(r *http.Request) map[string]string {
	params := map[string]string{}
	pattern := r.Pattern
	for _, part := range splitPath(pattern) {
		if len(part) > 0 && part[0] == '{' && part[len(part)-1] == '}' {
			name := part[1 : len(part)-1]
			params[name] = r.PathValue(name)
		}
	}
	return params
}

func splitPath(pattern string) []string {
	result := []string{}
	current := ""
	for _, ch := range pattern {
		if ch == '/' {
			if current != "" {
				result = append(result, current)
			}
			current = ""
		} else {
			current += string(ch)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

func respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("[gofast] failed to encode response: %v", err)
	}
}
