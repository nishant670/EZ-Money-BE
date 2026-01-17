package models

import (
	"time"
)

type User struct {
	ID                uint      `gorm:"primaryKey" json:"id"`
	UUID              string    `gorm:"uniqueIndex" json:"uuid"`                 // Public ID for API tokens check
	Email             *string   `gorm:"uniqueIndex" json:"email,omitempty"`      // Nullable unique email
	Phone             *string   `gorm:"uniqueIndex" json:"phone,omitempty"`      // Nullable unique phone
	DeviceID          *string   `gorm:"uniqueIndex" json:"device_id,omitempty"`  // For all users to track device
	PinHash           string    `json:"-"`                                       // Bcrypt hash, hidden from JSON
	BiometricsEnabled bool      `gorm:"default:false" json:"biometrics_enabled"` // User preference
	IsGuest           bool      `gorm:"default:false" json:"is_guest"`
	Username          string    `gorm:"uniqueIndex" json:"username"` // Unique username
	ProfileImage      string    `json:"profile_image"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	HasPin            bool      `gorm:"-" json:"has_pin"`
}
