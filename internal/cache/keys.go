// Package cache owns shared cache storage primitives and key conventions.
package cache

const (
	// PrefixGame namespaces cache entries derived from Infinite Flight game data.
	PrefixGame = "game:"

	// PrefixGameSessions namespaces session cache entries.
	PrefixGameSessions = PrefixGame + "sessions:"

	// KeyActiveSessions contains the latest active-session snapshot.
	KeyActiveSessions = PrefixGameSessions + "active"

	// KeySessionNames lists normalized session names from the latest refresh.
	KeySessionNames = PrefixGameSessions + "names"

	// PrefixAuth namespaces short-lived authentication cache entries.
	PrefixAuth = "auth:"

	// PrefixAPIKeyStatus namespaces API key active/inactive lookups.
	PrefixAPIKeyStatus = PrefixAuth + "api_key:"
)

func KeyAPIKeyStatus(apiKey string) string {
	return PrefixAPIKeyStatus + apiKey
}
