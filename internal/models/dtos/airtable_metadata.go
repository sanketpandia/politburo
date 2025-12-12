package dtos

// AirtableFieldMetadata represents a field in an Airtable table
type AirtableFieldMetadata struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"` // text, number, singleSelect, multipleSelect, date, checkbox, etc.
	Description string `json:"description,omitempty"`
}

// AirtableTableMetadata represents a table in an Airtable base
type AirtableTableMetadata struct {
	ID          string                    `json:"id"`
	Name        string                    `json:"name"`
	Description string                    `json:"description,omitempty"`
	Fields      []AirtableFieldMetadata   `json:"fields"`
	PrimaryFieldID string                 `json:"primaryFieldId,omitempty"`
}

// AirtableBaseMetadata represents the complete schema of an Airtable base
type AirtableBaseMetadata struct {
	Tables []AirtableTableMetadata `json:"tables"`
}

// FetchTableFieldsRequest is the request body for fetching table fields
// Credentials are fetched from saved configuration, not from the request
type FetchTableFieldsRequest struct {
	TableName  string `json:"table_name"`
	EntityType string `json:"entity_type"` // pilot, pirep, route, career_mode
}

// TableFieldMapperData is passed to the template for rendering field mapping UI
type TableFieldMapperData struct {
	EntityType      string
	TableName       string
	AirtableFields  []AirtableFieldMetadata
	InternalFields  []string
	ActiveVA        interface{}
}
