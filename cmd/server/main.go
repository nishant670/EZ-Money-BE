package main

import (
	"fmt"
	"log"
	"os"

	"finance-parser-go/internal/config"
	"finance-parser-go/internal/database"
	httpserver "finance-parser-go/internal/http"
	"finance-parser-go/internal/models"

	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load(".env")
	fmt.Println("DB_NAME:", os.Getenv("DB_NAME"))
	fmt.Println("DB_USER:", os.Getenv("DB_USER"))
	database.Connect()
	if err := database.DB.AutoMigrate(&models.User{}, &models.AuthSession{}, &models.Account{}, &models.Entry{}, &models.QuickPrompt{}); err != nil {
		log.Fatalf("database schema migration failed: %v", err)
	}

	cfg := config.Load()
	r := httpserver.NewServer(cfg)
	log.Printf("listening on :%s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatal(err)
	}
}
