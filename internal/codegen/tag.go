// internal/codegen/tag.go
package codegen

import "strings"

func extractValidateRules(rawTag string) []string {
	raw := strings.Trim(rawTag, "`")

	idx := strings.Index(raw, "validate:")
	if idx == -1 {
		return nil
	}

	rest := raw[idx+len("validate:"):]
	if len(rest) == 0 || rest[0] != '"' {
		return nil
	}
	rest = rest[1:]

	end := strings.Index(rest, "\"")
	if end == -1 {
		return nil
	}

	content := rest[:end]
	if content == "" {
		return nil
	}

	return strings.Split(content, ",")
}

func extractSimpleTag(rawTag, key string) string {
	raw := strings.Trim(rawTag, "`")

	idx := strings.Index(raw, key+":")
	if idx == -1 {
		return ""
	}

	rest := raw[idx+len(key+":"):]
	if len(rest) == 0 || rest[0] != '"' {
		return ""
	}
	rest = rest[1:]

	end := strings.Index(rest, "\"")
	if end == -1 {
		return ""
	}

	return rest[:end]
}

func parseQueryTag(rawValue string) (paramName, defaultValue string) {
	parts := strings.Split(rawValue, ",")
	paramName = parts[0]

	for _, p := range parts[1:] {
		if strings.HasPrefix(p, "default=") {
			defaultValue = strings.TrimPrefix(p, "default=")
		}
	}

	return paramName, defaultValue
}
