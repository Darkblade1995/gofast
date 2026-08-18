package gofast

import (
	"encoding/json"
	"reflect"
	"testing"
)

type testLoginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type testLoginOutput struct {
	Token string `json:"token"`
}

type testGetAccountInput struct {
	ID string `path:"id"`
}

type testGetAccountOutput struct {
	ID      string `json:"id"`
	Balance int64  `json:"balance"`
}

type testUpdateAccountInput struct {
	ID   string `path:"id"`
	Name string `json:"name"`
}

type testUpdateAccountOutput struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type testDeleteAccountInput struct {
	ID string `path:"id"`
}

type testDeleteAccountOutput struct {
	Deleted bool `json:"deleted"`
}

func typeOf(v any) reflect.Type {
	return reflect.TypeOf(v)
}

func TestGenerateOpenAPI(t *testing.T) {
	routes := []Route{
		{
			Method:  "POST",
			Path:    "/login",
			InType:  typeOf(testLoginInput{}),
			OutType: typeOf(testLoginOutput{}),
		},
	}

	spec := GenerateOpenAPI("GoFast Example", "1.0.0", routes)

	data, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal spec: %v", err)
	}

	t.Logf("generated spec:\n%s", data)

	pathItem, ok := spec.Paths["/login"]
	if !ok {
		t.Fatal("expected /login path in spec")
	}

	op, ok := pathItem["post"]
	if !ok {
		t.Fatal("expected post operation for /login")
	}

	if op.RequestBody == nil {
		t.Fatal("expected a request body schema for POST /login")
	}
}

func TestGenerateOpenAPIPathParam(t *testing.T) {
	routes := []Route{
		{
			Method:  "GET",
			Path:    "/accounts/{id}",
			InType:  typeOf(testGetAccountInput{}),
			OutType: typeOf(testGetAccountOutput{}),
		},
	}

	spec := GenerateOpenAPI("GoFast Example", "1.0.0", routes)

	data, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal spec: %v", err)
	}
	t.Logf("generated spec:\n%s", data)

	op := spec.Paths["/accounts/{id}"]["get"]

	if len(op.Parameters) != 1 {
		t.Fatalf("expected 1 parameter, got %d", len(op.Parameters))
	}

	p := op.Parameters[0]
	if p.Name != "id" || p.In != "path" {
		t.Errorf("expected path param 'id', got name=%s in=%s", p.Name, p.In)
	}

	if op.RequestBody != nil {
		t.Error("expected no request body for a struct with only path params")
	}
}

func TestGenerateOpenAPIPatch(t *testing.T) {
	routes := []Route{
		{
			Method:  "PATCH",
			Path:    "/accounts/{id}",
			InType:  typeOf(testUpdateAccountInput{}),
			OutType: typeOf(testUpdateAccountOutput{}),
		},
	}

	spec := GenerateOpenAPI("GoFast Example", "1.0.0", routes)

	data, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal spec: %v", err)
	}
	t.Logf("generated spec:\n%s", data)

	op, ok := spec.Paths["/accounts/{id}"]["patch"]
	if !ok {
		t.Fatal("expected patch operation for /accounts/{id}")
	}

	if len(op.Parameters) != 1 {
		t.Fatalf("expected 1 path parameter, got %d", len(op.Parameters))
	}
	if op.Parameters[0].Name != "id" || op.Parameters[0].In != "path" {
		t.Errorf("expected path param 'id', got name=%s in=%s", op.Parameters[0].Name, op.Parameters[0].In)
	}

	if op.RequestBody == nil {
		t.Fatal("expected a request body for PATCH with a Name field")
	}
}

func TestGenerateOpenAPIPut(t *testing.T) {
	routes := []Route{
		{
			Method:  "PUT",
			Path:    "/accounts/{id}",
			InType:  typeOf(testUpdateAccountInput{}),
			OutType: typeOf(testUpdateAccountOutput{}),
		},
	}

	spec := GenerateOpenAPI("GoFast Example", "1.0.0", routes)

	op, ok := spec.Paths["/accounts/{id}"]["put"]
	if !ok {
		t.Fatal("expected put operation for /accounts/{id}")
	}

	if op.RequestBody == nil {
		t.Fatal("expected a request body for PUT with a Name field")
	}
}

func TestGenerateOpenAPIDelete(t *testing.T) {
	routes := []Route{
		{
			Method:  "DELETE",
			Path:    "/accounts/{id}",
			InType:  typeOf(testDeleteAccountInput{}),
			OutType: typeOf(testDeleteAccountOutput{}),
		},
	}

	spec := GenerateOpenAPI("GoFast Example", "1.0.0", routes)

	op, ok := spec.Paths["/accounts/{id}"]["delete"]
	if !ok {
		t.Fatal("expected delete operation for /accounts/{id}")
	}

	if len(op.Parameters) != 1 {
		t.Fatalf("expected 1 path parameter, got %d", len(op.Parameters))
	}

	if op.RequestBody != nil {
		t.Error("expected no request body for DELETE with only a path param")
	}
}
