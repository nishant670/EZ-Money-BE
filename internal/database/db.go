package database

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

// connectMaxWait bounds the startup retry loop. Railway's private network
// (*.railway.internal) needs a few seconds after the container starts before
// DNS resolves, so the first attempts fail with "no such host".
const connectMaxWait = 90 * time.Second

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

	db, err := connectWithRetry(dsn)
	if err != nil {
		log.Fatalf("failed to connect to PostgreSQL after %s: %v", connectMaxWait, err)
	}

	log.Println("connected to PostgreSQL successfully")
	DB = db
}

// connectWithRetry dials Postgres until it succeeds or connectMaxWait elapses,
// backing off between attempts. Misconfiguration still fails, just later; a
// slow-to-appear network now recovers on its own.
func connectWithRetry(dsn string) (*gorm.DB, error) {
	deadline := time.Now().Add(connectMaxWait)
	delay := time.Second

	for attempt := 1; ; attempt++ {
		db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
		if err == nil {
			return db, nil
		}

		if time.Now().After(deadline) {
			return nil, err
		}

		log.Printf("PostgreSQL not reachable (attempt %d): %v; retrying in %s", attempt, err, delay)
		time.Sleep(delay)

		if delay < 8*time.Second {
			delay *= 2
		}
	}
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
