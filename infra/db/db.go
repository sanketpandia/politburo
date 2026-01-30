package db

import (
	"fmt"
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var PgDB *gorm.DB

func InitPostgresORM(dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})

	if err != nil {
		return nil, fmt.Errorf("failed to connect to postgres: %w", err)
	}

	PgDB = db
	log.Println("Connected to Postgres via GORM")
	return db, nil
}
