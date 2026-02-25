package services

import (
	stdContext "context"
	"database/sql"
	"errors"
	"fmt"
	context "infinite-experiment/politburo/internal/auth"
	"infinite-experiment/politburo/internal/common"
	"infinite-experiment/politburo/internal/constants"
	"infinite-experiment/politburo/internal/db/repositories"
	"infinite-experiment/politburo/internal/models/dtos"
	"infinite-experiment/politburo/internal/models/entities"
	"log"
	"net/http"
	"sync"
)

type RegistrationService struct {
	Cache          common.CacheService
	LiveAPI        *common.LiveAPIService
	UserRepository repositories.UserRepository
	VARepository   repositories.VARepository
}

func NewRegistrationService(liveAPI *common.LiveAPIService, cache common.CacheService, userRepo repositories.UserRepository, vaRepo repositories.VARepository) *RegistrationService {
	return &RegistrationService{
		LiveAPI:        liveAPI,
		Cache:          cache,
		UserRepository: userRepo,
		VARepository:   vaRepo,
	}
}

type InitRegistrationValidation struct {
	UserDB    *entities.User
	IFProfile *dtos.UserStats
}

func (svc *RegistrationService) InitUserRegistration(ctx stdContext.Context, ifcId string, lastFlight string) (*dtos.InitApiResponse, string, error) {

	var steps []dtos.RegistrationStep
	claims := context.GetUserClaims(ctx)

	// STEP - 1: API fetch & DB Fetch

	steps = append(steps, dtos.RegistrationStep{
		Name: "if_api&duplicacy_check", Status: true, Message: "Validated at Live API. User not duplicate.",
	})
	if claims.UserID() != "" {
		log.Printf("\nUser already registered: %s", claims.UserID())
		steps[0].Message = "User already registered"
		steps[0].Status = false
		return &dtos.InitApiResponse{
			IfcId:   ifcId,
			Status:  false,
			Message: constants.StatusRegistrationInit,
			Steps:   steps,
		}, constants.StatusError, nil
	}

	data, err := svc.UserValidation(ctx, ifcId)

	if err != nil {
		log.Printf("\n Registration Init Error: %v", err)
		steps[0].Message = err.Error()
		steps[0].Status = false
		return &dtos.InitApiResponse{
			IfcId:   ifcId,
			Status:  false,
			Message: constants.StatusRegistrationInit,
			Steps:   steps,
		}, constants.StatusError, nil
	}

	if data.IFProfile == nil {
		steps[0].Message = "User not found at Live API"
		steps[0].Status = false
		return &dtos.InitApiResponse{
			IfcId:   ifcId,
			Status:  false,
			Message: constants.StatusUserNotFound,
			Steps:   steps,
		}, constants.StatusError, nil
	}

	// STEP 2 - Fetch User Flights
	steps = append(steps, dtos.RegistrationStep{
		Name: "if_flight_history", Status: true, Message: "Fetched flight history",
	})

	steps = append(steps, dtos.RegistrationStep{
		Name: "user_check", Status: true, Message: "Last Flight validated.",
	})
	routeStr, err := svc.findRecentFlightRoute(data.IFProfile.UserID)

	if err != nil {
		steps[1].Status = false
		steps[1].Message = err.Error()
		return &dtos.InitApiResponse{
			IfcId:   ifcId,
			Status:  false,
			Message: constants.StatusFailedToFetch,
			Steps:   steps,
		}, constants.StatusFailedToFetch, nil
	}

	if routeStr == "" || routeStr != lastFlight {
		if routeStr == "" {
			steps[2].Message = "No flights found"
		} else {
			steps[2].Message = "Last flight did not match! Please check logbook"
		}
		steps[2].Status = false
		return &dtos.InitApiResponse{
			IfcId:   ifcId,
			Status:  false,
			Message: constants.StatusLogbookMismatch,
			Steps:   steps,
		}, constants.StatusLogbookMismatch, nil
	}

	if data.UserDB != nil {
		steps[1].Status = false
		steps[1].Message = "Duplicate request for user registration!"
		return &dtos.InitApiResponse{
			IfcId:   ifcId,
			Status:  false,
			Message: constants.StatusAlreadyPresent,
			Steps:   steps,
		}, constants.StatusAlreadyPresent, nil
	}

	insData := &entities.User{
		DiscordID:     claims.DiscordUserID(),
		IsActive:      true,
		IFCommunityID: ifcId,
		IFApiID:       &data.IFProfile.UserID,
	}
	log.Printf("Initiating insert: \n %v", *insData)

	if err := svc.UserRepository.InsertUser(ctx, insData); err != nil {
		log.Printf("Error: %v", err)
		return &dtos.InitApiResponse{
			IfcId:   ifcId,
			Status:  false,
			Message: constants.StatusInsertFailed,
			Steps:   steps,
		}, constants.StatusInsertFailed, nil
	}

	return &dtos.InitApiResponse{
		IfcId:   ifcId,
		Status:  true,
		Message: constants.StatusRegistered,
		Steps:   steps,
	}, "", nil

}

