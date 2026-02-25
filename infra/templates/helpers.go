package templates

import (
	"encoding/json"
	"html/template"
	"strings"
)

// DictFunction creates a map from key-value pairs for use in templates
func DictFunction(values ...interface{}) map[string]interface{} {
	dict := make(map[string]interface{}, len(values)/2)
	for i := 0; i < len(values)-1; i += 2 {
		key := values[i].(string)
		dict[key] = values[i+1]
	}
	return dict
}

// JSONFunction marshals a Go object to JSON string for use in JavaScript
func JSONFunction(v interface{}) template.JS {
	jsonBytes, err := json.Marshal(v)
	if err != nil {
		// Return empty object/array on marshal error to prevent template errors
		if _, ok := v.([]interface{}); ok {
			return template.JS("[]")
		}
		return template.JS("{}")
	}
	return template.JS(jsonBytes)
}

// JSEscapeFunction escapes strings for safe use in JavaScript string literals
func JSEscapeFunction(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "'", "\\'")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "\\r")
	s = strings.ReplaceAll(s, "\t", "\\t")
	return s
}
