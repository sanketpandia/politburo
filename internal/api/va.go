package api

// DEPRECATED: All VA management handlers have been migrated to va_handlers.go
// This file is kept for backward compatibility but may be removed in the future.
// Use VAHandlers methods instead:
//   - VAHandlers.SyncUser() replaces SyncUser()
//   - VAHandlers.SetRole() replaces SetRole()
//   - VAHandlers.GetConfigs() replaces GetVAConfigs()
//   - VAHandlers.ListConfigKeys() replaces ListConfigKeys()
//   - VAHandlers.SetConfigs() replaces SetConfigKeys()
//
// InitRegisterServer() is deprecated - use UserHandlers.InitServerRegistration() instead
