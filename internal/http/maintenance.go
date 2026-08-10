package http

import (
	"fmt"
	"log"
	"time"

	"finance-parser-go/internal/billing"
	"finance-parser-go/internal/config"
	"finance-parser-go/internal/database"
)

type maintenanceResult struct {
	ExpiredCreditGrants     int
	AnonymousGuestRetention billing.AnonymousGuestRetentionResult
}

func StartMaintenanceJobs(cfg *config.Config) {
	if cfg == nil || !cfg.MaintenanceJobsEnabled {
		return
	}
	interval := time.Duration(cfg.MaintenanceIntervalHours) * time.Hour
	if interval <= 0 {
		interval = 24 * time.Hour
	}

	go func() {
		runMaintenanceAndLog(cfg)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			runMaintenanceAndLog(cfg)
		}
	}()
}

func runMaintenanceAndLog(cfg *config.Config) {
	result, err := runMaintenanceOnce(cfg)
	if err != nil {
		log.Printf("maintenance jobs failed: %v", err)
		return
	}
	log.Printf(
		"maintenance jobs completed: expired_credit_grants=%d anonymous_guest_ai_usage=%d anonymous_guest_limit_events=%d anonymous_guest_daily_usage=%d anonymous_guest_keys=%d anonymous_guest_credit_ledger=%d anonymous_guest_credit_grants=%d",
		result.ExpiredCreditGrants,
		result.AnonymousGuestRetention.AIUsageEvents,
		result.AnonymousGuestRetention.AIUsageLimitEvents,
		result.AnonymousGuestRetention.DailyCreditUsage,
		result.AnonymousGuestRetention.GuestUsageKeys,
		result.AnonymousGuestRetention.CreditLedger,
		result.AnonymousGuestRetention.CreditGrants,
	)
}

func runMaintenanceOnce(cfg *config.Config) (maintenanceResult, error) {
	if cfg == nil {
		return maintenanceResult{}, fmt.Errorf("config is required")
	}
	retentionDays := cfg.AnonymousGuestRetentionDays
	if retentionDays <= 0 {
		retentionDays = int(billing.AnonymousGuestRetention / (24 * time.Hour))
	}

	service := billing.NewCreditService(database.DB)
	expired, err := service.ExpireCredits()
	if err != nil {
		return maintenanceResult{}, fmt.Errorf("expire credits: %w", err)
	}
	retention, err := service.PurgeAnonymousGuestUsage(time.Duration(retentionDays) * 24 * time.Hour)
	if err != nil {
		return maintenanceResult{}, fmt.Errorf("purge anonymous guest usage: %w", err)
	}
	return maintenanceResult{ExpiredCreditGrants: expired, AnonymousGuestRetention: retention}, nil
}
