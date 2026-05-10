package database

import (
	"fmt"
	"log"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Connect() (*gorm.DB, error) {
	dsn := os.Getenv("DATABASE_DSN")
	if dsn == "" {
		dsn = "host=localhost user=mpoints password=mpoints dbname=membership_points port=5432 sslmode=disable TimeZone=Asia/Shanghai"
	}
	cfg := &gorm.Config{Logger: logger.Default.LogMode(logger.Warn)}
	var db *gorm.DB
	var err error
	for i := 0; i < 60; i++ {
		db, err = gorm.Open(postgres.Open(dsn), cfg)
		if err != nil {
			log.Printf("database not ready (%d/60): %v", i+1, err)
			time.Sleep(2 * time.Second)
			continue
		}
		sqlDB, e := db.DB()
		if e != nil {
			err = e
			log.Printf("database sql (%d/60): %v", i+1, err)
			time.Sleep(2 * time.Second)
			continue
		}
		if e := sqlDB.Ping(); e != nil {
			err = e
			log.Printf("database ping (%d/60): %v", i+1, err)
			time.Sleep(2 * time.Second)
			continue
		}
		err = nil
		break
	}
	if err != nil || db == nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetMaxOpenConns(25)
	return db, nil
}
