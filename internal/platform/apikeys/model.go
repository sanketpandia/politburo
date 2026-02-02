package apikeys

// ApiKey represents an API key in the database
type ApiKey struct {
	ID     string `gorm:"column:id;primaryKey"`
	Status bool   `gorm:"column:status;default:true"`
}

// TableName specifies the table name for GORM
func (ApiKey) TableName() string {
	return "api_keys"
}
