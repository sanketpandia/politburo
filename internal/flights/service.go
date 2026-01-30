package flights

import (
	"context"
	"errors"
	"fmt"
	"infinite-experiment/politburo/infra/cache"
	"infinite-experiment/politburo/internal/common"
	"infinite-experiment/politburo/internal/constants"
	"infinite-experiment/politburo/internal/models/dtos"
	"infinite-experiment/politburo/internal/platform/aircraft"
	"infinite-experiment/politburo/internal/workers"
	"log"
	"math"
	"regexp"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
)

type Service struct {
	Cache      cache.CacheInterface
	ApiService *common.LiveAPIService
	Cfg        *common.VAConfigService
	LiverySvc  *aircraft.Service
}

const maxRouteWorkers = 8

func SplitCallsign(raw string) (variable, prefix, suffix string) {
	cs := strings.TrimSpace(raw)

	//----------------------------------------------------------------------
	// 1. strip suffix
	//----------------------------------------------------------------------
	switch {
	case strings.HasSuffix(strings.ToUpper(cs), " HEAVY"):
		suffix = "Heavy"
		cs = strings.TrimSpace(cs[:len(cs)-6])
	case strings.HasSuffix(strings.ToUpper(cs), " SUPER"):
		suffix = "Super"
		cs = strings.TrimSpace(cs[:len(cs)-6])
	default:
		reFlight := regexp.MustCompile(`(?i)\s+FLIGHT\s+OF\s+(\d+)$`)
		if m := reFlight.FindStringSubmatch(cs); len(m) == 2 {
			suffix = "Flight of " + m[1]
			cs = strings.TrimSpace(reFlight.ReplaceAllString(cs, ""))
		}
	}

	//----------------------------------------------------------------------
	// 2. split the remaining string
	//----------------------------------------------------------------------
	tokens := strings.Fields(cs)
	if len(tokens) == 0 {
		return "", "", suffix
	}

	variable = tokens[len(tokens)-1]
	if len(tokens) > 1 {
		prefix = strings.Join(tokens[:len(tokens)-1], " ")
	}
	return variable, prefix, suffix
}

func NewService(
	cache cache.CacheInterface,
	liveApi *common.LiveAPIService,
	cfgSvc *common.VAConfigService,
	liverySvc *aircraft.Service,
) *Service {
	return &Service{
		Cache:      cache,
		ApiService: liveApi,
		Cfg:        cfgSvc,
		LiverySvc:  liverySvc,
	}
}

const userTTL = 15 * time.Minute // Cache user stats for 15 minutes
const fltTTL = 15 * time.Minute  // Cache flight history for 15 minutes

// Caching Strategy:
// 1. User stats (IFC ID lookup) - cached by IFC ID
//    Key: LIVE_USER_{ifcID}
//    Value: UserStatsResponse (contains UserID needed for flight lookups)
//    TTL: 15 minutes
//
// 2. User flights (Live API) - cached by UserID AND page number
//    Key: LIVE_FLIGHTS_{userID}_page_{page}
//    Value: UserFlightsResponse (paginated results from Live API)
//    TTL: 15 minutes
//    Note: Each page is cached separately for correct pagination
//
// 3. Flight history (processed) - cached by UserID AND page number
//    Key: FH_{userID}_page_{page}
//    Value: FlightHistoryDto (our processed/enriched flight data)
//    TTL: 15 minutes
//
// 4. Flight route data - cached by FlightID (for map visualization)
//    Key: FH_{flightID}
//    Value: FlightInfo (route waypoints, metadata)
//    TTL: 7 days

