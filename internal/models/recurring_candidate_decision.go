package models

import "time"

type RecurringCandidateDecision struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	UserID         uint       `gorm:"uniqueIndex:idx_recurring_decision_user_key;index;not null" json:"user_id"`
	User           User       `json:"-" gorm:"foreignKey:UserID"`
	CandidateKey   string     `gorm:"type:varchar(180);uniqueIndex:idx_recurring_decision_user_key;not null" json:"candidate_key"`
	Merchant       string     `gorm:"type:varchar(120);not null;default:''" json:"merchant"`
	Category       string     `gorm:"type:varchar(80);not null;default:''" json:"category"`
	Decision       string     `gorm:"type:varchar(24);not null;index" json:"decision"`
	SnoozedUntil   *time.Time `gorm:"type:date" json:"snoozed_until,omitempty"`
	LastReviewedAt time.Time  `gorm:"not null" json:"last_reviewed_at"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}
