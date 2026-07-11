package main

import (
	"log"

	"finance-parser-go/internal/config"
	"finance-parser-go/internal/database"
	httpserver "finance-parser-go/internal/http"
	"finance-parser-go/internal/models"

	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load(".env")
	database.Connect()
	if err := database.DB.AutoMigrate(&models.User{}, &models.AuthSession{}, &models.AuthVerification{}, &models.Account{}, &models.Entry{}, &models.QuickPrompt{}, &models.Notification{}); err != nil {
		log.Fatalf("database schema migration failed: %v", err)
	}
	if err := database.EnsureRuntimeSchema(); err != nil {
		log.Fatalf("database runtime schema check failed: %v", err)
	}

	cfg := config.Load()
	r := httpserver.NewServer(cfg)
	log.Printf("listening on :%s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatal(err)
	}
}