// -----------------------------------------------------------------------------
// 1) User-lookup by IFC ID  (GET /users?ifcId=…)
// -----------------------------------------------------------------------------
func (svc *Service) getUserByIfcIDCached(ifcID string) (*dtos.UserStatsResponse, error) {
	cacheKey := "LIVE_USER_" + ifcID

	val, err := svc.Cache.GetOrSet(cacheKey, userTTL, func() (any, error) {
		resp, _, err := svc.ApiService.GetUserByIfcId(ifcID)
		return resp, err // ← store *value* (resp is a struct)
	})
	if err != nil {
		return nil, err
	}

	lookup, ok := val.(*dtos.UserStatsResponse) // cached value is already *ptr*
	if !ok {
		return nil, fmt.Errorf("cache assertion failed for %s", cacheKey)
	}
	return lookup, nil
}

// -----------------------------------------------------------------------------
// 2) Paged flight list for a userID  (GET /users/{id}/flights?page=n)
// Note: We cache at the user level AND page number for proper pagination
// Each page is cached separately with a unique cache key
// This ensures correct pagination without returning duplicate results across pages
// -----------------------------------------------------------------------------
func (svc *Service) getUserFlightsCached(userID string, page int) (*dtos.UserFlightsResponse, error) {
	// Cache key includes page number for correct pagination
	cacheKey := fmt.Sprintf("LIVE_FLIGHTS_%s_page_%d", userID, page)

	val, err := svc.Cache.GetOrSet(cacheKey, fltTTL, func() (any, error) {
		// Fetch from API with the specific page number
		resp, _, err := svc.ApiService.GetUserFlights(userID, page)
		return resp, err
	})
	if err != nil {
		return nil, err
	}

	flts, ok := val.(*dtos.UserFlightsResponse)
	if !ok {
		return nil, fmt.Errorf("cache assertion failed for %s", cacheKey)
	}
	return flts, nil
}

