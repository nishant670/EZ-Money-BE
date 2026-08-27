package models

import "time"

type AuthSession struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	UserID    uint   `gorm:"index;not null" json:"user_id"`
	User      User   `json:"-" gorm:"foreignKey:UserID"`
	TokenHash string `gorm:"type:char(64);uniqueIndex;not null" json:"-"`
	// Kind separates the short-lived admin-console sessions from ordinary app
	// logins. Without it any 30-day app token authenticated the admin API, so
	// the eight-hour admin TTL bounded nothing.
	Kind      string     `gorm:"type:varchar(16);index;not null;default:user" json:"kind"`
	ExpiresAt time.Time  `gorm:"index;not null" json:"expires_at"`
	RevokedAt *time.Time `gorm:"index" json:"revoked_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}
