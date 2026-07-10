package models

import "time"

type AuthVerification struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	IdentifierType string     `gorm:"type:varchar(16);index;not null" json:"identifier_type"`
	Identifier     string     `gorm:"index;not null" json:"identifier"`
	OTPHash        string     `gorm:"not null" json:"-"`
	ClaimTokenHash *string    `gorm:"type:char(64);uniqueIndex" json:"-"`
	OTPExpiresAt   time.Time  `gorm:"index;not null" json:"otp_expires_at"`
	VerifiedAt     *time.Time `gorm:"index" json:"verified_at,omitempty"`
	ClaimExpiresAt *time.Time `gorm:"index" json:"claim_expires_at,omitempty"`
	ClaimUsedAt    *time.Time `gorm:"index" json:"claim_used_at,omitempty"`
	Attempts       int        `gorm:"default:0" json:"attempts"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}