func (svc *Service) GetUserFlights(ifcID string, page int, sID string) (*dtos.FlightHistoryDto, error) {

	response := &dtos.FlightHistoryDto{
		PageNo:      page,
		Error:       "",
		Records:     nil,
		HasNext:     false,
		HasPrevious: false,
		TotalPages:  0,
		TotalCount:  0,
	}

	// Fetch user by IFC ID
	flt, err := svc.getUserByIfcIDCached(ifcID)
	if err != nil || len(flt.Result) < 1 {
		response.Error = "Unable to fetch user"
		return response, err
	}

	// The first result is the user we're looking for (cache key is IFC ID)
	userStats := flt.Result[0]
	uId := userStats.UserID
	username := ifcID // Use the IFC ID as the display name
	if userStats.DiscourseUsername != nil {
		username = *userStats.DiscourseUsername
	}

	log.Printf("[GetUserFlights] Fetching flights for IFC ID: %s, User ID: %s", ifcID, uId)

	flts, err := svc.getUserFlightsCached(uId, page)

	if err != nil {
		response.Error = "Unable to fetch flights from Live API"
		return response, err
	}
	if len(flts.Flights) < 1 {
		response.Error = "No flights"
		return response, fmt.Errorf("empty result")
	}

	// Fetch cached live servers and build session ID map (server name → session ID)
	// This allows us to map flight.Server to the actual session ID needed for route API calls
	sessions, err := svc.GetLiveServers()
	serverSessionMap := make(map[string]string) // serverName → sessionID
	if err == nil && sessions != nil {
		for _, session := range *sessions {
			serverSessionMap[session.Name] = session.ID
			log.Printf("[GetUserFlights] Mapped session: %s → %s", session.Name, session.ID)
		}
	} else {
		log.Printf("[GetUserFlights] Warning: Could not fetch sessions: %v", err)
	}

	// Populate pagination metadata from Live API response
	response.HasNext = flts.HasNext
	response.HasPrevious = flts.HasPrevious
	response.TotalPages = flts.TotalPages
	response.TotalCount = flts.TotalCount

	var newSummaries []dtos.FlightSummary

	for _, rec := range flts.Flights {
		// Use new livery service (cache-first, then DB)
		aircraftName := ""
		liveryName := ""
		if liveryData := svc.LiverySvc.GetAircraftLivery(context.Background(), rec.LiveryID); liveryData != nil {
			aircraftName = liveryData.AircraftName
			liveryName = liveryData.LiveryName
		}
		// rec.TotalTime is in minutes (from Live API)
		totalMinutes := int(rec.TotalTime)
		hours := totalMinutes / 60
		minutes := totalMinutes % 60
		dur := fmt.Sprintf("%02d:%02d", hours, minutes)

		newSummaries = append(newSummaries, dtos.FlightSummary{
			FlightID:    rec.ID,
			Origin:      rec.OriginAirport,
			Destination: rec.DestinationAirport,
			Aircraft:    aircraftName,
			Livery:      liveryName,
		})

		// Map server name to session ID for the route API call
		sessionID := serverSessionMap[rec.Server]

		dto := dtos.HistoryRecord{
			FlightID:   rec.ID,
			Origin:     rec.OriginAirport,
			Dest:       rec.DestinationAirport,
			TimeStamp:  rec.Created.UTC(),
			Landings:   rec.LandingCount,
			Server:     rec.Server,
			SessionID:  sessionID, // ← Store session ID in response
			Equipment:  fmt.Sprintf("%s %s", common.GetShortAircraftName(aircraftName), common.GetShortLiveryName(liveryName)),
			Livery:     liveryName,
			Callsign:   rec.Callsign,
			Violations: len(rec.Violations),
			Duration:   dur,
			Aircraft:   aircraftName,
			DayTime:    rec.DayTime,
			NightTime:  rec.NightTime,
			XP:         rec.XP,
			WorldType:  rec.WorldType,
			Username:   username,
		}
		// Use combo key: sessionId_flightId for direct retrieval
		cacheKey := string(constants.CachePrefixFlightHistory) + sessionID + "_" + rec.ID

		log.Printf("[GetUserFlights] Flight %s on server '%s' maps to session ID: %s (cache key: %s)", rec.ID, rec.Server, sessionID, cacheKey)

		log.Printf("Checking queue eligibility:\n origin: %s, dest: %s, time: %f, time since: %v, time: %v",
			rec.DestinationAirport, rec.OriginAirport, rec.TotalTime, time.Since(rec.Created), rec.Created)
		if rec.OriginAirport != "" && rec.DestinationAirport != "" && rec.TotalTime > 0 && time.Since(rec.Created) <= 72*time.Hour {
			log.Printf("[DEBUG] Attempting to send to LogbookQueue: flightID=%s, sessionID=%s, queue_addr=%p", rec.ID, sessionID, workers.LogbookQueue)
			select {
			case workers.LogbookQueue <- workers.LogbookRequest{FlightId: rec.ID, Flight: rec, SessionId: sessionID, CacheKey: cacheKey}:
				log.Printf("[DEBUG] Sent to LogbookQueue successfully: flightID=%s, sessionID=%s", rec.ID, sessionID)
				dto.MapUrl = fmt.Sprintf("http://%s%s", "localhost:8081?i=", rec.ID)
				//dto.MapUrl = ""
			default:
				log.Printf("[DEBUG] Skipping send to LogbookQueue: flightID=%s, Origin=%s, Dest=%s, TotalTime=%f, AgeHours=%.2f",
					rec.ID, rec.OriginAirport, rec.DestinationAirport, rec.TotalTime, time.Since(rec.Created).Hours())
				dto.MapUrl = ""

			}
		} else {
			dto.MapUrl = ""
		}
		response.Records = append(response.Records, dto)

	}
	// Cache the complete flight history response
	svc.UpdateUserFlightsCache(uId, response, page)
	return response, nil
}

