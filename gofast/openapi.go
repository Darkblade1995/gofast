package gofast

import (
	"reflect"
	"strings"
)

type OpenAPISpec struct {
	OpenAPI string              `json:"openapi"`
	Info    OpenAPIInfo         `json:"info"`
	Paths   map[string]PathItem `json:"paths"`
}

type OpenAPIInfo struct {
	Title   string `json:"title"`
	Version string `json:"version"`
}

type PathItem map[string]Operation

type Operation struct {
	Parameters  []Parameter         `json:"parameters,omitempty"`
	RequestBody *RequestBody        `json:"requestBody,omitempty"`
	Responses   map[string]Response `json:"responses"`
}

type Parameter struct {
	Name     string     `json:"name"`
	In       string     `json:"in"`
	Required bool       `json:"required"`
	Schema   JSONSchema `json:"schema"`
}

type RequestBody struct {
	Content map[string]MediaType `json:"content"`
}

type Response struct {
	Description string               `json:"description"`
	Content     map[string]MediaType `json:"content"`
}

type MediaType struct {
	Schema JSONSchema `json:"schema"`
}

type JSONSchema struct {
	Type       string                `json:"type,omitempty"`
	Properties map[string]JSONSchema `json:"properties,omitempty"`
	Items      *JSONSchema           `json:"items,omitempty"`
}

func GenerateOpenAPI(title, version string, routes []Route) OpenAPISpec {
	spec := OpenAPISpec{
		OpenAPI: "3.0.0",
		Info:    OpenAPIInfo{Title: title, Version: version},
		Paths:   map[string]PathItem{},
	}

	for _, route := range routes {
		method := strings.ToLower(route.Method)

		params, bodyType := splitParamsAndBody(route.InType)

		op := Operation{
			Parameters: params,
			Responses: map[string]Response{
				"200": {
					Description: "Successful response",
					Content: map[string]MediaType{
						"application/json": {Schema: typeToSchema(route.OutType)},
					},
				},
			},
		}

		if route.Method != "GET" && bodyType != nil {
			op.RequestBody = &RequestBody{
				Content: map[string]MediaType{
					"application/json": {Schema: typeToSchema(bodyType)},
				},
			}
		}

		if _, ok := spec.Paths[route.Path]; !ok {
			spec.Paths[route.Path] = PathItem{}
		}
		spec.Paths[route.Path][method] = op
	}

	return spec
}

func splitParamsAndBody(t reflect.Type) ([]Parameter, reflect.Type) {
	if t == nil {
		return nil, nil
	}

	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	if t.Kind() != reflect.Struct {
		return nil, t
	}

	var params []Parameter
	hasBodyFields := false

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		if pathName := field.Tag.Get("path"); pathName != "" {
			params = append(params, Parameter{
				Name:     pathName,
				In:       "path",
				Required: true,
				Schema:   typeToSchema(field.Type),
			})
			continue
		}

		if queryTag := field.Tag.Get("query"); queryTag != "" {
			queryName := strings.Split(queryTag, ",")[0]
			params = append(params, Parameter{
				Name:     queryName,
				In:       "query",
				Required: false,
				Schema:   typeToSchema(field.Type),
			})
			continue
		}

		hasBodyFields = true
	}

	if hasBodyFields {
		return params, t
	}

	return params, nil
}

func typeToSchema(t reflect.Type) JSONSchema {
	if t == nil {
		return JSONSchema{Type: "object"}
	}

	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	switch t.Kind() {
	case reflect.String:
		return JSONSchema{Type: "string"}
	case reflect.Int, reflect.Int32, reflect.Int64:
		return JSONSchema{Type: "integer"}
	case reflect.Bool:
		return JSONSchema{Type: "boolean"}
	case reflect.Slice:
		item := typeToSchema(t.Elem())
		return JSONSchema{Type: "array", Items: &item}
	case reflect.Struct:
		props := map[string]JSONSchema{}
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			if field.Tag.Get("path") != "" || field.Tag.Get("query") != "" {
				continue
			}
			jsonName := jsonFieldName(field)
			if jsonName == "-" {
				continue
			}
			props[jsonName] = typeToSchema(field.Type)
		}
		return JSONSchema{Type: "object", Properties: props}
	default:
		return JSONSchema{Type: "object"}
	}
}

func jsonFieldName(field reflect.StructField) string {
	tag := field.Tag.Get("json")
	if tag == "" {
		return field.Name
	}
	name := strings.Split(tag, ",")[0]
	if name == "" {
		return field.Name
	}
	return name
}
