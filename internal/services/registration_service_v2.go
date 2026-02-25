package services

import (
	"context"
	"fmt"
	"infinite-experiment/politburo/internal/platform/roles"
	"infinite-experiment/politburo/internal/models/dtos"
	gormModels "infinite-experiment/politburo/internal/models/gorm"
	"log"

	"gorm.io/gorm"
)

// LiveAPIProviderInterface defines the methods needed from the Live API provider
type LiveAPIProviderInterface interface {
	GetUserByIfcId(ctx context.Context, ifcId string) (*dtos.UserStatsResponse, int, error)
	GetUserFlights(ctx context.Context, userID string, page int) (*dtos.UserFlightsResponse, int, error)
}

// RegistrationServiceV2 handles user registration using GORM and provider pattern
type RegistrationServiceV2 struct {
	db              *gorm.DB
	liveAPIProvider LiveAPIProviderInterface
}

// NewRegistrationServiceV2 creates a new V2 registration service
func NewRegistrationServiceV2(db *gorm.DB, liveAPIProvider LiveAPIProviderInterface) *RegistrationServiceV2 {
	return &RegistrationServiceV2{
		db:              db,
		liveAPIProvider: liveAPIProvider,
	}
}

// InitUserRegistration validates and registers a new user
// If callsign is provided and server is a VA, also creates VA membership
func (svc *RegistrationServiceV2) InitUserRegistration(
	ctx context.Context,
	discordUserID string,
	discordServerID string,
	ifcId string,
	lastFlight string,
	callsign *string,
) (*dtos.InitApiResponse, error) {
	var steps []dtos.RegistrationStep

	// STEP 1: Check if user already exists
	steps = append(steps, dtos.RegistrationStep{
		Name:    "duplicate_check",
		Status:  true,
		Message: "User not already registered",
	})

	var existingUser gormModels.User
	err := svc.db.WithContext(ctx).
		Where("discord_id = ?", discordUserID).
		First(&existingUser).Error

	if err == nil {
		steps[0].Status = false
		steps[0].Message = "User already registered"
		return &dtos.InitApiResponse{
			IfcId:  ifcId,
			Status: false,
			Steps:  steps,
		}, fmt.Errorf("user already registered")
	}

	if err != gorm.ErrRecordNotFound {
		steps[0].Status = false
		steps[0].Message = "Database error during duplicate check"
		return &dtos.InitApiResponse{
			IfcId:  ifcId,
			Status: false,
			Steps:  steps,
		}, fmt.Errorf("database error: %w", err)
	}

	// STEP 2: Validate user exists in Infinite Flight Live API
	steps = append(steps, dtos.RegistrationStep{
		Name:    "if_api_validation",
		Status:  true,
		Message: "Validated at Live API",
	})

	userStatsResp, statusCode, err := svc.liveAPIProvider.GetUserByIfcId(ctx, ifcId)
	if err != nil {
		log.Printf("Live API error: %v (status: %d)", err, statusCode)
		steps[1].Status = false
		steps[1].Message = "User not found at Live API"
		return &dtos.InitApiResponse{
			IfcId:  ifcId,
			Status: false,
			Steps:  steps,
		}, fmt.Errorf("user not found in Infinite Flight: %w", err)
	}

	if len(userStatsResp.Result) == 0 {
		steps[1].Status = false
		steps[1].Message = "No user found with that IFC ID"
		return &dtos.InitApiResponse{
			IfcId:  ifcId,
			Status: false,
			Steps:  steps,
		}, fmt.Errorf("no user found with IFC ID: %s", ifcId)
	}

	ifProfile := userStatsResp.Result[0]
	log.Printf("Found IF user: %s (API ID: %s)", ifcId, ifProfile.UserID)

	// STEP 3: Validate last flight matches user's flight history
	steps = append(steps, dtos.RegistrationStep{
		Name:    "flight_validation",
		Status:  true,
		Message: "Last flight validated",
	})

	recentRoute, err := svc.findRecentFlightRoute(ctx, ifProfile.UserID)
	if err != nil {
		log.Printf("Failed to fetch flights: %v", err)
		steps[2].Status = false
		steps[2].Message = "Failed to fetch flight history"
		return &dtos.InitApiResponse{
			IfcId:  ifcId,
			Status: false,
			Steps:  steps,
		}, fmt.Errorf("failed to fetch flight history: %w", err)
	}

	if recentRoute == "" {
		steps[2].Status = false
		steps[2].Message = "No recent flights found"
		return &dtos.InitApiResponse{
			IfcId:  ifcId,
			Status: false,
			Steps:  steps,
		}, fmt.Errorf("no recent flights found")
	}

	if recentRoute != lastFlight {
		log.Printf("Flight mismatch: expected %s, got %s", lastFlight, recentRoute)
		steps[2].Status = false
		steps[2].Message = fmt.Sprintf("Last flight mismatch. Expected: %s, Found: %s", lastFlight, recentRoute)
		return &dtos.InitApiResponse{
			IfcId:  ifcId,
			Status: false,
			Steps:  steps,
		}, fmt.Errorf("last flight does not match logbook (expected: %s, found: %s)", lastFlight, recentRoute)
	}

	// STEP 4: Insert user into database (and optionally link to VA)
	steps = append(steps, dtos.RegistrationStep{
		Name:    "database_insert",
		Status:  true,
		Message: "User registered successfully",
	})

	newUser := gormModels.User{
		DiscordID:     discordUserID,
		IFCommunityID: ifcId,
		IFApiID:       &ifProfile.UserID,
		IsActive:      true,
	}

	// Use transaction if we need to create VA membership
	if callsign != nil && *callsign != "" {
		// Check if server is a VA
		var va gormModels.VA
		err := svc.db.WithContext(ctx).Where("discord_server_id = ?", discordServerID).First(&va).Error
		if err != nil && err != gorm.ErrRecordNotFound {
			steps[3].Status = false
			steps[3].Message = "Failed to check VA status"
			return &dtos.InitApiResponse{
				IfcId:  ifcId,
				Status: false,
				Steps:  steps,
			}, fmt.Errorf("failed to check VA status: %w", err)
		}

		// If VA exists, create user and membership in transaction
		if err == nil {
			err = svc.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
				// Create user
				if err := tx.Omit("id").Create(&newUser).Error; err != nil {
					return fmt.Errorf("failed to create user: %w", err)
				}

				// Create VA membership as pilot
				pilotRole := gormModels.UserVARole{
					UserID:   newUser.ID,
					VAID:     va.ID,
					Role:     roles.RolePilot,
					Callsign: *callsign,
					IsActive: true,
				}

				if err := tx.Create(&pilotRole).Error; err != nil {
					return fmt.Errorf("failed to create VA membership: %w", err)
				}

				return nil
			})

			if err != nil {
				log.Printf("Failed to register user with VA membership: %v", err)
				steps[3].Status = false
				steps[3].Message = "Failed to save user and VA membership"
				return &dtos.InitApiResponse{
					IfcId:  ifcId,
					Status: false,
					Steps:  steps,
				}, fmt.Errorf("failed to save user and VA membership: %w", err)
			}

			log.Printf("Successfully registered user %s and linked to VA %s with callsign %s", ifcId, va.Code, *callsign)
		} else {
			// Server is not a VA, just create user
			if err := svc.db.WithContext(ctx).Omit("id").Create(&newUser).Error; err != nil {
				log.Printf("Failed to insert user: %v", err)
				steps[3].Status = false
				steps[3].Message = "Failed to save user to database"
				return &dtos.InitApiResponse{
					IfcId:  ifcId,
					Status: false,
					Steps:  steps,
				}, fmt.Errorf("failed to save user: %w", err)
			}
			log.Printf("Successfully registered user: %s -> %s (callsign ignored, server not a VA)", discordUserID, ifcId)
		}
	} else {
		// No callsign provided, just create user
		if err := svc.db.WithContext(ctx).Omit("id").Create(&newUser).Error; err != nil {
			log.Printf("Failed to insert user: %v", err)
			steps[3].Status = false
			steps[3].Message = "Failed to save user to database"
			return &dtos.InitApiResponse{
				IfcId:  ifcId,
				Status: false,
				Steps:  steps,
			}, fmt.Errorf("failed to save user: %w", err)
		}
		log.Printf("Successfully registered user: %s -> %s", discordUserID, ifcId)
	}

	return &dtos.InitApiResponse{
		IfcId:   ifcId,
		Status:  true,
		Message: "User registered successfully",
		Steps:   steps,
	}, nil
}