// UpdateUserFlightsCache caches the complete flight history response for a user
// This allows efficient pagination by caching the full paginated response with metadata
func (svc *Service) UpdateUserFlightsCache(uId string, historyDto *dtos.FlightHistoryDto, page int) {
	// Cache the complete paginated flight history response with page-specific key
	histCacheKey := fmt.Sprintf("FH_%s_page_%d", uId, page)
	svc.Cache.Set(histCacheKey, historyDto, fltTTL)

	log.Printf("[UpdateUserFlightsCache] Cached flight history for user %s, page %d with %d records, key=%s",
		uId, historyDto.PageNo, len(historyDto.Records), histCacheKey)

	// Also cache flight summaries for quick lookups and pagination metadata
	sumCacheKey := fmt.Sprintf("%s%s", constants.CachePrefixUserFlights, uId)
	var summaries []dtos.FlightSummary
	for _, rec := range historyDto.Records {
		summaries = append(summaries, dtos.FlightSummary{
			FlightID:    rec.FlightID,
			Origin:      rec.Origin,
			Destination: rec.Dest,
			Aircraft:    rec.Aircraft,
			Livery:      rec.Livery,
		})
	}
	svc.Cache.Set(sumCacheKey, summaries, fltTTL)

	log.Printf("[UpdateUserFlightsCache] Cached %d flight summaries for user %s", len(summaries), uId)
}

func (svc *Service) mapToLiveFlight(resp *dtos.FlightsResponse, sId string) *[]dtos.LiveFlight {

	if resp == nil || len(resp.Flights) == 0 {
		return nil
	}

	out := make([]dtos.LiveFlight, len(resp.Flights))
	for i, flt := range resp.Flights {
		cVar, pfx, sfx := SplitCallsign(flt.Callsign)

		// Last report
		lastReport, err := common.ParseLiveAPITime(flt.LastReport)

		if err != nil {
			fmt.Printf("Couldn't parse to time: %s", flt.LastReport)
		}

		uname := flt.Username
		if uname == "" {
			uname = "<hidden>"
		}

		alt := (int(flt.Altitude) / 100) * 100
		spd := int(math.Round(flt.Speed))

		// Use new livery service (cache-first, then DB)
		acft, liv := "", ""
		if liveryData := svc.LiverySvc.GetAircraftLivery(context.Background(), flt.LiveryID); liveryData != nil {
			acft = liveryData.AircraftName
			liv = liveryData.LiveryName
		}
		// Make DTO
		out[i] = dtos.LiveFlight{
			ReportTime:     lastReport,
			Callsign:       flt.Callsign,
			CallsignSuffix: sfx,
			SessionID:      sId,
			CallsignVar:    cVar,
			CallsignPrefix: pfx,
			IsConnected:    flt.IsConnected,
			AircraftId:     flt.AircraftID,
			LiveryId:       flt.LiveryID,
			FlightID:       flt.FlightID,
			Username:       uname,
			UserID:         flt.UserID,
			AltitudeFt:     alt,
			SpeedKts:       spd,
			Aircraft:       acft,
			Livery:         liv,
		}
	}

	return &out
}
func MatchCallsignVar(variable, startsWith, endsWith string) bool {
	v := strings.ToUpper(variable)

	if startsWith != "" && !strings.HasPrefix(v, strings.ToUpper(startsWith)) {
		return false
	}
	if endsWith != "" && !strings.HasSuffix(v, strings.ToUpper(endsWith)) {
		return false
	}
	return true
}

func FilterFlights(in []dtos.LiveFlight, pfx, sfx string) []dtos.LiveFlight {
	out := make([]dtos.LiveFlight, 0, len(in)) // fresh backing array

	for _, f := range in {
		if MatchCallsignVar(f.CallsignVar, pfx, sfx) {
			out = append(out, f) // copies struct into 'out'
		}
	}
	return out
}
func (svc *Service) GetLiveFlights(sId string) (*[]dtos.LiveFlight, error) {
	cacheKey := string(constants.CachePrefixLiveFlights) + sId
	val, err := svc.Cache.GetOrSet(cacheKey, 1*time.Minute, func() (any, error) {
		f, _, err := svc.ApiService.GetFlights(sId)

		if err != nil {
			return nil, err
		}

		flights := svc.mapToLiveFlight(f, sId)

		return *flights, nil

	})

	if err != nil {
		fmt.Printf("\nError while fetching live flights: %v", err)
		return nil, err
	}

	flts, ok := val.([]dtos.LiveFlight)
	if !ok {
		fmt.Printf("\nError while parsing flights")
		return nil, errors.New("unable to fetch live flights")
	}

	return &flts, nil
}

