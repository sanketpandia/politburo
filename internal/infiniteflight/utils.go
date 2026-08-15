package infiniteflight

import "strings"

func NormalizeSessionName(name string) string {
	// Normalize the session name by removing extra whitespace and converting to lowercase
	normalized := strings.TrimSpace(name)
	normalized = strings.ToLower(normalized)
	normalized = strings.ReplaceAll(normalized, " ", "_")
	return normalized
}
