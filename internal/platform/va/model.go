package va

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/lib/pq"
)

// VA represents a virtual airline
type VA struct {
	ID                string    `gorm:"column:id;primaryKey;type:uuid;default:gen_random_uuid()"`
	DiscordID         string    `gorm:"column:discord_server_id;uniqueIndex"`
	Name              string    `gorm:"column:name"`
	Code              string    `gorm:"column:code;uniqueIndex"`
	IsActive          bool      `gorm:"column:is_active;default:true"`
	IsAirtableEnabled bool      `gorm:"column:is_airtable_enabled;default:false"`
	CreatedAt         time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt         time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

// TableName specifies the table name for GORM
func (VA) TableName() string {
	return "virtual_airlines"
}

// VAConfig represents a key-value configuration for a VA
type VAConfig struct {
	ID          string    `gorm:"column:id;primaryKey;type:uuid;default:gen_random_uuid()"`
	VAID        string    `gorm:"column:va_id;type:uuid"`
	ConfigKey   string    `gorm:"column:config_key"`
	ConfigValue string    `gorm:"column:config_value"`
	CreatedAt   time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt   time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

// TableName specifies the table name for GORM
func (VAConfig) TableName() string {
	return "va_configs"
}

// ValidationStatus represents the validation state of a config
type ValidationStatus string

const (
	ValidationStatusPending    ValidationStatus = "pending"
	ValidationStatusValidating ValidationStatus = "validating"
	ValidationStatusValid      ValidationStatus = "valid"
	ValidationStatusInvalid    ValidationStatus = "invalid"
)

// Scan implements the sql.Scanner interface for ValidationStatus
func (vs *ValidationStatus) Scan(value interface{}) error {
	if value == nil {
		*vs = ValidationStatusPending
		return nil
	}
	*vs = ValidationStatus(value.(string))
	return nil
}

// Value implements the driver.Valuer interface for ValidationStatus
func (vs ValidationStatus) Value() (driver.Value, error) {
	return string(vs), nil
}

// JSONB is a custom type for JSONB fields
type JSONB map[string]interface{}

// Scan implements the sql.Scanner interface for JSONB
func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = make(map[string]interface{})
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}

	result := make(map[string]interface{})
	if err := json.Unmarshal(bytes, &result); err != nil {
		return err
	}

	*j = result
	return nil
}

// Value implements the driver.Valuer interface for JSONB
func (j JSONB) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// DataProviderConfig represents a data provider configuration
type DataProviderConfig struct {
	ID               string           `gorm:"column:id;primaryKey;type:uuid;default:gen_random_uuid()"`
	VAID             string           `gorm:"column:va_id;type:uuid;not null"`
	ProviderType     string           `gorm:"column:provider_type;type:varchar(50);not null"`
	ConfigType       string           `gorm:"column:config_type;type:varchar(50);not null"` // 'credentials', 'route', 'pilot', etc.
	ConfigData       JSONB            `gorm:"column:config_data;type:jsonb;not null"`
	ConfigVersion    int              `gorm:"column:config_version;default:1"`
	IsActive         bool             `gorm:"column:is_active;default:false"`
	ValidationStatus ValidationStatus `gorm:"column:validation_status;type:validation_status;default:'pending'"`
	FeaturesEnabled  pq.StringArray   `gorm:"column:features_enabled;type:text[]"`
	LastValidatedAt  *time.Time       `gorm:"column:last_validated_at"`
	ValidationErrors JSONB            `gorm:"column:validation_errors;type:jsonb"`
	CreatedAt        time.Time        `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt        time.Time        `gorm:"column:updated_at;autoUpdateTime"`
	CreatedBy        *string          `gorm:"column:created_by;type:uuid"`
	UpdatedBy        *string         `gorm:"column:updated_by;type:uuid"`
}

// TableName specifies the table name for DataProviderConfig
func (DataProviderConfig) TableName() string {
	return "va_data_provider_configs"
}

// ProviderValidationHistory tracks validation history
type ProviderValidationHistory struct {
	ID               string           `gorm:"column:id;primaryKey;type:uuid;default:gen_random_uuid()"`
	ConfigID         string           `gorm:"column:config_id;type:uuid;not null"`
	ValidationStatus ValidationStatus `gorm:"column:validation_status;type:validation_status;not null"`
	ValidationErrors JSONB            `gorm:"column:validation_errors;type:jsonb"`
	PhasesCompleted  pq.StringArray   `gorm:"column:phases_completed;type:text[]"`
	PhasesFailed     pq.StringArray   `gorm:"column:phases_failed;type:text[]"`
	DurationMs       *int             `gorm:"column:duration_ms"`
	ValidatedAt      time.Time        `gorm:"column:validated_at;autoCreateTime"`
	TriggeredBy      *string          `gorm:"column:triggered_by;type:varchar(50)"`
}

// TableName specifies the table name for ProviderValidationHistory
func (ProviderValidationHistory) TableName() string {
	return "va_provider_validation_history"
}