func (svc *Service) getFPLCacheKey(ifSid string, flightId string) string {
	return string(constants.CachePrefixFPL) + ifSid + "_" + flightId
}

func (svc *Service) GetFlightPlan(ifSid string, flightId string) (*dtos.FlightPlanResponse, error) {
	cacheKey := svc.getFPLCacheKey(ifSid, flightId)
	// log.Printf("\n\nGet FPL called. cacheKey: %s", cacheKey)
	val, err := svc.Cache.GetOrSet(cacheKey, 5*time.Minute, func() (any, error) {
		// log.Printf("\nFetching FPL. cacheKey: %s", cacheKey)

		fpl, _, err := svc.ApiService.GetFlightPlan(ifSid, flightId)
		if err != nil {
			log.Printf("Failed to fetch FPL: %v", err)
			return nil, err
		}

		// log.Printf("\nFetched FPL. Waypoints: %d", len(fpl.Waypoints))

		return *fpl, nil
	})
	if err != nil {
		return nil, err
	}

	fpl, ok := val.(dtos.FlightPlanResponse)

	if !ok {
		return nil, fmt.Errorf("failed to unmarshal FPL")
	}
	// log.Printf("Fetched FPL with waypoints %d for %s", len(fpl.Waypoints), fpl.FlightID)
	return &fpl, nil
}

// Returns origin and Destination airports
func (svc *Service) GetFlightRoute(ifSid string, flightId string) (string, string) {
	org, dest := "", ""

	fpl, err := svc.GetFlightPlan(ifSid, flightId)

	if err == nil {
		wayp := fpl.Waypoints

		wl := len(wayp)
		if wl > 1 {
			x := wayp[0]
			y := wayp[wl-1]
			// log.Printf("\nFiltering waypoints. Length: %d, First: %s, Last: %s", len(fpl.Waypoints), x, y)

			if len(x) == 4 {
				org = x
			}
			if len(y) == 4 {
				dest = y
			}
		}
	}

	return org, dest

}

func (svc *Service) enrichFlightData(flts *[]dtos.LiveFlight) *[]dtos.LiveFlight {
	start := time.Now() // ← start timer
	defer func() {
		log.Printf("enriched %d flights in %v",
			len(*flts), time.Since(start))
	}()
	grp, sem := errgroup.Group{}, make(chan struct{}, maxRouteWorkers)

	for i := range *flts {
		i := i
		sem <- struct{}{}
		grp.Go(func() error {
			defer func() { <-sem }()

			f := &(*flts)[i]

			org, dest := svc.GetFlightRoute(f.SessionID, f.FlightID)
			f.Origin, f.Destination = org, dest
			return nil
		})
	}
	_ = grp.Wait()
	return flts
}

func (svc *Service) GetVALiveFlights(ctx context.Context, vaId string) (*[]dtos.LiveFlight, error) {

	sId, ok := svc.Cfg.GetConfigVal(ctx, vaId, common.ConfigKeyIFServerID)

	if !ok || sId == "" {
		return nil, errors.New("Game server not configured")
	}

	pfx, _ := svc.Cfg.GetConfigVal(ctx, vaId, common.ConfigKeyCallsignPrefix)
	sfx, _ := svc.Cfg.GetConfigVal(ctx, vaId, common.ConfigKeyCallsignSuffix)

	// At least one of prefix or suffix must be configured
	if pfx == "" && sfx == "" {
		return nil, errors.New("callsign prefix or suffix not configured for airline")
	}

	live_flt, err := svc.GetLiveFlights(sId)
	if err != nil {
		fmt.Printf("No live flights found with error: %v", err)
		return nil, err
	}

	va_flt := FilterFlights(*live_flt, pfx, sfx)

	va_flt = *svc.enrichFlightData(&va_flt)

	return &va_flt, nil

}

