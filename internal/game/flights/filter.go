package flights

import "strings"

func Username(flight Flight) string {
	if flight.Username == nil {
		return ""
	}
	return *flight.Username
}

func ContainsFold(value, query string) bool {
	query = strings.TrimSpace(query)
	if query == "" {
		return true
	}
	return strings.Contains(strings.ToLower(value), strings.ToLower(query))
}