func (svc *RegistrationService) UserValidation(ctx stdContext.Context, ifcId string) (*InitRegistrationValidation, error) {

	claims := context.GetUserClaims(ctx)
	var (
		user        *entities.User
		dbErr       error
		apiErr      error
		statusToken int
		statsResp   *dtos.UserStatsResponse
		wg          sync.WaitGroup
	)

	wg.Add(2)

	go func() {
		defer wg.Done()
		user, dbErr = svc.UserRepository.FindUserByDiscordId(ctx, claims.UserID())
	}()

	// 2) external API call
	go func() {
		defer wg.Done()
		statsResp, statusToken, apiErr = svc.LiveAPI.GetUserByIfcId(ifcId)
	}()

	wg.Wait()

	// 1) DB: ignore ErrNoRows
	if dbErr != nil {
		if !errors.Is(dbErr, sql.ErrNoRows) {
			return nil, dbErr
		}
		user = nil
	}

	// 2) API: ignore 404
	if apiErr != nil {
		if statusToken == http.StatusNotFound {
			statsResp = nil
		} else {
			return nil, apiErr
		}
	}

	if len(statsResp.Result) == 0 {
		return nil, errors.New("no user found")
	}

	log.Printf("API Called successfully. Status: %d. Response %v", statusToken, *statsResp)
	log.Printf("DB Result: %v", user)
	// 3) return whatever you got (nil if missing)
	return &InitRegistrationValidation{
		UserDB:    user,
		IFProfile: &statsResp.Result[0],
	}, nil
}

func (svc *RegistrationService) findRecentFlightRoute(userID string) (string, error) {
	for page := 1; page <= 3; page++ {
		fltResp, _, err := svc.LiveAPI.GetUserFlights(userID, page)
		if err != nil {
			return "", fmt.Errorf("failed to fetch user flights: %w", err)
		}
		for _, flight := range fltResp.Flights {
			if flight.OriginAirport != "" && flight.DestinationAirport != "" {
				return fmt.Sprintf("%s-%s", flight.OriginAirport, flight.DestinationAirport), nil
			}
		}
	}
	return "", errors.New("no recent flight found")
}

func (svc *RegistrationService) InitServerRegistration(ctx stdContext.Context, code string, name string) (bool, []dtos.RegistrationStep, error) {
	var steps []dtos.RegistrationStep
	claims := context.GetUserClaims(ctx)

	errResp := errors.New("Failed to register server")

	steps = append(steps, dtos.RegistrationStep{
		Name:    "ip_val",
		Status:  true,
		Message: "Inputs validated",
	})

	if code == "" {
		steps[0].Status = false
		steps[0].Message = "Inputs validation failed"
		return false, steps, errResp
	}

	steps = append(steps, dtos.RegistrationStep{
		Name:    "unique_server",
		Status:  true,
		Message: "Server not present already",
	})
	if claims.ServerID() != "" {
		steps[1].Status = false
		steps[1].Message = "Server already present in database"
		return false, steps, errResp
	}

	steps = append(steps, dtos.RegistrationStep{
		Status:  true,
		Name:    "validated_user",
		Message: "Is a registered user",
	})
	if claims.UserID() == "" {
		steps[2].Status = false
		steps[2].Message = "User not registered. Please use /register to register yourself"
		return false, steps, errResp
	}

	steps = append(steps, dtos.RegistrationStep{
		Status:  true,
		Name:    "database_insert",
		Message: "VA Inserted successfully",
	})

	va := &entities.VA{
		DiscordID: claims.DiscordServerID(),
		Code:      code,
		IsActive:  true,
		Name:      name,
	}

	_, err := svc.VARepository.InsertVAWithAdmin(ctx, va, claims.UserID())
	if err != nil {
		steps = append(steps, dtos.RegistrationStep{
			Name:    "db_tx",
			Status:  false,
			Message: "Failed to save VA and user",
		})
		return false, steps, err
	}

	steps = append(steps, dtos.RegistrationStep{
		Name:    "db_tx",
		Status:  true,
		Message: "VA and admin membership committed",
	})
	return true, steps, nil
}
