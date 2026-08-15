// Package cachedresponse defines the common response shape for endpoints that
// serve precomputed data from cache.
package cachedresponse

import "time"

type Filter struct {
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	Desc    string   `json:"desc"`
	Current any      `json:"current"`
	Default any      `json:"default"`
	Options []string `json:"options,omitempty"`
}

type Meta struct {
	LastCached          time.Time `json:"lastCached"`
	RefreshIntervalMins int       `json:"refreshIntervalMins"`
}

type Pagination struct {
	TotalLength int `json:"totalLength"`
	PageLength  int `json:"pageLength"`
	PageNumber  int `json:"pageNumber"`
}

type Data[T any] struct {
	AvailableFilters []Filter    `json:"availableFilters"`
	Result           []T         `json:"result"`
	History          *[]any      `json:"history,omitempty"`
	Meta             Meta        `json:"meta"`
	Pagination       *Pagination `json:"pagination,omitempty"`
}

type Response[T any] struct {
	Data Data[T] `json:"data"`
}
