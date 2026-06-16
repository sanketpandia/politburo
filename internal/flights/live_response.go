package flights

import (
	"context"
	"sort"
	"time"

	"infinite-experiment/politburo/infra/cache"
	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/internal/auth"
)

const (
	LiveFlightsFoundCode       = "LIVE_FLIGHTS_FOUND"
	NoLiveFlightsCode          = "NO_LIVE_FLIGHTS"
	SignedLinkUnavailableCode  = "SIGNED_LINK_UNAVAILABLE"
	VAContextNotConfiguredCode = "VA_CONTEXT_NOT_CONFIGURED"
	LiveFlightsUnavailableCode = "LIVE_FLIGHTS_UNAVAILABLE"
	liveFlightsSignedLinkTTL   = 15 * time.Minute
	liveFlightsDashboardPath   = "/dashboard/live"
)

type ResolvedVAContext struct {
	VAID   string
	UserID string
}

func BuildVALiveFlightsResponse(
	ctx context.Context,
	redisCache *cache.RedisCacheService,
	resolvedVA ResolvedVAContext,
	authSvc *auth.Service,
	uiBaseURL string,
) (VALiveFlightsResponse, error) {
	if resolvedVA.VAID == "" {
		return VALiveFlightsResponse{}, ErrVAContextNotConfigured
	}

	flights, err := GetVALiveFlightsDTOs(redisCache, resolvedVA.VAID)
	if err != nil {
		return VALiveFlightsResponse{}, err
	}

	response := VALiveFlightsResponse{
		Code:    liveFlightsCodeForFlights(flights),
		Message: liveFlightsMessageForFlights(flights),
		Flights: flights,
		Summary: BuildVALiveFlightsSummary(flights),
	}

	if authSvc == nil || resolvedVA.UserID == "" {
		return response, nil
	}

	token, err := authSvc.GenerateSignedLink(ctx, resolvedVA.UserID, resolvedVA.VAID, liveFlightsDashboardPath, liveFlightsSignedLinkTTL)
	if err != nil {
		logging.Warn("failed to generate live flights signed link",
			"error", err,
			"flight_count", len(flights),
			"summary_top_route_present", response.Summary.TopRoute != nil,
		)
		if len(flights) == 0 {
			return response, nil
		}
		response.Code = SignedLinkUnavailableCode
		response.Message = "Live flights are available, but the live map link is temporarily unavailable."
		return response, nil
	}

	response.SignedLink = auth.FormatSignedLinkURL(uiBaseURL, token)
	return response, nil
}

func BuildVALiveFlightsSummary(flights []VALiveFlightDTO) VALiveFlightsSummary {
	summary := VALiveFlightsSummary{TotalDetectedFlights: len(flights)}
	routeCounts := make(map[string]int)
	for _, flight := range flights {
		route := flight.Route
		if route == "" && flight.Origin != "" && flight.Destination != "" {
			route = flight.Origin + "-" + flight.Destination
		}
		if route == "" {
			continue
		}
		routeCounts[route]++
	}

	if len(routeCounts) == 0 {
		return summary
	}

	type routeCount struct {
		route string
		count int
	}
	routes := make([]routeCount, 0, len(routeCounts))
	for route, count := range routeCounts {
		routes = append(routes, routeCount{route: route, count: count})
	}
	sort.Slice(routes, func(i, j int) bool {
		if routes[i].count == routes[j].count {
			return routes[i].route < routes[j].route
		}
		return routes[i].count > routes[j].count
	})

	if routes[0].count > 1 {
		summary.TopRoute = &VALiveFlightsRoute{Route: routes[0].route, Count: routes[0].count}
	}
	return summary
}

func liveFlightsCodeForFlights(flights []VALiveFlightDTO) string {
	if len(flights) == 0 {
		return NoLiveFlightsCode
	}
	return LiveFlightsFoundCode
}

func liveFlightsMessageForFlights(flights []VALiveFlightDTO) string {
	if len(flights) == 0 {
		return "No live flights currently active for this VA."
	}
	return "Live flights fetched."
}
