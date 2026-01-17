package database

import (
	"fmt"
	"log"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func Connect() {
	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	dbname := os.Getenv("DB_NAME")
	sslmode := os.Getenv("DB_SSLMODE")

	fmt.Println("DB_NAME:", dbname)

	// dsn := fmt.Sprintf(
	// 	"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
	// 	host, port, user, password, dbname, sslmode,
	// )

	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		user, password, host, port, dbname, sslmode,
	)

	fmt.Println("dsn:", dsn)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("❌ Failed to connect to PostgreSQL:", err)
	}

	log.Println("✅ Connected to PostgreSQL successfully")
	DB = db
}
