package models

import "time"

type QuickPrompt struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index" json:"user_id"`
	Title     string    `json:"title"`
	Amount    float64   `json:"amount"`
	Mode      string    `json:"mode"`
	Category  string    `json:"category"`
	Icon      string    `json:"icon"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
