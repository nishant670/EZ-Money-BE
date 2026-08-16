package main

import (
	"log"
	"os"
	"strings"

	"finance-parser-go/internal/config"
	"finance-parser-go/internal/database"
	httpserver "finance-parser-go/internal/http"
	"finance-parser-go/internal/models"

	"github.com/joho/godotenv"
)

func main() {
	loadEnv()
	database.Connect()
	if err := database.DB.AutoMigrate(
		&models.User{},
		&models.AuthSession{},
		&models.AuthVerification{},
		&models.Account{},
		&models.Entry{},
		&models.QuickPrompt{},
		&models.Notification{},
		&models.Feedback{},
		&models.Budget{},
		&models.BudgetAlert{},
		&models.MonthlyReview{},
		&models.Subscription{},
		&models.SubscriptionReminder{},
		&models.SubscriptionOccurrence{},
		&models.PushDevice{},
		&models.SplitFriend{},
		&models.SplitGroup{},
		&models.SplitGroupMember{},
		&models.SplitBill{},
		&models.SplitParticipant{},
		&models.SplitSettlement{},
	); err != nil {
		log.Fatalf("database schema migration failed: %v", err)
	}
	if err := database.EnsureRuntimeSchema(); err != nil {
		log.Fatalf("database runtime schema check failed: %v", err)
	}

	cfg := config.Load()
	httpserver.StartMaintenanceJobs(cfg)
	httpserver.StartSubscriptionAutomation(cfg)
	httpserver.StartMonthlyReviewJob(cfg)
	r := httpserver.NewServer(cfg)
	log.Printf("listening on :%s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatal(err)
	}
}

func loadEnv() {
	loadEnvFile(".env")
	loadEnvFile(config.ResolveBackendPath(".env"))
}

func loadEnvFile(path string) {
	values, err := godotenv.Read(path)
	if err != nil {
		return
	}
	for key, value := range values {
		if strings.TrimSpace(os.Getenv(key)) == "" {
			_ = os.Setenv(key, value)
		}
	}
}
