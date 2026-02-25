package auth

import "infinite-experiment/politburo/internal/platform/roles"

// Common interface.
/**
* Should contain:
	- RequestType
	- DiscordId
	- UserId
	- ServerId
	- VA ID
*/
type UserClaims interface {
	UserID() string
	Role() string
	Source() string
	HasPermission(action string) bool
	ServerID() string
	DiscordUserID() string
	DiscordServerID() string
}

type JWTClaims struct {
	UserUUID  string
	RoleValue roles.VARole
	VaUUID    string
}

func (c *JWTClaims) UserID() string { return c.UserUUID }
func (c *JWTClaims) Role() string { // implements UserClaims
	return string(c.RoleValue) // or c.RoleValue.String()
}
func (c *JWTClaims) ServerID() string          { return c.VaUUID }
func (c *JWTClaims) Source() string            { return "JWT" }
func (c *JWTClaims) HasPermission(string) bool { return true }
func (c *JWTClaims) DiscordUserID() string     { return "" }
func (c *JWTClaims) DiscordServerID() string   { return "" }

type APIKeyClaims struct {
	UserUUID           string
	RoleValue          roles.VARole
	VaUUID             string
	DiscordUIDVal      string
	DiscordServerIDVal string
}

func (c *APIKeyClaims) UserID() string { return c.UserUUID }
func (c *APIKeyClaims) Role() string { // implements UserClaims
	return string(c.RoleValue) // or c.RoleValue.String()
}
func (c *APIKeyClaims) ServerID() string          { return c.VaUUID }
func (c *APIKeyClaims) Source() string            { return "API_KEY" }
func (c *APIKeyClaims) HasPermission(string) bool { return true }
func (c *APIKeyClaims) DiscordUserID() string     { return c.DiscordUIDVal }
func (c *APIKeyClaims) DiscordServerID() string   { return c.DiscordServerIDVal }
