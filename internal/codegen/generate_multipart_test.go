package codegen

import "testing"

func TestGenerateBindMultipart(t *testing.T) {
	structs := []StructInfo{
		{
			Name: "UploadInput",
			Fields: []Field{
				{Name: "Title", Type: "string", Tag: "`form:\"title\"`"},
				{Name: "File", Type: "*gofast.UploadedFile", Tag: "`form:\"file\"`"},
			},
		},
	}

	code := GenerateBindMultipart("main", structs)

	t.Logf("generated code:\n%s", code)

	if code == "" {
		t.Fatal("expected generated code, got empty string")
	}
}

func TestGenerateBindMultipart_NoTaggedFields_ReturnsEmpty(t *testing.T) {
	structs := []StructInfo{
		{
			Name: "PlainInput",
			Fields: []Field{
				{Name: "Name", Type: "string", Tag: ""},
			},
		},
	}

	code := GenerateBindMultipart("main", structs)

	if code != "" {
		t.Errorf("expected empty string for struct with no form-tagged fields, got: %q", code)
	}
}