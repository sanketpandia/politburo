package liverymappings

import "strings"

const (
	fieldTypeAircraft = "aircraft"
	fieldTypeAirline  = "airline"

	conflictStrategyOverwrite = "overwrite"
	conflictStrategySkip      = "skip"
)

type createMappingRequest struct {
	FieldType        string   `json:"fieldType"`
	SourceIDs        []string `json:"sourceIds"`
	SourceValue      string   `json:"sourceValue"`
	TargetValue      string   `json:"targetValue"`
	ConflictStrategy string   `json:"conflictStrategy"`
}

type createMappingResponse struct {
	Requested     int      `json:"requested"`
	Created       int      `json:"created"`
	Skipped       int      `json:"skipped"`
	FieldType     string   `json:"fieldType"`
	TargetValue   string   `json:"targetValue"`
	Message       string   `json:"message"`
	ConflictMode  string   `json:"conflictMode"`
	NotFoundIDs   []string `json:"notFoundIds,omitempty"`
	UsedLegacyAPI bool     `json:"usedLegacyApi,omitempty"`
}

func normalizeCreateMappingRequest(req *createMappingRequest) {
	req.FieldType = strings.TrimSpace(req.FieldType)
	req.TargetValue = strings.TrimSpace(req.TargetValue)
	req.SourceValue = strings.TrimSpace(req.SourceValue)
	req.ConflictStrategy = strings.TrimSpace(req.ConflictStrategy)
}

func isSupportedFieldType(fieldType string) bool {
	return fieldType == fieldTypeAircraft || fieldType == fieldTypeAirline
}
