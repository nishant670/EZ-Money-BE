package models

import "time"

// MonthlyReview is the record that a month's review was sent, and the figures
// it quoted when it was.
//
// The unique index on (user_id, month) is the whole delivery guarantee. The job
// that emits these runs on a ticker rather than a cron, so it will re-run many
// times on the 1st and again on the 2nd, 3rd and 31st; "one notification per
// month" is enforced by the database rejecting the second insert, not by the
// job being careful about when it wakes up. That is the same shape as
// BudgetAlert's idx_budget_alert_once_period, and for the same reason: a
// duplicate re-engagement notification is precisely the failure S1 was written
// against.
//
// The amounts here are a record of what was *sent*, not the source for what is
// *shown*. The review screen recomputes from the ledger, because a month can
// still be edited after it closes and the ledger is the truth; these columns
// exist so that "what did we tell them on the 1st" is answerable later.
type MonthlyReview struct {
	ID     uint   `gorm:"primaryKey" json:"id"`
	UserID uint   `gorm:"uniqueIndex:idx_monthly_review_once;index;not null" json:"user_id"`
	User   User   `json:"-" gorm:"foreignKey:UserID"`
	Month  string `gorm:"type:char(7);uniqueIndex:idx_monthly_review_once;not null" json:"month"`

	TotalSpent       Money `gorm:"type:numeric(19,2);not null;default:0" json:"total_spent"`
	TransactionCount int   `gorm:"not null;default:0" json:"transaction_count"`

	NotificationID *uint        `gorm:"index" json:"notification_id"`
	Notification   Notification `json:"-" gorm:"foreignKey:NotificationID"`
	CreatedAt      time.Time    `json:"created_at"`
}
