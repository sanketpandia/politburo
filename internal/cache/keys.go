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

	// PrefixGameFlights namespaces live flight cache entries.
	PrefixGameFlights = PrefixGame + "flights:"

	// PrefixGameFlightsActive namespaces per-server active-flight snapshots.
	PrefixGameFlightsActive = PrefixGameFlights + "active:"

	// PrefixGameFlightsHistory namespaces per-flight history snapshots keyed by flight ID.
	PrefixGameFlightsHistory = PrefixGameFlights + "history:"

	// PrefixGameLivery namespaces aircraft livery cache entries.
	PrefixGameLivery = PrefixGame + "livery:"

	// PrefixAuth namespaces short-lived authentication cache entries.
	PrefixAuth = "auth:"

	// PrefixAPIKeyStatus namespaces API key active/inactive lookups.
	PrefixAPIKeyStatus = PrefixAuth + "api_key:"

	// PrefixLoginTicket namespaces one-time signed-link tickets.
	PrefixLoginTicket = PrefixAuth + "login_ticket:"

	// PrefixSession namespaces browser session objects.
	PrefixSession = "session:"
)

func KeyAPIKeyStatus(apiKey string) string {
	return PrefixAPIKeyStatus + apiKey
}

func KeyLoginTicket(ticketID string) string {
	return PrefixLoginTicket + ticketID
}

func KeySession(sessionID string) string {
	return PrefixSession + sessionID
}

func KeyActiveFlights(normalizedName string) string {
	return PrefixGameFlightsActive + normalizedName
}

func KeyFlightHistory(flightID string) string {
	return PrefixGameFlightsHistory + flightID
}

func KeyLivery(liveryID string) string {
	return PrefixGameLivery + liveryID
}
