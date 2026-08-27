package db

import (
	"fmt"
	"time"

	"gxp-map-video-backend/internal/route"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Database struct {
	*gorm.DB
}

func New(path string) (*Database, error) {
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// Auto migrate models
	if err := db.AutoMigrate(
		&route.Route{},
		&route.TrackPoint{},
		&route.RouteSegment{},
		&route.StoryEvent{},
		&route.ExportTask{},
	); err != nil {
		return nil, fmt.Errorf("auto migrate: %w", err)
	}

	return &Database{DB: db}, nil
}

func (d *Database) Close() error {
	sqlDB, err := d.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// HealthCheck simple health check
func (d *Database) HealthCheck() error {
	sqlDB, err := d.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Ping()
}

// Now returns current time
func Now() time.Time {
	return time.Now()
}
