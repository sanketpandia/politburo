package api

// DEPRECATED: This file is deprecated in favor of feature-layer handlers
// Handlers are now initialized directly in router.go for clean architecture
// See: internal/memberships/handler.go for the new pattern

// Handlers struct is deprecated - handlers are now initialized directly in router.go
type Handlers struct {
	deps *Dependencies
}

// NewHandlers is deprecated - not used in new architecture
func NewHandlers(deps *Dependencies) *Handlers {
	return &Handlers{
		deps: deps,
	}
}
