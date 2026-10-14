package database

import (
	"fmt"
	"log"

	"software-exam/config"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var DB *gorm.DB

func InitDB() {
	cfg := config.AppConfig.Database
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.DBName)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect database: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("Failed to get database instance: %v", err)
	}
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetMaxIdleConns(10)

	DB = db
	log.Println("Database connected successfully")
}
