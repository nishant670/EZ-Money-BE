package database

import (
	"fmt"
	"log"
	"os"
	"strings"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func Connect() {
	dsn := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if dsn == "" {
		var missing []string
		for _, key := range []string{"DB_HOST", "DB_PORT", "DB_USER", "DB_NAME", "DB_SSLMODE"} {
			if strings.TrimSpace(os.Getenv(key)) == "" {
				missing = append(missing, key)
			}
		}
		if len(missing) > 0 {
			log.Fatalf("missing PostgreSQL configuration: set DATABASE_URL or %s", strings.Join(missing, ", "))
		}

		dsn = buildPostgresDSN()
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect to PostgreSQL: %v", err)
	}

	log.Println("connected to PostgreSQL successfully")
	DB = db
}

func buildPostgresDSN() string {
	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	dbname := os.Getenv("DB_NAME")
	sslmode := os.Getenv("DB_SSLMODE")

	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		user, password, host, port, dbname, sslmode,
	)
}