// findRecentFlightRoute searches through recent flight pages to find the most recent valid route
func (svc *RegistrationServiceV2) findRecentFlightRoute(ctx context.Context, userID string) (string, error) {
	const maxPages = 3

	for page := 1; page <= maxPages; page++ {
		flightsResp, _, err := svc.liveAPIProvider.GetUserFlights(ctx, userID, page)
		if err != nil {
			return "", fmt.Errorf("failed to fetch page %d: %w", page, err)
		}

		// Search for first flight with valid origin and destination
		for _, flight := range flightsResp.Flights {
			if flight.OriginAirport != "" && flight.DestinationAirport != "" {
				route := fmt.Sprintf("%s-%s", flight.OriginAirport, flight.DestinationAirport)
				log.Printf("Found recent flight route: %s", route)
				return route, nil
			}
		}
	}

	return "", fmt.Errorf("no recent flights with valid routes found in %d pages", maxPages)
}

// InitServerRegistration registers a new VA/server and assigns the user as admin
func (svc *RegistrationServiceV2) InitServerRegistration(
	ctx context.Context,
	discordServerID string,
	discordUserID string,
	vaCode string,
	vaName string,
	callsignPrefix string,
	callsignSuffix string,
) (*dtos.InitServerResponse, error) {
	var steps []dtos.RegistrationStep

	// STEP 1: Validate inputs
	steps = append(steps, dtos.RegistrationStep{
		Name:    "input_validation",
		Status:  true,
		Message: "Inputs validated",
	})

	if vaCode == "" {
		steps[0].Status = false
		steps[0].Message = "VA code is required"
		return &dtos.InitServerResponse{
			VACode: vaCode,
			Status: false,
			Steps:  steps,
		}, fmt.Errorf("VA code is required")
	}

	if vaName == "" {
		steps[0].Status = false
		steps[0].Message = "VA name is required"
		return &dtos.InitServerResponse{
			VACode: vaCode,
			Status: false,
			Steps:  steps,
		}, fmt.Errorf("VA name is required")
	}

	// STEP 2: Check if server already exists
	steps = append(steps, dtos.RegistrationStep{
		Name:    "duplicate_check",
		Status:  true,
		Message: "Server not registered yet",
	})

	var existingVA gormModels.VA
	err := svc.db.WithContext(ctx).
		Where("discord_server_id = ?", discordServerID).
		First(&existingVA).Error

	if err == nil {
		steps[1].Status = false
		steps[1].Message = "Server already registered"
		return &dtos.InitServerResponse{
			VACode: vaCode,
			Status: false,
			Steps:  steps,
		}, fmt.Errorf("server already registered")
	}

	if err != gorm.ErrRecordNotFound {
		steps[1].Status = false
		steps[1].Message = "Database error during duplicate check"
		return &dtos.InitServerResponse{
			VACode: vaCode,
			Status: false,
			Steps:  steps,
		}, fmt.Errorf("database error: %w", err)
	}

	// STEP 3: Check if user is registered
	steps = append(steps, dtos.RegistrationStep{
		Name:    "user_validation",
		Status:  true,
		Message: "User is registered",
	})

	var user gormModels.User
	err = svc.db.WithContext(ctx).
		Where("discord_id = ?", discordUserID).
		First(&user).Error

	if err != nil {
		steps[2].Status = false
		steps[2].Message = "User not registered. Please use /register first"
		return &dtos.InitServerResponse{
			VACode: vaCode,
			Status: false,
			Steps:  steps,
		}, fmt.Errorf("user not registered")
	}

	// STEP 4: Insert VA and create admin membership in transaction
	steps = append(steps, dtos.RegistrationStep{
		Name:    "database_insert",
		Status:  true,
		Message: "VA and admin membership created",
	})

	err = svc.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Create VA (GORM will return the generated ID)
		newVA := gormModels.VA{
			DiscordID: discordServerID,
			Code:      vaCode,
			Name:      vaName,
			IsActive:  true,
		}

		if err := tx.Create(&newVA).Error; err != nil {
			return fmt.Errorf("failed to create VA: %w", err)
		}

		// Store callsign patterns in va_configs if provided
		if callsignPrefix != "" {
			prefixConfig := gormModels.VAConfig{
				VAID:        newVA.ID,
				ConfigKey:   "callsign_prefix",
				ConfigValue: callsignPrefix,
			}
			if err := tx.Create(&prefixConfig).Error; err != nil {
				return fmt.Errorf("failed to store callsign prefix: %w", err)
			}
		}

		if callsignSuffix != "" {
			suffixConfig := gormModels.VAConfig{
				VAID:        newVA.ID,
				ConfigKey:   "callsign_suffix",
				ConfigValue: callsignSuffix,
			}
			if err := tx.Create(&suffixConfig).Error; err != nil {
				return fmt.Errorf("failed to store callsign suffix: %w", err)
			}
		}

		// Create admin membership
		adminRole := gormModels.UserVARole{
			UserID:   user.ID,
			VAID:     newVA.ID,
			Role:     roles.RoleAdmin, // Admin role
			IsActive: true,
		}

		if err := tx.Create(&adminRole).Error; err != nil {
			return fmt.Errorf("failed to create admin role: %w", err)
		}

		return nil
	})

	if err != nil {
		log.Printf("Failed to register server: %v", err)
		steps[3].Status = false
		steps[3].Message = "Failed to save VA and admin membership"
		return &dtos.InitServerResponse{
			VACode: vaCode,
			Status: false,
			Steps:  steps,
		}, fmt.Errorf("failed to register server: %w", err)
	}

	log.Printf("Successfully registered server: %s (%s) with admin %s", vaCode, vaName, discordUserID)

	return &dtos.InitServerResponse{
		VACode:  vaCode,
		Status:  true,
		Message: "Server registered successfully",
		Steps:   steps,
	}, nil
}