// GetVALiveFlightsFromCache retrieves VA flights from cache with API fallback
// This method tries to read from the cache populated by FlightsCacheJob first,
// then falls back to the direct API call (GetVALiveFlights) if cache is unavailable
func (svc *Service) GetVALiveFlightsFromCache(ctx context.Context, vaId string) (*[]dtos.LiveFlight, error) {
	// Get VA config (prefix/suffix patterns and session ID)
	sId, ok := svc.Cfg.GetConfigVal(ctx, vaId, common.ConfigKeyIFServerID)
	if !ok || sId == "" {
		return nil, errors.New("Game server not configured")
	}

	pfx, _ := svc.Cfg.GetConfigVal(ctx, vaId, common.ConfigKeyCallsignPrefix)
	sfx, _ := svc.Cfg.GetConfigVal(ctx, vaId, common.ConfigKeyCallsignSuffix)

	// At least one of prefix or suffix must be configured
	if pfx == "" && sfx == "" {
		return nil, errors.New("callsign prefix or suffix not configured for airline")
	}

	// Try to get flights from cache first
	cachedFlights, err := svc.getFlightsFromCache(sId, pfx, sfx)
	if err == nil && cachedFlights != nil && len(*cachedFlights) > 0 {
		// Cache hit - enrich and return
		enriched := svc.enrichFlightData(cachedFlights)
		return enriched, nil
	}

	// Cache miss - fallback to direct API call
	return svc.GetVALiveFlights(ctx, vaId)
}

// getFlightsFromCache reads flights from Redis cache populated by FlightsCacheJob
func (svc *Service) getFlightsFromCache(sId string, prefix string, suffix string) (*[]dtos.LiveFlight, error) {
	// Get callsign list for session
	callsignKey := fmt.Sprintf("if:session:callsigns:%s", sId)
	callsignVal, found := svc.Cache.Get(callsignKey)
	if !found {
		return nil, errors.New("cache miss: no callsigns")
	}

	callsignStr, ok := callsignVal.(string)
	if !ok || callsignStr == "" {
		return nil, errors.New("invalid callsign format")
	}

	callsigns := strings.Split(callsignStr, "|")
	flights := make([]dtos.LiveFlight, 0)

	// Fetch each flight from cache
	for _, callsign := range callsigns {
		if callsign == "" {
			continue
		}

		// Filter by VA pattern before fetching
		if !common.MatchesVAPattern(callsign, prefix, suffix) {
			continue
		}

		// Get flight data from cache using session+callsign key
		flightKey := fmt.Sprintf("if:session:%s:flight:%s", sId, callsign)
		flightVal, found := svc.Cache.Get(flightKey)
		if !found {
			continue
		}

		// Convert cached data to LiveFlight DTO
		flight, err := svc.convertCachedToLiveFlight(flightVal)
		if err != nil {
			continue
		}

		flights = append(flights, flight)
	}

	if len(flights) == 0 {
		return nil, errors.New("no flights found in cache")
	}

	return &flights, nil
}

