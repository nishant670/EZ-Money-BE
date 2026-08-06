package http

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm/clause"

	"finance-parser-go/internal/database"
	"finance-parser-go/internal/models"
)

const (
	recurringDecisionDismissed = "dismissed"
	recurringDecisionSnoozed   = "snoozed"
	recurringDecisionTracked   = "tracked"
)

type recurringCandidateDecisionInput struct {
	CandidateKey string `json:"candidate_key"`
	Merchant     string `json:"merchant"`
	Category     string `json:"category"`
	Decision     string `json:"decision"`
	SnoozedUntil string `json:"snoozed_until"`
}

func (s *Server) saveRecurringCandidateDecision(c *gin.Context) {
	userID := c.MustGet("userID").(uint)
	var input recurringCandidateDecisionInput
	if err := c.ShouldBindJSON(&input); err != nil {
		if requestBodyTooLarge(err) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "request_body_too_large"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_json"})
		return
	}

	decision, fields := input.toModel(userID)
	if len(fields) > 0 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid_recurring_candidate_decision", "fields": fields})
		return
	}

	if err := database.DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}, {Name: "candidate_key"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"merchant", "category", "decision", "snoozed_until", "last_reviewed_at", "updated_at",
		}),
	}).Create(&decision).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_save_recurring_candidate_decision"})
		return
	}
	c.JSON(http.StatusOK, decision)
}

func (input recurringCandidateDecisionInput) toModel(userID uint) (models.RecurringCandidateDecision, gin.H) {
	fields := gin.H{}
	merchant := strings.TrimSpace(input.Merchant)
	category := strings.TrimSpace(input.Category)
	candidateKey := strings.TrimSpace(input.CandidateKey)
	if candidateKey == "" {
		candidateKey = recurringCandidateKey(merchant, category)
	}
	if candidateKey == "|" || candidateKey == "" {
		fields["candidate_key"] = "is required"
	}

	decision := normalizeRecurringDecision(input.Decision)
	if decision == "" {
		fields["decision"] = "must be dismissed, snoozed, or tracked"
	}

	var snoozedUntil *time.Time
	if decision == recurringDecisionSnoozed {
		parsed, err := time.Parse("2006-01-02", strings.TrimSpace(input.SnoozedUntil))
		if err != nil {
			fields["snoozed_until"] = "must use YYYY-MM-DD"
		} else {
			snoozedUntil = &parsed
		}
	}

	now := time.Now().UTC()
	return models.RecurringCandidateDecision{
		UserID:         userID,
		CandidateKey:   candidateKey,
		Merchant:       merchant,
		Category:       category,
		Decision:       decision,
		SnoozedUntil:   snoozedUntil,
		LastReviewedAt: now,
	}, fields
}

func normalizeRecurringDecision(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case recurringDecisionDismissed:
		return recurringDecisionDismissed
	case recurringDecisionSnoozed:
		return recurringDecisionSnoozed
	case recurringDecisionTracked:
		return recurringDecisionTracked
	default:
		return ""
	}
}