// LinkUserToVA links an existing registered user to a VA with their callsign
func (svc *RegistrationServiceV2) LinkUserToVA(
	ctx context.Context,
	discordUserID string,
	discordServerID string,
	callsign string,
) (map[string]interface{}, error) {
	// Verify user exists
	var user gormModels.User
	err := svc.db.WithContext(ctx).
		Where("discord_id = ?", discordUserID).
		First(&user).Error

	if err == gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("user not registered. Please use /register first")
	}

	if err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}

	// Verify VA exists
	var va gormModels.VA
	err = svc.db.WithContext(ctx).
		Where("discord_server_id = ?", discordServerID).
		First(&va).Error

	if err == gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("server is not registered as a VA. Please use /initserver first")
	}

	if err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}

	// Check if user is already linked to this VA
	var existingRole gormModels.UserVARole
	err = svc.db.WithContext(ctx).
		Where("user_id = ? AND va_id = ?", user.ID, va.ID).
		First(&existingRole).Error

	if err == nil {
		return nil, fmt.Errorf("user already linked to this VA. Contact staff to update callsign")
	}

	if err != gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("database error: %w", err)
	}

	// Create VA membership as pilot
	pilotRole := gormModels.UserVARole{
		UserID:   user.ID,
		VAID:     va.ID,
		Role:     roles.RolePilot,
		Callsign: callsign,
		IsActive: true,
	}

	if err := svc.db.WithContext(ctx).Create(&pilotRole).Error; err != nil {
		return nil, fmt.Errorf("failed to create VA membership: %w", err)
	}

	log.Printf("Successfully linked user %s to VA %s with callsign %s", user.IFCommunityID, va.Code, callsign)

	return map[string]interface{}{
		"user_id":  user.ID,
		"va_code":  va.Code,
		"va_name":  va.Name,
		"callsign": callsign,
		"role":     string(roles.RolePilot),
	}, nil
}