// convertCachedToLiveFlight converts cached flight data to LiveFlight DTO
func (svc *Service) convertCachedToLiveFlight(cachedData interface{}) (dtos.LiveFlight, error) {
	// Cached data is a map[string]interface{}
	data, ok := cachedData.(map[string]interface{})
	if !ok {
		return dtos.LiveFlight{}, errors.New("invalid cached flight format")
	}

	// Helper to safely get string from interface{}
	getString := func(key string) string {
		if val, ok := data[key]; ok {
			if str, ok := val.(string); ok {
				return str
			}
		}
		return ""
	}

	// Helper to safely get float64 from interface{}
	getFloat := func(key string) float64 {
		if val, ok := data[key]; ok {
			if f, ok := val.(float64); ok {
				return f
			}
		}
		return 0
	}

	// Helper to safely get time from interface{}
	getTimeString := func(key string) string {
		if val, ok := data[key]; ok {
			if t, ok := val.(time.Time); ok {
				return t.Format(time.RFC3339)
			}
		}
		return ""
	}

	flight := dtos.LiveFlight{
		FlightID:    getString("flight_id"),
		Callsign:    getString("callsign"),
		SessionID:   getString("session_id"),
		SpeedKts:    int(getFloat("speed")),
		AltitudeFt:  int(getFloat("altitude")),
		FlightPhase: getString("phase"),            // NEW
		LastUpdated: getTimeString("last_updated"), // NEW
	}

	// Parse callsign into components
	variable, prefix, suffix := SplitCallsign(flight.Callsign)
	flight.CallsignVar = variable
	flight.CallsignPrefix = prefix
	flight.CallsignSuffix = suffix

	return flight, nil
}

func (svc *Service) GetLiveServers() (*[]dtos.Session, error) {
	const cacheKey = string(constants.CacheKeyServers)

	if val, found := svc.Cache.Get(cacheKey); found {
		if sessions, ok := val.(*[]dtos.Session); ok {
			return sessions, nil
		}
	}

	// Fetch fresh data
	data, err := svc.ApiService.GetSessions()
	if err != nil {
		return nil, err
	}

	sessions := &data.Result
	svc.Cache.Set(cacheKey, sessions, 5*time.Minute) // 5 minutes

	return sessions, nil
}

// FindUserCurrentFlight searches for the user's current flight in VA live flights
// Uses the callsign prefix, suffix, and user callsign to match against live flights
// Returns the matching LiveFlight or nil if not found
func (svc *Service) FindUserCurrentFlight(
	ctx context.Context,
	vaID string,
	userCallsign string,
	callsignPrefix string,
	callsignSuffix string,
) (*dtos.LiveFlight, error) {
	// Get VA live flights
	vaFlights, err := svc.GetVALiveFlights(ctx, vaID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch VA live flights: %w", err)
	}

	if vaFlights == nil || len(*vaFlights) == 0 {
		return nil, fmt.Errorf("no live flights found")
	}

	// Search for matching flight
	for _, lf := range *vaFlights {
		// Extract the components from the live flight
		lfVar, lfPrefix, lfSuffix := SplitCallsign(lf.Callsign)

		// Check if this flight matches the user's flight number
		// Match if:
		// 1. Full pattern matches (prefix+number+suffix)
		// 2. Just the flight number matches in the variable part
		matchesFullPattern := (lfPrefix == callsignPrefix) && (lfVar == userCallsign) && (lfSuffix == callsignSuffix)
		matchesNumber := lfVar == userCallsign || lfVar == (callsignPrefix+userCallsign+callsignSuffix)

		log.Printf("[FindUserCurrentFlight] Checking flight: callsign=%s (prefix=%s, var=%s, suffix=%s) - fullPattern=%v, matchesNumber=%v",
			lf.Callsign, lfPrefix, lfVar, lfSuffix, matchesFullPattern, matchesNumber)

		if matchesFullPattern || matchesNumber {
			log.Printf("[FindUserCurrentFlight] Found matching flight! Callsign=%s, Aircraft=%s, Livery=%s, Route=%s-%s, Alt=%dft, Speed=%dkts",
				lf.Callsign, lf.Aircraft, lf.Livery, lf.Origin, lf.Destination, lf.AltitudeFt, lf.SpeedKts)
			return &lf, nil
		}
	}

	return nil, fmt.Errorf("current flight not found for callsign: %s", userCallsign)
}
